package db_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nojyerac/aeneas/data/db"
	"github.com/nojyerac/aeneas/domain"
	golib "github.com/nojyerac/go-lib/db"
)

var _ = Describe("ExecutionRepository", func() {
	var (
		database       golib.Database
		repo           *db.ExecutionRepository
		workflowRepo   *db.WorkflowRepository
		ctx            context.Context
		testWorkflowID uuid.UUID
	)

	BeforeEach(func() {
		ctx = context.Background()
		database = setupTestDatabase(ctx)
		repo = db.NewExecutionRepository(database)
		workflowRepo = db.NewWorkflowRepository(database)

		// Create a test workflow
		workflow := &domain.Workflow{
			Name:        "Test Workflow",
			Description: "For testing executions",
			Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
			Status:      domain.WorkflowActive,
		}
		err := workflowRepo.Create(ctx, workflow)
		Expect(err).NotTo(HaveOccurred())
		testWorkflowID = workflow.ID
	})

	AfterEach(func() {
		if database != nil {
			database.Close()
		}
	})

	Describe("Create", func() {
		It("should create a new execution", func() {
			execution := &domain.Execution{
				WorkflowID: testWorkflowID,
				Status:     domain.ExecutionPending,
			}

			err := repo.Create(ctx, execution)
			Expect(err).NotTo(HaveOccurred())
			Expect(execution.ID).NotTo(Equal(uuid.Nil))
		})

		It("should create execution with timestamps", func() {
			now := time.Now().UTC()
			execution := &domain.Execution{
				WorkflowID: testWorkflowID,
				Status:     domain.ExecutionRunning,
				StartedAt:  &now,
			}

			err := repo.Create(ctx, execution)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := repo.Get(ctx, execution.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.StartedAt).NotTo(BeNil())
			Expect(*retrieved.StartedAt).To(BeTemporally("~", now, time.Second))
		})

		It("should create execution with error message", func() {
			execution := &domain.Execution{
				WorkflowID: testWorkflowID,
				Status:     domain.ExecutionFailed,
				Error:      "step failed with exit code 1",
			}

			err := repo.Create(ctx, execution)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := repo.Get(ctx, execution.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Error).To(Equal("step failed with exit code 1"))
		})
	})

	Describe("Get", func() {
		var existingExecution *domain.Execution

		BeforeEach(func() {
			existingExecution = &domain.Execution{
				WorkflowID: testWorkflowID,
				Status:     domain.ExecutionPending,
			}
			err := repo.Create(ctx, existingExecution)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should retrieve an existing execution", func() {
			retrieved, err := repo.Get(ctx, existingExecution.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.ID).To(Equal(existingExecution.ID))
			Expect(retrieved.WorkflowID).To(Equal(testWorkflowID))
		})

		It("should return error for non-existent execution", func() {
			_, err := repo.Get(ctx, uuid.New().String())
			Expect(err).To(Equal(db.ErrExecutionNotFound))
		})

		It("should return error for invalid UUID", func() {
			_, err := repo.Get(ctx, "invalid-uuid")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ListByWorkflow", func() {
		BeforeEach(func() {
			// Create multiple executions for the test workflow
			for i := 0; i < 5; i++ {
				execution := &domain.Execution{
					WorkflowID: testWorkflowID,
					Status:     domain.ExecutionPending,
				}
				err := repo.Create(ctx, execution)
				Expect(err).NotTo(HaveOccurred())
			}

			// Create execution for a different workflow
			otherWorkflow := &domain.Workflow{
				Name:        "Other Workflow",
				Description: "Should not appear in results",
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
				Status:      domain.WorkflowActive,
			}
			err := workflowRepo.Create(ctx, otherWorkflow)
			Expect(err).NotTo(HaveOccurred())

			otherExecution := &domain.Execution{
				WorkflowID: otherWorkflow.ID,
				Status:     domain.ExecutionPending,
			}
			err = repo.Create(ctx, otherExecution)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should list all executions for a workflow", func() {
			executions, err := repo.ListByWorkflow(ctx, testWorkflowID.String(), domain.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(executions).To(HaveLen(5))

			for _, exec := range executions {
				Expect(exec.WorkflowID).To(Equal(testWorkflowID))
			}
		})

		It("should respect limit", func() {
			executions, err := repo.ListByWorkflow(ctx, testWorkflowID.String(), domain.ListOptions{Limit: 2})
			Expect(err).NotTo(HaveOccurred())
			Expect(executions).To(HaveLen(2))
		})

		It("should respect offset", func() {
			executions, err := repo.ListByWorkflow(ctx, testWorkflowID.String(), domain.ListOptions{Limit: 2, Offset: 3})
			Expect(err).NotTo(HaveOccurred())
			Expect(executions).To(HaveLen(2))
		})

		It("should return error for invalid workflow UUID", func() {
			_, err := repo.ListByWorkflow(ctx, "invalid-uuid", domain.ListOptions{})
			Expect(err).To(HaveOccurred())
		})

		It("should return empty list for workflow with no executions", func() {
			newWorkflow := &domain.Workflow{
				Name:        "Empty Workflow",
				Description: "No executions",
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
				Status:      domain.WorkflowActive,
			}
			err := workflowRepo.Create(ctx, newWorkflow)
			Expect(err).NotTo(HaveOccurred())

			executions, err := repo.ListByWorkflow(ctx, newWorkflow.ID.String(), domain.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(executions).To(BeEmpty())
		})
	})

	Describe("UpdateStatus", func() {
		var existingExecution *domain.Execution

		BeforeEach(func() {
			existingExecution = &domain.Execution{
				WorkflowID: testWorkflowID,
				Status:     domain.ExecutionPending,
			}
			err := repo.Create(ctx, existingExecution)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update execution status", func() {
			err := repo.UpdateStatus(ctx, existingExecution.ID.String(), domain.ExecutionRunning)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := repo.Get(ctx, existingExecution.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Status).To(Equal(domain.ExecutionRunning))
		})

		It("should allow valid status transitions", func() {
			// Pending -> Running
			err := repo.UpdateStatus(ctx, existingExecution.ID.String(), domain.ExecutionRunning)
			Expect(err).NotTo(HaveOccurred())

			// Running -> Succeeded
			err = repo.UpdateStatus(ctx, existingExecution.ID.String(), domain.ExecutionSucceeded)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := repo.Get(ctx, existingExecution.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Status).To(Equal(domain.ExecutionSucceeded))
		})

		It("should return error for non-existent execution", func() {
			err := repo.UpdateStatus(ctx, uuid.New().String(), domain.ExecutionRunning)
			Expect(err).To(Equal(db.ErrExecutionNotFound))
		})

		It("should return error for invalid UUID", func() {
			err := repo.UpdateStatus(ctx, "invalid-uuid", domain.ExecutionRunning)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Foreign Key Constraints", func() {
		It("should prevent creating execution with non-existent workflow", func() {
			execution := &domain.Execution{
				WorkflowID: uuid.New(),
				Status:     domain.ExecutionPending,
			}

			err := repo.Create(ctx, execution)
			Expect(err).To(HaveOccurred())
		})
	})
})
