#!/usr/bin/env bash
set -Eeuo pipefail

: "${DOCKER_TAG:?Error: DOCKER_TAG environment variable is required.}"
: "${TARGET_BRANCH:?Error: TARGET_BRANCH environment variable is required.}"
: "${SERVICES:?Error: SERVICES environment variable (space-separated list) is required.}"
: "${DOCKER_USERNAME:?Error: DOCKER_USERNAME environment variable is required.}"

# Enable BuildKit for faster builds
export DOCKER_BUILDKIT=1

read -r -a SERVICES_ARRAY <<< "$SERVICES"
if [[ ${#SERVICES_ARRAY[@]} -eq 0 ]]; then
    echo "Error: Failed to parse services list from SERVICES environment variable." >&2
    exit 1
fi

echo "Building ${#SERVICES_ARRAY[@]} services with optimizations..."

# Start builds in parallel with limited concurrency
build_service() {
  local srv="$1"

  local dockerfile
  local context

  if [[ "$srv" == "db-migrations" ]]; then
    dockerfile="services/db/migrations/Dockerfile.prod"
    context="services/db/migrations"
  else
    dockerfile="services/${srv}/Dockerfile.prod"
    context="services/$srv"
  fi
  echo "Building $srv from $dockerfile..."
  


  # Prepare build args
  build_args="--progress=plain --build-arg BUILDKIT_INLINE_CACHE=1 --cache-from $DOCKER_USERNAME/$srv:latest"
  
  # Add environment-specific build args for frontend
  if [[ "$srv" == "frontend" ]]; then
    build_args="$build_args --build-arg VITE_ENVIRONMENT=${ENVIRONMENT:-development}"
    echo "Building frontend with VITE_ENVIRONMENT=${ENVIRONMENT:-development}"
  fi
  
  # Use BuildKit with caching and optimizations
  docker build \
    $build_args \
    -t "$DOCKER_USERNAME/$srv:${DOCKER_TAG}" \
    -f "$dockerfile" \
    "services/$srv"
  
  echo "✅ $srv build completed"
}

# Build services in parallel (max 3 concurrent builds to avoid resource exhaustion)
pids=()
MAX_CONCURRENT=3

for srv in "${SERVICES_ARRAY[@]}"; do
  # Wait if we've hit the concurrent limit
  if [[ ${#pids[@]} -ge $MAX_CONCURRENT ]]; then
    wait -n  # Wait for any one job to complete
    pids=($(jobs -pr))  # Update active PIDs
  fi
  
  build_service "$srv" &
  pids+=($!)
done

build_service "db-migrations"

# Wait for all remaining builds to complete
wait

echo "🎉 All images built successfully with optimizations!"
