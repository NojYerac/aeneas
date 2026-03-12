# Kubernetes Job Runner

The Kubernetes Job Runner (`runner/k8s`) implements the `runner.Runner` interface to execute workflow steps as Kubernetes Jobs.

## Features

- **Job Creation**: Creates Kubernetes Jobs from `StepDefinition` with proper configuration
- **Job Naming**: Generates DNS-compliant job names (max 63 chars): `aeneas-{exec-id}-{step-name}`
- **Environment Variables**: Passes environment variables from `StepDefinition` to the Job container
- **Timeouts**: Sets `ActiveDeadlineSeconds` on Jobs based on `TimeoutSeconds`
- **No Retries**: BackoffLimit set to 0 (no retries at Kubernetes level)
- **Log Capture**: Retrieves Pod logs after Job completion
- **Exit Code**: Returns container exit code from Pod termination state
- **Auto-cleanup**: Uses `TTLSecondsAfterFinished` for automatic Job cleanup (default: 300 seconds)
- **Flexible Config**: Supports both in-cluster and out-of-cluster configurations

## Usage

### Basic Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/nojyerac/aeneas/domain"
    "github.com/nojyerac/aeneas/runner/k8s"
    "github.com/sirupsen/logrus"
)

func main() {
    logger := logrus.New()

    // Configure the runner
    cfg := k8s.Config{
        Namespace:               "aeneas",              // Optional, default: "aeneas"
        Kubeconfig:              "",                     // Optional, uses in-cluster config if empty
        CleanupRetentionSeconds: 300,                    // Optional, default: 300
    }

    // Create the runner
    runner, err := k8s.NewK8sRunner(cfg, logger)
    if err != nil {
        log.Fatalf("Failed to create K8s runner: %v", err)
    }

    // Define a step
    step := domain.StepDefinition{
        Name:    "hello-world",
        Image:   "alpine:latest",
        Command: []string{"echo"},
        Args:    []string{"Hello from Kubernetes!"},
        Env: map[string]string{
            "MY_VAR": "my-value",
        },
        TimeoutSeconds: 60,
    }

    // Execute the step
    result, err := runner.Execute(context.Background(), step)
    if err != nil {
        log.Fatalf("Failed to execute step: %v", err)
    }

    fmt.Printf("Exit Code: %d\n", result.ExitCode)
    fmt.Printf("Logs:\n%s\n", result.Logs)
}
```

### Configuration Options

#### In-Cluster Configuration

When running inside a Kubernetes cluster (e.g., as a Pod):

```go
cfg := k8s.Config{
    Namespace: "aeneas",
    // Kubeconfig is empty, so in-cluster config is used
}
```

#### Out-of-Cluster Configuration

When running outside a Kubernetes cluster (e.g., local development):

```go
cfg := k8s.Config{
    Namespace:  "aeneas",
    Kubeconfig: "/path/to/.kube/config",
}
```

#### Custom Cleanup Retention

Control how long completed Jobs are retained before automatic deletion:

```go
cfg := k8s.Config{
    Namespace:               "aeneas",
    CleanupRetentionSeconds: 600, // 10 minutes
}
```

## Job Specification Details

### Job Naming

Jobs are named using the pattern: `aeneas-{execution-id}-{step-name}`

- **execution-id**: First 8 characters of a UUID
- **step-name**: Sanitized (lowercase, non-alphanumeric converted to dashes)
- **Total length**: Truncated to 63 characters for DNS compliance

Example: `aeneas-a1b2c3d4-my-build-step`

### Labels

Each Job is labeled with:

- `app: aeneas`
- `execution-id: {full-uuid}`
- `step-name: {step-name}`

### Container Configuration

- **Name**: `step`
- **Image**: From `StepDefinition.Image`
- **Command**: From `StepDefinition.Command`
- **Args**: From `StepDefinition.Args`
- **Env**: From `StepDefinition.Env`

### Job Policies

- **RestartPolicy**: `Never` (no Pod restarts)
- **BackoffLimit**: `0` (no Job retries)
- **ActiveDeadlineSeconds**: From `StepDefinition.TimeoutSeconds` (if > 0)
- **TTLSecondsAfterFinished**: Configurable (default: 300 seconds)

## Manual Integration Testing

Since the K8s runner requires a live Kubernetes cluster, integration tests must be performed manually.

### Prerequisites

1. **Kubernetes Cluster**: minikube, kind, or any Kubernetes cluster
2. **kubectl**: Configured to access the cluster
3. **Namespace**: Create the `aeneas` namespace (or configure a custom one)

### Setup Steps

#### Using minikube

```bash
# Start minikube
minikube start

# Create namespace
kubectl create namespace aeneas

# Verify cluster access
kubectl cluster-info
```

#### Using kind

```bash
# Create a kind cluster
kind create cluster --name aeneas-test

# Create namespace
kubectl create namespace aeneas

# Verify cluster access
kubectl cluster-info --context kind-aeneas-test
```

### Integration Test Scenarios

#### Test 1: Successful Execution

```go
step := domain.StepDefinition{
    Name:           "test-success",
    Image:          "alpine:latest",
    Command:        []string{"echo"},
    Args:           []string{"Hello Kubernetes!"},
    TimeoutSeconds: 60,
}

result, err := runner.Execute(ctx, step)
// Expected: err == nil, result.ExitCode == 0, result.Logs contains "Hello Kubernetes!"
```

**Verify:**

```bash
kubectl get jobs -n aeneas
kubectl get pods -n aeneas
kubectl logs <pod-name> -n aeneas
```

#### Test 2: Failed Execution

```go
step := domain.StepDefinition{
    Name:           "test-failure",
    Image:          "alpine:latest",
    Command:        []string{"sh"},
    Args:           []string{"-c", "exit 42"},
    TimeoutSeconds: 60,
}

result, err := runner.Execute(ctx, step)
// Expected: err == nil, result.ExitCode == 42
```

**Verify:**

```bash
kubectl get jobs -n aeneas
kubectl describe job <job-name> -n aeneas
```

#### Test 3: Environment Variables

```go
step := domain.StepDefinition{
    Name:    "test-env",
    Image:   "alpine:latest",
    Command: []string{"sh"},
    Args:    []string{"-c", "echo $TEST_VAR"},
    Env: map[string]string{
        "TEST_VAR": "test-value",
    },
    TimeoutSeconds: 60,
}

result, err := runner.Execute(ctx, step)
// Expected: err == nil, result.ExitCode == 0, result.Logs contains "test-value"
```

**Verify:**

```bash
kubectl get pods -n aeneas
kubectl logs <pod-name> -n aeneas
```

#### Test 4: Timeout

```go
step := domain.StepDefinition{
    Name:           "test-timeout",
    Image:          "alpine:latest",
    Command:        []string{"sleep"},
    Args:           []string{"120"},
    TimeoutSeconds: 5, // 5 seconds
}

result, err := runner.Execute(ctx, step)
// Expected: Job should be terminated due to ActiveDeadlineExceeded
```

**Verify:**

```bash
kubectl describe job <job-name> -n aeneas
# Should show: "Job has reached the specified deadline"
```

#### Test 5: Long-Running Job

```go
step := domain.StepDefinition{
    Name:    "test-long-running",
    Image:   "alpine:latest",
    Command: []string{"sh"},
    Args:    []string{"-c", "for i in $(seq 1 10); do echo Iteration $i; sleep 2; done"},
    TimeoutSeconds: 60,
}

result, err := runner.Execute(ctx, step)
// Expected: err == nil, result.ExitCode == 0, logs contain all iterations
```

**Verify:**

```bash
kubectl logs <pod-name> -n aeneas -f
# Should stream logs in real-time
```

#### Test 6: Job Cleanup

```go
cfg := k8s.Config{
    Namespace:               "aeneas",
    CleanupRetentionSeconds: 10, // 10 seconds
}

runner, _ := k8s.NewK8sRunner(cfg, logger)

step := domain.StepDefinition{
    Name:           "test-cleanup",
    Image:          "alpine:latest",
    Command:        []string{"echo"},
    Args:           []string{"cleanup test"},
    TimeoutSeconds: 60,
}

runner.Execute(ctx, step)

// Wait 15 seconds
time.Sleep(15 * time.Second)
```

**Verify:**

```bash
kubectl get jobs -n aeneas
# Job should be automatically deleted after TTL expires
```

### Monitoring and Debugging

#### View all Jobs

```bash
kubectl get jobs -n aeneas
```

#### View Job details

```bash
kubectl describe job <job-name> -n aeneas
```

#### View Pods

```bash
kubectl get pods -n aeneas
```

#### View Pod logs

```bash
kubectl logs <pod-name> -n aeneas
```

#### View events

```bash
kubectl get events -n aeneas --sort-by='.lastTimestamp'
```

#### Clean up test resources

```bash
kubectl delete jobs --all -n aeneas
kubectl delete pods --all -n aeneas
```

## Troubleshooting

### Job Fails to Create

**Symptoms**: `failed to create job` error

**Solutions**:

1. Verify namespace exists: `kubectl get namespace aeneas`
2. Check RBAC permissions: Ensure service account has permissions to create Jobs
3. Verify kubeconfig path and cluster connectivity

### Pod Logs Not Available

**Symptoms**: `failed to retrieve pod logs` error

**Solutions**:

1. Check if Pod exists: `kubectl get pods -n aeneas`
2. Verify Pod status: `kubectl describe pod <pod-name> -n aeneas`
3. Ensure container name is `step`

### Jobs Not Cleaning Up

**Symptoms**: Old Jobs remain in cluster

**Solutions**:

1. Verify TTLSecondsAfterFinished is supported (Kubernetes 1.21+)
2. Check TTL controller is enabled in cluster
3. Manually clean up: `kubectl delete jobs --all -n aeneas`

### Timeout Not Working

**Symptoms**: Job runs longer than TimeoutSeconds

**Solutions**:

1. Verify ActiveDeadlineSeconds is set on Job spec
2. Check Job status: `kubectl describe job <job-name> -n aeneas`
3. Ensure Kubernetes version supports ActiveDeadlineSeconds

## Dependencies

- `k8s.io/client-go` v0.32.3+
- `k8s.io/api` v0.32.3+
- `k8s.io/apimachinery` v0.32.3+

## Testing

### Unit Tests

Run unit tests with fake clientset:

```bash
ginkgo -r -v ./runner/k8s
```

### Integration Tests

Manual integration tests required. See "Manual Integration Testing" section above.

## See Also

- [Runner Interface](../runner.go)
- [Local Runner](../local/runner.go) - Docker-based local execution
- [Domain Entities](../../domain/entities.go)
