# MVP Plan: Kubernetes-Native Job Orchestrator

## Goal

Deliver a minimal but production-minded orchestrator that executes simple workflows as Kubernetes Jobs and exposes execution status through a REST API.

## Step 1: Repository and Runtime Foundation

- Initialize Go module, project layout, and Makefile targets
- Add config loading for server, database, and Kubernetes settings
- Wire structured logging and graceful shutdown
- Define coding/testing conventions and CI baseline

**Exit criteria:** Service starts with config, health endpoint responds, CI runs basic tests.

## Step 2: Domain Model and Persistence

- Define core entities: `Workflow`, `Execution`, `StepExecution`
- Define state machine and status transitions
- Implement persistence repository interfaces
- Add initial SQL schema and migration scripts
- Add repository unit tests

**Exit criteria:** Workflows and executions can be created/read/updated with tested state transitions.

## Step 3: REST API for Workflow and Execution Control

- Implement endpoints:
  - `POST /workflows`
  - `GET /workflows/{id}`
  - `POST /workflows/{id}/executions`
  - `GET /executions/{id}`
- Add request validation and consistent error model
- Add integration tests for API + persistence

**Exit criteria:** Users can register workflows and trigger executions through API.

## Step 4: Kubernetes Job Runner Adapter

- Implement runner interface and Kubernetes-backed adapter
- Map workflow step definitions to Kubernetes Job manifests
- Add watch/reconcile loop for Job completion status
- Propagate failure and retry metadata to execution state

**Exit criteria:** Triggered execution creates Kubernetes Jobs and updates statuses based on Job outcomes.

## Step 5: Dispatcher and Execution Engine

- Implement execution loop to process steps sequentially (MVP)
- Ensure idempotent dispatch behavior and duplicate-run protection
- Add timeout handling and cancellation hooks
- Persist step-level progress

**Exit criteria:** End-to-end execution works from API request to final workflow status.

## Step 6: Observability and Operational Hardening

- Add Prometheus metrics:
  - execution counts by status
  - execution duration histogram
  - active executions gauge
- Add structured logs with correlation IDs
- Add health/readiness checks and dependency probes

**Exit criteria:** Metrics and logs provide enough signal to diagnose failures and performance issues.

## Step 7: Kubernetes Packaging and Developer Experience

- Add Dockerfile and image build target
- Add minimal Helm chart or Kustomize manifests
- Include RBAC manifests for Job operations
- Document local and cluster runbooks

**Exit criteria:** Service can be deployed to a dev cluster with least-privilege RBAC and verified end-to-end.

## Step 8: MVP Validation and Release

- Run e2e scenario test on local cluster (e.g., kind)
- Validate failure path (invalid step image, timeout)
- Freeze MVP scope and tag `v0.1.0`
- Publish architecture/tradeoffs section in README

**Exit criteria:** Tagged MVP release with documented limitations and reproducible deployment steps.
