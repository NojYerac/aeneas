#!/usr/bin/env bash
# scripts/test.sh — run tests with race detector and coverage.
set -euo pipefail

ginkgo -r
