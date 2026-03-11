package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/service"
	transporthttp "github.com/nojyerac/aeneas/transport/http"
	libhttp "github.com/nojyerac/go-lib/transport/http"
)

var _ = Describe("HTTP Handlers", func() {
	var (
		server       libhttp.Server
		workflowSvc  *service.WorkflowService
		executionSvc *service.ExecutionService
		workflowRepo *mockWorkflowRepo
		execRepo     *mockExecutionRepo
		stepExecRepo *mockStepExecutionRepo
	)

	BeforeEach(func() {
		workflowRepo = newMockWorkflowRepo()
		execRepo = newMockExecutionRepo()
		stepExecRepo = newMockStepExecutionRepo()

		workflowSvc = service.NewWorkflowService(workflowRepo)
		executionSvc = service.NewExecutionService(workflowRepo, execRepo, stepExecRepo)

		config := &libhttp.Configuration{}
		server = libhttp.NewServer(config)
		transporthttp.RegisterRoutes(server, workflowSvc, executionSvc)
	})

	Describe("POST /api/v1/workflows", func() {
		It("creates a new workflow", func() {
			reqBody := map[string]interface{}{
				"name":        "Test Workflow",
				"description": "A test workflow",
				"steps": []map[string]interface{}{
					{
						"name":            "step1",
						"image":           "alpine",
						"command":         []string{"echo"},
						"args":            []string{"hello"},
						"env":             map[string]string{"KEY": "value"},
						"timeout_seconds": 30,
					},
				},
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["name"]).To(Equal("Test Workflow"))
			Expect(resp["description"]).To(Equal("A test workflow"))
			Expect(resp["status"]).To(Equal("draft"))
			Expect(resp["id"]).NotTo(BeEmpty())
		})

		It("returns validation error for invalid input", func() {
			reqBody := map[string]interface{}{
				"name": "", // Invalid: empty name
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnprocessableEntity))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["code"]).To(Equal("VALIDATION_ERROR"))
		})
	})

	Describe("GET /api/v1/workflows", func() {
		BeforeEach(func() {
			// Seed some workflows
			for i := 0; i < 5; i++ {
				wf := &domain.Workflow{
					ID:          uuid.New(),
					Name:        fmt.Sprintf("Workflow %d", i+1),
					Description: fmt.Sprintf("Description %d", i+1),
					Status:      domain.WorkflowDraft,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}
				_ = workflowRepo.Create(context.Background(), wf)
			}
		})

		It("lists workflows with default pagination", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp []map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp).To(HaveLen(5))
		})

		It("lists workflows with custom limit and offset", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows?limit=2&offset=1", http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp []map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp).To(HaveLen(2))
		})
	})

	Describe("GET /api/v1/workflows/{id}", func() {
		var workflowID string

		BeforeEach(func() {
			wf := &domain.Workflow{
				ID:          uuid.New(),
				Name:        "Test Workflow",
				Description: "A test workflow",
				Status:      domain.WorkflowActive,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_ = workflowRepo.Create(context.Background(), wf)
			workflowID = wf.ID.String()
		})

		It("retrieves a workflow by ID", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+workflowID, http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["id"]).To(Equal(workflowID))
			Expect(resp["name"]).To(Equal("Test Workflow"))
		})

		It("returns 404 for non-existent workflow", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+uuid.New().String(), http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["code"]).To(Equal("NOT_FOUND"))
		})
	})

	Describe("PUT /api/v1/workflows/{id}", func() {
		var workflowID string

		BeforeEach(func() {
			wf := &domain.Workflow{
				ID:          uuid.New(),
				Name:        "Original Name",
				Description: "Original Description",
				Status:      domain.WorkflowDraft,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_ = workflowRepo.Create(context.Background(), wf)
			workflowID = wf.ID.String()
		})

		It("updates a workflow", func() {
			newName := "Updated Name"
			reqBody := map[string]interface{}{
				"name": newName,
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/"+workflowID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["name"]).To(Equal(newName))
		})

		It("returns 409 when trying to update active workflow", func() {
			// Activate the workflow first
			wf, _ := workflowRepo.Get(context.Background(), workflowID)
			wf.Status = domain.WorkflowActive
			wf.Steps = []domain.StepDefinition{{Name: "step1", Image: "alpine"}}
			_ = workflowRepo.Update(context.Background(), wf)

			reqBody := map[string]interface{}{
				"name": "New Name",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/"+workflowID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusConflict))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["code"]).To(Equal("CONFLICT"))
		})
	})

	//nolint:dupl // Test structure is similar but tests different operations (activate vs archive)
	Describe("POST /api/v1/workflows/{id}/activate", func() {
		var workflowID string

		BeforeEach(func() {
			wf := &domain.Workflow{
				ID:          uuid.New(),
				Name:        "Test Workflow",
				Description: "Test",
				Status:      domain.WorkflowDraft,
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_ = workflowRepo.Create(context.Background(), wf)
			workflowID = wf.ID.String()
		})

		It("activates a draft workflow", func() {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+workflowID+"/activate", http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["status"]).To(Equal("active"))
		})
	})

	//nolint:dupl // Test structure is similar but tests different operations (activate vs archive)
	Describe("POST /api/v1/workflows/{id}/archive", func() {
		var workflowID string

		BeforeEach(func() {
			wf := &domain.Workflow{
				ID:          uuid.New(),
				Name:        "Test Workflow",
				Description: "Test",
				Status:      domain.WorkflowActive,
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_ = workflowRepo.Create(context.Background(), wf)
			workflowID = wf.ID.String()
		})

		It("archives an active workflow", func() {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+workflowID+"/archive", http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["status"]).To(Equal("archived"))
		})
	})

	Describe("POST /api/v1/workflows/{id}/executions", func() {
		var workflowID string

		BeforeEach(func() {
			wf := &domain.Workflow{
				ID:          uuid.New(),
				Name:        "Test Workflow",
				Description: "Test",
				Status:      domain.WorkflowActive,
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_ = workflowRepo.Create(context.Background(), wf)
			workflowID = wf.ID.String()
		})

		It("triggers a new execution", func() {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+workflowID+"/executions", http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["workflow_id"]).To(Equal(workflowID))
			Expect(resp["status"]).To(Equal("pending"))
		})
	})

	Describe("GET /api/v1/workflows/{id}/executions", func() {
		var workflowID string

		BeforeEach(func() {
			wf := &domain.Workflow{
				ID:          uuid.New(),
				Name:        "Test Workflow",
				Description: "Test",
				Status:      domain.WorkflowActive,
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_ = workflowRepo.Create(context.Background(), wf)
			workflowID = wf.ID.String()

			// Create some executions
			for i := 0; i < 3; i++ {
				now := time.Now()
				exec := &domain.Execution{
					ID:         uuid.New(),
					WorkflowID: wf.ID,
					Status:     domain.ExecutionPending,
					StartedAt:  &now,
				}
				_ = execRepo.Create(context.Background(), exec)
			}
		})

		It("lists executions for a workflow", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+workflowID+"/executions", http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp []map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp).To(HaveLen(3))
		})
	})

	Describe("GET /api/v1/executions/{id}", func() {
		var executionID string
		var workflowID uuid.UUID

		BeforeEach(func() {
			workflowID = uuid.New()
			wf := &domain.Workflow{
				ID:          workflowID,
				Name:        "Test Workflow",
				Description: "Test",
				Status:      domain.WorkflowActive,
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_ = workflowRepo.Create(context.Background(), wf)

			now := time.Now()
			exec := &domain.Execution{
				ID:         uuid.New(),
				WorkflowID: workflowID,
				Status:     domain.ExecutionRunning,
				StartedAt:  &now,
			}
			_ = execRepo.Create(context.Background(), exec)
			executionID = exec.ID.String()

			// Create step execution
			stepExec := &domain.StepExecution{
				ID:          uuid.New(),
				ExecutionID: exec.ID,
				StepName:    "step1",
				Status:      domain.StepExecutionRunning,
				StartedAt:   &now,
			}
			_ = stepExecRepo.Create(context.Background(), stepExec)
		})

		It("retrieves an execution with steps", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/"+executionID, http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["id"]).To(Equal(executionID))
			Expect(resp["status"]).To(Equal("running"))

			steps, ok := resp["steps"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(steps).To(HaveLen(1))
		})
	})

	Describe("POST /api/v1/executions/{id}/cancel", func() {
		var executionID string

		BeforeEach(func() {
			workflowID := uuid.New()
			wf := &domain.Workflow{
				ID:          workflowID,
				Name:        "Test Workflow",
				Description: "Test",
				Status:      domain.WorkflowActive,
				Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_ = workflowRepo.Create(context.Background(), wf)

			now := time.Now()
			exec := &domain.Execution{
				ID:         uuid.New(),
				WorkflowID: workflowID,
				Status:     domain.ExecutionRunning,
				StartedAt:  &now,
			}
			_ = execRepo.Create(context.Background(), exec)
			executionID = exec.ID.String()
		})

		It("cancels a running execution", func() {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/executions/"+executionID+"/cancel", http.NoBody)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})
	})
})

// Mock repositories for testing

type mockWorkflowRepo struct {
	workflows map[string]*domain.Workflow
}

func newMockWorkflowRepo() *mockWorkflowRepo {
	return &mockWorkflowRepo{
		workflows: make(map[string]*domain.Workflow),
	}
}

func (m *mockWorkflowRepo) Create(_ context.Context, w *domain.Workflow) error {
	m.workflows[w.ID.String()] = w
	return nil
}

func (m *mockWorkflowRepo) Get(_ context.Context, id string) (*domain.Workflow, error) {
	w, ok := m.workflows[id]
	if !ok {
		return nil, nil
	}
	return w, nil
}

func (m *mockWorkflowRepo) Update(_ context.Context, w *domain.Workflow) error {
	m.workflows[w.ID.String()] = w
	return nil
}

func (m *mockWorkflowRepo) List(_ context.Context, opts domain.ListOptions) ([]*domain.Workflow, error) {
	var all []*domain.Workflow
	for _, w := range m.workflows {
		all = append(all, w)
	}

	start := opts.Offset
	end := opts.Offset + opts.Limit
	if start > len(all) {
		return []*domain.Workflow{}, nil
	}
	if end > len(all) {
		end = len(all)
	}

	return all[start:end], nil
}

type mockExecutionRepo struct {
	executions map[string]*domain.Execution
}

func newMockExecutionRepo() *mockExecutionRepo {
	return &mockExecutionRepo{
		executions: make(map[string]*domain.Execution),
	}
}

func (m *mockExecutionRepo) Create(_ context.Context, e *domain.Execution) error {
	m.executions[e.ID.String()] = e
	return nil
}

func (m *mockExecutionRepo) Get(_ context.Context, id string) (*domain.Execution, error) {
	e, ok := m.executions[id]
	if !ok {
		return nil, nil
	}
	return e, nil
}

func (m *mockExecutionRepo) UpdateStatus(_ context.Context, id string, status domain.ExecutionStatus) error {
	if e, ok := m.executions[id]; ok {
		e.Status = status
	}
	return nil
}

func (m *mockExecutionRepo) ListByWorkflow(
	_ context.Context,
	workflowID string,
	opts domain.ListOptions,
) ([]*domain.Execution, error) {
	var filtered []*domain.Execution
	for _, e := range m.executions {
		if e.WorkflowID.String() == workflowID {
			filtered = append(filtered, e)
		}
	}

	start := opts.Offset
	end := opts.Offset + opts.Limit
	if start > len(filtered) {
		return []*domain.Execution{}, nil
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], nil
}

type mockStepExecutionRepo struct {
	stepExecutions map[string][]*domain.StepExecution
}

func newMockStepExecutionRepo() *mockStepExecutionRepo {
	return &mockStepExecutionRepo{
		stepExecutions: make(map[string][]*domain.StepExecution),
	}
}

func (m *mockStepExecutionRepo) Create(_ context.Context, s *domain.StepExecution) error {
	execID := s.ExecutionID.String()
	m.stepExecutions[execID] = append(m.stepExecutions[execID], s)
	return nil
}

func (m *mockStepExecutionRepo) Get(_ context.Context, id string) (*domain.StepExecution, error) {
	for _, steps := range m.stepExecutions {
		for _, s := range steps {
			if s.ID.String() == id {
				return s, nil
			}
		}
	}
	return nil, nil
}

func (m *mockStepExecutionRepo) Update(_ context.Context, s *domain.StepExecution) error {
	execID := s.ExecutionID.String()
	for i, step := range m.stepExecutions[execID] {
		if step.ID == s.ID {
			m.stepExecutions[execID][i] = s
			return nil
		}
	}
	return nil
}

func (m *mockStepExecutionRepo) ListByExecution(
	_ context.Context,
	executionID string,
) ([]*domain.StepExecution, error) {
	return m.stepExecutions[executionID], nil
}

func (m *mockStepExecutionRepo) UpdateStatus(
	_ context.Context,
	id string,
	status domain.StepExecutionStatus,
	exitCode *int,
) error {
	for _, steps := range m.stepExecutions {
		for _, s := range steps {
			if s.ID.String() == id {
				s.Status = status
				s.ExitCode = exitCode
				return nil
			}
		}
	}
	return nil
}
