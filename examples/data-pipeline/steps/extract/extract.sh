#!/bin/sh
set -e

echo "========================================="
echo "STEP 1: EXTRACT"
echo "========================================="
echo "Fetching raw data from source..."

# Simulate data extraction with a delay
sleep 2

# Generate sample data
cat > /data/raw.csv << EOF
id,name,value,timestamp
1,sensor_a,42.5,2026-03-11T12:00:00Z
2,sensor_b,38.2,2026-03-11T12:00:01Z
3,sensor_c,45.8,2026-03-11T12:00:02Z
4,sensor_d,41.1,2026-03-11T12:00:03Z
5,sensor_e,39.7,2026-03-11T12:00:04Z
EOF

echo "✓ Extracted 5 records to /data/raw.csv"
echo ""
cat /data/raw.csv
echo ""
echo "Extract phase complete."
