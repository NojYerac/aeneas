#!/bin/sh
set -e

echo "========================================="
echo "STEP 2: TRANSFORM"
echo "========================================="
echo "Processing raw data..."

# Check that input exists
if [ ! -f /data/raw.csv ]; then
  echo "ERROR: Input file /data/raw.csv not found!"
  exit 1
fi

# Simulate transformation with a delay
sleep 2

# Transform: filter out low values (<40), add derived column
{
  echo "id,name,value,timestamp,status"
  tail -n +2 /data/raw.csv | while IFS=, read -r id name value timestamp; do
    # Skip values below 40
    val_int=$(echo "$value" | cut -d. -f1)
    if [ "$val_int" -ge 40 ]; then
      if [ "$val_int" -ge 45 ]; then
        status="HIGH"
      else
        status="NORMAL"
      fi
      echo "$id,$name,$value,$timestamp,$status"
    fi
  done
} > /data/transformed.csv

record_count=$(tail -n +2 /data/transformed.csv | wc -l)
echo "✓ Transformed $record_count records (filtered low values)"
echo ""
cat /data/transformed.csv
echo ""
echo "Transform phase complete."
