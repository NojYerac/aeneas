package runner

import (
	"context"

	"github.com/nojyerac/aeneas/domain"
)

// Result contains the outcome of a step execution
type Result struct {
	ExitCode int
	Logs     string
}

// Runner defines the interface for executing workflow steps
type Runner interface {
	Execute(ctx context.Context, step domain.StepDefinition) (*Result, error)
}
