# Runner Package

The `runner` package provides interfaces and implementations for executing workflow steps.

## Components

### Runner Interface (`runner/runner.go`)

The core `Runner` interface defines how workflow steps are executed:

```go
type Runner interface {
    Execute(ctx context.Context, step domain.StepDefinition) (*Result, error)
}
```

#### Result Structure

```go
type Result struct {
    ExitCode int
    Logs     string
}
```

### MockRunner (`runner/mock/`)

A mock implementation for testing that allows configurable responses:

- **Purpose**: Unit testing without Docker dependencies
- **Features**:
  - Configurable responses per step name
  - Configurable errors for testing failure scenarios
  - Execution tracking for test verification
  - Chainable configuration methods

#### Example Usage

```go
runner := mock.NewMockRunner().
    WithResponse("build-step", 0, "build successful").
    WithError("failing-step", errors.New("expected failure"))

result, err := runner.Execute(ctx, buildStep)
```

### LocalRunner (`runner/local/`)

Docker-based implementation that executes steps in containers:

- **Purpose**: Production execution of workflow steps
- **Requirements**:
  - Docker daemon running and accessible
  - Docker SDK for Go (`github.com/docker/docker`)
  - Go 1.25+ (due to project requirements)

#### Features

- Automatic image pulling if not present locally
- Container lifecycle management (create, start, wait, cleanup)
- Timeout support via context deadlines
- Log capture (stdout + stderr)
- Proper resource cleanup even on errors

#### Example Usage

```go
runner, err := local.NewLocalRunner(logger)
if err != nil {
    log.Fatal(err)
}
defer runner.Close()

step := domain.StepDefinition{
    Name:           "build",
    Image:          "golang:1.25",
    Command:        []string{"go"},
    Args:           []string{"build", "./..."},
    Env:            map[string]string{"CGO_ENABLED": "0"},
    TimeoutSeconds: 300,
}

result, err := runner.Execute(ctx, step)
```

## Testing

### Unit Tests (MockRunner)

```bash
ginkgo -v runner/mock/
```

The MockRunner tests run without external dependencies and verify:
- Default behavior (success with mock logs)
- Configured responses
- Error handling
- Execution tracking
- Reset functionality

### Integration Tests (LocalRunner)

**Note**: LocalRunner tests are tagged with `// +build integration` and require:
- Docker daemon running
- Network access to pull images
- Go 1.25+ toolchain

To run integration tests:

```bash
ginkgo -v -tags=integration runner/local/
```

Integration tests verify:
- Container execution with exit codes
- Environment variable passing
- Command and argument handling
- Timeout enforcement
- Image pulling
- Log capture

## Dependencies

- `github.com/docker/docker` (v27.5.1+): Docker SDK for container operations
- `github.com/sirupsen/logrus`: Logging
- `github.com/onsi/ginkgo/v2`: Test framework
- `github.com/onsi/gomega`: Test matchers

## Known Issues / Notes

1. **Go Version Requirement**: The project requires Go 1.25.1+. Some development environments may have Go 1.24.x, which can cause compilation issues with the Docker SDK due to module compatibility. Use `GOTOOLCHAIN=auto` to automatically download the required version.

2. **Docker SDK Compatibility**: The `+incompatible` suffix on the Docker SDK version indicates it predates Go modules. Ensure Go 1.25+ is used for compilation.

3. **Integration Test Environment**: LocalRunner integration tests require a properly configured Docker environment. Consider using Docker-in-Docker or ensuring the test runner has Docker socket access.
