#!/usr/bin/env bash
# Loopback S3 for --fb-stage s3, the fallback if upload:// does not work on
# Firebolt Core. Pinned to the bench cores so it never competes with the DB.
# Prints FB_S3_* assignments on stdout for the caller to eval.
set -euo pipefail

VARIANT="${1:-minio}"
# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"

IMAGE="minio/minio:RELEASE.2025-09-07T16-13-09Z"
PORT=9000
BUCKET="bench"
ACCESS_KEY="bench"
SECRET_KEY="benchbench"

set_docker_common "${VARIANT}" "${BENCH_CPUS}" 2048
reset_variant
pull_image "${IMAGE}"
docker_publish "${PORT}"

docker run -d "${docker_common[@]}" \
    -e MINIO_ROOT_USER="${ACCESS_KEY}" \
    -e MINIO_ROOT_PASSWORD="${SECRET_KEY}" \
    -v "${DATA_DIR}:/data" \
    "${IMAGE}" server /data --address "${LISTEN_HOST}:${PORT}" --console-address "${LISTEN_HOST}:$((PORT + 1))" >/dev/null

if ! wait_http 60 '' "http://127.0.0.1:${PORT}/minio/health/live"; then
    docker logs --tail 50 "${VARIANT}" >&2 || true
    die "${VARIANT} never became healthy"
fi

# curl's built-in SigV4 signing avoids shipping an mc image just for one PUT.
curl -sf -X PUT --aws-sigv4 "aws:amz:us-east-1:s3" --user "${ACCESS_KEY}:${SECRET_KEY}" \
    "http://127.0.0.1:${PORT}/${BUCKET}" >/dev/null || die "could not create bucket ${BUCKET}"

echo "${VARIANT} healthy: $(image_ref "${IMAGE}") bucket=${BUCKET} data=${DATA_DIR}" >&2
printf 'FB_S3_ENDPOINT=%q\nFB_S3_BUCKET=%q\nFB_S3_KEY=%q\nFB_S3_SECRET=%q\n' \
    "http://127.0.0.1:${PORT}" "${BUCKET}" "${ACCESS_KEY}" "${SECRET_KEY}"
