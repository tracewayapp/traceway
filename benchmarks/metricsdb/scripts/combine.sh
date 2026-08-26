#!/usr/bin/env bash
# Pull the per-store result artifacts of one or more workflow runs into one
# folder and render a combined summary.md / report.html / charts over them.
# Runs dispatched for different stores execute side by side and each only
# summarizes its own cells; this is how the four end up on one page.
#
# Usage: combine.sh <out-dir> <run-id> [<run-id>...]
#   run ids come from the Actions URL (.../actions/runs/<id>) or `gh run list`
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${1:?usage: combine.sh <out-dir> <run-id> [<run-id>...]}"
shift
(( $# )) || { echo "usage: combine.sh <out-dir> <run-id> [<run-id>...]" >&2; exit 2; }
command -v gh >/dev/null || { echo "gh is required" >&2; exit 1; }

mkdir -p "${OUT}"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
for run in "$@"; do
    echo "downloading results of run ${run}" >&2
    gh run download "${run}" --pattern 'metricsdb-result-*' -D "${tmp}/${run}"
    find "${tmp}/${run}" -maxdepth 2 -name '*.json' -exec cp {} "${OUT}/" \;
done
ls "${OUT}"/*.json >&2
python3 "${SCRIPT_DIR}/summarize.py" "${OUT}"
cat "${OUT}/summary.md"
