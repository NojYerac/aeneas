package db

import (
	"context"

	"github.com/nojyerac/go-lib/db"
	"github.com/nojyerac/go-lib/log"
	"github.com/nojyerac/go-lib/tracing"
	"github.com/nojyerac/aeneas/data"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

var _ data.DataSource = (*source)(nil)

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

func NewDataSource(database db.Database, o ...Option) data.DataSource {
	opts := &options{
		t: tracing.TracerForPackage(),
		l: log.Nop(),
	}

	for _, opt := range o {
		opt(opts)
	}

	return &source{
		t: opts.t,
		l: opts.l,
		d: database,
	}
}

type source struct {
	t trace.Tracer
	l logrus.FieldLogger
	d db.Database
}

func (s *source) GetExample(ctx context.Context, id string) (*data.Example, error) {
	ctx, span := s.t.Start(ctx, "source.GetExample")
	defer span.End()
	ex := new(data.Example)
	err := s.d.Get(ctx, ex, "SELECT id, name, description FROM example WHERE id = $1", id)
	return ex, err
}
