package mock

import (
	"context"
	"fmt"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner"
)

// MockRunner is a configurable mock implementation of the Runner interface
// for testing purposes
type MockRunner struct {
	// Responses maps step names to their configured results
	Responses map[string]*runner.Result
	// Errors maps step names to their configured errors
	Errors map[string]error
	// DefaultResult is returned when a step name is not in Responses
	DefaultResult *runner.Result
	// DefaultError is returned when a step name is not in Errors
	DefaultError error
	// ExecutedSteps tracks which steps were executed
	ExecutedSteps []string
}

// NewMockRunner creates a new MockRunner with default success behavior
func NewMockRunner() *MockRunner {
	return &MockRunner{
		Responses: make(map[string]*runner.Result),
		Errors:    make(map[string]error),
		DefaultResult: &runner.Result{
			ExitCode: 0,
			Logs:     "mock execution successful",
		},
		ExecutedSteps: make([]string, 0),
	}
}

// Execute simulates step execution based on configured responses
//
//nolint:gocritic // Interface signature defined by Runner contract
func (m *MockRunner) Execute(ctx context.Context, step domain.StepDefinition) (*runner.Result, error) {
	m.ExecutedSteps = append(m.ExecutedSteps, step.Name)

	// Check for configured error first
	if err, ok := m.Errors[step.Name]; ok {
		return nil, err
	}
	if m.DefaultError != nil {
		return nil, m.DefaultError
	}

	// Return configured result
	if result, ok := m.Responses[step.Name]; ok {
		return result, nil
	}

	return m.DefaultResult, nil
}

// WithResponse configures a specific response for a step name
func (m *MockRunner) WithResponse(stepName string, exitCode int, logs string) *MockRunner {
	m.Responses[stepName] = &runner.Result{
		ExitCode: exitCode,
		Logs:     logs,
	}
	return m
}

// WithError configures an error for a step name
func (m *MockRunner) WithError(stepName string, err error) *MockRunner {
	m.Errors[stepName] = err
	return m
}

// WithDefaultError sets a default error for all unconfigured steps
func (m *MockRunner) WithDefaultError(err error) *MockRunner {
	m.DefaultError = err
	return m
}

// Reset clears all execution history and configured responses
func (m *MockRunner) Reset() {
	m.Responses = make(map[string]*runner.Result)
	m.Errors = make(map[string]error)
	m.ExecutedSteps = make([]string, 0)
	m.DefaultResult = &runner.Result{
		ExitCode: 0,
		Logs:     "mock execution successful",
	}
	m.DefaultError = nil
}

// AssertExecuted checks if a step was executed
func (m *MockRunner) AssertExecuted(stepName string) error {
	for _, executed := range m.ExecutedSteps {
		if executed == stepName {
			return nil
		}
	}
	return fmt.Errorf("step %q was not executed", stepName)
}

// GetExecutedCount returns the number of steps executed
func (m *MockRunner) GetExecutedCount() int {
	return len(m.ExecutedSteps)
}
