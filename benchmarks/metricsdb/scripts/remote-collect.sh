#!/usr/bin/env bash
# Post-mortem on the box before teardown. Writes into /opt/bench/out:
# db.log, dmesg.txt, df.txt and postmortem.json (container state, OOM
# evidence, cgroup peaks, disk). Never fails: a dead container is data too.
set -uo pipefail

cd /opt/bench/out || exit 1
if [[ -f db.env ]]; then
    # shellcheck disable=SC1091
    source db.env
fi
CONTAINER="${CONTAINER:-}"
CGROUP="${CGROUP:-}"
DATA_DIR="${DATA_DIR:-/data}"

bench_exit="$(cat bench.done 2>/dev/null || true)"

container_json="null"
: > db.log
if [[ -n "${CONTAINER}" ]] && docker inspect "${CONTAINER}" >/dev/null 2>&1; then
    docker logs "${CONTAINER}" > db.log 2>&1 || true
    container_json="$(docker inspect --format \
        '{"status":"{{.State.Status}}","oomKilled":{{.State.OOMKilled}},"exitCode":{{.State.ExitCode}},"restartCount":{{.RestartCount}},"startedAt":"{{.State.StartedAt}}","finishedAt":"{{.State.FinishedAt}}"}' \
        "${CONTAINER}" 2>/dev/null || echo null)"
fi

dmesg -T > dmesg.txt 2>/dev/null || dmesg > dmesg.txt 2>/dev/null || : > dmesg.txt
oom_count="$(grep -ciE 'out of memory|oom-kill|killed process' dmesg.txt || true)"

df -h / /data > df.txt 2>/dev/null || true
disk_used_pct="$(df --output=pcent /data 2>/dev/null | tail -1 | tr -dc '0-9')"
disk_avail="$(df --output=avail -B1 /data 2>/dev/null | tail -1 | tr -dc '0-9')"
data_bytes="$(du -sb "${DATA_DIR}" 2>/dev/null | cut -f1)"

cgroup_json() {
    local cg="$1" peak events
    if [[ -z "${cg}" || ! -d "${cg}" ]]; then
        echo null
        return
    fi
    peak="$(cat "${cg}/memory.peak" 2>/dev/null || true)"
    events="$(tr '\n' ' ' < "${cg}/memory.events" 2>/dev/null || true)"
    jq -nc --arg path "${cg}" --arg peak "${peak}" --arg events "${events}" \
        '{path: $path, memoryPeakBytes: ($peak | tonumber? // null), memoryEvents: $events}'
}

jq -nc \
    --argjson container "${container_json}" \
    --argjson dbCgroup "$(cgroup_json "${CGROUP}")" \
    --argjson benchCgroup "$(cgroup_json /sys/fs/cgroup/metricsdb-bench)" \
    --arg benchExit "${bench_exit}" \
    --arg oom "${oom_count:-0}" \
    --arg diskUsed "${disk_used_pct:-}" \
    --arg diskAvail "${disk_avail:-}" \
    --arg dataBytes "${data_bytes:-}" \
    --arg dataDir "${DATA_DIR}" \
    --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    '{
        container: $container,
        dbCgroup: $dbCgroup,
        benchCgroup: $benchCgroup,
        benchExitCode: ($benchExit | tonumber? // null),
        dmesgOomCount: ($oom | tonumber? // 0),
        diskUsedPercent: ($diskUsed | tonumber? // null),
        diskAvailBytes: ($diskAvail | tonumber? // null),
        dataDir: $dataDir,
        dataDirBytes: ($dataBytes | tonumber? // null),
        collectedAt: $at
    }' > postmortem.json

echo "postmortem: $(cat postmortem.json)"
