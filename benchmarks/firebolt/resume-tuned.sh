#!/usr/bin/env bash
# Resume of run-tuned.sh after the engine restart killed it mid-chain:
# tuned logs completed; run tuned spans + metrics.
set -euo pipefail
cd "$(dirname "$0")"
OUT="${OUT:-../results-firebolt-local}"

# Wait for the engine to actually answer before the first DDL statement.
for i in $(seq 1 60); do
  [ "$(curl -s -m 3 http://localhost:3473/ping)" = "Ok." ] && break
  sleep 3
done

run() { # sig fill-levels
  echo "=== firebolt-tuned $1 deep ==="
  ./firebolt-bench --signal "$1" --fb-tuned --cache-bust --probe-under-write \
    --step-seconds 15 --probe-runs 5 --fill-levels "$2" \
    --report-out "$OUT/local-firebolt-$1-tuned-deep.json"
}

run spans   30000000,100000000
run metrics 30000000,100000000
echo "TUNED ALL DONE"
