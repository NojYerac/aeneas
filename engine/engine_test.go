package engine_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/engine"
)

var _ = Describe("Engine", func() {
	var (
		ctx               context.Context
		cancel            context.CancelFunc
		workflowRepo      *mockWorkflowRepo
		executionRepo     *mockExecutionRepo
		stepExecutionRepo *mockStepExecutionRepo
		runner            *mockRunner
		eng               *engine.Engine
		testWorkflow      *domain.Workflow
		testExecution     *domain.Execution
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		workflowRepo = newMockWorkflowRepo()
		executionRepo = newMockExecutionRepo()
		stepExecutionRepo = newMockStepExecutionRepo()
		runner = newMockRunner()

		// Create test workflow with 3 steps
		testWorkflow = &domain.Workflow{
			ID:   uuid.New(),
			Name: "test-workflow",
			Steps: []domain.StepDefinition{
				{Name: "step1", Image: "alpine:latest", Command: []string{"echo"}, Args: []string{"hello"}},
				{Name: "step2", Image: "alpine:latest", Command: []string{"echo"}, Args: []string{"world"}},
				{Name: "step3", Image: "alpine:latest", Command: []string{"echo"}, Args: []string{"done"}},
			},
			Status: domain.WorkflowActive,
		}
		workflowRepo.workflows[testWorkflow.ID.String()] = testWorkflow

		// Create test execution
		testExecution = &domain.Execution{
			ID:         uuid.New(),
			WorkflowID: testWorkflow.ID,
			Status:     domain.ExecutionPending,
		}
		executionRepo.executions[testExecution.ID.String()] = testExecution
		executionRepo.executionsByWorkflow[testWorkflow.ID.String()] = []*domain.Execution{testExecution}

		// Configure engine with fast poll interval for testing
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
		eng = engine.New(
			workflowRepo,
			executionRepo,
			stepExecutionRepo,
			runner,
			engine.WithPollInterval(50*time.Millisecond),
			engine.WithLogger(logger),
		)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Lifecycle", func() {
		It("should start and stop cleanly", func() {
			err := eng.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(100 * time.Millisecond)

			err = eng.Stop()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when starting twice", func() {
			err := eng.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = eng.Start(ctx)
			Expect(err).To(MatchError(ContainSubstring("already running")))

			_ = eng.Stop()
		})

		It("should return error when stopping before starting", func() {
			err := eng.Stop()
			Expect(err).To(MatchError(ContainSubstring("not running")))
		})
	})

	Describe("Execution Processing", func() {
		Context("when all steps succeed", func() {
			BeforeEach(func() {
				// Configure runner to return success for all steps
				runner.results = map[string]engine.RunResult{
					"step1": {ExitCode: 0},
					"step2": {ExitCode: 0},
					"step3": {ExitCode: 0},
				}
			})

			It("should process execution successfully", func() {
				err := eng.Start(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Wait for execution to complete
				Eventually(func() domain.ExecutionStatus {
					exec, err := executionRepo.Get(ctx, testExecution.ID.String())
					if err != nil {
						return domain.ExecutionPending
					}
					return exec.Status
				}, "2s", "50ms").Should(Equal(domain.ExecutionSucceeded))

				_ = eng.Stop()

				// Verify all steps were executed
				steps, err := stepExecutionRepo.ListByExecution(ctx, testExecution.ID.String())
				Expect(err).NotTo(HaveOccurred())
				Expect(steps).To(HaveLen(3))

				// Verify step statuses
				for _, step := range steps {
					Expect(step.Status).To(Equal(domain.StepExecutionSucceeded))
					Expect(step.ExitCode).NotTo(BeNil())
					Expect(*step.ExitCode).To(Equal(0))
				}

				// Verify execution status
				execution, err := executionRepo.Get(ctx, testExecution.ID.String())
				Expect(err).NotTo(HaveOccurred())
				Expect(execution.Status).To(Equal(domain.ExecutionSucceeded))
				Expect(execution.StartedAt).NotTo(BeNil())
				Expect(execution.FinishedAt).NotTo(BeNil())
			})
		})

		Context("when a step fails", func() {
			BeforeEach(func() {
				// Configure runner: step1 succeeds, step2 fails
				runner.results = map[string]engine.RunResult{
					"step1": {ExitCode: 0},
					"step2": {ExitCode: 1, Error: fmt.Errorf("step failed")},
					"step3": {ExitCode: 0},
				}
			})

			It("should mark remaining steps as skipped and execution as failed", func() {
				err := eng.Start(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Wait for execution to fail
				Eventually(func() domain.ExecutionStatus {
					exec, err := executionRepo.Get(ctx, testExecution.ID.String())
					if err != nil {
						return domain.ExecutionPending
					}
					return exec.Status
				}, "2s", "50ms").Should(Equal(domain.ExecutionFailed))

				_ = eng.Stop()

				// Verify execution status
				execution, err := executionRepo.Get(ctx, testExecution.ID.String())
				Expect(err).NotTo(HaveOccurred())
				Expect(execution.Status).To(Equal(domain.ExecutionFailed))
				Expect(execution.Error).To(ContainSubstring("step failed"))

				// Verify step statuses
				steps, err := stepExecutionRepo.ListByExecution(ctx, testExecution.ID.String())
				Expect(err).NotTo(HaveOccurred())
				Expect(steps).To(HaveLen(3))

				var step1, step2, step3 *domain.StepExecution
				for _, step := range steps {
					switch step.StepName {
					case "step1":
						step1 = step
					case "step2":
						step2 = step
					case "step3":
						step3 = step
					}
				}

				Expect(step1.Status).To(Equal(domain.StepExecutionSucceeded))
				Expect(step2.Status).To(Equal(domain.StepExecutionFailed))
				Expect(step3.Status).To(Equal(domain.StepExecutionSkipped))
			})
		})

		Context("when execution is canceled", func() {
			BeforeEach(func() {
				// Configure runner with delay to allow cancellation
				runner.delay = 100 * time.Millisecond
				runner.results = map[string]engine.RunResult{
					"step1": {ExitCode: 0},
					"step2": {ExitCode: 0},
					"step3": {ExitCode: 0},
				}
			})

			It("should stop processing and mark execution as canceled", func() {
				err := eng.Start(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Wait for first step to start, then cancel
				time.Sleep(150 * time.Millisecond)

				// Simulate external cancellation
				err = executionRepo.UpdateStatus(ctx, testExecution.ID.String(), domain.ExecutionCanceled)
				Expect(err).NotTo(HaveOccurred())

				// Wait for engine to detect cancellation
				Eventually(func() int {
					steps, _ := stepExecutionRepo.ListByExecution(ctx, testExecution.ID.String())
					return len(steps)
				}, "2s", "50ms").Should(BeNumerically(">=", 1))

				_ = eng.Stop()

				// Verify not all steps were executed
				steps, err := stepExecutionRepo.ListByExecution(ctx, testExecution.ID.String())
				Expect(err).NotTo(HaveOccurred())
				Expect(steps).To(HaveLen(3)) // step1 + skipped step2 + skipped step3

				// Count skipped steps
				skippedCount := 0
				for _, step := range steps {
					if step.Status == domain.StepExecutionSkipped {
						skippedCount++
					}
				}
				Expect(skippedCount).To(BeNumerically(">=", 1))
			})

			It("should honor context cancellation", func() {
				err := eng.Start(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Wait for first step to start
				time.Sleep(150 * time.Millisecond)

				// Cancel context
				cancel()

				// Wait for engine to detect cancellation
				time.Sleep(200 * time.Millisecond)

				_ = eng.Stop()

				// Verify execution was canceled
				execution, err := executionRepo.Get(context.Background(), testExecution.ID.String())
				Expect(err).NotTo(HaveOccurred())
				Expect(execution.Status).To(Equal(domain.ExecutionCanceled))
			})
		})
	})

	Describe("Concurrency", func() {
		It("should process one execution at a time (serial scheduler)", func() {
			// Create second execution
			execution2 := &domain.Execution{
				ID:         uuid.New(),
				WorkflowID: testWorkflow.ID,
				Status:     domain.ExecutionPending,
			}
			err := executionRepo.Create(ctx, execution2)
			Expect(err).NotTo(HaveOccurred())

			executionRepo.mu.Lock()
			executionRepo.executionsByWorkflow[testWorkflow.ID.String()] = []*domain.Execution{
				testExecution,
				execution2,
			}
			executionRepo.mu.Unlock()

			// Configure runner with delay to verify serial processing
			runner.delay = 100 * time.Millisecond
			runner.results = map[string]engine.RunResult{
				"step1": {ExitCode: 0},
				"step2": {ExitCode: 0},
				"step3": {ExitCode: 0},
			}

			err = eng.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Wait for both executions to complete
			Eventually(func() bool {
				exec1, err1 := executionRepo.Get(ctx, testExecution.ID.String())
				exec2, err2 := executionRepo.Get(ctx, execution2.ID.String())
				if err1 != nil || err2 != nil {
					return false
				}
				return exec1.Status == domain.ExecutionSucceeded && exec2.Status == domain.ExecutionSucceeded
			}, "5s", "100ms").Should(BeTrue())

			_ = eng.Stop()

			// Verify both executions succeeded
			exec1, err := executionRepo.Get(ctx, testExecution.ID.String())
			Expect(err).NotTo(HaveOccurred())
			exec2, err := executionRepo.Get(ctx, execution2.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(exec1.Status).To(Equal(domain.ExecutionSucceeded))
			Expect(exec2.Status).To(Equal(domain.ExecutionSucceeded))

			// Verify total steps executed (3 per execution * 2 executions)
			allSteps1, err := stepExecutionRepo.ListByExecution(ctx, testExecution.ID.String())
			Expect(err).NotTo(HaveOccurred())
			allSteps2, err := stepExecutionRepo.ListByExecution(ctx, execution2.ID.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(len(allSteps1) + len(allSteps2)).To(Equal(6))
		})
	})
})

// Mock implementations

type mockWorkflowRepo struct {
	mu        sync.RWMutex
	workflows map[string]*domain.Workflow
}

func newMockWorkflowRepo() *mockWorkflowRepo {
	return &mockWorkflowRepo{
		workflows: make(map[string]*domain.Workflow),
	}
}

func (m *mockWorkflowRepo) Create(ctx context.Context, workflow *domain.Workflow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workflows[workflow.ID.String()] = workflow
	return nil
}

func (m *mockWorkflowRepo) Get(ctx context.Context, id string) (*domain.Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workflows[id]
	if !ok {
		return nil, fmt.Errorf("workflow not found")
	}
	return w, nil
}

func (m *mockWorkflowRepo) List(ctx context.Context, opts domain.ListOptions) ([]*domain.Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.Workflow, 0, len(m.workflows))
	for _, w := range m.workflows {
		result = append(result, w)
	}
	return result, nil
}

func (m *mockWorkflowRepo) Update(ctx context.Context, workflow *domain.Workflow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workflows[workflow.ID.String()] = workflow
	return nil
}

type mockExecutionRepo struct {
	mu                   sync.RWMutex
	executions           map[string]*domain.Execution
	executionsByWorkflow map[string][]*domain.Execution
}

func newMockExecutionRepo() *mockExecutionRepo {
	return &mockExecutionRepo{
		executions:           make(map[string]*domain.Execution),
		executionsByWorkflow: make(map[string][]*domain.Execution),
	}
}

func (m *mockExecutionRepo) Create(ctx context.Context, execution *domain.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions[execution.ID.String()] = execution
	return nil
}

func (m *mockExecutionRepo) Get(ctx context.Context, id string) (*domain.Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.executions[id]
	if !ok {
		return nil, fmt.Errorf("execution not found")
	}
	// Return a deep copy to allow concurrent updates
	execCopy := *e
	if e.StartedAt != nil {
		startedCopy := *e.StartedAt
		execCopy.StartedAt = &startedCopy
	}
	if e.FinishedAt != nil {
		finishedCopy := *e.FinishedAt
		execCopy.FinishedAt = &finishedCopy
	}
	return &execCopy, nil
}

func (m *mockExecutionRepo) ListByWorkflow(
	ctx context.Context,
	workflowID string,
	opts domain.ListOptions,
) ([]*domain.Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	originals := m.executionsByWorkflow[workflowID]
	// Return copies to prevent concurrent modification
	copies := make([]*domain.Execution, len(originals))
	for i, e := range originals {
		execCopy := *e
		if e.StartedAt != nil {
			startedCopy := *e.StartedAt
			execCopy.StartedAt = &startedCopy
		}
		if e.FinishedAt != nil {
			finishedCopy := *e.FinishedAt
			execCopy.FinishedAt = &finishedCopy
		}
		copies[i] = &execCopy
	}
	return copies, nil
}

func (m *mockExecutionRepo) Update(ctx context.Context, execution *domain.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.executions[execution.ID.String()]
	if !ok {
		return fmt.Errorf("execution not found")
	}
	// Update all fields
	e.Status = execution.Status
	e.StartedAt = execution.StartedAt
	e.FinishedAt = execution.FinishedAt
	e.Error = execution.Error
	return nil
}

func (m *mockExecutionRepo) UpdateStatus(ctx context.Context, id string, status domain.ExecutionStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.executions[id]
	if !ok {
		return fmt.Errorf("execution not found")
	}
	e.Status = status
	return nil
}

type mockStepExecutionRepo struct {
	mu             sync.RWMutex
	stepExecutions []*domain.StepExecution
}

func newMockStepExecutionRepo() *mockStepExecutionRepo {
	return &mockStepExecutionRepo{
		stepExecutions: make([]*domain.StepExecution, 0),
	}
}

func (m *mockStepExecutionRepo) Create(ctx context.Context, stepExecution *domain.StepExecution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stepExecutions = append(m.stepExecutions, stepExecution)
	return nil
}

func (m *mockStepExecutionRepo) ListByExecution(
	ctx context.Context,
	executionID string,
) ([]*domain.StepExecution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.StepExecution, 0)
	for _, se := range m.stepExecutions {
		if se.ExecutionID.String() == executionID {
			result = append(result, se)
		}
	}
	return result, nil
}

func (m *mockStepExecutionRepo) UpdateStatus(
	ctx context.Context,
	id string,
	status domain.StepExecutionStatus,
	exitCode *int,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, se := range m.stepExecutions {
		if se.ID.String() == id {
			se.Status = status
			if exitCode != nil {
				se.ExitCode = exitCode
			}
			return nil
		}
	}
	return fmt.Errorf("step execution not found")
}

type mockRunner struct {
	results map[string]engine.RunResult
	delay   time.Duration
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		results: make(map[string]engine.RunResult),
	}
}

func (m *mockRunner) Execute(ctx context.Context, step *domain.StepDefinition) engine.RunResult {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	result, ok := m.results[step.Name]
	if !ok {
		return engine.RunResult{ExitCode: 0}
	}

	return result
}
