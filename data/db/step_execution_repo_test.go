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

var _ = Describe("StepExecutionRepository", func() {
	var (
		database        golib.Database
		repo            *db.StepExecutionRepository
		executionRepo   *db.ExecutionRepository
		workflowRepo    *db.WorkflowRepository
		ctx             context.Context
		testExecutionID uuid.UUID
	)

	BeforeEach(func() {
		ctx = context.Background()
		database = setupTestDatabase(ctx)
		repo = db.NewStepExecutionRepository(database)
		executionRepo = db.NewExecutionRepository(database)
		workflowRepo = db.NewWorkflowRepository(database)

		// Create a test workflow and execution
		workflow := &domain.Workflow{
			Name:        "Test Workflow",
			Description: "For testing step executions",
			Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
			Status:      domain.WorkflowActive,
		}
		err := workflowRepo.Create(ctx, workflow)
		Expect(err).NotTo(HaveOccurred())

		execution := &domain.Execution{
			WorkflowID: workflow.ID,
			Status:     domain.ExecutionPending,
		}
		err = executionRepo.Create(ctx, execution)
		Expect(err).NotTo(HaveOccurred())
		testExecutionID = execution.ID
	})

	AfterEach(func() {
		if database != nil {
			database.Close()
		}
	})

	Describe("Create", func() {
		It("should create a new step execution", func() {
			stepExecution := &domain.StepExecution{
				ExecutionID: testExecutionID,
				StepName:    "build",
				Status:      domain.StepExecutionPending,
			}

			err := repo.Create(ctx, stepExecution)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepExecution.ID).NotTo(Equal(uuid.Nil))
		})

		It("should create step execution with timestamps", func() {
			now := time.Now().UTC()
			stepExecution := &domain.StepExecution{
				ExecutionID: testExecutionID,
				StepName:    "test",
				Status:      domain.StepExecutionRunning,
				StartedAt:   &now,
			}

			err := repo.Create(ctx, stepExecution)
			Expect(err).NotTo(HaveOccurred())

			steps, err := repo.ListByExecution(ctx, testExecutionID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps).To(HaveLen(1))
			Expect(steps[0].StartedAt).NotTo(BeNil())
		})

		It("should create step execution with exit code", func() {
			exitCode := 0
			stepExecution := &domain.StepExecution{
				ExecutionID: testExecutionID,
				StepName:    "deploy",
				Status:      domain.StepExecutionSucceeded,
				ExitCode:    &exitCode,
			}

			err := repo.Create(ctx, stepExecution)
			Expect(err).NotTo(HaveOccurred())

			steps, err := repo.ListByExecution(ctx, testExecutionID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps).To(HaveLen(1))
			Expect(steps[0].ExitCode).NotTo(BeNil())
			Expect(*steps[0].ExitCode).To(Equal(0))
		})

		It("should create step execution with error message", func() {
			stepExecution := &domain.StepExecution{
				ExecutionID: testExecutionID,
				StepName:    "lint",
				Status:      domain.StepExecutionFailed,
				Error:       "linting errors found",
			}

			err := repo.Create(ctx, stepExecution)
			Expect(err).NotTo(HaveOccurred())

			steps, err := repo.ListByExecution(ctx, testExecutionID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps).To(HaveLen(1))
			Expect(steps[0].Error).To(Equal("linting errors found"))
		})
	})

	Describe("ListByExecution", func() {
		BeforeEach(func() {
			// Create multiple step executions
			stepNames := []string{"build", "test", "lint", "deploy"}
			for _, name := range stepNames {
				stepExecution := &domain.StepExecution{
					ExecutionID: testExecutionID,
					StepName:    name,
					Status:      domain.StepExecutionPending,
				}
				err := repo.Create(ctx, stepExecution)
				Expect(err).NotTo(HaveOccurred())
			}

			// Create step execution for a different execution
			otherWorkflow := &domain.Workflow{
				Name:        "Other Workflow",
				Description: "Different execution",
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
				Status:      domain.WorkflowActive,
			}
			err := workflowRepo.Create(ctx, otherWorkflow)
			Expect(err).NotTo(HaveOccurred())

			otherExecution := &domain.Execution{
				WorkflowID: otherWorkflow.ID,
				Status:     domain.ExecutionPending,
			}
			err = executionRepo.Create(ctx, otherExecution)
			Expect(err).NotTo(HaveOccurred())

			otherStepExecution := &domain.StepExecution{
				ExecutionID: otherExecution.ID,
				StepName:    "other_step",
				Status:      domain.StepExecutionPending,
			}
			err = repo.Create(ctx, otherStepExecution)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should list all step executions for an execution", func() {
			steps, err := repo.ListByExecution(ctx, testExecutionID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps).To(HaveLen(4))

			stepNames := make([]string, len(steps))
			for i, step := range steps {
				stepNames[i] = step.StepName
				Expect(step.ExecutionID).To(Equal(testExecutionID))
			}
			Expect(stepNames).To(ContainElements("build", "test", "lint", "deploy"))
		})

		It("should return steps in creation order", func() {
			steps, err := repo.ListByExecution(ctx, testExecutionID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps[0].StepName).To(Equal("build"))
			Expect(steps[1].StepName).To(Equal("test"))
			Expect(steps[2].StepName).To(Equal("lint"))
			Expect(steps[3].StepName).To(Equal("deploy"))
		})

		It("should return error for invalid execution UUID", func() {
			_, err := repo.ListByExecution(ctx, "invalid-uuid")
			Expect(err).To(HaveOccurred())
		})

		It("should return empty list for execution with no steps", func() {
			newWorkflow := &domain.Workflow{
				Name:        "Empty Workflow",
				Description: "No steps",
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
				Status:      domain.WorkflowActive,
			}
			err := workflowRepo.Create(ctx, newWorkflow)
			Expect(err).NotTo(HaveOccurred())

			newExecution := &domain.Execution{
				WorkflowID: newWorkflow.ID,
				Status:     domain.ExecutionPending,
			}
			err = executionRepo.Create(ctx, newExecution)
			Expect(err).NotTo(HaveOccurred())

			steps, err := repo.ListByExecution(ctx, newExecution.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps).To(BeEmpty())
		})
	})

	Describe("UpdateStatus", func() {
		var existingStepExecution *domain.StepExecution

		BeforeEach(func() {
			existingStepExecution = &domain.StepExecution{
				ExecutionID: testExecutionID,
				StepName:    "build",
				Status:      domain.StepExecutionPending,
			}
			err := repo.Create(ctx, existingStepExecution)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update step execution status", func() {
			err := repo.UpdateStatus(ctx, existingStepExecution.ID.String(), domain.StepExecutionRunning, nil)
			Expect(err).NotTo(HaveOccurred())

			steps, err := repo.ListByExecution(ctx, testExecutionID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps).To(HaveLen(1))
			Expect(steps[0].Status).To(Equal(domain.StepExecutionRunning))
		})

		It("should update status with exit code", func() {
			exitCode := 0
			err := repo.UpdateStatus(ctx, existingStepExecution.ID.String(), domain.StepExecutionSucceeded, &exitCode)
			Expect(err).NotTo(HaveOccurred())

			steps, err := repo.ListByExecution(ctx, testExecutionID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps).To(HaveLen(1))
			Expect(steps[0].Status).To(Equal(domain.StepExecutionSucceeded))
			Expect(steps[0].ExitCode).NotTo(BeNil())
			Expect(*steps[0].ExitCode).To(Equal(0))
		})

		It("should allow valid status transitions", func() {
			// Pending -> Running
			err := repo.UpdateStatus(ctx, existingStepExecution.ID.String(), domain.StepExecutionRunning, nil)
			Expect(err).NotTo(HaveOccurred())

			// Running -> Succeeded
			exitCode := 0
			err = repo.UpdateStatus(ctx, existingStepExecution.ID.String(), domain.StepExecutionSucceeded, &exitCode)
			Expect(err).NotTo(HaveOccurred())

			steps, err := repo.ListByExecution(ctx, testExecutionID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps[0].Status).To(Equal(domain.StepExecutionSucceeded))
		})

		It("should allow skipping pending steps", func() {
			err := repo.UpdateStatus(ctx, existingStepExecution.ID.String(), domain.StepExecutionSkipped, nil)
			Expect(err).NotTo(HaveOccurred())

			steps, err := repo.ListByExecution(ctx, testExecutionID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(steps[0].Status).To(Equal(domain.StepExecutionSkipped))
		})

		It("should return error for non-existent step execution", func() {
			err := repo.UpdateStatus(ctx, uuid.New().String(), domain.StepExecutionRunning, nil)
			Expect(err).To(Equal(db.ErrStepExecutionNotFound))
		})

		It("should return error for invalid UUID", func() {
			err := repo.UpdateStatus(ctx, "invalid-uuid", domain.StepExecutionRunning, nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Foreign Key Constraints", func() {
		It("should prevent creating step execution with non-existent execution", func() {
			stepExecution := &domain.StepExecution{
				ExecutionID: uuid.New(),
				StepName:    "build",
				Status:      domain.StepExecutionPending,
			}

			err := repo.Create(ctx, stepExecution)
			Expect(err).To(HaveOccurred())
		})
	})
})
