package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/nojyerac/aeneas/domain"
	mockrepo "github.com/nojyerac/aeneas/mocks/domain"
	"github.com/nojyerac/aeneas/service"
	transporthttp "github.com/nojyerac/aeneas/transport/http"
	"github.com/nojyerac/go-lib/log"
	libhttp "github.com/nojyerac/go-lib/transport/http"
)

var ctxMatcher = mock.MatchedBy(func(arg any) bool {
	_, ok := arg.(context.Context)
	return ok
})

var _ = Describe("HTTP Handlers", func() {
	var (
		server       libhttp.Server
		workflowSvc  *service.WorkflowService
		executionSvc *service.ExecutionService
		workflowRepo *mockrepo.MockWorkflowRepository
		execRepo     *mockrepo.MockExecutionRepository
		stepExecRepo *mockrepo.MockStepExecutionRepository
		r            *http.Request
		w            *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		workflowRepo = new(mockrepo.MockWorkflowRepository)
		execRepo = new(mockrepo.MockExecutionRepository)
		stepExecRepo = new(mockrepo.MockStepExecutionRepository)

		workflowSvc = service.NewWorkflowService(workflowRepo)
		executionSvc = service.NewExecutionService(workflowRepo, execRepo, stepExecRepo)

		config := &libhttp.Configuration{}
		server = libhttp.NewServer(
			config,
			libhttp.WithLogger(log.NewLogger(log.TestConfig)),
		)
		transporthttp.RegisterRoutes(server, workflowSvc, executionSvc)
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		workflowRepo.AssertExpectations(GinkgoT())
		execRepo.AssertExpectations(GinkgoT())
		stepExecRepo.AssertExpectations(GinkgoT())
	})

	JustBeforeEach(func() {
		server.ServeHTTP(w, r)
	})

	Describe("POST /api/v1/workflows", func() {
		When("input is valid", func() {
			BeforeEach(func() {
				workflowRepo.On(
					"Create",
					ctxMatcher,
					mock.AnythingOfType("*domain.Workflow"),
				).Return(nil).Once()
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
				body, err := json.Marshal(reqBody)
				Expect(err).NotTo(HaveOccurred())

				r = httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewReader(body))
				r.Header.Set("Content-Type", "application/json")
			})

			It("creates a new workflow", func() {
				Expect(w.Code).To(Equal(http.StatusCreated))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["name"]).To(Equal("Test Workflow"))
				Expect(resp["description"]).To(Equal("A test workflow"))
				Expect(resp["status"]).To(Equal("draft"))
				Expect(resp["id"]).NotTo(BeEmpty())
			})
		})

		When("input is invalid", func() {
			BeforeEach(func() {
				reqBody := map[string]any{
					"name":  "", // Invalid: empty name
					"steps": []map[string]any{},
				}
				body, err := json.Marshal(reqBody)
				Expect(err).NotTo(HaveOccurred())

				r = httptest.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewReader(body))
				r.Header.Set("Content-Type", "application/json")
			})
			It("returns validation error for invalid input", func() {
				Expect(w.Code).To(Equal(http.StatusUnprocessableEntity))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["code"]).To(Equal("VALIDATION_ERROR"))
			})
		})
	})
	Describe("GET /api/v1/workflows", func() {
		When("no query params are provided", func() {
			BeforeEach(func() {
				expectedListOpts := domain.ListOptions{
					Limit:   10,
					Offset:  0,
					OrderBy: "created_at DESC",
				}
				workflowRepo.On("List", ctxMatcher, expectedListOpts).Return([]*domain.Workflow{
					{ID: uuid.New(), Name: "Workflow 1", Status: domain.WorkflowActive},
					{ID: uuid.New(), Name: "Workflow 2", Status: domain.WorkflowDraft},
					{ID: uuid.New(), Name: "Workflow 3", Status: domain.WorkflowArchived},
					{ID: uuid.New(), Name: "Workflow 4", Status: domain.WorkflowActive},
					{ID: uuid.New(), Name: "Workflow 5", Status: domain.WorkflowDraft},
				}, nil).Once()

				r = httptest.NewRequest(http.MethodGet, "/api/v1/workflows", http.NoBody)
			})

			It("lists workflows with default pagination", func() {
				Expect(w.Code).To(Equal(http.StatusOK))

				var resp map[string][]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["workflows"]).To(HaveLen(5))
			})
		})

		When("limit and offset query params are provided", func() {
			BeforeEach(func() {
				expectedListOpts := domain.ListOptions{
					Limit:   2,
					Offset:  1,
					OrderBy: "created_at DESC",
				}
				workflowRepo.On("List", ctxMatcher, expectedListOpts).Return([]*domain.Workflow{
					{ID: uuid.New(), Name: "Workflow 2", Status: domain.WorkflowDraft},
					{ID: uuid.New(), Name: "Workflow 3", Status: domain.WorkflowArchived},
				}, nil).Once()

				r = httptest.NewRequest(http.MethodGet, "/api/v1/workflows?limit=2&offset=1", http.NoBody)
			})

			It("lists workflows with custom limit and offset", func() {
				Expect(w.Code).To(Equal(http.StatusOK))

				var resp map[string][]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["workflows"]).To(HaveLen(2))
			})
		})
	})
	Describe("GET /api/v1/workflows/{id}", func() {
		var workflowID string
		When("workflow exists", func() {
			BeforeEach(func() {
				wf := &domain.Workflow{
					ID:          uuid.New(),
					Name:        "Test Workflow",
					Description: "A test workflow",
					Status:      domain.WorkflowActive,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}
				workflowID = wf.ID.String()
				workflowRepo.On("Get", ctxMatcher, workflowID).Return(wf, nil).Once()
				r = httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+workflowID, http.NoBody)
			})

			It("retrieves a workflow by ID", func() {
				Expect(w.Code).To(Equal(http.StatusOK))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["id"]).To(Equal(workflowID))
				Expect(resp["name"]).To(Equal("Test Workflow"))
			})
		})

		When("workflow does not exist", func() {
			BeforeEach(func() {
				workflowRepo.On("Get", ctxMatcher, mock.AnythingOfType("string")).Return(nil, nil).Once()
				r = httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+uuid.New().String(), http.NoBody)
			})

			It("returns 404 for non-existent workflow", func() {
				Expect(w.Code).To(Equal(http.StatusNotFound))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["code"]).To(Equal("NOT_FOUND"))
			})
		})
	})
	Describe("PUT /api/v1/workflows/{id}", func() {
		var (
			newName    = "Updated Name"
			workflowID string
			wf         *domain.Workflow
		)

		When("workflow exists and is in draft status", func() {
			BeforeEach(func() {
				wf = &domain.Workflow{
					ID:          uuid.New(),
					Name:        "Original Name",
					Description: "Original Description",
					Status:      domain.WorkflowDraft,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}
				workflowID = wf.ID.String()

				reqBody := map[string]interface{}{
					"name": newName,
				}
				body, _ := json.Marshal(reqBody)

				r = httptest.NewRequest(http.MethodPut, "/api/v1/workflows/"+workflowID, bytes.NewReader(body))
				r.Header.Set("Content-Type", "application/json")

				workflowRepo.On("Get", ctxMatcher, workflowID).Return(wf, nil).Once()
				workflowRepo.On("Update", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(nil).Once()
			})

			It("updates the workflow name", func() {
				Expect(w.Code).To(Equal(http.StatusOK))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["name"]).To(Equal(newName))
			})
		})
		When("workflow is active", func() {
			BeforeEach(func() {
				wf = &domain.Workflow{
					ID:          uuid.New(),
					Name:        "Original Name",
					Description: "Original Description",
					Status:      domain.WorkflowActive,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}
				workflowID = wf.ID.String()
				reqBody := map[string]interface{}{
					"name": newName,
				}
				body, _ := json.Marshal(reqBody)

				r = httptest.NewRequest(http.MethodPut, "/api/v1/workflows/"+workflowID, bytes.NewReader(body))
				r.Header.Set("Content-Type", "application/json")

				workflowRepo.On("Get", ctxMatcher, workflowID).Return(wf, nil).Once()
			})
			It("returns 409 (conflict)", func() {
				Expect(w.Code).To(Equal(http.StatusConflict))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["code"]).To(Equal("CONFLICT"))
			})
		})
	})
	Describe("POST /api/v1/workflows/{id}/activate", func() {
		var workflowID string

		BeforeEach(func() {
			workflowID = uuid.New().String()
		})
		//nolint:dupl // Similar setup for both active and draft workflows
		When("workflow is in draft status", func() {
			BeforeEach(func() {
				workflowRepo.On("Get", ctxMatcher, workflowID).Return(&domain.Workflow{
					ID:          uuid.MustParse(workflowID),
					Name:        "Test Workflow",
					Description: "Test",
					Status:      domain.WorkflowDraft,
					Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}, nil).Once()
				workflowRepo.On("Update", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(nil).Once()
				r = httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+workflowID+"/activate", http.NoBody)
			})

			It("activates a draft workflow", func() {
				Expect(w.Code).To(Equal(http.StatusOK))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["status"]).To(Equal("active"))
			})
		})
		When("workflow is already active", func() {
			BeforeEach(func() {
				workflowRepo.On("Get", ctxMatcher, workflowID).Return(&domain.Workflow{
					ID:          uuid.MustParse(workflowID),
					Name:        "Test Workflow",
					Description: "Test",
					Status:      domain.WorkflowActive,
					Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}, nil).Once()
				r = httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+workflowID+"/activate", http.NoBody)
			})
			It("returns 409 (conflict)", func() {
				Expect(w.Code).To(Equal(http.StatusConflict))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["code"]).To(Equal("CONFLICT"))
			})
		})
	})
	Describe("POST /api/v1/workflows/{id}/archive", func() {
		var workflowID string

		BeforeEach(func() {
			workflowID = uuid.New().String()
		})
		//nolint:dupl // Similar setup for both active and draft workflows
		When("workflow is active", func() {
			BeforeEach(func() {
				workflowRepo.On("Get", ctxMatcher, workflowID).Return(&domain.Workflow{
					ID:          uuid.MustParse(workflowID),
					Name:        "Test Workflow",
					Description: "Test",
					Status:      domain.WorkflowActive,
					Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}, nil).Once()
				workflowRepo.On("Update", ctxMatcher, mock.AnythingOfType("*domain.Workflow")).Return(nil).Once()
				r = httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+workflowID+"/archive", http.NoBody)
			})
			It("archives an active workflow", func() {
				Expect(w.Code).To(Equal(http.StatusOK))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["status"]).To(Equal("archived"))
			})
		})
	})
	Describe("POST /api/v1/workflows/{id}/executions", func() {
		var workflowID string

		BeforeEach(func() {
			workflowID = uuid.New().String()
		})
		When("workflow id is valid and workflow is active", func() {
			BeforeEach(func() {
				workflowRepo.On("Get", ctxMatcher, workflowID).Return(&domain.Workflow{
					ID:          uuid.MustParse(workflowID),
					Name:        "Test Workflow",
					Description: "Test",
					Status:      domain.WorkflowActive,
					Steps:       []domain.StepDefinition{{Name: "step1", Image: "alpine"}},
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}, nil).Once()
				execRepo.On("Create", ctxMatcher, mock.AnythingOfType("*domain.Execution")).Return(nil).Once()
				stepExecRepo.On("Create", ctxMatcher, mock.AnythingOfType("*domain.StepExecution")).Return(nil).Once()
				r = httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+workflowID+"/executions", http.NoBody)
			})
			It("triggers a new execution", func() {
				Expect(w.Code).To(Equal(http.StatusCreated))

				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp["workflowID"]).To(Equal(workflowID))
				Expect(resp["status"]).To(Equal("pending"))
			})
		})
	})
	Describe("GET /api/v1/workflows/{id}/executions", func() {
		var workflowID string

		BeforeEach(func() {
			workflowID = uuid.New().String()
			// return some executions
			var execs []*domain.Execution
			for i := 0; i < 3; i++ {
				now := time.Now()
				exec := &domain.Execution{
					ID:         uuid.New(),
					WorkflowID: uuid.MustParse(workflowID),
					Status:     domain.ExecutionPending,
					StartedAt:  &now,
				}
				execs = append(execs, exec)
			}
			execRepo.On(
				"ListByWorkflow",
				ctxMatcher,
				workflowID,
				mock.AnythingOfType("domain.ListOptions"),
			).Return(execs, nil).Once()
			r = httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+workflowID+"/executions", http.NoBody)
		})

		It("lists executions for a workflow", func() {
			Expect(w.Code).To(Equal(http.StatusOK))

			var resp map[string][]any
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp["executions"]).To(HaveLen(3))
		})
	})
	Describe("GET /api/v1/executions/{id}", func() {
		var executionID string
		var workflowID uuid.UUID

		BeforeEach(func() {
			workflowID = uuid.New()

			now := time.Now()
			exec := &domain.Execution{
				ID:         uuid.New(),
				WorkflowID: workflowID,
				Status:     domain.ExecutionRunning,
				StartedAt:  &now,
			}
			executionID = exec.ID.String()

			// Create step execution
			stepExec := &domain.StepExecution{
				ID:          uuid.New(),
				ExecutionID: exec.ID,
				StepName:    "step1",
				Status:      domain.StepExecutionRunning,
				StartedAt:   &now,
			}
			r = httptest.NewRequest(http.MethodGet, "/api/v1/executions/"+executionID, http.NoBody)
			execRepo.On("Get", ctxMatcher, executionID).Return(exec, nil).Once()
			stepExecRepo.On("ListByExecution", ctxMatcher, executionID).Return([]*domain.StepExecution{stepExec}, nil).Once()
		})

		It("retrieves an execution with steps", func() {
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
		BeforeEach(func() {
			workflowID := uuid.New()
			now := time.Now()
			exec := &domain.Execution{
				ID:         uuid.New(),
				WorkflowID: workflowID,
				Status:     domain.ExecutionRunning,
				StartedAt:  &now,
			}
			executionID := exec.ID.String()
			execRepo.On("Get", ctxMatcher, executionID).Return(exec, nil).Once()
			execRepo.On("UpdateStatus", ctxMatcher, executionID, domain.ExecutionCanceled).Return(nil).Once()
			r = httptest.NewRequest(http.MethodPost, "/api/v1/executions/"+executionID+"/cancel", http.NoBody)
		})

		It("cancels a running execution", func() {
			Expect(w.Code).To(Equal(http.StatusNoContent))
		})
	})
})
