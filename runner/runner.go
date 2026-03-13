package runner

import (
	"context"

	"github.com/nojyerac/aeneas/domain"
)

// Result represents the outcome of a step execution
type Result struct {
	ExitCode int
	Error    error
	Logs     string
}

// Runner defines the interface for executing workflow steps
type Runner interface {
	Execute(ctx context.Context, step *domain.StepDefinition) *Result
}
