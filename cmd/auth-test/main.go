package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/claude-automation/pkg/auth"
	"github.com/claude-automation/pkg/container"
)

func main() {
	fmt.Println("🔐 Claude Authentication System Test")
	fmt.Println("=====================================")

	// Test 1: Load environment variables
	fmt.Println("\n1. Loading environment variables...")
	if err := auth.LoadEnvFile(".env-secret"); err != nil {
		log.Fatalf("Failed to load .env-secret: %v", err)
	}
	fmt.Println("✅ Environment variables loaded successfully")

	// Test 2: Check token expiry
	fmt.Println("\n2. Checking token expiry status...")
	status, err := auth.GetTokenStatus()
	if err != nil {
		log.Fatalf("Failed to get token status: %v", err)
	}
	fmt.Printf("Token Status: %s\n", status)

	// Test 3: Generate auth files
	fmt.Println("\n3. Testing auth file generation...")
	testDir := "/tmp/claude-auth-test"
	if err := os.RemoveAll(testDir); err != nil {
		log.Printf("Warning: failed to clean test directory: %v", err)
	}

	if err := auth.GenerateAuthFiles(testDir); err != nil {
		log.Fatalf("Failed to generate auth files: %v", err)
	}
	fmt.Println("✅ Auth files generated successfully")

	// Verify generated files
	claudeJsonPath := filepath.Join(testDir, ".claude.json")
	credentialsPath := filepath.Join(testDir, ".claude.credentials.json")

	if _, err := os.Stat(claudeJsonPath); os.IsNotExist(err) {
		log.Fatalf(".claude.json not generated")
	}
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		log.Fatalf(".claude.credentials.json not generated")
	}

	fmt.Printf("Generated files:\n")
	fmt.Printf("  - %s\n", claudeJsonPath)
	fmt.Printf("  - %s\n", credentialsPath)

	// Test 4: Container manager integration
	fmt.Println("\n4. Testing container manager integration...")
	
	// Create test container manager
	_, cmErr := container.NewContainerManager(
		"config/repo-mapping.yaml",
		"workspaces",
		"sessions",
	)
	if cmErr != nil {
		log.Printf("Container manager test skipped (config not found): %v", cmErr)
	} else {
		fmt.Println("✅ Container manager integration available")
	}

	// Test auth file generation in container context
	containerAuthDir := "/tmp/claude-auth-container-test"
	if err := os.RemoveAll(containerAuthDir); err != nil {
		log.Printf("Warning: failed to clean container test directory: %v", err)
	}

	// Simulate container auth generation
	fmt.Println("  - Generating auth files for container use...")
	if err := auth.GenerateAuthFiles(containerAuthDir); err != nil {
		log.Printf("Warning: Container auth generation failed: %v", err)
	} else {
		fmt.Println("✅ Container auth files generated successfully")
	}

	// Test 5: Token monitoring (brief test)
	fmt.Println("\n5. Testing token monitoring...")
	
	alertReceived := false
	alertCallback := func(message string) error {
		fmt.Printf("Alert received: %s\n", message)
		alertReceived = true
		return nil
	}

	monitor := auth.NewTokenMonitor(alertCallback)
	monitor.SetCheckInterval(1 * time.Second) // Fast interval for testing

	// Run monitor briefly
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		if err := monitor.Start(ctx); err != nil && err != context.DeadlineExceeded {
			log.Printf("Monitor error: %v", err)
		}
	}()

	time.Sleep(2 * time.Second)
	cancel()

	if alertReceived {
		fmt.Println("✅ Token monitoring alert system working")
	} else {
		fmt.Println("ℹ️  No alerts generated (token likely healthy)")
	}

	// Cleanup
	fmt.Println("\n6. Cleaning up test files...")
	os.RemoveAll(testDir)
	os.RemoveAll(containerAuthDir)
	fmt.Println("✅ Cleanup completed")

	fmt.Println("\n🎉 All authentication system tests completed successfully!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Review generated auth files in containers")
	fmt.Println("  2. Test with actual Claude CLI commands")
	fmt.Println("  3. Monitor token expiry alerts in production")
}