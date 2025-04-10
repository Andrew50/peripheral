#!/usr/bin/env bash
set -Eeuo pipefail

DOCKER_TAG="${1:-}"
ENVIRONMENT="${2:-}"

echo "Deploying to Kubernetes with tag: $DOCKER_TAG, environment: $ENVIRONMENT"

# 1. Apply all YAML files in config/<ENVIRONMENT>. This might include Deployments, Services, etc.
if [[ -d "config/prod" ]]; then
  echo "Applying config in config/$ENVIRONMENT..."
  kubectl apply -f "config/$ENVIRONMENT" --recursive --validate=false
else
  echo "No environment directory found at config/prod. Skipping apply."
  exit 1
fi

# 2. Update relevant Deployments with the new image tag
DEPLOYMENTS=( "db" "cache" "tf" "backend" "worker" "frontend" "nginx" )
for dep in "${DEPLOYMENTS[@]}"; do
  if kubectl get deployment "$dep" &>/dev/null; then
    echo "Setting $dep to use image $DOCKER_USERNAME/$dep:$DOCKER_TAG"
    kubectl set image "deployment/$dep" "$dep=$DOCKER_USERNAME/$dep:$DOCKER_TAG" || \
      echo "WARNING: Failed to set image for $dep"
    # Special logic for worker healthcheck container
    if [[ "$dep" == "worker" ]]; then
      CONTAINERS=$(kubectl get deployment/worker -o jsonpath='{.spec.template.spec.containers[*].name}' 2>/dev/null || echo "")
      if [[ $CONTAINERS == *"db-healthcheck"* ]]; then
        kubectl set image deployment/worker db-healthcheck="$DOCKER_USERNAME/worker-healthcheck:$DOCKER_TAG"
      elif [[ $CONTAINERS == *"worker-healthcheck"* ]]; then
        kubectl set image deployment/worker worker-healthcheck="$DOCKER_USERNAME/worker-healthcheck:$DOCKER_TAG"
      fi
    fi
  else
    echo "Note: Deployment $dep not found in cluster; skipping image update."
  fi
done

# 3. Rollout restart all updated deployments to force pulling new images
for dep in "${DEPLOYMENTS[@]}"; do
  if kubectl get deployment "$dep" &>/dev/null; then
    echo "Rolling out deployment/$dep..."
    kubectl rollout restart "deployment/$dep"
  fi
done

# 4. (Optional) Wait for them to become ready
for dep in "${DEPLOYMENTS[@]}"; do
  if kubectl get deployment "$dep" &>/dev/null; then
    echo "Waiting for rollout to finish for $dep..."
    if ! kubectl rollout status "deployment/$dep" --timeout=180s; then
      echo "WARNING: $dep failed to roll out in time."
    fi
  fi
done

echo "Deploy-to-K8s script complete."
