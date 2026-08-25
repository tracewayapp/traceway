#!/usr/bin/env bash
# Breaking-point pass: deep fills + cache-busted probes + probes-under-write,
# run identically against Firebolt and ClickHouse. Sequential on purpose —
# both engines share this machine, parallel runs would pollute each other.
set -euo pipefail
cd "$(dirname "$0")"

OUT="${OUT:-../results-firebolt-local}"
mkdir -p "$OUT"

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

for engine in firebolt clickhouse; do
  run "$engine" logs    --fill-levels 30000000,60000000,100000000
  run "$engine" spans   --fill-levels 30000000,100000000
  run "$engine" metrics --fill-levels 30000000,100000000

  # Concurrency: 16 workers of fat batches into the already-100M spans table,
  # then probes on the freshly-written table.
  echo "=== $engine spans concurrency ==="
  ./firebolt-bench $( [ "$engine" = clickhouse ] && echo "--dialect clickhouse --target http://localhost:8123" ) \
    --signal spans --reset=false --workers 16 --batch-sizes 16384 --step-seconds 15 \
    --fill-levels 1 --cache-bust --probe-runs 5 \
    --report-out "$OUT/local-$engine-spans-concurrency.json"
done
echo "ALL DONE"
