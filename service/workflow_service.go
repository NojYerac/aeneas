package service

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
)

// WorkflowService provides business logic for workflow operations
type WorkflowService struct {
	repo     domain.WorkflowRepository
	validate *validator.Validate
}

// NewWorkflowService creates a new WorkflowService
func NewWorkflowService(repo domain.WorkflowRepository) *WorkflowService {
	return &WorkflowService{
		repo:     repo,
		validate: validator.New(),
	}
}

// CreateWorkflowInput represents input for creating a workflow
type CreateWorkflowInput struct {
	Name        string                  `validate:"required,min=1,max=255"`
	Description string                  `validate:"max=1000"`
	Steps       []domain.StepDefinition `validate:"dive"`
}

// UpdateWorkflowInput represents input for updating a workflow
type UpdateWorkflowInput struct {
	Name        *string                  `validate:"omitempty,min=1,max=255"`
	Description *string                  `validate:"omitempty,max=1000"`
	Steps       *[]domain.StepDefinition `validate:"omitempty,dive"`
}

// Create creates a new workflow
func (s *WorkflowService) Create(ctx context.Context, input CreateWorkflowInput) (*domain.Workflow, error) {
	if err := s.validate.Struct(input); err != nil {
		return nil, NewValidationError("invalid workflow input", err)
	}

	workflow := &domain.Workflow{
		ID:          uuid.New(),
		Name:        input.Name,
		Description: input.Description,
		Steps:       input.Steps,
		Status:      domain.WorkflowDraft,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, workflow); err != nil {
		return nil, NewInternalError("failed to create workflow", err)
	}

	return workflow, nil
}

// Get retrieves a workflow by ID
func (s *WorkflowService) Get(ctx context.Context, id string) (*domain.Workflow, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, NewValidationError("invalid workflow ID format", err)
	}

	workflow, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, NewInternalError("failed to get workflow", err)
	}

	if workflow == nil {
		return nil, NewNotFoundError("workflow not found")
	}

	return workflow, nil
}

// List retrieves workflows with pagination
func (s *WorkflowService) List(ctx context.Context, limit, offset int) ([]*domain.Workflow, error) {
	if limit < 0 || limit > 100 {
		return nil, NewValidationError("limit must be between 0 and 100", nil)
	}

	if offset < 0 {
		return nil, NewValidationError("offset must be non-negative", nil)
	}

	opts := domain.ListOptions{
		Limit:   limit,
		Offset:  offset,
		OrderBy: "created_at DESC",
	}

	workflows, err := s.repo.List(ctx, opts)
	if err != nil {
		return nil, NewInternalError("failed to list workflows", err)
	}

	return workflows, nil
}

// Update updates a workflow
func (s *WorkflowService) Update(ctx context.Context, id string, input UpdateWorkflowInput) (*domain.Workflow, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, NewValidationError("invalid workflow ID format", err)
	}

	if err := s.validate.Struct(input); err != nil {
		return nil, NewValidationError("invalid update input", err)
	}

	workflow, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Prevent modification of active workflows
	if workflow.Status == domain.WorkflowActive {
		return nil, NewConflictError("cannot modify active workflow; archive it first")
	}

	// Apply updates
	if input.Name != nil {
		workflow.Name = *input.Name
	}
	if input.Description != nil {
		workflow.Description = *input.Description
	}
	if input.Steps != nil {
		workflow.Steps = *input.Steps
	}
	workflow.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, workflow); err != nil {
		return nil, NewInternalError("failed to update workflow", err)
	}

	return workflow, nil
}

// Activate activates a workflow
func (s *WorkflowService) Activate(ctx context.Context, id string) (*domain.Workflow, error) {
	workflow, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Validate workflow has steps before activation
	if len(workflow.Steps) == 0 {
		return nil, NewValidationError("cannot activate workflow without steps", nil)
	}

	// Validate state transition
	if err := domain.TransitionWorkflow(workflow.Status, domain.WorkflowActive); err != nil {
		return nil, NewConflictError(err.Error())
	}

	workflow.Status = domain.WorkflowActive
	workflow.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, workflow); err != nil {
		return nil, NewInternalError("failed to activate workflow", err)
	}

	return workflow, nil
}

// Archive archives a workflow
func (s *WorkflowService) Archive(ctx context.Context, id string) (*domain.Workflow, error) {
	workflow, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Validate state transition
	if err := domain.TransitionWorkflow(workflow.Status, domain.WorkflowArchived); err != nil {
		return nil, NewConflictError(err.Error())
	}

	workflow.Status = domain.WorkflowArchived
	workflow.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, workflow); err != nil {
		return nil, NewInternalError("failed to archive workflow", err)
	}

	return workflow, nil
}
