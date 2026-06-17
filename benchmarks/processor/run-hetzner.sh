#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

IMPL="${1:?usage: run-hetzner.sh <impl> (honeycomb, traceway-oxc-mem/-disk, traceway-goja-mem/-disk, traceway-dart-mem/-disk, traceway-ios-mem/-disk, honeycomb-ios)}"
SCENARIOS="${SCENARIOS:-hot churn oom}"
CHURN_ENTRIES="${CHURN_ENTRIES:-512}"
PAD_KB="${PAD_KB:-256}"
OOM_ENTRIES="${OOM_ENTRIES:-4096}"
OOM_PAD_KB="${OOM_PAD_KB:-1024}"
OOM_MAP_PAD_KB="${OOM_MAP_PAD_KB:-1024}"
OOM_MAPPINGS_PAD_KB="${OOM_MAPPINGS_PAD_KB:-1024}"
OOM_CONNECTIONS="${OOM_CONNECTIONS:-32}"
OOM_DURATION="${OOM_DURATION:-30m}"
CONNECTIONS="${CONNECTIONS:-8,16,32,64,128,256}"
STEP_DURATION="${STEP_DURATION:-60s}"
SPANS_PER_REQUEST="${SPANS_PER_REQUEST:-20}"
SUT_TYPE="${SUT_TYPE:-ccx33}"
OOM_SUT_TYPE="${OOM_SUT_TYPE:-ccx13}"
LDG_TYPE="${LDG_TYPE:-cpx42}"
LOCATION="${LOCATION:-fsn1}"
RESULTS="${RESULTS:-./results}"
RUN_ID="${GITHUB_RUN_ID:-local}-$IMPL"

command -v hcloud >/dev/null || { echo "hcloud CLI required" >&2; exit 1; }
[ -d artifacts ] || { echo "artifacts/ missing, build first (see workflow or run-local.sh build_all)" >&2; exit 1; }

SIGNAL=traces
case "$IMPL" in
  honeycomb)          COL_BIN=otelcol-bench-honeycomb; COL_CFG=config-honeycomb.yaml; PARSER=goja; DISK=; LANG=js ;;
  traceway-oxc-mem)   COL_BIN=otelcol-bench-traceway; COL_CFG=config-traceway.yaml; PARSER=oxc; DISK=; LANG=js ;;
  traceway-oxc-disk)  COL_BIN=otelcol-bench-traceway; COL_CFG=config-traceway.yaml; PARSER=oxc; DISK=1; LANG=js ;;
  traceway-goja-mem)  COL_BIN=otelcol-bench-traceway; COL_CFG=config-traceway.yaml; PARSER=goja; DISK=; LANG=js ;;
  traceway-goja-disk) COL_BIN=otelcol-bench-traceway; COL_CFG=config-traceway.yaml; PARSER=goja; DISK=1; LANG=js ;;
  traceway-dart-mem)  COL_BIN=otelcol-bench-traceway; COL_CFG=config-traceway.yaml; PARSER=goja; DISK=; LANG=dart ;;
  traceway-dart-disk) COL_BIN=otelcol-bench-traceway; COL_CFG=config-traceway.yaml; PARSER=goja; DISK=1; LANG=dart ;;
  traceway-ios-mem)   COL_BIN=otelcol-bench-traceway; COL_CFG=config-traceway.yaml; PARSER=goja; DISK=; LANG=ios ;;
  traceway-ios-disk)  COL_BIN=otelcol-bench-traceway; COL_CFG=config-traceway.yaml; PARSER=goja; DISK=1; LANG=ios ;;
  honeycomb-ios)      COL_BIN=otelcol-bench-honeycomb; COL_CFG=config-honeycomb-ios.yaml; PARSER=goja; DISK=; LANG=honeycomb-ios; SIGNAL=logs ;;
  *) echo "unknown impl $IMPL" >&2; exit 1 ;;
esac

case "$LANG" in
  dart) OK_MARKER=crash.dart; FAIL_MARKER=_kDartIsolateSnapshotInstructions ;;
  ios|honeycomb-ios) OK_MARKER=sample.c; FAIL_MARKER=sample+0x ;;
  *)    OK_MARKER=../src/inventory.js; FAIL_MARKER=.mjs:1: ;;
esac

scenario_params() {
  local oom_cache="$OOM_ENTRIES"
  case "$LANG" in ios|honeycomb-ios) oom_cache="${OOM_CACHE:-128}" ;; esac
  case "$1" in
    hot)   echo "1 $PAD_KB 0:0 128 $CONNECTIONS $STEP_DURATION $SUT_TYPE" ;;
    churn) echo "$CHURN_ENTRIES $PAD_KB 0:0 128 $CONNECTIONS $STEP_DURATION $SUT_TYPE" ;;
    oom)   echo "$OOM_ENTRIES $OOM_PAD_KB $OOM_MAP_PAD_KB:$OOM_MAPPINGS_PAD_KB $oom_cache $OOM_CONNECTIONS $OOM_DURATION $OOM_SUT_TYPE" ;;
  esac
}

KEY_FILE=$(mktemp -u)
ssh-keygen -t ed25519 -N '' -f "$KEY_FILE" -q
SSH="ssh -i $KEY_FILE -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=10"
SCP="scp -i $KEY_FILE -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

cleanup() {
  hcloud server delete "bench-sut-$RUN_ID" 2>/dev/null || true
  hcloud server delete "bench-ldg-$RUN_ID" 2>/dev/null || true
  hcloud ssh-key delete "bench-key-$RUN_ID" 2>/dev/null || true
  rm -f "$KEY_FILE" "$KEY_FILE.pub"
}
trap cleanup EXIT

wait_ssh() {
  for i in $(seq 1 60); do $SSH "root@$1" true 2>/dev/null && return 0; sleep 5; done
  return 1
}

provision() {
  local name="$1" type="$2"
  hcloud server create --name "$name" --type "$type" --image ubuntu-24.04 --location "$LOCATION" --ssh-key "bench-key-$RUN_ID" > /dev/null || exit 1
  local ip
  ip=$(hcloud server ip "$name")
  [ -n "$ip" ] || { echo "no ip for $name" >&2; exit 1; }
  wait_ssh "$ip" || { echo "ssh unreachable on $name ($ip)" >&2; exit 1; }
  $SCP -r artifacts "root@$ip:/opt/bench" > /dev/null
  $SSH "root@$ip" "chmod +x /opt/bench/* 2>/dev/null || true"
  echo "$ip"
}

hcloud ssh-key create --name "bench-key-$RUN_ID" --public-key-from-file "$KEY_FILE.pub" > /dev/null
LDG_IP=$(provision "bench-ldg-$RUN_ID" "$LDG_TYPE")
$SSH "root@$LDG_IP" "nohup env DRAIN_ADDR=0.0.0.0:9319 OK_MARKER='$OK_MARKER' FAIL_MARKER='$FAIL_MARKER' /opt/bench/drain > /opt/bench/drain.log 2>&1 & sleep 1; curl -sf http://127.0.0.1:9319/stats"

PREV_SUT_TYPE=""
SUT_IP=""
for scenario in $SCENARIOS; do
  read -r entries pad mappad cachesize conns dur sut_type <<< "$(scenario_params "$scenario")"
  if [ "$sut_type" != "$PREV_SUT_TYPE" ]; then
    [ -n "$SUT_IP" ] && hcloud server delete "bench-sut-$RUN_ID" && sleep 3
    SUT_IP=$(provision "bench-sut-$RUN_ID" "$sut_type")
    PREV_SUT_TYPE="$sut_type"
  fi
  tag="$IMPL-$scenario"
  outdir="$RESULTS/$tag"
  mkdir -p "$outdir"

  case "$LANG" in
    dart) corpusgen_cmd="./corpusgen --language dart --symbols seeds/dart/app.debug.elf --trace seeds/dart/trace.txt --entries $entries --out corpus-$scenario" ;;
    ios)  corpusgen_cmd="./corpusgen --language ios --dsym seeds/ios/app.dsym --trace seeds/ios/trace.txt --entries $entries --out corpus-$scenario" ;;
    honeycomb-ios) corpusgen_cmd="./corpusgen --language honeycomb-ios --dsym seeds/ios/app.dsym --trace seeds/ios/trace.txt --binary sample --entries $entries --out corpus-$scenario" ;;
    *)    corpusgen_cmd="./corpusgen --bundle app.mjs --map app.mjs.map --entries $entries --pad-kb $pad --map-pad-kb ${mappad%%:*} --mappings-pad-kb ${mappad##*:} --out corpus-$scenario" ;;
  esac
  for ip in "$SUT_IP" "$LDG_IP"; do
    $SSH "root@$ip" "cd /opt/bench && [ -f corpus-$scenario/corpus.json ] || $corpusgen_cmd"
  done

  CACHE_DIR_REMOTE=""
  [ -n "$DISK" ] && CACHE_DIR_REMOTE="/opt/bench/twcache"
  $SSH "root@$LDG_IP" "curl -sf -X POST http://127.0.0.1:9319/reset"
  $SSH "root@$SUT_IP" "cd /opt/bench || exit 1; [ ! -f collector.pid ] || { kill \$(cat collector.pid) 2>/dev/null || true; sleep 1; }; rm -rf twcache; nohup env STORE_PATH=./corpus-$scenario CACHE_DIR=$CACHE_DIR_REMOTE CACHE_SIZE=$cachesize SYMB_PARSER=$PARSER DRAIN_ENDPOINT=http://$LDG_IP:9319 LD_LIBRARY_PATH=/opt/bench ./$COL_BIN --config $COL_CFG > collector.log 2>&1 & echo \$! > collector.pid; sleep 3; kill -0 \$!"
  T0=$($SSH "root@$SUT_IP" "date +%s")
  $SSH "root@$SUT_IP" "cd /opt/bench || exit 1; nohup bash rss-sampler.sh \$(cat collector.pid) rss.csv > /dev/null 2>&1 &"

  $SSH "root@$LDG_IP" "rm -f /opt/bench/loadgen-$scenario.json; cd /opt/bench && ./loadgen --target http://$SUT_IP:4318/v1/$SIGNAL --corpus corpus-$scenario/corpus.json --connections $conns --step-duration $dur --spans-per-request $SPANS_PER_REQUEST --out loadgen-$scenario.json" || true

  if ! $SSH "root@$SUT_IP" "kill -0 \$(cat /opt/bench/collector.pid) 2>/dev/null"; then
    TDEAD=$($SSH "root@$SUT_IP" "tail -1 /opt/bench/rss.csv | cut -d, -f1")
    echo "$(( TDEAD - T0 ))" > "$outdir/died"
  fi
  $SSH "root@$LDG_IP" "curl -sf http://127.0.0.1:9319/stats" > "$outdir/drain.json" || echo '{}' > "$outdir/drain.json"
  $SCP "root@$LDG_IP:/opt/bench/loadgen-$scenario.json" "$outdir/loadgen.json" || true
  $SSH "root@$SUT_IP" "kill \$(cat /opt/bench/collector.pid) 2>/dev/null; sleep 2" || true
  $SCP "root@$SUT_IP:/opt/bench/rss.csv" "$outdir/rss.csv" || true
  $SCP "root@$SUT_IP:/opt/bench/collector.log" "$outdir/collector.log" || true
  echo "== $tag =="
  cat "$outdir/drain.json"; echo
done

bash ./summarize.sh "$RESULTS"
