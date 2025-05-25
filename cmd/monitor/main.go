package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
	"github.com/claude-automation/pkg/kubernetes"
)

type IssueMonitor struct {
	client       *github.Client
	owner        string
	repo         string
	pollInterval time.Duration
	lastChecked  time.Time
	podManager   *kubernetes.PodManager
}

type IssueRequest struct {
	IssueNumber int
	Task        string
	Repository  string
}

func NewIssueMonitor() (*IssueMonitor, error) {
	// Load .env-secret file (fallback to .env)
	if err := godotenv.Load(".env-secret"); err != nil {
		// Fallback to .env if .env-secret doesn't exist
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: .env-secret and .env files not found, using environment variables")
		}
	}

	// Get configuration from environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN not set")
	}

	owner := os.Getenv("GITHUB_OWNER")
	if owner == "" {
		owner = "worldscandy"
	}

	repo := os.Getenv("GITHUB_REPO")
	if repo == "" {
		repo = "claude-automation"
	}

	// Create GitHub client with OAuth2 token
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Initialize Kubernetes Pod Manager
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "claude-automation"
	}
	
	podManager, err := kubernetes.NewPodManager(namespace, "/app/workspaces", "/app/sessions")
	if err != nil {
		return nil, fmt.Errorf("failed to create pod manager: %w", err)
	}

	// Setup ServiceAccount and RBAC
	if err := podManager.SetupServiceAccount(ctx); err != nil {
		log.Printf("Warning: Failed to setup ServiceAccount: %v", err)
	}

	return &IssueMonitor{
		client:       client,
		owner:        owner,
		repo:         repo,
		pollInterval: 30 * time.Second,
		lastChecked:  time.Now(),
		podManager:   podManager,
	}, nil
}

func (m *IssueMonitor) Start(ctx context.Context) error {
	log.Printf("Starting GitHub Issue Monitor for %s/%s", m.owner, m.repo)
	log.Printf("Polling interval: %v", m.pollInterval)
	
	// Initial check
	if err := m.checkIssues(ctx); err != nil {
		log.Printf("Initial check failed: %v", err)
	}

	// Start polling
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down issue monitor")
			return ctx.Err()
		case <-ticker.C:
			if err := m.checkIssues(ctx); err != nil {
				log.Printf("Error checking issues: %v", err)
			}
		}
	}
}

func (m *IssueMonitor) checkIssues(ctx context.Context) error {
	// List issues updated since last check
	opts := &github.IssueListByRepoOptions{
		State:     "open",
		Sort:      "updated",
		Direction: "desc",
		Since:     m.lastChecked,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	issues, _, err := m.client.Issues.ListByRepo(ctx, m.owner, m.repo, opts)
	if err != nil {
		return fmt.Errorf("failed to list issues: %w", err)
	}

	// Check each issue for @claude mentions
	for _, issue := range issues {
		if issue.Body == nil {
			continue
		}

		// Check issue body for @claude mention
		if m.hasClauldeMention(*issue.Body) {
			log.Printf("Found @claude mention in issue #%d: %s", *issue.Number, *issue.Title)
			m.handleIssue(ctx, issue)
		}

		// Check recent comments
		if err := m.checkIssueComments(ctx, issue); err != nil {
			log.Printf("Error checking comments for issue #%d: %v", *issue.Number, err)
		}
	}

	m.lastChecked = time.Now()
	return nil
}

func (m *IssueMonitor) checkIssueComments(ctx context.Context, issue *github.Issue) error {
	opts := &github.IssueListCommentsOptions{
		Since: &m.lastChecked,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	comments, _, err := m.client.Issues.ListComments(ctx, m.owner, m.repo, *issue.Number, opts)
	if err != nil {
		return err
	}

	for _, comment := range comments {
		if comment.Body == nil {
			continue
		}

		if m.hasClauldeMention(*comment.Body) {
			log.Printf("Found @claude mention in comment on issue #%d", *issue.Number)
			m.handleIssueComment(ctx, issue, comment)
		}
	}

	return nil
}

func (m *IssueMonitor) hasClauldeMention(text string) bool {
	// Check for @claude mention (case insensitive, not in email addresses)
	mentionRegex := regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9.])@claude\b`)
	return mentionRegex.MatchString(text)
}

func (m *IssueMonitor) handleIssue(ctx context.Context, issue *github.Issue) {
	// Extract task from issue
	task := m.extractTask(issue)
	
	// Detect target repository
	repository := m.detectTargetRepository(issue, task)
	
	log.Printf("Processing task from issue #%d", *issue.Number)
	log.Printf("Task: %s", task)
	log.Printf("Target repository: %s", repository)

	// Trigger orchestrator to handle the task
	m.triggerOrchestrator(ctx, *issue.Number, task, repository)
}

func (m *IssueMonitor) handleIssueComment(ctx context.Context, issue *github.Issue, comment *github.IssueComment) {
	// Extract task from comment
	task := m.extractTaskFromComment(comment)
	
	// Detect target repository
	repository := m.detectTargetRepository(issue, task)
	
	log.Printf("Processing task from comment on issue #%d", *issue.Number)
	log.Printf("Task: %s", task)
	log.Printf("Target repository: %s", repository)

	// Trigger orchestrator to handle the task
	m.triggerOrchestrator(ctx, *issue.Number, task, repository)
}

func (m *IssueMonitor) extractTask(issue *github.Issue) string {
	// Remove @claude mention and extract the actual task
	text := *issue.Body
	mentionRegex := regexp.MustCompile(`(?i)@claude\s*`)
	task := mentionRegex.ReplaceAllString(text, "")
	return strings.TrimSpace(task)
}

func (m *IssueMonitor) extractTaskFromComment(comment *github.IssueComment) string {
	text := *comment.Body
	mentionRegex := regexp.MustCompile(`(?i)@claude\s*`)
	task := mentionRegex.ReplaceAllString(text, "")
	return strings.TrimSpace(task)
}

// detectTargetRepository determines which repository the task should target
func (m *IssueMonitor) detectTargetRepository(issue *github.Issue, task string) string {
	// Priority 1: Look for explicit repository mention in task
	repoRegex := regexp.MustCompile(`(?i)(?:repository|repo):\s*([a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+)`)
	if matches := repoRegex.FindStringSubmatch(task); len(matches) > 1 {
		return matches[1]
	}
	
	// Priority 2: Look for GitHub URL patterns
	urlRegex := regexp.MustCompile(`https://github\.com/([a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+)`)
	fullText := ""
	if issue.Body != nil {
		fullText += *issue.Body + " "
	}
	fullText += task
	
	if matches := urlRegex.FindStringSubmatch(fullText); len(matches) > 1 {
		return matches[1]
	}
	
	// Priority 3: Look for owner/repo pattern in text
	ownerRepoRegex := regexp.MustCompile(`\b([a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+)\b`)
	if matches := ownerRepoRegex.FindStringSubmatch(task); len(matches) > 1 {
		// Validate this looks like a repository
		parts := strings.Split(matches[1], "/")
		if len(parts) == 2 && !strings.Contains(matches[1], ".") {
			return matches[1]
		}
	}
	
	// Priority 4: Check issue labels for repository hints
	for _, label := range issue.Labels {
		if label.Name != nil {
			labelName := *label.Name
			if strings.HasPrefix(labelName, "repo:") {
				return strings.TrimPrefix(labelName, "repo:")
			}
		}
	}
	
	// Priority 5: Use current repository as default
	return fmt.Sprintf("%s/%s", m.owner, m.repo)
}

// triggerOrchestrator communicates with orchestrator to process the task
func (m *IssueMonitor) triggerOrchestrator(ctx context.Context, issueNumber int, task string, repository string) {
	// In a real implementation, this would communicate with the orchestrator
	// via HTTP, gRPC, or message queue. For now, we'll simulate it.
	
	log.Printf("Triggering orchestrator for issue #%d (repository: %s)", issueNumber, repository)
	
	// For this implementation, we'll spawn the orchestrator process
	// In production, the orchestrator would be a separate service
	go m.executeOrchestratorTask(ctx, issueNumber, task, repository)
}

func (m *IssueMonitor) executeOrchestratorTask(ctx context.Context, issueNumber int, task string, repository string) {
	// Create worker pod instead of Docker container
	
	// Default configuration for Claude workers
	config := &kubernetes.RepositoryConfig{
		Image:     "worldscandy/claude-automation:latest",
		Workspace: "/workspace",
		Env: []string{
			"CLAUDE_API_KEY=" + os.Getenv("CLAUDE_API_KEY"),
			"GITHUB_TOKEN=" + os.Getenv("GITHUB_TOKEN"),
		},
		Commands: map[string]string{
			"claude": "claude",
		},
	}

	// Create worker pod
	workerPod, err := m.podManager.CreateWorkerPod(ctx, issueNumber, repository, config)
	if err != nil {
		log.Printf("Failed to create worker pod for issue #%d: %v", issueNumber, err)
		
		// Post error to issue
		errorBody := fmt.Sprintf("❌ **Kubernetes Pod作成に失敗しました**\n\n```\n%s\n```", err.Error())
		comment := &github.IssueComment{Body: &errorBody}
		m.client.Issues.CreateComment(ctx, m.owner, m.repo, issueNumber, comment)
		return
	}

	log.Printf("Created worker pod %s for issue #%d", workerPod.PodName, issueNumber)

	// Post progress update to issue
	progressBody := fmt.Sprintf("🚀 **タスク処理を開始しました**\n\nIssue #%d の処理を Kubernetes Pod `%s` で実行中です...", 
		issueNumber, workerPod.PodName)
	comment := &github.IssueComment{Body: &progressBody}
	m.client.Issues.CreateComment(ctx, m.owner, m.repo, issueNumber, comment)

	// Wait for pod to be ready
	if err := m.podManager.WaitForPodReady(ctx, workerPod.PodName, 5*time.Minute); err != nil {
		log.Printf("Pod %s failed to become ready: %v", workerPod.PodName, err)
		
		// Get pod logs for debugging
		logs, _ := m.podManager.GetPodLogs(ctx, workerPod.PodName)
		
		errorBody := fmt.Sprintf("❌ **Pod起動に失敗しました**\n\n```\n%s\n```\n\n**Pod Logs:**\n```\n%s\n```", 
			err.Error(), logs)
		comment := &github.IssueComment{Body: &errorBody}
		m.client.Issues.CreateComment(ctx, m.owner, m.repo, issueNumber, comment)
		
		// Cleanup failed pod
		m.podManager.DeleteWorkerPod(ctx, workerPod.PodName)
		return
	}

	log.Printf("Pod %s is ready, executing Claude CLI task", workerPod.PodName)

	// Execute Claude CLI task in the pod
	claudeCommand := fmt.Sprintf("claude --print --max-turns 10 --verbose '%s'", task)
	output, err := m.podManager.ExecuteInPod(ctx, workerPod.PodName, claudeCommand)
	
	if err != nil {
		log.Printf("Claude CLI execution failed in pod %s: %v", workerPod.PodName, err)
		
		// Get pod logs for debugging
		logs, _ := m.podManager.GetPodLogs(ctx, workerPod.PodName)
		
		errorBody := fmt.Sprintf("❌ **Claude CLI実行に失敗しました**\n\n```\n%s\n```\n\n**Pod Logs:**\n```\n%s\n```", 
			err.Error(), logs)
		comment := &github.IssueComment{Body: &errorBody}
		m.client.Issues.CreateComment(ctx, m.owner, m.repo, issueNumber, comment)
	} else {
		// Post successful result
		resultBody := fmt.Sprintf("✅ **タスクが完了しました**\n\n**実行結果:**\n```\n%s\n```", output)
		comment := &github.IssueComment{Body: &resultBody}
		m.client.Issues.CreateComment(ctx, m.owner, m.repo, issueNumber, comment)
		
		log.Printf("Task completed successfully for issue #%d", issueNumber)
	}

	// Cleanup: Delete the worker pod after task completion
	if err := m.podManager.DeleteWorkerPod(ctx, workerPod.PodName); err != nil {
		log.Printf("Warning: Failed to cleanup pod %s: %v", workerPod.PodName, err)
	}
}

func main() {
	ctx := context.Background()

	monitor, err := NewIssueMonitor()
	if err != nil {
		log.Fatal("Failed to create issue monitor:", err)
	}

	// Start monitoring
	if err := monitor.Start(ctx); err != nil {
		log.Fatal("Monitor error:", err)
	}
}