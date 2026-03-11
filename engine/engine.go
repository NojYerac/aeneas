package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner"
	"github.com/sirupsen/logrus"
)

// Engine orchestrates workflow execution lifecycle
type Engine struct {
	workflowRepo      domain.WorkflowRepository
	executionRepo     domain.ExecutionRepository
	stepExecutionRepo domain.StepExecutionRepository
	runner            runner.Runner
	logger            *logrus.Logger
	pollInterval      time.Duration
	wg                sync.WaitGroup
	stopChan          chan struct{}
	mu                sync.Mutex
}

// Config holds engine configuration
type Config struct {
	PollInterval time.Duration
}

// NewEngine creates a new Engine instance
func NewEngine(
	workflowRepo domain.WorkflowRepository,
	executionRepo domain.ExecutionRepository,
	stepExecutionRepo domain.StepExecutionRepository,
	runner runner.Runner,
	logger *logrus.Logger,
	cfg Config,
) *Engine {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 2 * time.Second
	}

	return &Engine{
		workflowRepo:      workflowRepo,
		executionRepo:     executionRepo,
		stepExecutionRepo: stepExecutionRepo,
		runner:            runner,
		logger:            logger,
		pollInterval:      cfg.PollInterval,
		stopChan:          make(chan struct{}),
	}
}

// Start begins the engine's execution loop
func (e *Engine) Start(ctx context.Context) error {
	e.logger.Info("Engine starting")

	e.wg.Add(1)
	go e.pollLoop(ctx)

	return nil
}

// Stop gracefully shuts down the engine
func (e *Engine) Stop(ctx context.Context) error {
	e.logger.Info("Engine stopping")
	close(e.stopChan)
	e.wg.Wait()
	e.logger.Info("Engine stopped")
	return nil
}

// pollLoop continuously polls for pending executions
func (e *Engine) pollLoop(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-ticker.C:
			if err := e.processPendingExecutions(ctx); err != nil {
				e.logger.WithError(err).Error("Failed to process pending executions")
			}
		}
	}
}

// processPendingExecutions finds and processes all pending executions
func (e *Engine) processPendingExecutions(ctx context.Context) error {
	// Get all workflows to find their pending executions
	workflows, err := e.workflowRepo.List(ctx, domain.ListOptions{Limit: 100})
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	for _, workflow := range workflows {
		executions, err := e.executionRepo.ListByWorkflow(ctx, workflow.ID.String(), domain.ListOptions{Limit: 100})
		if err != nil {
			e.logger.WithError(err).WithField("workflow_id", workflow.ID).Error("Failed to list executions")
			continue
		}

		for _, execution := range executions {
			if execution.Status == domain.ExecutionPending {
				e.wg.Add(1)
				go e.processExecution(ctx, workflow, execution)
			}
		}
	}

	return nil
}

// processExecution executes a single workflow execution
func (e *Engine) processExecution(ctx context.Context, workflow *domain.Workflow, execution *domain.Execution) {
	defer e.wg.Done()

	executionID := execution.ID.String()
	logger := e.logger.WithFields(logrus.Fields{
		"execution_id": executionID,
		"workflow_id":  workflow.ID,
	})

	logger.Info("Processing execution")

	// Mark execution as running
	if err := e.executionRepo.UpdateStatus(ctx, executionID, domain.ExecutionRunning); err != nil {
		logger.WithError(err).Error("Failed to update execution status to Running")
		return
	}

	// Get step executions
	steps, err := e.stepExecutionRepo.ListByExecution(ctx, executionID)
	if err != nil {
		logger.WithError(err).Error("Failed to list step executions")
		e.failExecution(ctx, executionID, "failed to list step executions")
		return
	}

	// Execute steps sequentially
	for i, step := range steps {
		// Check if execution was canceled
		currentExecution, err := e.executionRepo.Get(ctx, executionID)
		if err != nil {
			logger.WithError(err).Error("Failed to get execution status")
			break
		}
		if currentExecution.Status == domain.ExecutionCanceled {
			logger.Info("Execution canceled, skipping remaining steps")
			e.skipRemainingSteps(ctx, steps[i:])
			return
		}

		// Find the corresponding step definition
		var stepDef *domain.StepDefinition
		for _, def := range workflow.Steps {
			if def.Name == step.StepName {
				stepDef = &def
				break
			}
		}

		if stepDef == nil {
			logger.WithField("step_name", step.StepName).Error("Step definition not found")
			e.failExecution(ctx, executionID, fmt.Sprintf("step definition not found: %s", step.StepName))
			e.skipRemainingSteps(ctx, steps[i+1:])
			return
		}

		// Execute the step
		if err := e.executeStep(ctx, step, *stepDef); err != nil {
			logger.WithError(err).WithField("step_name", step.StepName).Error("Step execution failed")
			e.failExecution(ctx, executionID, fmt.Sprintf("step %s failed: %v", step.StepName, err))
			e.skipRemainingSteps(ctx, steps[i+1:])
			return
		}

		// Check if step failed (non-zero exit code)
		updatedStep, err := e.stepExecutionRepo.ListByExecution(ctx, executionID)
		if err == nil {
			for _, s := range updatedStep {
				if s.ID == step.ID && s.ExitCode != nil && *s.ExitCode != 0 {
					logger.WithFields(logrus.Fields{
						"step_name": step.StepName,
						"exit_code": *s.ExitCode,
					}).Info("Step failed with non-zero exit code")
					e.failExecution(ctx, executionID, fmt.Sprintf("step %s failed with exit code %d", step.StepName, *s.ExitCode))
					e.skipRemainingSteps(ctx, steps[i+1:])
					return
				}
			}
		}
	}

	// Mark execution as succeeded
	if err := e.executionRepo.UpdateStatus(ctx, executionID, domain.ExecutionSucceeded); err != nil {
		logger.WithError(err).Error("Failed to update execution status to Succeeded")
		return
	}

	now := time.Now()
	execution.FinishedAt = &now
	logger.Info("Execution completed successfully")
}

// executeStep runs a single step and updates its status
func (e *Engine) executeStep(ctx context.Context, step *domain.StepExecution, stepDef domain.StepDefinition) error {
	stepID := step.ID.String()
	logger := e.logger.WithFields(logrus.Fields{
		"step_id":   stepID,
		"step_name": step.StepName,
	})

	logger.Info("Executing step")

	// Mark step as running
	now := time.Now()
	step.StartedAt = &now
	if err := e.stepExecutionRepo.UpdateStatus(ctx, stepID, domain.StepExecutionRunning, nil); err != nil {
		return fmt.Errorf("failed to update step status to Running: %w", err)
	}

	// Execute the step using the runner
	result, err := e.runner.Execute(ctx, stepDef)
	finishedAt := time.Now()
	step.FinishedAt = &finishedAt

	if err != nil {
		logger.WithError(err).Error("Runner execution failed")
		if err := e.stepExecutionRepo.UpdateStatus(ctx, stepID, domain.StepExecutionFailed, nil); err != nil {
			logger.WithError(err).Error("Failed to update step status to Failed")
		}
		return fmt.Errorf("runner execution failed: %w", err)
	}

	// Update step with result
	exitCode := result.ExitCode
	step.ExitCode = &exitCode

	var status domain.StepExecutionStatus
	if exitCode == 0 {
		status = domain.StepExecutionSucceeded
		logger.Info("Step succeeded")
	} else {
		status = domain.StepExecutionFailed
		logger.WithField("exit_code", exitCode).Info("Step failed")
	}

	if err := e.stepExecutionRepo.UpdateStatus(ctx, stepID, status, &exitCode); err != nil {
		return fmt.Errorf("failed to update step status: %w", err)
	}

	return nil
}

// failExecution marks an execution as failed
func (e *Engine) failExecution(ctx context.Context, executionID, errorMsg string) {
	if err := e.executionRepo.UpdateStatus(ctx, executionID, domain.ExecutionFailed); err != nil {
		e.logger.WithError(err).WithField("execution_id", executionID).Error("Failed to update execution status to Failed")
	}
}

// skipRemainingSteps marks remaining steps as skipped
func (e *Engine) skipRemainingSteps(ctx context.Context, steps []*domain.StepExecution) {
	for _, step := range steps {
		if err := e.stepExecutionRepo.UpdateStatus(ctx, step.ID.String(), domain.StepExecutionSkipped, nil); err != nil {
			e.logger.WithError(err).WithField("step_id", step.ID).Error("Failed to skip step")
		}
	}
}
