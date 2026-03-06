package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/nojyerac/go-lib/tracing"
	libhttp "github.com/nojyerac/go-lib/transport/http"
	"github.com/nojyerac/aeneas/data"
	"go.opentelemetry.io/otel/trace"
)

func RegisterRoutes(src data.DataSource, srv libhttp.Server) {
	r := &Routes{
		v:   validator.New(),
		t:   tracing.TracerForPackage(),
		src: src,
	}

	srv.HandleFunc("GET /example/{id}", r.GetExampleHandler)
}

type Routes struct {
	v   *validator.Validate
	t   trace.Tracer
	src data.DataSource
}

func (r *Routes) GetExampleHandler(w http.ResponseWriter, req *http.Request) {
	ctx, span := r.t.Start(req.Context(), "GetExampleHandler")
	defer span.End()
	ex, err := r.src.GetExample(ctx, req.PathValue("id"))
	if err != nil {
		http.Error(w, "failed to get example", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ex)
}
