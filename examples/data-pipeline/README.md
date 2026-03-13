# Data Pipeline Example

This example demonstrates a complete ETL (Extract-Transform-Load) workflow orchestrated by Aeneas.

## Overview

The pipeline consists of three sequential steps:

1. **Extract**: Fetches raw sensor data and writes to `/data/raw.csv`
2. **Transform**: Filters and enriches the data, writes to `/data/transformed.csv`
3. **Load**: Simulates loading the transformed data into a target database

Each step runs in an isolated Alpine Linux container with a shared volume for data passing.

## Workflow Definition

The workflow is defined in `workflow.yaml` using Kubernetes-style syntax:

```yaml
apiVersion: aeneas.io/v1
kind: Workflow
metadata:
  name: data-pipeline
spec:
  steps:
    - name: extract
      image: alpine:3.19
      command: ["/bin/sh", "/scripts/extract.sh"]
      
    - name: transform
      image: alpine:3.19
      command: ["/bin/sh", "/scripts/transform.sh"]
      dependsOn: [extract]
      
    - name: load
      image: alpine:3.19
      command: ["/bin/sh", "/scripts/load.sh"]
      dependsOn: [transform]
```

## Files

```
examples/data-pipeline/
├── README.md              # This file
├── workflow.yaml          # Workflow definition
├── seed.sql               # Database seed script
└── steps/                 # Step implementations
    ├── extract/
    │   └── extract.sh     # Data extraction script
    ├── transform/
    │   └── transform.sh   # Data transformation script
    └── load/
        └── load.sh        # Data loading script
```

## Running the Example

From the repository root:

```bash
./scripts/demo.sh
```

This will:
1. Start PostgreSQL and Aeneas services
2. Register the data-pipeline workflow
3. Create an execution record
4. Run each step sequentially with live output
5. Update execution state in the database
6. Display final execution metrics

## Expected Output

```
=========================================
Aeneas Workflow Orchestration Demo
=========================================

[1/6] Starting services...
✓ Services started

[2/6] Waiting for PostgreSQL...
✓ PostgreSQL ready

[3/6] Loading workflow definition...
✓ Workflow 'data-pipeline' registered

[4/6] Workflow details:
...

[6/6] Simulating workflow execution...

=== Running Step 1: Extract ===
=========================================
STEP 1: EXTRACT
=========================================
Fetching raw data from source...
✓ Extracted 5 records to /data/raw.csv
...

=== Running Step 2: Transform ===
=========================================
STEP 2: TRANSFORM
=========================================
Processing raw data...
✓ Transformed 3 records (filtered low values)
...

=== Running Step 3: Load ===
=========================================
STEP 3: LOAD
=========================================
Loading transformed data to destination...
✓ Inserted 3 records into target database
...

=========================================
✓ Pipeline execution complete!
=========================================
```

## What This Demonstrates

- **State Management**: Workflow and execution records stored in PostgreSQL
- **Sequential Orchestration**: Steps run in dependency order (extract → transform → load)
- **Container Isolation**: Each step runs in a clean Alpine container
- **Data Passing**: Shared volume enables inter-step communication
- **Lifecycle Tracking**: Status transitions from `pending` → `running` → `succeeded`

## Extending This Example

To create your own workflow:

1. Copy `examples/data-pipeline/` to `examples/your-workflow/`
2. Update `workflow.yaml` with your step definitions
3. Write step scripts in `steps/*/`
4. Create a seed script for database initialization
5. Run with `./scripts/demo.sh`

For production use, workflows would be registered via the HTTP API rather than direct SQL insertion.
