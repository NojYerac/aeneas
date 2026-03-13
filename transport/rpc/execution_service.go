package rpc

import (
	"context"
	"errors"
	"math"

	"github.com/nojyerac/aeneas/domain"
	pb "github.com/nojyerac/aeneas/pb/workflow"
	"github.com/nojyerac/aeneas/service"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ pb.ExecutionServiceServer = (*ExecutionService)(nil)

type ExecutionService struct {
	t   trace.Tracer
	l   logrus.FieldLogger
	svc *service.ExecutionService
	pb.UnimplementedExecutionServiceServer
}

func NewExecutionService(svc *service.ExecutionService, opts ...Option) *ExecutionService {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &ExecutionService{
		t:   o.t,
		l:   o.l,
		svc: svc,
	}
}

func (s *ExecutionService) GetExecution(
	parentCtx context.Context,
	req *pb.GetExecutionRequest,
) (*pb.ExecutionResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.ExecutionService.GetExecution")
	defer span.End()
	execution, steps, err := s.svc.GetWithSteps(ctx, req.GetExecutionId())
	if err != nil {
		return nil, err
	}
	ex, err := toPbExecution(execution, steps)
	if err != nil {
		return nil, toPbError(err)
	}
	return ex, nil
}

func (s *ExecutionService) CancelExecution(
	parentCtx context.Context,
	req *pb.CancelExecutionRequest,
) (*pb.CancelExecutionResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.ExecutionService.CancelExecution")
	defer span.End()
	err := s.svc.Cancel(ctx, req.GetExecutionId())
	if err != nil {
		return nil, toPbError(err)
	}
	return &pb.CancelExecutionResponse{}, nil
}

func (s *ExecutionService) ExecuteWorkflow(
	parentCtx context.Context,
	req *pb.ExecuteWorkflowRequest,
) (*pb.ExecutionResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.ExecutionService.ExecuteWorkflow")
	defer span.End()
	execution, err := s.svc.Trigger(ctx, req.GetWorkflowId())
	if err != nil {
		return nil, toPbError(err)
	}
	return toPbExecution(execution, nil)
}

func (s *ExecutionService) ListWorkflowExecutions(
	parentCtx context.Context,
	req *pb.ListWorkflowExecutionsRequest,
) (*pb.ListWorkflowExecutionsResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.ExecutionService.ListWorkflowExecutions")
	defer span.End()
	executions, err := s.svc.ListByWorkflow(ctx, req.GetWorkflowId(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toPbError(err)
	}
	pbExecutions := make([]*pb.ExecutionResponse, len(executions))
	for i, ex := range executions {
		pbEx, err := toPbExecution(ex, nil)
		if err != nil {
			return nil, err
		}
		pbExecutions[i] = pbEx
	}
	return &pb.ListWorkflowExecutionsResponse{
		Executions: pbExecutions,
	}, nil
}

// toPb helpers

func toPbError(err error) error {
	if err == nil {
		return nil
	}
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		switch svcErr.Type {
		case service.ErrorTypeNotFound:
			return status.Error(codes.NotFound, svcErr.Error())
		case service.ErrorTypeValidation:
			return status.Error(codes.InvalidArgument, svcErr.Error())
		case service.ErrorTypeConflict:
			return status.Error(codes.FailedPrecondition, svcErr.Error())
		case service.ErrorTypeInternal:
			return status.Error(codes.Internal, svcErr.Error())
		default:
			// fall back to internal error for unknown error types
			return status.Error(codes.Internal, svcErr.Error())
		}
	}
	return status.Error(codes.Internal, "an unexpected error occurred")
}

func toPbStepExecutionStatus(st domain.StepExecutionStatus) (pb.StepExecutionStatus, error) {
	switch st {
	case domain.StepExecutionFailed:
		return pb.StepExecutionStatus_STEP_EXECUTION_STATUS_FAILED, nil
	case domain.StepExecutionPending:
		return pb.StepExecutionStatus_STEP_EXECUTION_STATUS_PENDING, nil
	case domain.StepExecutionRunning:
		return pb.StepExecutionStatus_STEP_EXECUTION_STATUS_RUNNING, nil
	case domain.StepExecutionSkipped:
		return pb.StepExecutionStatus_STEP_EXECUTION_STATUS_SKIPPED, nil
	case domain.StepExecutionSucceeded:
		return pb.StepExecutionStatus_STEP_EXECUTION_STATUS_SUCCEEDED, nil

	default:
		return pb.StepExecutionStatus_STEP_EXECUTION_STATUS_UNSPECIFIED,
			status.Error(codes.InvalidArgument, "unknown execution status")
	}
}

func toPbStep(step *domain.StepExecution) (*pb.StepExecutionResponse, error) {
	st, err := toPbStepExecutionStatus(step.Status)
	if err != nil {
		return nil, err
	}

	pbStep := &pb.StepExecutionResponse{
		Id:       step.ID.String(),
		StepName: step.StepName,
		Status:   st,
	}
	var exitCode *int32
	if step.ExitCode != nil {
		exitCode = new(int32)
		if *step.ExitCode > math.MaxInt32 || *step.ExitCode < math.MinInt32 {
			return nil, status.Error(codes.InvalidArgument, "exit code out of int32 range")
		}
		*exitCode = int32(*step.ExitCode)
	}
	pbStep.ExitCode = exitCode
	if step.StartedAt != nil {
		pbStep.StartedAt = timestamppb.New(*step.StartedAt)
	}
	if step.FinishedAt != nil {
		pbStep.FinishedAt = timestamppb.New(*step.FinishedAt)
	}
	return pbStep, nil
}

func toPbExecutionStatus(st domain.ExecutionStatus) (pb.ExecutionStatus, error) {
	switch st {
	case domain.ExecutionFailed:
		return pb.ExecutionStatus_EXECUTION_STATUS_FAILED, nil
	case domain.ExecutionPending:
		return pb.ExecutionStatus_EXECUTION_STATUS_PENDING, nil
	case domain.ExecutionRunning:
		return pb.ExecutionStatus_EXECUTION_STATUS_RUNNING, nil
	case domain.ExecutionSucceeded:
		return pb.ExecutionStatus_EXECUTION_STATUS_SUCCEEDED, nil
	case domain.ExecutionCanceled:
		return pb.ExecutionStatus_EXECUTION_STATUS_CANCELED, nil
	default:
		return pb.ExecutionStatus_EXECUTION_STATUS_UNSPECIFIED,
			status.Error(codes.InvalidArgument, "unknown execution status")
	}
}

func toPbExecution(execution *domain.Execution, steps []*domain.StepExecution) (*pb.ExecutionResponse, error) {
	var pbSteps []*pb.StepExecutionResponse
	for _, step := range steps {
		pbStep, err := toPbStep(step)
		if err != nil {
			return nil, err
		}
		pbSteps = append(pbSteps, pbStep)
	}
	pbStatus, err := toPbExecutionStatus(execution.Status)
	if err != nil {
		return nil, err
	}
	pbEx := &pb.ExecutionResponse{
		Id:         execution.ID.String(),
		WorkflowId: execution.WorkflowID.String(),
		Status:     pbStatus,
		Steps:      pbSteps,
	}
	if execution.Error != "" {
		pbEx.Error = new(string)
		*pbEx.Error = execution.Error
	}
	if execution.StartedAt != nil {
		pbEx.StartedAt = timestamppb.New(*execution.StartedAt)
	}
	if execution.FinishedAt != nil {
		pbEx.FinishedAt = timestamppb.New(*execution.FinishedAt)
	}
	return pbEx, nil
}
