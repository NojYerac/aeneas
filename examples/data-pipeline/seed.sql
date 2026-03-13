-- Seed data-pipeline workflow
INSERT INTO workflows (id, name, description, steps, status, created_at, updated_at)
VALUES (
    'workflow-data-pipeline',
    'data-pipeline',
    'Three-step ETL pipeline demonstrating sequential task orchestration',
    '[
        {
            "name": "extract",
            "image": "alpine:3.19",
            "command": ["/bin/sh", "/scripts/extract.sh"]
        },
        {
            "name": "transform",
            "image": "alpine:3.19",
            "command": ["/bin/sh", "/scripts/transform.sh"],
            "dependsOn": ["extract"]
        },
        {
            "name": "load",
            "image": "alpine:3.19",
            "command": ["/bin/sh", "/scripts/load.sh"],
            "dependsOn": ["transform"]
        }
    ]',
    'active',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
);

-- Create a sample execution
INSERT INTO executions (id, workflow_id, status, created_at)
VALUES (
    'exec-demo-001',
    'workflow-data-pipeline',
    'pending',
    CURRENT_TIMESTAMP
);

-- Create step executions for the demo
INSERT INTO step_executions (id, execution_id, step_name, status, created_at)
VALUES
    ('step-demo-001-extract', 'exec-demo-001', 'extract', 'pending', CURRENT_TIMESTAMP),
    ('step-demo-001-transform', 'exec-demo-001', 'transform', 'pending', CURRENT_TIMESTAMP),
    ('step-demo-001-load', 'exec-demo-001', 'load', 'pending', CURRENT_TIMESTAMP);
