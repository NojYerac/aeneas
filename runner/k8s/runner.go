package k8s

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
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
	defaultNamespace       = "aeneas"
	maxJobNameLength       = 63
	jobCleanupRetentionSec = 300 // 5 minutes
)

// Config holds configuration for the K8sRunner
type Config struct {
	// Namespace to create Jobs in (default: "aeneas")
	Namespace string

	// Kubeconfig path for out-of-cluster config (optional, uses in-cluster config if empty)
	Kubeconfig string

	// CleanupRetentionSeconds defines how long to keep completed Jobs before deletion (default: 300)
	CleanupRetentionSeconds int
}

// K8sRunner executes workflow steps as Kubernetes Jobs
type K8sRunner struct {
	client                  kubernetes.Interface
	namespace               string
	cleanupRetentionSeconds int
	logger                  *logrus.Logger
}

// NewK8sRunner creates a new K8sRunner instance
func NewK8sRunner(cfg Config, logger *logrus.Logger) (*K8sRunner, error) {
	var config *rest.Config
	var err error

	// Try in-cluster config first, then fall back to kubeconfig
	if cfg.Kubeconfig == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create config from kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}

	cleanupRetention := cfg.CleanupRetentionSeconds
	if cleanupRetention == 0 {
		cleanupRetention = jobCleanupRetentionSec
	}

	return &K8sRunner{
		client:                  clientset,
		namespace:               namespace,
		cleanupRetentionSeconds: cleanupRetention,
		logger:                  logger,
	}, nil
}

// NewK8sRunnerForTest creates a K8sRunner with a provided client for testing purposes.
// This is exported to allow injection of fake clients in tests.
func NewK8sRunnerForTest(client kubernetes.Interface, namespace string, cleanupRetentionSeconds int, logger *logrus.Logger) *K8sRunner {
	if namespace == "" {
		namespace = defaultNamespace
	}
	if cleanupRetentionSeconds == 0 {
		cleanupRetentionSeconds = jobCleanupRetentionSec
	}
	return &K8sRunner{
		client:                  client,
		namespace:               namespace,
		cleanupRetentionSeconds: cleanupRetentionSeconds,
		logger:                  logger,
	}
}

// Execute runs a workflow step as a Kubernetes Job
func (r *K8sRunner) Execute(ctx context.Context, step domain.StepDefinition) (*runner.Result, error) {
	// Generate a unique execution ID for the job
	executionID := uuid.New().String()

	// Create Job name (truncated to 63 chars for DNS compliance)
	jobName := r.generateJobName(executionID, step.Name)

	// Build the Job spec
	job := r.buildJob(jobName, executionID, step)

	// Create the Job
	r.logger.WithFields(logrus.Fields{
		"job_name":     jobName,
		"namespace":    r.namespace,
		"image":        step.Image,
		"execution_id": executionID,
	}).Info("creating kubernetes job")

	createdJob, err := r.client.BatchV1().Jobs(r.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	// Defer cleanup of the job
	defer r.cleanupJob(context.Background(), jobName)

	// Watch the Job until completion
	exitCode, err := r.watchJobCompletion(ctx, createdJob)
	if err != nil {
		return nil, fmt.Errorf("failed to watch job completion: %w", err)
	}

	// Get the Pod associated with the Job to retrieve logs
	podName, err := r.getJobPodName(ctx, jobName)
	if err != nil {
		r.logger.WithError(err).Warn("failed to get job pod name")
		return &runner.Result{
			ExitCode: exitCode,
			Logs:     fmt.Sprintf("(failed to retrieve logs: %v)", err),
		}, nil
	}

	// Retrieve Pod logs
	logs, err := r.getPodLogs(ctx, podName)
	if err != nil {
		r.logger.WithError(err).Warn("failed to retrieve pod logs")
		logs = fmt.Sprintf("(failed to retrieve logs: %v)", err)
	}

	return &runner.Result{
		ExitCode: exitCode,
		Logs:     logs,
	}, nil
}

// generateJobName creates a DNS-compliant job name
func (r *K8sRunner) generateJobName(executionID, stepName string) string {
	// Sanitize step name (remove non-alphanumeric, convert to lowercase)
	sanitized := strings.ToLower(stepName)
	sanitized = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, sanitized)

	// Trim leading/trailing dashes
	sanitized = strings.Trim(sanitized, "-")

	// Build job name: aeneas-{execution-id}-{step-name}
	// Truncate execution ID to first 8 chars for brevity
	shortExecID := executionID[:8]
	jobName := fmt.Sprintf("aeneas-%s-%s", shortExecID, sanitized)

	// Truncate to max length if needed
	if len(jobName) > maxJobNameLength {
		jobName = jobName[:maxJobNameLength]
	}

	// Ensure it doesn't end with a dash
	jobName = strings.TrimRight(jobName, "-")

	return jobName
}

// buildJob constructs a Kubernetes Job from a StepDefinition
func (r *K8sRunner) buildJob(jobName, executionID string, step domain.StepDefinition) *batchv1.Job {
	// Prepare environment variables
	env := make([]corev1.EnvVar, 0, len(step.Env))
	for k, v := range step.Env {
		env = append(env, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	// Prepare command
	command := step.Command
	args := step.Args

	// Prepare timeout
	var activeDeadlineSeconds *int64
	if step.TimeoutSeconds > 0 {
		timeout := int64(step.TimeoutSeconds)
		activeDeadlineSeconds = &timeout
	}

	// Build the Job
	backoffLimit := int32(0) // No retries at K8s level
	ttlSecondsAfterFinished := int32(r.cleanupRetentionSeconds)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.namespace,
			Labels: map[string]string{
				"app":          "aeneas",
				"execution-id": executionID,
				"step-name":    step.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			ActiveDeadlineSeconds:   activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":          "aeneas",
						"execution-id": executionID,
						"step-name":    step.Name,
					},
				},
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

	return job
}

// watchJobCompletion watches a Job until it completes or fails
func (r *K8sRunner) watchJobCompletion(ctx context.Context, job *batchv1.Job) (int, error) {
	watcher, err := r.client.BatchV1().Jobs(r.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", job.Name),
		Watch:         true,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to watch job: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case event := <-watcher.ResultChan():
			if event.Type == watch.Error {
				return 0, fmt.Errorf("watch error: %v", event.Object)
			}

			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}

			// Check if job has completed
			if r.isJobComplete(job) {
				exitCode := r.getJobExitCode(ctx, job)
				return exitCode, nil
			}

			// Check if job has failed
			if r.isJobFailed(job) {
				exitCode := r.getJobExitCode(ctx, job)
				return exitCode, nil
			}
		}
	}
}

// isJobComplete checks if a Job has completed successfully
func (r *K8sRunner) isJobComplete(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// isJobFailed checks if a Job has failed
func (r *K8sRunner) isJobFailed(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// getJobExitCode retrieves the exit code from the Job's Pod
func (r *K8sRunner) getJobExitCode(ctx context.Context, job *batchv1.Job) int {
	podName, err := r.getJobPodName(ctx, job.Name)
	if err != nil {
		r.logger.WithError(err).Warn("failed to get pod name for exit code")
		return 1
	}

	pod, err := r.client.CoreV1().Pods(r.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		r.logger.WithError(err).Warn("failed to get pod for exit code")
		return 1
	}

	// Check container status for exit code
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == "step" {
			if containerStatus.State.Terminated != nil {
				return int(containerStatus.State.Terminated.ExitCode)
			}
		}
	}

	// Default to exit code 1 if we can't determine it
	return 1
}

// getJobPodName retrieves the Pod name associated with a Job
func (r *K8sRunner) getJobPodName(ctx context.Context, jobName string) (string, error) {
	// List Pods with the job-name label
	pods, err := r.client.CoreV1().Pods(r.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pod found for job %s", jobName)
	}

	// Return the first pod (there should only be one for a Job)
	return pods.Items[0].Name, nil
}

// getPodLogs retrieves logs from a Pod
func (r *K8sRunner) getPodLogs(ctx context.Context, podName string) (string, error) {
	// Wait a brief moment for logs to be available
	time.Sleep(1 * time.Second)

	logOptions := &corev1.PodLogOptions{
		Container: "step",
	}

	req := r.client.CoreV1().Pods(r.namespace).GetLogs(podName, logOptions)
	logStream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to stream pod logs: %w", err)
	}
	defer logStream.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, logStream)
	if err != nil {
		return "", fmt.Errorf("failed to read pod logs: %w", err)
	}

	return buf.String(), nil
}

// cleanupJob deletes a completed Job
func (r *K8sRunner) cleanupJob(ctx context.Context, jobName string) {
	// Note: With TTLSecondsAfterFinished set, Kubernetes will automatically clean up
	// the Job after the retention period. This method is kept for explicit cleanup
	// if needed in the future.
	r.logger.WithField("job_name", jobName).Debug("job cleanup handled by TTLSecondsAfterFinished")
}
