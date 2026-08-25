#!/usr/bin/env bash
# Usage: up.sh [variant]
set -euo pipefail

VARIANT="${1:-firebolt}"
# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE="$(tr -d '[:space:]' < "${HERE}/image.txt")"
PORT=3473
FB_DATA_MOUNT="${FB_DATA_MOUNT:-/var/lib/firebolt}"

# Firebolt Core needs io_uring (kernel 6.1+) and at least 16 GB. A laptop
# that deliberately passes a smaller MEM_LIMIT_MB gets a warning instead.
if [[ "${OS}" == "Linux" ]]; then
    kernel_major="$(uname -r | cut -d. -f1)"
    if (( kernel_major < 6 )); then
        die "firebolt core needs kernel >= 6.1, this box runs $(uname -r)"
    fi
fi
if (( MEM_LIMIT_MB < 16384 )); then
    if [[ "${MEM_LIMIT_OVERRIDDEN}" == "1" ]]; then
        warn "firebolt core wants >= 16384 MB, running with MEM_LIMIT_MB=${MEM_LIMIT_MB} because it was set explicitly"
    else
        die "firebolt core needs >= 16 GB, MEM_LIMIT_MB resolved to ${MEM_LIMIT_MB} (set it explicitly to override)"
    fi
fi

reset_variant
# The image runs as the unprivileged "firebolt" user and refuses to start on a
# data dir it cannot write.
chmod 777 "${DATA_DIR}"
pull_image "${IMAGE}"
docker_publish "${PORT}"

docker run -d "${docker_common[@]}" \
    --ulimit memlock=8589934592:8589934592 \
    --security-opt seccomp=unconfined \
    -v "${DATA_DIR}:${FB_DATA_MOUNT}" \
    "${IMAGE}" >/dev/null

if ! wait_http 300 '"data"' -X POST --data-binary 'SELECT 1' "http://127.0.0.1:${PORT}/?output_format=JSON_Compact"; then
    docker logs --tail 50 "${VARIANT}" >&2 || true
    die "${VARIANT} never answered SELECT 1"
fi

CONTAINER="${VARIANT}"
IMAGE_REF="$(image_ref "${IMAGE}")"
CGROUP="$(container_cgroup "${VARIANT}")"
DB_URL="http://127.0.0.1:${PORT}"
# The dev tag moves; the digest is what the result records.
echo "${VARIANT} healthy: ${IMAGE_REF} cpus=${DB_CPUS:-all} mem=${MEM_LIMIT_MB}m data=${DATA_DIR}" >&2
emit_env
