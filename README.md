# Aeneas

> Scaffolded with [go-lib](github.com/nojyerac/go-lib) — scaffold v1.25.1.

## Overview

`aeneas` is a Go microservice built on go-lib conventions.  It provides:

- **HTTP** and optional **gRPC** serving on a single multiplexed port (`cmux`)
- Structured JSON logging via `logrus`
- Distributed tracing via OpenTelemetry
- Prometheus metrics (exported at `/metrics`)
- Liveness (`/livez`) and readiness (`/healthz`) probes
- Build-time version metadata (`/version`)

## Quick start

```bash
# Install dependencies
go mod tidy

# Run locally (no TLS, port 8080)
make run

# Run tests
make test

# Lint
make lint
```

## Configuration

All configuration is driven by environment variables prefixed with `AENEAS_`.
The table below lists the most important knobs; refer to the go-lib sub-package
READMEs for full documentation.

| Variable | Default | Description |
|----------|---------|-------------|
| `AENEAS_PORT` | `80` | Listening port |
| `AENEAS_NO_TLS` | `false` | Disable TLS (useful for local dev) |
| `AENEAS_LOG_LEVEL` | `info` | Log level (`trace`…`panic`) |
| `AENEAS_SERVICE_NAME` | `aeneas` | Value embedded in log fields |
| `AENEAS_EXPORTER_TYPE` | `noop` | Trace exporter (`stdout`, `otlp`, `noop`) |
| `AENEAS_HEALTHCHECK_CHECK_INTERVAL` | `30s` | Health-check tick interval |

## Building a Docker image

```bash
make docker          # builds aeneas:dev
docker run --rm -p 8080:8080 aeneas:dev
```

## Project layout

```text
.github/workflows/ — CI pipeline
api/               — protobuf & OpenAPI definitions
cmd/aeneas/     — main package / entry-point
config/            — root Config struct + defaults
data/              — business logic + data access (repositories, etc.)
mocks/             — generated test mocks (via mockery)
pb/                — generated protobuf code (do not edit)
transport/http     — HTTP route registration
transport/rpc      — gRPC service definitions + registration
scripts/           — lint + test helpers
Dockerfile
Makefile
```

## Adding routes

Edit `transport/server.go` and call `s.HandleFunc` or `s.Handle` with your
handler.  The API prefix `/api` is applied automatically by go-lib.

## Adding gRPC services
Add your protobuf definitions to the `api/` directory and generate code with `protoc`.
Then, edit `transport/rpc/server.go` and call `s.RegisterService` with your service
implementation.
