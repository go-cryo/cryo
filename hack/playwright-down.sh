#!/usr/bin/env bash
#
# Tears down the Playwright stack: stops the port-forward and deletes the
# cryo-ui namespace. Leaves the KIND cluster intact (use hack/cluster-down.sh
# to remove that).
set -euo pipefail

NAMESPACE="${NAMESPACE:-cryo-ui}"
PF_PID_FILE="/tmp/cryo-ui-pf.pid"

log() { echo -e "\033[1;36m==>\033[0m $*"; }

if [ -f "${PF_PID_FILE}" ]; then
  log "Stopping port-forward"
  kill "$(cat "${PF_PID_FILE}")" 2>/dev/null || true
  rm -f "${PF_PID_FILE}"
fi

log "Deleting namespace ${NAMESPACE}"
kubectl delete namespace "${NAMESPACE}" --ignore-not-found --wait=false >/dev/null || true

log "Done."
