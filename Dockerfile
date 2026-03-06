ARG GO_VERSION=1.25.1
# ── build stage ───────────────────────────────────────────────────────────────
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

# Cache module downloads separately from source compilation.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w \
      -X github.com/nojyerac/go-lib/version.semVer=$(cat VERSION 2>/dev/null || echo 0.0.0) \
      -X github.com/nojyerac/go-lib/version.gitSHA=$(git rev-list -1 HEAD 2>/dev/null || echo unknown)" \
    -o /out/aeneas \
    ./cmd/aeneas

# ── runtime stage ─────────────────────────────────────────────────────────────
FROM scratch

COPY --from=builder /out/aeneas /aeneas

# Default: no-TLS, port 8080.  Override at runtime via env vars.
ENV AENEAS_NO_TLS=true \
    AENEAS_PORT=8080 \
    AENEAS_LOG_LEVEL=info \
    AENEAS_EXPORTER_TYPE=noop \
    AENEAS_HEALTHCHECK_CHECK_INTERVAL=30s

EXPOSE 8080

ENTRYPOINT ["/aeneas"]
