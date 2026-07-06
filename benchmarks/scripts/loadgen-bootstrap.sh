#!/usr/bin/env bash
# Cross-compile the loadgen for linux/amd64, scp it to the loadgen box, seed a
# project on the SUT, run the loadgen pointed at the SUT's private IP, and pull
# the result JSON back.
#
# Usage: loadgen-bootstrap.sh <loadgen-public-ip> <sut-private-ip> <sut-public-ip> <duration> <tier> <mode> <signal> <out-path> [extra-loadgen-args...]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=_ssh.sh
source "${SCRIPT_DIR}/_ssh.sh"

if [[ $# -lt 8 ]]; then
    echo "usage: $0 <loadgen-public-ip> <sut-private-ip> <sut-public-ip> <duration> <tier> <mode> <signal> <out-path> [extra-loadgen-args...]" >&2
    exit 2
fi
LG_IP="$1"; SUT_PRIVATE_IP="$2"; SUT_PUBLIC_IP="$3"; DURATION="$4"; TIER="$5"; MODE="$6"; SIGNAL="$7"; OUT_PATH="$8"
shift 8

# 1. Seed a project on the SUT (over its PUBLIC IP from the orchestrator). The
#    project token + JWT + project id are then handed to the loadgen, which
#    talks to the SUT over the PRIVATE network for the actual benchmark.
#    Standalone VictoriaMetrics has no /api/register and no auth — empty
#    credentials make the loadgen skip the Authorization header, and the empty
#    JWT short-circuits its /api/health/deep merge-idle polling.
if [[ "${MODE}" == "victoria" ]]; then
    echo "mode=victoria: skipping project seeding (unauthenticated standalone target)" >&2
    JWT=""
    TOKEN=""
    PROJECT_ID=""
else
    echo "seeding project via http://${SUT_PUBLIC_IP}/api/register" >&2
    SEED_JSON=$("${SCRIPT_DIR}/seed-project.sh" "http://${SUT_PUBLIC_IP}")
    JWT=$(printf '%s' "${SEED_JSON}" | jq -r '.jwt')
    TOKEN=$(printf '%s' "${SEED_JSON}" | jq -r '.projectToken')
    PROJECT_ID=$(printf '%s' "${SEED_JSON}" | jq -r '.projectId')
fi

# 2. Wait for SSH on the loadgen box and detect its arch so we cross-compile
#    correctly (CAX* tiers are arm64, CX*/CPX* are amd64).
echo "waiting for ssh on loadgen ${LG_IP}" >&2
wait_for_ssh "${LG_IP}"
LG_UNAME=$(bench_ssh "${LG_IP}" "uname -m" | tr -d '\r\n')
case "${LG_UNAME}" in
    x86_64)  GOARCH=amd64 ;;
    aarch64) GOARCH=arm64 ;;
    *) echo "unknown loadgen arch '${LG_UNAME}'" >&2; exit 1 ;;
esac

# 3. Cross-compile loadgen on the orchestrator (laptop / GH runner) for the
#    loadgen box's arch.
echo "cross-compiling loadgen for linux/${GOARCH}" >&2
(
    cd "${REPO_ROOT}/benchmarks/loadgen"
    GOOS=linux GOARCH="${GOARCH}" CGO_ENABLED=0 go build -o "loadgen-linux-${GOARCH}" .
)

bench_ssh "${LG_IP}" "mkdir -p /root/loadgen"
bench_scp "${REPO_ROOT}/benchmarks/loadgen/loadgen-linux-${GOARCH}" "root@${LG_IP}:/root/loadgen/loadgen"
bench_ssh "${LG_IP}" "chmod +x /root/loadgen/loadgen"

# 4. Run the benchmark, streaming stderr back so progress is visible. The
#    loadgen writes JSON to /root/loadgen/result.json on the loadgen box;
#    we scp it home afterwards. JWT and project ID are passed unconditionally
#    so the read-probe scenario can hit dashboard endpoints; the throughput
#    scenario ignores them.
echo "running loadgen on ${LG_IP} -> http://${SUT_PRIVATE_IP} (tier=${TIER} mode=${MODE} signal=${SIGNAL} duration=${DURATION})" >&2

# Capture the loadgen's exit code without letting set -e abort the script.
# The loadgen now writes its result.json incrementally after every step, so
# even if it dies mid-run (OOM, SSH drop, panic) the file on the SUT has
# everything up to the last completed step. We must scp regardless of the
# loadgen's exit status to avoid losing that partial data.
loadgen_rc=0
loadgen_args=(
    --target "http://${SUT_PRIVATE_IP}"
    --signal "${SIGNAL}"
    --duration "${DURATION}"
    --tier "${TIER}"
    --mode "${MODE}"
    --report-out /root/loadgen/result.json
)
# Only pass credential flags when set. bench_ssh runs the loadgen through ssh,
# which flattens the remote command into one string and drops empty arguments —
# an empty --token/--jwt/--project-id would let the following flag be swallowed
# as its value, truncating the argv (breaks unauthenticated targets like
# standalone VictoriaMetrics, where all three are intentionally empty).
[[ -n "${TOKEN}" ]]      && loadgen_args+=( --token "${TOKEN}" )
[[ -n "${JWT}" ]]        && loadgen_args+=( --jwt "${JWT}" )
[[ -n "${PROJECT_ID}" ]] && loadgen_args+=( --project-id "${PROJECT_ID}" )
bench_ssh "${LG_IP}" /root/loadgen/loadgen "${loadgen_args[@]}" "$@" || loadgen_rc=$?

if [[ "${loadgen_rc}" -ne 0 ]]; then
    echo "loadgen exited with status ${loadgen_rc} — attempting to fetch any partial result.json from the loadgen box" >&2
fi

mkdir -p "$(dirname "${OUT_PATH}")"
if bench_scp "root@${LG_IP}:/root/loadgen/result.json" "${OUT_PATH}" 2>/dev/null; then
    echo "wrote ${OUT_PATH}" >&2
else
    echo "no result.json on loadgen box (loadgen died before writing the first checkpoint)" >&2
fi

exit "${loadgen_rc}"
