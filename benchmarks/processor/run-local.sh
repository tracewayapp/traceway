#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

IMPLS="${IMPLS:-honeycomb traceway-oxc-mem traceway-oxc-disk traceway-goja-mem traceway-goja-disk}"
SCENARIOS="${SCENARIOS:-hot churn oom}"
CHURN_ENTRIES="${CHURN_ENTRIES:-512}"
PAD_KB="${PAD_KB:-256}"
OOM_ENTRIES="${OOM_ENTRIES:-4096}"
OOM_PAD_KB="${OOM_PAD_KB:-1024}"
OOM_MAP_PAD_KB="${OOM_MAP_PAD_KB:-1024}"
OOM_MAPPINGS_PAD_KB="${OOM_MAPPINGS_PAD_KB:-1024}"
OOM_CONNECTIONS="${OOM_CONNECTIONS:-32}"
OOM_DURATION="${OOM_DURATION:-30m}"
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

scenario_params() {
  case "$1" in
    hot)   echo "1 $PAD_KB 0:0 128 $CONNECTIONS $STEP_DURATION" ;;
    churn) echo "$CHURN_ENTRIES $PAD_KB 0:0 128 $CONNECTIONS $STEP_DURATION" ;;
    oom)   echo "$OOM_ENTRIES $OOM_PAD_KB $OOM_MAP_PAD_KB:$OOM_MAPPINGS_PAD_KB $OOM_ENTRIES $OOM_CONNECTIONS $OOM_DURATION" ;;
  esac
}

gen_corpus() {
  local scenario="$1" entries="$2" pad="$3" mappad="$4"
  local dir="./corpus-$scenario"
  if [ ! -f "$dir/corpus.json" ]; then
    ./corpusgen/corpusgen --entries "$entries" --pad-kb "$pad" --map-pad-kb "${mappad%%:*}" --mappings-pad-kb "${mappad##*:}" --out "$dir" >&2
  fi
  echo "$dir"
}

impl_env() {
  case "$1" in
    honeycomb) echo "BIN=./build-honeycomb/otelcol-bench-honeycomb CFG=config-honeycomb.yaml PARSER= DISK=" ;;
    traceway-oxc-mem)   echo "BIN=./build-traceway/otelcol-bench-traceway CFG=config-traceway.yaml PARSER=oxc DISK=" ;;
    traceway-oxc-disk)  echo "BIN=./build-traceway/otelcol-bench-traceway CFG=config-traceway.yaml PARSER=oxc DISK=1" ;;
    traceway-goja-mem)  echo "BIN=./build-traceway/otelcol-bench-traceway CFG=config-traceway.yaml PARSER=goja DISK=" ;;
    traceway-goja-disk) echo "BIN=./build-traceway/otelcol-bench-traceway CFG=config-traceway.yaml PARSER=goja DISK=1" ;;
    *) echo "unknown impl $1" >&2; return 1 ;;
  esac
}

run_one() {
  local impl="$1" scenario="$2"
  read -r entries pad mappad cachesize conns dur <<< "$(scenario_params "$scenario")"
  local store
  store=$(gen_corpus "$scenario" "$entries" "$pad" "$mappad")
  local BIN CFG PARSER DISK
  eval "$(impl_env "$impl")"
  local tag="$impl-$scenario"
  local outdir="$RESULTS/$tag"
  mkdir -p "$outdir"

  pkill -f 'target/release/drain' 2>/dev/null || true
  pkill -f otelcol-bench 2>/dev/null || true
  for i in $(seq 1 10); do
    lsof -iTCP:9319 -sTCP:LISTEN >/dev/null 2>&1 || lsof -iTCP:4318 -sTCP:LISTEN >/dev/null 2>&1 || break
    sleep 1
  done

  DRAIN_ADDR=127.0.0.1:9319 ./drain/target/release/drain &
  local drain_pid=$!
  sleep 1
  curl -sf -X POST http://127.0.0.1:9319/reset > /dev/null

  local cache_dir=""
  [ -n "$DISK" ] && cache_dir=$(mktemp -d)
  STORE_PATH="$store" CACHE_DIR="$cache_dir" CACHE_SIZE="$cachesize" SYMB_PARSER="${PARSER:-goja}" \
    DRAIN_ENDPOINT=http://127.0.0.1:9319 \
    "$BIN" --config "$CFG" > "$outdir/collector.log" 2>&1 &
  local col_pid=$!

  for i in $(seq 1 30); do
    curl -s -o /dev/null http://127.0.0.1:4318/ 2>/dev/null && break
    kill -0 "$col_pid" 2>/dev/null || { echo "collector died at startup, see $outdir/collector.log" >&2; kill "$drain_pid"; return 1; }
    sleep 1
  done

  bash ./rss-sampler.sh "$col_pid" "$outdir/rss.csv" &
  local rss_pid=$!
  local t0
  t0=$(date +%s)

  ./loadgen/loadgen --target http://127.0.0.1:4318/v1/traces --corpus "$store/corpus.json" \
    --connections "$conns" --step-duration "$dur" \
    --spans-per-request "$SPANS_PER_REQUEST" --out "$outdir/loadgen.json" || true

  if ! kill -0 "$col_pid" 2>/dev/null; then
    echo "$(( $(tail -1 "$outdir/rss.csv" | cut -d, -f1) - t0 ))" > "$outdir/died"
  fi
  curl -sf http://127.0.0.1:9319/stats > "$outdir/drain.json" || echo '{}' > "$outdir/drain.json"
  kill "$col_pid" "$drain_pid" 2>/dev/null || true
  wait "$rss_pid" 2>/dev/null || true
  [ -n "$cache_dir" ] && rm -rf "$cache_dir"
  echo "== $tag =="
  cat "$outdir/drain.json"; echo
}

[ -n "$SKIP_BUILD" ] || build_all
for impl in $IMPLS; do
  for scenario in $SCENARIOS; do
    run_one "$impl" "$scenario"
  done
done
bash ./summarize.sh "$RESULTS"
