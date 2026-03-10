package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockExecutionRepository is a mock implementation of domain.ExecutionRepository
type MockExecutionRepository struct {
	mock.Mock
}

func (m *MockExecutionRepository) Create(ctx context.Context, execution *domain.Execution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockExecutionRepository) Get(ctx context.Context, id string) (*domain.Execution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Execution), args.Error(1)
}

func (m *MockExecutionRepository) ListByWorkflow(
	ctx context.Context,
	workflowID string,
	opts domain.ListOptions,
) ([]*domain.Execution, error) {
	args := m.Called(ctx, workflowID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Execution), args.Error(1)
}

func (m *MockExecutionRepository) UpdateStatus(ctx context.Context, id string, status domain.ExecutionStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

// MockStepExecutionRepository is a mock implementation of domain.StepExecutionRepository
type MockStepExecutionRepository struct {
	mock.Mock
}

func (m *MockStepExecutionRepository) Create(ctx context.Context, stepExecution *domain.StepExecution) error {
	args := m.Called(ctx, stepExecution)
	return args.Error(0)
}

func (m *MockStepExecutionRepository) ListByExecution(
	ctx context.Context,
	executionID string,
) ([]*domain.StepExecution, error) {
	args := m.Called(ctx, executionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.StepExecution), args.Error(1)
}

func (m *MockStepExecutionRepository) UpdateStatus(
	ctx context.Context,
	id string,
	status domain.StepExecutionStatus,
	exitCode *int,
) error {
	args := m.Called(ctx, id, status, exitCode)
	return args.Error(0)
}

func TestExecutionService_Trigger(t *testing.T) {
	ctx := context.Background()
	workflowID := uuid.New().String()

	t.Run("successful trigger", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		workflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Test Workflow",
			Status: domain.WorkflowActive,
			Steps: []domain.StepDefinition{
				{Name: "step1", Image: "alpine:latest"},
				{Name: "step2", Image: "ubuntu:latest"},
			},
		}

		workflowRepo.On("Get", ctx, workflowID).Return(workflow, nil)
		executionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Execution")).Return(nil)
		stepExecutionRepo.On("Create", ctx, mock.AnythingOfType("*domain.StepExecution")).Return(nil).Times(2)

		execution, err := svc.Trigger(ctx, workflowID)

		assert.NoError(t, err)
		assert.NotNil(t, execution)
		assert.Equal(t, workflow.ID, execution.WorkflowID)
		assert.Equal(t, domain.ExecutionPending, execution.Status)
		assert.NotNil(t, execution.StartedAt)
		workflowRepo.AssertExpectations(t)
		executionRepo.AssertExpectations(t)
		stepExecutionRepo.AssertExpectations(t)
	})

	t.Run("invalid workflow ID format", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		execution, err := svc.Trigger(ctx, "invalid-uuid")

		assert.Error(t, err)
		assert.Nil(t, execution)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
	})

	t.Run("workflow not found", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		workflowRepo.On("Get", ctx, workflowID).Return(nil, nil)

		execution, err := svc.Trigger(ctx, workflowID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeNotFound, svcErr.Type)
		workflowRepo.AssertExpectations(t)
	})

	t.Run("cannot execute inactive workflow", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		workflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Test Workflow",
			Status: domain.WorkflowDraft,
		}

		workflowRepo.On("Get", ctx, workflowID).Return(workflow, nil)

		execution, err := svc.Trigger(ctx, workflowID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeConflict, svcErr.Type)
		workflowRepo.AssertExpectations(t)
	})
}

func TestExecutionService_GetWithSteps(t *testing.T) {
	ctx := context.Background()
	executionID := uuid.New().String()

	t.Run("successful retrieval", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		now := time.Now()
		execution := &domain.Execution{
			ID:        uuid.MustParse(executionID),
			Status:    domain.ExecutionRunning,
			StartedAt: &now,
		}

		steps := []*domain.StepExecution{
			{ID: uuid.New(), ExecutionID: execution.ID, StepName: "step1", Status: domain.StepExecutionSucceeded},
			{ID: uuid.New(), ExecutionID: execution.ID, StepName: "step2", Status: domain.StepExecutionRunning},
		}

		executionRepo.On("Get", ctx, executionID).Return(execution, nil)
		stepExecutionRepo.On("ListByExecution", ctx, executionID).Return(steps, nil)

		retrievedExecution, retrievedSteps, err := svc.GetWithSteps(ctx, executionID)

		assert.NoError(t, err)
		assert.NotNil(t, retrievedExecution)
		assert.NotNil(t, retrievedSteps)
		assert.Equal(t, execution.ID, retrievedExecution.ID)
		assert.Len(t, retrievedSteps, 2)
		executionRepo.AssertExpectations(t)
		stepExecutionRepo.AssertExpectations(t)
	})

	t.Run("invalid execution ID format", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		execution, steps, err := svc.GetWithSteps(ctx, "invalid-uuid")

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Nil(t, steps)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
	})

	t.Run("execution not found", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		executionRepo.On("Get", ctx, executionID).Return(nil, nil)

		execution, steps, err := svc.GetWithSteps(ctx, executionID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Nil(t, steps)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeNotFound, svcErr.Type)
		executionRepo.AssertExpectations(t)
	})
}

func TestExecutionService_ListByWorkflow(t *testing.T) {
	ctx := context.Background()
	workflowID := uuid.New().String()

	t.Run("successful list", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		now := time.Now()
		expectedExecutions := []*domain.Execution{
			{ID: uuid.New(), WorkflowID: uuid.MustParse(workflowID), Status: domain.ExecutionSucceeded, StartedAt: &now},
			{ID: uuid.New(), WorkflowID: uuid.MustParse(workflowID), Status: domain.ExecutionRunning, StartedAt: &now},
		}

		executionRepo.On(
			"ListByWorkflow",
			ctx,
			workflowID,
			mock.AnythingOfType("domain.ListOptions"),
		).Return(expectedExecutions, nil)

		executions, err := svc.ListByWorkflow(ctx, workflowID, 10, 0)

		assert.NoError(t, err)
		assert.NotNil(t, executions)
		assert.Len(t, executions, 2)
		executionRepo.AssertExpectations(t)
	})

	t.Run("invalid workflow ID format", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		executions, err := svc.ListByWorkflow(ctx, "invalid-uuid", 10, 0)

		assert.Error(t, err)
		assert.Nil(t, executions)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
	})

	t.Run("invalid limit", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		executions, err := svc.ListByWorkflow(ctx, workflowID, 150, 0)

		assert.Error(t, err)
		assert.Nil(t, executions)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
	})
}

func TestExecutionService_Cancel(t *testing.T) {
	ctx := context.Background()
	executionID := uuid.New().String()

	t.Run("successful cancel from pending", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		execution := &domain.Execution{
			ID:     uuid.MustParse(executionID),
			Status: domain.ExecutionPending,
		}

		executionRepo.On("Get", ctx, executionID).Return(execution, nil)
		executionRepo.On("UpdateStatus", ctx, executionID, domain.ExecutionCanceled).Return(nil)

		err := svc.Cancel(ctx, executionID)

		assert.NoError(t, err)
		executionRepo.AssertExpectations(t)
	})

	t.Run("successful cancel from running", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		execution := &domain.Execution{
			ID:     uuid.MustParse(executionID),
			Status: domain.ExecutionRunning,
		}

		executionRepo.On("Get", ctx, executionID).Return(execution, nil)
		executionRepo.On("UpdateStatus", ctx, executionID, domain.ExecutionCanceled).Return(nil)

		err := svc.Cancel(ctx, executionID)

		assert.NoError(t, err)
		executionRepo.AssertExpectations(t)
	})

	t.Run("invalid execution ID format", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		err := svc.Cancel(ctx, "invalid-uuid")

		assert.Error(t, err)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
	})

	t.Run("execution not found", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		executionRepo.On("Get", ctx, executionID).Return(nil, nil)

		err := svc.Cancel(ctx, executionID)

		assert.Error(t, err)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeNotFound, svcErr.Type)
		executionRepo.AssertExpectations(t)
	})

	t.Run("cannot cancel completed execution", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		execution := &domain.Execution{
			ID:     uuid.MustParse(executionID),
			Status: domain.ExecutionSucceeded,
		}

		executionRepo.On("Get", ctx, executionID).Return(execution, nil)

		err := svc.Cancel(ctx, executionID)

		assert.Error(t, err)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeConflict, svcErr.Type)
		executionRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		workflowRepo := new(MockWorkflowRepository)
		executionRepo := new(MockExecutionRepository)
		stepExecutionRepo := new(MockStepExecutionRepository)
		svc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		execution := &domain.Execution{
			ID:     uuid.MustParse(executionID),
			Status: domain.ExecutionRunning,
		}

		executionRepo.On("Get", ctx, executionID).Return(execution, nil)
		executionRepo.On("UpdateStatus", ctx, executionID, domain.ExecutionCanceled).Return(errors.New("database error"))

		err := svc.Cancel(ctx, executionID)

		assert.Error(t, err)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeInternal, svcErr.Type)
		executionRepo.AssertExpectations(t)
	})
}
