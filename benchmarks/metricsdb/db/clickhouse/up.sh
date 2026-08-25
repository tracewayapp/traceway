#!/usr/bin/env bash
# Usage: up.sh [variant]   (clickhouse or clickhouse-map; names the container and DATA_DIR)
set -euo pipefail

VARIANT="${1:-clickhouse}"
# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE="clickhouse/clickhouse-server:26.3.22-alpine"
HTTP_PORT=8123
NATIVE_PORT=9000

reset_variant
render_template "${HERE}/config.d/bench.xml.tmpl" "${HERE}/config.d/bench.xml"
render_template "${HERE}/users.d/bench.xml.tmpl" "${HERE}/users.d/bench.xml"
pull_image "${IMAGE}"
docker_publish "${HTTP_PORT}" "${NATIVE_PORT}"

# The server config is mounted over the image's docker_related_config.xml:
# that file adds listen_host :: and 0.0.0.0, and config.d merging appends
# listen_host entries instead of replacing them, so shadowing the file is the
# only way to end up with exactly one listener.
docker run -d "${docker_common[@]}" \
    -e TZ=UTC \
    --ulimit nofile=262144:262144 \
    -v "${DATA_DIR}:/var/lib/clickhouse" \
    -v "${HERE}/config.d/bench.xml:/etc/clickhouse-server/config.d/docker_related_config.xml:ro" \
    -v "${HERE}/users.d/bench.xml:/etc/clickhouse-server/users.d/bench.xml:ro" \
    "${IMAGE}" >/dev/null

if ! wait_http 180 '^Ok\.$' "http://127.0.0.1:${HTTP_PORT}/ping"; then
    docker logs --tail 50 "${VARIANT}" >&2 || true
    die "${VARIANT} never became healthy"
fi

CONTAINER="${VARIANT}"
IMAGE_REF="$(image_ref "${IMAGE}")"
CGROUP="$(container_cgroup "${VARIANT}")"
DB_URL="http://127.0.0.1:${HTTP_PORT}"
echo "${VARIANT} healthy: ${IMAGE_REF} cpus=${DB_CPUS:-all} (${DB_CORES} cores) mem=${MEM_LIMIT_MB}m data=${DATA_DIR}" >&2
emit_env
