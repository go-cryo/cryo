#!/usr/bin/env bash
set -euo pipefail

REGISTRY="ghcr.io/go-cryo"
TAG="development"
PLATFORMS="linux/amd64,linux/arm64"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "Building development images for ${PLATFORMS}"
echo "Registry: ${REGISTRY}"
echo "Tag: ${TAG}"
echo ""

# Ensure buildx builder exists
if ! docker buildx inspect cryo-builder >/dev/null 2>&1; then
  echo "Creating buildx builder..."
  docker buildx create --name cryo-builder --use
else
  docker buildx use cryo-builder
fi

# echo "==> Building controller"
# docker buildx build \
#   --platform "${PLATFORMS}" \
#   --build-arg VERSION="${TAG}" \
#   -f "${ROOT_DIR}/docker/controller.Dockerfile" \
#   -t "${REGISTRY}/cryo:${TAG}" \
#   --push \
#   "${ROOT_DIR}"

echo "==> Building psql"
docker buildx build \
  --platform "${PLATFORMS}" \
  -f "${ROOT_DIR}/docker/psql.Dockerfile" \
  -t "${REGISTRY}/cryo-psql:${TAG}" \
  --push \
  "${ROOT_DIR}/hack/backup"

echo "==> Building s3"
docker buildx build \
  --platform "${PLATFORMS}" \
  -f "${ROOT_DIR}/docker/s3.Dockerfile" \
  -t "${REGISTRY}/cryo-s3:${TAG}" \
  --push \
  "${ROOT_DIR}/hack/backup"

echo "==> Building pvc"
docker buildx build \
  --platform "${PLATFORMS}" \
  -f "${ROOT_DIR}/docker/pvc.Dockerfile" \
  -t "${REGISTRY}/cryo-pvc:${TAG}" \
  --push \
  "${ROOT_DIR}/hack/backup"

echo ""
echo "All development images pushed to ${REGISTRY} with tag ${TAG}"
