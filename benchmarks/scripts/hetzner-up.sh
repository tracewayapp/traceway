#!/usr/bin/env bash
# Provision a system-under-test (SUT) box + a load generator box + a private
# network in Hetzner Cloud. Idempotent on its own outputs: re-running with the
# same RUN_ID short-circuits if the resources already exist.
#
# Usage: hetzner-up.sh <tier> <run-id> [location]
#   <tier>      ccx13 | ccx23 | ccx33 | ccx43
#   <run-id>    short label, used to name the resources (e.g. 20260512-ccx13-sqlite-a1b2)
#   [location]  Hetzner datacenter, default nbg1
#
# Emits a JSON line on stdout:
#   {"sutPublicIp":"...","sutPrivateIp":"...","loadgenPublicIp":"...","loadgenPrivateIp":"...","networkId":"...","runId":"..."}
set -euo pipefail

if [[ $# -lt 2 ]]; then
    echo "usage: $0 <tier> <run-id> [location]" >&2
    exit 2
fi

TIER="$1"
RUN_ID="$2"
LOCATION="${3:-nbg1}"

# Trim leading/trailing whitespace from positional args. CI inputs and shell
# variables occasionally arrive with a trailing space (e.g. "hil ") which then
# fails downstream API calls with cryptic "datacenter not found" errors.
TIER="$(printf '%s' "${TIER}" | xargs)"
RUN_ID="$(printf '%s' "${RUN_ID}" | xargs)"
LOCATION="$(printf '%s' "${LOCATION}" | xargs)"
IMAGE="${BENCH_IMAGE:-debian-12}"

# Loadgen tier: pick the cheapest shared-vCPU instance available in the
# target location. CAX (ARM) is EU-only; CPX (x86) is global. Override with
# LOADGEN_TIER if you want a beefier loadgen box.
case "${LOCATION}" in
    nbg1|fsn1|hel1) LOADGEN_DEFAULT="cax11" ;;   # ARM, cheaper, EU only
    ash|hil)        LOADGEN_DEFAULT="cpx11" ;;   # x86, available in US
    sin)            LOADGEN_DEFAULT="cpx11" ;;   # x86, safest in Singapore
    *)              LOADGEN_DEFAULT="cpx11" ;;   # x86 is the safe global default
esac
LOADGEN_TIER="${LOADGEN_TIER:-${LOADGEN_DEFAULT}}"
NET_NAME="bench-net-${RUN_ID}"
SUT_NAME="bench-sut-${RUN_ID}"
LOADGEN_NAME="bench-loadgen-${RUN_ID}"

# Hetzner private networks live in a "network zone" that spans several
# physical locations. A subnet in one zone cannot host servers from another,
# so the zone must match the LOCATION the servers will be created in.
# Reference: https://docs.hetzner.cloud/#network-zones
case "${LOCATION}" in
    nbg1|fsn1|hel1)        NETWORK_ZONE="eu-central" ;;
    ash)                   NETWORK_ZONE="us-east" ;;
    hil)                   NETWORK_ZONE="us-west" ;;
    sin)                   NETWORK_ZONE="ap-southeast" ;;
    *)
        echo "unknown location '${LOCATION}', defaulting network-zone to eu-central — fix me if this is wrong" >&2
        NETWORK_ZONE="eu-central"
        ;;
esac

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

# Create a /24 private network so SUT and loadgen can talk over 10.0.0.x.
if ! hcloud network describe "${NET_NAME}" >/dev/null 2>&1; then
    echo "creating network ${NET_NAME} in zone ${NETWORK_ZONE}" >&2
    retry_eof hcloud network create --name "${NET_NAME}" --ip-range 10.0.0.0/24 >/dev/null
    retry_eof hcloud network add-subnet "${NET_NAME}" --network-zone "${NETWORK_ZONE}" --type cloud --ip-range 10.0.0.0/24 >/dev/null
fi
NET_ID=$(retry_eof hcloud network describe "${NET_NAME}" -o format='{{.ID}}')

create_server() {
    local name="$1" type="$2" private_ip="$3"
    if hcloud server describe "${name}" >/dev/null 2>&1; then
        echo "server ${name} already exists, reusing" >&2
        return
    fi
    echo "creating server ${name} (${type}) in ${LOCATION}" >&2
    retry_eof hcloud server create \
        --name "${name}" \
        --type "${type}" \
        --image "${IMAGE}" \
        --location "${LOCATION}" \
        --ssh-key benchmark-key \
        --network "${NET_NAME}" \
        --label "bench=true,run=${RUN_ID}" \
        >/dev/null
}

create_server "${SUT_NAME}" "${TIER}" "10.0.0.2"
create_server "${LOADGEN_NAME}" "${LOADGEN_TIER}" "10.0.0.3"

# Hetzner doesn't let you pin the private IP at create time on shared networks
# without --network-extra; we re-read both servers' assigned private IPs and
# return them.
get_field() {
    local name="$1" field="$2"
    retry_eof hcloud server describe "${name}" -o format="{{${field}}}"
}

SUT_PUBLIC_IP=$(get_field "${SUT_NAME}" ".PublicNet.IPv4.IP")
LOADGEN_PUBLIC_IP=$(get_field "${LOADGEN_NAME}" ".PublicNet.IPv4.IP")
SUT_PRIVATE_IP=$(retry_eof hcloud server describe "${SUT_NAME}" -o json | jq -r '.private_net[0].ip')
LOADGEN_PRIVATE_IP=$(retry_eof hcloud server describe "${LOADGEN_NAME}" -o json | jq -r '.private_net[0].ip')

jq -nc \
    --arg sutPub "${SUT_PUBLIC_IP}" \
    --arg sutPriv "${SUT_PRIVATE_IP}" \
    --arg lgPub "${LOADGEN_PUBLIC_IP}" \
    --arg lgPriv "${LOADGEN_PRIVATE_IP}" \
    --arg netId "${NET_ID}" \
    --arg runId "${RUN_ID}" \
    '{sutPublicIp: $sutPub, sutPrivateIp: $sutPriv, loadgenPublicIp: $lgPub, loadgenPrivateIp: $lgPriv, networkId: $netId, runId: $runId}'
