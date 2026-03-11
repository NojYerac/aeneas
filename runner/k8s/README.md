# Kubernetes Job Runner

The K8s runner executes Aeneas workflow steps as Kubernetes Jobs.

## Features

- Executes steps in isolated Kubernetes Jobs
- Supports both in-cluster and out-of-cluster configuration
- Captures pod logs and exit codes
- Respects step timeouts via ActiveDeadlineSeconds
- Automatic job cleanup after execution (configurable)
- Handles environment variables
- Generates valid Kubernetes-compliant job names

## Usage

### Basic Setup

```go
import (
    "github.com/nojyerac/aeneas/runner/k8s"
    "github.com/sirupsen/logrus"
)

// Create runner with default configuration
runner, err := k8s.NewK8sRunner(k8s.Config{
    Logger:  logrus.New(),
    Cleanup: true,
})
if err != nil {
    // Handle error
}

// Execute a step
result, err := runner.Execute(ctx, stepDefinition)
if err != nil {
    // Handle error
}

fmt.Printf("Exit Code: %d\n", result.ExitCode)
fmt.Printf("Logs: %s\n", result.Logs)
```

### Out-of-Cluster Configuration

```go
runner, err := k8s.NewK8sRunner(k8s.Config{
    Kubeconfig: "/path/to/kubeconfig",
    Namespace:  "my-namespace",
    Logger:     logrus.New(),
    Cleanup:    true,
})
```

### In-Cluster Configuration

When running inside a Kubernetes cluster:

```go
runner, err := k8s.NewK8sRunner(k8s.Config{
    // Empty Kubeconfig uses in-cluster config
    Namespace: "aeneas",  // Optional, defaults to "aeneas"
    Logger:    logrus.New(),
    Cleanup:   true,
})
```

## Configuration Options

- **Kubeconfig**: Path to kubeconfig file (empty string uses in-cluster config)
- **Namespace**: Kubernetes namespace for jobs (default: "aeneas")
- **Logger**: Logger instance (optional, creates default if nil)
- **Cleanup**: Whether to delete jobs after completion (default: true)

## Integration Testing

The unit tests use fake Kubernetes clients for fast, isolated testing. For integration testing with a real Kubernetes cluster:

### Prerequisites

1. **Kubernetes Cluster**: minikube, kind, or any K8s cluster
2. **kubectl**: Configured to access the cluster
3. **Namespace**: Create the test namespace

```bash
# Using minikube
minikube start

# Or using kind
kind create cluster --name aeneas-test

# Create namespace
kubectl create namespace aeneas
```

### Manual Integration Test Steps

1. **Build a test binary**:
   ```bash
   cd /path/to/aeneas
   go build -o test-k8s-runner ./cmd/test-k8s-runner
   ```

2. **Create a test step definition**:
   ```go
   package main

   import (
       "context"
       "fmt"
       "os"
       "time"

       "github.com/nojyerac/aeneas/domain"
       "github.com/nojyerac/aeneas/runner/k8s"
       "github.com/sirupsen/logrus"
   )

   func main() {
       logger := logrus.New()
       logger.SetLevel(logrus.DebugLevel)

       // Use kubeconfig from environment or default location
       kubeconfig := os.Getenv("KUBECONFIG")
       if kubeconfig == "" {
           kubeconfig = os.ExpandEnv("$HOME/.kube/config")
       }

       runner, err := k8s.NewK8sRunner(k8s.Config{
           Kubeconfig: kubeconfig,
           Namespace:  "aeneas",
           Logger:     logger,
           Cleanup:    false, // Keep jobs for inspection
       })
       if err != nil {
           logger.Fatalf("Failed to create runner: %v", err)
       }

       ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
       defer cancel()

       // Test step: simple echo command
       step := domain.StepDefinition{
           Name:    "integration-test",
           Image:   "alpine:latest",
           Command: []string{"sh", "-c"},
           Args:    []string{"echo 'Hello from K8s!' && sleep 2 && echo 'Done'"},
           Env: map[string]string{
               "TEST_VAR": "integration",
           },
           TimeoutSeconds: 60,
       }

       logger.Info("Executing step...")
       result, err := runner.Execute(ctx, step)
       if err != nil {
           logger.Fatalf("Execution failed: %v", err)
       }

       fmt.Printf("\n=== Results ===\n")
       fmt.Printf("Exit Code: %d\n", result.ExitCode)
       fmt.Printf("Logs:\n%s\n", result.Logs)

       if result.ExitCode != 0 {
           os.Exit(1)
       }
   }
   ```

3. **Run the test**:
   ```bash
   go run test-k8s-runner.go
   ```

4. **Verify job creation**:
   ```bash
   # List jobs
   kubectl get jobs -n aeneas

   # Check job details
   kubectl describe job <job-name> -n aeneas

   # View pod logs (if job still exists)
   kubectl logs -n aeneas -l job-name=<job-name>
   ```

5. **Test failure scenarios**:
   ```go
   // Failing step
   step := domain.StepDefinition{
       Name:    "failing-test",
       Image:   "alpine:latest",
       Command: []string{"sh", "-c"},
       Args:    []string{"exit 1"},
   }
   ```

6. **Test timeout handling**:
   ```go
   // Timeout test
   step := domain.StepDefinition{
       Name:           "timeout-test",
       Image:          "alpine:latest",
       Command:        []string{"sleep"},
       Args:           []string{"120"},
       TimeoutSeconds: 5, // Will timeout
   }
   ```

### Cleanup

```bash
# Delete test namespace and all jobs
kubectl delete namespace aeneas

# Stop minikube/kind if done
minikube stop
# or
kind delete cluster --name aeneas-test
```

## Job Naming

Jobs are named using the pattern: `aeneas-<step-name>-<timestamp>`

- Step names are sanitized to meet Kubernetes naming requirements
- Names are truncated to 63 characters maximum
- Invalid characters are replaced with hyphens

## Architecture

### Job Creation

1. Converts StepDefinition to Kubernetes Job spec
2. Sets RestartPolicy to Never
3. Sets BackoffLimit to 0 (no K8s-level retries)
4. Applies ActiveDeadlineSeconds if timeout specified

### Execution Flow

1. Create Job in Kubernetes
2. Watch Job using informers
3. Wait for Job completion (succeeded or failed)
4. Retrieve Pod created by Job
5. Capture logs from Pod
6. Extract exit code from container termination state
7. Delete Job if cleanup enabled

### Error Handling

- Context cancellation propagates to all operations
- Timeout errors are returned immediately
- Pod logs are captured even on failure
- Jobs are cleaned up on panic (when cleanup enabled)

## Dependencies

- `k8s.io/client-go`: Kubernetes Go client
- `k8s.io/api`: Kubernetes API types
- `k8s.io/apimachinery`: Kubernetes API machinery
- `github.com/sirupsen/logrus`: Structured logging

## Unit Tests

Run unit tests with fake client:

```bash
ginkgo -v ./runner/k8s
# or
go test -v ./runner/k8s
```

All unit tests use `fake.NewSimpleClientset()` for isolated, fast testing without requiring a real Kubernetes cluster.
