package rpc

import (
	pb "github.com/nojyerac/aeneas/pb/workflow"
	"github.com/nojyerac/aeneas/service"
	"github.com/nojyerac/go-lib/log"
	"github.com/nojyerac/go-lib/tracing"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// RegisterServices registers all gRPC services with the server
func RegisterServices(
	wfSvc *service.WorkflowService,
	execSrv *service.ExecutionService,
	o ...Option,
) func(s *grpc.Server) {
	return func(s *grpc.Server) {
		// Repository-based services will be registered here
		wf := NewWorkflowService(wfSvc, o...)
		ex := NewExecutionService(execSrv, o...)
		pb.RegisterWorkflowServiceServer(s, wf)
		pb.RegisterExecutionServiceServer(s, ex)
	}
}

type options struct {
	t trace.Tracer
	l logrus.FieldLogger
}

type Option func(*options)

func WithTracer(t trace.Tracer) Option {
	return func(opts *options) {
		opts.t = t
	}
}

func WithLogger(l logrus.FieldLogger) Option {
	return func(opts *options) {
		opts.l = l
	}
}

//nolint:unused // Reserved for upcoming service registration
func defaultOptions() *options {
	return &options{
		t: tracing.TracerForPackage(),
		l: log.Nop(),
	}
}
