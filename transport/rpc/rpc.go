package rpc

import (
	"context"

	"github.com/nojyerac/go-lib/tracing"
	"github.com/nojyerac/aeneas/data"
	pb "github.com/nojyerac/aeneas/pb/example"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

func RegisterServices(src data.DataSource, o ...Option) func(s *grpc.Server) {
	exSrv := NewExampleService(src, o...)
	return func(s *grpc.Server) {
		pb.RegisterExampleServiceServer(s, exSrv)
	}
}

type ExampleService struct {
	pb.UnimplementedExampleServiceServer
	src data.DataSource
	t   trace.Tracer
	l   logrus.FieldLogger
}

func NewExampleService(src data.DataSource, o ...Option) *ExampleService {
	opts := &options{
		t: tracing.TracerForPackage(),
		l: logrus.New(),
	}
	for _, applyOpt := range o {
		applyOpt(opts)
	}
	return &ExampleService{
		src: src,
		t:   opts.t,
		l:   opts.l,
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

func (s *ExampleService) GetExample(ctx context.Context, req *pb.GetExampleRequest) (*pb.GetExampleResponse, error) {
	ctx, span := s.t.Start(ctx, "ExampleService.GetExample")
	defer span.End()
	s.l.WithField("id", req.Id).Info("handling GetExample request")
	ex, err := s.src.GetExample(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.GetExampleResponse{
		Example: &pb.Example{
			Id:          ex.ID,
			Name:        ex.Name,
			Description: ex.Description,
		},
	}, nil
}
