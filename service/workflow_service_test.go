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

const testNewName = "New Name"

var _ = Describe("WorkflowService", func() {
	var (
		repo *mockrepo.MockWorkflowRepository
		svc  *service.WorkflowService
		ctx  context.Context
	)
	BeforeEach(func() {
		repo = &mockrepo.MockWorkflowRepository{}
		svc = service.NewWorkflowService(repo)
		ctx = context.Background()
	})
	AfterEach(func() {
		repo.AssertExpectations(GinkgoT())
	})
	Describe("Create", func() {
		var (
			input    service.CreateWorkflowInput
			workflow *domain.Workflow
			err      error
		)
		JustBeforeEach(func() {
			workflow, err = svc.Create(ctx, input)
		})
		When("input is valid", func() {
			BeforeEach(func() {
				input = service.CreateWorkflowInput{
					Name:        "Test Workflow",
					Description: "A test workflow",
					Steps: []domain.StepDefinition{
						{Name: "step1", Image: "alpine:latest"},
					},
				}

				repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Workflow")).Return(nil).Once()
			})
			It("successful creation", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(workflow).NotTo(BeNil())
				Expect(workflow.Name).To(Equal(input.Name))
				Expect(workflow.Description).To(Equal(input.Description))
				Expect(workflow.Status).To(Equal(domain.WorkflowDraft))
				Expect(workflow.ID).NotTo(Equal(uuid.Nil))
			})
		})
		When("input is invalid", func() {
			BeforeEach(func() {
				input = service.CreateWorkflowInput{
					Name: "",
				}
			})

			It("validation error - empty name", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflow).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeValidation))
			})
		})
		When("repository returns an error", func() {
			BeforeEach(func() {
				input = service.CreateWorkflowInput{
					Name: "Test Workflow",
				}
				repo.On("Create", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(errors.New("database error")).Once()
			})
			It("repository error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("database error"))
				Expect(workflow).To(BeNil())
			})
		})
	})
	Describe("Get", func() {
		var (
			workflowID string
			workflow   *domain.Workflow
			err        error
		)
		JustBeforeEach(func() {
			workflow, err = svc.Get(ctx, workflowID)
		})
		When("workflow exists", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				expectedWorkflow := &domain.Workflow{
					ID:     uuid.MustParse(workflowID),
					Name:   "Test Workflow",
					Status: domain.WorkflowDraft,
				}
				repo.On("Get", ctxMatcher, workflowID).Return(expectedWorkflow, nil).Once()
			})
			It("successful retrieval", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(workflow).NotTo(BeNil())
				Expect(workflow.ID.String()).To(Equal(workflowID))
				Expect(workflow.Name).To(Equal("Test Workflow"))
				Expect(workflow.Status).To(Equal(domain.WorkflowDraft))
			})
		})
		When("invalid ID format", func() {
			BeforeEach(func() {
				workflowID = "invalid-uuid"
			})
			It("validation error", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflow).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeValidation))
			})
		})
		//nolint:dupl // Similar test case for not found error
		When("workflow not found", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				repo.On("Get", ctxMatcher, workflowID).Return(nil, nil).Once()
			})
			It("not found error", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflow).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeNotFound))
			})
		})
	})
	Describe("Update", func() {
		var (
			workflowID string
			input      service.UpdateWorkflowInput
			workflow   *domain.Workflow
			err        error
		)
		JustBeforeEach(func() {
			workflow, err = svc.Update(ctx, workflowID, input)
		})
		When("workflow exists and input is valid", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				existingWorkflow := &domain.Workflow{
					ID:     uuid.MustParse(workflowID),
					Name:   "Old Name",
					Status: domain.WorkflowDraft,
				}
				newName := testNewName
				input = service.UpdateWorkflowInput{
					Name: &newName,
				}

				repo.On("Get", ctxMatcher, workflowID).Return(existingWorkflow, nil).Once()
				repo.On("Update", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(nil).Once()
			})
			It("successful update", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(workflow).NotTo(BeNil())
				Expect(workflow.ID.String()).To(Equal(workflowID))
				Expect(workflow.Name).To(Equal(testNewName))
			})
		})
		When("input is invalid", func() {
			BeforeEach(func() {
				badName := ""
				input = service.UpdateWorkflowInput{
					Name: &badName,
				}
			})
			It("validation error", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflow).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeValidation))
			})
		})
		When("workflow does not exist", func() {
			BeforeEach(func() {
				tnn := testNewName
				workflowID = uuid.New().String()
				input = service.UpdateWorkflowInput{
					Name: &tnn,
				}
				repo.On("Get", ctxMatcher, workflowID).Return(nil, nil).Once()
			})
			It("not found error", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflow).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeNotFound))
			})
		})
		When("cannot modify active workflow", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				existingWorkflow := &domain.Workflow{
					ID:     uuid.MustParse(workflowID),
					Name:   "Active Workflow",
					Status: domain.WorkflowActive,
				}
				newName := testNewName
				input = service.UpdateWorkflowInput{
					Name: &newName,
				}

				repo.On("Get", ctxMatcher, workflowID).Return(existingWorkflow, nil).Once()
			})
			It("conflict error", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflow).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeConflict))
			})
		})
	})
	Describe("Activate", func() {
		var (
			workflowID string
			workflow   *domain.Workflow
			err        error
		)
		JustBeforeEach(func() {
			workflow, err = svc.Activate(ctx, workflowID)
		})
		When("workflow exists and can be activated", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				existingWorkflow := &domain.Workflow{
					ID:     uuid.MustParse(workflowID),
					Name:   "Test Workflow",
					Status: domain.WorkflowDraft,
					Steps: []domain.StepDefinition{
						{Name: "step1", Image: "alpine:latest"},
					},
				}
				repo.On("Get", ctxMatcher, workflowID).Return(existingWorkflow, nil).Once()
				repo.On("Update", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(nil).Once()
			})
			It("successful activation", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(workflow).NotTo(BeNil())
				Expect(workflow.ID).To(Equal(uuid.MustParse(workflowID)))
				Expect(workflow.Status).To(Equal(domain.WorkflowActive))
			})
		})
		When("workflow cannot be activated due to validation error", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				existingWorkflow := &domain.Workflow{
					ID:     uuid.MustParse(workflowID),
					Name:   "Test Workflow",
					Status: domain.WorkflowDraft,
					Steps:  []domain.StepDefinition{},
				}
				repo.On("Get", ctxMatcher, workflowID).Return(existingWorkflow, nil).Once()
			})
			It("validation error", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflow).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeValidation))
			})
		})
		When("workflow cannot be activated due to invalid state transition", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				existingWorkflow := &domain.Workflow{
					ID:     uuid.MustParse(workflowID),
					Name:   "Test Workflow",
					Status: domain.WorkflowArchived,
					Steps: []domain.StepDefinition{
						{Name: "step1", Image: "alpine:latest"},
					},
				}
				repo.On("Get", ctxMatcher, workflowID).Return(existingWorkflow, nil).Once()
			})
			It("conflict error", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflow).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeConflict))
			})
		})
		//nolint:dupl // Similar test case for not found error
		When("workflow does not exist", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				repo.On("Get", ctxMatcher, workflowID).Return(nil, nil).Once()
			})
			It("not found error", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflow).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeNotFound))
			})
		})
	})
	Describe("Archive", func() {
		var (
			workflowID string
			workflow   *domain.Workflow
			err        error
		)
		JustBeforeEach(func() {
			workflow, err = svc.Archive(ctx, workflowID)
		})
		When("workflow exists and can be archived", func() {
			BeforeEach(func() {
				workflowID = uuid.New().String()
				existingWorkflow := &domain.Workflow{
					ID:     uuid.MustParse(workflowID),
					Name:   "Test Workflow",
					Status: domain.WorkflowActive,
				}
				repo.On("Get", ctxMatcher, workflowID).Return(existingWorkflow, nil).Once()
				repo.On("Update", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(nil).Once()
			})
			It("successful archiving", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(workflow).NotTo(BeNil())
				Expect(workflow.ID).To(Equal(uuid.MustParse(workflowID)))
				Expect(workflow.Status).To(Equal(domain.WorkflowArchived))
			})
		})
	})
	Describe("List", func() {
		var (
			limit  int
			offset int

			workflows []*domain.Workflow
			err       error
		)
		JustBeforeEach(func() {
			workflows, err = svc.List(ctx, limit, offset)
		})
		When("pagination parameters are valid", func() {
			BeforeEach(func() {
				limit = 10
				offset = 0
				expectedWorkflows := []*domain.Workflow{
					{ID: uuid.New(), Name: "Workflow 1", Status: domain.WorkflowDraft},
					{ID: uuid.New(), Name: "Workflow 2", Status: domain.WorkflowActive},
				}
				repo.On("List", ctxMatcher, mock.AnythingOfType("domain.ListOptions")).Return(expectedWorkflows, nil).Once()
			})
			It("successful listing", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(workflows).NotTo(BeNil())
				Expect(workflows).To(HaveLen(2))
			})
		})
		When("limit is invalid", func() {
			BeforeEach(func() {
				limit = 150
				offset = 0
			})
			It("validation error", func() {
				Expect(err).To(HaveOccurred())
				Expect(workflows).To(BeNil())
				var svcErr *service.ServiceError
				Expect(errors.As(err, &svcErr)).To(BeTrue())
				Expect(svcErr.Type).To(Equal(service.ErrorTypeValidation))
			})
		})
	})
})
