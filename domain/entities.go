package domain

import (
	"time"

	"github.com/google/uuid"
)

// StepDefinition defines a single step in a workflow
type StepDefinition struct {
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	Command        []string          `json:"command"`
	Args           []string          `json:"args"`
	Env            map[string]string `json:"env"`
	TimeoutSeconds int               `json:"timeout_seconds"`
}

// Workflow represents a workflow definition
type Workflow struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Steps       []StepDefinition `json:"steps"`
	Status      WorkflowStatus   `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// Execution represents a workflow execution instance
type Execution struct {
	ID         uuid.UUID       `json:"id"`
	WorkflowID uuid.UUID       `json:"workflow_id"`
	Status     ExecutionStatus `json:"status"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// StepExecution represents a single step execution within a workflow execution
type StepExecution struct {
	ID          uuid.UUID           `json:"id"`
	ExecutionID uuid.UUID           `json:"execution_id"`
	StepName    string              `json:"step_name"`
	Status      StepExecutionStatus `json:"status"`
	StartedAt   *time.Time          `json:"started_at,omitempty"`
	FinishedAt  *time.Time          `json:"finished_at,omitempty"`
	ExitCode    *int                `json:"exit_code,omitempty"`
	Error       string              `json:"error,omitempty"`
}
