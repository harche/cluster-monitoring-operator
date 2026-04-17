#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

IMAGE_NAME="${IMAGE_NAME:-quay.io/openshift/cmo-lightspeed-skills}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

echo "Building CMO Lightspeed skills image: ${IMAGE_NAME}:${IMAGE_TAG}"
podman build \
  -f "${REPO_DIR}/lightspeed/Containerfile.skills" \
  -t "${IMAGE_NAME}:${IMAGE_TAG}" \
  "${REPO_DIR}/lightspeed/"

echo "Skills image built: ${IMAGE_NAME}:${IMAGE_TAG}"

if [[ "${PUSH:-}" == "true" ]]; then
  echo "Pushing ${IMAGE_NAME}:${IMAGE_TAG}..."
  podman push "${IMAGE_NAME}:${IMAGE_TAG}"
  echo "Pushed."
fi
