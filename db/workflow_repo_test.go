package db_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nojyerac/aeneas/db"
	"github.com/nojyerac/aeneas/domain"
	golib "github.com/nojyerac/go-lib/db"
)

var _ = Describe("WorkflowRepository", func() {
	var (
		database golib.Database
		repo     *db.WorkflowRepository
		ctx      context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		database = setupTestDatabase(ctx)
		repo = db.NewWorkflowRepository(database)
	})

	AfterEach(func() {
		if database != nil {
			database.Close()
		}
	})

	Describe("Create", func() {
		It("should create a new workflow", func() {
			workflow := &domain.Workflow{
				Name:        "Test Workflow",
				Description: "A test workflow",
				Steps: []domain.StepDefinition{
					{
						Name:           "step1",
						Image:          "alpine:latest",
						Command:        []string{"echo"},
						Args:           []string{"hello"},
						Env:            map[string]string{"FOO": "bar"},
						TimeoutSeconds: 30,
					},
				},
				Status: domain.WorkflowActive,
			}

			err := repo.Create(ctx, workflow)
			Expect(err).NotTo(HaveOccurred())
			Expect(workflow.ID).NotTo(Equal(uuid.Nil))
			Expect(workflow.CreatedAt).NotTo(BeZero())
			Expect(workflow.UpdatedAt).NotTo(BeZero())
		})

		It("should persist workflow with all fields", func() {
			workflow := &domain.Workflow{
				Name:        "Complex Workflow",
				Description: "Multi-step workflow",
				Steps: []domain.StepDefinition{
					{Name: "step1", Image: "alpine:latest", Command: []string{"echo"}, Args: []string{"hello"}},
					{Name: "step2", Image: "alpine:latest", Command: []string{"echo"}, Args: []string{"world"}},
				},
				Status: domain.WorkflowDraft,
			}

			err := repo.Create(ctx, workflow)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := repo.Get(ctx, workflow.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Name).To(Equal(workflow.Name))
			Expect(retrieved.Description).To(Equal(workflow.Description))
			Expect(retrieved.Steps).To(HaveLen(2))
			Expect(retrieved.Status).To(Equal(domain.WorkflowDraft))
		})
	})

	Describe("Get", func() {
		var existingWorkflow *domain.Workflow

		BeforeEach(func() {
			existingWorkflow = &domain.Workflow{
				Name:        "Existing Workflow",
				Description: "Test",
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
				Status:      domain.WorkflowActive,
			}
			err := repo.Create(ctx, existingWorkflow)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should retrieve an existing workflow", func() {
			retrieved, err := repo.Get(ctx, existingWorkflow.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.ID).To(Equal(existingWorkflow.ID))
			Expect(retrieved.Name).To(Equal(existingWorkflow.Name))
		})

		It("should return error for non-existent workflow", func() {
			_, err := repo.Get(ctx, uuid.New().String())
			Expect(err).To(Equal(db.ErrWorkflowNotFound))
		})

		It("should return error for invalid UUID", func() {
			_, err := repo.Get(ctx, "invalid-uuid")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("List", func() {
		BeforeEach(func() {
			// Create multiple workflows
			for i := 0; i < 5; i++ {
				workflow := &domain.Workflow{
					Name:        "Workflow " + string(rune('A'+i)),
					Description: "Test workflow",
					Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
					Status:      domain.WorkflowActive,
				}
				err := repo.Create(ctx, workflow)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should list all workflows without pagination", func() {
			workflows, err := repo.List(ctx, domain.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(workflows).To(HaveLen(5))
		})

		It("should respect limit", func() {
			workflows, err := repo.List(ctx, domain.ListOptions{Limit: 2})
			Expect(err).NotTo(HaveOccurred())
			Expect(workflows).To(HaveLen(2))
		})

		It("should respect offset", func() {
			workflows, err := repo.List(ctx, domain.ListOptions{Limit: 2, Offset: 3})
			Expect(err).NotTo(HaveOccurred())
			Expect(workflows).To(HaveLen(2))
		})

		It("should order by custom field", func() {
			workflows, err := repo.List(ctx, domain.ListOptions{OrderBy: "name ASC"})
			Expect(err).NotTo(HaveOccurred())
			Expect(workflows).To(HaveLen(5))
			Expect(workflows[0].Name).To(Equal("Workflow A"))
			Expect(workflows[4].Name).To(Equal("Workflow E"))
		})

		It("should handle empty result set", func() {
			// Clear all workflows
			workflows, err := repo.List(ctx, domain.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			for _, w := range workflows {
				w.Status = domain.WorkflowArchived
				err := repo.Update(ctx, w)
				Expect(err).NotTo(HaveOccurred())
			}

			// List with offset and limit beyond available records
			workflows, err = repo.List(ctx, domain.ListOptions{Limit: 10, Offset: 100})
			Expect(err).NotTo(HaveOccurred())
			Expect(workflows).To(BeEmpty())
		})
	})

	Describe("Update", func() {
		var existingWorkflow *domain.Workflow

		BeforeEach(func() {
			existingWorkflow = &domain.Workflow{
				Name:        "Original Name",
				Description: "Original Description",
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
				Status:      domain.WorkflowActive,
			}
			err := repo.Create(ctx, existingWorkflow)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update workflow fields", func() {
			existingWorkflow.Name = "Updated Name"
			existingWorkflow.Description = "Updated Description"
			existingWorkflow.Status = domain.WorkflowArchived

			err := repo.Update(ctx, existingWorkflow)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := repo.Get(ctx, existingWorkflow.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Name).To(Equal("Updated Name"))
			Expect(retrieved.Description).To(Equal("Updated Description"))
			Expect(retrieved.Status).To(Equal(domain.WorkflowArchived))
		})

		It("should update steps", func() {
			existingWorkflow.Steps = []domain.StepDefinition{
				{Name: "new_step", Image: "alpine:3.17", Command: []string{"sh"}},
			}

			err := repo.Update(ctx, existingWorkflow)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := repo.Get(ctx, existingWorkflow.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Steps).To(HaveLen(1))
			Expect(retrieved.Steps[0].Name).To(Equal("new_step"))
		})

		It("should return error for non-existent workflow", func() {
			nonExistent := &domain.Workflow{
				ID:          uuid.New(),
				Name:        "Non-existent",
				Description: "Test",
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine:latest"}},
				Status:      domain.WorkflowActive,
			}

			err := repo.Update(ctx, nonExistent)
			Expect(err).To(Equal(db.ErrWorkflowNotFound))
		})

		It("should update UpdatedAt timestamp", func() {
			originalUpdatedAt := existingWorkflow.UpdatedAt
			existingWorkflow.Name = "New Name"

			err := repo.Update(ctx, existingWorkflow)
			Expect(err).NotTo(HaveOccurred())
			Expect(existingWorkflow.UpdatedAt).To(BeTemporally(">", originalUpdatedAt))
		})
	})
})
