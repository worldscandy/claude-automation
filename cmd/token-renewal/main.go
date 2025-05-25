package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
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
	// Check if we're in an interactive terminal
	isInteractive := isTerminalInteractive()
	
	// Generate unique container name
	containerName := fmt.Sprintf("claude-token-renewal-%d", time.Now().Unix())

	var args []string
	if isInteractive {
		// Run interactive container
		args = []string{"run", "--rm", "-it", "--name", containerName, "claude-automation-token-renewal"}
	} else {
		// Run in non-interactive mode with helpful instructions
		fmt.Println("⚠️  Non-interactive terminal detected.")
		fmt.Println("   Starting container in background mode...")
		fmt.Println()
		fmt.Printf("🔗 To access the container interactively, run:\n")
		fmt.Printf("   docker exec -it %s bash\n", containerName)
		fmt.Println()
		fmt.Println("📋 Inside the container:")
		fmt.Println("   1. Run: claude login")
		fmt.Println("   2. Run: /app/scripts/token-renewal.sh")
		fmt.Println("   3. Copy the output to your .env-secret file")
		fmt.Println("   4. Exit with: exit")
		fmt.Println()
		fmt.Printf("🛑 To stop the container: docker stop %s\n", containerName)
		fmt.Println()
		
		args = []string{"run", "--rm", "-d", "--name", containerName, "claude-automation-token-renewal", "sleep", "3600"}
	}

	// Run container
	cmd := exec.Command("docker", args...)
	if isInteractive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else {
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to start container: %w\nOutput: %s", err, string(output))
		}
		fmt.Printf("✅ Container %s started successfully\n", containerName)
		fmt.Printf("   Container ID: %s\n", string(output)[:12])
		return nil
	}
}

func isTerminalInteractive() bool {
	// Check if stdin is a terminal
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
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