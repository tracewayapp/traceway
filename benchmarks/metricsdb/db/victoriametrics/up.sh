#!/usr/bin/env bash
# Usage: up.sh [variant]   (variant names the container and DATA_DIR)
set -euo pipefail

VARIANT="${1:-victoriametrics}"
# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"

IMAGE="victoriametrics/victoria-metrics:v1.150.0"
PORT=8428

reset_variant
pull_image "${IMAGE}"
docker_publish "${PORT}"

# Series caps off and a 100y retention: the corpus carries simulated
# timestamps over ~1M series, which the defaults would reject or age out.
# -maxConcurrentInserts, -insert.maxQueueDuration and
# -dedup.minScrapeInterval stay at their defaults on purpose.
docker run -d "${docker_common[@]}" \
    -v "${DATA_DIR}:/storage" \
    "${IMAGE}" \
    -storageDataPath=/storage \
    -retentionPeriod=100y \
    -httpListenAddr="${LISTEN_HOST}:${PORT}" \
    -storage.maxHourlySeries=0 \
    -storage.maxDailySeries=0 \
    -search.maxUniqueTimeseries=3000000 \
    -search.maxSeries=3000000 \
    -search.maxPointsPerTimeseries=200000 \
    -search.maxQueryDuration=120s >/dev/null

if ! wait_http 120 '^OK$' "http://127.0.0.1:${PORT}/health"; then
    docker logs --tail 50 "${VARIANT}" >&2 || true
    die "${VARIANT} never became healthy"
fi

CONTAINER="${VARIANT}"
IMAGE_REF="$(image_ref "${IMAGE}")"
CGROUP="$(container_cgroup "${VARIANT}")"
DB_URL="http://127.0.0.1:${PORT}"
echo "${VARIANT} healthy: ${IMAGE_REF} cpus=${DB_CPUS:-all} mem=${MEM_LIMIT_MB}m data=${DATA_DIR}" >&2
emit_env
