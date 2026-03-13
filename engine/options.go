package engine

import (
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Options configures the execution engine
type Options struct {
	PollInterval time.Duration
	Logger       logrus.FieldLogger
	Tracer       trace.Tracer
}

// Option is a functional option for configuring the Engine
type Option func(*Options)

// WithPollInterval sets the interval for polling pending executions
func WithPollInterval(interval time.Duration) Option {
	return func(o *Options) {
		o.PollInterval = interval
	}
}

// WithLogger sets the logger for the engine
func WithLogger(logger logrus.FieldLogger) Option {
	return func(o *Options) {
		o.Logger = logger
	}
}

// WithTracer sets the OpenTelemetry tracer for the engine
func WithTracer(tracer trace.Tracer) Option {
	return func(o *Options) {
		o.Tracer = tracer
	}
}

// defaultOptions returns the default engine options
func defaultOptions() Options {
	return Options{
		PollInterval: 5 * time.Second,
		Logger:       logrus.New(),
		Tracer:       noop.NewTracerProvider().Tracer("aeneas.engine"),
	}
}
