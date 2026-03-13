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

var _ runner.Runner = (*LocalRunner)(nil)

// LocalRunner executes workflow steps using Docker containers on the local machine
type LocalRunner struct {
	client *client.Client
	logger logrus.FieldLogger
}

// NewLocalRunner creates a new LocalRunner instance
func NewLocalRunner(logger logrus.FieldLogger) (*LocalRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &LocalRunner{
		client: cli,
		logger: logger,
	}, nil
}

// Execute runs a workflow step in a Docker container
func (r *LocalRunner) Execute(ctx context.Context, step *domain.StepDefinition) *runner.Result {
	// Set timeout if specified
	if step.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Pull image if not present
	if err := r.ensureImage(ctx, step.Image); err != nil {
		return &runner.Result{
			Error: fmt.Errorf("failed to ensure image %s: %w", step.Image, err),
		}
	}

	// Prepare container configuration
	config := &container.Config{
		Image: step.Image,
		Cmd:   append(step.Command, step.Args...),
		Env:   mapToEnvSlice(step.Env),
	}

	// Create container
	resp, err := r.client.ContainerCreate(ctx, config, nil, nil, nil, "")
	if err != nil {
		return &runner.Result{
			Error: fmt.Errorf("failed to create container: %w", err),
		}
	}
	containerID := resp.ID

	// Ensure cleanup
	defer func() {
		removeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := r.client.ContainerRemove(removeCtx, containerID, container.RemoveOptions{Force: true}); err != nil {
			r.logger.WithError(err).WithField("container_id", containerID).Warn("Failed to remove container")
		}
	}()

	// Start container
	if err := r.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return &runner.Result{
			Error: fmt.Errorf("failed to start container: %w", err),
		}
	}

	// Wait for container to finish
	statusCh, errCh := r.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return &runner.Result{
				Error: fmt.Errorf("error waiting for container: %w", err),
			}
		}
	case status := <-statusCh:
		// Get container logs
		logs, err := r.captureLogs(ctx, containerID)
		if err != nil {
			r.logger.WithError(err).Warn("failed to retrieve container logs")
			logs = fmt.Sprintf("(failed to retrieve logs: %v)", err)
		}

		return &runner.Result{
			ExitCode: int(status.StatusCode),
			Logs:     logs,
		}
	}

	return &runner.Result{
		Error: ctx.Err(),
	}
}

// ensureImage pulls the image if it's not already present locally
func (r *LocalRunner) ensureImage(ctx context.Context, imageName string) error {
	_, err := r.client.ImageInspect(ctx, imageName)
	if err == nil {
		// Image already exists
		return nil
	}

	r.logger.WithField("image", imageName).Info("Pulling image")
	reader, err := r.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Consume the pull output to ensure it completes
	_, err = io.Copy(io.Discard, reader)
	return err
}

// captureLogs retrieves container logs (stdout + stderr combined)
func (r *LocalRunner) captureLogs(ctx context.Context, containerID string) (string, error) {
	logs, err := r.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", err
	}
	defer logs.Close()

	logBytes, err := io.ReadAll(logs)
	if err != nil {
		return "", err
	}

	return string(logBytes), nil
}

// mapToEnvSlice converts a map of environment variables to a slice of KEY=VALUE strings
func mapToEnvSlice(env map[string]string) []string {
	if env == nil {
		return nil
	}

	envSlice := make([]string, 0, len(env))
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}
	return envSlice
}

// Close releases resources held by the LocalRunner
func (r *LocalRunner) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
