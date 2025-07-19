#!/usr/bin/env bash
set -Eeuo pipefail

# --- Environment Variable Validation ---
: "${DOCKER_TAG:?Error: DOCKER_TAG environment variable is required.}"
: "${TARGET_BRANCH:?Error: TARGET_BRANCH environment variable is required.}"
: "${SERVICES:?Error: SERVICES environment variable (space-separated list) is required.}"
: "${DOCKER_USERNAME:?Error: DOCKER_USERNAME environment variable is required.}"
: "${DOCKER_TOKEN:?Error: DOCKER_TOKEN environment variable is required.}"

# Convert the space-separated string of services into a bash array
read -r -a SERVICES_ARRAY <<< "$SERVICES"

echo "Logging into Docker Hub..."
echo "$DOCKER_TOKEN" | docker login -u "$DOCKER_USERNAME" --password-stdin

echo "Pushing Docker images with tag: $DOCKER_TAG"

push_image() {
  local service="$1"
  local tag="$2"
  local full_image_name="$DOCKER_USERNAME/$service:$tag"
  
  # Check if the image exists locally
  if ! docker image inspect "$full_image_name" &>/dev/null; then
    echo "Error: Image $full_image_name does not exist locally."
    echo "Make sure you've built the image before pushing."
    return 1
  fi
  
  echo "Pushing $full_image_name"
  docker push "$full_image_name"
}

pids=()
MAX_CONCURRENT_PUSH=3

# Push the branch-specific tags
for srv in "${SERVICES_ARRAY[@]}"; do
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
  for srv in "${SERVICES_ARRAY[@]}"; do
    if [[ ${#pids[@]} -ge $MAX_CONCURRENT_PUSH ]]; then
      wait -n
    fi
    # Tag the specific DOCKER_TAG as latest before pushing
    docker tag "$DOCKER_USERNAME/$srv:$DOCKER_TAG" "$DOCKER_USERNAME/$srv:latest"
    push_image "$srv" "latest" &
    pids+=( $! )
  done
elif [[ "$TARGET_BRANCH" == "dev" ]]; then
  echo "Pushing 'development' tagged images..."
  pids=()
  for srv in "${SERVICES_ARRAY[@]}"; do
    if [[ ${#pids[@]} -ge $MAX_CONCURRENT_PUSH ]]; then
      wait -n
    fi
    # Tag the specific DOCKER_TAG as development before pushing
    docker tag "$DOCKER_USERNAME/$srv:$DOCKER_TAG" "$DOCKER_USERNAME/$srv:development"
    push_image "$srv" "development" &
    pids+=( $! )
  done
fi

echo "Pushing db-migrations image..."
push_image "db" "$DOCKER_TAG-migrations"


wait

echo "All images pushed successfully!"
