#!/usr/bin/env bash
#
# Brings up a reproducible local KIND cluster for the full test loop:
#   - local image registry (so backup-job images can be pulled with PullAlways)
#   - snapshot-controller + CSI hostpath driver (for PVC backups)
#   - builds & pushes the restic backup-job images (psql/s3/pvc)
#
# Idempotent: safe to re-run. The Go e2e suite and the Playwright suite both
# run against the cluster this script creates.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-cryo-test}"
REGISTRY_NAME="${REGISTRY_NAME:-kind-registry}"
REGISTRY_PORT="${REGISTRY_PORT:-5001}"
SKIP_CSI="${SKIP_CSI:-false}"
SKIP_IMAGES="${SKIP_IMAGES:-false}"

log() { echo -e "\033[1;36m==>\033[0m $*"; }

#
# 1. Local registry — kind nodes pull localhost:5001/* via the containerd mirror
#    configured in test/e2e/kind-config.yaml.
#
if [ "$(docker inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)" != "true" ]; then
  log "Starting local registry '${REGISTRY_NAME}' on :${REGISTRY_PORT}"
  docker run -d --restart=always -p "127.0.0.1:${REGISTRY_PORT}:5000" --name "${REGISTRY_NAME}" registry:2 >/dev/null
else
  log "Local registry '${REGISTRY_NAME}' already running"
fi

#
# 2. KIND cluster
#
if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  log "KIND cluster '${CLUSTER_NAME}' already exists"
else
  log "Creating KIND cluster '${CLUSTER_NAME}'"
  kind create cluster --name "${CLUSTER_NAME}" --config "${ROOT_DIR}/test/e2e/kind-config.yaml"
fi

# Connect the registry to the kind network so nodes can reach kind-registry:5000.
if [ "$(docker inspect -f '{{json .NetworkSettings.Networks.kind}}' "${REGISTRY_NAME}" 2>/dev/null || echo null)" = "null" ]; then
  log "Connecting registry to 'kind' network"
  docker network connect kind "${REGISTRY_NAME}" || true
fi

# Advertise the registry to the cluster (KEP-1755).
kubectl apply -f - <<'EOF' >/dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:5001"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null

#
# 3. snapshot-controller + CSI hostpath driver (needed for PVC backups)
#
if [ "${SKIP_CSI}" != "true" ]; then
  if ! kubectl get deployment snapshot-controller -n snapshot-controller >/dev/null 2>&1; then
    log "Installing snapshot-controller"
    kubectl create namespace snapshot-controller --dry-run=client -o yaml | kubectl apply -f -
    helm repo add piraeus https://piraeus.io/helm-charts --force-update >/dev/null
    helm repo update >/dev/null
    helm install snapshot-controller piraeus/snapshot-controller --namespace snapshot-controller --wait
  else
    log "snapshot-controller already installed"
  fi

  if ! kubectl get csidriver hostpath.csi.k8s.io >/dev/null 2>&1; then
    log "Deploying CSI hostpath driver"
    DEPLOY_DIR="$(ls -d "${ROOT_DIR}"/hack/csi-driver-host-path/deploy/kubernetes-* | grep -v -- '-test$' | sort -V | tail -1)"
    log "Using ${DEPLOY_DIR}"
    "${DEPLOY_DIR}/deploy.sh"
  else
    log "CSI hostpath driver already installed"
  fi

  # StorageClass + VolumeSnapshotClass used by the PVC backup tests.
  kubectl apply -f "${ROOT_DIR}/hack/storage-class.yml" >/dev/null
else
  log "Skipping CSI install (SKIP_CSI=true)"
fi

#
# 4. Build & push the restic backup-job images for the host architecture.
#    Backup jobs run with ImagePullPolicy: Always, so they must come from the
#    registry (kind load is not enough).
#
if [ "${SKIP_IMAGES}" != "true" ]; then
  for img in psql s3 pvc; do
    ref="localhost:${REGISTRY_PORT}/cryo-${img}:test"
    log "Building & pushing ${ref}"
    docker build -f "${ROOT_DIR}/docker/${img}.Dockerfile" -t "${ref}" "${ROOT_DIR}/hack/backup"
    docker push "${ref}"
  done
else
  log "Skipping backup image build (SKIP_IMAGES=true)"
fi

log "Cluster '${CLUSTER_NAME}' is ready."
log "  kubectl context: kind-${CLUSTER_NAME}"
log "  backup images:   localhost:${REGISTRY_PORT}/cryo-{psql,s3,pvc}:test"
