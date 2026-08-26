#!/usr/bin/env bash
# Bring up the Traceway backend on a freshly-provisioned Hetzner box.
#
# 1. Wait for SSH.
# 2. Install Docker if missing.
# 3. rsync the repo to /opt/traceway (sans heavy junk).
# 4. `docker compose -f benchmarks/compose/docker-compose.<mode>.yml up -d --build`
# 5. Poll /health until 200.
#
# Usage: sut-bootstrap.sh <sut-public-ip> <mode>
#   <mode>  sqlite | duckdb | pgch | pgfb | managed-ch
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=_ssh.sh
source "${SCRIPT_DIR}/_ssh.sh"

if [[ $# -lt 2 ]]; then
    echo "usage: $0 <sut-public-ip> <mode>" >&2
    exit 2
fi
SUT_IP="$1"
MODE="$2"

case "${MODE}" in
    sqlite|duckdb|pgch|pgfb|managed-ch) ;;
    *) echo "mode must be sqlite, duckdb, pgch, pgfb, or managed-ch, got: ${MODE}" >&2; exit 2 ;;
esac

if [[ "${MODE}" == "managed-ch" ]]; then
    : "${CLICKHOUSE_SERVER:?CLICKHOUSE_SERVER required for managed-ch mode}"
    : "${CLICKHOUSE_USERNAME:?CLICKHOUSE_USERNAME required for managed-ch mode}"
    : "${CLICKHOUSE_PASSWORD:?CLICKHOUSE_PASSWORD required for managed-ch mode}"
fi

echo "waiting for ssh on ${SUT_IP}" >&2
wait_for_ssh "${SUT_IP}"

echo "installing docker on ${SUT_IP}" >&2
bench_ssh "${SUT_IP}" 'bash -s' <<'REMOTE'
set -euo pipefail
if ! command -v docker >/dev/null 2>&1; then
    curl -fsSL https://get.docker.com | sh
fi
# compose plugin lives at /usr/libexec/docker/cli-plugins after get.docker.com.
docker compose version >/dev/null || { echo "docker compose plugin missing" >&2; exit 1; }
mkdir -p /opt/traceway
REMOTE

echo "rsyncing source to ${SUT_IP}:/opt/traceway" >&2
bench_rsync \
    --exclude '.git' \
    --exclude 'node_modules' \
    --exclude 'frontend/build' \
    --exclude 'frontend/.svelte-kit' \
    --exclude 'benchmarks/loadgen/loadgen' \
    --exclude 'benchmarks/results' \
    --exclude 'traceway.db*' \
    --exclude 'traceway_telemetry.db*' \
    --exclude 'traceway_telemetry.duckdb*' \
    --exclude 'backend/storage' \
    "${REPO_ROOT}/" "root@${SUT_IP}:/opt/traceway/"

compose_args=( -f "benchmarks/compose/docker-compose.${MODE}.yml" )

if [[ "${MODE}" == "managed-ch" ]]; then
    # Wipe the managed CH database so this entry starts on an empty cluster.
    # The script runs locally (orchestrator-side); the SUT itself never needs
    # the curl + CH credentials.
    "${SCRIPT_DIR}/reset-managed-ch.sh"

    # Ship CH credentials to the SUT via a tempfile + --env-file. Inline env
    # via ssh would have to survive two layers of shell quoting; the file is
    # simpler and safe for arbitrary password characters.
    env_tmp="$(mktemp)"
    trap 'rm -f "${env_tmp}"' EXIT
    cat > "${env_tmp}" <<EOF
CLICKHOUSE_SERVER=${CLICKHOUSE_SERVER}
CLICKHOUSE_USERNAME=${CLICKHOUSE_USERNAME}
CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD}
CLICKHOUSE_DATABASE=${CLICKHOUSE_DATABASE:-traceway}
CLICKHOUSE_TLS=${CLICKHOUSE_TLS:-true}
EOF
    bench_scp "${env_tmp}" "root@${SUT_IP}:/opt/traceway/benchmarks/compose/managed-ch.env"
    compose_args=( --env-file benchmarks/compose/managed-ch.env "${compose_args[@]}" )
fi

# Firebolt engine memory: ~60% of the tier's RAM so Postgres and the
# backend keep headroom. Exact-percentile index-state merges at deep fills
# scale with the scanned range and OOM a 4g engine around 100M rows.
if [[ "${MODE}" == "pgfb" && -z "${FIREBOLT_MEM_LIMIT:-}" ]]; then
    case "${TIER:-}" in
        ccx13) FIREBOLT_MEM_LIMIT="4g" ;;
        ccx23) FIREBOLT_MEM_LIMIT="10g" ;;
        ccx33) FIREBOLT_MEM_LIMIT="20g" ;;
        ccx43) FIREBOLT_MEM_LIMIT="40g" ;;
        *)     FIREBOLT_MEM_LIMIT="4g" ;;
    esac
fi
export FIREBOLT_MEM_LIMIT="${FIREBOLT_MEM_LIMIT:-4g}"

CH_ASYNC_INSERT_VAL="${CH_ASYNC_INSERT:-0}"
echo "bringing up compose stack (mode=${MODE}, CH_ASYNC_INSERT=${CH_ASYNC_INSERT_VAL}) on ${SUT_IP}" >&2
# Detached: the build has multi-minute silent phases during which flaky
# SUT hosts have dropped the ssh channel (exit 255), killing the build with
# it. nohup survives the disconnect; the /health poll below (plain curl,
# no ssh) is the completion signal, and the timeout path dumps the log.
bench_ssh "${SUT_IP}" "cd /opt/traceway && nohup env BENCH_PORT=80 CH_ASYNC_INSERT=${CH_ASYNC_INSERT_VAL} FIREBOLT_MEM_LIMIT=${FIREBOLT_MEM_LIMIT} docker compose ${compose_args[*]} up -d --build > /tmp/compose-up.log 2>&1 & echo build-started"

echo "polling /health on ${SUT_IP}" >&2
deadline=$(( $(date +%s) + 900 ))   # cold compose build can hit 10+ min on small tiers
while [[ $(date +%s) -lt ${deadline} ]]; do
    if curl -sf --max-time 5 "http://${SUT_IP}/health" >/dev/null 2>&1; then
        echo "SUT ${SUT_IP} healthy (mode=${MODE})" >&2
        exit 0
    fi
    sleep 5
done
echo "SUT ${SUT_IP} never reported healthy" >&2
bench_ssh "${SUT_IP}" "tail -40 /tmp/compose-up.log; cd /opt/traceway && docker compose -f benchmarks/compose/docker-compose.${MODE}.yml ps -a; docker compose -f benchmarks/compose/docker-compose.${MODE}.yml logs --tail=80" >&2 || true
exit 1
