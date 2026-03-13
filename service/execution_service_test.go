package service_test

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/service"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	mockrepo "github.com/nojyerac/aeneas/mocks/domain"
)

const invalidID = "invalid-uuid"

var ctxMatcher = mock.MatchedBy(func(arg any) bool {
	_, ok := arg.(context.Context)
	return ok
})

var _ = Describe("ExecutionService", func() {
	var (
		executionRepo *mockrepo.MockExecutionRepository
		workflowRepo  *mockrepo.MockWorkflowRepository
		stepRepo      *mockrepo.MockStepExecutionRepository
		svc           *service.ExecutionService
		ctx           context.Context
	)
	BeforeEach(func() {
		executionRepo = new(mockrepo.MockExecutionRepository)
		workflowRepo = new(mockrepo.MockWorkflowRepository)
		stepRepo = new(mockrepo.MockStepExecutionRepository)
		svc = service.NewExecutionService(workflowRepo, executionRepo, stepRepo)
		ctx = context.Background()
	})

	Describe("Trigger", func() {
		var (
			workflowID string
			exe        *domain.Execution
			err        error
		)
		JustBeforeEach(func() {
			exe, err = svc.Trigger(ctx, workflowID)
		})
		When("triggering a valid workflow", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				workflow := &domain.Workflow{
					ID:     uuid.MustParse(workflowID),
					Name:   "Test Workflow",
					Status: domain.WorkflowActive,
					Steps: []domain.StepDefinition{
						{Name: "step1", Image: "alpine:latest"},
						{Name: "step2", Image: "ubuntu:latest"},
					},
				}
				workflowRepo.On("Get", ctxMatcher, workflowID).Return(workflow, nil)
				executionRepo.On("Create", ctxMatcher, mock.AnythingOfType("*domain.Execution")).Return(nil)
				stepRepo.On("Create", ctxMatcher, mock.AnythingOfType("*domain.StepExecution")).Return(nil).Times(2)
			})
			It("should trigger successfully", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(exe).NotTo(BeNil())
				Expect(exe.WorkflowID.String()).To(Equal(workflowID))
				Expect(exe.Status).To(Equal(domain.ExecutionPending))
				Expect(exe.StartedAt).NotTo(BeNil())
			})
		})
		When("workflow ID is invalid", func() {
			BeforeEach(func() {
				workflowID = invalidID
			})
			It("should return a validation error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeValidation))
			})
		})
		When("workflow is not found", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				workflowRepo.On("Get", ctxMatcher, workflowID).Return(nil, nil)
			})
			It("should return a not found error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeNotFound))
			})
		})
		When("workflow is inactive", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				workflow := &domain.Workflow{
					ID:     uuid.MustParse(workflowID),
					Name:   "Test Workflow",
					Status: domain.WorkflowDraft,
				}
				workflowRepo.On("Get", ctxMatcher, workflowID).Return(workflow, nil)
			})
			It("should return a conflict error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeConflict))
			})
		})
	})
	Describe("GetWithSteps", func() {
		var (
			executionID string
			execution   *domain.Execution
			steps       []*domain.StepExecution
			err         error
		)
		JustBeforeEach(func() {
			execution, steps, err = svc.GetWithSteps(ctx, executionID)
		})
		When("retrieving an existing execution", func() {
			BeforeEach(func() {
				executionID = uuid.New().String()
				execution = &domain.Execution{
					ID:     uuid.MustParse(executionID),
					Status: domain.ExecutionRunning,
				}
				steps = []*domain.StepExecution{
					{
						ID:          uuid.New(),
						ExecutionID: uuid.MustParse(executionID),
						StepName:    "step1",
						Status:      domain.StepExecutionSucceeded,
					},
					{
						ID:          uuid.New(),
						ExecutionID: uuid.MustParse(executionID),
						StepName:    "step2",
						Status:      domain.StepExecutionRunning,
					},
				}
				executionRepo.On("Get", ctxMatcher, executionID).Return(execution, nil)
				stepRepo.On("ListByExecution", ctxMatcher, executionID).Return(steps, nil)
			})
			It("should retrieve successfully", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(execution).NotTo(BeNil())
				Expect(steps).NotTo(BeNil())
				Expect(execution.ID.String()).To(Equal(executionID))
				Expect(steps).To(HaveLen(2))
			})
		})
		When("execution ID is invalid", func() {
			BeforeEach(func() {
				executionID = invalidID
			})
			It("should return a validation error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeValidation))
			})
		})
		When("execution is not found", func() {
			BeforeEach(func() {
				executionID = uuid.New().String()
				executionRepo.On("Get", ctxMatcher, executionID).Return(nil, nil)
			})
			It("should return a not found error", func() {
				Expect(err).To(HaveOccurred())
				Expect(execution).To(BeNil())
				Expect(steps).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeNotFound))
			})
		})
	})
	Describe("ListByWorkflow", func() {
		var (
			workflowID string
			executions []*domain.Execution
			err        error
		)
		JustBeforeEach(func() {
			executions, err = svc.ListByWorkflow(ctx, workflowID, 10, 0)
		})
		When("listing executions for a workflow", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				executions = []*domain.Execution{
					{ID: uuid.New(), WorkflowID: uuid.MustParse(workflowID), Status: domain.ExecutionSucceeded},
					{ID: uuid.New(), WorkflowID: uuid.MustParse(workflowID), Status: domain.ExecutionRunning},
				}
				executionRepo.On(
					"ListByWorkflow",
					ctxMatcher,
					workflowID,
					mock.AnythingOfType("domain.ListOptions"),
				).Return(executions, nil)
			})
			It("should list successfully", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executions).NotTo(BeNil())
				Expect(executions).To(HaveLen(2))
				for _, exe := range executions {
					Expect(exe.WorkflowID.String()).To(Equal(workflowID))
				}
			})
		})
		When("workflow ID is invalid", func() {
			BeforeEach(func() {
				workflowID = invalidID
			})
			It("should return a validation error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeValidation))
			})
		})
		When("repository returns an error", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				executionRepo.On(
					"ListByWorkflow",
					ctxMatcher,
					workflowID,
					mock.AnythingOfType("domain.ListOptions"),
				).Return(nil, errors.New("database error"))
			})
			It("should return an internal error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeInternal))
			})
		})
	})
	Describe("Cancel", func() {
		var (
			executionID string
			err         error
		)
		JustBeforeEach(func() {
			err = svc.Cancel(ctx, executionID)
		})
		When("canceling a pending execution", func() {
			BeforeEach(func() {
				executionID = uuid.New().String()
				execution := &domain.Execution{
					ID:     uuid.MustParse(executionID),
					Status: domain.ExecutionPending,
				}
				executionRepo.On("Get", ctxMatcher, executionID).Return(execution, nil)
				executionRepo.On("UpdateStatus", ctxMatcher, executionID, domain.ExecutionCanceled).Return(nil)
			})
			It("should cancel successfully", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
		When("canceling a running execution", func() {
			BeforeEach(func() {
				executionID = uuid.New().String()
				execution := &domain.Execution{
					ID:     uuid.MustParse(executionID),
					Status: domain.ExecutionRunning,
				}
				executionRepo.On("Get", ctxMatcher, executionID).Return(execution, nil)
				executionRepo.On("UpdateStatus", ctxMatcher, executionID, domain.ExecutionCanceled).Return(nil)
			})
			It("should cancel successfully", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
		When("execution ID is invalid", func() {
			BeforeEach(func() {
				executionID = invalidID
			})
			It("should return a validation error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeValidation))
			})
		})
		When("execution is not found", func() {
			BeforeEach(func() {
				executionID = uuid.New().String()
				executionRepo.On("Get", ctxMatcher, executionID).Return(nil, nil)
			})
			It("should return a not found error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeNotFound))
			})
		})
		When("canceling a completed execution", func() {
			BeforeEach(func() {
				executionID = uuid.New().String()
				execution := &domain.Execution{
					ID:     uuid.MustParse(executionID),
					Status: domain.ExecutionSucceeded,
				}
				executionRepo.On("Get", ctxMatcher, executionID).Return(execution, nil)
			})
			It("should return a conflict error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeConflict))
			})
		})
		When("repository returns an error", func() {
			BeforeEach(func() {
				executionID = uuid.New().String()
				execution := &domain.Execution{
					ID:     uuid.MustParse(executionID),
					Status: domain.ExecutionRunning,
				}
				executionRepo.On("Get", ctxMatcher, executionID).Return(execution, nil)
				executionRepo.On(
					"UpdateStatus",
					ctxMatcher,
					executionID,
					domain.ExecutionCanceled,
				).Return(errors.New("database error"))
			})
			It("should return an internal error", func() {
				Expect(err).To(HaveOccurred())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeInternal))
			})
		})
	})
})
