#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
source "${SCRIPT_DIR}/_ssh.sh"
source "${SCRIPT_DIR}/_cachebench.sh"

TIER="${TIER:-ccx13}"
LOCATION="${LOCATION:-nbg1}"
IMAGE="${BENCH_IMAGE:-debian-12}"
OUT_DIR="${OUT_DIR:-${REPO_ROOT}/benchmarks/results-cachebench}"

ENTRIES="${ENTRIES:-1000,2000,4000,6000,8000,12000}"
COLD_RATIOS="${COLD_RATIOS:-0,0.05,0.25}"
DURATION="${DURATION:-60s}"
WARMUP="${WARMUP:-10s}"
HOT="${HOT:-30}"
TOKENS="${TOKENS:-40000}"
CONCURRENCY="${CONCURRENCY:-16}"
UNBOUNDED="${UNBOUNDED:-1}"
DISK_CACHE_MB="${DISK_CACHE_MB:-16384}"

if [[ "${SMOKE:-0}" == "1" ]]; then
    ENTRIES="200"
    COLD_RATIOS="0.1"
    DURATION="10s"
    WARMUP="3s"
    TOKENS="4000"
    UNBOUNDED="0"
fi

for tool in hcloud go ssh scp; do
    command -v "${tool}" >/dev/null || { echo "missing required tool: ${tool}" >&2; exit 1; }
done
[[ -n "${HCLOUD_TOKEN:-}" ]] || { echo "HCLOUD_TOKEN is required" >&2; exit 1; }

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d-%H%M%S)-cachebench-${TIER}-$RANDOM}"
SERVER_NAME="bench-cachebench-${RUN_ID}"

cleanup() {
    local rc=$?
    echo "--- teardown for ${RUN_ID} (exit=${rc}) ---" >&2
    hcloud server delete "${SERVER_NAME}" >/dev/null 2>&1 || true
    exit "${rc}"
}
trap cleanup EXIT INT TERM

if ! hcloud server describe "${SERVER_NAME}" >/dev/null 2>&1; then
    echo "creating server ${SERVER_NAME} (${TIER}) in ${LOCATION}" >&2
    retry_eof hcloud server create \
        --name "${SERVER_NAME}" --type "${TIER}" --image "${IMAGE}" \
        --location "${LOCATION}" --ssh-key benchmark-key \
        --label "bench=true,run=${RUN_ID}" >/dev/null
fi
SUT_IP="$(hcloud server ip "${SERVER_NAME}")"
echo "server up at ${SUT_IP}" >&2

wait_for_ssh "${SUT_IP}"

ARCH="$(bench_ssh "${SUT_IP}" "uname -m")"
case "${ARCH}" in
    x86_64) GOARCH=amd64 ;;
    aarch64) GOARCH=arm64 ;;
    *) echo "unsupported SUT arch ${ARCH}" >&2; exit 1 ;;
esac

(cd "${REPO_ROOT}/backend" && GOOS=linux GOARCH="${GOARCH}" CGO_ENABLED=0 go build -o "/tmp/cachebench-linux-${GOARCH}" ./tools/cachebench)
bench_ssh "${SUT_IP}" "mkdir -p /root/cachebench/results"
bench_scp "/tmp/cachebench-linux-${GOARCH}" "root@${SUT_IP}:/root/cachebench/cachebench"
bench_ssh "${SUT_IP}" "chmod +x /root/cachebench/cachebench"

parse_matrix

echo "generating corpus: ${max_entries} entries, ${TOKENS} tokens each" >&2
bench_ssh "${SUT_IP}" "/root/cachebench/cachebench generate --corpus-dir /root/cachebench/corpus --entries ${max_entries} --tokens ${TOKENS}"

MEM_TOTAL_MB="$(bench_ssh "${SUT_IP}" "awk '/MemTotal/{print int(\$2/1024)}' /proc/meminfo")"
MEM_BUDGET_MB="${MEM_BUDGET_MB:-$(( MEM_TOTAL_MB * 70 / 100 ))}"
echo "box has ${MEM_TOTAL_MB} MB RAM, bounded memory cache budget ${MEM_BUDGET_MB} MB" >&2

mkdir -p "${OUT_DIR}"
rm -f "${OUT_DIR}"/*.json "${OUT_DIR}/summary.md" 2>/dev/null || true

TIMEOUT_CMD="$(command -v timeout || command -v gtimeout || true)"
dur_s="$(( $(echo "${DURATION}" | sed 's/s$//') + $(echo "${WARMUP}" | sed 's/s$//') + 300 ))"

run_cell() {
    local label="$1" mode="$2" n="$3" ratio="$4"
    local extra=""
    case "${label}" in
        memory) extra="--mem-cache-mb ${MEM_BUDGET_MB}" ;;
        memory-unbounded) extra="--mem-cache-mb 0" ;;
        disk) extra="--disk-cache-dir /root/cachebench/twcache --disk-cache-mb ${DISK_CACHE_MB}" ;;
    esac
    local cell="${TIER}-${label}-n${n}-cold${ratio}"
    local remote_out="/root/cachebench/results/${cell}.json"
    local local_out="${OUT_DIR}/${cell}.json"
    echo "--- cell: ${cell} ---" >&2
    bench_ssh "${SUT_IP}" "rm -rf /root/cachebench/twcache && sync && echo 3 > /proc/sys/vm/drop_caches"
    local rc=0
    ${TIMEOUT_CMD:+${TIMEOUT_CMD} ${dur_s}} ssh "${ssh_opts[@]}" "root@${SUT_IP}" \
        "/root/cachebench/cachebench run --corpus-dir /root/cachebench/corpus --mode ${mode} --label ${label} --entries ${n} --hot ${HOT} --cold-ratio ${ratio} --duration ${DURATION} --warmup ${WARMUP} --concurrency ${CONCURRENCY} --tier ${TIER} --out ${remote_out} ${extra}" || rc=$?
    if [[ ${rc} -ne 0 ]]; then
        echo "cell ${cell} exited with ${rc}, recording break" >&2
        write_stub "${local_out}" "${TIER}" "${label}" "${mode}" "${n}" "${HOT}" "${ratio}" "${rc}"
        wait_for_ssh "${SUT_IP}" 300
        return 0
    fi
    bench_scp "root@${SUT_IP}:${remote_out}" "${local_out}"
}

run_sweep

(cd "${REPO_ROOT}/backend" && go build -o /tmp/cachebench-host ./tools/cachebench)
/tmp/cachebench-host table --results-dir "${OUT_DIR}" --out "${OUT_DIR}/summary.md"
cat "${OUT_DIR}/summary.md"
echo "results in ${OUT_DIR}" >&2
