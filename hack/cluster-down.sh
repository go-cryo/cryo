#!/usr/bin/env bash
#
# Tears down the local KIND test cluster and the local registry.
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-cryo-test}"
REGISTRY_NAME="${REGISTRY_NAME:-kind-registry}"

log() { echo -e "\033[1;36m==>\033[0m $*"; }

log "Deleting KIND cluster '${CLUSTER_NAME}'"
kind delete cluster --name "${CLUSTER_NAME}" || true

if [ "$(docker inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)" = "true" ]; then
  log "Removing local registry '${REGISTRY_NAME}'"
  docker rm -f "${REGISTRY_NAME}" >/dev/null || true
fi

log "Done."
