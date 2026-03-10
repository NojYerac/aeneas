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

const (
	testNewName = "New Name"
)

// MockWorkflowRepository is a mock implementation of domain.WorkflowRepository
type MockWorkflowRepository struct {
	mock.Mock
}

func (m *MockWorkflowRepository) Create(ctx context.Context, workflow *domain.Workflow) error {
	args := m.Called(ctx, workflow)
	return args.Error(0)
}

func (m *MockWorkflowRepository) Get(ctx context.Context, id string) (*domain.Workflow, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Workflow), args.Error(1)
}

func (m *MockWorkflowRepository) List(ctx context.Context, opts domain.ListOptions) ([]*domain.Workflow, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Workflow), args.Error(1)
}

func (m *MockWorkflowRepository) Update(ctx context.Context, workflow *domain.Workflow) error {
	args := m.Called(ctx, workflow)
	return args.Error(0)
}

func TestWorkflowService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		input := service.CreateWorkflowInput{
			Name:        "Test Workflow",
			Description: "A test workflow",
			Steps: []domain.StepDefinition{
				{Name: "step1", Image: "alpine:latest"},
			},
		}

		repo.On("Create", ctx, mock.AnythingOfType("*domain.Workflow")).Return(nil)

		workflow, err := svc.Create(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, workflow)
		assert.Equal(t, input.Name, workflow.Name)
		assert.Equal(t, input.Description, workflow.Description)
		assert.Equal(t, domain.WorkflowDraft, workflow.Status)
		assert.NotEqual(t, uuid.Nil, workflow.ID)
		repo.AssertExpectations(t)
	})

	t.Run("validation error - empty name", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		input := service.CreateWorkflowInput{
			Name: "",
		}

		workflow, err := svc.Create(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, workflow)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
	})

	t.Run("repository error", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		input := service.CreateWorkflowInput{
			Name: "Test Workflow",
		}

		repo.On("Create", ctx, mock.AnythingOfType("*domain.Workflow")).Return(errors.New("database error"))

		workflow, err := svc.Create(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, workflow)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeInternal, svcErr.Type)
		repo.AssertExpectations(t)
	})
}

func TestWorkflowService_Get(t *testing.T) {
	ctx := context.Background()
	workflowID := uuid.New().String()

	t.Run("successful retrieval", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		expectedWorkflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Test Workflow",
			Status: domain.WorkflowDraft,
		}

		repo.On("Get", ctx, workflowID).Return(expectedWorkflow, nil)

		workflow, err := svc.Get(ctx, workflowID)

		assert.NoError(t, err)
		assert.NotNil(t, workflow)
		assert.Equal(t, expectedWorkflow.ID, workflow.ID)
		repo.AssertExpectations(t)
	})

	t.Run("invalid ID format", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		workflow, err := svc.Get(ctx, "invalid-uuid")

		assert.Error(t, err)
		assert.Nil(t, workflow)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
	})

	t.Run("not found", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		repo.On("Get", ctx, workflowID).Return(nil, nil)

		workflow, err := svc.Get(ctx, workflowID)

		assert.Error(t, err)
		assert.Nil(t, workflow)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeNotFound, svcErr.Type)
		repo.AssertExpectations(t)
	})
}

func TestWorkflowService_Update(t *testing.T) {
	ctx := context.Background()
	workflowID := uuid.New().String()

	t.Run("successful update", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		existingWorkflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Old Name",
			Status: domain.WorkflowDraft,
		}

		newName := testNewName
		input := service.UpdateWorkflowInput{
			Name: &newName,
		}

		repo.On("Get", ctx, workflowID).Return(existingWorkflow, nil)
		repo.On("Update", ctx, mock.AnythingOfType("*domain.Workflow")).Return(nil)

		workflow, err := svc.Update(ctx, workflowID, input)

		assert.NoError(t, err)
		assert.NotNil(t, workflow)
		assert.Equal(t, newName, workflow.Name)
		repo.AssertExpectations(t)
	})

	t.Run("cannot modify active workflow", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		existingWorkflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Active Workflow",
			Status: domain.WorkflowActive,
		}

		newName := testNewName
		input := service.UpdateWorkflowInput{
			Name: &newName,
		}

		repo.On("Get", ctx, workflowID).Return(existingWorkflow, nil)

		workflow, err := svc.Update(ctx, workflowID, input)

		assert.Error(t, err)
		assert.Nil(t, workflow)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeConflict, svcErr.Type)
		repo.AssertExpectations(t)
	})
}

func TestWorkflowService_Activate(t *testing.T) {
	ctx := context.Background()
	workflowID := uuid.New().String()

	t.Run("successful activation", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		existingWorkflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Test Workflow",
			Status: domain.WorkflowDraft,
			Steps: []domain.StepDefinition{
				{Name: "step1", Image: "alpine:latest"},
			},
		}

		repo.On("Get", ctx, workflowID).Return(existingWorkflow, nil)
		repo.On("Update", ctx, mock.AnythingOfType("*domain.Workflow")).Return(nil)

		workflow, err := svc.Activate(ctx, workflowID)

		assert.NoError(t, err)
		assert.NotNil(t, workflow)
		assert.Equal(t, domain.WorkflowActive, workflow.Status)
		repo.AssertExpectations(t)
	})

	t.Run("cannot activate workflow without steps", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		existingWorkflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Test Workflow",
			Status: domain.WorkflowDraft,
			Steps:  []domain.StepDefinition{},
		}

		repo.On("Get", ctx, workflowID).Return(existingWorkflow, nil)

		workflow, err := svc.Activate(ctx, workflowID)

		assert.Error(t, err)
		assert.Nil(t, workflow)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
		repo.AssertExpectations(t)
	})

	t.Run("invalid state transition", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		existingWorkflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Test Workflow",
			Status: domain.WorkflowArchived,
			Steps: []domain.StepDefinition{
				{Name: "step1", Image: "alpine:latest"},
			},
		}

		repo.On("Get", ctx, workflowID).Return(existingWorkflow, nil)

		workflow, err := svc.Activate(ctx, workflowID)

		assert.Error(t, err)
		assert.Nil(t, workflow)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeConflict, svcErr.Type)
		repo.AssertExpectations(t)
	})
}

func TestWorkflowService_Archive(t *testing.T) {
	ctx := context.Background()
	workflowID := uuid.New().String()

	t.Run("successful archive", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		existingWorkflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Test Workflow",
			Status: domain.WorkflowActive,
		}

		repo.On("Get", ctx, workflowID).Return(existingWorkflow, nil)
		repo.On("Update", ctx, mock.AnythingOfType("*domain.Workflow")).Return(nil)

		workflow, err := svc.Archive(ctx, workflowID)

		assert.NoError(t, err)
		assert.NotNil(t, workflow)
		assert.Equal(t, domain.WorkflowArchived, workflow.Status)
		repo.AssertExpectations(t)
	})

	t.Run("invalid state transition", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		existingWorkflow := &domain.Workflow{
			ID:     uuid.MustParse(workflowID),
			Name:   "Test Workflow",
			Status: domain.WorkflowDraft,
		}

		repo.On("Get", ctx, workflowID).Return(existingWorkflow, nil)

		workflow, err := svc.Archive(ctx, workflowID)

		assert.Error(t, err)
		assert.Nil(t, workflow)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeConflict, svcErr.Type)
		repo.AssertExpectations(t)
	})
}

func TestWorkflowService_List(t *testing.T) {
	ctx := context.Background()

	t.Run("successful list", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		expectedWorkflows := []*domain.Workflow{
			{ID: uuid.New(), Name: "Workflow 1", Status: domain.WorkflowDraft, CreatedAt: time.Now()},
			{ID: uuid.New(), Name: "Workflow 2", Status: domain.WorkflowActive, CreatedAt: time.Now()},
		}

		repo.On("List", ctx, mock.AnythingOfType("domain.ListOptions")).Return(expectedWorkflows, nil)

		workflows, err := svc.List(ctx, 10, 0)

		assert.NoError(t, err)
		assert.NotNil(t, workflows)
		assert.Len(t, workflows, 2)
		repo.AssertExpectations(t)
	})

	t.Run("invalid limit", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		workflows, err := svc.List(ctx, 150, 0)

		assert.Error(t, err)
		assert.Nil(t, workflows)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
	})

	t.Run("invalid offset", func(t *testing.T) {
		repo := new(MockWorkflowRepository)
		svc := service.NewWorkflowService(repo)

		workflows, err := svc.List(ctx, 10, -1)

		assert.Error(t, err)
		assert.Nil(t, workflows)
		var svcErr *service.ServiceError
		assert.ErrorAs(t, err, &svcErr)
		assert.Equal(t, service.ErrorTypeValidation, svcErr.Type)
	})
}
