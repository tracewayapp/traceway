#!/usr/bin/env bash
# Run the metricsdb benchmark from a laptop.
#
# Default: mirror one cell locally with Docker (Docker Desktop on macOS works;
# Firebolt is best effort there). Builds the bench, starts the database,
# runs the bench in the foreground, then summarizes results-local/.
#
# --hetzner: the real thing, sequentially over the listed databases, through
# scripts/entry.sh exactly like the GitHub workflow. Needs HCLOUD_TOKEN,
# BENCHMARK_SSH_KEY and a linux-amd64 artifact in _artifact/ (built here on
# linux-amd64, or downloaded from the workflow's build job with
# `gh run download -n metricsdb-bench -D benchmarks/metricsdb/_artifact`).
#
# Usage: run-local.sh <db>[,<db>...] [--smoke] [--hetzner] [--dry-run]
#   <db>       victoriametrics | clickhouse | clickhouse-map | duckdb | firebolt
#   --smoke    20M points / 10k series / 5 min ingest / 1 min settle / 2 min cold
#   --hetzner  provision Hetzner boxes instead of running locally
#   --dry-run  print the plan (and run preflight for --hetzner) without running anything
#
# Env overrides: TIER LOCATION TARGET_POINTS SERIES INTERVAL_SECONDS MAX_MINUTES
# MAX_SETTLE_MINUTES MAX_COLD_MINUTES QUERY_THRESHOLD_MS WRITERS BATCH_SIZE
# FB_STAGE OUT_DIR DATA_ROOT MEM_LIMIT_MB EXTRA_ARGS (extra metricsdb-bench flags,
# word-split, e.g. "--warmup 10s --gen-threads 4")
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MDB_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

usage() {
    sed -n '2,23p' "$0" >&2
    exit 2
}

DBS=""
SMOKE=0
HETZNER=0
DRY_RUN=0
while [[ $# -gt 0 ]]; do
    case "$1" in
        --smoke)   SMOKE=1; shift ;;
        --hetzner) HETZNER=1; shift ;;
        --dry-run) DRY_RUN=1; shift ;;
        -h|--help) usage ;;
        -*)        echo "unknown flag: $1" >&2; usage ;;
        *)         DBS="$1"; shift ;;
    esac
done
[[ -n "${DBS}" ]] || usage

IFS=',' read -ra db_list <<< "${DBS}"
for db in "${db_list[@]}"; do
    case "${db}" in
        victoriametrics|clickhouse|clickhouse-map|duckdb|firebolt) ;;
        *) echo "unknown db '${db}'" >&2; exit 2 ;;
    esac
done

TARGET_POINTS="${TARGET_POINTS:-10000000000}"
SERIES="${SERIES:-1000000}"
INTERVAL_SECONDS="${INTERVAL_SECONDS:-10}"
MAX_MINUTES="${MAX_MINUTES:-225}"
MAX_SETTLE_MINUTES="${MAX_SETTLE_MINUTES:-30}"
MAX_COLD_MINUTES="${MAX_COLD_MINUTES:-30}"
QUERY_THRESHOLD_MS="${QUERY_THRESHOLD_MS:-5000}"
WRITERS="${WRITERS:-}"
BATCH_SIZE="${BATCH_SIZE:-}"
FB_STAGE="${FB_STAGE:-upload}"
EXTRA_ARGS="${EXTRA_ARGS:-}"
if (( SMOKE )); then
    TARGET_POINTS=20000000
    SERIES=10000
    MAX_MINUTES=5
    MAX_SETTLE_MINUTES=1
    MAX_COLD_MINUTES=2
fi

case "$(uname -s)-$(uname -m)" in
    Linux-x86_64)  PLATFORM="linux-amd64" ;;
    Linux-aarch64) PLATFORM="linux-arm64" ;;
    Darwin-*)      PLATFORM="osx-universal" ;;
    *) echo "unsupported host $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac

build_bench() {
    "${SCRIPT_DIR}/fetch-libduckdb.sh" "${PLATFORM}"
    echo "building metricsdb-bench (release)" >&2
    (cd "${MDB_ROOT}/bench" && DUCKDB_LIB_DIR="${MDB_ROOT}/lib" DUCKDB_INCLUDE_DIR="${MDB_ROOT}/lib" cargo build --release)
}

summarize() {
    if [[ -f "${SCRIPT_DIR}/summarize.py" ]]; then
        python3 "${SCRIPT_DIR}/summarize.py" "${OUT_DIR}"
        [[ -f "${OUT_DIR}/summary.md" ]] && cat "${OUT_DIR}/summary.md"
    else
        echo "scripts/summarize.py not present, skipping summary" >&2
    fi
    echo "results in ${OUT_DIR}" >&2
}

# ---------------------------------------------------------------- hetzner
if (( HETZNER )); then
    export TIER="${TIER:-ccx33}" LOCATION="${LOCATION:-nbg1}"
    export TARGET_POINTS SERIES INTERVAL_SECONDS MAX_MINUTES MAX_SETTLE_MINUTES MAX_COLD_MINUTES
    export QUERY_THRESHOLD_MS WRITERS BATCH_SIZE FB_STAGE EXTRA_ARGS
    export SMOKE
    export OUT_DIR="${OUT_DIR:-${MDB_ROOT}/results-local}"
    export ARTIFACT_DIR="${ARTIFACT_DIR:-${MDB_ROOT}/_artifact}"

    if [[ ! -f "${ARTIFACT_DIR}/metricsdb-bench" ]]; then
        if [[ "${PLATFORM}" == "linux-amd64" ]]; then
            build_bench
            rm -rf "${ARTIFACT_DIR}"
            mkdir -p "${ARTIFACT_DIR}"
            cp "${MDB_ROOT}/bench/target/release/metricsdb-bench" "${MDB_ROOT}/lib/libduckdb.so" "${MDB_ROOT}/bench/duckdb-version.txt" "${ARTIFACT_DIR}/"
            cp -R "${MDB_ROOT}/db" "${ARTIFACT_DIR}/db"
            cp "${SCRIPT_DIR}"/remote-*.sh "${ARTIFACT_DIR}/"
        else
            echo "no linux-amd64 artifact at ${ARTIFACT_DIR}; build it on linux-amd64 or download the workflow's metricsdb-bench artifact there" >&2
            exit 1
        fi
    fi

    for db in "${db_list[@]}"; do
        "${SCRIPT_DIR}/preflight.sh" "${db}"
    done
    echo "plan: tier=${TIER} location=${LOCATION} dbs=${DBS} points=${TARGET_POINTS} series=${SERIES} ingest=${MAX_MINUTES}m smoke=${SMOKE} out=${OUT_DIR}" >&2
    if (( DRY_RUN )); then
        echo "dry run: nothing provisioned" >&2
        exit 0
    fi

    mkdir -p "${OUT_DIR}"
    failed=()
    for db in "${db_list[@]}"; do
        "${SCRIPT_DIR}/entry.sh" "${db}" || failed+=("${db}")
    done
    summarize
    if (( ${#failed[@]} )); then
        echo "cells without a real result: ${failed[*]}" >&2
        exit 1
    fi
    exit 0
fi

# ------------------------------------------------------------------ local
TIER="${TIER:-local}"
export DB_CPUS="" BENCH_CPUS=""
export DATA_ROOT="${DATA_ROOT:-${MDB_ROOT}/results-local/data}"
DATA_ROOT="${DATA_ROOT%/}"
export OUT_DIR="${OUT_DIR:-${MDB_ROOT}/results-local}"
mkdir -p "${OUT_DIR}"
OUT_DIR="$(cd "${OUT_DIR}" && pwd)"
export RESET=1

command -v docker >/dev/null || { echo "docker is required for local runs" >&2; exit 1; }
command -v cargo >/dev/null || { echo "cargo is required to build the bench" >&2; exit 1; }
echo "plan: local tier=${TIER} dbs=${DBS} points=${TARGET_POINTS} series=${SERIES} ingest=${MAX_MINUTES}m smoke=${SMOKE} data=${DATA_ROOT} out=${OUT_DIR}" >&2
if (( DRY_RUN )); then
    exit 0
fi

build_bench
BIN="${MDB_ROOT}/bench/target/release/metricsdb-bench"
export LD_LIBRARY_PATH="${MDB_ROOT}/lib${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"
export DYLD_LIBRARY_PATH="${MDB_ROOT}/lib${DYLD_LIBRARY_PATH:+:${DYLD_LIBRARY_PATH}}"

failed=()
for db in "${db_list[@]}"; do
    db_dir="${db%-map}"
    run_id="$(date -u +%Y%m%d-%H%M%S)-local-${db}"
    result="${OUT_DIR}/${TIER}-${db}.json"
    echo "--- cell: ${TIER}-${db} ---" >&2

    if ! db_env="$("${MDB_ROOT}/db/${db_dir}/up.sh" "${db}")"; then
        if [[ "${db}" == "firebolt" ]]; then
            echo "WARN: firebolt could not start here (Docker Desktop is best effort); skipping" >&2
        else
            echo "${db} failed to start" >&2
        fi
        failed+=("${db}")
        continue
    fi
    eval "${db_env}"
    export DB_CONTAINER="${CONTAINER}"

    args=(
        --db "${db}"
        --points "${TARGET_POINTS}"
        --series "${SERIES}"
        --interval "${INTERVAL_SECONDS}s"
        --max-ingest "${MAX_MINUTES}m"
        --max-settle "${MAX_SETTLE_MINUTES}m"
        --max-cold "${MAX_COLD_MINUTES}m"
        --query-slow-ms "${QUERY_THRESHOLD_MS}"
        --data-dir "${DATA_DIR}"
        --cold-hook "${MDB_ROOT}/db/${db_dir}/restart.sh"
        --tier "${TIER}"
        --run-id "${run_id}"
        --out "${result}"
    )
    if [[ -n "${WRITERS}" ]]; then
        args+=(--writers "${WRITERS}")
    fi
    if [[ -n "${BATCH_SIZE}" ]]; then
        args+=(--batch-points "${BATCH_SIZE}")
    fi
    # Docker Desktop exposes no cgroup, and an unprivileged Linux user cannot
    # create the bench's own; sampling is skipped rather than failing the run.
    if [[ -n "${CGROUP}" && -d "${CGROUP}" ]]; then
        args+=(--cgroup "${CGROUP}")
    fi
    if [[ -n "${CONTAINER}" ]]; then
        args+=(--db-container "${CONTAINER}")
    fi
    if [[ -n "${DB_URL}" ]]; then
        args+=(--url "${DB_URL}")
    fi
    if [[ "${db}" == "duckdb" ]]; then
        args+=(--duckdb-path "${DATA_DIR}/points.duckdb" --duckdb-threads "${DB_CORES}" --duckdb-memory-limit "${MEM_LIMIT_MB}MB")
    fi
    if [[ "${db}" == "firebolt" ]]; then
        args+=(--fb-stage "${FB_STAGE}")
        if [[ "${FB_STAGE}" == "s3" ]]; then
            s3_env="$("${MDB_ROOT}/db/firebolt/minio-up.sh")"
            eval "${s3_env}"
            args+=(--fb-s3-endpoint "${FB_S3_ENDPOINT}" --fb-s3-bucket "${FB_S3_BUCKET}" --fb-s3-key "${FB_S3_KEY}" --fb-s3-secret "${FB_S3_SECRET}")
        fi
    fi

    if [[ -n "${EXTRA_ARGS}" ]]; then
        read -ra extra <<< "${EXTRA_ARGS}"
        args+=("${extra[@]}")
    fi

    rc=0
    "${BIN}" "${args[@]}" 2>&1 | tee "${OUT_DIR}/${TIER}-${db}.bench.log" || rc=${PIPESTATUS[0]}
    if [[ ! -s "${result}" ]]; then
        jq -nc --arg db "${db}" --arg tier "${TIER}" --arg run_id "${run_id}" --arg err "bench exited ${rc} without a result" \
            '{db: $db, tier: $tier, run_id: $run_id, stub: true, verdict: {stopped_reason: "setup_failed"}, error: $err}' > "${result}"
        failed+=("${db}")
    fi
    echo "cell ${TIER}-${db} exit=${rc} -> ${result}" >&2
done

summarize
if (( ${#failed[@]} )); then
    echo "cells without a real result: ${failed[*]}" >&2
    exit 1
fi
