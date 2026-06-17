#!/usr/bin/env bash
#
# Brings up the stack the Playwright UI suite runs against:
#   - builds & pushes the controller image (UI embedded + restic)
#   - (re)creates the cryo-ui namespace with RustFS + seeded buckets
#   - deploys the controller in-cluster with BasicAuth enabled
#   - port-forwards it to localhost:8080 and writes ui/.e2e-env
#
# Requires a running cluster (hack/cluster-up.sh first).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-cryo-test}"
REGISTRY_PORT="${REGISTRY_PORT:-5001}"
NAMESPACE="${NAMESPACE:-cryo-ui}"
PORT="${PORT:-8080}"
PF_PID_FILE="/tmp/cryo-ui-pf.pid"
SKIP_CONTROLLER_BUILD="${SKIP_CONTROLLER_BUILD:-false}"

log() { echo -e "\033[1;36m==>\033[0m $*"; }

if ! kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  echo "Cluster '${CLUSTER_NAME}' not found. Run hack/cluster-up.sh first." >&2
  exit 1
fi
kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null

#
# 1. Controller image
#
if [ "${SKIP_CONTROLLER_BUILD}" != "true" ]; then
  ref="localhost:${REGISTRY_PORT}/cryo-controller:test"
  log "Building & pushing ${ref}"
  docker build -f "${ROOT_DIR}/docker/controller.Dockerfile" -t "${ref}" "${ROOT_DIR}"
  docker push "${ref}"
fi

#
# 2. Fresh namespace + RustFS
#
log "Recreating namespace ${NAMESPACE}"
kubectl delete namespace "${NAMESPACE}" --ignore-not-found --wait=true >/dev/null
kubectl create namespace "${NAMESPACE}" >/dev/null

log "Deploying RustFS"
kubectl apply -n "${NAMESPACE}" -f "${ROOT_DIR}/test/e2e/manifests/rustfs.yaml" >/dev/null
kubectl rollout status -n "${NAMESPACE}" deploy/rustfs --timeout=180s

log "Seeding RustFS buckets"
kubectl apply -n "${NAMESPACE}" -f - <<'EOF' >/dev/null
apiVersion: batch/v1
kind: Job
metadata:
  name: rustfs-seed
spec:
  backoffLimit: 3
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: seed
          image: minio/mc:latest
          command: ["/bin/sh", "-c"]
          args:
            - >
              until mc alias set rfs http://rustfs:9000 rustfsadmin rustfsadmin; do echo waiting; sleep 2; done &&
              mc mb --ignore-existing rfs/cryo-repo &&
              mc mb --ignore-existing rfs/test-source &&
              echo 'cryo ui e2e test data' > /tmp/test-file.txt &&
              mc cp /tmp/test-file.txt rfs/test-source/test-file.txt
EOF
kubectl wait -n "${NAMESPACE}" --for=condition=complete job/rustfs-seed --timeout=120s

#
# 3. Controller
#
log "Deploying controller"
kubectl apply -f "${ROOT_DIR}/test/e2e/manifests/controller.yaml" >/dev/null
kubectl rollout status -n "${NAMESPACE}" deploy/cryo --timeout=180s

#
# 4. Admin credentials (created by the controller on first boot)
#
log "Waiting for admin credentials secret"
for _ in $(seq 1 30); do
  if kubectl get secret cryo-admin-credentials -n "${NAMESPACE}" >/dev/null 2>&1; then break; fi
  sleep 2
done
ADMIN_PASSWORD="$(kubectl get secret cryo-admin-credentials -n "${NAMESPACE}" -o jsonpath='{.data.PASSWORD}' | base64 -d)"
if [ -z "${ADMIN_PASSWORD}" ]; then
  echo "Failed to read admin password from secret" >&2
  exit 1
fi

#
# 5. Port-forward
#
if [ -f "${PF_PID_FILE}" ]; then kill "$(cat "${PF_PID_FILE}")" 2>/dev/null || true; fi
log "Port-forwarding svc/cryo -> localhost:${PORT}"
kubectl port-forward -n "${NAMESPACE}" svc/cryo "${PORT}:8080" >/tmp/cryo-ui-pf.log 2>&1 &
echo $! > "${PF_PID_FILE}"

log "Waiting for http://localhost:${PORT}/api/v1/health"
for _ in $(seq 1 30); do
  if curl -fsS "http://localhost:${PORT}/api/v1/health" >/dev/null 2>&1; then break; fi
  sleep 1
done

# Values are single-quoted so the random admin password (which contains $, !, %,
# etc.) survives `source`-ing the file unchanged.
cat > "${ROOT_DIR}/ui/.e2e-env" <<EOF
CRYO_BASE_URL='http://localhost:${PORT}'
CRYO_ADMIN_USERNAME='admin'
CRYO_ADMIN_PASSWORD='${ADMIN_PASSWORD}'
CRYO_NAMESPACE='${NAMESPACE}'
EOF

log "Stack ready."
log "  UI:    http://localhost:${PORT}"
log "  admin: admin / ${ADMIN_PASSWORD}"
log "  env:   ui/.e2e-env  (sourced by 'just test-ui')"
