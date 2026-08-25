#!/usr/bin/env bash
# Cold-phase hook: restart the container, drop the page cache, wait healthy.
# Usage: restart.sh [variant]   (falls back to $DB_CONTAINER, then the default name)
set -euo pipefail

VARIANT="${1:-${DB_CONTAINER:-firebolt}}"
# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"

docker restart "${VARIANT}" >/dev/null
wait_http 300 '"data"' -X POST --data-binary 'SELECT 1' "http://127.0.0.1:3473/?output_format=JSON_Compact" \
    || die "${VARIANT} did not come back after restart"
drop_caches
