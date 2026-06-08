#!/usr/bin/env bash
# Run ONE (tier, mode) cycle end-to-end: provision Hetzner -> bootstrap SUT ->
# run loadgen -> fetch JSON -> tear down. Called by both run-local.sh and the
# GitHub workflow; treat this as the single source of truth for matrix-entry
# orchestration.
#
# Usage: run-matrix-entry.sh <tier> <mode> <signal> <duration> <out-dir> [smoke] [async]
#   <tier>      ccx13 | ccx23 | ccx33 | ccx43
#   <mode>      sqlite | pgch
#   <signal>    spans | metrics | logs
#   <duration>  Loadgen total runtime, e.g. 30m, 3m
#   <out-dir>   Directory to write <tier>-<mode>-<signal>.json into
#   [smoke]     "smoke" to enable short-step overrides (--phase1-batch-sizes
#               256,1024 --phase2-request-rates 1,5 --step-duration 15s).
#               Optional; pass "" to skip when also passing [async].
#   [async]     "async" to set CH_ASYNC_INSERT=1 on the SUT. Output filename
#               gains a -async suffix when set. Optional.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [[ $# -lt 5 ]]; then
    echo "usage: $0 <tier> <mode> <signal> <duration> <out-dir> [smoke] [async]" >&2
    exit 2
fi
TIER="$1"; MODE="$2"; SIGNAL="$3"; DURATION="$4"; OUT_DIR="$5"; SMOKE="${6:-}"; ASYNC_FLAG="${7:-}"
LOCATION="${BENCH_LOCATION:-nbg1}"
SCENARIO="${BENCH_SCENARIO:-throughput}"

case "${SIGNAL}" in
    spans|metrics|logs) ;;
    *) echo "invalid signal '${SIGNAL}' (expected spans|metrics|logs)" >&2; exit 2 ;;
esac

case "${SCENARIO}" in
    throughput|read-probe) ;;
    *) echo "invalid BENCH_SCENARIO '${SCENARIO}' (expected throughput|read-probe)" >&2; exit 2 ;;
esac

# Hetzner caps server names at 63 chars; the prefix `bench-loadgen-` eats 14,
# so the RUN_ID must stay <= 49 chars. Abbreviate the scenario to keep margin
# for the worst combo (e.g. managed-ch + metrics + read-probe).
case "${SCENARIO}" in
    throughput) SCEN_SHORT="tp" ;;
    read-probe) SCEN_SHORT="rp" ;;
    *)          SCEN_SHORT="${SCENARIO}" ;;
esac
RUN_ID="$(date -u +%Y%m%d-%H%M%S)-${TIER}-${MODE}-${SIGNAL}-${SCEN_SHORT}-$RANDOM"
echo "=== run-matrix-entry tier=${TIER} mode=${MODE} signal=${SIGNAL} scenario=${SCENARIO} duration=${DURATION} run_id=${RUN_ID} ===" >&2

mkdir -p "${OUT_DIR}"

# Always tear down — even on failure, even on Ctrl-C. The trap is set BEFORE
# any hcloud create call so a failure mid-provision still cleans up.
cleanup() {
    local rc=$?
    echo "--- teardown for ${RUN_ID} (exit=${rc}) ---" >&2
    "${SCRIPT_DIR}/hetzner-down.sh" "${RUN_ID}" || true
    exit "${rc}"
}
trap cleanup EXIT INT TERM

# 1. Provision.
INFRA_JSON=$("${SCRIPT_DIR}/hetzner-up.sh" "${TIER}" "${RUN_ID}" "${LOCATION}")
echo "infra: ${INFRA_JSON}" >&2
SUT_PUBLIC_IP=$(printf '%s' "${INFRA_JSON}" | jq -r '.sutPublicIp')
SUT_PRIVATE_IP=$(printf '%s' "${INFRA_JSON}" | jq -r '.sutPrivateIp')
LOADGEN_PUBLIC_IP=$(printf '%s' "${INFRA_JSON}" | jq -r '.loadgenPublicIp')

# 2. Bring up the backend on the SUT. CH_ASYNC_INSERT propagates through to
# the docker compose env via sut-bootstrap.sh.
async_suffix=""
if [[ "${ASYNC_FLAG}" == "async" ]]; then
    export CH_ASYNC_INSERT=1
    async_suffix="-async"
    echo "CH_ASYNC_INSERT=1 (async-insert benchmark pass)" >&2
fi
"${SCRIPT_DIR}/sut-bootstrap.sh" "${SUT_PUBLIC_IP}" "${MODE}"

# 3. Run the loadgen, pulling JSON back into OUT_DIR.
extra_args=( --scenario "${SCENARIO}" )
if [[ "${SMOKE}" == "smoke" ]]; then
    if [[ "${SCENARIO}" == "read-probe" ]]; then
        extra_args+=( --fill-levels 100000,1000000 --settle-seconds 5s )
    else
        extra_args+=( --phase1-batch-sizes 256,1024 --phase2-request-rates 1,5 --phase3-request-rates 10,100 --step-duration 15s )
    fi
fi

# Optional read-probe fill-phase overrides. Defaults (8192 × 100 req/s) push
# ~160k items per 200ms drain tick, which blows past low fill-level targets on
# slower DBs (notably SQLite: a "100k" step actually overshoots to ~1M for
# spans and ~4M for logs, smearing the cliff measurement across decades of
# table size). Lowering both knobs together keeps per-step overshoot under the
# next fill-level boundary. Only meaningful when scenario=read-probe; loadgen
# silently ignores them otherwise.
if [[ "${SCENARIO}" == "read-probe" ]]; then
    if [[ -n "${BENCH_FILL_BATCH_SIZE:-}" ]]; then
        extra_args+=( --fill-batch-size "${BENCH_FILL_BATCH_SIZE}" )
    fi
    if [[ -n "${BENCH_FILL_REQUEST_RATE:-}" ]]; then
        extra_args+=( --fill-request-rate "${BENCH_FILL_REQUEST_RATE}" )
    fi
fi

# SQLite has no merge-idle equivalent — /health/deep returns chReachable=false
# and waitForMergesIdle skips immediately. Compensate with a longer per-step
# drain and a fixed inter-phase cooldown so the SUT can finish digesting
# Phase 1's wake (zombie goroutines + WAL checkpoint) before Phase 2 starts.
# Without this, Phase 1 step-cliff contaminates Phase 2's first step.
if [[ "${MODE}" == "sqlite" && "${SCENARIO}" == "throughput" && "${SMOKE}" != "smoke" ]]; then
    extra_args+=( --step-drain-seconds 60s --inter-phase-cooldown-seconds 60s )
fi

OUT_PATH="${OUT_DIR}/${TIER}-${MODE}-${SIGNAL}-${SCENARIO}${async_suffix}.json"
"${SCRIPT_DIR}/loadgen-bootstrap.sh" \
    "${LOADGEN_PUBLIC_IP}" \
    "${SUT_PRIVATE_IP}" \
    "${SUT_PUBLIC_IP}" \
    "${DURATION}" \
    "${TIER}" \
    "${MODE}" \
    "${SIGNAL}" \
    "${OUT_PATH}" \
    "${extra_args[@]}"

# Trap handles teardown — no explicit call needed.
echo "matrix entry ${TIER}-${MODE}-${SIGNAL}-${SCENARIO}${async_suffix} complete -> ${OUT_PATH}" >&2
