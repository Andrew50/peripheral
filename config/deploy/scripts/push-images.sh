#!/usr/bin/env bash
set -Eeuo pipefail

DOCKER_TAG="${1:-}"
TARGET_BRANCH="${2:-}"
SERVICES="${3:-}"

: "${DOCKER_USERNAME:?DOCKER_USERNAME is required}"
: "${DOCKER_TOKEN:?DOCKER_TOKEN is required}"
: "${SERVICES:?SERVICES is required}"

read -r -a SERVICES <<< "$SERVICES"

echo "Logging into Docker Hub..."
echo "$DOCKER_TOKEN" | docker login -u "$DOCKER_USERNAME" --password-stdin

echo "Pushing Docker images with tag: $DOCKER_TAG"

push_image() {
  local service="$1"
  local tag="$2"
  echo "Pushing $service:$tag"
  docker push "$DOCKER_USERNAME/$service:$tag"
}

pids=()
MAX_CONCURRENT_PUSH=3

# Push the branch-specific tags
for srv in "${SERVICES[@]}"; do
  if [[ ${#pids[@]} -ge $MAX_CONCURRENT_PUSH ]]; then
    wait -n
  fi

  push_image "$srv" "$DOCKER_TAG" &
  pids+=( $! )
done

wait

# Push environment-specific tags
if [[ "$TARGET_BRANCH" == "prod" ]]; then
  echo "Pushing 'latest' tagged images..."
  pids=()
  for srv in "${SERVICES[@]}"; do
    if [[ ${#pids[@]} -ge $MAX_CONCURRENT_PUSH ]]; then
      wait -n
    fi
    push_image "$srv" "latest" &
    pids+=( $! )
  done
elif [[ "$TARGET_BRANCH" == "dev" ]]; then
  echo "Pushing 'development' tagged images..."
  pids=()
  for srv in "${SERVICES[@]}"; do
    if [[ ${#pids[@]} -ge $MAX_CONCURRENT_PUSH ]]; then
      wait -n
    fi
    push_image "$srv" "development" &
    pids+=( $! )
  done
fi

wait

echo "All images pushed successfully!"
