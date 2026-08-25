#!/usr/bin/env bash
# Runs once on a fresh Hetzner box via `bash -s`: docker, jq, firewall,
# sysctls, directories. Prints kernel, cores, memory and disk for the log.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

# Unattended upgrades would otherwise grab the apt lock or restart docker
# mid-run.
systemctl disable --now unattended-upgrades apt-daily.timer apt-daily-upgrade.timer >/dev/null 2>&1 || true
for _ in $(seq 1 60); do
    if ! fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; then
        break
    fi
    sleep 5
done

apt-get update -qq
apt-get install -y -qq jq curl rsync ca-certificates >/dev/null
if ! command -v docker >/dev/null 2>&1; then
    curl -fsSL https://get.docker.com | sh >/dev/null
fi

ufw default deny incoming >/dev/null
ufw default allow outgoing >/dev/null
ufw allow 22/tcp >/dev/null
ufw --force enable >/dev/null

sysctl -qw vm.max_map_count=1048576
sysctl -qw fs.aio-max-nr=1048576
sysctl -qw net.core.somaxconn=4096
sysctl -qw vm.swappiness=1

mkdir -p /opt/bench /data

echo "kernel: $(uname -r)  cores: $(nproc)  mem: $(awk '/MemTotal/{print int($2/1024)}' /proc/meminfo) MB  docker: $(docker --version | cut -d' ' -f3 | tr -d ,)"
df -h /data | tail -1
