package mock

import (
	"context"
	"fmt"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner"
)

// MockRunner is a configurable mock implementation of the Runner interface
type MockRunner struct {
	// Responses maps step names to their expected results
	Responses map[string]*runner.Result
	// Errors maps step names to errors to return
	Errors map[string]error
	// ExecutedSteps tracks which steps have been executed
	ExecutedSteps []string
}

// NewMockRunner creates a new MockRunner instance
func NewMockRunner() *MockRunner {
	return &MockRunner{
		Responses:     make(map[string]*runner.Result),
		Errors:        make(map[string]error),
		ExecutedSteps: make([]string, 0),
	}
}

// Execute simulates step execution with configured responses
func (m *MockRunner) Execute(ctx context.Context, step *domain.StepDefinition) (*runner.Result, error) {
	m.ExecutedSteps = append(m.ExecutedSteps, step.Name)

	// Check for configured error
	if err, ok := m.Errors[step.Name]; ok {
		return nil, err
	}

	// Check for configured response
	if result, ok := m.Responses[step.Name]; ok {
		return result, nil
	}

	// Default successful response
	return &runner.Result{
		ExitCode: 0,
		Logs:     fmt.Sprintf("Mock execution of step: %s", step.Name),
	}, nil
}

// AddResponse configures a successful response for a step
func (m *MockRunner) AddResponse(stepName string, exitCode int, logs string) {
	m.Responses[stepName] = &runner.Result{
		ExitCode: exitCode,
		Logs:     logs,
	}
}

// AddError configures an error response for a step
func (m *MockRunner) AddError(stepName string, err error) {
	m.Errors[stepName] = err
}

// Reset clears all configured responses and execution history
func (m *MockRunner) Reset() {
	m.Responses = make(map[string]*runner.Result)
	m.Errors = make(map[string]error)
	m.ExecutedSteps = make([]string, 0)
}
