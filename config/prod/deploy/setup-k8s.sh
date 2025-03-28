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
kubectl cluster-info

echo "Kubernetes setup complete. Current context: $CURRENT_CONTEXT, namespace: $K8S_NAMESPACE"
