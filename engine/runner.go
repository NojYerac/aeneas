package engine

import (
	"context"

	"github.com/nojyerac/aeneas/domain"
)

// RunResult represents the outcome of a step execution
type RunResult struct {
	ExitCode int
	Error    error
}

// Runner defines the interface for executing workflow steps.
// Concrete implementations (Docker, K8s, etc.) are provided by Task C.
// The engine orchestrates step execution without knowing implementation details.
type Runner interface {
	// Execute runs a single step and returns the result
	Execute(ctx context.Context, step *domain.StepDefinition) RunResult
}
