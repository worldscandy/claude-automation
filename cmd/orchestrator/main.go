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
	"github.com/claude-automation/pkg/kubernetes"
	"github.com/google/go-github/v57/github"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

type Orchestrator struct {
	githubClient      *github.Client
	workspaceRoot     string
	sessionManager    *SessionManager
	containerManager  *container.ContainerManager
	podManager        *kubernetes.PodManager
	owner             string
	repo              string
	containerMode     bool
	kubernetesMode    bool
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
	UseKubernetes   bool
	WorkerContainer *container.WorkerContainer
	WorkerPod       *kubernetes.WorkerPod
}

func NewOrchestrator() (*Orchestrator, error) {
	// Load environment variables
	if err := godotenv.Load(".env-secret"); err != nil {
		// Fallback to .env if .env-secret doesn't exist
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: .env-secret and .env files not found, using environment variables")
		}
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

	// Workspace setup (Pod内完結型では不要、レガシー互換性のためのみ保持)
	workspaceRoot := "/tmp/orchestrator-workspace" // ホスト依存を削除
	
	// Sessions setup (Pod内完結型では不要、レガシー互換性のためのみ保持)  
	sessionsDir := "/tmp/orchestrator-sessions" // ホスト依存を削除

	// Container manager setup
	containerMode := os.Getenv("CONTAINER_MANAGER_MODE") == "docker"
	kubernetesMode := os.Getenv("ORCHESTRATOR_MODE") == "kubernetes"
	var containerManager *container.ContainerManager
	var podManager *kubernetes.PodManager
	
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

	// Kubernetes Pod Manager setup
	if kubernetesMode {
		namespace := os.Getenv("NAMESPACE")
		if namespace == "" {
			namespace = "claude-automation"
		}
		
		pm, err := kubernetes.NewPodManager(namespace, "/tmp/k8s-workspaces", "/tmp/k8s-sessions")
		if err != nil {
			log.Printf("Warning: Failed to create pod manager: %v", err)
			kubernetesMode = false
		} else {
			podManager = pm
			log.Println("Kubernetes pod manager initialized successfully")
		}
	}

	return &Orchestrator{
		githubClient:     githubClient,
		workspaceRoot:    workspaceRoot,
		sessionManager:   &SessionManager{},
		containerManager: containerManager,
		podManager:       podManager,
		owner:            owner,
		repo:             repo,
		containerMode:    containerMode,
		kubernetesMode:   kubernetesMode,
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

	// Create worker container or pod based on mode
	var workerContainer *container.WorkerContainer
	var workerPod *kubernetes.WorkerPod
	useContainer := o.containerMode && o.containerManager != nil
	useKubernetes := o.kubernetesMode && o.podManager != nil
	
	if useKubernetes {
		// Kubernetes mode - create worker pod
		config := &kubernetes.RepositoryConfig{
			Image:     "claude-automation-claude", // From repo-mapping.yaml
			Workspace: "/home/claude/workspace",
			Env:       []string{"NODE_ENV=development"},
		}
		
		workerPod, err = o.podManager.CreateWorkerPod(ctx, issueNumber, repository, config)
		if err != nil {
			log.Printf("Failed to create worker pod, falling back to host execution: %v", err)
			useKubernetes = false
		} else {
			log.Printf("Created worker pod for issue #%d: %s", issueNumber, workerPod.ID)
			
			// Wait for pod to be ready
			if err := o.podManager.WaitForPodReady(ctx, workerPod.PodName, 2*time.Minute); err != nil {
				log.Printf("Pod failed to become ready, falling back to host execution: %v", err)
				useKubernetes = false
			}
		}
	} else if useContainer {
		// Docker mode - create worker container
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
		UseKubernetes:   useKubernetes,
		WorkerContainer: workerContainer,
		WorkerPod:       workerPod,
	}

	// Cleanup container or pod when done
	if useKubernetes && workerPod != nil {
		defer func() {
			if err := o.podManager.DeleteWorkerPod(ctx, workerPod.PodName); err != nil {
				log.Printf("Failed to cleanup worker pod: %v", err)
			}
		}()
	} else if useContainer && workerContainer != nil {
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
	if execution.UseKubernetes && execution.WorkerPod != nil {
		return o.executeInPod(ctx, execution)
	} else if execution.UseContainer && execution.WorkerContainer != nil {
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
	
	// Execute Claude CLI directly (host execution)
	claudeCmd := append([]string{"claude"}, args...)
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

// executeInPod executes Claude CLI inside a Kubernetes worker pod (Pod内完結型)
func (o *Orchestrator) executeInPod(ctx context.Context, execution *TaskExecution) (string, error) {
	podName := execution.WorkerPod.PodName
	
	// Pod内完結型: 固定パスを使用
	workspaceDir := "/workspace"
	sessionFile := fmt.Sprintf("/tmp/claude/session-%s.json", execution.IssueID)
	taskFile := fmt.Sprintf("/tmp/claude/task-%s.txt", execution.IssueID)
	
	// Pod内でディレクトリ構造をセットアップ
	setupCmd := fmt.Sprintf("mkdir -p %s && mkdir -p /tmp/claude && mkdir -p /app/auth", workspaceDir)
	if _, err := o.podManager.ExecuteInPod(ctx, podName, setupCmd); err != nil {
		return "", fmt.Errorf("failed to setup pod workspace: %w", err)
	}
	
	// Prepare Claude CLI command for pod execution
	claudeArgs := []string{
		"claude",
		"--print",
		"--max-turns", strconv.Itoa(execution.MaxTurns),
		"--verbose",
	}
	
	if execution.OutputFormat != "" {
		claudeArgs = append(claudeArgs, "--output-format", execution.OutputFormat)
	}
	
	// Session management (Pod内パス)
	claudeArgs = append(claudeArgs, "--continue", sessionFile)

	// Build task context (Pod内完結型)
	taskContext := o.buildPodTaskContext(execution, workspaceDir)
	
	// Create task file in pod using proper escaping
	createTaskFileCmd := fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF", taskFile, taskContext)
	if _, err := o.podManager.ExecuteInPod(ctx, podName, createTaskFileCmd); err != nil {
		return "", fmt.Errorf("failed to create task file in pod: %w", err)
	}
	
	// Execute Claude CLI in pod with task file as input
	claudeCmd := fmt.Sprintf("cd %s && %s < %s", workspaceDir, strings.Join(claudeArgs, " "), taskFile)
	output, err := o.podManager.ExecuteInPod(ctx, podName, claudeCmd)
	if err != nil {
		// Get pod logs for debugging
		logs, logErr := o.podManager.GetPodLogs(ctx, podName)
		if logErr != nil {
			log.Printf("Failed to get pod logs: %v", logErr)
		} else {
			log.Printf("Pod logs:\n%s", logs)
		}
		return "", fmt.Errorf("claude command failed in pod: %w\nOutput: %s", err, output)
	}

	// Cleanup temp files
	cleanupCmd := fmt.Sprintf("rm -f %s", taskFile)
	if _, err := o.podManager.ExecuteInPod(ctx, podName, cleanupCmd); err != nil {
		log.Printf("Warning: failed to cleanup temp file: %v", err)
	}

	// Update session usage (Pod内管理)
	o.sessionManager.UpdateSessionUsage(execution.IssueID)
	
	return output, nil
}

// buildTaskContext creates comprehensive context for Claude CLI (Legacy Host版)
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

// buildPodTaskContext creates comprehensive context for Claude CLI (Pod内完結型)
func (o *Orchestrator) buildPodTaskContext(execution *TaskExecution, workspaceDir string) string {
	return fmt.Sprintf(`## Kubernetes Pod GitHub Issue Automation Context

You are Claude Code running inside a Kubernetes Pod, automating GitHub issue processing. 

### 🎯 Task Details:
- **Issue ID**: #%s
- **Repository**: %s  
- **Task**: %s

### 🐳 Pod Environment:
- **Workspace**: %s (Pod内固定パス)
- **Session File**: /tmp/claude/session-%s.json
- **Auth Files**: /app/auth/.claude.json, /app/auth/.claude/.credentials.json

### 🛠️ Available Tools:
- Read/Write/Edit files using Claude Code tools
- TodoWrite/TodoRead for task management
- Bash tool for command execution (Pod内)
- All MCP tools are available

### 📋 Instructions:
1. Use TodoWrite to plan your approach
2. Break down the task into manageable steps
3. Execute each step using appropriate tools
4. Work within the Pod's /workspace directory
5. Provide clear progress updates
6. Use session management for multi-turn conversations

### ⚡ Pod Advantages:
- Isolated execution environment
- Pre-configured Claude CLI authentication  
- Dedicated workspace and session management
- Kubernetes native scalability

Begin processing this task autonomously in the Pod environment. Use --continue for session continuity.`,
		execution.IssueID,
		execution.Repository,
		execution.Task,
		workspaceDir,
		execution.IssueID)
}

// SessionManager methods (Pod内完結型対応)
func (sm *SessionManager) CreateSession(issueID string) (string, error) {
	// Pod内完結型: ホストファイルシステムに依存しない
	sessionFile := fmt.Sprintf("/tmp/claude/session-%s.json", issueID)
	
	sessionInfo := SessionInfo{
		SessionFile: sessionFile,
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
	}
	
	sm.sessions.Store(issueID, sessionInfo)
	
	// Pod内でセッションファイルは動的作成されるため、ここでは作成しない
	log.Printf("Registered session for issue %s: %s (Pod内で動的作成)", issueID, sessionFile)
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
	if o.kubernetesMode {
		executionMode = "Kubernetes Pod"
	} else if o.containerMode {
		executionMode = "Docker Container"
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