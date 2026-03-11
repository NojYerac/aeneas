package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Engine is the workflow execution orchestrator.
// It polls for pending executions and processes them step-by-step.
type Engine struct {
	workflowRepo      domain.WorkflowRepository
	executionRepo     domain.ExecutionRepository
	stepExecutionRepo domain.StepExecutionRepository
	runner            Runner
	opts              Options

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// New creates a new execution engine
func New(
	workflowRepo domain.WorkflowRepository,
	executionRepo domain.ExecutionRepository,
	stepExecutionRepo domain.StepExecutionRepository,
	runner Runner,
	options ...Option,
) *Engine {
	opts := defaultOptions()
	for _, opt := range options {
		opt(&opts)
	}

	return &Engine{
		workflowRepo:      workflowRepo,
		executionRepo:     executionRepo,
		stepExecutionRepo: stepExecutionRepo,
		runner:            runner,
		opts:              opts,
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
	}
}

// Start begins the engine's poll loop
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}
	e.running = true
	e.mu.Unlock()

	e.opts.Logger.Info("Engine starting")
	go e.pollLoop(ctx)

	return nil
}

// Stop gracefully shuts down the engine
func (e *Engine) Stop() error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine not running")
	}
	e.mu.Unlock()

	e.opts.Logger.Info("Engine stopping")
	close(e.stopCh)
	<-e.doneCh
	e.opts.Logger.Info("Engine stopped")

	e.mu.Lock()
	e.running = false
	e.mu.Unlock()

	return nil
}

// pollLoop continuously checks for pending executions
func (e *Engine) pollLoop(ctx context.Context) {
	defer close(e.doneCh)

	ticker := time.NewTicker(e.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.opts.Logger.Info("Context canceled, stopping poll loop")
			return
		case <-e.stopCh:
			e.opts.Logger.Info("Stop signal received, stopping poll loop")
			return
		case <-ticker.C:
			e.processPendingExecutions(ctx)
		}
	}
}

// processPendingExecutions finds and processes all pending executions
func (e *Engine) processPendingExecutions(ctx context.Context) {
	// For now, we query all workflows and check for pending executions
	// A more efficient approach would be to have a dedicated query for pending executions
	workflows, err := e.workflowRepo.List(ctx, domain.ListOptions{Limit: 100})
	if err != nil {
		e.opts.Logger.WithError(err).Error("Failed to list workflows")
		return
	}

	for _, workflow := range workflows {
		executions, err := e.executionRepo.ListByWorkflow(ctx, workflow.ID.String(), domain.ListOptions{Limit: 10})
		if err != nil {
			e.opts.Logger.WithError(err).WithField("workflow_id", workflow.ID).Error("Failed to list executions")
			continue
		}

		for _, execution := range executions {
			if execution.Status == domain.ExecutionPending {
				if err := e.processExecution(ctx, workflow, execution); err != nil {
					e.opts.Logger.WithError(err).WithField("execution_id", execution.ID).Error("Failed to process execution")
				}
			}
		}
	}
}

// processExecution orchestrates a single workflow execution
func (e *Engine) processExecution(ctx context.Context, workflow *domain.Workflow, execution *domain.Execution) error {
	spanCtx, span := e.opts.Tracer.Start(ctx, "engine.processExecution",
		trace.WithAttributes(
			attribute.String("execution_id", execution.ID.String()),
			attribute.String("workflow_id", workflow.ID.String()),
		))
	defer span.End()

	logger := e.opts.Logger.WithFields(logrus.Fields{
		"execution_id": execution.ID,
		"workflow_id":  workflow.ID,
	})

	logger.Info("Starting execution")

	// Transition to Running
	if err := e.executionRepo.UpdateStatus(spanCtx, execution.ID.String(), domain.ExecutionRunning); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update execution status")
		return fmt.Errorf("failed to transition execution to running: %w", err)
	}

	now := time.Now()
	execution.StartedAt = &now
	execution.Status = domain.ExecutionRunning

	// Process steps sequentially
	for i, stepDef := range workflow.Steps {
		// Check for cancellation before each step
		select {
		case <-ctx.Done():
			logger.Info("Context canceled, marking execution as canceled")
			if err := e.executionRepo.UpdateStatus(spanCtx, execution.ID.String(), domain.ExecutionCanceled); err != nil {
				logger.WithError(err).Error("Failed to mark execution as canceled")
			}
			return ctx.Err()
		default:
		}

		// Check if execution was canceled externally
		currentExecution, err := e.executionRepo.Get(spanCtx, execution.ID.String())
		if err != nil {
			logger.WithError(err).Error("Failed to get execution status")
			return fmt.Errorf("failed to get execution status: %w", err)
		}

		if currentExecution.Status == domain.ExecutionCanceled {
			logger.Info("Execution was canceled externally")
			// Mark remaining steps as skipped
			e.skipRemainingSteps(spanCtx, execution, workflow.Steps[i:], logger)
			return nil
		}

		if err := e.processStep(spanCtx, execution, &stepDef, logger); err != nil {
			// Step failed - mark remaining steps as skipped and execution as failed
			logger.WithError(err).WithField("step", stepDef.Name).Error("Step failed")
			e.skipRemainingSteps(spanCtx, execution, workflow.Steps[i+1:], logger)

			finishedAt := time.Now()
			execution.FinishedAt = &finishedAt
			execution.Status = domain.ExecutionFailed
			execution.Error = err.Error()

			updateErr := e.executionRepo.UpdateStatus(
				spanCtx,
				execution.ID.String(),
				domain.ExecutionFailed,
			)
			if updateErr != nil {
				logger.WithError(updateErr).Error("Failed to mark execution as failed")
			}

			span.RecordError(err)
			span.SetStatus(codes.Error, "step execution failed")
			return err
		}
	}

	// All steps succeeded
	finishedAt := time.Now()
	execution.FinishedAt = &finishedAt
	execution.Status = domain.ExecutionSucceeded

	if err := e.executionRepo.UpdateStatus(spanCtx, execution.ID.String(), domain.ExecutionSucceeded); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to mark execution as succeeded")
		return fmt.Errorf("failed to mark execution as succeeded: %w", err)
	}

	logger.Info("Execution completed successfully")
	span.SetStatus(codes.Ok, "execution completed")
	return nil
}

// processStep executes a single workflow step
func (e *Engine) processStep(
	ctx context.Context,
	execution *domain.Execution,
	step *domain.StepDefinition,
	logger *logrus.Entry,
) error {
	ctx, span := e.opts.Tracer.Start(ctx, "engine.processStep",
		trace.WithAttributes(
			attribute.String("step_name", step.Name),
		))
	defer span.End()

	stepLogger := logger.WithField("step", step.Name)
	stepLogger.Info("Starting step")

	// Create StepExecution record
	stepExecution := &domain.StepExecution{
		ID:          uuid.New(),
		ExecutionID: execution.ID,
		StepName:    step.Name,
		Status:      domain.StepExecutionPending,
	}

	if err := e.stepExecutionRepo.Create(ctx, stepExecution); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create step execution")
		return fmt.Errorf("failed to create step execution: %w", err)
	}

	// Transition to Running
	err := e.stepExecutionRepo.UpdateStatus(
		ctx,
		stepExecution.ID.String(),
		domain.StepExecutionRunning,
		nil,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update step status")
		return fmt.Errorf("failed to transition step to running: %w", err)
	}

	startedAt := time.Now()
	stepExecution.StartedAt = &startedAt
	stepExecution.Status = domain.StepExecutionRunning

	// Execute the step via Runner
	result := e.runner.Execute(ctx, step)

	finishedAt := time.Now()
	stepExecution.FinishedAt = &finishedAt
	stepExecution.ExitCode = &result.ExitCode

	// Update step status based on result
	if result.Error != nil || result.ExitCode != 0 {
		stepExecution.Status = domain.StepExecutionFailed
		stepExecution.Error = ""
		if result.Error != nil {
			stepExecution.Error = result.Error.Error()
		}

		updateErr := e.stepExecutionRepo.UpdateStatus(
			ctx,
			stepExecution.ID.String(),
			domain.StepExecutionFailed,
			&result.ExitCode,
		)
		if updateErr != nil {
			stepLogger.WithError(updateErr).Error("Failed to mark step as failed")
		}

		span.RecordError(result.Error)
		span.SetStatus(codes.Error, "step execution failed")
		return fmt.Errorf("step %s failed with exit code %d: %w", step.Name, result.ExitCode, result.Error)
	}

	stepExecution.Status = domain.StepExecutionSucceeded

	err = e.stepExecutionRepo.UpdateStatus(
		ctx,
		stepExecution.ID.String(),
		domain.StepExecutionSucceeded,
		&result.ExitCode,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update step status")
		return fmt.Errorf("failed to mark step as succeeded: %w", err)
	}

	duration := finishedAt.Sub(startedAt)
	stepLogger.WithField("duration", duration).Info("Step completed successfully")
	span.SetAttributes(attribute.Int64("duration_ms", duration.Milliseconds()))
	span.SetStatus(codes.Ok, "step completed")

	return nil
}

// skipRemainingSteps marks all remaining steps as skipped
func (e *Engine) skipRemainingSteps(
	ctx context.Context,
	execution *domain.Execution,
	remainingSteps []domain.StepDefinition,
	logger *logrus.Entry,
) {
	for _, step := range remainingSteps {
		stepExecution := &domain.StepExecution{
			ID:          uuid.New(),
			ExecutionID: execution.ID,
			StepName:    step.Name,
			Status:      domain.StepExecutionSkipped,
		}

		if err := e.stepExecutionRepo.Create(ctx, stepExecution); err != nil {
			logger.WithError(err).WithField("step", step.Name).Error("Failed to create skipped step execution")
			continue
		}

		updateErr := e.stepExecutionRepo.UpdateStatus(
			ctx,
			stepExecution.ID.String(),
			domain.StepExecutionSkipped,
			nil,
		)
		if updateErr != nil {
			logger.WithError(updateErr).
				WithField("step", step.Name).
				Error("Failed to mark step as skipped")
		}
	}
}
