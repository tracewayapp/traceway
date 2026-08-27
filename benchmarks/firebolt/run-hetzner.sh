#!/usr/bin/env bash
# Direct-engine breaking-point suite on one dedicated Hetzner box:
# Firebolt (untuned + tuned) vs ClickHouse at 30M/60M/100M rows, with
# cache-busted probes, probes-under-write, and the 16-worker concurrency
# check. This is the engine-level companion to the end-to-end matrix
# (`run-local.sh --mode pgfb`); Linux gives Firebolt native io_uring, so
# VACUUM/spill behave unlike macOS Docker.
#
# Usage:
#   export HCLOUD_TOKEN=... BENCHMARK_SSH_KEY=~/.ssh/hetzner_benchmark
#   ./run-hetzner.sh                # full suite on ccx33 (~3-4h, ~EUR 1.50)
#   SMOKE=1 ./run-hetzner.sh       # 1M-row pipeline check (~20 min)
#   TIER=ccx43 LOCATION=fsn1 ./run-hetzner.sh
set -euo pipefail
cd "$(dirname "$0")"
SCRIPTS="$(cd ../scripts && pwd)"
source "${SCRIPTS}/_ssh.sh"

: "${HCLOUD_TOKEN:?export HCLOUD_TOKEN}"
: "${BENCHMARK_SSH_KEY:?export BENCHMARK_SSH_KEY (private key whose public half is the Hetzner 'benchmark-key')}"
TIER="${TIER:-ccx33}"
LOCATION="${LOCATION:-nbg1}"
FIREBOLT_IMAGE="${FIREBOLT_IMAGE:-ghcr.io/firebolt-db/engine:dev}"
SMOKE="${SMOKE:-0}"
OUT="../results-firebolt-hetzner"

RUN_ID="fb-$(date +%Y%m%d%H%M%S)"
SERVER_NAME="bench-fbdirect-${RUN_ID}"

cleanup() {
    echo "deleting ${SERVER_NAME}" >&2
    hcloud server delete "${SERVER_NAME}" >/dev/null 2>&1 || true
}
# INT/TERM included: a cancelled workflow kills the step with SIGTERM, which
# does not run an EXIT-only trap — that is how a box leaks.
trap cleanup EXIT INT TERM

# Sweep leaks from previous cancelled runs: names are unique per run and the
# workflow's concurrency group serializes runs, so any pre-existing
# bench-fbdirect-* server is an orphan still billing.
hcloud server list -o noheader -o columns=name 2>/dev/null | grep '^bench-fbdirect-' | while read -r leaked; do
    echo "deleting leaked server ${leaked}" >&2
    hcloud server delete "${leaked}" >/dev/null 2>&1 || true
done

echo "cross-compiling harness for linux/amd64" >&2
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o firebolt-bench-linux .

echo "creating ${SERVER_NAME} (${TIER} in ${LOCATION})" >&2
hcloud server create --name "${SERVER_NAME}" --type "${TIER}" --image debian-12 \
    --location "${LOCATION}" --ssh-key benchmark-key --label bench=true >/dev/null
IP="$(hcloud server ip "${SERVER_NAME}")"
echo "server up at ${IP}" >&2
wait_for_ssh "${IP}"

echo "installing docker + starting engines" >&2
bench_ssh "${IP}" 'bash -s' <<REMOTE
set -euo pipefail
command -v docker >/dev/null 2>&1 || curl -fsSL https://get.docker.com | sh
mkdir -p /root/firebolt-data /root/results
# The engine image runs as uid:gid 3473:3473 (recent dev builds); a
# root-owned bind mount is unwritable for it and the server exits at boot.
chown -R 3473:3473 /root/firebolt-data
cat > /root/firebolt-config.yaml <<'CFG'
schema_version: "1.0"
engine:
  auto_vacuum:
    enabled: true
CFG
docker run -d --name firebolt --restart on-failure:3 -p 3473:3473 \
    --security-opt seccomp=unconfined --ulimit memlock=-1:-1 \
    -v /root/firebolt-data:/var/lib/firebolt \
    -v /root/firebolt-config.yaml:/etc/firebolt/config.yaml:ro \
    ${FIREBOLT_IMAGE} server --data-dir /var/lib/firebolt --server-config /etc/firebolt/config.yaml
docker run -d --name clickhouse --restart on-failure:3 -p 8123:8123 \
    -e CLICKHOUSE_PASSWORD=bench --ulimit nofile=262144:262144 \
    clickhouse/clickhouse-server:24.8-alpine
fb_ready=0
for i in \$(seq 1 100); do
    curl -sf http://localhost:3473/health/ready >/dev/null && { fb_ready=1; break; }; sleep 3
done
if [ "\$fb_ready" != "1" ]; then
    echo "FIREBOLT ENGINE NEVER BECAME READY — container state and logs:" >&2
    docker ps -a >&2
    docker logs --tail 60 firebolt >&2 || true
    exit 1
fi
ch_ready=0
for i in \$(seq 1 60); do
    curl -sf "http://localhost:8123/ping" >/dev/null && { ch_ready=1; break; }; sleep 3
done
if [ "\$ch_ready" != "1" ]; then
    echo "CLICKHOUSE NEVER BECAME READY:" >&2; docker logs --tail 60 clickhouse >&2 || true; exit 1
fi
echo "engines ready"
REMOTE

bench_scp firebolt-bench-linux "root@${IP}:/root/firebolt-bench"

if [[ "${SMOKE}" == "1" ]]; then
    FILL_LOGS="1000000"; FILL_STD="1000000"; STEP=8; RUNS=3
else
    FILL_LOGS="30000000,60000000,100000000"; FILL_STD="30000000,100000000"; STEP=15; RUNS=5
fi

echo "running suite (SMOKE=${SMOKE})" >&2
suite_script="$(mktemp)"
cat > "${suite_script}" <<REMOTE
set -uo pipefail   # deliberately no -e: a failed cell must not kill the suite
cd /root
run() { # label args...
    local label="\$1"; shift
    echo "=== \${label} ==="
    ./firebolt-bench "\$@" --report-out "/root/results/\${label}.json" || echo "=== \${label} FAILED (continuing) ==="
}

# Untuned deep cells are deliberately omitted: the untuned collapse is
# fully documented from the local campaign, and its timeout-bound probes
# at depth burn hours. Tuned is what the evaluation cares about.
run "hetzner-firebolt-spans-fill-concurrency" --signal spans --fb-tuned --workers 16 \
    --batch-sizes 16384 --step-seconds ${STEP} --fill-levels 1 --cache-bust --probe-runs ${RUNS}

# A/B: does the admission controller turn OOM-death into rejection under the
# concurrency-killer workload? Needs Linux (spill/io_uring); swap the config,
# restart, rerun the same cell, restore.
cp /root/firebolt-config.yaml /root/firebolt-config.base.yaml
cat > /root/firebolt-config.yaml <<'CFG2'
schema_version: "1.0"
engine:
  auto_vacuum:
    enabled: true
execution:
  admission_controller:
    enabled: true
CFG2
docker restart firebolt >/dev/null
for i in \$(seq 1 60); do curl -sf http://localhost:3473/health/ready >/dev/null && break; sleep 3; done
run "hetzner-firebolt-spans-concurrency-admission" --signal spans --fb-tuned --workers 16 \
    --batch-sizes 16384 --step-seconds ${STEP} --fill-levels 1 --cache-bust --probe-runs ${RUNS}
cp /root/firebolt-config.base.yaml /root/firebolt-config.yaml
docker restart firebolt >/dev/null
for i in \$(seq 1 60); do curl -sf http://localhost:3473/health/ready >/dev/null && break; sleep 3; done

# Firebolt tuned: aggregating indexes + VACUUM (io_uring works here)
for sig in logs spans metrics; do
    fills="${FILL_STD}"; [ "\${sig}" = "logs" ] && fills="${FILL_LOGS}"
    run "hetzner-firebolt-\${sig}-tuned-deep" --signal "\${sig}" --fb-tuned --step-seconds ${STEP} \
        --probe-runs ${RUNS} --cache-bust --probe-under-write --fill-levels "\${fills}"
done

# ClickHouse baseline: deep + concurrency
for sig in spans metrics logs; do
    fills="${FILL_STD}"; [ "\${sig}" = "logs" ] && fills="${FILL_LOGS}"
    run "hetzner-clickhouse-\${sig}-deep" --dialect clickhouse --target http://localhost:8123 \
        --ch-password bench --signal "\${sig}" --step-seconds ${STEP} --probe-runs ${RUNS} \
        --cache-bust --probe-under-write --fill-levels "\${fills}"
done
run "hetzner-clickhouse-spans-concurrency" --dialect clickhouse --target http://localhost:8123 \
    --ch-password bench --signal spans --reset=false --workers 16 \
    --batch-sizes 16384 --step-seconds ${STEP} --fill-levels 1 --cache-bust --probe-runs ${RUNS}
echo "SUITE DONE"
touch /root/suite_done
REMOTE

# Detached execution + incremental result pulls: a cancelled workflow or a
# dropped ssh session no longer loses completed cells, and the suite log
# streams through short-lived sessions.
bench_scp "${suite_script}" "root@${IP}:/root/suite.sh"
bench_ssh "${IP}" "nohup bash /root/suite.sh > /root/suite.log 2>&1 & echo started"
mkdir -p "${OUT}"
streamed=0
while true; do
    sleep 60
    bench_rsync "root@${IP}:/root/results/" "${OUT}/" 2>/dev/null || true
    if out=$(bench_ssh "${IP}" "tail -n +$((streamed + 1)) /root/suite.log 2>/dev/null | head -100; test -f /root/suite_done && echo __SUITE_DONE__"); then
        body=$(printf '%s\n' "${out}" | grep -v '^__SUITE_DONE__' || true)
        if [[ -n "${body}" ]]; then
            printf '%s\n' "${body}" >&2
            streamed=$((streamed + $(printf '%s\n' "${body}" | wc -l | tr -d ' ')))
        fi
        if printf '%s\n' "${out}" | grep -q '^__SUITE_DONE__'; then
            break
        fi
    fi
done
bench_rsync "root@${IP}:/root/results/" "${OUT}/" 2>/dev/null || true
echo "results in ${OUT}/" >&2
python3 summarize.py "${OUT}" > "${OUT}/summary.md" || true
echo "wrote ${OUT}/summary.md" >&2
