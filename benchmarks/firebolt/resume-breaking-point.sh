#!/usr/bin/env bash
# Resume of run-breaking-point.sh after the DROP-with-index failure:
# firebolt logs deep already completed, continue from firebolt spans deep.
set -euo pipefail
cd "$(dirname "$0")"
OUT="${OUT:-../results-firebolt-local}"

run() { # engine sig extra-args...
  local engine="$1" sig="$2"; shift 2
  local args=()
  if [ "$engine" = "clickhouse" ]; then
    args=(--dialect clickhouse --target http://localhost:8123)
  fi
  echo "=== $engine $sig deep ==="
  ./firebolt-bench ${args[@]+"${args[@]}"} --signal "$sig" --skip-ramp --cache-bust --probe-under-write \
    --probe-runs 5 "$@" \
    --report-out "$OUT/local-$engine-$sig-deep.json"
}

run firebolt spans   --fill-levels 30000000,100000000
run firebolt metrics --fill-levels 30000000,100000000
echo "=== firebolt spans concurrency ==="
./firebolt-bench --signal spans --reset=false --workers 16 --batch-sizes 16384 --step-seconds 15 \
  --fill-levels 1 --cache-bust --probe-runs 5 \
  --report-out "$OUT/local-firebolt-spans-concurrency.json"

run clickhouse logs    --fill-levels 30000000,60000000,100000000
run clickhouse spans   --fill-levels 30000000,100000000
run clickhouse metrics --fill-levels 30000000,100000000
echo "=== clickhouse spans concurrency ==="
./firebolt-bench --dialect clickhouse --target http://localhost:8123 \
  --signal spans --reset=false --workers 16 --batch-sizes 16384 --step-seconds 15 \
  --fill-levels 1 --cache-bust --probe-runs 5 \
  --report-out "$OUT/local-clickhouse-spans-concurrency.json"
echo "ALL DONE"
