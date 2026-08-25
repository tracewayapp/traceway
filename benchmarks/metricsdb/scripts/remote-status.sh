#!/usr/bin/env bash
# One poll of a running cell. Prints: <done|running|gone> <disk-used-%> <last log line>
set -uo pipefail

cd /opt/bench/out 2>/dev/null || { echo "gone 0 -"; exit 0; }

disk="$(df --output=pcent /data 2>/dev/null | tail -1 | tr -dc '0-9')"
# The progress line is redrawn with carriage returns; keep only the last draw.
last="$(tail -n 1 bench.log 2>/dev/null | tr '\r' '\n' | tail -n 1 | cut -c1-220)"

if [[ -f bench.done ]]; then
    state="done"
elif [[ -f bench.pid ]] && kill -0 "$(cat bench.pid)" 2>/dev/null; then
    state="running"
else
    state="gone"
fi
echo "${state} ${disk:-0} ${last:--}"
