#!/usr/bin/env bash
set -Eeuo pipefail

# --- Environment Variable Validation ---
: "${DOCKER_TAG:?Error: DOCKER_TAG environment variable is required.}"
: "${TARGET_BRANCH:?Error: TARGET_BRANCH environment variable is required.}"
: "${SERVICES:?Error: SERVICES environment variable (space-separated list) is required.}"
: "${DOCKER_USERNAME:?Error: DOCKER_USERNAME environment variable is required.}"



# Convert the space-separated SERVICES env var into a bash array
# Note: We rename the array to avoid conflict with the env var name
read -r -a SERVICES_ARRAY <<< "$SERVICES"
if [[ ${#SERVICES_ARRAY[@]} -eq 0 ]]; then
    echo "Error: Failed to parse services list from SERVICES environment variable." >&2
    exit 1
fi

MAX_CONCURRENT_BUILDS=3

build_service() {
  local service="$1"
  local dockerfile="services/${service}/Dockerfile.prod"
  echo "Building $service from $dockerfile..."
  docker build -t "$DOCKER_USERNAME/$service:${DOCKER_TAG}" -f "$dockerfile" "services/$service"
}

#concurrent docker image builds for speed
pids=()
MAX_CONCURRENT_BUILDS=3

# Iterate over the SERVICES_ARRAY
for srv in "${SERVICES_ARRAY[@]}"; do
  if [[ ${#pids[@]} -ge $MAX_CONCURRENT_BUILDS ]]; then
    wait -n
  fi
  build_service "$srv" &
  pids+=( $! )
done

wait

echo "All images built successfully."
