package k8s

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultNamespace    = "aeneas"
	jobNamePrefix       = "aeneas-"
	maxJobNameLength    = 63
	jobCleanupRetention = 5 * time.Minute
)

// K8sRunner executes workflow steps using Kubernetes Jobs
type K8sRunner struct {
	clientset kubernetes.Interface
	namespace string
	logger    *logrus.Logger
	cleanup   bool
}

// Config holds configuration options for K8sRunner
type Config struct {
	// Kubeconfig path (empty for in-cluster config)
	Kubeconfig string
	// Namespace to run jobs in (defaults to "aeneas" or current namespace in-cluster)
	Namespace string
	// Logger instance
	Logger *logrus.Logger
	// Cleanup controls whether to delete jobs after completion (default: true)
	Cleanup bool
}

// NewK8sRunner creates a new K8sRunner instance
func NewK8sRunner(cfg Config) (*K8sRunner, error) {
	var config *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		// Out-of-cluster config
		config, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	} else {
		// In-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load in-cluster config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}

	cleanup := cfg.Cleanup
	if !cfg.Cleanup && cfg.Kubeconfig == "" {
		// Default to cleanup enabled
		cleanup = true
	}

	logger := cfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	return &K8sRunner{
		clientset: clientset,
		namespace: namespace,
		logger:    logger,
		cleanup:   cleanup,
	}, nil
}

// Execute runs a step as a Kubernetes Job
//
//nolint:gocritic // Interface signature defined by Runner contract
func (r *K8sRunner) Execute(ctx context.Context, step domain.StepDefinition) (*runner.Result, error) {
	// Apply timeout if specified
	if step.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Create the Job
	job, err := r.createJob(ctx, &step)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	jobName := job.Name
	r.logger.WithFields(logrus.Fields{
		"job":  jobName,
		"step": step.Name,
	}).Info("Created Kubernetes Job")

	// Ensure cleanup if configured
	if r.cleanup {
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := r.cleanupJob(cleanupCtx, jobName); err != nil {
				r.logger.WithError(err).WithField("job", jobName).Warn("Failed to cleanup job")
			}
		}()
	}

	// Wait for Job to complete
	pod, err := r.waitForJobCompletion(ctx, jobName)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for job completion: %w", err)
	}

	// Capture logs
	logs, err := r.capturePodLogs(ctx, pod.Name)
	if err != nil {
		r.logger.WithError(err).Warn("Failed to capture pod logs")
	}

	// Extract exit code from container status
	exitCode := r.getExitCode(pod)

	return &runner.Result{
		ExitCode: exitCode,
		Logs:     logs,
	}, nil
}

// createJob creates a Kubernetes Job from a StepDefinition
func (r *K8sRunner) createJob(ctx context.Context, step *domain.StepDefinition) (*batchv1.Job, error) {
	jobName := r.generateJobName(step.Name)

	// Build environment variables
	env := make([]corev1.EnvVar, 0, len(step.Env))
	for k, v := range step.Env {
		env = append(env, corev1.EnvVar{Name: k, Value: v})
	}

	// Build command and args
	command := step.Command
	args := step.Args

	// Build job spec
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.namespace,
			Labels: map[string]string{
				"app":       "aeneas",
				"step-name": sanitizeLabel(step.Name),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(0), // No retries at K8s level
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "step",
							Image:   step.Image,
							Command: command,
							Args:    args,
							Env:     env,
						},
					},
				},
			},
		},
	}

	// Add timeout if specified
	if step.TimeoutSeconds > 0 {
		job.Spec.ActiveDeadlineSeconds = int64Ptr(int64(step.TimeoutSeconds))
	}

	return r.clientset.BatchV1().Jobs(r.namespace).Create(ctx, job, metav1.CreateOptions{})
}

// waitForJobCompletion watches the Job until it completes and returns the Pod
func (r *K8sRunner) waitForJobCompletion(ctx context.Context, jobName string) (*corev1.Pod, error) {
	// Start watching the job
	watcher, err := r.clientset.BatchV1().Jobs(r.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to watch job: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event := <-watcher.ResultChan():
			if event.Type == watch.Deleted {
				return nil, fmt.Errorf("job was deleted")
			}

			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}

			// Check if job has completed (succeeded or failed)
			if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
				// Get the pod created by this job
				return r.getPodForJob(ctx, jobName)
			}
		}
	}
}

// getPodForJob retrieves the Pod created by the Job
func (r *K8sRunner) getPodForJob(ctx context.Context, jobName string) (*corev1.Pod, error) {
	pods, err := r.clientset.CoreV1().Pods(r.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for job: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for job %s", jobName)
	}

	return &pods.Items[0], nil
}

// capturePodLogs retrieves logs from a pod
func (r *K8sRunner) capturePodLogs(ctx context.Context, podName string) (string, error) {
	req := r.clientset.CoreV1().Pods(r.namespace).GetLogs(podName, &corev1.PodLogOptions{})

	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to stream logs: %w", err)
	}
	defer stream.Close()

	logBytes, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(logBytes), nil
}

// getExitCode extracts the exit code from the pod's container status
func (r *K8sRunner) getExitCode(pod *corev1.Pod) int {
	if len(pod.Status.ContainerStatuses) == 0 {
		r.logger.Warn("No container statuses found in pod")
		return -1
	}

	containerStatus := pod.Status.ContainerStatuses[0]

	if containerStatus.State.Terminated != nil {
		return int(containerStatus.State.Terminated.ExitCode)
	}

	r.logger.Warn("Container not in terminated state")
	return -1
}

// cleanupJob deletes the Job and its pods
func (r *K8sRunner) cleanupJob(ctx context.Context, jobName string) error {
	deletePolicy := metav1.DeletePropagationForeground
	return r.clientset.BatchV1().Jobs(r.namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
}

// generateJobName creates a valid Kubernetes Job name from step name
func (r *K8sRunner) generateJobName(stepName string) string {
	// Convert to lowercase and replace invalid characters
	name := strings.ToLower(stepName)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)

	// Add timestamp to ensure uniqueness
	timestamp := time.Now().Unix()
	fullName := fmt.Sprintf("%s%s-%d", jobNamePrefix, name, timestamp)

	// Truncate if too long
	if len(fullName) > maxJobNameLength {
		fullName = fullName[:maxJobNameLength]
	}

	// Ensure it doesn't end with a hyphen
	fullName = strings.TrimRight(fullName, "-")

	return fullName
}

// sanitizeLabel ensures label values meet Kubernetes requirements
func sanitizeLabel(value string) string {
	// Max length for label values is 63 characters
	if len(value) > 63 {
		value = value[:63]
	}

	// Replace invalid characters with hyphens
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, value)

	// Trim leading/trailing non-alphanumeric
	value = strings.Trim(value, "-_.")

	return value
}

// Helper functions for pointer conversion
func int32Ptr(i int32) *int32 { return &i }
func int64Ptr(i int64) *int64 { return &i }

// Test support types and functions

// ExecuteResult wraps the result and error for testing
type ExecuteResult struct {
	Result *runner.Result
	Err    error
}

// TestableK8sRunner exposes the same interface for testing with fake clients
type TestableK8sRunner struct {
	*K8sRunner
}

// NewTestableK8sRunner creates a runner with an injected fake clientset for testing
func NewTestableK8sRunner(
	clientset kubernetes.Interface,
	namespace string,
	logger *logrus.Logger,
	cleanup bool,
) *TestableK8sRunner {
	if logger == nil {
		logger = logrus.New()
	}

	return &TestableK8sRunner{
		K8sRunner: &K8sRunner{
			clientset: clientset,
			namespace: namespace,
			logger:    logger,
			cleanup:   cleanup,
		},
	}
}
