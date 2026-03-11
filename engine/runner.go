package engine

import (
	"context"

	"github.com/nojyerac/aeneas/domain"
)

// RunResult represents the result of a step execution
type RunResult struct {
	ExitCode int
	Error    error
}

// Success returns true if the step executed successfully
func (r RunResult) Success() bool {
	return r.Error == nil && r.ExitCode == 0
}

// Runner executes workflow steps
// Implementations handle the actual execution environment (Docker, K8s, etc.)
type Runner interface {
	// Execute runs a single step and returns the result
	Execute(ctx context.Context, step *domain.StepDefinition) RunResult
}
