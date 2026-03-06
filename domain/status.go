package domain

import "fmt"

// WorkflowStatus represents the lifecycle state of a workflow definition
type WorkflowStatus string

const (
	WorkflowDraft    WorkflowStatus = "draft"
	WorkflowActive   WorkflowStatus = "active"
	WorkflowArchived WorkflowStatus = "archived"
)

// ExecutionStatus represents the lifecycle state of a workflow execution
type ExecutionStatus string

const (
	ExecutionPending   ExecutionStatus = "pending"
	ExecutionRunning   ExecutionStatus = "running"
	ExecutionSucceeded ExecutionStatus = "succeeded"
	ExecutionFailed    ExecutionStatus = "failed"
	ExecutionCancelled ExecutionStatus = "canceled"
)

// StepExecutionStatus represents the lifecycle state of a step execution
type StepExecutionStatus string

const (
	StepExecutionPending   StepExecutionStatus = "pending"
	StepExecutionRunning   StepExecutionStatus = "running"
	StepExecutionSucceeded StepExecutionStatus = "succeeded"
	StepExecutionFailed    StepExecutionStatus = "failed"
	StepExecutionSkipped   StepExecutionStatus = "skipped"
)

// Valid workflow status transitions
var workflowTransitions = map[WorkflowStatus][]WorkflowStatus{
	WorkflowDraft:    {WorkflowActive},
	WorkflowActive:   {WorkflowArchived},
	WorkflowArchived: {}, // terminal state
}

// Valid execution status transitions
var executionTransitions = map[ExecutionStatus][]ExecutionStatus{
	ExecutionPending:   {ExecutionRunning, ExecutionCancelled},
	ExecutionRunning:   {ExecutionSucceeded, ExecutionFailed, ExecutionCancelled},
	ExecutionSucceeded: {}, // terminal state
	ExecutionFailed:    {}, // terminal state
	ExecutionCancelled: {}, // terminal state
}

// Valid step execution status transitions
var stepExecutionTransitions = map[StepExecutionStatus][]StepExecutionStatus{
	StepExecutionPending:   {StepExecutionRunning, StepExecutionSkipped},
	StepExecutionRunning:   {StepExecutionSucceeded, StepExecutionFailed},
	StepExecutionSucceeded: {}, // terminal state
	StepExecutionFailed:    {}, // terminal state
	StepExecutionSkipped:   {}, // terminal state
}

// TransitionWorkflow validates a workflow status transition
func TransitionWorkflow(from, to WorkflowStatus) error {
	validTransitions, exists := workflowTransitions[from]
	if !exists {
		return fmt.Errorf("invalid current workflow status: %s", from)
	}

	for _, valid := range validTransitions {
		if valid == to {
			return nil
		}
	}

	return fmt.Errorf("invalid workflow transition from %s to %s", from, to)
}

// TransitionExecution validates an execution status transition
func TransitionExecution(from, to ExecutionStatus) error {
	validTransitions, exists := executionTransitions[from]
	if !exists {
		return fmt.Errorf("invalid current execution status: %s", from)
	}

	for _, valid := range validTransitions {
		if valid == to {
			return nil
		}
	}

	return fmt.Errorf("invalid execution transition from %s to %s", from, to)
}

// TransitionStepExecution validates a step execution status transition
func TransitionStepExecution(from, to StepExecutionStatus) error {
	validTransitions, exists := stepExecutionTransitions[from]
	if !exists {
		return fmt.Errorf("invalid current step execution status: %s", from)
	}

	for _, valid := range validTransitions {
		if valid == to {
			return nil
		}
	}

	return fmt.Errorf("invalid step execution transition from %s to %s", from, to)
}
