#!/usr/bin/env bash
# Firebolt-native tuned pass: aggregating indexes + VACUUM + bucket-aligned
# queries, at the same deep fill levels as run-breaking-point.sh. The ramp is
# kept ON so the index-maintenance cost on inserts is measured too.
set -euo pipefail
cd "$(dirname "$0")"

OUT="${OUT:-../results-firebolt-local}"
mkdir -p "$OUT"

run() { # sig fill-levels
  echo "=== firebolt-tuned $1 deep ==="
  ./firebolt-bench --signal "$1" --fb-tuned --cache-bust --probe-under-write \
    --step-seconds 15 --probe-runs 5 --fill-levels "$2" \
    --report-out "$OUT/local-firebolt-$1-tuned-deep.json"
}

run logs    30000000,60000000,100000000
run spans   30000000,100000000
run metrics 30000000,100000000
echo "TUNED ALL DONE"
