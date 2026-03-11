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

// LocalRunner executes workflow steps using Docker containers on the local machine
type LocalRunner struct {
	client *client.Client
	logger *logrus.Logger
}

// NewLocalRunner creates a new LocalRunner instance
func NewLocalRunner(logger *logrus.Logger) (*LocalRunner, error) {
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
func (r *LocalRunner) Execute(ctx context.Context, step domain.StepDefinition) (*runner.Result, error) {
	// Set timeout if specified
	if step.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Pull image if not present locally
	if err := r.ensureImage(ctx, step.Image); err != nil {
		return nil, fmt.Errorf("failed to ensure image %s: %w", step.Image, err)
	}

	// Prepare environment variables
	env := make([]string, 0, len(step.Env))
	for k, v := range step.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Prepare command
	cmd := step.Command
	if len(step.Args) > 0 {
		cmd = append(cmd, step.Args...)
	}

	// Create container configuration
	containerConfig := &container.Config{
		Image: step.Image,
		Cmd:   cmd,
		Env:   env,
	}

	// Create the container
	resp, err := r.client.ContainerCreate(ctx, containerConfig, nil, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	containerID := resp.ID

	// Ensure cleanup
	defer func() {
		removeCtx := context.Background()
		if err := r.client.ContainerRemove(removeCtx, containerID, container.RemoveOptions{Force: true}); err != nil {
			r.logger.WithError(err).WithField("container_id", containerID).Warn("failed to remove container")
		}
	}()

	// Start the container
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
		// Get container logs
		logs, err := r.getContainerLogs(ctx, containerID)
		if err != nil {
			r.logger.WithError(err).Warn("failed to retrieve container logs")
			logs = fmt.Sprintf("(failed to retrieve logs: %v)", err)
		}

		return &runner.Result{
			ExitCode: int(status.StatusCode),
			Logs:     logs,
		}, nil
	}

	return nil, fmt.Errorf("unexpected end of wait")
}

// ensureImage pulls the image if it's not present locally
func (r *LocalRunner) ensureImage(ctx context.Context, imageName string) error {
	// Check if image exists locally
	images, err := r.client.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == imageName {
				// Image already exists
				return nil
			}
		}
	}

	// Pull the image
	r.logger.WithField("image", imageName).Info("pulling docker image")
	reader, err := r.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Consume the pull output to ensure completion
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read image pull output: %w", err)
	}

	return nil
}

// getContainerLogs retrieves logs from a container
func (r *LocalRunner) getContainerLogs(ctx context.Context, containerID string) (string, error) {
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	logs, err := r.client.ContainerLogs(ctx, containerID, logOptions)
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

// Close closes the Docker client connection
func (r *LocalRunner) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
