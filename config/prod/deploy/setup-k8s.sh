#!/usr/bin/env bash
set -Eeuo pipefail

K8S_CONTEXT="${1:-}"
K8S_NAMESPACE="${2:-}"

echo "Switching to Kubernetes context: $K8S_CONTEXT"
kubectl config use-context "$K8S_CONTEXT"

CURRENT_CONTEXT=$(kubectl config current-context)
if [[ "$CURRENT_CONTEXT" != "$K8S_CONTEXT" ]]; then
  echo "ERROR: Wrong context! Expected '$K8S_CONTEXT' but got '$CURRENT_CONTEXT'"
  exit 1
fi

echo "Setting namespace to: $K8S_NAMESPACE"
kubectl config set-context --current --namespace="$K8S_NAMESPACE"

echo "Verifying cluster connectivity..."
# Add retry logic for cluster connectivity
max_retries=5
retry_count=0
retry_delay=10

while [ $retry_count -lt $max_retries ]; do
  if kubectl cluster-info; then
    echo "Successfully connected to Kubernetes cluster"
    echo "Kubernetes setup complete. Current context: $CURRENT_CONTEXT, namespace: $K8S_NAMESPACE"
    exit 0
  else
    retry_count=$((retry_count+1))
    if [ $retry_count -lt $max_retries ]; then
      echo "Failed to connect to cluster. Retrying in ${retry_delay} seconds... (Attempt ${retry_count}/${max_retries})"
      sleep $retry_delay
      # Increase delay for next retry (exponential backoff)
      retry_delay=$((retry_delay*2))
    else
      echo "Failed to connect to cluster after ${max_retries} attempts."
      exit 1
    fi
  fi
done
