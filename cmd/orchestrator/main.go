package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/claude-automation/pkg/container"
	"github.com/google/go-github/v57/github"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

type Orchestrator struct {
	githubClient      *github.Client
	workspaceRoot     string
	sessionManager    *SessionManager
	containerManager  *container.ContainerManager
	owner             string
	repo              string
	containerMode     bool
	mu                sync.Mutex
}

type SessionManager struct {
	sessions sync.Map // issue_id -> SessionInfo
}

type SessionInfo struct {
	SessionFile string
	LastUsed    time.Time
	CreatedAt   time.Time
}

type TaskExecution struct {
	IssueID         string
	IssueNumber     int
	Task            string
	Repository      string
	SessionFile     string
	MaxTurns        int
	OutputFormat    string
	UseContainer    bool
	WorkerContainer *container.WorkerContainer
}

func NewOrchestrator() (*Orchestrator, error) {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// GitHub client
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	githubClient := github.NewClient(tc)

	// Repository info
	owner := os.Getenv("GITHUB_OWNER")
	if owner == "" {
		owner = "worldscandy"
	}
	repo := os.Getenv("GITHUB_REPO")
	if repo == "" {
		repo = "claude-automation"
	}

	// Workspace setup
	workspaceRoot := filepath.Join(".", "workspaces")
	os.MkdirAll(workspaceRoot, 0755)
	
	// Sessions setup
	sessionsDir := filepath.Join(".", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	// Container manager setup
	containerMode := os.Getenv("CONTAINER_MANAGER_MODE") == "docker"
	var containerManager *container.ContainerManager
	
	if containerMode {
		configPath := filepath.Join(".", "config", "repo-mapping.yaml")
		cm, err := container.NewContainerManager(configPath, workspaceRoot, sessionsDir)
		if err != nil {
			log.Printf("Warning: Failed to create container manager: %v", err)
			containerMode = false
		} else {
			containerManager = cm
			log.Println("Container manager initialized successfully")
		}
	}

	return &Orchestrator{
		githubClient:     githubClient,
		workspaceRoot:    workspaceRoot,
		sessionManager:   &SessionManager{},
		containerManager: containerManager,
		owner:            owner,
		repo:             repo,
		containerMode:    containerMode,
	}, nil
}

// ProcessIssueTask processes a GitHub issue with @claude mention
func (o *Orchestrator) ProcessIssueTask(ctx context.Context, issueNumber int, task string, repository string) error {
	issueID := strconv.Itoa(issueNumber)
	log.Printf("Processing issue #%d: %s (repository: %s)", issueNumber, task, repository)

	// Create session for this issue
	sessionFile, err := o.sessionManager.CreateSession(issueID)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Create worker container if in container mode
	var workerContainer *container.WorkerContainer
	useContainer := o.containerMode && o.containerManager != nil
	
	if useContainer {
		workerContainer, err = o.containerManager.CreateWorkerContainer(ctx, issueNumber, repository)
		if err != nil {
			log.Printf("Failed to create worker container, falling back to host execution: %v", err)
			useContainer = false
		} else {
			log.Printf("Created worker container for issue #%d: %s", issueNumber, workerContainer.ID)
		}
	}

	// Execute task with Claude CLI
	execution := &TaskExecution{
		IssueID:         issueID,
		IssueNumber:     issueNumber,
		Task:            task,
		Repository:      repository,
		SessionFile:     sessionFile,
		MaxTurns:        10, // Allow autonomous execution up to 10 turns
		OutputFormat:    "json",
		UseContainer:    useContainer,
		WorkerContainer: workerContainer,
	}

	// Cleanup container when done
	if useContainer && workerContainer != nil {
		defer func() {
			if err := o.containerManager.StopWorkerContainer(ctx, workerContainer.ID); err != nil {
				log.Printf("Failed to cleanup worker container: %v", err)
			}
		}()
	}

	result, err := o.ExecuteClaudeTask(ctx, execution)
	if err != nil {
		// Post error to issue
		o.PostToIssue(ctx, issueNumber, fmt.Sprintf("❌ **エラーが発生しました**\n\n```\n%v\n```", err))
		return err
	}

	// Post success result to issue
	o.PostToIssue(ctx, issueNumber, fmt.Sprintf("✅ **タスク完了**\n\n%s", result))
	log.Printf("Task completed for issue #%d", issueNumber)
	return nil
}

// ExecuteClaudeTask executes a task using advanced Claude CLI features
func (o *Orchestrator) ExecuteClaudeTask(ctx context.Context, execution *TaskExecution) (string, error) {
	if execution.UseContainer && execution.WorkerContainer != nil {
		return o.executeInContainer(ctx, execution)
	}
	return o.executeOnHost(ctx, execution)
}

// executeOnHost executes Claude CLI on the host system
func (o *Orchestrator) executeOnHost(ctx context.Context, execution *TaskExecution) (string, error) {
	workDir := filepath.Join(o.workspaceRoot, execution.IssueID)
	
	// Ensure workspace exists
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}

	// Prepare Claude CLI command with advanced features
	args := []string{
		"--print", // Non-interactive mode with output
		"--max-turns", strconv.Itoa(execution.MaxTurns), // Allow autonomous execution
		"--verbose", // Get detailed progress
	}
	
	if execution.OutputFormat != "" {
		args = append(args, "--output-format", execution.OutputFormat)
	}
	
	if execution.SessionFile != "" {
		args = append(args, "--continue", execution.SessionFile)
	}

	// Build comprehensive task context
	taskContext := o.buildTaskContext(execution)
	
	// Execute Claude CLI with bash wrapper for compatibility
	claudeCmd := append([]string{"bash", "/usr/local/bin/claude"}, args...)
	cmd := exec.Command(claudeCmd[0], claudeCmd[1:]...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(taskContext)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude command failed: %w\nOutput: %s", err, output)
	}

	// Update session usage
	o.sessionManager.UpdateSessionUsage(execution.IssueID)
	
	return string(output), nil
}

// executeInContainer executes Claude CLI inside a worker container
func (o *Orchestrator) executeInContainer(ctx context.Context, execution *TaskExecution) (string, error) {
	containerID := execution.WorkerContainer.ID
	
	// Prepare Claude CLI command for container execution
	claudeArgs := []string{
		"claude",
		"--print",
		"--max-turns", strconv.Itoa(execution.MaxTurns),
		"--verbose",
	}
	
	if execution.OutputFormat != "" {
		claudeArgs = append(claudeArgs, "--output-format", execution.OutputFormat)
	}
	
	if execution.SessionFile != "" {
		// Map session file to container path
		sessionPath := "/app/sessions/" + filepath.Base(execution.SessionFile)
		claudeArgs = append(claudeArgs, "--continue", sessionPath)
	}

	// Build task context
	taskContext := o.buildTaskContext(execution)
	
	// Create temporary file with task context
	tempFile := fmt.Sprintf("/tmp/claude-task-%s.txt", execution.IssueID)
	createTaskFileCmd := fmt.Sprintf("echo %s > %s", 
		strings.ReplaceAll(taskContext, `"`, `\"`), tempFile)
	
	if _, err := o.containerManager.ExecuteInContainer(ctx, containerID, createTaskFileCmd); err != nil {
		return "", fmt.Errorf("failed to create task file in container: %w", err)
	}
	
	// Execute Claude CLI in container with task file as input
	claudeCmd := strings.Join(claudeArgs, " ") + " < " + tempFile
	output, err := o.containerManager.ExecuteInContainer(ctx, containerID, claudeCmd)
	if err != nil {
		// Get container logs for debugging
		logs, logErr := o.containerManager.GetContainerLogs(ctx, containerID)
		if logErr != nil {
			log.Printf("Failed to get container logs: %v", logErr)
		} else {
			log.Printf("Container logs:\n%s", logs)
		}
		return "", fmt.Errorf("claude command failed in container: %w\nOutput: %s", err, output)
	}

	// Cleanup temp file
	cleanupCmd := "rm -f " + tempFile
	if _, err := o.containerManager.ExecuteInContainer(ctx, containerID, cleanupCmd); err != nil {
		log.Printf("Warning: failed to cleanup temp file: %v", err)
	}

	// Update session usage
	o.sessionManager.UpdateSessionUsage(execution.IssueID)
	
	return output, nil
}

// buildTaskContext creates comprehensive context for Claude CLI
func (o *Orchestrator) buildTaskContext(execution *TaskExecution) string {
	return fmt.Sprintf(`## GitHub Issue Automation Context

You are Claude Code automating GitHub issue processing. Your task is to autonomously complete the following request.

### Issue ID: #%s
### Task: %s

### Available Tools:
- Read/Write/Edit files using Claude Code tools
- TodoWrite/TodoRead for task management
- Bash tool for command execution
- All MCP tools are available

### Instructions:
1. Use TodoWrite to plan your approach
2. Break down the task into manageable steps
3. Execute each step using appropriate tools
4. Provide clear progress updates
5. Ensure all work is completed within the workspace

### Workspace: %s

Begin processing this task autonomously. Use --continue if you need multiple conversation turns.`,
		execution.IssueID,
		execution.Task,
		filepath.Join(o.workspaceRoot, execution.IssueID))
}

// SessionManager methods
func (sm *SessionManager) CreateSession(issueID string) (string, error) {
	sessionDir := filepath.Join(".", "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}
	
	sessionFile := filepath.Join(sessionDir, fmt.Sprintf("issue-%s.session", issueID))
	
	sessionInfo := SessionInfo{
		SessionFile: sessionFile,
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
	}
	
	sm.sessions.Store(issueID, sessionInfo)
	
	// Create empty session file
	if _, err := os.Create(sessionFile); err != nil {
		return "", fmt.Errorf("failed to create session file: %w", err)
	}
	
	log.Printf("Created session file for issue %s: %s", issueID, sessionFile)
	return sessionFile, nil
}

func (sm *SessionManager) UpdateSessionUsage(issueID string) {
	if sessionInfoVal, ok := sm.sessions.Load(issueID); ok {
		sessionInfo := sessionInfoVal.(SessionInfo)
		sessionInfo.LastUsed = time.Now()
		sm.sessions.Store(issueID, sessionInfo)
	}
}

func (sm *SessionManager) GetSession(issueID string) (string, bool) {
	if sessionInfoVal, ok := sm.sessions.Load(issueID); ok {
		sessionInfo := sessionInfoVal.(SessionInfo)
		return sessionInfo.SessionFile, true
	}
	return "", false
}

// PostToIssue posts a comment to a GitHub issue
func (o *Orchestrator) PostToIssue(ctx context.Context, issueNumber int, message string) error {
	comment := &github.IssueComment{
		Body: &message,
	}
	
	_, _, err := o.githubClient.Issues.CreateComment(ctx, o.owner, o.repo, issueNumber, comment)
	if err != nil {
		log.Printf("Failed to post comment to issue #%d: %v", issueNumber, err)
		return err
	}
	
	log.Printf("Posted comment to issue #%d", issueNumber)
	return nil
}

// Integration with monitor - this would be called by the monitor
func (o *Orchestrator) HandleIssueRequest(ctx context.Context, issueNumber int, task string, repository string) {
	log.Printf("Received issue processing request: #%d (repository: %s)", issueNumber, repository)
	
	// Determine execution mode
	executionMode := "Host"
	if o.containerMode {
		executionMode = "Container"
	}
	
	// Acknowledge the task
	acknowledgment := fmt.Sprintf("🤖 **Claude Automation System**\n\nタスクを受信しました。処理を開始します...\n\n**Issue ID:** #%d\n**Repository:** %s\n**Execution Mode:** %s\n**Session:** `issue-%d`\n**Workspace:** `workspaces/issue-%d/`", 
		issueNumber, repository, executionMode, issueNumber, issueNumber)
	
	if err := o.PostToIssue(ctx, issueNumber, acknowledgment); err != nil {
		log.Printf("Failed to acknowledge task: %v", err)
		return
	}
	
	// Process the task asynchronously
	go func() {
		if err := o.ProcessIssueTask(ctx, issueNumber, task, repository); err != nil {
			log.Printf("Failed to process issue #%d: %v", issueNumber, err)
		}
	}()
}

func main() {
	ctx := context.Background()
	
	orchestrator, err := NewOrchestrator()
	if err != nil {
		log.Fatal("Failed to create orchestrator:", err)
	}

	// Check for command line arguments for issue processing
	if len(os.Args) >= 7 && os.Args[1] == "-issue" && os.Args[3] == "-task" && os.Args[5] == "-repo" {
		issueNumber, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatal("Invalid issue number:", err)
		}
		
		task := os.Args[4]
		repository := os.Args[6]
		
		log.Printf("Processing issue #%d with task: %s (repository: %s)", issueNumber, task, repository)
		
		// Process the issue task
		orchestrator.HandleIssueRequest(ctx, issueNumber, task, repository)
		
		// Wait for completion (in real implementation, this would be handled differently)
		time.Sleep(30 * time.Second)
		return
	}

	// Default mode: Run as standalone service
	log.Println("Starting Orchestrator in service mode...")
	
	// Test with a sample issue if no arguments provided
	issueID := "test-001"
	
	// Test Claude CLI integration
	log.Printf("Testing Claude CLI integration...")
	
	execution := &TaskExecution{
		IssueID:      issueID,
		IssueNumber:  1,
		Task:         "Create a simple test file with 'Hello World' content",
		Repository:   "worldscandy/claude-automation",
		MaxTurns:     3,
		OutputFormat: "json",
		UseContainer: false,
	}
	
	result, err := orchestrator.ExecuteClaudeTask(ctx, execution)
	if err != nil {
		log.Printf("Failed to execute Claude task: %v", err)
	} else {
		log.Printf("Claude task result:\n%s", result)
	}
}