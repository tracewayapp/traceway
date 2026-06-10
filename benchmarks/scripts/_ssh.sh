#!/usr/bin/env bash
# Shared SSH/rsync invocation helpers. Source this from other scripts:
#   source "$(dirname "$0")/_ssh.sh"
# Requires BENCHMARK_SSH_KEY env var pointing at a private key file.

ssh_opts=(
    -o StrictHostKeyChecking=accept-new
    -o UserKnownHostsFile=/dev/null
    -o LogLevel=ERROR
    -o ServerAliveInterval=30
    -o ConnectTimeout=15
)

if [[ -n "${BENCHMARK_SSH_KEY:-}" ]]; then
    ssh_opts+=(-i "${BENCHMARK_SSH_KEY}")
fi

bench_ssh() {
    local host="$1"; shift
    ssh "${ssh_opts[@]}" "root@${host}" "$@"
}

bench_scp() {
    scp "${ssh_opts[@]}" "$@"
}

bench_rsync() {
    rsync -az --delete \
        -e "ssh ${ssh_opts[*]}" \
        "$@"
}

# Block until ssh-into-box succeeds or timeout expires (default 180s). Hetzner
# boxes take ~15-30s to finish cloud-init; the first ssh attempt usually fails
# even though hcloud reports the server as "running".
wait_for_ssh() {
    local host="$1" timeout="${2:-180}"
    local deadline=$(( $(date +%s) + timeout ))
    while [[ $(date +%s) -lt ${deadline} ]]; do
        if bench_ssh "${host}" -o ConnectTimeout=5 true 2>/dev/null; then
            return 0
        fi
        sleep 5
    done
    echo "ssh to ${host} not ready after ${timeout}s" >&2
    return 1
}

# retry_eof: run an `hcloud` command, capturing stderr. If the command exits
# non-zero AND its stderr contains "EOF", wait 5 seconds and retry — up to 3
# attempts total. Hetzner's API occasionally drops its keep-alive TCP
# connection mid-action-poll, so hcloud surfaces an EOF even though the
# server-side operation actually succeeded. On any retry attempt, an
# "already exists" error is treated as success because it means the original
# (EOF'd) call did go through. All other failure modes propagate immediately.
retry_eof() {
    local attempt=1 max=3 delay=5 errfile rc
    errfile="$(mktemp)"
    while true; do
        "$@" 2>"${errfile}" && rc=0 || rc=$?
        cat "${errfile}" >&2
        if [[ $rc -eq 0 ]]; then
            rm -f "${errfile}"
            return 0
        fi
        if [[ ${attempt} -gt 1 ]] && grep -qiE "already (exists|in use)|name .* not unique" "${errfile}"; then
            echo "  ↳ resource already exists from earlier attempt; treating as success" >&2
            rm -f "${errfile}"
            return 0
        fi
        if grep -q "EOF" "${errfile}" && [[ $attempt -lt $max ]]; then
            echo "  ↳ hcloud hit EOF, sleeping ${delay}s and retrying (attempt $((attempt+1))/${max})" >&2
            sleep $delay
            attempt=$((attempt + 1))
            : > "${errfile}"
            continue
        fi
        rm -f "${errfile}"
        return $rc
    done
}
