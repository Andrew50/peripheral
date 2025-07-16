#!/usr/bin/env bash

#setup-cluster.sh
set -Eeuo pipefail


: "${K8S_NAMESPACE:?Missing K8S_NAMESPACE}"
: "${MINIKUBE_PROFILE:?Missing MINIKUBE_PROFILE}" # use this as kubectl context every time
echo "Checking minikube status for profile: $MINIKUBE_PROFILE..."
if ! minikube status -p "$MINIKUBE_PROFILE" &> /dev/null; then
    echo "Minikube profile '$MINIKUBE_PROFILE' is not running. Starting with 16 CPUs and 64GB RAM..."
    

    if [[ "$MINIKUBE_PROFILE" != "minikube-prod" ]]; then #less resources for stage
        CPU_COUNT=8
        MEM_SIZE=32768  # 32GB
    else
        CPU_COUNT=16
        MEM_SIZE=65536  # 64GB
    fi
    
    minikube start -p "$MINIKUBE_PROFILE" --cpus="$CPU_COUNT" --memory="$MEM_SIZE" --disk-size="400g" --v=1
    
    if minikube status -p "$MINIKUBE_PROFILE" &> /dev/null; then
        echo "Minikube profile '$MINIKUBE_PROFILE' started successfully with CPU=$CPU_COUNT, Memory=${MEM_SIZE}MB."
    else
        echo "ERROR: Failed to start minikube profile '$MINIKUBE_PROFILE'. Please check logs above."
        exit 1
    fi
else
    echo "Minikube profile '$MINIKUBE_PROFILE' is already running."
    CURRENT_CPU=$(minikube config view -p "$MINIKUBE_PROFILE" | grep -i cpus | awk '{print $3}' 2>/dev/null || echo "unknown")
    CURRENT_MEM=$(minikube config view -p "$MINIKUBE_PROFILE" | grep -i memory | awk '{print $3}' 2>/dev/null || echo "unknown")
    
    echo "Current settings for profile '$MINIKUBE_PROFILE': CPUs=$CURRENT_CPU, Memory=$CURRENT_MEM"
fi

echo "Setting kubectl to use minikube profile '$MINIKUBE_PROFILE' context..."
minikube update-context -p "$MINIKUBE_PROFILE"

enable_if_missing () {
  local addon=$1
  if ! minikube addons list -p "$profile" | grep -qE "${addon}[[:space:]]+enabled"; then
    echo "Enabling $addon addon for profile '$profile'..."
    minikube addons enable "$addon" -p "$profile"
    echo "$addon addon enabled."
  else
    echo "$addon addon already enabled for '$profile'."
  fi
}

# --- Required for Ingresses ---------------------------------------------------
enable_if_missing ingress           # nginx‑ingress controller + admission webhook
enable_if_missing metrics-server    # lets HPAs fetch CPU/memory metrics

# Wait for the ingress-dns pods to be ready (up to 2 minutes)
echo "Waiting for ingress-dns pods to become ready..."
if ! kubectl wait --namespace kube-system --context="${MINIKUBE_PROFILE}" \
  --for=condition=ready pod \
  --selector=k8s-app=ingress-dns \
  --timeout=120s; then
  echo "ERROR: ingress-dns pods did not become ready in time."
  kubectl get pods --namespace kube-system --context="${MINIKUBE_PROFILE}" --selector=k8s-app=ingress-dns -o wide
  exit 1
fi
echo "Ingress-dns pods are ready."

echo "Waiting for ingress-nginx admission configuration jobs to complete..."
# Wait up to 2 minutes for the admission create job
if ! kubectl wait --namespace ingress-nginx --context="${MINIKUBE_PROFILE}" \
  --for=condition=complete job \
  --selector=app.kubernetes.io/component=admission-webhook \
  --timeout=120s; then
  echo "ERROR: Ingress Nginx admission jobs did not complete in time."
  echo "Checking job status:"
  kubectl get jobs --namespace ingress-nginx --context="${MINIKUBE_PROFILE}" --selector=app.kubernetes.io/component=admission-webhook -o wide
  echo "Checking pod status for jobs:"
  kubectl get pods --namespace ingress-nginx --context="${MINIKUBE_PROFILE}" --selector=app.kubernetes.io/component=admission-webhook -o wide
  exit 1
fi
echo "Ingress Nginx admission jobs completed."


echo "Waiting for ingress-nginx controller deployment to be ready..."
# Wait up to 2 minutes for the deployment to become available in the ingress-nginx namespace
if ! kubectl wait --namespace ingress-nginx --context="${MINIKUBE_PROFILE}" \
  --for=condition=available deployment \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s; then
  echo "ERROR: Ingress Nginx controller deployment did not become ready in time."
  echo "Describing deployment:"
  kubectl describe deployment --namespace ingress-nginx --context="${MINIKUBE_PROFILE}" --selector=app.kubernetes.io/component=controller
  echo "Checking pods:"
  kubectl get pods --namespace ingress-nginx --context="${MINIKUBE_PROFILE}" --selector=app.kubernetes.io/component=controller -o wide
  exit 1
fi
echo "Ingress Nginx controller deployment is ready."


#kubectl config use-context "${MINIKUBE_PROFILE}" #should --context arg be passed here? remove this becuase all kuectl commands statelessly use --context="{MINIKUBE_PROFILE}"

# Set namespace if provided and create it if it doesn't exist
if [[ -n "$K8S_NAMESPACE" ]]; then
  echo "Setting namespace to: $K8S_NAMESPACE"
  
  # Check if namespace exists, create it if it doesn't
  if ! kubectl get namespace "$K8S_NAMESPACE" --context=${MINIKUBE_PROFILE} &>/dev/null; then
    echo "Namespace '$K8S_NAMESPACE' does not exist. Creating it..."
    kubectl create namespace "$K8S_NAMESPACE" --context=${MINIKUBE_PROFILE}
    echo "Namespace '$K8S_NAMESPACE' created successfully."
  else
    echo "Namespace '$K8S_NAMESPACE' already exists."
  fi
  
  # Set the namespace in the current context
  kubectl config set-context "$MINIKUBE_PROFILE" --namespace="$K8S_NAMESPACE"
else
  echo "No namespace specified, using default namespace"
  K8S_NAMESPACE=$(kubectl config view --context=${MINIKUBE_PROFILE} --minify --output 'jsonpath={..namespace}')
  if [[ -z "$K8S_NAMESPACE" ]]; then
    K8S_NAMESPACE="default"
  fi
  echo "Current namespace: $K8S_NAMESPACE"
fi

echo "Verifying cluster connectivity..."
echo "Current kubectl configuration:"
kubectl config view --context=${MINIKUBE_PROFILE} --minify

# Ensure kubectl is properly configured with the correct context
echo "Current kubectl context: $(kubectl config current-context)"

# Check if kubectl can reach the Kubernetes API server
echo "Testing connection to Kubernetes API server..."
if ! kubectl get nodes --namespace="$K8S_NAMESPACE" --context="${MINIKUBE_PROFILE}" &>/dev/null; then
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
  minikube update-context -p "$MINIKUBE_PROFILE"
  
  # Wait a moment for connections to stabilize
  sleep 5
fi

# Now try cluster-info with explicit output redirection to capture any errors
echo "Running kubectl cluster-info..."
if ! kubectl cluster-info --namespace="$K8S_NAMESPACE" --context="${MINIKUBE_PROFILE}" > >(tee /tmp/cluster-info-out.log) 2> >(tee /tmp/cluster-info-err.log >&2); then
  echo "ERROR: kubectl cluster-info failed. See error output above."
  echo "Contents of /tmp/cluster-info-err.log:"
  cat /tmp/cluster-info-err.log
  
  # Try one more time with a different approach
  echo "Trying alternative approach to verify cluster..."
  if kubectl version --short --namespace="$K8S_NAMESPACE" --context="${MINIKUBE_PROFILE}"; then
    echo "kubectl version succeeded, continuing despite cluster-info failure."
  else
    echo "ERROR: Both kubectl cluster-info and kubectl version failed."
    echo "Please check your Kubernetes configuration and network connectivity."
    exit 1
  fi
else
  echo "Cluster info command succeeded."
fi

echo "Kubernetes setup complete. Current context: {$MINIKUBE_PROFILE}, namespace: {$K8S_NAMESPACE}"
