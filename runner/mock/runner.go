package mock

import (
	"context"
	"fmt"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner"
)

var _ runner.Runner = (*MockRunner)(nil)

// MockRunner is a mock implementation of the Runner interface for testing
type MockRunner struct {
	// Responses maps step names to their configured results
	Responses map[string]*runner.Result
	// Errors maps step names to their configured errors
	Errors map[string]error
	// ExecutedSteps tracks which steps have been executed
	ExecutedSteps []domain.StepDefinition
}

// NewMockRunner creates a new MockRunner instance
func NewMockRunner() *MockRunner {
	return &MockRunner{
		Responses:     make(map[string]*runner.Result),
		Errors:        make(map[string]error),
		ExecutedSteps: make([]domain.StepDefinition, 0),
	}
}

// WithResponse configures a successful response for a specific step name
func (m *MockRunner) WithResponse(stepName string, exitCode int, logs string) *MockRunner {
	m.Responses[stepName] = &runner.Result{
		ExitCode: exitCode,
		Logs:     logs,
	}
	return m
}

// WithError configures an error response for a specific step name
func (m *MockRunner) WithError(stepName string, err error) *MockRunner {
	m.Errors[stepName] = err
	return m
}

// Execute implements the Runner interface
func (m *MockRunner) Execute(ctx context.Context, step *domain.StepDefinition) (*runner.Result, error) {
	// Track execution
	m.ExecutedSteps = append(m.ExecutedSteps, *step)

	// Check for configured error
	if err, ok := m.Errors[step.Name]; ok {
		return nil, err
	}

	// Check for configured response
	if result, ok := m.Responses[step.Name]; ok {
		return result, nil
	}

	// Default response: success with empty logs
	return &runner.Result{
		ExitCode: 0,
		Logs:     fmt.Sprintf("Mock execution of step: %s", step.Name),
	}, nil
}

// Reset clears all execution history and configured responses
func (m *MockRunner) Reset() {
	m.ExecutedSteps = make([]domain.StepDefinition, 0)
	m.Responses = make(map[string]*runner.Result)
	m.Errors = make(map[string]error)
}

// GetExecutionCount returns the number of times Execute was called
func (m *MockRunner) GetExecutionCount() int {
	return len(m.ExecutedSteps)
}

// GetLastExecutedStep returns the last executed step, or nil if none
func (m *MockRunner) GetLastExecutedStep() *domain.StepDefinition {
	if len(m.ExecutedSteps) == 0 {
		return nil
	}
	return &m.ExecutedSteps[len(m.ExecutedSteps)-1]
}
