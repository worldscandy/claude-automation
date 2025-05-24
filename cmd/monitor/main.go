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
)

type IssueMonitor struct {
	client       *github.Client
	owner        string
	repo         string
	pollInterval time.Duration
	lastChecked  time.Time
}

func NewIssueMonitor() (*IssueMonitor, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
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

	return &IssueMonitor{
		client:       client,
		owner:        owner,
		repo:         repo,
		pollInterval: 30 * time.Second,
		lastChecked:  time.Now(),
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
	// Check for @claude mention (case insensitive)
	mentionRegex := regexp.MustCompile(`(?i)@claude\b`)
	return mentionRegex.MatchString(text)
}

func (m *IssueMonitor) handleIssue(ctx context.Context, issue *github.Issue) {
	// Extract task from issue
	task := m.extractTask(issue)
	
	log.Printf("Processing task from issue #%d", *issue.Number)
	log.Printf("Task: %s", task)

	// TODO: Start container and execute task
	// For now, just acknowledge
	m.acknowledgeTask(ctx, issue, "issue")
}

func (m *IssueMonitor) handleIssueComment(ctx context.Context, issue *github.Issue, comment *github.IssueComment) {
	// Extract task from comment
	task := m.extractTaskFromComment(comment)
	
	log.Printf("Processing task from comment on issue #%d", *issue.Number)
	log.Printf("Task: %s", task)

	// TODO: Start container and execute task
	// For now, just acknowledge
	m.acknowledgeTask(ctx, issue, "comment")
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

func (m *IssueMonitor) acknowledgeTask(ctx context.Context, issue *github.Issue, source string) {
	// Post acknowledgment comment
	body := fmt.Sprintf("🤖 Task received from %s. Starting execution...\n\nIssue ID: #%d\nContainer: `claude-worker-%d`", 
		source, *issue.Number, *issue.Number)
	
	comment := &github.IssueComment{
		Body: &body,
	}

	_, _, err := m.client.Issues.CreateComment(ctx, m.owner, m.repo, *issue.Number, comment)
	if err != nil {
		log.Printf("Failed to post acknowledgment: %v", err)
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