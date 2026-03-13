package rpc_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
	mockdomain "github.com/nojyerac/aeneas/mocks/domain"
	pb "github.com/nojyerac/aeneas/pb/workflow"
	"github.com/nojyerac/aeneas/service"
	. "github.com/nojyerac/aeneas/transport/rpc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

var _ = Describe("ExecutionService", func() {
	var (
		exSvc        *ExecutionService
		mockWfRepo   *mockdomain.MockWorkflowRepository
		mockExRepo   *mockdomain.MockExecutionRepository
		mockStepRepo *mockdomain.MockStepExecutionRepository
		ctx          context.Context
		err          error
	)
	BeforeEach(func() {
		ctx = context.Background()
		mockWfRepo = &mockdomain.MockWorkflowRepository{}
		mockExRepo = &mockdomain.MockExecutionRepository{}
		mockStepRepo = &mockdomain.MockStepExecutionRepository{}
		mockSvc := service.NewExecutionService(mockWfRepo, mockExRepo, mockStepRepo)
		exSvc = NewExecutionService(mockSvc)
	})
	AfterEach(func() {
		mockWfRepo.AssertExpectations(GinkgoT())
		mockExRepo.AssertExpectations(GinkgoT())
		mockStepRepo.AssertExpectations(GinkgoT())
	})
	Describe("GetExecution", func() {
		var (
			req *pb.GetExecutionRequest
			res *pb.ExecutionResponse
		)
		JustBeforeEach(func() {
			res, err = exSvc.GetExecution(ctx, req)
		})
		When("the execution exists", func() {
			BeforeEach(func() {
				req = &pb.GetExecutionRequest{ExecutionId: uuid.New().String()}
				mockExRepo.
					On("Get", ctxMatcher, req.GetExecutionId()).
					Return(&domain.Execution{
						ID:         uuid.MustParse(req.GetExecutionId()),
						WorkflowID: uuid.New(),
						Status:     domain.ExecutionRunning,
					}, nil)
				mockStepRepo.
					On("ListByExecution", ctxMatcher, req.GetExecutionId()).
					Return([]*domain.StepExecution{
						{
							ID:          uuid.New(),
							ExecutionID: uuid.MustParse(req.GetExecutionId()),
							StepName:    "step-one",
							Status:      domain.StepExecutionRunning,
						},
					}, nil)
			})
			It("should return the execution with steps", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.GetId()).To(Equal(req.GetExecutionId()))
				Expect(res.GetSteps()).To(HaveLen(1))
				Expect(res.GetSteps()[0].GetStepName()).To(Equal("step-one"))
				Expect(res.GetSteps()[0].GetStatus()).To(Equal(pb.StepExecutionStatus_STEP_EXECUTION_STATUS_RUNNING))
			})
		})
	})
	Describe("CancelExecution", func() {
		var (
			req *pb.CancelExecutionRequest
			res *pb.CancelExecutionResponse
		)
		JustBeforeEach(func() {
			res, err = exSvc.CancelExecution(ctx, req)
		})
		When("the execution exists and is running", func() {
			BeforeEach(func() {
				req = &pb.CancelExecutionRequest{ExecutionId: uuid.New().String()}
				mockExRepo.
					On("Get", ctxMatcher, req.GetExecutionId()).
					Return(&domain.Execution{
						ID:         uuid.MustParse(req.GetExecutionId()),
						WorkflowID: uuid.New(),
						Status:     domain.ExecutionRunning,
					}, nil)
				mockExRepo.
					On("UpdateStatus", ctxMatcher, req.GetExecutionId(), domain.ExecutionCanceled).
					Return(nil)
			})
			It("should cancel the execution", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
			})
		})
	})
	Describe("ExecuteWorkflow", func() {
		var (
			req *pb.ExecuteWorkflowRequest
			res *pb.ExecutionResponse
		)
		JustBeforeEach(func() {
			res, err = exSvc.ExecuteWorkflow(ctx, req)
		})
		When("the workflow exists and is active", func() {
			BeforeEach(func() {
				req = &pb.ExecuteWorkflowRequest{WorkflowId: uuid.New().String()}
				mockWfRepo.
					On("Get", ctxMatcher, req.GetWorkflowId()).
					Return(&domain.Workflow{
						ID:     uuid.MustParse(req.GetWorkflowId()),
						Status: domain.WorkflowActive,
						Steps:  []domain.StepDefinition{{Name: "step-one"}},
					}, nil)
				mockExRepo.
					On("Create", ctxMatcher, mock.AnythingOfType("*domain.Execution")).
					Return(nil)
				mockStepRepo.
					On("Create", ctxMatcher, mock.AnythingOfType("*domain.StepExecution")).
					Return(nil)
			})
			It("should create a new execution with steps", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.GetId()).ToNot(BeEmpty())
			})
		})
	})
	Describe("ListWorkflowExecutions", func() {
		var (
			req *pb.ListWorkflowExecutionsRequest
			res *pb.ListWorkflowExecutionsResponse
		)
		JustBeforeEach(func() {
			res, err = exSvc.ListWorkflowExecutions(ctx, req)
		})
		When("the workflow exists and has executions", func() {
			BeforeEach(func() {
				req = &pb.ListWorkflowExecutionsRequest{WorkflowId: uuid.New().String()}
				mockExRepo.
					On("ListByWorkflow", ctxMatcher, req.GetWorkflowId(), mock.AnythingOfType("domain.ListOptions")).
					Return([]*domain.Execution{
						{
							ID:         uuid.New(),
							WorkflowID: uuid.MustParse(req.GetWorkflowId()),
							Status:     domain.ExecutionSucceeded,
						},
					}, nil)
			})
			It("should return the list of executions", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.GetExecutions()).To(HaveLen(1))
				Expect(res.GetExecutions()[0].GetWorkflowId()).To(Equal(req.GetWorkflowId()))
			})
		})
	})
})
