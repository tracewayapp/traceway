#!/usr/bin/env bash
# Cold-phase hook for firebolt-s3: bounce every Firebolt pod (engines,
# metadata, gateway), drop the page cache, wait until both engines answer
# through the gateway again. A cold query then pays the full disaggregated
# price: metadata reload plus tablet fetches from the object store into an
# empty engine cache. MinIO itself stays up - it is the storage, not the DB.
set -euo pipefail

# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
NS="firebolt"

if [[ ! -f /opt/bench/out/db.env ]]; then
    die "no db.env; up.sh must run first"
fi
# shellcheck disable=SC1091
source /opt/bench/out/db.env

kubectl -n "${NS}" delete pods --all --wait=false >/dev/null
drop_caches
for eng in ingest analytics; do
    wait_http 600 '"data"' -X POST -H "X-Firebolt-Engine: ${eng}" \
        --data-binary 'SELECT 1' "${DB_URL}/?output_format=JSON_Compact" \
        || die "engine ${eng} did not come back after restart"
done
