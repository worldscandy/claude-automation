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

	"github.com/google/go-github/v57/github"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

type Orchestrator struct {
	githubClient   *github.Client
	workspaceRoot  string
	sessionManager *SessionManager
	owner          string
	repo           string
	mu             sync.Mutex
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
	IssueID      string
	Task         string
	SessionFile  string
	MaxTurns     int
	OutputFormat string
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

	return &Orchestrator{
		githubClient:   githubClient,
		workspaceRoot:  workspaceRoot,
		sessionManager: &SessionManager{},
		owner:          owner,
		repo:           repo,
	}, nil
}

// ProcessIssueTask processes a GitHub issue with @claude mention
func (o *Orchestrator) ProcessIssueTask(ctx context.Context, issueNumber int, task string) error {
	issueID := strconv.Itoa(issueNumber)
	log.Printf("Processing issue #%d: %s", issueNumber, task)

	// Create session for this issue
	sessionFile, err := o.sessionManager.CreateSession(issueID)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Execute task with Claude CLI
	execution := &TaskExecution{
		IssueID:      issueID,
		Task:         task,
		SessionFile:  sessionFile,
		MaxTurns:     10, // Allow autonomous execution up to 10 turns
		OutputFormat: "json",
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
	
	// Execute Claude CLI
	cmd := exec.Command("claude", args...)
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
func (o *Orchestrator) HandleIssueRequest(ctx context.Context, issueNumber int, task string) {
	log.Printf("Received issue processing request: #%d", issueNumber)
	
	// Acknowledge the task
	acknowledgment := fmt.Sprintf("🤖 **Claude Automation System**\n\nタスクを受信しました。処理を開始します...\n\n**Issue ID:** #%d\n**Session:** `issue-%d`\n**Workspace:** `shared/workspaces/%d/`", 
		issueNumber, issueNumber, issueNumber)
	
	if err := o.PostToIssue(ctx, issueNumber, acknowledgment); err != nil {
		log.Printf("Failed to acknowledge task: %v", err)
		return
	}
	
	// Process the task asynchronously
	go func() {
		if err := o.ProcessIssueTask(ctx, issueNumber, task); err != nil {
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
	if len(os.Args) >= 5 && os.Args[1] == "-issue" && os.Args[3] == "-task" {
		issueNumber, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatal("Invalid issue number:", err)
		}
		
		task := os.Args[4]
		
		log.Printf("Processing issue #%d with task: %s", issueNumber, task)
		
		// Process the issue task
		orchestrator.HandleIssueRequest(ctx, issueNumber, task)
		
		// Wait for completion (in real implementation, this would be handled differently)
		time.Sleep(30 * time.Second)
		return
	}

	// Default mode: Run as standalone service
	log.Println("Starting Orchestrator in service mode...")
	
	// Test with a sample issue if no arguments provided
	issueID := "test-001"
	
	// Test Claude CLI integration without container
	log.Printf("Testing Claude CLI integration...")
	
	execution := &TaskExecution{
		IssueID:      issueID,
		Task:         "Create a simple test file with 'Hello World' content",
		MaxTurns:     3,
		OutputFormat: "json",
	}
	
	result, err := orchestrator.ExecuteClaudeTask(ctx, execution)
	if err != nil {
		log.Printf("Failed to execute Claude task: %v", err)
	} else {
		log.Printf("Claude task result:\n%s", result)
	}
}