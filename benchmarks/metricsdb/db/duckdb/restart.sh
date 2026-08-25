#!/usr/bin/env bash
# Cold-phase hook. DuckDB is in-process, so the bench closes and reopens the
# file itself; the only thing left to do here is drop the page cache.
set -euo pipefail

VARIANT="${1:-${DB_CONTAINER:-duckdb}}"
# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"

drop_caches
