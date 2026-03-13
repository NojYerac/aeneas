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

var _ = Describe("WorkflowService", func() {
	var (
		wfSvc    *WorkflowService
		mockRepo *mockdomain.MockWorkflowRepository
		ctx      context.Context
		err      error
	)
	BeforeEach(func() {
		ctx = context.Background()
		mockRepo = &mockdomain.MockWorkflowRepository{}
		mockSvc := service.NewWorkflowService(mockRepo)
		wfSvc = NewWorkflowService(mockSvc)
	})
	AfterEach(func() {
		mockRepo.AssertExpectations(GinkgoT())
	})
	Describe("ActivateWorkflow", func() {
		var (
			req *pb.ActivateWorkflowRequest
			res *pb.WorkflowResponse
		)
		JustBeforeEach(func() {
			res, err = wfSvc.ActivateWorkflow(ctx, req)
		})
		When("the workflow exists, has steps, and is in draft status", func() {
			BeforeEach(func() {
				req = &pb.ActivateWorkflowRequest{WorkflowId: uuid.New().String()}
				mockRepo.
					On("Get", ctxMatcher, req.GetWorkflowId()).
					Return(&domain.Workflow{
						ID:     uuid.MustParse(req.GetWorkflowId()),
						Status: domain.WorkflowDraft,
						Steps:  []domain.StepDefinition{{Name: "step-one"}},
					}, nil)
				mockRepo.On("Update", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(nil)
			})
			It("should return the workflow", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.GetId()).To(Equal(req.GetWorkflowId()))
			})
		})
	})
	Describe("ArchiveWorkflow", func() {
		var (
			req *pb.ArchiveWorkflowRequest
			res *pb.WorkflowResponse
		)
		JustBeforeEach(func() {
			res, err = wfSvc.ArchiveWorkflow(ctx, req)
		})
		When("the workflow exists and is active", func() {
			BeforeEach(func() {
				req = &pb.ArchiveWorkflowRequest{WorkflowId: uuid.New().String()}
				mockRepo.
					On("Get", ctxMatcher, req.GetWorkflowId()).
					Return(&domain.Workflow{
						ID:     uuid.MustParse(req.GetWorkflowId()),
						Status: domain.WorkflowActive,
					}, nil)
				mockRepo.On("Update", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(nil)
			})
			It("should return the workflow", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.GetId()).To(Equal(req.GetWorkflowId()))
			})
		})
	})
	Describe("CreateWorkflow", func() {
		var (
			req *pb.CreateWorkflowRequest
			res *pb.CreateWorkflowResponse
		)
		JustBeforeEach(func() {
			res, err = wfSvc.CreateWorkflow(ctx, req)
		})
		When("the input is valid", func() {
			BeforeEach(func() {
				desc := "A workflow for testing"
				req = &pb.CreateWorkflowRequest{
					Name:        "Test Workflow",
					Description: &desc,
					Steps:       []*pb.StepDefinition{{Name: "step-one"}},
				}
				mockRepo.
					On("Create", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).
					Return(nil)
			})
			It("should return the new workflow ID", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.GetWorkflowId()).ToNot(BeEmpty())
			})
		})
	})
	Describe("GetWorkflow", func() {
		var (
			req *pb.GetWorkflowByIdRequest
			res *pb.WorkflowResponse
		)
		JustBeforeEach(func() {
			res, err = wfSvc.GetWorkflow(ctx, req)
		})
		When("the workflow exists", func() {
			BeforeEach(func() {
				req = &pb.GetWorkflowByIdRequest{WorkflowId: uuid.New().String()}
				mockRepo.
					On("Get", ctxMatcher, req.GetWorkflowId()).
					Return(&domain.Workflow{
						ID:          uuid.MustParse(req.GetWorkflowId()),
						Name:        "Test Workflow",
						Description: "A workflow for testing",
						Status:      domain.WorkflowDraft,
						Steps:       []domain.StepDefinition{{Name: "step-one"}},
					}, nil)
			})
			It("should return the workflow", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.GetId()).To(Equal(req.GetWorkflowId()))
				Expect(res.GetName()).To(Equal("Test Workflow"))
				Expect(res.GetDescription()).To(Equal("A workflow for testing"))
				Expect(res.GetStatus()).To(Equal(pb.WorkflowStatus_WORKFLOW_STATUS_INACTIVE))
				Expect(res.GetSteps()).To(HaveLen(1))
				Expect(res.GetSteps()[0].GetName()).To(Equal("step-one"))
			})
		})
	})
	Describe("ListWorkflows", func() {
		var (
			req *pb.ListWorkflowsRequest
			res *pb.ListWorkflowsResponse
		)
		JustBeforeEach(func() {
			res, err = wfSvc.ListWorkflows(ctx, req)
		})
		When("there are workflows in the repository", func() {
			BeforeEach(func() {
				req = &pb.ListWorkflowsRequest{
					Limit:  2,
					Offset: 0,
				}
				mockRepo.
					On("List", ctxMatcher, mock.AnythingOfType("domain.ListOptions")).
					Return([]*domain.Workflow{
						{
							ID:          uuid.New(),
							Name:        "Workflow One",
							Description: "First workflow",
							Status:      domain.WorkflowDraft,
							Steps:       []domain.StepDefinition{{Name: "step-one"}},
						},
						{
							ID:          uuid.New(),
							Name:        "Workflow Two",
							Description: "Second workflow",
							Status:      domain.WorkflowActive,
							Steps:       []domain.StepDefinition{{Name: "step-two"}},
						},
					}, nil)
			})
			It("should return the list of workflows", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.GetWorkflows()).To(HaveLen(2))
			})
		})
	})
	Describe("UpdateWorkflow", func() {
		var (
			req *pb.UpdateWorkflowRequest
			res *pb.WorkflowResponse
		)
		JustBeforeEach(func() {
			res, err = wfSvc.UpdateWorkflow(ctx, req)
		})
		When("the workflow exists and input is valid", func() {
			BeforeEach(func() {
				desc := "Updated description"
				req = &pb.UpdateWorkflowRequest{
					WorkflowId:  uuid.New().String(),
					Description: &desc,
				}
				mockRepo.On("Get", ctxMatcher, req.GetWorkflowId()).Return(&domain.Workflow{
					ID:          uuid.MustParse(req.GetWorkflowId()),
					Name:        "Test Workflow",
					Description: "A workflow for testing",
					Status:      domain.WorkflowDraft,
					Steps:       []domain.StepDefinition{{Name: "step-one"}},
				}, nil)
				mockRepo.On("Update", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(nil)
			})
			It("should return the updated workflow", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.GetId()).To(Equal(req.GetWorkflowId()))
				Expect(res.GetDescription()).To(Equal("Updated description"))
			})
		})
	})
})
