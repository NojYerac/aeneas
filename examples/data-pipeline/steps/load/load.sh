#!/bin/sh
set -e

echo "========================================="
echo "STEP 3: LOAD"
echo "========================================="
echo "Loading transformed data to destination..."

# Check that input exists
if [ ! -f /data/transformed.csv ]; then
  echo "ERROR: Input file /data/transformed.csv not found!"
  exit 1
fi

# Simulate loading with a delay
sleep 2

# Simulate database insertion
record_count=$(tail -n +2 /data/transformed.csv | wc -l)
echo "✓ Inserted $record_count records into target database"
echo ""

# Create manifest of what was loaded
cat > /data/manifest.txt << EOF
Pipeline Execution Summary
==========================
Timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
Records Loaded: $record_count
Status: SUCCESS

Data Sample:
EOF

head -n 3 /data/transformed.csv >> /data/manifest.txt

echo "Pipeline manifest:"
cat /data/manifest.txt
echo ""
echo "Load phase complete. Pipeline finished successfully! ✓"
