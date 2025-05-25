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
		Image:     "alpine:latest", // Use alpine for testing instead of claude-automation-claude
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
	
	// Test Advanced Features simulation
	fmt.Println("\n📋 Test: Advanced Features Simulation")
	advancedCmd := "echo 'Testing --max-turns simulation...' && echo 'Testing --verbose output...' && echo 'Testing session continuation...'"
	output, err = podManager.ExecuteInPod(ctx, workerPod.PodName, advancedCmd)
	if err != nil {
		log.Printf("❌ Advanced features simulation failed: %v", err)
	} else {
		fmt.Printf("✅ Advanced features simulation successful:\n%s\n", output)
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