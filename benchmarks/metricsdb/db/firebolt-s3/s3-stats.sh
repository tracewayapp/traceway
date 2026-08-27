#!/usr/bin/env bash
# Sidecar for the firebolt-s3 cell: every 30 s append one JSON line with the
# MinIO S3 request counters (the hidden bill of a disaggregated deployment),
# the object-store directory size, and per-pod memory from the k3s
# metrics-server. Lands in /opt/bench/out so remote-collect ships it with the
# logs; summarize by diffing counters between lines.
#
# Usage: s3-stats.sh <minio-data-dir> <out-jsonl> <namespace>
set -uo pipefail

DATA_DIR="${1:?minio data dir}"
OUT="${2:?output jsonl path}"
NS="${3:-firebolt}"
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

while :; do
    ts="$(date -u +%s)"
    # s3_requests_total{api="..."} counters from the public cluster metrics.
    reqs="$(curl -sf --max-time 5 "http://127.0.0.1:9000/minio/v2/metrics/cluster" 2>/dev/null \
        | awk '/^minio_s3_requests_total\{/ {
            api = $0; sub(/.*api="/, "", api); sub(/".*/, "", api);
            printf "%s\"%s\":%s", sep, api, $NF; sep=","
        } END { print "" }')"
    traffic="$(curl -sf --max-time 5 "http://127.0.0.1:9000/minio/v2/metrics/cluster" 2>/dev/null \
        | awk '/^minio_s3_traffic_(sent|received)_bytes/ { printf "%s\"%s\":%s", sep, $1, $NF; sep="," } END { print "" }')"
    du_bytes="$(du -sb "${DATA_DIR}" 2>/dev/null | cut -f1)"
    pods="$(kubectl -n "${NS}" top pods --no-headers 2>/dev/null \
        | awk '{ printf "%s\"%s\":{\"cpu\":\"%s\",\"mem\":\"%s\"}", sep, $1, $2, $3; sep="," } END { print "" }')"
    printf '{"ts":%s,"du_bytes":%s,"s3_requests":{%s},"s3_traffic":{%s},"pods":{%s}}\n' \
        "${ts}" "${du_bytes:-0}" "${reqs}" "${traffic}" "${pods}" >> "${OUT}"
    sleep 30
done
