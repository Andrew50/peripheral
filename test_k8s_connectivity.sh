#!/bin/bash

# Function to log messages with timestamps
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to log errors
error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" >&2
}

log "Testing Kubernetes connectivity with retry logic..."

# Check Kubernetes cluster connectivity with retry logic
log "Checking Kubernetes cluster connectivity..."
MAX_RETRIES=3
RETRY_COUNT=0
CONNECTED=false

while [ $RETRY_COUNT -lt $MAX_RETRIES ] && [ "$CONNECTED" = false ]; do
  if kubectl cluster-info &>/dev/null; then
    log "Kubernetes cluster is accessible."
    CONNECTED=true
  else
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
      log "Cannot connect to Kubernetes cluster. Retrying in 5 seconds... (Attempt $RETRY_COUNT of $MAX_RETRIES)"
      sleep 5
      
      # If using minikube, try to start it
      if command -v minikube &>/dev/null; then
        log "Attempting to start minikube..."
        minikube status &>/dev/null || minikube start
      fi
    else
      error_log "Cannot connect to Kubernetes cluster after $MAX_RETRIES attempts."
      error_log "If using minikube, run 'minikube start' to start the cluster."
      error_log "Cluster connection details:"
      kubectl config view
      exit 1
    fi
  fi
done

log "Test completed successfully!" 