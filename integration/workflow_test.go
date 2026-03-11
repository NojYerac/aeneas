//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/nojyerac/aeneas/config"
	"github.com/nojyerac/aeneas/data/db"
	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/engine"
	"github.com/nojyerac/aeneas/runner/local"
	"github.com/nojyerac/aeneas/service"
	httptransport "github.com/nojyerac/aeneas/transport/http"
	libdb "github.com/nojyerac/go-lib/db"
	libhttp "github.com/nojyerac/go-lib/transport/http"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Workflow Integration Tests", func() {
	var (
		server     *httptest.Server
		ctx        context.Context
		cancel     context.CancelFunc
		database   *libdb.Database
		eng        *engine.Engine
		dbPath     string
		logger     *logrus.Logger
		baseURL    string
		httpClient *http.Client
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		// Create temporary database
		tmpDir, err := os.MkdirTemp("", "aeneas-integration-*")
		Expect(err).NotTo(HaveOccurred())
		dbPath = filepath.Join(tmpDir, "test.db")

		// Initialize logger
		logger = logrus.New()
		logger.SetLevel(logrus.DebugLevel)

		// Initialize database
		dbConfig := &libdb.Configuration{
			Type:     libdb.SQLite,
			Host:     dbPath,
			Database: dbPath,
		}
		database = libdb.NewDatabase(dbConfig, libdb.WithLogger(logger))
		err = database.Open(ctx)
		Expect(err).NotTo(HaveOccurred())

		// Initialize repositories
		workflowRepo := db.NewWorkflowRepository(database, db.WithLogger(logger))
		executionRepo := db.NewExecutionRepository(database, db.WithLogger(logger))
		stepExecutionRepo := db.NewStepExecutionRepository(database, db.WithLogger(logger))

		// Initialize services
		workflowSvc := service.NewWorkflowService(workflowRepo)
		executionSvc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

		// Initialize runner and engine
		runner, err := local.NewLocalRunner(logger)
		Expect(err).NotTo(HaveOccurred())

		engineConfig := engine.Config{
			PollInterval: 500 * time.Millisecond, // Faster polling for tests
		}
		eng = engine.NewEngine(workflowRepo, executionRepo, stepExecutionRepo, runner, logger, engineConfig)

		// Start engine
		err = eng.Start(ctx)
		Expect(err).NotTo(HaveOccurred())

		// Initialize HTTP server
		httpConfig := &libhttp.Configuration{
			Port: 0, // Use random port
		}
		hSrv := libhttp.NewServer(httpConfig)
		httptransport.RegisterRoutes(hSrv, workflowSvc, executionSvc)

		// Create test server
		server = httptest.NewServer(hSrv.Handler())
		baseURL = server.URL
		httpClient = &http.Client{Timeout: 10 * time.Second}
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
		if eng != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			_ = eng.Stop(shutdownCtx)
		}
		if database != nil {
			_ = database.Close()
		}
		if dbPath != "" {
			_ = os.RemoveAll(filepath.Dir(dbPath))
		}
		cancel()
	})

	Describe("Successful Workflow Execution", func() {
		It("should execute a simple workflow with 3 steps successfully", func() {
			// Create workflow
			workflow := createWorkflow(baseURL, httpClient, "test-workflow", []domain.StepDefinition{
				{
					Name:    "step1",
					Image:   "alpine:latest",
					Command: []string{"echo"},
					Args:    []string{"Hello from step 1"},
				},
				{
					Name:    "step2",
					Image:   "alpine:latest",
					Command: []string{"echo"},
					Args:    []string{"Hello from step 2"},
				},
				{
					Name:    "step3",
					Image:   "alpine:latest",
					Command: []string{"echo"},
					Args:    []string{"Hello from step 3"},
				},
			})
			Expect(workflow).NotTo(BeNil())

			// Activate workflow
			activateWorkflow(baseURL, httpClient, workflow.ID.String())

			// Trigger execution
			execution := triggerExecution(baseURL, httpClient, workflow.ID.String())
			Expect(execution).NotTo(BeNil())
			Expect(execution.Status).To(Equal(domain.ExecutionPending))

			// Poll until terminal state
			finalExecution, steps := pollExecutionUntilTerminal(baseURL, httpClient, execution.ID.String(), 30*time.Second)
			Expect(finalExecution).NotTo(BeNil())
			Expect(finalExecution.Status).To(Equal(domain.ExecutionSucceeded))

			// Verify all steps succeeded
			Expect(steps).To(HaveLen(3))
			for _, step := range steps {
				Expect(step.Status).To(Equal(domain.StepExecutionSucceeded))
				Expect(step.ExitCode).NotTo(BeNil())
				Expect(*step.ExitCode).To(Equal(0))
			}
		})
	})

	Describe("Failed Workflow Execution", func() {
		It("should handle step failure and skip subsequent steps", func() {
			// Create workflow with a failing step
			workflow := createWorkflow(baseURL, httpClient, "failing-workflow", []domain.StepDefinition{
				{
					Name:    "step1",
					Image:   "alpine:latest",
					Command: []string{"echo"},
					Args:    []string{"This will succeed"},
				},
				{
					Name:    "step2",
					Image:   "alpine:latest",
					Command: []string{"sh"},
					Args:    []string{"-c", "exit 1"}, // This will fail
				},
				{
					Name:    "step3",
					Image:   "alpine:latest",
					Command: []string{"echo"},
					Args:    []string{"This should be skipped"},
				},
			})
			Expect(workflow).NotTo(BeNil())

			// Activate workflow
			activateWorkflow(baseURL, httpClient, workflow.ID.String())

			// Trigger execution
			execution := triggerExecution(baseURL, httpClient, workflow.ID.String())
			Expect(execution).NotTo(BeNil())

			// Poll until terminal state
			finalExecution, steps := pollExecutionUntilTerminal(baseURL, httpClient, execution.ID.String(), 30*time.Second)
			Expect(finalExecution).NotTo(BeNil())
			Expect(finalExecution.Status).To(Equal(domain.ExecutionFailed))

			// Verify step statuses
			Expect(steps).To(HaveLen(3))
			Expect(steps[0].Status).To(Equal(domain.StepExecutionSucceeded))
			Expect(steps[1].Status).To(Equal(domain.StepExecutionFailed))
			Expect(*steps[1].ExitCode).To(Equal(1))
			Expect(steps[2].Status).To(Equal(domain.StepExecutionSkipped))
		})
	})

	Describe("Execution Cancellation", func() {
		It("should cancel a running execution and skip remaining steps", func() {
			// Create workflow with a long-running step
			workflow := createWorkflow(baseURL, httpClient, "cancellable-workflow", []domain.StepDefinition{
				{
					Name:    "step1",
					Image:   "alpine:latest",
					Command: []string{"sleep"},
					Args:    []string{"2"}, // Sleep for 2 seconds
				},
				{
					Name:    "step2",
					Image:   "alpine:latest",
					Command: []string{"echo"},
					Args:    []string{"This should be skipped"},
				},
			})
			Expect(workflow).NotTo(BeNil())

			// Activate workflow
			activateWorkflow(baseURL, httpClient, workflow.ID.String())

			// Trigger execution
			execution := triggerExecution(baseURL, httpClient, workflow.ID.String())
			Expect(execution).NotTo(BeNil())

			// Wait a moment for execution to start
			time.Sleep(1 * time.Second)

			// Cancel execution
			cancelExecution(baseURL, httpClient, execution.ID.String())

			// Poll until terminal state
			finalExecution, steps := pollExecutionUntilTerminal(baseURL, httpClient, execution.ID.String(), 30*time.Second)
			Expect(finalExecution).NotTo(BeNil())
			Expect(finalExecution.Status).To(Equal(domain.ExecutionCanceled))

			// At least some steps should be skipped
			Expect(steps).To(HaveLen(2))
			hasSkipped := false
			for _, step := range steps {
				if step.Status == domain.StepExecutionSkipped {
					hasSkipped = true
					break
				}
			}
			Expect(hasSkipped).To(BeTrue(), "Expected at least one skipped step")
		})
	})
})

// Helper functions

func createWorkflow(baseURL string, client *http.Client, name string, steps []domain.StepDefinition) *domain.Workflow {
	input := service.CreateWorkflowInput{
		Name:        name,
		Description: "Integration test workflow",
		Steps:       steps,
	}

	body, err := json.Marshal(input)
	Expect(err).NotTo(HaveOccurred())

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/workflows", bytes.NewReader(body))
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusCreated))

	var workflow domain.Workflow
	err = json.NewDecoder(resp.Body).Decode(&workflow)
	Expect(err).NotTo(HaveOccurred())

	return &workflow
}

func activateWorkflow(baseURL string, client *http.Client, workflowID string) {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/workflows/"+workflowID+"/activate", nil)
	Expect(err).NotTo(HaveOccurred())

	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

func triggerExecution(baseURL string, client *http.Client, workflowID string) *domain.Execution {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/workflows/"+workflowID+"/execute", nil)
	Expect(err).NotTo(HaveOccurred())

	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusCreated))

	var execution domain.Execution
	err = json.NewDecoder(resp.Body).Decode(&execution)
	Expect(err).NotTo(HaveOccurred())

	return &execution
}

func cancelExecution(baseURL string, client *http.Client, executionID string) {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/executions/"+executionID+"/cancel", nil)
	Expect(err).NotTo(HaveOccurred())

	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

func pollExecutionUntilTerminal(baseURL string, client *http.Client, executionID string, timeout time.Duration) (*domain.Execution, []*domain.StepExecution) {
	start := time.Now()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		if time.Since(start) > timeout {
			Fail(fmt.Sprintf("Timeout waiting for execution %s to reach terminal state", executionID))
		}

		req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/executions/"+executionID, nil)
		Expect(err).NotTo(HaveOccurred())

		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		Expect(err).NotTo(HaveOccurred())

		if resp.StatusCode != http.StatusOK {
			Fail(fmt.Sprintf("Failed to get execution: status=%d, body=%s", resp.StatusCode, string(bodyBytes)))
		}

		var result struct {
			Execution *domain.Execution        `json:"execution"`
			Steps     []*domain.StepExecution `json:"steps"`
		}
		err = json.Unmarshal(bodyBytes, &result)
		Expect(err).NotTo(HaveOccurred())

		// Check if terminal state
		if result.Execution.Status == domain.ExecutionSucceeded ||
			result.Execution.Status == domain.ExecutionFailed ||
			result.Execution.Status == domain.ExecutionCanceled {
			return result.Execution, result.Steps
		}

		<-ticker.C
	}
}
