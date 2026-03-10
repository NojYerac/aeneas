# Aeneas vs. Other Workflow Orchestrators

This document provides an honest comparison of Aeneas against established workflow orchestration tools. The goal is not to claim Aeneas is "better" — it's to clarify when Aeneas is the right choice and when it's not.

---

## Feature Matrix

| Feature | Aeneas | Temporal | Argo Workflows | Conductor |
|---------|--------|----------|----------------|-----------|
| **Setup Complexity** | Low (single binary + DB) | High (cluster + workers) | Medium (K8s + CRDs) | Medium (cluster + backend) |
| **Kubernetes-Native** | ✅ Core design | ❌ K8s is optional | ✅ CRD-based | ❌ Framework-agnostic |
| **Language Support** | Container-based (any) | Go, Java, TypeScript, Python, PHP, .NET | Container-based (any) | Java, Python, Go, C# |
| **Durable Execution** | ❌ No replay semantics | ✅ Event sourcing + replay | ❌ Job-based execution | ✅ Task retries + state recovery |
| **Scalability** | Small-medium (100s of workflows) | Very high (10,000s+ workflows) | High (enterprise-scale) | High (Netflix production workloads) |
| **Learning Curve** | Low (REST API + containers) | Steep (workflow DSL + concepts) | Medium (YAML + K8s knowledge) | Medium (workflow JSON + SDK) |
| **State Visibility** | PostgreSQL + HTTP API | Temporal UI + tctl CLI | Argo UI + kubectl | Conductor UI + REST API |
| **Operational Overhead** | Low (single service) | High (cluster management) | Medium (K8s dependencies) | Medium (backend + queue management) |
| **Built-in Observability** | ⚠️ Planned (Prometheus) | ✅ Metrics + tracing | ✅ Prometheus + logs | ✅ Metrics + task analytics |
| **GitOps Integration** | ❌ Not a GitOps tool | ❌ Not a GitOps tool | ✅ Native support | ❌ Not a GitOps tool |
| **Long-Running Workflows** | ❌ Not designed for this | ✅ Years-long workflows | ⚠️ Limited (job timeouts) | ✅ Indefinite duration |
| **First Release** | 2024 (in development) | 2019 | 2018 | 2016 |
| **Production Maturity** | ⚠️ Not yet production-ready | ✅ Battle-tested (Uber, Netflix) | ✅ CNCF graduated | ✅ Battle-tested (Netflix) |

---

## Architectural Philosophy

### Aeneas
- **Philosophy**: Minimal abstraction over Kubernetes Jobs
- **Approach**: Direct mapping of workflow steps to container execution
- **State Management**: PostgreSQL for execution history
- **Execution Model**: Fire-and-forget Jobs with lifecycle tracking

**Best For:** Teams that already understand Kubernetes and want orchestration without a new DSL.

---

### Temporal
- **Philosophy**: Durable execution as a programming model
- **Approach**: Workflows as code with automatic checkpointing and replay
- **State Management**: Event sourcing with immutable history
- **Execution Model**: Long-running processes with guaranteed completion (even across failures)

**Best For:** Complex, multi-month workflows where partial execution recovery is critical (e.g., financial transactions, compliance workflows).

---

### Argo Workflows
- **Philosophy**: GitOps-native workflow orchestration
- **Approach**: Workflows as Kubernetes CRDs (Custom Resource Definitions)
- **State Management**: etcd (Kubernetes control plane)
- **Execution Model**: DAG-based execution with artifact passing

**Best For:** CI/CD pipelines, data processing, and teams committed to GitOps workflows.

---

### Conductor
- **Philosophy**: Microservices orchestration with explicit task definitions
- **Approach**: Centralized workflow definitions with worker-based execution
- **State Management**: Pluggable backends (Redis, Cassandra, Postgres)
- **Execution Model**: Task workers poll for work items

**Best For:** Distributed systems with heterogeneous services (polyglot microservices).

---

## When to Use Aeneas

✅ **Choose Aeneas if:**
- You're already running workloads on Kubernetes
- Your workflows are **simple, linear sequences** (build → test → deploy)
- You want orchestration without learning a new DSL or SDK
- You need a **lightweight solution** (single service + database)
- Your team is comfortable with REST APIs and container management
- You're orchestrating **hundreds** of workflows, not tens of thousands
- You value **code clarity** and want to understand the orchestrator's internals

**Example Use Cases:**
- CI/CD pipelines for small-to-medium teams
- Scheduled data processing jobs (ETL for analytics)
- Multi-step deployment workflows (build → test → push → rollout)
- Periodic report generation (fetch data → aggregate → publish)

---

## When NOT to Use Aeneas

❌ **Do NOT choose Aeneas if:**
- You need **durable execution** with replay semantics (use Temporal instead)
- Your workflows run for **days or weeks** (Aeneas is designed for short-lived Jobs)
- You require **horizontal scaling** beyond 1,000 concurrent executions (use Argo or Temporal)
- You need **built-in retries** with exponential backoff (Conductor or Temporal handle this better)
- You want **visual workflow builders** (Conductor has a UI for non-engineers)
- You're orchestrating **distributed transactions** (Temporal's sagas and compensation are better suited)
- Your workflows require **human-in-the-loop approvals** (Temporal or Conductor support this natively)
- You need **production-proven stability** right now (Aeneas is in active development)

**Anti-Patterns:**
- ❌ Financial transaction processing (use Temporal for guaranteed completion)
- ❌ Complex ML training pipelines with dynamic DAGs (use Argo or Kubeflow)
- ❌ Multi-tenant SaaS with thousands of concurrent workflows (use Temporal or Conductor)
- ❌ Workflows that require state snapshots and rollback (use Temporal)

---

## Technical Tradeoffs

### Aeneas: Simplicity Over Features
**Gains:**
- Easy to understand and debug (clean Go codebase)
- Low operational overhead (single service deployment)
- No vendor lock-in (standard PostgreSQL + Kubernetes)

**Loses:**
- No built-in retry logic (you implement it in your containers)
- No workflow versioning (breaking changes require careful migration)
- No long-running workflow support (Jobs have finite lifetimes)

---

### Temporal: Durability Over Simplicity
**Gains:**
- Workflows that can run for months or years
- Automatic retries and recovery from failures
- Time-travel debugging with workflow history

**Loses:**
- High operational complexity (requires a Temporal cluster)
- Steep learning curve (workflows-as-code paradigm is non-intuitive)
- Infrastructure cost (needs separate worker pools)

---

### Argo Workflows: GitOps Over Flexibility
**Gains:**
- Workflows versioned in Git alongside code
- Native Kubernetes integration (CRDs + kubectl)
- Rich DAG support with conditional steps

**Loses:**
- Tightly coupled to Kubernetes (not portable)
- Requires understanding of CRD lifecycle management
- Complex YAML for advanced workflows

---

### Conductor: Polyglot Over Simplicity
**Gains:**
- Works across any runtime (not tied to Kubernetes)
- Strong support for dynamic task definitions
- Visual workflow designer for non-technical users

**Loses:**
- Requires dedicated infrastructure (queue + backend)
- Task workers must poll for work (potential latency)
- Less intuitive for teams already invested in Kubernetes

---

## Decision Framework

Ask yourself these questions:

1. **Do you need guaranteed execution even if your infrastructure fails?**
   - Yes → Temporal
   - No → Consider simpler options (Aeneas, Argo)

2. **Are your workflows part of a GitOps deployment strategy?**
   - Yes → Argo Workflows
   - No → Consider API-driven options (Aeneas, Conductor)

3. **Do you run on Kubernetes already?**
   - Yes → Aeneas or Argo
   - No → Temporal or Conductor (more portable)

4. **Do you need to orchestrate microservices across multiple languages/runtimes?**
   - Yes → Conductor (built for polyglot environments)
   - No → Aeneas (if K8s-native) or Temporal (if durable execution matters)

5. **Is your team willing to invest time learning a new paradigm?**
   - Yes → Temporal (most powerful, steepest learning curve)
   - No → Aeneas (minimal new concepts)

---

## Summary

**Aeneas is a teaching tool and a starting point**, not a replacement for production-grade orchestrators. It demonstrates how to build a clean, maintainable workflow engine using hexagonal architecture and domain-driven design.

If you're starting a new project and your workflows are simple, Aeneas might be enough. If you're orchestrating critical business processes at scale, use Temporal or Conductor. If you're already deep in the Kubernetes ecosystem and want GitOps-native workflows, use Argo.

**The best orchestrator is the one that matches your team's expertise and your system's requirements** — not the one with the most features.

---

## Further Reading

- [Temporal Documentation](https://docs.temporal.io/)
- [Argo Workflows Documentation](https://argoproj.github.io/workflows/)
- [Conductor Documentation](https://conductor.netflix.com/)
- [Aeneas MVP Plan](../plans/01-MVP-PLAN.md)
- [Aeneas Architecture Design](../README.md#architecture)
