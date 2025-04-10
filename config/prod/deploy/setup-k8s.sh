#!/usr/bin/env bash
set -Eeuo pipefail

K8S_CONTEXT="${1:-}"
K8S_NAMESPACE="${2:-}"

# Check if minikube is installed
echo "Checking minikube status..."
if ! minikube status &> /dev/null; then
    echo "Minikube is not running. Starting minikube with 16 CPUs and 64GB RAM..."
    # Start minikube with more verbose output
    minikube start --cpus=16 --memory=65536 --v=1 # Memory in MB (64GB = 65536MB)
    
    # Check if minikube started successfully
    if minikube status &> /dev/null; then
        echo "Minikube started successfully with increased resources."
    else
        echo "ERROR: Failed to start minikube. Please check logs above."
        exit 1
    fi
else
    echo "Minikube is already running."
    # Check if we need to update the resource allocation
    CURRENT_CPU=$(minikube config view | grep -i cpus | awk '{print $3}' 2>/dev/null || echo "unknown")
    CURRENT_MEM=$(minikube config view | grep -i memory | awk '{print $3}' 2>/dev/null || echo "unknown")
    
    if [[ "$CURRENT_CPU" != "16" || "$CURRENT_MEM" != "65536" ]]; then
        echo "WARNING: Minikube is running with different resource settings."
        echo "Current settings: CPUs=$CURRENT_CPU, Memory=$CURRENT_MEM"
        echo "Desired settings: CPUs=16, Memory=65536MB"
        echo "Note: You cannot change memory/CPU for an existing minikube cluster. To apply these settings, run 'minikube delete' first."
    fi
fi

# Ensure minikube is the current context
echo "Setting kubectl to use minikube context..."
minikube update-context

# Check if we're using minikube or a specific context
if [[ "$K8S_CONTEXT" == "minikube" || -z "$K8S_CONTEXT" ]]; then
  echo "Using minikube as the Kubernetes context"
  kubectl config use-context minikube
  CURRENT_CONTEXT="minikube"
else
  echo "Switching to Kubernetes context: $K8S_CONTEXT"
  if kubectl config get-contexts "$K8S_CONTEXT" &>/dev/null; then
    kubectl config use-context "$K8S_CONTEXT"
    CURRENT_CONTEXT=$(kubectl config current-context)
    if [[ "$CURRENT_CONTEXT" != "$K8S_CONTEXT" ]]; then
      echo "ERROR: Wrong context! Expected '$K8S_CONTEXT' but got '$CURRENT_CONTEXT'"
      exit 1
    fi
  else
    echo "WARNING: Context '$K8S_CONTEXT' not found. Using current context instead."
    CURRENT_CONTEXT=$(kubectl config current-context)
    echo "Current context: $CURRENT_CONTEXT"
  fi
fi

# Set namespace if provided
if [[ -n "$K8S_NAMESPACE" ]]; then
  echo "Setting namespace to: $K8S_NAMESPACE"
  kubectl config set-context --current --namespace="$K8S_NAMESPACE"
else
  echo "No namespace specified, using default namespace"
  K8S_NAMESPACE=$(kubectl config view --minify --output 'jsonpath={..namespace}')
  if [[ -z "$K8S_NAMESPACE" ]]; then
    K8S_NAMESPACE="default"
  fi
  echo "Current namespace: $K8S_NAMESPACE"
fi

echo "Verifying cluster connectivity..."
kubectl cluster-info #dont retry, already has retry built in

echo "Kubernetes setup complete. Current context: $CURRENT_CONTEXT, namespace: $K8S_NAMESPACE"
