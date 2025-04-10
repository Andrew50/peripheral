#!/usr/bin/env bash
set -Eeuo pipefail

# Assign arguments to variables
DOCKER_TAG="${1:-}"
<<<<<<< HEAD
DOCKER_USERNAME="${2:-}"
SERVICES="${2:-}" # Services passed as a single space-separated string
=======
TARGET_BRANCH="${2:-}"
TARGET_BRANCH="${2:-}"
SERVICES="${3:-}" # Services passed as a single space-separated string

# --- Argument Validation ---
if [[ -z "$DOCKER_TAG" ]]; then
  echo "Error: DOCKER_TAG (argument 1) is required." >&2
  exit 1
fi

<<<<<<< HEAD
if [[ -z "$DOCKER_USERNAME" ]]; then
  echo "Error: DOCKER_USERNAME (argument 3) is required." >&2
  exit 1
fi

if [[ -z "$SERVICES" ]]; then
  echo "Error: List of services (argument 4, space-separated) is required." >&2
=======
if [[ -z "$TARGET_BRANCH" ]]; then
  echo "Error: TARGET_BRANCH (argument 2) is required." >&2
  exit 1
fi

if [[ -z "$SERVICES" ]]; then
  echo "Error: List of services (argument 3, space-separated) is required." >&2
if [[ -z "$TARGET_BRANCH" ]]; then
  echo "Error: TARGET_BRANCH (argument 2) is required." >&2
  exit 1
fi

if [[ -z "$SERVICES" ]]; then
  echo "Error: List of services (argument 3, space-separated) is required." >&2
  exit 1
fi

# Use DOCKER_USERNAME from environment variable
if [[ -z "${DOCKER_USERNAME:-}" ]]; then
  echo "Error: DOCKER_USERNAME environment variable is required." >&2
  exit 1
fi

# Convert the space-separated string into a bash array and verify
read -r -a SERVICES <<< "$SERVICES"
if [[ ${#SERVICES[@]} -eq 0 ]]; then
    echo "Error: Failed to parse services list from argument 4." >&2
    exit 1
fi

#standard build func
build_service() {
  local service="$1"
  local dockerfile="services/${service}/Dockerfile.prod"
  echo "Building $service from $dockerfile..."
  docker build -t "$DOCKER_USERNAME/$service:${DOCKER_TAG}" -f "$dockerfile" "services/$service"
}

#concurrent docker image builds for speed
pids=()
MAX_CONCURRENT_BUILDS=3

for srv in "${SERVICES[@]}"; do
  if [[ ${#pids[@]} -ge $MAX_CONCURRENT_BUILDS ]]; then
    wait -n
  fi
  build_service "$srv" &
  pids+=( $! )
done

wait

echo "All images built successfully."
