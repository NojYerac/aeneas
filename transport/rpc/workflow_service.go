package rpc

import (
	"context"
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

var _ pb.WorkflowServiceServer = (*WorkflowService)(nil)

type WorkflowService struct {
	t   trace.Tracer
	l   logrus.FieldLogger
	svc *service.WorkflowService
	pb.UnimplementedWorkflowServiceServer
}

func NewWorkflowService(svc *service.WorkflowService, opts ...Option) *WorkflowService {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &WorkflowService{
		t:   o.t,
		l:   o.l,
		svc: svc,
	}
}

func (s *WorkflowService) ActivateWorkflow(
	parentCtx context.Context,
	req *pb.ActivateWorkflowRequest,
) (*pb.WorkflowResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.WorkflowService.ActivateWorkflow")
	defer span.End()
	workflow, err := s.svc.Activate(ctx, req.GetWorkflowId())
	if err != nil {
		return nil, toPbError(err)
	}
	wf, err := toPbWorkflow(workflow)
	if err != nil {
		return nil, err
	}
	return wf, nil
}

func (s *WorkflowService) ArchiveWorkflow(
	parentCtx context.Context,
	req *pb.ArchiveWorkflowRequest,
) (*pb.WorkflowResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.WorkflowService.ArchiveWorkflow")
	defer span.End()
	workflow, err := s.svc.Archive(ctx, req.GetWorkflowId())
	if err != nil {
		return nil, toPbError(err)
	}
	wf, err := toPbWorkflow(workflow)
	if err != nil {
		return nil, err
	}
	return wf, nil
}

func (s *WorkflowService) CreateWorkflow(
	parentCtx context.Context,
	req *pb.CreateWorkflowRequest,
) (*pb.CreateWorkflowResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.WorkflowService.CreateWorkflow")
	defer span.End()
	workflow, err := s.svc.Create(ctx, service.CreateWorkflowInput{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Steps:       toDomainSteps(req.GetSteps()),
	})
	if err != nil {
		return nil, toPbError(err)
	}
	return &pb.CreateWorkflowResponse{WorkflowId: workflow.ID.String()}, nil
}

func (s *WorkflowService) GetWorkflow(
	parentCtx context.Context,
	req *pb.GetWorkflowByIdRequest,
) (*pb.WorkflowResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.WorkflowService.GetWorkflow")
	defer span.End()
	workflow, err := s.svc.Get(ctx, req.GetWorkflowId())
	if err != nil {
		return nil, toPbError(err)
	}
	if workflow == nil {
		return nil, status.Error(codes.NotFound, "workflow not found")
	}
	wf, err := toPbWorkflow(workflow)
	if err != nil {
		return nil, err
	}
	return wf, nil
}

func (s *WorkflowService) ListWorkflows(
	parentCtx context.Context,
	req *pb.ListWorkflowsRequest,
) (*pb.ListWorkflowsResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.WorkflowService.ListWorkflows")
	defer span.End()
	workflows, err := s.svc.List(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toPbError(err)
	}
	pbWorkflows := make([]*pb.WorkflowResponse, len(workflows))
	for i, wf := range workflows {
		pbWf, err := toPbWorkflow(wf)
		if err != nil {
			return nil, err
		}
		pbWorkflows[i] = pbWf
	}
	return &pb.ListWorkflowsResponse{
		Workflows: pbWorkflows,
	}, nil
}

func (s *WorkflowService) UpdateWorkflow(
	parentCtx context.Context,
	req *pb.UpdateWorkflowRequest,
) (*pb.WorkflowResponse, error) {
	ctx, span := s.t.Start(parentCtx, "rpc.WorkflowService.UpdateWorkflow")
	defer span.End()
	input := service.UpdateWorkflowInput{
		Name:        req.Name,
		Description: req.Description,
	}
	if req.Steps != nil {
		input.Steps = toDomainSteps(req.GetSteps())
	}

	workflow, err := s.svc.Update(ctx, req.GetWorkflowId(), input)
	if err != nil {
		return nil, toPbError(err)
	}
	wf, err := toPbWorkflow(workflow)
	if err != nil {
		return nil, err
	}
	return wf, nil
}

// toDomain helpers

func toDomainSteps(pbSteps []*pb.StepDefinition) []domain.StepDefinition {
	domainSteps := make([]domain.StepDefinition, len(pbSteps))
	for i, pbStep := range pbSteps {
		domainSteps[i] = domain.StepDefinition{
			Name:           pbStep.GetName(),
			Image:          pbStep.GetImage(),
			Command:        pbStep.GetCommand(),
			Args:           pbStep.GetArgs(),
			Env:            pbStep.GetEnv(),
			TimeoutSeconds: toDomainTimeout(pbStep.TimeoutSeconds),
		}
	}
	return domainSteps
}

func toDomainTimeout(timeout *int32) int {
	if timeout == nil {
		return 0
	}
	return int(*timeout)
}

// toPb helpers

func toPbWorkflowStatus(st domain.WorkflowStatus) (pb.WorkflowStatus, error) {
	switch st {
	case domain.WorkflowActive:
		return pb.WorkflowStatus_WORKFLOW_STATUS_ACTIVE, nil
	case domain.WorkflowDraft:
		return pb.WorkflowStatus_WORKFLOW_STATUS_INACTIVE, nil
	case domain.WorkflowArchived:
		return pb.WorkflowStatus_WORKFLOW_STATUS_ARCHIVED, nil
	default:
		return pb.WorkflowStatus_WORKFLOW_STATUS_UNSPECIFIED,
			status.Error(codes.InvalidArgument, "unknown workflow status")
	}
}

func toPbWorkflow(wf *domain.Workflow) (*pb.WorkflowResponse, error) {
	pbStatus, err := toPbWorkflowStatus(wf.Status)
	if err != nil {
		return nil, err
	}
	pbSteps := make([]*pb.StepDefinition, len(wf.Steps))
	for i, step := range wf.Steps {
		pbSteps[i] = &pb.StepDefinition{
			Name:    step.Name,
			Image:   step.Image,
			Command: step.Command,
			Args:    step.Args,
			Env:     step.Env,
		}
		if step.TimeoutSeconds != 0 {
			if step.TimeoutSeconds < math.MinInt32 || step.TimeoutSeconds > math.MaxInt32 {
				return nil, status.Error(codes.InvalidArgument, "invalid timeout value")
			}
			pbSteps[i].TimeoutSeconds = new(int32)
			*pbSteps[i].TimeoutSeconds = int32(step.TimeoutSeconds)
		}
	}
	pbWorkflow := &pb.WorkflowResponse{
		Id:          wf.ID.String(),
		Name:        wf.Name,
		Description: wf.Description,
		Status:      pbStatus,
		CreatedAt:   timestamppb.New(wf.CreatedAt),
		UpdatedAt:   timestamppb.New(wf.UpdatedAt),
	}
	if len(pbSteps) > 0 {
		pbWorkflow.Steps = pbSteps
	}
	return pbWorkflow, nil
}
