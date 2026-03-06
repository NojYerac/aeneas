#!/usr/bin/env bash
# scripts/generate.sh — run code generation tools.
set -euo pipefail

mockery
echo "[generate] mockery done — generating protobuf code…"
project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
protoc -I ${project_root}/api  ${project_root}/api/*.proto --go_out=${project_root} --go-grpc_out=${project_root}
echo "[generate] protobuf code generated successfully."
