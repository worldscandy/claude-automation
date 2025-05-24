package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

type Orchestrator struct {
	dockerClient    *client.Client
	socketPath      string
	workspaceRoot   string
	activeContainers sync.Map // issue_id -> container_id
	mu              sync.Mutex
}

type AgentMessage struct {
	IssueID string `json:"issue_id"`
	Command string `json:"command"`
	Type    string `json:"type"` // "exec", "result", "error"
}

type AgentResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

func NewOrchestrator() (*Orchestrator, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	workspaceRoot := filepath.Join(".", "shared", "workspaces")
	os.MkdirAll(workspaceRoot, 0755)

	return &Orchestrator{
		dockerClient:  cli,
		socketPath:    "/tmp/claude-agent.sock",
		workspaceRoot: workspaceRoot,
	}, nil
}

func (o *Orchestrator) StartContainer(ctx context.Context, issueID string, image string) (string, error) {
	containerName := fmt.Sprintf("claude-worker-%s", issueID)
	workDir := filepath.Join(o.workspaceRoot, issueID)
	
	// Create workspace directory
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}

	// Get absolute paths
	absWorkDir, _ := filepath.Abs(workDir)
	absAgentPath, _ := filepath.Abs("./cmd/agent/agent")
	
	// Container configuration
	config := &container.Config{
		Image: image,
		Env: []string{
			fmt.Sprintf("ISSUE_ID=%s", issueID),
			"SOCKET_PATH=/tmp/claude-agent.sock",
		},
		Cmd: []string{"/agent"},
		WorkingDir: "/workspace",
	}

	// Host configuration with mounts
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: absWorkDir,
				Target: "/workspace",
			},
			{
				Type:   mount.TypeBind,
				Source: absAgentPath,
				Target: "/agent",
				ReadOnly: true,
			},
			{
				Type:   mount.TypeBind,
				Source: o.socketPath,
				Target: "/tmp/claude-agent.sock",
			},
		},
		AutoRemove: true,
	}

	// Create container
	resp, err := o.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := o.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	// Store container ID
	o.activeContainers.Store(issueID, resp.ID)
	
	log.Printf("Started container %s for issue %s", resp.ID[:12], issueID)
	return resp.ID, nil
}

func (o *Orchestrator) ExecuteClaudeCommand(ctx context.Context, issueID, instruction string) (string, error) {
	workDir := filepath.Join(o.workspaceRoot, issueID)
	
	// Check if container exists
	containerID, ok := o.activeContainers.Load(issueID)
	if !ok {
		return "", fmt.Errorf("no container for issue %s", issueID)
	}

	// Prepare context for Claude
	context := fmt.Sprintf(`You are working in a Docker container environment.
Working directory: /workspace (mounted from %s)
Container ID: %s
To execute commands in the container, output them in the following format:
EXEC: <command>
For example:
EXEC: pip install requests
EXEC: python script.py`, workDir, containerID.(string)[:12])

	// Execute Claude CLI
	cmd := exec.Command("claude", "--print")
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s\n\n%s", context, instruction))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude command failed: %w\nOutput: %s", err, output)
	}

	// Parse output for EXEC commands
	result := string(output)
	lines := strings.Split(result, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "EXEC:") {
			command := strings.TrimSpace(strings.TrimPrefix(line, "EXEC:"))
			if execResult, err := o.ExecuteInContainer(ctx, issueID, command); err != nil {
				log.Printf("Failed to execute in container: %v", err)
			} else {
				log.Printf("Container execution result: %s", execResult)
			}
		}
	}

	return result, nil
}

func (o *Orchestrator) ExecuteInContainer(ctx context.Context, issueID, command string) (string, error) {
	// Connect to agent socket
	conn, err := net.Dial("unix", o.socketPath)
	if err != nil {
		return "", fmt.Errorf("failed to connect to agent: %w", err)
	}
	defer conn.Close()

	// Send command
	msg := AgentMessage{
		IssueID: issueID,
		Command: command,
		Type:    "exec",
	}
	
	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	// Read response
	var resp AgentResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("command failed: %s", resp.Error)
	}

	return resp.Output, nil
}

func (o *Orchestrator) StopContainer(ctx context.Context, issueID string) error {
	containerID, ok := o.activeContainers.Load(issueID)
	if !ok {
		return fmt.Errorf("no container for issue %s", issueID)
	}

	timeout := 10
	if err := o.dockerClient.ContainerStop(ctx, containerID.(string), container.StopOptions{
		Timeout: &timeout,
	}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	o.activeContainers.Delete(issueID)
	return nil
}

func main() {
	ctx := context.Background()
	
	orchestrator, err := NewOrchestrator()
	if err != nil {
		log.Fatal("Failed to create orchestrator:", err)
	}

	// Test with a sample issue
	issueID := "test-001"
	
	// Start container
	containerID, err := orchestrator.StartContainer(ctx, issueID, "alpine:latest")
	if err != nil {
		log.Fatal("Failed to start container:", err)
	}
	
	log.Printf("Started container: %s", containerID[:12])
	
	// Wait for agent to initialize
	time.Sleep(2 * time.Second)

	// Test Claude command
	result, err := orchestrator.ExecuteClaudeCommand(ctx, issueID, 
		"Create a simple shell script that prints 'Hello from Container!' and execute it to test the environment")
	if err != nil {
		log.Printf("Failed to execute Claude command: %v", err)
	} else {
		log.Printf("Claude output:\n%s", result)
	}

	// Cleanup
	time.Sleep(5 * time.Second)
	orchestrator.StopContainer(ctx, issueID)
}