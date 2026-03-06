package db

import (
	"github.com/nojyerac/go-lib/db"
	"github.com/nojyerac/go-lib/log"
	"github.com/nojyerac/go-lib/tracing"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

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

//nolint:unused // Reserved for upcoming repository constructor
func defaultOptions() *options {
	return &options{
		t: tracing.TracerForPackage(),
		l: log.Nop(),
	}
}

// Repositories will be implemented here using the domain.Repository interfaces
//
//nolint:unused // Reserved for upcoming repository implementations
type repositories struct {
	t trace.Tracer
	l logrus.FieldLogger
	d db.Database
}
