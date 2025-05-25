package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fmt.Println("🔐 Claude CLI Token Renewal Container Manager")
	fmt.Println("===========================================")

	// Check if Docker is available
	if !isDockerAvailable() {
		log.Fatal("❌ Docker is not available or not running")
	}

	// Build token renewal container if needed
	fmt.Println("🏗️  Building token renewal container...")
	if err := buildTokenRenewalContainer(); err != nil {
		log.Fatalf("❌ Failed to build container: %v", err)
	}

	fmt.Println("✅ Token renewal container ready")
	fmt.Println()

	// Display instructions
	displayInstructions()

	// Start interactive container
	fmt.Println("🚀 Starting Claude CLI token renewal container...")
	fmt.Println("   (Press Ctrl+C to exit when done)")
	fmt.Println()

	if err := runTokenRenewalContainer(); err != nil {
		log.Fatalf("❌ Failed to run container: %v", err)
	}

	fmt.Println()
	fmt.Println("🎉 Token renewal session completed!")
	fmt.Println("   Remember to update your .env-secret file with the new token values.")
}

func isDockerAvailable() bool {
	cmd := exec.Command("docker", "--version")
	return cmd.Run() == nil
}

func buildTokenRenewalContainer() error {
	cmd := exec.Command("docker", "build", "-t", "claude-automation-token-renewal", "-f", "Dockerfile.token-renewal", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runTokenRenewalContainer() error {
	// Create context that can be cancelled with Ctrl+C
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\n🛑 Interrupt received, stopping container...")
		cancel()
	}()

	// Generate unique container name
	containerName := fmt.Sprintf("claude-token-renewal-%d", time.Now().Unix())

	// Run interactive container
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-it", "--name", containerName, "claude-automation-token-renewal")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func displayInstructions() {
	fmt.Println("📋 Token Renewal Instructions:")
	fmt.Println("   1. The container will start with Claude CLI installed but not authenticated")
	fmt.Println("   2. Run 'claude login' inside the container")
	fmt.Println("   3. Follow the browser authentication flow")
	fmt.Println("   4. Run '/app/scripts/token-renewal.sh' to extract token values")
	fmt.Println("   5. Copy the generated configuration to your host .env-secret file")
	fmt.Println("   6. Exit the container with 'exit' or Ctrl+D")
	fmt.Println()
}