package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
)

// ExecutionService provides business logic for execution operations
type ExecutionService struct {
	workflowRepo      domain.WorkflowRepository
	executionRepo     domain.ExecutionRepository
	stepExecutionRepo domain.StepExecutionRepository
}

// NewExecutionService creates a new ExecutionService
func NewExecutionService(
	workflowRepo domain.WorkflowRepository,
	executionRepo domain.ExecutionRepository,
	stepExecutionRepo domain.StepExecutionRepository,
) *ExecutionService {
	return &ExecutionService{
		workflowRepo:      workflowRepo,
		executionRepo:     executionRepo,
		stepExecutionRepo: stepExecutionRepo,
	}
}

// Trigger creates and starts a new execution for a workflow
func (s *ExecutionService) Trigger(ctx context.Context, workflowID string) (*domain.Execution, error) {
	if _, err := uuid.Parse(workflowID); err != nil {
		return nil, NewValidationError("invalid workflow ID format", err)
	}

	// Get the workflow
	workflow, err := s.workflowRepo.Get(ctx, workflowID)
	if err != nil {
		return nil, NewInternalError("failed to get workflow", err)
	}
	if workflow == nil {
		return nil, NewNotFoundError("workflow not found")
	}

	// Only active workflows can be executed
	if workflow.Status != domain.WorkflowActive {
		return nil, NewConflictError("only active workflows can be executed")
	}

	// Create execution
	now := time.Now()
	execution := &domain.Execution{
		ID:         uuid.New(),
		WorkflowID: workflow.ID,
		Status:     domain.ExecutionPending,
		StartedAt:  &now,
	}

	if err := s.executionRepo.Create(ctx, execution); err != nil {
		return nil, NewInternalError("failed to create execution", err)
	}

	// Create step executions for each workflow step
	for _, step := range workflow.Steps {
		stepExecution := &domain.StepExecution{
			ID:          uuid.New(),
			ExecutionID: execution.ID,
			StepName:    step.Name,
			Status:      domain.StepExecutionPending,
		}
		if err := s.stepExecutionRepo.Create(ctx, stepExecution); err != nil {
			return nil, NewInternalError("failed to create step execution", err)
		}
	}

	return execution, nil
}

// GetWithSteps retrieves an execution with its step executions
func (s *ExecutionService) GetWithSteps(
	ctx context.Context,
	id string,
) (*domain.Execution, []*domain.StepExecution, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil, NewValidationError("invalid execution ID format", err)
	}

	execution, err := s.executionRepo.Get(ctx, id)
	if err != nil {
		return nil, nil, NewInternalError("failed to get execution", err)
	}
	if execution == nil {
		return nil, nil, NewNotFoundError("execution not found")
	}

	steps, err := s.stepExecutionRepo.ListByExecution(ctx, id)
	if err != nil {
		return nil, nil, NewInternalError("failed to get step executions", err)
	}

	return execution, steps, nil
}

// ListByWorkflow retrieves executions for a workflow
func (s *ExecutionService) ListByWorkflow(
	ctx context.Context,
	workflowID string,
	limit, offset int,
) ([]*domain.Execution, error) {
	if _, err := uuid.Parse(workflowID); err != nil {
		return nil, NewValidationError("invalid workflow ID format", err)
	}

	if limit < 0 || limit > 100 {
		return nil, NewValidationError("limit must be between 0 and 100", nil)
	}

	if offset < 0 {
		return nil, NewValidationError("offset must be non-negative", nil)
	}

	opts := domain.ListOptions{
		Limit:   limit,
		Offset:  offset,
		OrderBy: "started_at DESC",
	}

	executions, err := s.executionRepo.ListByWorkflow(ctx, workflowID, opts)
	if err != nil {
		return nil, NewInternalError("failed to list executions", err)
	}

	return executions, nil
}

// Cancel cancels a running or pending execution
func (s *ExecutionService) Cancel(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return NewValidationError("invalid execution ID format", err)
	}

	execution, err := s.executionRepo.Get(ctx, id)
	if err != nil {
		return NewInternalError("failed to get execution", err)
	}
	if execution == nil {
		return NewNotFoundError("execution not found")
	}

	// Cancellation only for Pending/Running executions
	if execution.Status != domain.ExecutionPending && execution.Status != domain.ExecutionRunning {
		return NewConflictError("can only cancel pending or running executions")
	}

	// Validate state transition
	if err := domain.TransitionExecution(execution.Status, domain.ExecutionCanceled); err != nil {
		return NewConflictError(err.Error())
	}

	if err := s.executionRepo.UpdateStatus(ctx, id, domain.ExecutionCanceled); err != nil {
		return NewInternalError("failed to cancel execution", err)
	}

	return nil
}
