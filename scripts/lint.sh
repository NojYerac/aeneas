#!/usr/bin/env bash
# scripts/lint.sh — run golangci-lint with project settings.
set -euo pipefail

if ! command -v golangci-lint &>/dev/null; then
  echo "[lint] golangci-lint not found — installing latest…"
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi

golangci-lint run
