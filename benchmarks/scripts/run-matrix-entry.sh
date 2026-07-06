#!/usr/bin/env bash
# Run ONE (tier, mode) cycle end-to-end: provision Hetzner -> bootstrap SUT ->
# run loadgen -> fetch JSON -> tear down. Called by both run-local.sh and the
# GitHub workflow; treat this as the single source of truth for matrix-entry
# orchestration.
#
# Usage: run-matrix-entry.sh <tier> <mode> <signal> <duration> <out-dir> [smoke] [async]
#   <tier>      ccx13 | ccx23 | ccx33 | ccx43
#   <mode>      sqlite | pgch | managed-ch | victoria (standalone VictoriaMetrics,
#               no Traceway — metrics signal + throughput scenario only)
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

# Standalone VictoriaMetrics only ingests OTLP metrics (spans/logs are separate
# VictoriaTraces/VictoriaLogs products) and has no Traceway dashboard to probe.
if [[ "${MODE}" == "victoria" ]]; then
    if [[ "${SIGNAL}" != "metrics" ]]; then
        echo "mode=victoria supports signal=metrics only (got '${SIGNAL}')" >&2; exit 2
    fi
    if [[ "${SCENARIO}" != "throughput" ]]; then
        echo "mode=victoria supports scenario=throughput only (got '${SCENARIO}')" >&2; exit 2
    fi
fi

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
if [[ "${MODE}" == "victoria" ]]; then
    extra_args+=( --otlp-path-prefix /opentelemetry )
fi
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

# Free-form loadgen flag overrides (e.g. "--phase2-request-rates 25,40,55,70,85,100"
# to pin a cliff at finer granularity). Appended last so a repeated flag wins
# over any default set above. Space-separated; not for values containing spaces.
if [[ -n "${BENCH_LOADGEN_EXTRA_ARGS:-}" ]]; then
    read -ra _loadgen_extra <<< "${BENCH_LOADGEN_EXTRA_ARGS}"
    extra_args+=( "${_loadgen_extra[@]}" )
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

# VM-only on-disk / compression measurement. VM runs the throughput scenario
# (it has no Traceway dashboard, so read-probe is guarded off), and unlike a
# cross-backend comparison we don't need fixed row counts here: VM reports its
# own stored-sample count in /metrics (vm_rows), so bytesOnDisk / storedRows is
# an exact bytes-per-sample regardless of how much was ingested. To avoid
# counting un-merged parts, force_merge and wait for the active-merge gauge to
# drain before sizing; `du` runs from a throwaway alpine container mounting VM's
# data volume, since the VM image is scratch-based (no du of its own). If the
# vm_active_merges metric name ever changes, the loop just waits the full
# timeout then measures - never worse than a fixed settle. All non-fatal.
DISK_COMPACT_WAIT="${BENCH_DISK_COMPACT_WAIT:-300}"
if [[ "${MODE}" == "victoria" && -f "${OUT_PATH}" ]]; then
    echo "measuring VM on-disk size (force_merge + up to ${DISK_COMPACT_WAIT}s compaction wait)" >&2
    disk_remote="set -e
curl -sf -X POST 'http://localhost:80/internal/force_merge' >/dev/null 2>&1 || true
deadline=\$(( \$(date +%s) + ${DISK_COMPACT_WAIT} ))
while [ \"\$(date +%s)\" -lt \"\$deadline\" ]; do
    m=\$(curl -s 'http://localhost:80/metrics')
    present=\$(printf '%s' \"\$m\" | grep -c '^vm_active_merges' || true)
    active=\$(printf '%s' \"\$m\" | awk '/^vm_active_merges/{s+=\$2} END{print s+0}')
    [ \"\$present\" -gt 0 ] && [ \"\$active\" = '0' ] && break
    sleep 10
done
sleep 2
rows=\$(curl -s 'http://localhost:80/metrics' | awk '/^vm_rows{type=\"storage\//{s+=\$2} END{print s+0}')
vol=\$(docker volume ls --format '{{.Name}}' | grep -E 'victoria-bench-data' | head -1)
[ -n \"\$vol\" ] || { echo 'no matching volume' >&2; exit 3; }
bytes=\$(docker run --rm -v \"\$vol\":/t alpine:3.20 du -sb /t | awk '{print \$1}')
echo \"\$bytes \$rows\""
    disk_out="$(bench_ssh "${SUT_PUBLIC_IP}" "${disk_remote}" 2>/dev/null | tail -1 || true)"
    disk_bytes="${disk_out%% *}"
    disk_rows="${disk_out##* }"
    if [[ "${disk_bytes}" =~ ^[0-9]+$ ]]; then
        [[ "${disk_rows}" =~ ^[0-9]+$ ]] || disk_rows=0
        tmp_disk="$(mktemp)"
        if jq --argjson b "${disk_bytes}" --argjson r "${disk_rows}" \
              '. + {bytesOnDisk: $b, storedRows: $r, diskForceMerged: true}' \
              "${OUT_PATH}" > "${tmp_disk}"; then
            mv "${tmp_disk}" "${OUT_PATH}"
            bps="n/a"; [[ "${disk_rows}" -gt 0 ]] && bps="$(awk -v b="${disk_bytes}" -v r="${disk_rows}" 'BEGIN{printf "%.2f", b/r}')"
            echo "disk: victoria = ${disk_bytes} bytes over ${disk_rows} stored samples (${bps} bytes/sample, post force_merge)" >&2
        else
            rm -f "${tmp_disk}"
            echo "disk: jq patch failed (non-fatal)" >&2
        fi
    else
        echo "disk: measurement failed (got '${disk_out}', non-fatal)" >&2
    fi
fi

# Trap handles teardown — no explicit call needed.
echo "matrix entry ${TIER}-${MODE}-${SIGNAL}-${SCENARIO}${async_suffix} complete -> ${OUT_PATH}" >&2
