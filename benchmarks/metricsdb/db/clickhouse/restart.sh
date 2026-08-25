#!/usr/bin/env bash
# Cold-phase hook: restart the container, drop the page cache, wait healthy.
# Usage: restart.sh [variant]   (falls back to $DB_CONTAINER, then the default name)
set -euo pipefail

VARIANT="${1:-${DB_CONTAINER:-clickhouse}}"
# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"

docker restart "${VARIANT}" >/dev/null
wait_http 180 '^Ok\.$' "http://127.0.0.1:8123/ping" || die "${VARIANT} did not come back after restart"
drop_caches
