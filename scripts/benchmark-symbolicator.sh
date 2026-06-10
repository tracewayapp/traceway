#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

BENCHTIME="${BENCHTIME:-2s}"
COUNT="${COUNT:-6}"
OUT="${OUT:-$ROOT_DIR/symbolicator-bench.txt}"

"$SCRIPT_DIR/build-oxc-shim.sh"

cd "$ROOT_DIR/backend"

go test -tags oxc ./app/symbolicator/... -count=1

go test -tags oxc ./app/symbolicator/... \
    -run '^$' \
    -bench 'BenchmarkBundleParsers|BenchmarkNewResolver|BenchmarkOpenTW' \
    -benchtime "$BENCHTIME" \
    -count "$COUNT" \
    -timeout 60m \
    | tee "$OUT"

echo
echo "Results written to $OUT"
