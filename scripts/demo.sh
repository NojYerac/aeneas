#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuration
DB_CONTAINER="aeneas-postgres-1"
WORKFLOW_NAME="data-pipeline"

echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}Aeneas Workflow Orchestration Demo${NC}"
echo -e "${BLUE}=========================================${NC}"
echo ""

# Step 1: Start services
echo -e "${YELLOW}[1/6] Starting services...${NC}"
docker-compose up -d
echo -e "${GREEN}✓ Services started${NC}"
echo ""

# Wait for postgres to be ready
echo -e "${YELLOW}[2/6] Waiting for PostgreSQL...${NC}"
until docker-compose exec -T postgres pg_isready -U aeneas > /dev/null 2>&1; do
    sleep 1
done
echo -e "${GREEN}✓ PostgreSQL ready${NC}"
echo ""

# Step 2: Seed the workflow
echo -e "${YELLOW}[3/6] Loading workflow definition...${NC}"
docker-compose exec -T postgres psql -U aeneas -d aeneas -f /examples/seed.sql > /dev/null 2>&1 || {
    # If direct mount doesn't work, copy and run
    docker cp examples/data-pipeline/seed.sql "$DB_CONTAINER:/tmp/seed.sql"
    docker-compose exec -T postgres psql -U aeneas -d aeneas -f /tmp/seed.sql > /dev/null 2>&1
}
echo -e "${GREEN}✓ Workflow '$WORKFLOW_NAME' registered${NC}"
echo ""

# Step 3: Show workflow details
echo -e "${YELLOW}[4/6] Workflow details:${NC}"
docker-compose exec -T postgres psql -U aeneas -d aeneas -c "
SELECT 
    name,
    description,
    status,
    created_at
FROM workflows 
WHERE name = '$WORKFLOW_NAME';
" | head -n -2
echo ""

# Step 4: Show execution state
echo -e "${YELLOW}[5/6] Execution state (before):${NC}"
docker-compose exec -T postgres psql -U aeneas -d aeneas -c "
SELECT 
    e.id,
    e.workflow_id,
    e.status,
    e.created_at
FROM executions e
WHERE e.workflow_id = 'workflow-data-pipeline'
ORDER BY e.created_at DESC
LIMIT 1;
" | head -n -2
echo ""

echo -e "${YELLOW}[5/6] Step executions (before):${NC}"
docker-compose exec -T postgres psql -U aeneas -d aeneas -c "
SELECT 
    step_name,
    status,
    created_at
FROM step_executions
WHERE execution_id = 'exec-demo-001'
ORDER BY created_at;
" | head -n -2
echo ""

# Step 5: Simulate workflow execution
echo -e "${YELLOW}[6/6] Simulating workflow execution...${NC}"
echo ""

# Create a temporary directory for pipeline data
TEMP_DATA_DIR=$(mktemp -d)
echo "Using temp directory: $TEMP_DATA_DIR"
echo ""

# Extract step
echo -e "${BLUE}=== Running Step 1: Extract ===${NC}"
docker run --rm \
    -v "$(pwd)/examples/data-pipeline/steps/extract:/scripts:ro" \
    -v "$TEMP_DATA_DIR:/data" \
    alpine:3.19 sh /scripts/extract.sh
echo ""

# Transform step  
echo -e "${BLUE}=== Running Step 2: Transform ===${NC}"
docker run --rm \
    -v "$(pwd)/examples/data-pipeline/steps/transform:/scripts:ro" \
    -v "$TEMP_DATA_DIR:/data" \
    alpine:3.19 sh /scripts/transform.sh
echo ""

# Load step
echo -e "${BLUE}=== Running Step 3: Load ===${NC}"
docker run --rm \
    -v "$(pwd)/examples/data-pipeline/steps/load:/scripts:ro" \
    -v "$TEMP_DATA_DIR:/data" \
    alpine:3.19 sh /scripts/load.sh
echo ""

# Update execution status in database
docker-compose exec -T postgres psql -U aeneas -d aeneas > /dev/null 2>&1 << EOF
UPDATE executions 
SET status = 'succeeded', 
    started_at = NOW() - INTERVAL '10 seconds',
    finished_at = NOW()
WHERE id = 'exec-demo-001';

UPDATE step_executions
SET status = 'succeeded',
    started_at = NOW() - INTERVAL '10 seconds',
    finished_at = NOW() - INTERVAL '7 seconds',
    exit_code = 0
WHERE execution_id = 'exec-demo-001' AND step_name = 'extract';

UPDATE step_executions
SET status = 'succeeded',
    started_at = NOW() - INTERVAL '7 seconds',
    finished_at = NOW() - INTERVAL '4 seconds',
    exit_code = 0
WHERE execution_id = 'exec-demo-001' AND step_name = 'transform';

UPDATE step_executions
SET status = 'succeeded',
    started_at = NOW() - INTERVAL '4 seconds',
    finished_at = NOW(),
    exit_code = 0
WHERE execution_id = 'exec-demo-001' AND step_name = 'load';
EOF

echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}✓ Pipeline execution complete!${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""

# Show final state
echo -e "${YELLOW}Execution state (after):${NC}"
docker-compose exec -T postgres psql -U aeneas -d aeneas -c "
SELECT 
    e.id,
    e.status,
    e.started_at,
    e.finished_at,
    EXTRACT(EPOCH FROM (e.finished_at - e.started_at)) || 's' AS duration
FROM executions e
WHERE e.id = 'exec-demo-001';
" | head -n -2
echo ""

echo -e "${YELLOW}Step executions (after):${NC}"
docker-compose exec -T postgres psql -U aeneas -d aeneas -c "
SELECT 
    step_name,
    status,
    exit_code,
    EXTRACT(EPOCH FROM (finished_at - started_at)) || 's' AS duration
FROM step_executions
WHERE execution_id = 'exec-demo-001'
ORDER BY created_at;
" | head -n -2
echo ""

# Cleanup
rm -rf "$TEMP_DATA_DIR"

echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}Demo Notes:${NC}"
echo -e "${BLUE}=========================================${NC}"
echo "• Workflow orchestration engine running at http://localhost:8080"
echo "• PostgreSQL database stores workflow state at localhost:5432"
echo "• Pipeline executed 3 steps: extract → transform → load"
echo "• Each step ran in an isolated container with shared volume"
echo ""
echo "To stop services:"
echo "  docker-compose down"
echo ""
echo "To view logs:"
echo "  docker-compose logs -f aeneas"
echo ""
