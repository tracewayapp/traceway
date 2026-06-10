#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
source "${SCRIPT_DIR}/_cachebench.sh"

ENTRIES="${ENTRIES:-100,300}"
COLD_RATIOS="${COLD_RATIOS:-0,0.2}"
DURATION="${DURATION:-5s}"
WARMUP="${WARMUP:-2s}"
HOT="${HOT:-10}"
TOKENS="${TOKENS:-4000}"
CONCURRENCY="${CONCURRENCY:-8}"
UNBOUNDED="${UNBOUNDED:-0}"
MEM_BUDGET_MB="${MEM_BUDGET_MB:-256}"
WORK_DIR="${WORK_DIR:-/tmp/cachebench-local}"

BIN="${WORK_DIR}/cachebench"
CORPUS="${WORK_DIR}/corpus"
RESULTS="${WORK_DIR}/results"
TWCACHE="${WORK_DIR}/twcache"

mkdir -p "${WORK_DIR}"
rm -rf "${RESULTS}"
mkdir -p "${RESULTS}"

(cd "${REPO_ROOT}/backend" && go build -o "${BIN}" ./tools/cachebench)

parse_matrix
"${BIN}" generate --corpus-dir "${CORPUS}" --entries "${max_entries}" --tokens "${TOKENS}"

run_cell() {
    local label="$1" mode="$2" n="$3" ratio="$4"
    local extra=()
    case "${label}" in
        memory) extra=(--mem-cache-mb "${MEM_BUDGET_MB}") ;;
        memory-unbounded) extra=(--mem-cache-mb 0) ;;
        disk) rm -rf "${TWCACHE}"; extra=(--disk-cache-dir "${TWCACHE}") ;;
    esac
    local out="${RESULTS}/local-${label}-n${n}-cold${ratio}.json"
    echo "--- cell: ${label} entries=${n} cold=${ratio} ---" >&2
    local rc=0
    "${BIN}" run \
        --corpus-dir "${CORPUS}" --mode "${mode}" --label "${label}" \
        --entries "${n}" --hot "${HOT}" --cold-ratio "${ratio}" \
        --duration "${DURATION}" --warmup "${WARMUP}" --concurrency "${CONCURRENCY}" \
        --tier local --out "${out}" "${extra[@]}" || rc=$?
    if [[ ${rc} -ne 0 ]]; then
        echo "cell exited with ${rc}, recording break" >&2
        write_stub "${out}" local "${label}" "${mode}" "${n}" "${HOT}" "${ratio}" "${rc}"
    fi
}

run_sweep

"${BIN}" table --results-dir "${RESULTS}" --out "${RESULTS}/summary.md"
cat "${RESULTS}/summary.md"
echo "results in ${RESULTS}" >&2
