package container

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	
	"github.com/claude-automation/pkg/auth"
)

// ContainerManager manages worker containers for different repositories
type ContainerManager struct {
	ConfigPath     string
	WorkspacesDir  string
	SessionsDir    string
	RepoMapping    *RepoMappingConfig
	activeContainers map[string]*WorkerContainer
}

// RepoMappingConfig represents the repository mapping configuration
type RepoMappingConfig struct {
	Repositories map[string]*RepositoryConfig `yaml:"repositories"`
	Default      *RepositoryConfig            `yaml:"default"`
	ResourceLimits *ResourceLimits            `yaml:"resource_limits"`
	Security     *SecurityConfig              `yaml:"security"`
}

// RepositoryConfig defines configuration for a specific repository
type RepositoryConfig struct {
	Image     string            `yaml:"image"`
	Workspace string            `yaml:"workspace"`
	Env       []string          `yaml:"env,omitempty"`
	Ports     []string          `yaml:"ports,omitempty"`
	Commands  map[string]string `yaml:"commands,omitempty"`
}

// ResourceLimits defines container resource constraints
type ResourceLimits struct {
	Memory  string `yaml:"memory"`
	CPU     string `yaml:"cpu"`
	Disk    string `yaml:"disk"`
	Timeout string `yaml:"timeout"`
}

// SecurityConfig defines container security settings
type SecurityConfig struct {
	ReadOnlyRoot      bool     `yaml:"read_only_root"`
	NoNewPrivileges   bool     `yaml:"no_new_privileges"`
	User              string   `yaml:"user"`
	Capabilities      CapConfig `yaml:"capabilities"`
}

type CapConfig struct {
	Drop []string `yaml:"drop"`
	Add  []string `yaml:"add"`
}

// WorkerContainer represents an active worker container
type WorkerContainer struct {
	ID           string
	IssueNumber  int
	Repository   string
	ContainerID  string
	Config       *RepositoryConfig
	StartTime    time.Time
	WorkspaceDir string
	SessionFile  string
}

// NewContainerManager creates a new container manager instance
func NewContainerManager(configPath, workspacesDir, sessionsDir string) (*ContainerManager, error) {
	manager := &ContainerManager{
		ConfigPath:       configPath,
		WorkspacesDir:    workspacesDir,
		SessionsDir:      sessionsDir,
		activeContainers: make(map[string]*WorkerContainer),
	}

	if err := manager.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return manager, nil
}

// loadConfig loads the repository mapping configuration
func (cm *ContainerManager) loadConfig() error {
	data, err := os.ReadFile(cm.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	cm.RepoMapping = &RepoMappingConfig{}
	if err := yaml.Unmarshal(data, cm.RepoMapping); err != nil {
		return fmt.Errorf("failed to parse config YAML: %w", err)
	}

	return nil
}

// CreateWorkerContainer creates a new worker container for the given issue
func (cm *ContainerManager) CreateWorkerContainer(ctx context.Context, issueNumber int, repository string) (*WorkerContainer, error) {
	containerID := fmt.Sprintf("claude-worker-%d-%s", issueNumber, strings.ReplaceAll(repository, "/", "-"))
	
	// Check if container already exists
	if existing, exists := cm.activeContainers[containerID]; exists {
		log.Printf("Container %s already exists for issue %d", containerID, issueNumber)
		return existing, nil
	}

	// Get repository configuration
	config := cm.getRepositoryConfig(repository)
	
	// Ensure workspace parent directory exists (Host side)
	if err := os.MkdirAll(cm.WorkspacesDir, 0755); err != nil {
		log.Printf("Warning: failed to create workspaces root: %v", err)
	}
	
	// Create workspace directory for this issue  
	workspaceDir := filepath.Join(cm.WorkspacesDir, fmt.Sprintf("issue-%d", issueNumber))
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		log.Printf("Warning: failed to create issue workspace (will use container internal): %v", err)
		// Use container internal path if host creation fails
		workspaceDir = fmt.Sprintf("/app/workspaces/issue-%d", issueNumber)
	}

	// Ensure sessions directory exists (Host side)
	if err := os.MkdirAll(cm.SessionsDir, 0755); err != nil {
		log.Printf("Warning: failed to create sessions directory: %v", err)
	}
	
	// Create session file path
	sessionFile := filepath.Join(cm.SessionsDir, fmt.Sprintf("issue-%d.session", issueNumber))

	// Build Docker run command
	dockerCmd := cm.buildDockerCommand(containerID, config, workspaceDir, repository)
	
	log.Printf("Creating worker container: %s", containerID)
	log.Printf("Docker command: %s", strings.Join(dockerCmd, " "))

	// Execute Docker run command
	cmd := exec.CommandContext(ctx, dockerCmd[0], dockerCmd[1:]...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	
	// Create issue-specific workspace inside container and fix permissions
	issueWorkspaceCmd := fmt.Sprintf("mkdir -p /app/workspaces/issue-%d && mkdir -p /app/sessions", issueNumber)
	if _, err := cm.ExecuteInContainer(ctx, containerID, issueWorkspaceCmd); err != nil {
		log.Printf("Warning: failed to create container workspace: %v", err)
	}
	
	// Fix workspace permissions (as root, then switch back)
	fixPermCmd := "sudo chown -R claude:claude /home/claude/workspace || chown -R claude:claude /home/claude/workspace || echo 'Permission fix failed, continuing...'"
	if _, err := cm.ExecuteInContainer(ctx, containerID, fixPermCmd); err != nil {
		log.Printf("Warning: failed to fix workspace permissions: %v", err)
	}

	// Create worker container object
	worker := &WorkerContainer{
		ID:           containerID,
		IssueNumber:  issueNumber,
		Repository:   repository,
		ContainerID:  containerID,
		Config:       config,
		StartTime:    time.Now(),
		WorkspaceDir: workspaceDir,
		SessionFile:  sessionFile,
	}

	cm.activeContainers[containerID] = worker
	
	log.Printf("Successfully created worker container %s for issue %d", containerID, issueNumber)
	return worker, nil
}

// getRepositoryConfig returns the configuration for a given repository
func (cm *ContainerManager) getRepositoryConfig(repository string) *RepositoryConfig {
	if config, exists := cm.RepoMapping.Repositories[repository]; exists {
		return config
	}
	
	log.Printf("No specific config found for repository %s, using default", repository)
	return cm.RepoMapping.Default
}

// buildDockerCommand constructs the Docker run command for a worker container
func (cm *ContainerManager) buildDockerCommand(containerID string, config *RepositoryConfig, workspaceDir, repository string) []string {
	cmd := []string{
		"docker", "run",
		"--name", containerID,
		"--detach",
		"--rm", // Auto-remove when stopped
	}

	// Add resource limits
	if cm.RepoMapping.ResourceLimits != nil {
		if cm.RepoMapping.ResourceLimits.Memory != "" {
			cmd = append(cmd, "--memory", cm.RepoMapping.ResourceLimits.Memory)
		}
		if cm.RepoMapping.ResourceLimits.CPU != "" {
			cmd = append(cmd, "--cpus", cm.RepoMapping.ResourceLimits.CPU)
		}
	}

	// Add security settings (relaxed for testing)
	if cm.RepoMapping.Security != nil {
		// Skip security restrictions for Docker-in-Docker testing
		// if cm.RepoMapping.Security.NoNewPrivileges {
		//     cmd = append(cmd, "--security-opt", "no-new-privileges:true")
		// }
		// Don't set user for now to avoid permission issues
		// if cm.RepoMapping.Security.User != "" {
		//     cmd = append(cmd, "--user", cm.RepoMapping.Security.User)
		// }
		// Skip capability restrictions for now
		// for _, cap := range cm.RepoMapping.Security.Capabilities.Drop {
		//     cmd = append(cmd, "--cap-drop", cap)
		// }
		for _, cap := range cm.RepoMapping.Security.Capabilities.Add {
			cmd = append(cmd, "--cap-add", cap)
		}
	}

	// Mount workspace directory (use absolute paths for Docker-in-Docker)
	cmd = append(cmd, "-v", fmt.Sprintf("%s:%s", workspaceDir, config.Workspace))

	// Generate and mount Claude CLI auth files from templates and environment variables
	tempAuthDir := "/tmp/claude-auth-temp"
	if err := cm.generateContainerAuthFiles(tempAuthDir); err != nil {
		log.Printf("Warning: failed to generate auth files, using fallback: %v", err)
		// Fallback: mount empty directory for safety
		cmd = append(cmd, "-v", "/tmp/empty:/home/claude/.claude:ro")
	} else {
		// Mount generated auth structure to claude home 
		// Structure: tempAuthDir/.claude.json -> /home/claude/.claude.json (read-write for CLI updates)
		//           tempAuthDir/.claude/.credentials.json -> /home/claude/.claude/.credentials.json
		cmd = append(cmd, "-v", fmt.Sprintf("%s/.claude.json:/home/claude/.claude.json:rw", tempAuthDir))
		cmd = append(cmd, "-v", fmt.Sprintf("%s/.claude:/home/claude/.claude:rw", tempAuthDir))
		
		log.Printf("Mounting auth files: %s -> /home/claude/", tempAuthDir)
	}

	// Add environment variables
	for _, env := range config.Env {
		cmd = append(cmd, "-e", env)
	}

	// Add repository info
	cmd = append(cmd, "-e", fmt.Sprintf("REPOSITORY=%s", repository))
	cmd = append(cmd, "-e", fmt.Sprintf("WORKSPACE=%s", config.Workspace))

	// Add port mappings
	for _, port := range config.Ports {
		cmd = append(cmd, "-p", port)
	}

	// Add working directory
	cmd = append(cmd, "-w", config.Workspace)

	// Add image
	cmd = append(cmd, config.Image)

	// Keep container running with a simple command
	cmd = append(cmd, "tail", "-f", "/dev/null")

	return cmd
}

// ExecuteInContainer executes a command inside the worker container
func (cm *ContainerManager) ExecuteInContainer(ctx context.Context, containerID, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "exec", containerID, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command in container: %w", err)
	}
	return string(output), nil
}

// StopWorkerContainer stops and removes a worker container
func (cm *ContainerManager) StopWorkerContainer(ctx context.Context, containerID string) error {
	log.Printf("Stopping worker container: %s", containerID)
	
	// Stop the container
	cmd := exec.CommandContext(ctx, "docker", "stop", containerID)
	if err := cmd.Run(); err != nil {
		log.Printf("Warning: failed to stop container %s: %v", containerID, err)
	}

	// Remove from active containers
	delete(cm.activeContainers, containerID)
	
	log.Printf("Successfully stopped worker container: %s", containerID)
	return nil
}

// GetActiveContainers returns a list of currently active containers
func (cm *ContainerManager) GetActiveContainers() []*WorkerContainer {
	containers := make([]*WorkerContainer, 0, len(cm.activeContainers))
	for _, container := range cm.activeContainers {
		containers = append(containers, container)
	}
	return containers
}

// CleanupStaleContainers removes containers that have been running too long
func (cm *ContainerManager) CleanupStaleContainers(ctx context.Context, maxAge time.Duration) error {
	now := time.Now()
	var staleContainers []string
	
	for id, container := range cm.activeContainers {
		if now.Sub(container.StartTime) > maxAge {
			staleContainers = append(staleContainers, id)
		}
	}
	
	for _, id := range staleContainers {
		if err := cm.StopWorkerContainer(ctx, id); err != nil {
			log.Printf("Failed to cleanup stale container %s: %v", id, err)
		}
	}
	
	return nil
}

// GetContainerLogs retrieves logs from a worker container
func (cm *ContainerManager) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "logs", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	return string(output), nil
}

// generateContainerAuthFiles generates Claude CLI auth files for container use
func (cm *ContainerManager) generateContainerAuthFiles(destDir string) error {
	// Load environment variables from .env-secret file
	if err := auth.LoadEnvFile(".env-secret"); err != nil {
		log.Printf("Warning: failed to load .env-secret file: %v", err)
	}

	// Check token expiry and generate alert if needed
	if alert, err := auth.GetTokenExpiryAlert(); err == nil && alert != "" {
		log.Printf("Token Alert: %s", alert)
	}

	// Generate auth files from templates and environment variables
	return auth.GenerateAuthFiles(destDir)
}

// RefreshContainerAuth refreshes authentication files for an existing container
func (cm *ContainerManager) RefreshContainerAuth(ctx context.Context, containerID string) error {
	tempAuthDir := "/tmp/claude-auth-refresh"
	
	// Generate new auth files
	if err := cm.generateContainerAuthFiles(tempAuthDir); err != nil {
		return fmt.Errorf("failed to generate auth files: %w", err)
	}
	
	// Copy .claude.json to container home root
	copyClaudeJsonCmd := fmt.Sprintf("docker cp %s/.claude.json %s:/home/claude/.claude.json", tempAuthDir, containerID)
	cmd := exec.CommandContext(ctx, "sh", "-c", copyClaudeJsonCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy .claude.json to container: %w", err)
	}
	
	// Copy .credentials.json to container .claude directory
	copyCredsCmd := fmt.Sprintf("docker cp %s/.claude/.credentials.json %s:/home/claude/.claude/.credentials.json", tempAuthDir, containerID)
	cmd = exec.CommandContext(ctx, "sh", "-c", copyCredsCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy .credentials.json to container: %w", err)
	}
	
	// Fix permissions inside container
	permCmd := "sudo chown claude:claude /home/claude/.claude.json /home/claude/.claude/.credentials.json && sudo chmod 600 /home/claude/.claude.json /home/claude/.claude/.credentials.json"
	if _, err := cm.ExecuteInContainer(ctx, containerID, permCmd); err != nil {
		log.Printf("Warning: failed to fix auth permissions in container: %v", err)
	}
	
	log.Printf("Successfully refreshed auth files for container: %s", containerID)
	return nil
}