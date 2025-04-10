#!/usr/bin/env bash
set -Eeuo pipefail

# Parameters:
# $1: K8S_CONTEXT - The Kubernetes context to use (default: minikube)
# $2: K8S_NAMESPACE - The namespace to use (default: default)
# $3: PROFILE_NAME - The minikube profile name (default: minikube)

K8S_CONTEXT="${1:-minikube}"
K8S_NAMESPACE="${2:-default}"
PROFILE_NAME="${3:-minikube}"  # Add profile parameter for multiple clusters

echo "Checking minikube status for profile: $PROFILE_NAME..."
if ! minikube status -p "$PROFILE_NAME" &> /dev/null; then
    echo "Minikube profile '$PROFILE_NAME' is not running. Starting with 16 CPUs and 64GB RAM..."
    

    if [[ "$PROFILE_NAME" != "minikube" ]]; then #less resources for stage
        CPU_COUNT=8
        MEM_SIZE=32768  # 32GB
    else
        CPU_COUNT=16
        MEM_SIZE=65536  # 64GB
    fi
    
    minikube start -p "$PROFILE_NAME" --cpus="$CPU_COUNT" --memory="$MEM_SIZE" --v=1
    
    if minikube status -p "$PROFILE_NAME" &> /dev/null; then
        echo "Minikube profile '$PROFILE_NAME' started successfully with CPU=$CPU_COUNT, Memory=${MEM_SIZE}MB."
    else
        echo "ERROR: Failed to start minikube profile '$PROFILE_NAME'. Please check logs above."
        exit 1
    fi
else
    echo "Minikube profile '$PROFILE_NAME' is already running."
    CURRENT_CPU=$(minikube config view -p "$PROFILE_NAME" | grep -i cpus | awk '{print $3}' 2>/dev/null || echo "unknown")
    CURRENT_MEM=$(minikube config view -p "$PROFILE_NAME" | grep -i memory | awk '{print $3}' 2>/dev/null || echo "unknown")
    
    echo "Current settings for profile '$PROFILE_NAME': CPUs=$CURRENT_CPU, Memory=$CURRENT_MEM"
fi

echo "Setting kubectl to use minikube profile '$PROFILE_NAME' context..."
minikube update-context -p "$PROFILE_NAME"

EXPECTED_CONTEXT="$PROFILE_NAME"

# Check if we're using the minikube profile or a specific context
if [[ "$K8S_CONTEXT" == "$EXPECTED_CONTEXT" || -z "$K8S_CONTEXT" ]]; then
  echo "Using minikube profile '$PROFILE_NAME' as the Kubernetes context"
  kubectl config use-context "$EXPECTED_CONTEXT"
  CURRENT_CONTEXT="$EXPECTED_CONTEXT"
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

# Set namespace if provided and create it if it doesn't exist
if [[ -n "$K8S_NAMESPACE" ]]; then
  echo "Setting namespace to: $K8S_NAMESPACE"
  
  # Check if namespace exists, create it if it doesn't
  if ! kubectl get namespace "$K8S_NAMESPACE" &>/dev/null; then
    echo "Namespace '$K8S_NAMESPACE' does not exist. Creating it..."
    kubectl create namespace "$K8S_NAMESPACE"
    echo "Namespace '$K8S_NAMESPACE' created successfully."
  else
    echo "Namespace '$K8S_NAMESPACE' already exists."
  fi
  
  # Set the namespace in the current context
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
echo "Current kubectl configuration:"
kubectl config view --minify

# Ensure kubectl is properly configured with the correct context
echo "Current kubectl context: $(kubectl config current-context)"

# Check if kubectl can reach the Kubernetes API server
echo "Testing connection to Kubernetes API server..."
if ! kubectl get nodes --namespace="$K8S_NAMESPACE" &>/dev/null; then
  echo "WARNING: Cannot connect to Kubernetes API server. Trying to fix connection..."
  
  # Try to get minikube IP
  MINIKUBE_IP=$(minikube ip 2>/dev/null)
  if [[ -n "$MINIKUBE_IP" ]]; then
    echo "Minikube IP: $MINIKUBE_IP"
    echo "Testing network connectivity to minikube IP..."
    ping -c 2 "$MINIKUBE_IP" || echo "Cannot ping minikube IP"
  fi
  
  # Try to update the minikube context again
  echo "Updating minikube context..."
  minikube update-context
  
  # Wait a moment for connections to stabilize
  sleep 5
fi

# Now try cluster-info with explicit output redirection to capture any errors
echo "Running kubectl cluster-info..."
if ! kubectl cluster-info --namespace="$K8S_NAMESPACE" > >(tee /tmp/cluster-info-out.log) 2> >(tee /tmp/cluster-info-err.log >&2); then
  echo "ERROR: kubectl cluster-info failed. See error output above."
  echo "Contents of /tmp/cluster-info-err.log:"
  cat /tmp/cluster-info-err.log
  
  # Try one more time with a different approach
  echo "Trying alternative approach to verify cluster..."
  if kubectl version --short --namespace="$K8S_NAMESPACE"; then
    echo "kubectl version succeeded, continuing despite cluster-info failure."
  else
    echo "ERROR: Both kubectl cluster-info and kubectl version failed."
    echo "Please check your Kubernetes configuration and network connectivity."
    exit 1
  fi
else
  echo "Cluster info command succeeded."
fi

echo "Kubernetes setup complete. Current context: $CURRENT_CONTEXT, namespace: $K8S_NAMESPACE"
