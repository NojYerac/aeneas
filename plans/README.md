# Kubernetes-Native Job Orchestrator

A lightweight orchestration service that accepts workflow definitions and executes them as Kubernetes Jobs.

## Problem

Platform teams often need a simple internal service to run controlled, observable background workflows without introducing a full workflow engine.

## MVP Scope

- Accept workflow definitions via REST API (JSON)
- Validate and persist workflow/job state in a lightweight DB
- Execute steps as Kubernetes Jobs
- Track lifecycle states: `pending`, `running`, `succeeded`, `failed`
- Expose status/query APIs for workflows and executions
- Publish operational metrics and structured logs

## Architecture (MVP)

```text
Client -> REST API -> Orchestrator Core -> K8s Job Runner
                      |                \
                      |                 -> State Store (SQLite/Postgres)
                      -> Metrics/Logs
```

## Key Components

- API Layer: create workflow, trigger run, fetch status
- Scheduler/Dispatcher: translate workflow steps into executable jobs
- Runner Adapter: Kubernetes Job creation and watch loop
- State Store: workflow specs + execution records
- Observability: Prometheus metrics + structured logs

## MVP Non-Goals

- DAG with complex branching
- Cron scheduling
- Multi-cluster execution
- Multi-tenant RBAC model

## Local Run (target)

- `make run`
- `make test`
- `docker compose up` for dependencies

## Kubernetes Deployment (target)

- Deploy API service as a Deployment
- Configure service account with Job CRUD permissions
- Deploy via Helm chart or Kustomize overlay

## Roadmap

See [plans/01-MVP-PLAN.md](plans/01-MVP-PLAN.md).
