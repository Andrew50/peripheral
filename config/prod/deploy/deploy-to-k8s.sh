#!/usr/bin/env bash
set -Eeuo pipefail

DOCKER_TAG="${1:-}"
ENVIRONMENT="${2:-}"
K8S_NAMESPACE="${3:-}"Use environment variable or default to "default"

echo "Deploying to Kubernetes with tag: $DOCKER_TAG, environment: $ENVIRONMENT, namespace: $K8S_NAMESPACE"

# 1. Apply all YAML files in config/prod. This might include Deployments, Services, etc.
if [[ -d "config/prod" ]]; then
  echo "Applying config in config/prod to namespace ${K8S_NAMESPACE}..."
  kubectl apply -f "config/prod" --recursive --validate=false --namespace="${K8S_NAMESPACE}"
else
  echo "No environment directory found at config/prod. Skipping apply."
  exit 1
fi

# 2. Update relevant Deployments with the new image tag
DEPLOYMENTS=( "db" "cache" "tf" "backend" "worker" "frontend" "nginx" )
for dep in "${DEPLOYMENTS[@]}"; do
  if kubectl get deployment "$dep" --namespace="${K8S_NAMESPACE}" &>/dev/null; then
    echo "Setting $dep to use image $DOCKER_USERNAME/$dep:$DOCKER_TAG in namespace ${K8S_NAMESPACE}"
    kubectl set image "deployment/$dep" "$dep=$DOCKER_USERNAME/$dep:$DOCKER_TAG" --namespace="${K8S_NAMESPACE}" || \
      echo "WARNING: Failed to set image for $dep"
    # Special logic for worker healthcheck container
    if [[ "$dep" == "worker" ]]; then
      CONTAINERS=$(kubectl get deployment/worker --namespace="${K8S_NAMESPACE}" -o jsonpath='{.spec.template.spec.containers[*].name}' 2>/dev/null || echo "")
      if [[ $CONTAINERS == *"db-healthcheck"* ]]; then
        kubectl set image deployment/worker db-healthcheck="$DOCKER_USERNAME/worker-healthcheck:$DOCKER_TAG" --namespace="${K8S_NAMESPACE}"
      elif [[ $CONTAINERS == *"worker-healthcheck"* ]]; then
        kubectl set image deployment/worker worker-healthcheck="$DOCKER_USERNAME/worker-healthcheck:$DOCKER_TAG" --namespace="${K8S_NAMESPACE}"
      fi
    fi
  else
    echo "Note: Deployment $dep not found in namespace ${K8S_NAMESPACE}; skipping image update."
  fi
done

# 3. Rollout restart all updated deployments to force pulling new images
for dep in "${DEPLOYMENTS[@]}"; do
  if kubectl get deployment "$dep" --namespace="${K8S_NAMESPACE}" &>/dev/null; then
    echo "Rolling out deployment/$dep in namespace ${K8S_NAMESPACE}..."
    kubectl rollout restart "deployment/$dep" --namespace="${K8S_NAMESPACE}"
  fi
done

# 4. (Optional) Wait for them to become ready
for dep in "${DEPLOYMENTS[@]}"; do
  if kubectl get deployment "$dep" --namespace="${K8S_NAMESPACE}" &>/dev/null; then
    echo "Waiting for rollout to finish for $dep in namespace ${K8S_NAMESPACE}..."
    if ! kubectl rollout status "deployment/$dep" --namespace="${K8S_NAMESPACE}" --timeout=180s; then
      echo "WARNING: $dep failed to roll out in time."
    fi
  fi
done

echo "Deploy-to-K8s script complete."
