#!/usr/bin/env bash
set -Eeuo pipefail

: "${DOCKER_TAG:?Error: DOCKER_TAG environment variable is required.}"
: "${TARGET_BRANCH:?Error: TARGET_BRANCH environment variable is required.}"
: "${SERVICES:?Error: SERVICES environment variable (space-separated list) is required.}"
: "${DOCKER_USERNAME:?Error: DOCKER_USERNAME environment variable is required.}"

read -r -a SERVICES_ARRAY <<< "$SERVICES"
if [[ ${#SERVICES_ARRAY[@]} -eq 0 ]]; then
    echo "Error: Failed to parse services list from SERVICES environment variable." >&2
    exit 1
fi

for srv in "${SERVICES_ARRAY[@]}"; do
  dockerfile="services/${srv}/Dockerfile.prod"
  echo "Building $srv from $dockerfile..."
  docker build -t "$DOCKER_USERNAME/$srv:${DOCKER_TAG}" -f "$dockerfile" "services/$srv"
done

echo "All images built successfully."
