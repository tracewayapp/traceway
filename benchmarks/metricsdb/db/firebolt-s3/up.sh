#!/usr/bin/env bash
# Firebolt disaggregated: the operator's production shape on one Hetzner box.
#
#   MinIO (docker, host net)          <- object storage for tablet data
#   k3s single-node cluster
#     firebolt-operator (helm)
#     FireboltInstance  (metadata + gateway)
#     FireboltEngine "ingest"    <- all writes land here
#     FireboltEngine "analytics" <- all queries AND all VACUUM run here
#
# Engines share tablets through the object store and the instance metadata
# service; the gateway routes each request to the engine named in its
# X-Firebolt-Engine header. This is the cell that answers whether the
# single-node collapse (merge debt -> 12x disk -> writers stop acking) is
# an artifact of local-disk Core or fundamental: compaction runs on compute
# the ingest path never sees, and "disk" is a bucket that cannot fill.
#
# DATA_DIR (what the bench's du-walk measures) is MinIO's data directory,
# so bytes/point reports true object-store amplification.
#
# Usage: up.sh [variant]   (called by entry.sh as db/firebolt-s3/up.sh firebolt-s3)
set -euo pipefail

VARIANT="${1:-firebolt-s3}"
# shellcheck source=../_common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/_common.sh"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Same engine image as the single-node cell so the comparison is build-for-build.
IMAGE="$(tr -d '[:space:]' < "${HERE}/../firebolt/image.txt")"

MINIO_IMAGE="minio/minio:RELEASE.2025-09-07T16-13-09Z"
MINIO_PORT=9000
BUCKET="firebolt-tablets"
ACCESS_KEY="bench"
SECRET_KEY="benchbench"
NS="firebolt"

if [[ "${OS}" == "Linux" ]]; then
    kernel_major="$(uname -r | cut -d. -f1)"
    (( kernel_major >= 6 )) || die "firebolt engine needs kernel >= 6.1, this box runs $(uname -r)"
fi
if (( MEM_TOTAL_MB < 48000 )); then
    warn "firebolt-s3 wants a ccx43-class box (>= 64 GB): two engines + metadata + MinIO + k3s in ${MEM_TOTAL_MB} MB will be tight"
fi

NODE_IP="$(ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}' | head -1)"
[[ -n "${NODE_IP}" ]] || die "could not determine the node IP"

# --- MinIO (object storage) -------------------------------------------------
docker rm -f "${VARIANT}-minio" >/dev/null 2>&1 || true
if [[ "${RESET}" == "1" ]]; then
    rm -rf "${DATA_DIR}"
fi
mkdir -p "${DATA_DIR}"
pull_image "${MINIO_IMAGE}"
docker run -d --name "${VARIANT}-minio" --restart no --network host \
    -e MINIO_ROOT_USER="${ACCESS_KEY}" \
    -e MINIO_ROOT_PASSWORD="${SECRET_KEY}" \
    -e MINIO_PROMETHEUS_AUTH_TYPE=public \
    -v "${DATA_DIR}:/data" \
    "${MINIO_IMAGE}" server /data --address ":${MINIO_PORT}" --console-address ":$((MINIO_PORT + 1))" >/dev/null
wait_http 60 '' "http://127.0.0.1:${MINIO_PORT}/minio/health/live" || {
    docker logs --tail 50 "${VARIANT}-minio" >&2 || true
    die "minio never became healthy"
}
curl -sf -X PUT --aws-sigv4 "aws:amz:us-east-1:s3" --user "${ACCESS_KEY}:${SECRET_KEY}" \
    "http://127.0.0.1:${MINIO_PORT}/${BUCKET}" >/dev/null || die "could not create bucket ${BUCKET}"
echo "minio up on :${MINIO_PORT}, bucket ${BUCKET}, data ${DATA_DIR}" >&2

# --- k3s + helm -------------------------------------------------------------
if ! command -v k3s >/dev/null 2>&1; then
    echo "installing k3s" >&2
    curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh - >&2
fi
# The engine registers io_uring buffers at startup; containerd's default
# MEMLOCK limit (inherited from the k3s service) kills it with
# "io_uring_register_buffers failed: Cannot allocate memory" - the same
# failure the docker cells solve with --ulimit memlock=-1.
mkdir -p /etc/systemd/system/k3s.service.d
cat > /etc/systemd/system/k3s.service.d/limits.conf <<'LIMITS'
[Service]
LimitMEMLOCK=infinity
LimitNOFILE=1048576
LIMITS
systemctl daemon-reload
systemctl restart k3s
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
# The node object registers a few seconds after the k3s service starts;
# `kubectl wait` on zero resources errors instead of waiting.
node_deadline=$(( $(date +%s) + 180 ))
until kubectl get nodes --no-headers 2>/dev/null | grep -q .; do
    (( $(date +%s) < node_deadline )) || die "k3s node never registered"
    sleep 3
done
kubectl wait --for=condition=Ready node --all --timeout=180s >&2

if ! command -v helm >/dev/null 2>&1; then
    echo "installing helm" >&2
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash >&2
fi

echo "installing firebolt operator" >&2
helm upgrade --install firebolt-crds oci://oci.firebolt.io/firebolt-db/helm-charts/firebolt-operator-crds \
    -n firebolt-operator --create-namespace --wait --timeout 5m >&2
helm upgrade --skip-crds --install firebolt-operator oci://oci.firebolt.io/firebolt-db/helm-charts/firebolt-operator \
    -n firebolt-operator --wait --timeout 5m >&2

# --- Instance + engines -----------------------------------------------------
if [[ "${RESET}" == "1" ]]; then
    kubectl delete namespace "${NS}" --ignore-not-found --wait=true --timeout=180s >&2 || true
fi
kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f - >&2
kubectl -n "${NS}" create secret generic minio-credentials \
    --from-literal=AWS_ACCESS_KEY_ID="${ACCESS_KEY}" \
    --from-literal=AWS_SECRET_ACCESS_KEY="${SECRET_KEY}" \
    --from-literal=AWS_REGION=us-east-1 \
    --dry-run=client -o yaml | kubectl apply -f - >&2

# Engine sizing from the box: ingest gets ~40% of cores and RAM, analytics
# ~25%; k3s, the metadata service, gateway, MinIO and the bench share the
# rest. On ccx43 (16 cores / 64 GB): ingest 6 cpu / 25 Gi, analytics 4 / 16.
ING_CPU=$(( NPROC * 40 / 100 )); (( ING_CPU >= 2 )) || ING_CPU=2
ANA_CPU=$(( NPROC * 25 / 100 )); (( ANA_CPU >= 2 )) || ANA_CPU=2
ING_MEM_MB=$(( MEM_TOTAL_MB * 40 / 100 ))
ANA_MEM_MB=$(( MEM_TOTAL_MB * 25 / 100 ))

storage_block() {
    cat <<EOF
    storage:
      managed_table_storage: s3
      managed_table_bucket_name: ${BUCKET}
      aws:
        endpoint: http://${NODE_IP}:${MINIO_PORT}
        path_style_addressing: true
        verify_ssl: false
EOF
}

kubectl apply -f - >&2 <<EOF
apiVersion: compute.firebolt.io/v1alpha1
kind: FireboltEngineClass
metadata:
  name: bench-compute
  namespace: ${NS}
spec:
  template:
    spec:
      containers:
        - name: engine
          image: ${IMAGE}
          envFrom:
            - secretRef:
                name: minio-credentials
---
apiVersion: compute.firebolt.io/v1alpha1
kind: FireboltInstance
metadata:
  name: firebolt
  namespace: ${NS}
spec:
  metadata: {}
  gateway: {}
---
apiVersion: compute.firebolt.io/v1alpha1
kind: FireboltEngine
metadata:
  name: ingest
  namespace: ${NS}
spec:
  instanceRef: firebolt
  engineClassRef: bench-compute
  replicas: 1
  customEngineConfig:
$(storage_block)
  template:
    spec:
      containers:
        - name: engine
          resources:
            requests:
              cpu: "${ING_CPU}"
              memory: ${ING_MEM_MB}Mi
            limits:
              cpu: "${ING_CPU}"
              memory: ${ING_MEM_MB}Mi
---
apiVersion: compute.firebolt.io/v1alpha1
kind: FireboltEngine
metadata:
  name: analytics
  namespace: ${NS}
spec:
  instanceRef: firebolt
  engineClassRef: bench-compute
  replicas: 1
  customEngineConfig:
$(storage_block)
  template:
    spec:
      containers:
        - name: engine
          resources:
            requests:
              cpu: "${ANA_CPU}"
              memory: ${ANA_MEM_MB}Mi
            limits:
              cpu: "${ANA_CPU}"
              memory: ${ANA_MEM_MB}Mi
EOF

echo "waiting for engine pods" >&2
deadline=$(( $(date +%s) + 600 ))
while (( $(date +%s) < deadline )); do
    not_ready="$(kubectl -n "${NS}" get pods --no-headers 2>/dev/null | awk '$3 != "Running" && $3 != "Completed"' | wc -l)"
    total="$(kubectl -n "${NS}" get pods --no-headers 2>/dev/null | wc -l)"
    if (( total >= 3 )) && (( not_ready == 0 )); then
        break
    fi
    sleep 5
done
kubectl -n "${NS}" get pods >&2

# --- Gateway exposure -------------------------------------------------------
GW_SVC="$(kubectl -n "${NS}" get svc -o name 2>/dev/null | sed 's|service/||' | grep -i gateway | head -1 || true)"
if [[ -z "${GW_SVC}" ]]; then
    GW_SVC="$(kubectl -n "${NS}" get svc -o name 2>/dev/null | sed 's|service/||' | grep -ivE 'engine|metadata|headless' | head -1 || true)"
fi
if [[ -z "${GW_SVC}" ]]; then
    kubectl -n "${NS}" get svc >&2 || true
    die "could not find the gateway service"
fi
# The operator owns the gateway Service and reconciles spec drift away (a
# NodePort patch reverts to ClusterIP within seconds), so expose the gateway
# through our own NodePort service on the same pod selector, fixed port.
NODEPORT=30347
gw_port="$(kubectl -n "${NS}" get svc "${GW_SVC}" -o jsonpath='{.spec.ports[0].port}')"
gw_target="$(kubectl -n "${NS}" get svc "${GW_SVC}" -o jsonpath='{.spec.ports[0].targetPort}')"
kubectl apply -f - >&2 <<SVC
apiVersion: v1
kind: Service
metadata:
  name: gateway-bench
  namespace: ${NS}
spec:
  type: NodePort
  selector:
    firebolt.io/component: gateway
    firebolt.io/instance: firebolt
  ports:
    - name: http
      port: ${gw_port:-80}
      targetPort: ${gw_target:-8080}
      nodePort: ${NODEPORT}
SVC
DB_URL="http://${NODE_IP}:${NODEPORT}"
echo "gateway ${GW_SVC} on ${DB_URL}" >&2

for eng in ingest analytics; do
    wait_http 300 '"data"' -X POST -H "X-Firebolt-Engine: ${eng}" \
        --data-binary 'SELECT 1' "${DB_URL}/?output_format=JSON_Compact" || {
        kubectl -n "${NS}" get pods >&2 || true
        pod="$(kubectl -n "${NS}" get pods -o name | grep "^pod/${eng}-" | head -1)"
        if [[ -n "${pod}" ]]; then
            kubectl -n "${NS}" describe "${pod}" | tail -30 >&2 || true
            echo "--- ${pod} current logs ---" >&2
            kubectl -n "${NS}" logs "${pod#pod/}" --all-containers --tail 60 >&2 || true
            echo "--- ${pod} previous logs (crash loop) ---" >&2
            kubectl -n "${NS}" logs "${pod#pod/}" --all-containers --previous --tail 60 >&2 || true
        fi
        die "engine ${eng} never answered SELECT 1 through the gateway"
    }
    echo "engine ${eng} answering through the gateway" >&2
done

# --- S3 request-rate + engine-RSS sidecar ----------------------------------
mkdir -p "${OUT_DIR}"
nohup bash "${HERE}/s3-stats.sh" "${DATA_DIR}" "${OUT_DIR}/s3-metrics.jsonl" "${NS}" \
    > "${OUT_DIR}/s3-stats.log" 2>&1 &
echo $! > "${OUT_DIR}/s3-stats.pid"

CGROUP=""
CONTAINER=""
IMAGE_REF="$(image_ref "${IMAGE}" 2>/dev/null || echo "${IMAGE}@k8s")"

emit_env
printf 'FB_WRITE_ENGINE=%q\nFB_QUERY_ENGINE=%q\nFB_MAINT_ENGINE=%q\n' ingest analytics analytics
