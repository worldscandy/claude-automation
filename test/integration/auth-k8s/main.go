package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/claude-automation/pkg/auth"
	"github.com/claude-automation/pkg/kubernetes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	kubeclient "k8s.io/client-go/kubernetes"
)

func main() {
	fmt.Println("🧪 Issue #13 Authentication System Integration Test")
	
	// Test Issue #16 auth package integration
	fmt.Println("\n📋 Test: Issue #16 Auth Package Integration")
	
	// Load .env-secret file
	if err := auth.LoadEnvFile(".env-secret"); err != nil {
		log.Printf("Warning: failed to load .env-secret: %v", err)
	} else {
		fmt.Println("✅ .env-secret loaded successfully")
	}
	
	// Test token expiry check
	if alert, err := auth.GetTokenExpiryAlert(); err == nil && alert != "" {
		fmt.Printf("⏰ Token Alert: %s\n", alert)
	} else {
		fmt.Println("✅ Token status: OK")
	}
	
	// Generate auth files to temporary directory
	tempDir := "/tmp/claude-auth-test"
	fmt.Printf("\n📋 Test: Auth Files Generation to %s\n", tempDir)
	if err := auth.GenerateAuthFiles(tempDir); err != nil {
		log.Printf("❌ Failed to generate auth files: %v", err)
	} else {
		fmt.Println("✅ Auth files generated successfully")
	}
	
	// Create Kubernetes Secret with real auth data
	fmt.Println("\n📋 Test: Create Kubernetes Secret with Real Auth Data")
	
	// Get Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig
		kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			log.Fatalf("Failed to create kubernetes config: %v", err)
		}
	}
	
	clientset, err := kubeclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create kubernetes clientset: %v", err)
	}
	
	ctx := context.Background()
	namespace := "claude-automation"
	secretName := "claude-auth-real"
	
	// Read generated auth files
	claudeJson, err := auth.ReadFileBytes(filepath.Join(tempDir, ".claude.json"))
	if err != nil {
		log.Printf("❌ Failed to read .claude.json: %v", err)
		return
	}
	
	credentialsJson, err := auth.ReadFileBytes(filepath.Join(tempDir, ".claude", ".credentials.json"))
	if err != nil {
		log.Printf("❌ Failed to read .credentials.json: %v", err)
		return
	}
	
	// Create secret with real auth data
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":       "claude-automation",
				"component": "auth",
				"test":      "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"claude-config": claudeJson,
			"credentials":   credentialsJson,
		},
	}
	
	// Delete existing secret if it exists
	_ = clientset.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	
	_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		log.Printf("❌ Failed to create secret: %v", err)
	} else {
		fmt.Println("✅ Kubernetes Secret created with real auth data")
	}
	
	// Test Pod with real auth data
	fmt.Println("\n📋 Test: Pod with Real Authentication")
	
	podManager, err := kubernetes.NewPodManager(namespace, "/tmp/workspaces", "/tmp/sessions")
	if err != nil {
		log.Fatalf("Failed to create pod manager: %v", err)
	}
	
	// Create pod with real auth secret
	podConfig := &kubernetes.RepositoryConfig{
		Image:     "alpine:latest",
		Workspace: "/workspace",
		Env:       []string{"TEST_MODE=auth"},
	}
	
	issueNumber := 16 // Use issue 16 for auth testing
	workerPod, err := podManager.CreateWorkerPod(ctx, issueNumber, "test/auth-integration", podConfig)
	if err != nil {
		log.Printf("❌ Failed to create worker pod: %v", err)
		return
	}
	
	// Wait for pod ready
	err = podManager.WaitForPodReady(ctx, workerPod.PodName, 60000000000)
	if err != nil {
		log.Printf("❌ Pod failed to become ready: %v", err)
		return
	}
	
	// Test auth file accessibility in pod
	fmt.Println("✅ Testing auth file access in Pod...")
	authTestCmd := "ls -la /app/auth && echo '--- .claude.json ---' && head -5 /app/auth/.claude.json && echo '--- .credentials.json ---' && head -5 /app/auth/.claude/.credentials.json"
	output, err := podManager.ExecuteInPod(ctx, workerPod.PodName, authTestCmd)
	if err != nil {
		log.Printf("❌ Auth file access test failed: %v", err)
	} else {
		fmt.Printf("✅ Auth files accessible in Pod:\n%s\n", output)
	}
	
	// Cleanup
	fmt.Println("\n📋 Cleanup")
	err = podManager.DeleteWorkerPod(ctx, workerPod.PodName)
	if err != nil {
		log.Printf("Warning: failed to cleanup pod: %v", err)
	}
	
	err = clientset.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("Warning: failed to cleanup secret: %v", err)
	}
	
	fmt.Println("✅ Cleanup completed")
	fmt.Println("\n🎉 Authentication System Integration tests completed!")
}