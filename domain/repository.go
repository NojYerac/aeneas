package domain

import (
	"context"
)

// ListOptions provides pagination and ordering options for repository list operations
type ListOptions struct {
	Limit   int
	Offset  int
	OrderBy string
}

// WorkflowRepository defines persistence operations for Workflow entities
type WorkflowRepository interface {
	Create(ctx context.Context, workflow *Workflow) error
	Get(ctx context.Context, id string) (*Workflow, error)
	List(ctx context.Context, opts ListOptions) ([]*Workflow, error)
	Update(ctx context.Context, workflow *Workflow) error
}

// ExecutionRepository defines persistence operations for Execution entities
type ExecutionRepository interface {
	Create(ctx context.Context, execution *Execution) error
	Get(ctx context.Context, id string) (*Execution, error)
	ListByWorkflow(ctx context.Context, workflowID string, opts ListOptions) ([]*Execution, error)
	UpdateStatus(ctx context.Context, id string, status ExecutionStatus) error
}

// StepExecutionRepository defines persistence operations for StepExecution entities
type StepExecutionRepository interface {
	Create(ctx context.Context, stepExecution *StepExecution) error
	ListByExecution(ctx context.Context, executionID string) ([]*StepExecution, error)
	UpdateStatus(ctx context.Context, id string, status StepExecutionStatus, exitCode *int) error
}
