package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

func main() {
	ctx := context.Background()
	
	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal("Failed to create Docker client:", err)
	}

	// Create workspace
	workDir := "./shared/workspaces/simple-test"
	if err := exec.Command("mkdir", "-p", workDir).Run(); err != nil {
		log.Fatal("Failed to create workspace:", err)
	}

	absWorkDir, _ := filepath.Abs(workDir)
	
	fmt.Println("🚀 Simple Remote Execution Test")
	fmt.Println("================================")
	
	// Test 1: Claude CLI basic functionality
	fmt.Println("\n📝 Test 1: Claude CLI Basic Test")
	cmd := exec.Command("claude", "--print")
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader("Create a simple shell script named 'test.sh' that prints 'Hello from Claude!' and make it executable")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Claude command failed: %v\nOutput: %s", err, output)
	} else {
		outputStr := string(output)
		if len(outputStr) > 200 {
			fmt.Printf("✅ Claude response received:\n%s...\n", outputStr[:200])
		} else {
			fmt.Printf("✅ Claude response received:\n%s\n", outputStr)
		}
	}

	// Test 2: Container creation and basic execution
	fmt.Println("\n🐳 Test 2: Container Creation Test")
	
	config := &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"sh", "-c", "echo 'Container started successfully' && sleep 10"},
		WorkingDir: "/workspace",
	}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: absWorkDir,
				Target: "/workspace",
			},
		},
		AutoRemove: true,
	}

	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "claude-simple-test")
	if err != nil {
		log.Printf("❌ Failed to create container: %v", err)
		return
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		log.Printf("❌ Failed to start container: %v", err)
		return
	}

	fmt.Printf("✅ Container started: %s\n", resp.ID[:12])

	// Test 3: Direct command execution in container
	fmt.Println("\n⚡ Test 3: Direct Command Execution Test")
	
	// Execute command directly
	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", "echo 'Direct execution works!' > /workspace/direct_test.txt && cat /workspace/direct_test.txt"},
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := cli.ContainerExecCreate(ctx, resp.ID, execConfig)
	if err != nil {
		log.Printf("❌ Failed to create exec: %v", err)
	} else {
		execAttachResp, err := cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
		if err != nil {
			log.Printf("❌ Failed to attach exec: %v", err)
		} else {
			defer execAttachResp.Close()
			
			// Read output
			output := make([]byte, 1024)
			n, _ := execAttachResp.Reader.Read(output)
			fmt.Printf("✅ Command output: %s", output[:n])
		}
	}

	// Cleanup
	time.Sleep(2 * time.Second)
	timeout := 5
	cli.ContainerStop(ctx, resp.ID, container.StopOptions{Timeout: &timeout})
	
	fmt.Println("\n🎉 Simple test completed!")
	fmt.Println("\nNext steps:")
	fmt.Println("- Implement proper Unix socket communication")
	fmt.Println("- Add GitHub Issue monitoring")
	fmt.Println("- Build full automation system")
}