#!/bin/sh
set -e

USERNAME="$1"
PASSWORD="$2"
OUTPUT_DIR="${3:-/output}"

if [ -z "$USERNAME" ] || [ -z "$PASSWORD" ]; then
  echo "Usage: $0 <username> <password> [output_dir]"
  exit 1
fi

CSV_PATH="$OUTPUT_DIR/scores.csv"

echo "[INFO] Starting scraping..."
./scraper "$USERNAME" "$PASSWORD" "$CSV_PATH"

echo "[INFO] Starting analysis..."
python3 analyze.py "$CSV_PATH"

echo "[INFO] Done! Output files:"
ls -la "$OUTPUT_DIR"/*.csv "$OUTPUT_DIR"/*.md "$OUTPUT_DIR"/*.json 2>/dev/null || true
