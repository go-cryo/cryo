#!/bin/bash

BASE="$(git rev-parse --show-toplevel)"
[[ $? -eq 0 ]] || {
  echo 'Run this script from inside the repository (cannot determine toplevel directory)'
  exit 1
}

#
# .env file
#
if [ ! -f "$BASE/.env" ]; then
  cp "$BASE/.env.template" "$BASE/.env"
  echo "Created .env from .env.template"
fi

#
# KIND Cluster
#
kind create cluster --name backup-cluster

kubectl get nodes

#
# Install snapshot-controller
#
kubectl create namespace snapshot-controller --dry-run=client -o yaml | kubectl apply -f -
helm repo add piraeus https://piraeus.io/helm-charts --force-update
helm repo update
helm install snapshot-controller piraeus/snapshot-controller --namespace snapshot-controller

#
# Install CSI HostPath driver
#

git clone https://github.com/kubernetes-csi/csi-driver-host-path.git $BASE/hack/csi-driver-host-path
cd $BASE/hack/csi-driver-host-path
git checkout v1.17.0

# pick the newest deploy folder shipped in the repo
DEPLOY_DIR="$(ls -d deploy/kubernetes-* | sort | tail -1)"
echo "Using ${DEPLOY_DIR}"
"${DEPLOY_DIR}/deploy.sh"