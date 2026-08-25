#!/usr/bin/env bash
# Resume after the firebolt concurrency OOM killed the engine: the firebolt
# untuned phases are complete (concurrency recorded as a fatal OOM finding);
# run the remaining ClickHouse baseline, then hand off to the tuned watcher.
set -euo pipefail
cd "$(dirname "$0")"
OUT="${OUT:-../results-firebolt-local}"

run() { # sig fill-levels
  echo "=== clickhouse $1 deep ==="
  ./firebolt-bench --dialect clickhouse --target http://localhost:8123 \
    --signal "$1" --skip-ramp --cache-bust --probe-under-write \
    --probe-runs 5 --fill-levels "$2" \
    --report-out "$OUT/local-clickhouse-$1-deep.json"
}

run logs    30000000,60000000,100000000
run spans   30000000,100000000
run metrics 30000000,100000000
echo "=== clickhouse spans concurrency ==="
./firebolt-bench --dialect clickhouse --target http://localhost:8123 \
  --signal spans --reset=false --workers 16 --batch-sizes 16384 --step-seconds 15 \
  --fill-levels 1 --cache-bust --probe-runs 5 \
  --report-out "$OUT/local-clickhouse-spans-concurrency.json"
echo "ALL DONE"
