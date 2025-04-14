#!/usr/bin/env bash
set -Eeuo pipefail

DOCKER_TAG="${1:-}"
TARGET_BRANCH="${2:-}"

# Make sure required variables are set in the environment
: "${DOCKER_USERNAME:?DOCKER_USERNAME is required}"

# The services to build
SERVICES=( "frontend" "backend" "worker" "worker-healthcheck" "tf" "nginx" "db" )

echo "Building Docker images with tag: ${DOCKER_TAG} from branch: ${TARGET_BRANCH}"

build_service() {
  local service="$1"
  local dockerfile="services/${service}/Dockerfile.prod"

  # Special handling for certain services
  if [[ "$service" == "nginx" ]]; then
    dockerfile="services/nginx/Dockerfile"
  elif [[ "$service" == "worker-healthcheck" ]]; then
    dockerfile="services/worker/Dockerfile.healthcheck"
    echo "Building $service from $dockerfile..."
    docker build -t "$DOCKER_USERNAME/$service:${DOCKER_TAG}" -f "$dockerfile" "services/worker"
    return
  fi

  echo "Building $service from $dockerfile..."
  docker build -t "$DOCKER_USERNAME/$service:${DOCKER_TAG}" -f "$dockerfile" "services/$service"
}

# Build in parallel with concurrency=3
pids=()
MAX_CONCURRENT_BUILDS=3

for srv in "${SERVICES[@]}"; do
  # If we already have $MAX_CONCURRENT_BUILDS processes, wait for one to finish
  if [[ ${#pids[@]} -ge $MAX_CONCURRENT_BUILDS ]]; then
    wait -n
    # Clean up finished PID from array (this is optional in bash 5.0+; wait -n will handle it)
  fi

  build_service "$srv" &
  pids+=( $! )
done

# Wait for all to finish
wait

echo "All images built successfully."

# You can optionally tag production or dev images
if [[ "$TARGET_BRANCH" == "prod" ]]; then
  echo "Tagging images as 'latest' for production..."
  for srv in "${SERVICES[@]}"; do
    docker tag "$DOCKER_USERNAME/$srv:${DOCKER_TAG}" "$DOCKER_USERNAME/$srv:latest"
  done
elif [[ "$TARGET_BRANCH" == "dev" ]]; then
  echo "Tagging images as 'development'..."
  for srv in "${SERVICES[@]}"; do
    docker tag "$DOCKER_USERNAME/$srv:${DOCKER_TAG}" "$DOCKER_USERNAME/$srv:development"
  done
fi
