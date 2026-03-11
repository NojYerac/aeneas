package runner

import (
	"context"

	"github.com/nojyerac/aeneas/domain"
)

// Result represents the outcome of a step execution
type Result struct {
	ExitCode int
	Logs     string
}

// Runner defines the interface for executing workflow steps.
// Implementations can execute steps in different environments
// (local Docker, Kubernetes, cloud platforms, etc.)
type Runner interface {
	// Execute runs a single step and returns the result
	Execute(ctx context.Context, step domain.StepDefinition) (*Result, error)
}
