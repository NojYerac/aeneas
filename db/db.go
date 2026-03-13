package db

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/nojyerac/aeneas/config"
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

func defaultOptions() *options {
	return &options{
		t: tracing.TracerForPackage(),
		l: log.Nop(),
	}
}

func getPlaceholderFormat() sq.PlaceholderFormat {
	if config.DBConfig == nil {
		return sq.Dollar
	}
	switch config.DBConfig.Driver {
	case "postgres":
		return sq.Dollar
	case "sqlite":
		return sq.Question
	default:
		return sq.Dollar
	}
}
