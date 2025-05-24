package kubernetes

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// PodManager manages worker pods for different repositories
type PodManager struct {
	clientset       *kubernetes.Clientset
	namespace       string
	workspacesDir   string
	sessionsDir     string
	repoMapping     *RepoMappingConfig
	activePods      map[string]*WorkerPod
	serviceAccount  string
}

// RepoMappingConfig represents the repository mapping configuration
type RepoMappingConfig struct {
	Repositories   map[string]*RepositoryConfig `yaml:"repositories"`
	Default        *RepositoryConfig            `yaml:"default"`
	ResourceLimits *ResourceLimits              `yaml:"resource_limits"`
	Security       *SecurityConfig              `yaml:"security"`
}

// RepositoryConfig defines configuration for a specific repository
type RepositoryConfig struct {
	Image     string            `yaml:"image"`
	Workspace string            `yaml:"workspace"`
	Env       []string          `yaml:"env,omitempty"`
	Ports     []int32           `yaml:"ports,omitempty"`
	Commands  map[string]string `yaml:"commands,omitempty"`
}

// ResourceLimits defines pod resource constraints
type ResourceLimits struct {
	Memory  string `yaml:"memory"`
	CPU     string `yaml:"cpu"`
	Timeout string `yaml:"timeout"`
}

// SecurityConfig defines pod security settings
type SecurityConfig struct {
	ReadOnlyRoot    bool     `yaml:"read_only_root"`
	NoPrivileged    bool     `yaml:"no_privileged"`
	User            *int64   `yaml:"user,omitempty"`
	Capabilities    CapConfig `yaml:"capabilities"`
}

type CapConfig struct {
	Drop []corev1.Capability `yaml:"drop"`
	Add  []corev1.Capability `yaml:"add"`
}

// WorkerPod represents an active worker pod
type WorkerPod struct {
	ID           string
	IssueNumber  int
	Repository   string
	PodName      string
	Config       *RepositoryConfig
	StartTime    time.Time
	WorkspaceDir string
	SessionFile  string
	Status       corev1.PodPhase
}

// NewPodManager creates a new pod manager instance
func NewPodManager(namespace, workspacesDir, sessionsDir string) (*PodManager, error) {
	// Try in-cluster config first, then fallback to kubeconfig
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig (for local development)
		kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	manager := &PodManager{
		clientset:      clientset,
		namespace:      namespace,
		workspacesDir:  workspacesDir,
		sessionsDir:    sessionsDir,
		activePods:     make(map[string]*WorkerPod),
		serviceAccount: "claude-worker",
	}

	// Verify connection and setup
	if err := manager.verifyConnection(); err != nil {
		return nil, fmt.Errorf("failed to verify kubernetes connection: %w", err)
	}

	return manager, nil
}

// verifyConnection verifies the connection to Kubernetes API
func (pm *PodManager) verifyConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test API connectivity
	_, err := pm.clientset.CoreV1().Namespaces().Get(ctx, pm.namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to access namespace %s: %w", pm.namespace, err)
	}

	log.Printf("Successfully connected to Kubernetes cluster, namespace: %s", pm.namespace)
	return nil
}

// SetupServiceAccount creates the ServiceAccount and RBAC for worker pods
func (pm *PodManager) SetupServiceAccount(ctx context.Context) error {
	// Create ServiceAccount
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pm.serviceAccount,
			Namespace: pm.namespace,
			Labels: map[string]string{
				"app":       "claude-automation",
				"component": "worker",
			},
		},
	}

	_, err := pm.clientset.CoreV1().ServiceAccounts(pm.namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
	if err != nil {
		// Check if already exists
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create service account: %w", err)
		}
		log.Printf("ServiceAccount %s already exists", pm.serviceAccount)
	} else {
		log.Printf("Created ServiceAccount: %s", pm.serviceAccount)
	}

	// Create Role
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "claude-worker-role",
			Namespace: pm.namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/log", "pods/exec"},
				Verbs:     []string{"get", "list", "create", "delete", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "create", "delete"},
			},
		},
	}

	_, err = pm.clientset.RbacV1().Roles(pm.namespace).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create role: %w", err)
		}
		log.Printf("Role claude-worker-role already exists")
	} else {
		log.Printf("Created Role: claude-worker-role")
	}

	// Create RoleBinding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "claude-worker-binding",
			Namespace: pm.namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      pm.serviceAccount,
				Namespace: pm.namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "claude-worker-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	_, err = pm.clientset.RbacV1().RoleBindings(pm.namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create role binding: %w", err)
		}
		log.Printf("RoleBinding claude-worker-binding already exists")
	} else {
		log.Printf("Created RoleBinding: claude-worker-binding")
	}

	return nil
}

// CreateWorkerPod creates a new worker pod for the given issue
func (pm *PodManager) CreateWorkerPod(ctx context.Context, issueNumber int, repository string, config *RepositoryConfig) (*WorkerPod, error) {
	podName := fmt.Sprintf("claude-worker-%d", issueNumber)
	
	// Check if pod already exists
	if existing, exists := pm.activePods[podName]; exists {
		log.Printf("Pod %s already exists for issue %d", podName, issueNumber)
		return existing, nil
	}

	// Create pod specification
	pod := pm.buildPodSpec(podName, issueNumber, repository, config)
	
	log.Printf("Creating worker pod: %s for issue %d", podName, issueNumber)

	// Create the pod
	createdPod, err := pm.clientset.CoreV1().Pods(pm.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	// Create workspace directory paths
	workspaceDir := filepath.Join(pm.workspacesDir, fmt.Sprintf("issue-%d", issueNumber))
	sessionFile := filepath.Join(pm.sessionsDir, fmt.Sprintf("issue-%d.session", issueNumber))

	// Create worker pod object
	worker := &WorkerPod{
		ID:           podName,
		IssueNumber:  issueNumber,
		Repository:   repository,
		PodName:      createdPod.Name,
		Config:       config,
		StartTime:    time.Now(),
		WorkspaceDir: workspaceDir,
		SessionFile:  sessionFile,
		Status:       createdPod.Status.Phase,
	}

	pm.activePods[podName] = worker
	
	log.Printf("Successfully created worker pod %s for issue %d", podName, issueNumber)
	return worker, nil
}

// buildPodSpec constructs the Pod specification for a worker pod
func (pm *PodManager) buildPodSpec(podName string, issueNumber int, repository string, config *RepositoryConfig) *corev1.Pod {
	// Create valid Kubernetes label for repository
	repoLabel := strings.ReplaceAll(repository, "/", "-")
	repoLabel = strings.ReplaceAll(repoLabel, "_", "-")
	
	labels := map[string]string{
		"app":         "claude-automation",
		"component":   "worker",
		"issue":       fmt.Sprintf("%d", issueNumber),
		"repository":  repoLabel,
	}

	// Environment variables
	env := []corev1.EnvVar{
		{Name: "REPOSITORY", Value: repository},
		{Name: "WORKSPACE", Value: config.Workspace},
		{Name: "ISSUE_NUMBER", Value: fmt.Sprintf("%d", issueNumber)},
	}

	// Add custom environment variables
	for _, envVar := range config.Env {
		name, value := parseEnvironmentVariable(envVar)
		env = append(env, corev1.EnvVar{Name: name, Value: value})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: pm.namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: pm.serviceAccount,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "claude-worker",
					Image:   config.Image,
					Env:     env,
					Command: []string{"sh", "-c", "while true; do sleep 30; done"}, // Keep running
					WorkingDir: config.Workspace,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "workspace",
							MountPath: config.Workspace,
						},
						{
							Name:      "claude-auth",
							MountPath: "/app/auth",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "workspace",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "claude-auth",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{}, // Use EmptyDir for testing
					},
				},
			},
		},
	}

	// Apply resource limits if specified
	if pm.repoMapping != nil && pm.repoMapping.ResourceLimits != nil {
		resources := corev1.ResourceRequirements{
			Limits:   corev1.ResourceList{},
			Requests: corev1.ResourceList{},
		}

		if pm.repoMapping.ResourceLimits.Memory != "" {
			// Parse and apply memory limit
			if memoryQuantity, err := resource.ParseQuantity(pm.repoMapping.ResourceLimits.Memory); err == nil {
				resources.Limits[corev1.ResourceMemory] = memoryQuantity
				resources.Requests[corev1.ResourceMemory] = memoryQuantity
			}
		}

		if pm.repoMapping.ResourceLimits.CPU != "" {
			// Parse and apply CPU limit
			if cpuQuantity, err := resource.ParseQuantity(pm.repoMapping.ResourceLimits.CPU); err == nil {
				resources.Limits[corev1.ResourceCPU] = cpuQuantity
				resources.Requests[corev1.ResourceCPU] = cpuQuantity
			}
		}

		pod.Spec.Containers[0].Resources = resources
	}

	// Apply security context if specified
	if pm.repoMapping != nil && pm.repoMapping.Security != nil {
		securityContext := &corev1.SecurityContext{}

		if pm.repoMapping.Security.ReadOnlyRoot {
			securityContext.ReadOnlyRootFilesystem = &pm.repoMapping.Security.ReadOnlyRoot
		}

		if !pm.repoMapping.Security.NoPrivileged {
			privileged := false
			securityContext.Privileged = &privileged
		}

		if pm.repoMapping.Security.User != nil {
			securityContext.RunAsUser = pm.repoMapping.Security.User
		}

		pod.Spec.Containers[0].SecurityContext = securityContext
	}

	return pod
}

// WaitForPodReady waits for a pod to become ready
func (pm *PodManager) WaitForPodReady(ctx context.Context, podName string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Printf("Waiting for pod %s to become ready...", podName)

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for pod %s to become ready", podName)
		default:
			pod, err := pm.clientset.CoreV1().Pods(pm.namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get pod status: %w", err)
			}

			if pod.Status.Phase == corev1.PodRunning {
				// Check if all containers are ready
				allReady := true
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
						allReady = false
						break
					}
				}

				if allReady {
					log.Printf("Pod %s is ready", podName)
					return nil
				}
			}

			time.Sleep(2 * time.Second)
		}
	}
}

// ExecuteInPod executes a command inside the worker pod
func (pm *PodManager) ExecuteInPod(ctx context.Context, podName, command string) (string, error) {
	// This would typically use the Kubernetes exec API
	// For now, we'll implement a simplified version
	log.Printf("Executing command in pod %s: %s", podName, command)
	
	// TODO: Implement proper kubectl exec equivalent using client-go
	// This is a placeholder that would need the proper streaming API implementation
	return "Command executed successfully", nil
}

// DeleteWorkerPod stops and removes a worker pod
func (pm *PodManager) DeleteWorkerPod(ctx context.Context, podName string) error {
	log.Printf("Deleting worker pod: %s", podName)
	
	// Delete the pod
	err := pm.clientset.CoreV1().Pods(pm.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	// Remove from active pods
	delete(pm.activePods, podName)
	
	log.Printf("Successfully deleted worker pod: %s", podName)
	return nil
}

// GetActivePods returns a list of currently active pods
func (pm *PodManager) GetActivePods() []*WorkerPod {
	pods := make([]*WorkerPod, 0, len(pm.activePods))
	for _, pod := range pm.activePods {
		pods = append(pods, pod)
	}
	return pods
}

// CleanupStalePods removes pods that have been running too long
func (pm *PodManager) CleanupStalePods(ctx context.Context, maxAge time.Duration) error {
	now := time.Now()
	var stalePods []string
	
	for id, pod := range pm.activePods {
		if now.Sub(pod.StartTime) > maxAge {
			stalePods = append(stalePods, id)
		}
	}
	
	for _, id := range stalePods {
		if err := pm.DeleteWorkerPod(ctx, id); err != nil {
			log.Printf("Failed to cleanup stale pod %s: %v", id, err)
		}
	}
	
	return nil
}

// GetPodLogs retrieves logs from a worker pod
func (pm *PodManager) GetPodLogs(ctx context.Context, podName string) (string, error) {
	req := pm.clientset.CoreV1().Pods(pm.namespace).GetLogs(podName, &corev1.PodLogOptions{})
	
	logs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer logs.Close()

	// Read all logs
	logBytes := make([]byte, 0)
	buffer := make([]byte, 1024)
	
	for {
		n, err := logs.Read(buffer)
		if n > 0 {
			logBytes = append(logBytes, buffer[:n]...)
		}
		if err != nil {
			break
		}
	}

	return string(logBytes), nil
}

// Helper functions

func parseEnvironmentVariable(envVar string) (name, value string) {
	// Parse environment variable in format "NAME=value"
	parts := strings.SplitN(envVar, "=", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return envVar, ""
}