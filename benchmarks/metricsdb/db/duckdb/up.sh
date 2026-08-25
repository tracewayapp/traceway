#!/usr/bin/env bash
# Usage: up.sh [variant]
# No container: the bench opens DATA_DIR/points.duckdb in-process. This only
# prepares the directory and reports the bench's own cgroup as the thing to
# sample, which entry.sh creates right before launch.
set -euo pipefail

VARIANT="${1:-duckdb}"
# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

reset_variant
rm -f "${DATA_DIR}"/points.duckdb*
mkdir -p "${DATA_DIR}/tmp"

if [[ -f /sys/fs/cgroup/cgroup.controllers ]]; then
    CGROUP="/sys/fs/cgroup/metricsdb-bench"
fi

lib_version=""
for candidate in "${HERE}/../../bench/duckdb-version.txt" "${HERE}/../../duckdb-version.txt"; do
    if [[ -f "${candidate}" ]]; then
        lib_version="$(tr -d '[:space:]' < "${candidate}")"
        break
    fi
done
IMAGE_REF="libduckdb:${lib_version:-unknown}"
CONTAINER=""
DB_URL=""
echo "${VARIANT} ready: ${IMAGE_REF} threads=${DB_CORES} mem=${MEM_LIMIT_MB}m data=${DATA_DIR}" >&2
emit_env
