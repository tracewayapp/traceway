#!/usr/bin/env bash
# Run ONE (tier, mode) cycle end-to-end: provision Hetzner -> bootstrap SUT ->
# run loadgen -> fetch JSON -> tear down. Called by both run-local.sh and the
# GitHub workflow; treat this as the single source of truth for matrix-entry
# orchestration.
#
# Usage: run-matrix-entry.sh <tier> <mode> <signal> <duration> <out-dir> [smoke] [async]
#   <tier>      ccx13 | ccx23 | ccx33 | ccx43
#   <mode>      sqlite | duckdb | pgch | managed-ch
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
source "${SCRIPT_DIR}/_ssh.sh"

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
    # Custom fill ladder (e.g. 10000000,100000000,1000000000 to probe 1B).
    # Appended after any smoke override, so the explicit request wins.
    if [[ -n "${BENCH_FILL_LEVELS:-}" ]]; then
        extra_args+=( --fill-levels "${BENCH_FILL_LEVELS}" )
    fi
fi

# SQLite and DuckDB have no merge-idle equivalent — /health/deep returns
# chReachable=false and waitForMergesIdle skips immediately. Compensate with a
# longer per-step drain and a fixed inter-phase cooldown so the SUT can finish
# digesting Phase 1's wake (zombie goroutines + WAL checkpoint) before Phase 2
# starts. Without this, Phase 1 step-cliff contaminates Phase 2's first step —
# observed on DuckDB as a phase-2 "no requests completed" wipeout while its
# post-phase-1 checkpoint stalled the synchronous ingest path.
if [[ ( "${MODE}" == "sqlite" || "${MODE}" == "duckdb" ) && "${SCENARIO}" == "throughput" && "${SMOKE}" != "smoke" ]]; then
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

if [[ ( "${MODE}" == "sqlite" || "${MODE}" == "duckdb" ) && -f "${OUT_PATH}" ]]; then
    if [[ "${SCENARIO}" == "read-probe" ]]; then
        # rowsIngested is cumulative per step, so max = total.
        disk_rows="$(jq '[.readProbe.steps[]?.rowsIngested] | max // 0' "${OUT_PATH}")"
    else
        disk_rows="$(jq '[.phase1.steps[]?, .phase2.steps[]?, .phase3.steps[]? | (.ingest.ok // 0) * (.batchSize // 0)] | add // 0' "${OUT_PATH}")"
    fi
    case "${MODE}" in
        sqlite) disk_glob='/t/*_telemetry.db*' ;;
        duckdb) disk_glob='/t/*_telemetry.duckdb*' ;;
    esac
    echo "measuring ${MODE} telemetry on-disk size (rows ingested per result JSON: ${disk_rows})" >&2
    # busybox du has no -b, so measure in KB and convert.
    disk_remote="vol=\$(docker volume ls --format '{{.Name}}' | grep -E '${MODE}-data' | head -1)
if [ -z \"\$vol\" ]; then echo 'no matching volume' >&2; exit 3; fi
kb=\$(docker run --rm -v \"\$vol\":/t alpine:3.20 sh -c 'du -ck ${disk_glob} 2>/dev/null | tail -1' | awk '{print \$1}')
if [ -z \"\$kb\" ]; then echo 'du produced no output (telemetry file missing?)' >&2; exit 5; fi
echo \$((kb * 1024))"
    disk_bytes="$(bench_ssh "${SUT_PUBLIC_IP}" "${disk_remote}" | tail -1 || true)"
    if [[ "${disk_bytes}" =~ ^[0-9]+$ ]]; then
        [[ "${disk_rows}" =~ ^[0-9]+$ ]] || disk_rows=0
        tmp_disk="$(mktemp)"
        if jq --argjson b "${disk_bytes}" --argjson r "${disk_rows}" \
              '. + {bytesOnDisk: $b, storedRows: $r}' \
              "${OUT_PATH}" > "${tmp_disk}"; then
            mv "${tmp_disk}" "${OUT_PATH}"
            bps="n/a"; [[ "${disk_rows}" -gt 0 ]] && bps="$(awk -v b="${disk_bytes}" -v r="${disk_rows}" 'BEGIN{printf "%.2f", b/r}')"
            echo "disk: ${MODE} telemetry = ${disk_bytes} bytes over ${disk_rows} ingested items (${bps} bytes/item)" >&2
        else
            rm -f "${tmp_disk}"
            echo "disk: jq patch failed (non-fatal)" >&2
        fi
    else
        echo "disk: measurement failed (got '${disk_bytes}', non-fatal)" >&2
    fi
fi

# Post-mortem before teardown deletes the server — after that, an OOM kill and
# a crash loop are indistinguishable.
if [[ -f "${OUT_PATH}" ]]; then
    sut_state_remote="cid=\$(docker ps -aq --filter 'name=traceway' | head -1)
if [ -z \"\$cid\" ]; then echo '{}'; exit 0; fi
docker inspect --format '{\"status\":\"{{.State.Status}}\",\"oomKilled\":{{.State.OOMKilled}},\"exitCode\":{{.State.ExitCode}},\"restartCount\":{{.RestartCount}}}' \"\$cid\""
    sut_state="$(bench_ssh "${SUT_PUBLIC_IP}" "${sut_state_remote}" | tail -1 || echo '{}')"
    if jq -e . >/dev/null 2>&1 <<< "${sut_state}"; then
        tmp_state="$(mktemp)"
        if jq --argjson s "${sut_state}" '. + {sutContainer: $s}' "${OUT_PATH}" > "${tmp_state}"; then
            mv "${tmp_state}" "${OUT_PATH}"
            echo "sut container state: ${sut_state}" >&2
        else
            rm -f "${tmp_state}"
        fi
    else
        echo "sut container state capture failed (non-fatal): ${sut_state}" >&2
    fi
    bench_ssh "${SUT_PUBLIC_IP}" "cid=\$(docker ps -aq --filter 'name=traceway' | head -1); [ -n \"\$cid\" ] && docker logs --tail 30 \"\$cid\" 2>&1 | sed 's/^/sut-log: /'" >&2 || true
fi

# Trap handles teardown — no explicit call needed.
echo "matrix entry ${TIER}-${MODE}-${SIGNAL}-${SCENARIO}${async_suffix} complete -> ${OUT_PATH}" >&2
