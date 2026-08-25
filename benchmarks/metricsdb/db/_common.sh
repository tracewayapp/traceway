#!/usr/bin/env bash
# Sourced by every db/<name>/up.sh and restart.sh with VARIANT already set.
# Works out the cpuset split, memory cap, data dir and network mode for one
# cell on Linux (the Hetzner box) and macOS (Docker Desktop), and provides
# the docker, health and env helpers the per-DB scripts share.
#
# Overridable env:
#   DB_CPUS, BENCH_CPUS   cpuset ranges; set either to "" to run unpinned
#   MEM_LIMIT_MB          container memory cap (and the DuckDB memory_limit)
#   DATA_ROOT             parent of DATA_DIR
#   OUT_DIR               where db.env is written
#   HOST_NET              1 = --network host, 0 = publish the port on loopback
#   RESET                 1 = wipe DATA_DIR before starting (default)

: "${VARIANT:?VARIANT must be set before sourcing _common.sh}"

OS="$(uname -s)"

die() { echo "$*" >&2; exit 1; }
warn() { echo "WARN: $*" >&2; }

if command -v nproc >/dev/null 2>&1; then
    NPROC="$(nproc)"
else
    NPROC="$(sysctl -n hw.ncpu)"
fi

if [[ -r /proc/meminfo ]]; then
    MEM_TOTAL_MB="$(awk '/MemTotal/{print int($2/1024)}' /proc/meminfo)"
elif command -v sysctl >/dev/null 2>&1; then
    MEM_TOTAL_MB="$(( $(sysctl -n hw.memsize) / 1048576 ))"
else
    die "cannot determine total memory: no /proc/meminfo and no sysctl"
fi

MEM_LIMIT_OVERRIDDEN=0
if [[ -n "${MEM_LIMIT_MB:-}" ]]; then
    MEM_LIMIT_OVERRIDDEN=1
fi

tmp_root="${TMPDIR:-/tmp}"
if [[ "${OS}" == "Darwin" ]]; then
    # Docker Desktop runs containers in a VM with no cgroup or cpuset access
    # from the host and only a slice of the machine's RAM.
    DB_CPUS="${DB_CPUS-}"
    BENCH_CPUS="${BENCH_CPUS-}"
    HOST_NET="${HOST_NET:-0}"
    darwin_cap=$(( MEM_TOTAL_MB - 3072 ))
    if (( darwin_cap > 6144 )); then
        darwin_cap=6144
    fi
    MEM_LIMIT_MB="${MEM_LIMIT_MB:-${darwin_cap}}"
    # Docker Desktop only shares /Users by default, so a bind mount under
    # /tmp or $TMPDIR is invisible from the host and the disk walk sees nothing.
    DATA_ROOT="${DATA_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/results-local/data}"
    OUT_DIR="${OUT_DIR:-./results-local}"
else
    if (( NPROC >= 4 )); then
        DB_CPUS="${DB_CPUS-0-$((NPROC - 3))}"
        BENCH_CPUS="${BENCH_CPUS-$((NPROC - 2))-$((NPROC - 1))}"
    else
        echo "WARN: ${NPROC} cores is too few for a DB/bench cpuset split, running unpinned" >&2
        DB_CPUS="${DB_CPUS-}"
        BENCH_CPUS="${BENCH_CPUS-}"
    fi
    HOST_NET="${HOST_NET:-1}"
    MEM_LIMIT_MB="${MEM_LIMIT_MB:-$(( MEM_TOTAL_MB - 3072 ))}"
    DATA_ROOT="${DATA_ROOT:-/data}"
    OUT_DIR="${OUT_DIR:-/opt/bench/out}"
fi
RESET="${RESET:-1}"
DATA_DIR="${DATA_ROOT}/${VARIANT}"

# Without host networking the server sits behind docker-proxy, so it has to
# listen on all interfaces inside the container and accept the bridge
# gateway as a client.
if [[ "${HOST_NET}" == "1" ]]; then
    LISTEN_HOST="127.0.0.1"
    CLIENT_NET="127.0.0.1"
else
    LISTEN_HOST="0.0.0.0"
    CLIENT_NET="::/0"
fi

CGROUP=""
CONTAINER=""
IMAGE_REF=""
DB_URL=""

cpuset_count() {
    local -a parts
    local part total=0
    IFS=',' read -ra parts <<< "$1"
    for part in "${parts[@]}"; do
        if [[ "${part}" == *-* ]]; then
            total=$(( total + ${part#*-} - ${part%-*} + 1 ))
        else
            total=$(( total + 1 ))
        fi
    done
    echo "${total}"
}

if [[ -n "${DB_CPUS}" ]]; then
    DB_CORES="$(cpuset_count "${DB_CPUS}")"
else
    DB_CORES="${NPROC}"
fi

set_docker_common() {
    local name="$1" cpus="$2" mem_mb="$3"
    docker_common=(--name "${name}" --restart no)
    if [[ "${HOST_NET}" == "1" ]]; then
        docker_common+=(--network host)
    fi
    if [[ -n "${cpus}" ]]; then
        docker_common+=(--cpuset-cpus "${cpus}")
    fi
    docker_common+=(--memory "${mem_mb}m" --memory-swap "${mem_mb}m")
}
set_docker_common "${VARIANT}" "${DB_CPUS}" "${MEM_LIMIT_MB}"

docker_publish() {
    local port
    if [[ "${HOST_NET}" == "1" ]]; then
        return 0
    fi
    for port in "$@"; do
        docker_common+=(-p "127.0.0.1:${port}:${port}")
    done
}

reset_variant() {
    if command -v docker >/dev/null 2>&1; then
        docker rm -f "${VARIANT}" >/dev/null 2>&1 || true
    fi
    if [[ "${RESET}" == "1" ]]; then
        rm -rf "${DATA_DIR}"
    fi
    mkdir -p "${DATA_DIR}"
}

pull_image() {
    docker pull -q "$1" >&2
}

image_ref() {
    local digest
    digest="$(docker image inspect --format '{{if .RepoDigests}}{{index .RepoDigests 0}}{{end}}' "$1" 2>/dev/null || true)"
    if [[ "${digest}" == *@* ]]; then
        printf '%s@%s\n' "$1" "${digest#*@}"
    else
        printf '%s@%s\n' "$1" "$(docker image inspect --format '{{.Id}}' "$1")"
    fi
}

container_cgroup() {
    local id candidate
    id="$(docker inspect --format '{{.Id}}' "$1" 2>/dev/null || true)"
    if [[ -z "${id}" ]]; then
        return 0
    fi
    for candidate in "/sys/fs/cgroup/system.slice/docker-${id}.scope" "/sys/fs/cgroup/docker/${id}"; do
        if [[ -d "${candidate}" ]]; then
            printf '%s\n' "${candidate}"
            return 0
        fi
    done
}

# wait_http <timeout-seconds> <grep -E pattern, empty = any 2xx> <curl args...>
wait_http() {
    local timeout="$1" expect="$2" body deadline
    shift 2
    deadline=$(( $(date +%s) + timeout ))
    while (( $(date +%s) < deadline )); do
        if body="$(curl -sf --max-time 5 "$@" 2>/dev/null)"; then
            if [[ -z "${expect}" ]] || grep -qE -- "${expect}" <<< "${body}"; then
                return 0
            fi
        fi
        sleep 2
    done
    return 1
}

render_template() {
    sed -e "s|@@DB_CORES@@|${DB_CORES}|g" \
        -e "s|@@LISTEN_HOST@@|${LISTEN_HOST}|g" \
        -e "s|@@CLIENT_NET@@|${CLIENT_NET}|g" \
        "$1" > "$2"
}

drop_caches() {
    sync
    if [[ -w /proc/sys/vm/drop_caches ]]; then
        echo 3 > /proc/sys/vm/drop_caches
    fi
}

# Writes db.env and prints it, so a caller over ssh can eval the stdout.
# Everything else the up.sh scripts print goes to stderr.
emit_env() {
    local env_file key
    mkdir -p "${OUT_DIR}"
    OUT_DIR="$(cd "${OUT_DIR}" && pwd)"
    env_file="${OUT_DIR}/db.env"
    : > "${env_file}"
    for key in DATA_DIR CGROUP CONTAINER BENCH_CPUS IMAGE_REF DB_URL DB_CORES MEM_LIMIT_MB; do
        printf '%s=%q\n' "${key}" "${!key}" >> "${env_file}"
    done
    cat "${env_file}"
}
