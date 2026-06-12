#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

IMPL="${1:?usage: run-hetzner.sh <traceway-oxc|traceway-goja|honeycomb>}"
SCENARIOS="${SCENARIOS:-hot churn}"
CHURN_ENTRIES="${CHURN_ENTRIES:-512}"
PAD_KB="${PAD_KB:-256}"
CONNECTIONS="${CONNECTIONS:-8,16,32,64,128,256}"
STEP_DURATION="${STEP_DURATION:-60s}"
SPANS_PER_REQUEST="${SPANS_PER_REQUEST:-20}"
SUT_TYPE="${SUT_TYPE:-ccx33}"
LDG_TYPE="${LDG_TYPE:-ccx23}"
LOCATION="${LOCATION:-fsn1}"
RESULTS="${RESULTS:-./results}"
RUN_ID="${GITHUB_RUN_ID:-local}-$IMPL"

command -v hcloud >/dev/null || { echo "hcloud CLI required" >&2; exit 1; }
[ -d artifacts ] || { echo "artifacts/ missing, build first (see workflow or run-local.sh build_all)" >&2; exit 1; }

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

hcloud ssh-key create --name "bench-key-$RUN_ID" --public-key-from-file "$KEY_FILE.pub"
hcloud server create --name "bench-sut-$RUN_ID" --type "$SUT_TYPE" --image ubuntu-24.04 --location "$LOCATION" --ssh-key "bench-key-$RUN_ID"
hcloud server create --name "bench-ldg-$RUN_ID" --type "$LDG_TYPE" --image ubuntu-24.04 --location "$LOCATION" --ssh-key "bench-key-$RUN_ID"
SUT_IP=$(hcloud server ip "bench-sut-$RUN_ID")
LDG_IP=$(hcloud server ip "bench-ldg-$RUN_ID")

for ip in "$SUT_IP" "$LDG_IP"; do
  for i in $(seq 1 60); do $SSH "root@$ip" true 2>/dev/null && break; sleep 5; done
done

for ip in "$SUT_IP" "$LDG_IP"; do
  $SCP -r artifacts "root@$ip:/opt/bench"
  $SSH "root@$ip" "chmod +x /opt/bench/* 2>/dev/null || true"
done

case "$IMPL" in
  traceway-*) COL_BIN=otelcol-bench-traceway; COL_CFG=config-traceway.yaml ;;
  honeycomb)  COL_BIN=otelcol-bench-honeycomb; COL_CFG=config-honeycomb.yaml ;;
esac

$SSH "root@$LDG_IP" "nohup env DRAIN_ADDR=0.0.0.0:9319 /opt/bench/drain > /opt/bench/drain.log 2>&1 & sleep 1; curl -sf http://127.0.0.1:9319/stats"

for scenario in $SCENARIOS; do
  entries=1
  [ "$scenario" = "churn" ] && entries="$CHURN_ENTRIES"
  tag="$IMPL-$scenario"
  outdir="$RESULTS/$tag"
  mkdir -p "$outdir"

  $SSH "root@$SUT_IP" "cd /opt/bench && ./corpusgen --bundle app.mjs --map app.mjs.map --entries $entries --pad-kb $PAD_KB --out corpus-$scenario"
  $SSH "root@$LDG_IP" "cd /opt/bench && ./corpusgen --bundle app.mjs --map app.mjs.map --entries $entries --pad-kb $PAD_KB --out corpus-$scenario"

  $SSH "root@$LDG_IP" "curl -sf -X POST http://127.0.0.1:9319/reset"
  $SSH "root@$SUT_IP" "pkill -f $COL_BIN 2>/dev/null; rm -rf /opt/bench/twcache; cd /opt/bench && nohup env STORE_PATH=./corpus-$scenario CACHE_DIR=./twcache SYMB_PARSER=${IMPL#traceway-} DRAIN_ENDPOINT=http://$LDG_IP:9319 LD_LIBRARY_PATH=/opt/bench ./$COL_BIN --config $COL_CFG > collector.log 2>&1 & sleep 3; pgrep -f $COL_BIN"
  $SSH "root@$SUT_IP" "cd /opt/bench && nohup bash rss-sampler.sh \$(pgrep -f $COL_BIN | head -1) rss.csv > /dev/null 2>&1 &"

  $SSH "root@$LDG_IP" "cd /opt/bench && ./loadgen --target http://$SUT_IP:4318/v1/traces --corpus corpus-$scenario/corpus.json --connections $CONNECTIONS --step-duration $STEP_DURATION --spans-per-request $SPANS_PER_REQUEST --out loadgen.json"

  $SSH "root@$LDG_IP" "curl -sf http://127.0.0.1:9319/stats" > "$outdir/drain.json"
  $SCP "root@$LDG_IP:/opt/bench/loadgen.json" "$outdir/loadgen.json"
  $SSH "root@$SUT_IP" "pkill -f $COL_BIN; sleep 2" || true
  $SCP "root@$SUT_IP:/opt/bench/rss.csv" "$outdir/rss.csv"
  $SCP "root@$SUT_IP:/opt/bench/collector.log" "$outdir/collector.log"
  echo "== $tag =="
  cat "$outdir/drain.json"; echo
done
