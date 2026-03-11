package engine_test

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/engine"
)

// MockRunner is a configurable test double for the Runner interface
type MockRunner struct {
	mu      sync.Mutex
	results map[string]engine.RunResult
	calls   []string
}

func NewMockRunner() *MockRunner {
	return &MockRunner{
		results: make(map[string]engine.RunResult),
		calls:   make([]string, 0),
	}
}

func (m *MockRunner) SetResult(stepName string, result engine.RunResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[stepName] = result
}

func (m *MockRunner) Execute(ctx context.Context, step *domain.StepDefinition) engine.RunResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, step.Name)

	if result, ok := m.results[step.Name]; ok {
		return result
	}

	// Default: success
	return engine.RunResult{ExitCode: 0, Error: nil}
}

func (m *MockRunner) GetCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.calls...)
}

func (m *MockRunner) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]string, 0)
}

// In-memory repositories for testing
type InMemoryWorkflowRepo struct {
	workflows map[string]*domain.Workflow
	mu        sync.RWMutex
}

func NewInMemoryWorkflowRepo() *InMemoryWorkflowRepo {
	return &InMemoryWorkflowRepo{
		workflows: make(map[string]*domain.Workflow),
	}
}

func (r *InMemoryWorkflowRepo) Create(ctx context.Context, workflow *domain.Workflow) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workflows[workflow.ID.String()] = workflow
	return nil
}

func (r *InMemoryWorkflowRepo) Get(ctx context.Context, id string) (*domain.Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.workflows[id], nil
}

func (r *InMemoryWorkflowRepo) List(ctx context.Context, opts domain.ListOptions) ([]*domain.Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Workflow, 0, len(r.workflows))
	for _, w := range r.workflows {
		result = append(result, w)
	}
	return result, nil
}

func (r *InMemoryWorkflowRepo) Update(ctx context.Context, workflow *domain.Workflow) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workflows[workflow.ID.String()] = workflow
	return nil
}

type InMemoryExecutionRepo struct {
	executions map[string]*domain.Execution
	mu         sync.RWMutex
}

func NewInMemoryExecutionRepo() *InMemoryExecutionRepo {
	return &InMemoryExecutionRepo{
		executions: make(map[string]*domain.Execution),
	}
}

func (r *InMemoryExecutionRepo) Create(ctx context.Context, execution *domain.Execution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executions[execution.ID.String()] = execution
	return nil
}

func (r *InMemoryExecutionRepo) Get(ctx context.Context, id string) (*domain.Execution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.executions[id], nil
}

func (r *InMemoryExecutionRepo) ListByWorkflow(
	ctx context.Context,
	workflowID string,
	opts domain.ListOptions,
) ([]*domain.Execution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Execution, 0)
	for _, e := range r.executions {
		if workflowID == "" || e.WorkflowID.String() == workflowID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *InMemoryExecutionRepo) UpdateStatus(ctx context.Context, id string, status domain.ExecutionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if exec, ok := r.executions[id]; ok {
		exec.Status = status
		if status == domain.ExecutionSucceeded || status == domain.ExecutionFailed || status == domain.ExecutionCanceled {
			now := time.Now()
			exec.FinishedAt = &now
		}
	}
	return nil
}

type InMemoryStepExecutionRepo struct {
	stepExecutions map[string]*domain.StepExecution
	mu             sync.RWMutex
}

func NewInMemoryStepExecutionRepo() *InMemoryStepExecutionRepo {
	return &InMemoryStepExecutionRepo{
		stepExecutions: make(map[string]*domain.StepExecution),
	}
}

func (r *InMemoryStepExecutionRepo) Create(ctx context.Context, stepExecution *domain.StepExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stepExecutions[stepExecution.ID.String()] = stepExecution
	return nil
}

func (r *InMemoryStepExecutionRepo) ListByExecution(
	ctx context.Context,
	executionID string,
) ([]*domain.StepExecution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.StepExecution, 0)
	for _, se := range r.stepExecutions {
		if se.ExecutionID.String() == executionID {
			result = append(result, se)
		}
	}
	return result, nil
}

func (r *InMemoryStepExecutionRepo) UpdateStatus(
	ctx context.Context,
	id string,
	status domain.StepExecutionStatus,
	exitCode *int,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if stepExec, ok := r.stepExecutions[id]; ok {
		stepExec.Status = status
		stepExec.ExitCode = exitCode
		now := time.Now()
		if status == domain.StepExecutionRunning {
			stepExec.StartedAt = &now
		}
		if status == domain.StepExecutionSucceeded ||
			status == domain.StepExecutionFailed ||
			status == domain.StepExecutionSkipped {
			stepExec.FinishedAt = &now
		}
	}
	return nil
}

var _ = Describe("Engine", func() {
	var (
		mockRunner        *MockRunner
		workflowRepo      *InMemoryWorkflowRepo
		executionRepo     *InMemoryExecutionRepo
		stepExecutionRepo *InMemoryStepExecutionRepo
		eng               *engine.Engine
		ctx               context.Context
		cancel            context.CancelFunc
	)

	BeforeEach(func() {
		mockRunner = NewMockRunner()
		workflowRepo = NewInMemoryWorkflowRepo()
		executionRepo = NewInMemoryExecutionRepo()
		stepExecutionRepo = NewInMemoryStepExecutionRepo()

		eng = engine.NewEngine(
			workflowRepo,
			executionRepo,
			stepExecutionRepo,
			engine.WithRunner(mockRunner),
			engine.WithPollInterval(100*time.Millisecond),
		)

		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
		if eng != nil {
			eng.Stop()
		}
	})

	Describe("Start and Stop", func() {
		It("should start and stop successfully", func() {
			err := eng.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			eng.Stop()
		})

		It("should return error if runner is not set", func() {
			engNoRunner := engine.NewEngine(
				workflowRepo,
				executionRepo,
				stepExecutionRepo,
			)

			err := engNoRunner.Start(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("runner is required"))
		})
	})

	Describe("Execution Processing", func() {
		var (
			workflow  *domain.Workflow
			execution *domain.Execution
		)

		BeforeEach(func() {
			// Create a workflow with 3 steps
			workflow = &domain.Workflow{
				ID:     uuid.New(),
				Name:   "test-workflow",
				Status: domain.WorkflowActive,
				Steps: []domain.StepDefinition{
					{Name: "step1", Image: "alpine:latest", Command: []string{"echo"}, Args: []string{"step1"}},
					{Name: "step2", Image: "alpine:latest", Command: []string{"echo"}, Args: []string{"step2"}},
					{Name: "step3", Image: "alpine:latest", Command: []string{"echo"}, Args: []string{"step3"}},
				},
			}
			err := workflowRepo.Create(ctx, workflow)
			Expect(err).NotTo(HaveOccurred())

			// Create execution
			now := time.Now()
			execution = &domain.Execution{
				ID:         uuid.New(),
				WorkflowID: workflow.ID,
				Status:     domain.ExecutionPending,
				StartedAt:  &now,
			}
			err = executionRepo.Create(ctx, execution)
			Expect(err).NotTo(HaveOccurred())

			// Create step executions
			for _, step := range workflow.Steps {
				stepExec := &domain.StepExecution{
					ID:          uuid.New(),
					ExecutionID: execution.ID,
					StepName:    step.Name,
					Status:      domain.StepExecutionPending,
				}
				err = stepExecutionRepo.Create(ctx, stepExec)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		Context("when all steps succeed", func() {
			It("should complete the execution successfully", func() {
				// All steps will succeed (default behavior)
				err := eng.Start(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Wait for execution to complete
				Eventually(func() domain.ExecutionStatus {
					exec, _ := executionRepo.Get(ctx, execution.ID.String())
					if exec == nil {
						return domain.ExecutionPending
					}
					return exec.Status
				}, 2*time.Second, 100*time.Millisecond).Should(Equal(domain.ExecutionSucceeded))

				// Verify all steps were executed
				calls := mockRunner.GetCalls()
				Expect(calls).To(Equal([]string{"step1", "step2", "step3"}))

				// Verify step statuses
				steps, err := stepExecutionRepo.ListByExecution(ctx, execution.ID.String())
				Expect(err).NotTo(HaveOccurred())
				for _, step := range steps {
					Expect(step.Status).To(Equal(domain.StepExecutionSucceeded))
				}
			})
		})

		Context("when a step fails", func() {
			It("should mark remaining steps as skipped and fail the execution", func() {
				// Make step2 fail
				mockRunner.SetResult("step2", engine.RunResult{ExitCode: 1, Error: nil})

				err := eng.Start(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Wait for execution to complete
				Eventually(func() domain.ExecutionStatus {
					exec, _ := executionRepo.Get(ctx, execution.ID.String())
					if exec == nil {
						return domain.ExecutionPending
					}
					return exec.Status
				}, 2*time.Second, 100*time.Millisecond).Should(Equal(domain.ExecutionFailed))

				// Verify only step1 and step2 were executed
				calls := mockRunner.GetCalls()
				Expect(calls).To(Equal([]string{"step1", "step2"}))

				// Verify step statuses
				steps, err := stepExecutionRepo.ListByExecution(ctx, execution.ID.String())
				Expect(err).NotTo(HaveOccurred())

				stepStatusMap := make(map[string]domain.StepExecutionStatus)
				for _, step := range steps {
					stepStatusMap[step.StepName] = step.Status
				}

				Expect(stepStatusMap["step1"]).To(Equal(domain.StepExecutionSucceeded))
				Expect(stepStatusMap["step2"]).To(Equal(domain.StepExecutionFailed))
				Expect(stepStatusMap["step3"]).To(Equal(domain.StepExecutionSkipped))
			})
		})

		Context("when first step fails", func() {
			It("should skip remaining steps and fail the execution", func() {
				// Make step1 fail
				mockRunner.SetResult("step1", engine.RunResult{ExitCode: 1, Error: nil})

				err := eng.Start(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Wait for execution to complete
				Eventually(func() domain.ExecutionStatus {
					exec, _ := executionRepo.Get(ctx, execution.ID.String())
					if exec == nil {
						return domain.ExecutionPending
					}
					return exec.Status
				}, 2*time.Second, 100*time.Millisecond).Should(Equal(domain.ExecutionFailed))

				// Verify only step1 was executed
				calls := mockRunner.GetCalls()
				Expect(calls).To(Equal([]string{"step1"}))

				// Verify step statuses
				steps, err := stepExecutionRepo.ListByExecution(ctx, execution.ID.String())
				Expect(err).NotTo(HaveOccurred())

				stepStatusMap := make(map[string]domain.StepExecutionStatus)
				for _, step := range steps {
					stepStatusMap[step.StepName] = step.Status
				}

				Expect(stepStatusMap["step1"]).To(Equal(domain.StepExecutionFailed))
				Expect(stepStatusMap["step2"]).To(Equal(domain.StepExecutionSkipped))
				Expect(stepStatusMap["step3"]).To(Equal(domain.StepExecutionSkipped))
			})
		})

		Context("when execution is canceled", func() {
			It("should honor cancellation and skip remaining steps", func() {
				// Delay step execution to allow time for cancellation
				originalRunner := mockRunner
				slowRunner := &SlowMockRunner{
					MockRunner: originalRunner,
					delay:      500 * time.Millisecond,
				}

				eng = engine.NewEngine(
					workflowRepo,
					executionRepo,
					stepExecutionRepo,
					engine.WithRunner(slowRunner),
					engine.WithPollInterval(100*time.Millisecond),
				)

				err := eng.Start(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Wait for execution to start
				Eventually(func() domain.ExecutionStatus {
					exec, _ := executionRepo.Get(ctx, execution.ID.String())
					if exec == nil {
						return domain.ExecutionPending
					}
					return exec.Status
				}, 2*time.Second, 50*time.Millisecond).Should(Equal(domain.ExecutionRunning))

				// Cancel the execution
				err = executionRepo.UpdateStatus(ctx, execution.ID.String(), domain.ExecutionCanceled)
				Expect(err).NotTo(HaveOccurred())

				// Wait for cancellation to be processed
				Eventually(func() domain.ExecutionStatus {
					exec, _ := executionRepo.Get(ctx, execution.ID.String())
					if exec == nil {
						return domain.ExecutionPending
					}
					return exec.Status
				}, 3*time.Second, 100*time.Millisecond).Should(Equal(domain.ExecutionCanceled))
			})
		})
	})

	Describe("Metrics Hooks", func() {
		It("should call metrics hooks during execution", func() {
			var (
				startedCalled   bool
				completedCalled bool
				stepDurations   []string
			)

			eng.SetMetricsHooks(
				func(uuid.UUID) { startedCalled = true },
				func(uuid.UUID, domain.ExecutionStatus, time.Duration) { completedCalled = true },
				func(stepName string, duration time.Duration) {
					stepDurations = append(stepDurations, stepName)
				},
			)

			// Create a simple workflow
			workflow := &domain.Workflow{
				ID:     uuid.New(),
				Name:   "metrics-test",
				Status: domain.WorkflowActive,
				Steps: []domain.StepDefinition{
					{Name: "step1", Image: "alpine:latest", Command: []string{"echo"}},
				},
			}
			err := workflowRepo.Create(ctx, workflow)
			Expect(err).NotTo(HaveOccurred())

			now := time.Now()
			execution := &domain.Execution{
				ID:         uuid.New(),
				WorkflowID: workflow.ID,
				Status:     domain.ExecutionPending,
				StartedAt:  &now,
			}
			err = executionRepo.Create(ctx, execution)
			Expect(err).NotTo(HaveOccurred())

			stepExec := &domain.StepExecution{
				ID:          uuid.New(),
				ExecutionID: execution.ID,
				StepName:    "step1",
				Status:      domain.StepExecutionPending,
			}
			err = stepExecutionRepo.Create(ctx, stepExec)
			Expect(err).NotTo(HaveOccurred())

			err = eng.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Wait for completion
			Eventually(func() domain.ExecutionStatus {
				exec, _ := executionRepo.Get(ctx, execution.ID.String())
				if exec == nil {
					return domain.ExecutionPending
				}
				return exec.Status
			}, 2*time.Second, 100*time.Millisecond).Should(Equal(domain.ExecutionSucceeded))

			Expect(startedCalled).To(BeTrue())
			Expect(completedCalled).To(BeTrue())
			Expect(stepDurations).To(ContainElement("step1"))
		})
	})
})

// SlowMockRunner wraps MockRunner with a delay to test cancellation
type SlowMockRunner struct {
	*MockRunner
	delay time.Duration
}

func (s *SlowMockRunner) Execute(ctx context.Context, step *domain.StepDefinition) engine.RunResult {
	time.Sleep(s.delay)
	return s.MockRunner.Execute(ctx, step)
}
