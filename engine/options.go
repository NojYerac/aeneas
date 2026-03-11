package engine

import (
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

// Option is a functional option for configuring the Engine
type Option func(*Engine)

// WithPollInterval sets the interval for polling pending executions
func WithPollInterval(interval time.Duration) Option {
	return func(e *Engine) {
		e.pollInterval = interval
	}
}

// WithLogger sets the logger for the engine
func WithLogger(logger *logrus.Logger) Option {
	return func(e *Engine) {
		e.logger = logger
	}
}

// WithTracer sets the OpenTelemetry tracer for the engine
func WithTracer(tracer trace.Tracer) Option {
	return func(e *Engine) {
		e.tracer = tracer
	}
}

// WithRunner sets the runner implementation
func WithRunner(runner Runner) Option {
	return func(e *Engine) {
		e.runner = runner
	}
}
