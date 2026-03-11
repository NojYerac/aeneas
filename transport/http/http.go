package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/service"
	"github.com/nojyerac/go-lib/tracing"
	libhttp "github.com/nojyerac/go-lib/transport/http"
	"go.opentelemetry.io/otel/trace"
)

// RegisterRoutes registers all HTTP routes with the server
func RegisterRoutes(srv libhttp.Server, workflowSvc *service.WorkflowService, executionSvc *service.ExecutionService) {
	r := &Routes{
		v:            validator.New(),
		t:            tracing.TracerForPackage(),
		workflowSvc:  workflowSvc,
		executionSvc: executionSvc,
	}

	// Workflow endpoints
	srv.HandleFunc("POST /v1/workflows", r.CreateWorkflow)
	srv.HandleFunc("GET /v1/workflows", r.ListWorkflows)
	srv.HandleFunc("GET /v1/workflows/{id}", r.GetWorkflow)
	srv.HandleFunc("PUT /v1/workflows/{id}", r.UpdateWorkflow)
	srv.HandleFunc("POST /v1/workflows/{id}/activate", r.ActivateWorkflow)
	srv.HandleFunc("POST /v1/workflows/{id}/archive", r.ArchiveWorkflow)

	// Execution endpoints
	srv.HandleFunc("POST /v1/workflows/{id}/executions", r.TriggerExecution)
	srv.HandleFunc("GET /v1/workflows/{id}/executions", r.ListExecutions)
	srv.HandleFunc("GET /v1/executions/{id}", r.GetExecution)
	srv.HandleFunc("POST /v1/executions/{id}/cancel", r.CancelExecution)
}

type Routes struct {
	v            *validator.Validate
	t            trace.Tracer
	workflowSvc  *service.WorkflowService
	executionSvc *service.ExecutionService
}

// DTOs

type CreateWorkflowRequest struct {
	Name        string                  `json:"name" validate:"required,min=1,max=255"`
	Description string                  `json:"description" validate:"max=1000"`
	Steps       []domain.StepDefinition `json:"steps" validate:"dive"`
}

type UpdateWorkflowRequest struct {
	Name        *string                  `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string                  `json:"description,omitempty" validate:"omitempty,max=1000"`
	Steps       *[]domain.StepDefinition `json:"steps,omitempty" validate:"omitempty,dive"`
}

type WorkflowResponse struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Steps       []domain.StepDefinition `json:"steps"`
	Status      string                  `json:"status"`
	CreatedAt   string                  `json:"created_at"`
	UpdatedAt   string                  `json:"updated_at"`
}

type ExecutionResponse struct {
	ID         string                  `json:"id"`
	WorkflowID string                  `json:"workflow_id"`
	Status     string                  `json:"status"`
	StartedAt  *string                 `json:"started_at,omitempty"`
	FinishedAt *string                 `json:"finished_at,omitempty"`
	Error      string                  `json:"error,omitempty"`
	Steps      []StepExecutionResponse `json:"steps,omitempty"`
}

type StepExecutionResponse struct {
	ID         string  `json:"id"`
	StepName   string  `json:"step_name"`
	Status     string  `json:"status"`
	StartedAt  *string `json:"started_at,omitempty"`
	FinishedAt *string `json:"finished_at,omitempty"`
	ExitCode   *int    `json:"exit_code,omitempty"`
	Error      string  `json:"error,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Helper functions

func (r *Routes) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (r *Routes) writeError(w http.ResponseWriter, status int, message, code string) {
	r.writeJSON(w, status, ErrorResponse{
		Error: message,
		Code:  code,
	})
}

func (r *Routes) handleServiceError(w http.ResponseWriter, err error) {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		switch svcErr.Type {
		case service.ErrorTypeNotFound:
			r.writeError(w, http.StatusNotFound, svcErr.Message, "NOT_FOUND")
		case service.ErrorTypeValidation:
			r.writeError(w, http.StatusUnprocessableEntity, svcErr.Message, "VALIDATION_ERROR")
		case service.ErrorTypeConflict:
			r.writeError(w, http.StatusConflict, svcErr.Message, "CONFLICT")
		default:
			r.writeError(w, http.StatusInternalServerError, "Internal server error", "INTERNAL_ERROR")
		}
		return
	}
	r.writeError(w, http.StatusInternalServerError, "Internal server error", "INTERNAL_ERROR")
}

func (r *Routes) decodeJSON(req *http.Request, v interface{}) error {
	if err := json.NewDecoder(req.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}
	if err := r.v.Struct(v); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}

func workflowToResponse(w *domain.Workflow) WorkflowResponse {
	return WorkflowResponse{
		ID:          w.ID.String(),
		Name:        w.Name,
		Description: w.Description,
		Steps:       w.Steps,
		Status:      string(w.Status),
		CreatedAt:   w.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   w.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func executionToResponse(e *domain.Execution, steps []*domain.StepExecution) ExecutionResponse {
	resp := ExecutionResponse{
		ID:         e.ID.String(),
		WorkflowID: e.WorkflowID.String(),
		Status:     string(e.Status),
		Error:      e.Error,
	}

	if e.StartedAt != nil {
		startedAt := e.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.StartedAt = &startedAt
	}
	if e.FinishedAt != nil {
		finishedAt := e.FinishedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.FinishedAt = &finishedAt
	}

	if steps != nil {
		resp.Steps = make([]StepExecutionResponse, len(steps))
		for i, step := range steps {
			stepResp := StepExecutionResponse{
				ID:       step.ID.String(),
				StepName: step.StepName,
				Status:   string(step.Status),
				ExitCode: step.ExitCode,
				Error:    step.Error,
			}
			if step.StartedAt != nil {
				startedAt := step.StartedAt.Format("2006-01-02T15:04:05Z07:00")
				stepResp.StartedAt = &startedAt
			}
			if step.FinishedAt != nil {
				finishedAt := step.FinishedAt.Format("2006-01-02T15:04:05Z07:00")
				stepResp.FinishedAt = &finishedAt
			}
			resp.Steps[i] = stepResp
		}
	}

	return resp
}

// Handlers

func (r *Routes) CreateWorkflow(w http.ResponseWriter, req *http.Request) {
	var createReq CreateWorkflowRequest
	if err := r.decodeJSON(req, &createReq); err != nil {
		r.writeError(w, http.StatusUnprocessableEntity, err.Error(), "VALIDATION_ERROR")
		return
	}

	input := service.CreateWorkflowInput{
		Name:        createReq.Name,
		Description: createReq.Description,
		Steps:       createReq.Steps,
	}

	workflow, err := r.workflowSvc.Create(req.Context(), input)
	if err != nil {
		r.handleServiceError(w, err)
		return
	}

	r.writeJSON(w, http.StatusCreated, workflowToResponse(workflow))
}

func (r *Routes) ListWorkflows(w http.ResponseWriter, req *http.Request) {
	limit := 10
	offset := 0

	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}

	if offsetStr := req.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}

	workflows, err := r.workflowSvc.List(req.Context(), limit, offset)
	if err != nil {
		r.handleServiceError(w, err)
		return
	}

	response := make([]WorkflowResponse, len(workflows))
	for i, wf := range workflows {
		response[i] = workflowToResponse(wf)
	}

	r.writeJSON(w, http.StatusOK, response)
}

func (r *Routes) GetWorkflow(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	workflow, err := r.workflowSvc.Get(req.Context(), id)
	if err != nil {
		r.handleServiceError(w, err)
		return
	}

	r.writeJSON(w, http.StatusOK, workflowToResponse(workflow))
}

func (r *Routes) UpdateWorkflow(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	var updateReq UpdateWorkflowRequest
	if err := r.decodeJSON(req, &updateReq); err != nil {
		r.writeError(w, http.StatusUnprocessableEntity, err.Error(), "VALIDATION_ERROR")
		return
	}

	input := service.UpdateWorkflowInput{
		Name:        updateReq.Name,
		Description: updateReq.Description,
		Steps:       updateReq.Steps,
	}

	workflow, err := r.workflowSvc.Update(req.Context(), id, input)
	if err != nil {
		r.handleServiceError(w, err)
		return
	}

	r.writeJSON(w, http.StatusOK, workflowToResponse(workflow))
}

func (r *Routes) ActivateWorkflow(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	workflow, err := r.workflowSvc.Activate(req.Context(), id)
	if err != nil {
		r.handleServiceError(w, err)
		return
	}

	r.writeJSON(w, http.StatusOK, workflowToResponse(workflow))
}

func (r *Routes) ArchiveWorkflow(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	workflow, err := r.workflowSvc.Archive(req.Context(), id)
	if err != nil {
		r.handleServiceError(w, err)
		return
	}

	r.writeJSON(w, http.StatusOK, workflowToResponse(workflow))
}

func (r *Routes) TriggerExecution(w http.ResponseWriter, req *http.Request) {
	workflowID := req.PathValue("id")

	execution, err := r.executionSvc.Trigger(req.Context(), workflowID)
	if err != nil {
		r.handleServiceError(w, err)
		return
	}

	r.writeJSON(w, http.StatusCreated, executionToResponse(execution, nil))
}

func (r *Routes) ListExecutions(w http.ResponseWriter, req *http.Request) {
	workflowID := req.PathValue("id")

	limit := 10
	offset := 0

	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}

	if offsetStr := req.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}

	executions, err := r.executionSvc.ListByWorkflow(req.Context(), workflowID, limit, offset)
	if err != nil {
		r.handleServiceError(w, err)
		return
	}

	response := make([]ExecutionResponse, len(executions))
	for i, exec := range executions {
		response[i] = executionToResponse(exec, nil)
	}

	r.writeJSON(w, http.StatusOK, response)
}

func (r *Routes) GetExecution(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	execution, steps, err := r.executionSvc.GetWithSteps(req.Context(), id)
	if err != nil {
		r.handleServiceError(w, err)
		return
	}

	r.writeJSON(w, http.StatusOK, executionToResponse(execution, steps))
}

func (r *Routes) CancelExecution(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	if err := r.executionSvc.Cancel(req.Context(), id); err != nil {
		r.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
