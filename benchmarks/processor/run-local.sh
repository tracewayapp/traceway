#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

IMPLS="${IMPLS:-traceway-oxc honeycomb}"
SCENARIOS="${SCENARIOS:-hot churn}"
CHURN_ENTRIES="${CHURN_ENTRIES:-512}"
PAD_KB="${PAD_KB:-256}"
CONNECTIONS="${CONNECTIONS:-4,8,16,32,64,128}"
STEP_DURATION="${STEP_DURATION:-30s}"
SPANS_PER_REQUEST="${SPANS_PER_REQUEST:-20}"
SKIP_BUILD="${SKIP_BUILD:-}"
RESULTS="${RESULTS:-./results}"

OCB_154=./bin/ocb-0.154.0
OCB_143=./bin/ocb-0.143.0

build_all() {
  mkdir -p bin
  [ -x "$OCB_154" ] || { GOBIN="$PWD/bin" go install go.opentelemetry.io/collector/cmd/builder@v0.154.0 && mv bin/builder "$OCB_154"; }
  [ -x "$OCB_143" ] || { GOBIN="$PWD/bin" go install go.opentelemetry.io/collector/cmd/builder@v0.143.0 && mv bin/builder "$OCB_143"; }

  if echo "$IMPLS" | grep -q traceway; then
    bash ../../scripts/build-oxc-shim.sh
    "$OCB_154" --config manifest-traceway.yaml --skip-compilation
    (cd build-traceway && CGO_ENABLED=1 go build -tags oxc -o otelcol-bench-traceway .)
  fi
  if echo "$IMPLS" | grep -q honeycomb; then
    CGO_ENABLED=1 "$OCB_143" --config manifest-honeycomb.yaml
  fi

  (cd drain && cargo build --release)
  (cd loadgen && go mod tidy >/dev/null 2>&1; go build -o loadgen .)
  (cd corpusgen && go build -o corpusgen .)

  if [ ! -f ../../testing/symbolication/node-app/dist/app.mjs ]; then
    (cd ../../testing/symbolication/node-app && npm install && npm run build)
  fi
}

gen_corpus() {
  local scenario="$1" entries="$2"
  local dir="./corpus-$scenario"
  if [ ! -f "$dir/corpus.json" ]; then
    ./corpusgen/corpusgen --entries "$entries" --pad-kb "$PAD_KB" --out "$dir"
  fi
  echo "$dir"
}

collector_bin() {
  case "$1" in
    traceway-*) echo "./build-traceway/otelcol-bench-traceway" ;;
    honeycomb)  echo "./build-honeycomb/otelcol-bench-honeycomb" ;;
  esac
}

collector_cfg() {
  case "$1" in
    traceway-*) echo "config-traceway.yaml" ;;
    honeycomb)  echo "config-honeycomb.yaml" ;;
  esac
}

run_one() {
  local impl="$1" scenario="$2"
  local entries=1
  [ "$scenario" = "churn" ] && entries="$CHURN_ENTRIES"
  local store
  store=$(gen_corpus "$scenario" "$entries")
  local tag="$impl-$scenario"
  local outdir="$RESULTS/$tag"
  mkdir -p "$outdir"

  DRAIN_ADDR=127.0.0.1:9319 ./drain/target/release/drain &
  local drain_pid=$!
  sleep 1
  curl -sf -X POST http://127.0.0.1:9319/reset > /dev/null

  local cache_dir
  cache_dir=$(mktemp -d)
  STORE_PATH="$store" CACHE_DIR="$cache_dir" SYMB_PARSER="${impl#traceway-}" \
    DRAIN_ENDPOINT=http://127.0.0.1:9319 \
    "$(collector_bin "$impl")" --config "$(collector_cfg "$impl")" > "$outdir/collector.log" 2>&1 &
  local col_pid=$!

  for i in $(seq 1 30); do
    curl -sf -o /dev/null -X POST -H 'Content-Type: application/x-protobuf' --data-binary '' http://127.0.0.1:4318/v1/traces 2>/dev/null && break
    kill -0 "$col_pid" 2>/dev/null || { echo "collector died, see $outdir/collector.log" >&2; kill "$drain_pid"; return 1; }
    sleep 1
  done

  bash ./rss-sampler.sh "$col_pid" "$outdir/rss.csv" &
  local rss_pid=$!

  ./loadgen/loadgen --target http://127.0.0.1:4318/v1/traces --corpus "$store/corpus.json" \
    --connections "$CONNECTIONS" --step-duration "$STEP_DURATION" \
    --spans-per-request "$SPANS_PER_REQUEST" --out "$outdir/loadgen.json"

  curl -sf http://127.0.0.1:9319/stats > "$outdir/drain.json"
  kill "$col_pid" "$drain_pid" 2>/dev/null || true
  wait "$rss_pid" 2>/dev/null || true
  rm -rf "$cache_dir"
  echo "== $tag =="
  cat "$outdir/drain.json"; echo
}

summarize() {
  echo
  printf '%-28s %12s %12s %10s %14s\n' run max_stacks/s peak_rss_mb p99_ms drain_symb_pct
  for d in "$RESULTS"/*/; do
    [ -f "$d/loadgen.json" ] || continue
    local name peak maxr p99 pct
    name=$(basename "$d")
    maxr=$(jq '[.[].stacks_per_sec] | max' "$d/loadgen.json")
    p99=$(jq '[.[] | select(.stacks_per_sec == ([.[].stacks_per_sec] | max))] | .[0].p99_ms' "$d/loadgen.json" 2>/dev/null || jq '.[-1].p99_ms' "$d/loadgen.json")
    peak=$(tail -n +2 "$d/rss.csv" | cut -d, -f2 | sort -n | tail -1)
    pct=$(jq -r 'if .requests > 0 then (100 * .symbolicated / .requests | floor) else 0 end' "$d/drain.json")
    printf '%-28s %12.0f %12d %10.1f %13s%%\n' "$name" "$maxr" "$((${peak:-0} / 1024))" "$p99" "$pct"
  done
}

[ -n "$SKIP_BUILD" ] || build_all
for impl in $IMPLS; do
  for scenario in $SCENARIOS; do
    run_one "$impl" "$scenario"
  done
done
summarize
