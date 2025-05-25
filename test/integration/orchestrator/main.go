package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/claude-automation/pkg/kubernetes"
)

func main() {
	fmt.Println("🧪 Issue #13 Orchestrator Kubernetes Integration Test")
	
	// Initialize Pod Manager like in Orchestrator
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "claude-automation"
	}
	
	podManager, err := kubernetes.NewPodManager(namespace, "/tmp/workspaces", "/tmp/sessions")
	if err != nil {
		log.Fatalf("Failed to create pod manager: %v", err)
	}
	
	ctx := context.Background()
	issueNumber := 13
	repository := "worldscandy/claude-automation"
	
	// Test Pod creation like in Orchestrator
	fmt.Println("\n📋 Test: Dynamic Pod Creation")
	config := &kubernetes.RepositoryConfig{
		Image:     "worldscandy/claude-automation:k8s", // Use actual Claude CLI image
		Workspace: "/workspace",
		Env:       []string{"NODE_ENV=development", "TEST_MODE=true"},
	}
	
	workerPod, err := podManager.CreateWorkerPod(ctx, issueNumber, repository, config)
	if err != nil {
		log.Fatalf("❌ Failed to create worker pod: %v", err)
	}
	
	fmt.Printf("✅ Worker pod created successfully: %s\n", workerPod.ID)
	
	// Wait for pod to be ready
	fmt.Println("\n📋 Test: Wait for Pod Ready")
	err = podManager.WaitForPodReady(ctx, workerPod.PodName, 60000000000) // 60 seconds
	if err != nil {
		log.Printf("❌ Pod failed to become ready: %v", err)
	} else {
		fmt.Printf("✅ Pod %s is ready!\n", workerPod.PodName)
	}
	
	// Test Claude CLI-like command execution
	fmt.Println("\n📋 Test: Claude CLI Simulation")
	claudeCmd := "echo 'Simulating Claude CLI execution...' && echo 'Working directory: /workspace' && ls -la /workspace || echo 'Workspace not accessible'"
	output, err := podManager.ExecuteInPod(ctx, workerPod.PodName, claudeCmd)
	if err != nil {
		log.Printf("❌ Claude CLI simulation failed: %v", err)
	} else {
		fmt.Printf("✅ Claude CLI simulation successful:\n%s\n", output)
	}
	
	// Test Environment Debug
	fmt.Println("\n📋 Test: Environment Debug")
	debugCmd := "echo 'PATH:' && echo $PATH && echo 'Node Version:' && node --version 2>&1 || echo 'No node' && echo 'NPM Global Bin:' && npm bin -g 2>&1 || echo 'No npm' && echo 'Local Claude Files:' && ls -la /usr/local/bin/claude* 2>&1 || echo 'No claude files' && echo 'Image Info:' && cat /etc/os-release | head -3"
	output, err = podManager.ExecuteInPod(ctx, workerPod.PodName, debugCmd)
	if err != nil {
		log.Printf("❌ Environment debug failed: %v", err)
	} else {
		fmt.Printf("✅ Environment debug successful:\n%s\n", output)
	}
	
	// Test Real Claude CLI execution with full path
	fmt.Println("\n📋 Test: Real Claude CLI Execution")
	realClaudeCmd := "/usr/local/bin/claude --version && echo '--- Claude CLI Available ---'"
	output, err = podManager.ExecuteInPod(ctx, workerPod.PodName, realClaudeCmd)
	if err != nil {
		log.Printf("❌ Real Claude CLI execution failed: %v", err)
	} else {
		fmt.Printf("✅ Real Claude CLI execution successful:\n%s\n", output)
	}
	
	// Test Authentication Setup
	fmt.Println("\n📋 Test: Authentication Setup")
	authSetupCmd := "echo 'Setting up Claude CLI authentication...' && ls -la /app/auth/ && mkdir -p $HOME/.claude && cp /app/auth/.claude.json $HOME/ 2>/dev/null && cp /app/auth/.claude/.credentials.json $HOME/.claude/ 2>/dev/null && ls -la $HOME/.claude* 2>/dev/null || echo 'Auth files not found, checking alternative paths'"
	output, err = podManager.ExecuteInPod(ctx, workerPod.PodName, authSetupCmd)
	if err != nil {
		log.Printf("❌ Authentication setup failed: %v", err)
	} else {
		fmt.Printf("✅ Authentication setup output:\n%s\n", output)
	}
	
	// Test Real Claude CLI Task Execution
	fmt.Println("\n📋 Test: Real Claude CLI Task Execution")
	taskCmd := "cd /workspace && echo 'console.log(\"Hello from Claude CLI!\");' > hello.js && claude --print 'What is in hello.js file and can you run it?' 2>&1 | head -20"
	output, err = podManager.ExecuteInPod(ctx, workerPod.PodName, taskCmd)
	if err != nil {
		log.Printf("❌ Real Claude task execution failed: %v", err)
	} else {
		fmt.Printf("✅ Real Claude task execution output:\n%s\n", output)
	}
	
	// Test Advanced Features
	fmt.Println("\n📋 Test: Advanced Claude CLI Features")
	advancedCmd := "claude --help | grep -E '(--max-turns|--verbose|--continue)' || echo 'Advanced features check'"
	output, err = podManager.ExecuteInPod(ctx, workerPod.PodName, advancedCmd)
	if err != nil {
		log.Printf("❌ Advanced features check failed: %v", err)
	} else {
		fmt.Printf("✅ Advanced features available:\n%s\n", output)
	}
	
	// Cleanup
	fmt.Println("\n📋 Test: Pod Cleanup")
	err = podManager.DeleteWorkerPod(ctx, workerPod.PodName)
	if err != nil {
		log.Printf("❌ Failed to cleanup pod: %v", err)
	} else {
		fmt.Printf("✅ Pod %s cleaned up successfully\n", workerPod.PodName)
	}
	
	fmt.Println("\n🎉 Orchestrator Kubernetes Integration tests completed!")
}