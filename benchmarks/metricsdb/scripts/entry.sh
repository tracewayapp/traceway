#!/usr/bin/env bash
# Run ONE metricsdb cell end to end: provision a Hetzner box -> prep it ->
# bring up the database -> launch metricsdb-bench -> poll it once a minute,
# pulling the partial result each time -> post-mortem -> tear down. The
# GitHub workflow and run-local.sh --hetzner both call this; it is the single
# source of truth for cell orchestration.
#
# Usage: entry.sh <db>
#   <db>  victoriametrics | clickhouse | clickhouse-map | duckdb | firebolt
#
# Env (all optional):
#   TIER LOCATION TARGET_POINTS SERIES INTERVAL_SECONDS MAX_MINUTES
#   MAX_SETTLE_MINUTES MAX_COLD_MINUTES QUERY_THRESHOLD_MS WRITERS BATCH_SIZE
#   SMOKE=1               20M points / 10k series / 5 min ingest / 1 min settle / 2 min cold
#   FB_STAGE=upload|s3    firebolt staging; s3 also starts MinIO on the box
#   EXTRA_ARGS            extra metricsdb-bench flags, word-split (e.g. "--gen-threads 4")
#   MODE                  ramp-then-fill (default) | saturate | ramp
#   RAMP_RATES, RAMP_STEP_SECONDS, RAMP_IN_SECONDS, RAMP_BISECT   ramp ladder and step shape
#   ARTIFACT_DIR          built bench artifact (binary, libduckdb.so, db/, remote-*.sh)
#   OUT_DIR               where <tier>-<db>.json and logs/ land
#   BENCH_RUN_ID          CI passes <run_id>-<attempt> so the always() step can delete by name
#   BENCH_IMAGE           Hetzner image, ubuntu-24.04 (glibc matches the build runner, kernel 6.8 for Firebolt)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MDB_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${MDB_ROOT}/../.." && pwd)"
# shellcheck source=../../scripts/_ssh.sh
source "${REPO_ROOT}/benchmarks/scripts/_ssh.sh"

DB="$(printf '%s' "${1:-}" | xargs)"
case "${DB}" in
    victoriametrics|clickhouse|clickhouse-map|duckdb|firebolt) ;;
    "") echo "usage: $0 <db>" >&2; exit 2 ;;
    *) echo "unknown db '${DB}' (expected victoriametrics|clickhouse|clickhouse-map|duckdb|firebolt)" >&2; exit 2 ;;
esac
DB_DIR="${DB%-map}"

TIER="$(printf '%s' "${TIER:-ccx33}" | xargs)"
LOCATION="$(printf '%s' "${LOCATION:-nbg1}" | xargs)"
IMAGE="${BENCH_IMAGE:-ubuntu-24.04}"
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
MODE="${MODE:-ramp-then-fill}"
RAMP_RATES="${RAMP_RATES:-250k,500k,1M,2M,4M,8M}"
RAMP_STEP_SECONDS="${RAMP_STEP_SECONDS:-480}"
RAMP_IN_SECONDS="${RAMP_IN_SECONDS:-60}"
RAMP_BISECT="${RAMP_BISECT:-1}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${MDB_ROOT}/_artifact}"
OUT_DIR="${OUT_DIR:-${MDB_ROOT}/results-local}"
POLL_SECONDS="${POLL_SECONDS:-60}"

if [[ "${SMOKE:-0}" == "1" ]]; then
    TARGET_POINTS=150000000
    SERIES=10000
    MAX_MINUTES=5
    MAX_SETTLE_MINUTES=1
    MAX_COLD_MINUTES=2
    RAMP_RATES="500k,1M,2M"
    RAMP_STEP_SECONDS=30
    RAMP_IN_SECONDS=10
    RAMP_BISECT=0
fi

for tool in hcloud jq ssh scp rsync; do
    command -v "${tool}" >/dev/null || { echo "missing required tool: ${tool}" >&2; exit 1; }
done
[[ -n "${HCLOUD_TOKEN:-}" ]] || { echo "HCLOUD_TOKEN is required" >&2; exit 1; }
[[ -f "${ARTIFACT_DIR}/metricsdb-bench" ]] || { echo "no bench artifact at ${ARTIFACT_DIR}/metricsdb-bench" >&2; exit 1; }
chmod +x "${ARTIFACT_DIR}/metricsdb-bench" "${ARTIFACT_DIR}"/*.sh "${ARTIFACT_DIR}"/db/*/*.sh 2>/dev/null || true

RUN_ID="${BENCH_RUN_ID:-$(date -u +%Y%m%d-%H%M%S)-$RANDOM}-${DB}"
SERVER_NAME="bench-mdb-${RUN_ID}"
if (( ${#SERVER_NAME} > 63 )); then
    echo "server name '${SERVER_NAME}' exceeds Hetzner's 63 char limit; shorten BENCH_RUN_ID" >&2
    exit 2
fi

mkdir -p "${OUT_DIR}/logs"
RESULT="${OUT_DIR}/${TIER}-${DB}.json"
LOGS="${OUT_DIR}/logs/${TIER}-${DB}"
rm -f "${RESULT}"
echo "=== metricsdb entry db=${DB} tier=${TIER} location=${LOCATION} points=${TARGET_POINTS} series=${SERIES} max=${MAX_MINUTES}m run_id=${RUN_ID} ===" >&2

write_stub() {
    local reason="$1" msg="$2"
    jq -nc --arg db "${DB}" --arg tier "${TIER}" --arg run_id "${RUN_ID}" --arg reason "${reason}" --arg err "${msg}" \
        '{db: $db, tier: $tier, run_id: $run_id, stub: true, verdict: {stopped_reason: $reason}, error: $err}' > "${RESULT}"
    echo "wrote stub (${reason}): ${msg}" >&2
}

LAUNCHED=0
cleanup() {
    local rc=$?
    echo "--- teardown for ${RUN_ID} (exit=${rc}) ---" >&2
    if [[ ! -s "${RESULT}" ]]; then
        if (( LAUNCHED )); then
            write_stub crashed "bench launched but no result.json came back (entry.sh exit ${rc})"
        else
            write_stub setup_failed "entry.sh exited ${rc} before the bench started"
        fi
    fi
    hcloud server delete "${SERVER_NAME}" >/dev/null 2>&1 || true
    hcloud server list -l "run=${RUN_ID}" -o noheader -o columns=name 2>/dev/null | while read -r leftover; do
        [[ -n "${leftover}" ]] && hcloud server delete "${leftover}" >/dev/null 2>&1 || true
    done
    exit "${rc}"
}
trap cleanup EXIT INT TERM

# 1. Provision.
if ! hcloud server describe "${SERVER_NAME}" >/dev/null 2>&1; then
    echo "creating server ${SERVER_NAME} (${TIER}, ${IMAGE}) in ${LOCATION}" >&2
    retry_eof hcloud server create \
        --name "${SERVER_NAME}" --type "${TIER}" --image "${IMAGE}" \
        --location "${LOCATION}" --ssh-key benchmark-key \
        --label "bench=true,run=${RUN_ID}" >/dev/null
fi
BOX_IP="$(hcloud server ip "${SERVER_NAME}")"
echo "server up at ${BOX_IP}" >&2
wait_for_ssh "${BOX_IP}" 300

# 2. Prep and ship the artifact.
echo "preparing ${BOX_IP}" >&2
bench_ssh "${BOX_IP}" 'bash -s' < "${SCRIPT_DIR}/remote-prep.sh"
bench_rsync "${ARTIFACT_DIR}/" "root@${BOX_IP}:/opt/bench/"
bench_ssh "${BOX_IP}" "chmod +x /opt/bench/metricsdb-bench /opt/bench/*.sh /opt/bench/db/*/*.sh"

# 3. Bring up the database. up.sh prints db.env on stdout, progress on stderr.
echo "starting ${DB} on ${BOX_IP}" >&2
db_env="$(bench_ssh "${BOX_IP}" "cd /opt/bench && RESET=1 OUT_DIR=/opt/bench/out db/${DB_DIR}/up.sh ${DB}")"
eval "${db_env}"
echo "db.env: ${db_env//$'\n'/ }" >&2

fb_args=()
if [[ "${DB}" == "firebolt" ]]; then
    fb_args+=(--fb-stage "${FB_STAGE}")
    if [[ "${FB_STAGE}" == "s3" ]]; then
        s3_env="$(bench_ssh "${BOX_IP}" "cd /opt/bench && db/firebolt/minio-up.sh")"
        eval "${s3_env}"
        fb_args+=(--fb-s3-endpoint "${FB_S3_ENDPOINT}" --fb-s3-bucket "${FB_S3_BUCKET}" --fb-s3-key "${FB_S3_KEY}" --fb-s3-secret "${FB_S3_SECRET}")
    fi
fi

# 4. Launch. The launch script is generated here and shipped as a file so the
# argument list never has to survive a second layer of ssh quoting.
bench_args=(
    --db "${DB}"
    --points "${TARGET_POINTS}"
    --series "${SERIES}"
    --interval "${INTERVAL_SECONDS}s"
    --max-ingest "${MAX_MINUTES}m"
    --max-settle "${MAX_SETTLE_MINUTES}m"
    --max-cold "${MAX_COLD_MINUTES}m"
    --query-slow-ms "${QUERY_THRESHOLD_MS}"
    --mode "${MODE}"
    --ramp-rates "${RAMP_RATES}"
    --step "${RAMP_STEP_SECONDS}s"
    --ramp-in "${RAMP_IN_SECONDS}s"
    --ramp-bisect "${RAMP_BISECT}"
    --data-dir "${DATA_DIR}"
    --cold-hook "db/${DB_DIR}/restart.sh"
    --tier "${TIER}"
    --run-id "${RUN_ID}"
    --out out/result.json
)
if [[ -n "${WRITERS}" ]]; then
    bench_args+=(--writers "${WRITERS}")
fi
if [[ -n "${BATCH_SIZE}" ]]; then
    bench_args+=(--batch-points "${BATCH_SIZE}")
fi
if [[ -n "${CGROUP}" ]]; then
    bench_args+=(--cgroup "${CGROUP}")
fi
if [[ -n "${CONTAINER}" ]]; then
    bench_args+=(--db-container "${CONTAINER}")
fi
if [[ -n "${DB_URL}" ]]; then
    bench_args+=(--url "${DB_URL}")
fi
if [[ "${DB}" == "duckdb" ]]; then
    bench_args+=(--duckdb-path "${DATA_DIR}/points.duckdb" --duckdb-threads "${DB_CORES}" --duckdb-memory-limit "${MEM_LIMIT_MB}MB")
    pin=""
else
    pin="taskset -c ${BENCH_CPUS} "
fi
if (( ${#fb_args[@]} )); then
    bench_args+=("${fb_args[@]}")
fi
if [[ -n "${EXTRA_ARGS:-}" ]]; then
    read -ra extra_args <<< "${EXTRA_ARGS}"
    bench_args+=("${extra_args[@]}")
fi

launch_file="$(mktemp)"
{
    echo '#!/usr/bin/env bash'
    echo 'cd /opt/bench || exit 1'
    echo 'echo $$ > /sys/fs/cgroup/metricsdb-bench/cgroup.procs 2>/dev/null || true'
    printf 'export LD_LIBRARY_PATH=/opt/bench DB_CONTAINER=%q\n' "${CONTAINER}"
    printf '%s./metricsdb-bench' "${pin}"
    printf ' %q' "${bench_args[@]}"
    printf ' > out/bench.log 2>&1 &\n'
    echo 'echo $! > out/bench.pid'
    echo 'wait $!'
    echo 'echo $? > out/bench.done'
} > "${launch_file}"
bench_scp "${launch_file}" "root@${BOX_IP}:/opt/bench/launch.sh"
rm -f "${launch_file}"

echo "launching bench (pin='${pin}', cgroup='${CGROUP}')" >&2
bench_ssh "${BOX_IP}" "cd /opt/bench && mkdir -p out && rm -f out/result.json out/bench.done out/bench.pid \
    && sync && echo 3 > /proc/sys/vm/drop_caches \
    && mkdir -p /sys/fs/cgroup/metricsdb-bench \
    && (nohup bash launch.sh </dev/null >/dev/null 2>&1 &) \
    && sleep 5 && kill -0 \$(cat out/bench.pid)" \
    || { bench_ssh "${BOX_IP}" "tail -n 40 /opt/bench/out/bench.log" >&2 || true; echo "bench died within 5s of launch" >&2; exit 1; }
LAUNCHED=1

# 5. Poll. Every poll pulls the partial result so a dead box or a timed-out
# step still leaves data. The disk guard and the hard deadline stop the bench
# with SIGTERM (it finishes the window and writes its JSON), SIGKILL 180s later.
deadline_min=$(( MAX_MINUTES + MAX_SETTLE_MINUTES + MAX_COLD_MINUTES + 10 ))
deadline=$(( $(date +%s) + deadline_min * 60 ))
term_sent=0
term_at=0
disk_full=0
ssh_failures=0
signal_bench() {
    bench_ssh "${BOX_IP}" "kill -$1 \$(cat /opt/bench/out/bench.pid) 2>/dev/null" || true
}
while :; do
    sleep "${POLL_SECONDS}"
    line="$(bench_ssh "${BOX_IP}" "bash /opt/bench/remote-status.sh" 2>/dev/null || echo "ssh-failed 0 -")"
    state="${line%% *}"
    rest="${line#* }"
    disk="${rest%% *}"
    last="${rest#* }"
    echo "[$(date -u +%H:%M:%S)] ${state} disk=${disk}% ${last}" >&2
    if bench_scp "root@${BOX_IP}:/opt/bench/out/result.json" "${RESULT}.partial" 2>/dev/null; then
        mv "${RESULT}.partial" "${RESULT}"
    fi
    case "${state}" in
        done|gone) break ;;
        ssh-failed)
            ssh_failures=$(( ssh_failures + 1 ))
            if (( ssh_failures >= 5 )); then
                echo "box unreachable for ${ssh_failures} polls, giving up on it" >&2
                break
            fi
            continue
            ;;
        *) ssh_failures=0 ;;
    esac
    now=$(date +%s)
    if (( term_sent == 0 )) && [[ "${disk}" =~ ^[0-9]+$ ]] && (( disk >= 92 )); then
        echo "disk guard: /data at ${disk}%, sending SIGTERM" >&2
        signal_bench TERM
        term_sent=1; term_at=${now}; disk_full=1
    elif (( term_sent == 0 )) && (( now >= deadline )); then
        echo "hard deadline of ${deadline_min}m reached, sending SIGTERM" >&2
        signal_bench TERM
        term_sent=1; term_at=${now}
    elif (( term_sent == 1 )) && (( now - term_at >= 180 )); then
        echo "still running 180s after SIGTERM, sending SIGKILL" >&2
        signal_bench KILL
        term_sent=2
    elif (( term_sent == 2 )); then
        echo "still reported running after SIGKILL, collecting what exists" >&2
        break
    fi
done

# 6. Post-mortem before teardown; after that an OOM kill and a crash loop are
# indistinguishable. Then pull everything.
bench_ssh "${BOX_IP}" "bash /opt/bench/remote-collect.sh" >&2 || echo "remote-collect failed (non-fatal)" >&2
if bench_scp "root@${BOX_IP}:/opt/bench/out/result.json" "${RESULT}.partial" 2>/dev/null; then
    mv "${RESULT}.partial" "${RESULT}"
fi
for f in bench.log db.log postmortem.json dmesg.txt df.txt; do
    bench_scp "root@${BOX_IP}:/opt/bench/out/${f}" "${LOGS}.${f}" 2>/dev/null || true
done
bench_exit="$(bench_ssh "${BOX_IP}" "cat /opt/bench/out/bench.done 2>/dev/null" || true)"
echo "bench exit code: ${bench_exit:-unknown}" >&2

if [[ ! -s "${RESULT}" ]]; then
    tail_msg="$(tail -n 5 "${LOGS}.bench.log" 2>/dev/null | tr '\n' ' ' || true)"
    case "${bench_exit}" in
        2|3) write_stub setup_failed "bench exited ${bench_exit}: ${tail_msg}" ;;
        *)   write_stub crashed "bench exited ${bench_exit:-unknown} without a result: ${tail_msg}" ;;
    esac
    exit 1
fi

if jq -e '.stub != true' "${RESULT}" >/dev/null 2>&1; then
    if jq -e . "${LOGS}.postmortem.json" >/dev/null 2>&1; then
        tmp="$(mktemp)"
        if jq --slurpfile pm "${LOGS}.postmortem.json" '. + {postmortem: $pm[0]}' "${RESULT}" > "${tmp}"; then
            mv "${tmp}" "${RESULT}"
        else
            rm -f "${tmp}"
        fi
    fi
    if (( disk_full )); then
        tmp="$(mktemp)"
        jq '.verdict.stopped_reason = "disk_full"' "${RESULT}" > "${tmp}" && mv "${tmp}" "${RESULT}"
    fi
fi

jq -r '"result: db=\(.db) verdict=\(.verdict.stopped_reason) acked=\(.throughput.acked_points // "n/a") plateau_pps=\(.throughput.plateau_pps // "n/a") bytes_per_point=\(.disk.bytes_per_point // "n/a")"' "${RESULT}" >&2
echo "cell ${TIER}-${DB} complete -> ${RESULT}" >&2
