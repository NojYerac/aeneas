package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

// Engine is the core execution engine that processes workflow executions
type Engine struct {
	workflowRepo      domain.WorkflowRepository
	executionRepo     domain.ExecutionRepository
	stepExecutionRepo domain.StepExecutionRepository
	runner            Runner

	pollInterval time.Duration
	logger       *logrus.Logger
	tracer       trace.Tracer

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics hooks
	onExecutionStarted   func(executionID uuid.UUID)
	onExecutionCompleted func(executionID uuid.UUID, status domain.ExecutionStatus, duration time.Duration)
	onStepDuration       func(stepName string, duration time.Duration)
}

// NewEngine creates a new execution engine
func NewEngine(
	workflowRepo domain.WorkflowRepository,
	executionRepo domain.ExecutionRepository,
	stepExecutionRepo domain.StepExecutionRepository,
	opts ...Option,
) *Engine {
	e := &Engine{
		workflowRepo:      workflowRepo,
		executionRepo:     executionRepo,
		stepExecutionRepo: stepExecutionRepo,
		pollInterval:      5 * time.Second,
		logger:            logrus.New(),
	}

	for _, opt := range opts {
		opt(e)
	}

	// Set default no-op metrics hooks if not provided
	if e.onExecutionStarted == nil {
		e.onExecutionStarted = func(uuid.UUID) {}
	}
	if e.onExecutionCompleted == nil {
		e.onExecutionCompleted = func(uuid.UUID, domain.ExecutionStatus, time.Duration) {}
	}
	if e.onStepDuration == nil {
		e.onStepDuration = func(string, time.Duration) {}
	}

	return e
}

// SetMetricsHooks configures the metrics callback functions
func (e *Engine) SetMetricsHooks(
	onExecutionStarted func(uuid.UUID),
	onExecutionCompleted func(uuid.UUID, domain.ExecutionStatus, time.Duration),
	onStepDuration func(string, time.Duration),
) {
	if onExecutionStarted != nil {
		e.onExecutionStarted = onExecutionStarted
	}
	if onExecutionCompleted != nil {
		e.onExecutionCompleted = onExecutionCompleted
	}
	if onStepDuration != nil {
		e.onStepDuration = onStepDuration
	}
}

// Start begins the engine's poll loop
func (e *Engine) Start(ctx context.Context) error {
	if e.runner == nil {
		return fmt.Errorf("runner is required")
	}

	e.ctx, e.cancel = context.WithCancel(ctx)
	e.wg.Add(1)

	go e.pollLoop()

	e.logger.Info("engine started")
	return nil
}

// Stop gracefully shuts down the engine
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
	e.logger.Info("engine stopped")
}

// pollLoop continuously polls for pending executions
func (e *Engine) pollLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.processPendingExecutions()
		}
	}
}

// processPendingExecutions finds and processes all pending executions
func (e *Engine) processPendingExecutions() {
	ctx := e.ctx

	// Query for pending executions
	executions, err := e.executionRepo.ListByWorkflow(ctx, "", domain.ListOptions{
		Limit:   100,
		Offset:  0,
		OrderBy: "started_at ASC",
	})
	if err != nil {
		e.logger.WithError(err).Error("failed to list executions")
		return
	}

	// Filter for pending executions
	for _, execution := range executions {
		if execution.Status == domain.ExecutionPending {
			// Process one execution at a time (serial scheduler)
			e.processExecution(ctx, execution)
		}
	}
}

// processExecution orchestrates the execution of a single workflow
func (e *Engine) processExecution(ctx context.Context, execution *domain.Execution) {
	executionID := execution.ID.String()
	startTime := time.Now()

	e.logger.WithField("execution_id", executionID).Info("processing execution")
	e.onExecutionStarted(execution.ID)

	// Transition to Running
	if err := e.executionRepo.UpdateStatus(ctx, executionID, domain.ExecutionRunning); err != nil {
		e.logger.WithError(err).WithField("execution_id", executionID).Error("failed to update execution status to running")
		return
	}

	// Get workflow definition
	workflow, err := e.workflowRepo.Get(ctx, execution.WorkflowID.String())
	if err != nil {
		e.failExecution(ctx, executionID, fmt.Sprintf("failed to get workflow: %v", err))
		e.onExecutionCompleted(execution.ID, domain.ExecutionFailed, time.Since(startTime))
		return
	}

	// Get step executions
	stepExecutions, err := e.stepExecutionRepo.ListByExecution(ctx, executionID)
	if err != nil {
		e.failExecution(ctx, executionID, fmt.Sprintf("failed to list step executions: %v", err))
		e.onExecutionCompleted(execution.ID, domain.ExecutionFailed, time.Since(startTime))
		return
	}

	// Create a map for quick lookup
	stepExecMap := make(map[string]*domain.StepExecution)
	for _, se := range stepExecutions {
		stepExecMap[se.StepName] = se
	}

	// Execute steps sequentially
	for i, stepDef := range workflow.Steps {
		// Check for cancellation between steps
		select {
		case <-ctx.Done():
			e.logger.WithField("execution_id", executionID).Info("execution canceled")
			return
		default:
		}

		// Check if execution was canceled
		currentExec, err := e.executionRepo.Get(ctx, executionID)
		if err != nil {
			e.logger.WithError(err).WithField("execution_id", executionID).Error("failed to get execution status")
			continue
		}
		if currentExec.Status == domain.ExecutionCanceled {
			e.skipRemainingSteps(ctx, workflow.Steps[i:], stepExecMap)
			e.onExecutionCompleted(execution.ID, domain.ExecutionCanceled, time.Since(startTime))
			return
		}

		stepExec := stepExecMap[stepDef.Name]
		if stepExec == nil {
			e.logger.WithField("step_name", stepDef.Name).Error("step execution not found")
			continue
		}

		// Execute the step
		if !e.executeStep(ctx, &stepDef, stepExec) {
			// Step failed - skip remaining steps and mark execution as failed
			e.skipRemainingSteps(ctx, workflow.Steps[i+1:], stepExecMap)
			e.failExecution(ctx, executionID, fmt.Sprintf("step %s failed", stepDef.Name))
			e.onExecutionCompleted(execution.ID, domain.ExecutionFailed, time.Since(startTime))
			return
		}
	}

	// All steps succeeded
	if err := e.executionRepo.UpdateStatus(ctx, executionID, domain.ExecutionSucceeded); err != nil {
		e.logger.WithError(err).WithField("execution_id", executionID).Error("failed to update execution status to succeeded")
		return
	}

	e.logger.WithField("execution_id", executionID).Info("execution succeeded")
	e.onExecutionCompleted(execution.ID, domain.ExecutionSucceeded, time.Since(startTime))
}

// executeStep runs a single step and updates its status
func (e *Engine) executeStep(
	ctx context.Context,
	stepDef *domain.StepDefinition,
	stepExec *domain.StepExecution,
) bool {
	stepID := stepExec.ID.String()
	stepStartTime := time.Now()

	e.logger.WithField("step_name", stepDef.Name).Info("executing step")

	// Transition to Running
	if err := e.stepExecutionRepo.UpdateStatus(ctx, stepID, domain.StepExecutionRunning, nil); err != nil {
		e.logger.WithError(err).WithField("step_id", stepID).Error("failed to update step status to running")
		return false
	}

	// Execute the step using the runner
	result := e.runner.Execute(ctx, stepDef)

	stepDuration := time.Since(stepStartTime)
	e.onStepDuration(stepDef.Name, stepDuration)

	// Update status based on result
	if result.Success() {
		exitCode := result.ExitCode
		if err := e.stepExecutionRepo.UpdateStatus(ctx, stepID, domain.StepExecutionSucceeded, &exitCode); err != nil {
			e.logger.WithError(err).WithField("step_id", stepID).Error("failed to update step status to succeeded")
			return false
		}
		e.logger.WithFields(logrus.Fields{
			"step_name": stepDef.Name,
			"duration":  stepDuration,
		}).Info("step succeeded")
		return true
	}

	// Step failed
	exitCode := result.ExitCode
	if err := e.stepExecutionRepo.UpdateStatus(ctx, stepID, domain.StepExecutionFailed, &exitCode); err != nil {
		e.logger.WithError(err).WithField("step_id", stepID).Error("failed to update step status to failed")
	}

	e.logger.WithFields(logrus.Fields{
		"step_name": stepDef.Name,
		"exit_code": exitCode,
		"error":     result.Error,
		"duration":  stepDuration,
	}).Error("step failed")

	return false
}

// skipRemainingSteps marks all remaining steps as skipped
func (e *Engine) skipRemainingSteps(
	ctx context.Context,
	steps []domain.StepDefinition,
	stepExecMap map[string]*domain.StepExecution,
) {
	for _, stepDef := range steps {
		stepExec := stepExecMap[stepDef.Name]
		if stepExec == nil {
			continue
		}

		if err := e.stepExecutionRepo.UpdateStatus(ctx, stepExec.ID.String(), domain.StepExecutionSkipped, nil); err != nil {
			e.logger.WithError(err).WithField("step_name", stepDef.Name).Error("failed to update step status to skipped")
		}
	}
}

// failExecution marks an execution as failed
func (e *Engine) failExecution(ctx context.Context, executionID, errorMsg string) {
	if err := e.executionRepo.UpdateStatus(ctx, executionID, domain.ExecutionFailed); err != nil {
		e.logger.WithError(err).WithField("execution_id", executionID).Error("failed to update execution status to failed")
	}
	e.logger.WithFields(logrus.Fields{
		"execution_id": executionID,
		"error":        errorMsg,
	}).Error("execution failed")
}
