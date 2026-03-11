package local

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner"
	"github.com/sirupsen/logrus"
)

// LocalRunner executes workflow steps using local Docker daemon
type LocalRunner struct {
	client *client.Client
	logger *logrus.Logger
}

// NewLocalRunner creates a new LocalRunner instance
func NewLocalRunner(logger *logrus.Logger) (*LocalRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &LocalRunner{
		client: cli,
		logger: logger,
	}, nil
}

// Execute runs a workflow step in a Docker container
func (r *LocalRunner) Execute(ctx context.Context, step *domain.StepDefinition) (*runner.Result, error) {
	// Set timeout if specified
	if step.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Pull image if not present
	if err := r.ensureImage(ctx, step.Image); err != nil {
		return nil, fmt.Errorf("failed to ensure image: %w", err)
	}

	// Prepare container configuration
	containerConfig := &container.Config{
		Image: step.Image,
		Cmd:   append(step.Command, step.Args...),
		Env:   mapToEnvSlice(step.Env),
	}

	// Create container
	resp, err := r.client.ContainerCreate(ctx, containerConfig, nil, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	containerID := resp.ID
	defer r.cleanup(context.Background(), containerID)

	// Start container
	if err := r.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := r.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		// Capture logs
		logs, err := r.captureLogs(ctx, containerID)
		if err != nil {
			r.logger.WithError(err).Warn("Failed to capture container logs")
		}

		return &runner.Result{
			ExitCode: int(status.StatusCode),
			Logs:     logs,
		}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("execution timeout exceeded")
	}

	return nil, fmt.Errorf("unexpected execution termination")
}

// ensureImage pulls the image if it's not already present locally
func (r *LocalRunner) ensureImage(ctx context.Context, imageName string) error {
	// Check if image exists locally
	_, _, err := r.client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		// Image exists
		return nil
	}

	r.logger.Infof("Pulling image: %s", imageName)

	reader, err := r.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// Drain the pull output
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read image pull output: %w", err)
	}

	return nil
}

// captureLogs retrieves logs from the container
func (r *LocalRunner) captureLogs(ctx context.Context, containerID string) (string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	reader, err := r.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}

	return string(logs), nil
}

// cleanup removes the container after execution
func (r *LocalRunner) cleanup(ctx context.Context, containerID string) {
	err := r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
	if err != nil {
		r.logger.WithError(err).Warnf("Failed to remove container %s", containerID)
	}
}

// mapToEnvSlice converts a map to environment variable slice format
func mapToEnvSlice(envMap map[string]string) []string {
	if envMap == nil {
		return nil
	}

	envSlice := make([]string, 0, len(envMap))
	for key, value := range envMap {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
	}
	return envSlice
}
