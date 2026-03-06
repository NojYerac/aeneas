package http

import (
	"github.com/go-playground/validator/v10"
	"github.com/nojyerac/go-lib/tracing"
	libhttp "github.com/nojyerac/go-lib/transport/http"
	"go.opentelemetry.io/otel/trace"
)

// RegisterRoutes registers all HTTP routes with the server
func RegisterRoutes(srv libhttp.Server) {
	r := &Routes{
		v: validator.New(),
		t: tracing.TracerForPackage(),
	}

	// Repository-based handlers will be registered here
	_ = r
}

type Routes struct {
	v *validator.Validate
	t trace.Tracer
}
