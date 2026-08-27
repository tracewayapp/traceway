#!/usr/bin/env bash
# Fail before spending money: tooling, Hetzner credentials, the benchmark-key
# SSH key, the db name and a runnable bench artifact.
#
# Usage: preflight.sh <db>
# Env:   HCLOUD_TOKEN, BENCHMARK_SSH_KEY (path to the private key), ARTIFACT_DIR
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MDB_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ARTIFACT_DIR="${ARTIFACT_DIR:-${MDB_ROOT}/_artifact}"

fail=0
warn() { printf 'WARN: %s\n' "$*" >&2; }
err()  { printf 'FAIL: %s\n' "$*" >&2; fail=1; }

DB="${1:-}"
case "${DB}" in
    victoriametrics|clickhouse|clickhouse-map|duckdb|firebolt|firebolt-s3) ;;
    "") err "usage: preflight.sh <db>" ;;
    *) err "unknown db '${DB}' (expected victoriametrics|clickhouse|clickhouse-map|duckdb|firebolt|firebolt-s3)" ;;
esac

for cmd in hcloud jq ssh scp rsync curl; do
    if ! command -v "${cmd}" >/dev/null 2>&1; then
        err "missing command: ${cmd}"
    fi
done

if [[ -z "${HCLOUD_TOKEN:-}" ]]; then
    err "HCLOUD_TOKEN not set. Export your Hetzner Cloud API token."
elif command -v hcloud >/dev/null 2>&1 && ! hcloud server-type list >/dev/null 2>&1; then
    err "HCLOUD_TOKEN is set but 'hcloud server-type list' failed: token invalid or revoked?"
fi

if [[ -z "${BENCHMARK_SSH_KEY:-}" ]]; then
    warn "BENCHMARK_SSH_KEY not set; skipping key-on-disk checks. The Hetzner-side key 'benchmark-key' is still required."
elif [[ ! -f "${BENCHMARK_SSH_KEY}" ]]; then
    err "BENCHMARK_SSH_KEY=${BENCHMARK_SSH_KEY} does not exist"
elif [[ "$(stat -f '%Lp' "${BENCHMARK_SSH_KEY}" 2>/dev/null || stat -c '%a' "${BENCHMARK_SSH_KEY}" 2>/dev/null)" != "600" ]]; then
    warn "BENCHMARK_SSH_KEY permissions are not 600; ssh may refuse to use it"
fi

if command -v hcloud >/dev/null 2>&1 && [[ -n "${HCLOUD_TOKEN:-}" ]] && ! hcloud ssh-key describe benchmark-key >/dev/null 2>&1; then
    err "no SSH key named 'benchmark-key' in your Hetzner project. Upload it via: hcloud ssh-key create --name benchmark-key --public-key-from-file ~/.ssh/hetzner_benchmark.pub"
fi

# A zipped artifact drops the exec bit; restore it here rather than on the box.
for f in "${ARTIFACT_DIR}/metricsdb-bench" "${ARTIFACT_DIR}/libduckdb.so" "${ARTIFACT_DIR}/remote-status.sh" "${ARTIFACT_DIR}/remote-collect.sh"; do
    if [[ ! -f "${f}" ]]; then
        err "artifact file missing: ${f} (build it with the workflow's build job or run-local.sh --hetzner on linux-amd64)"
    fi
done
if [[ ! -d "${ARTIFACT_DIR}/db" ]]; then
    err "artifact is missing db/ (the per-DB up.sh scripts)"
fi
if [[ -f "${ARTIFACT_DIR}/metricsdb-bench" ]]; then
    chmod +x "${ARTIFACT_DIR}/metricsdb-bench" 2>/dev/null || true
    if [[ "$(uname -s)" == "Linux" && "$(uname -m)" == "x86_64" ]]; then
        if ! version="$(LD_LIBRARY_PATH="${ARTIFACT_DIR}" "${ARTIFACT_DIR}/metricsdb-bench" --version 2>&1)"; then
            err "artifact does not run here: ${version}"
        else
            echo "artifact: ${version}" >&2
        fi
    else
        warn "not linux-amd64, skipping the artifact --version check"
    fi
fi

if [[ "${fail}" -ne 0 ]]; then
    echo >&2
    echo "preflight failed. fix the issues above before running." >&2
    exit 1
fi
echo "preflight: OK (${DB})" >&2
