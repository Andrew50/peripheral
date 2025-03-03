#!/bin/bash

# Function to log messages with timestamps
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to log errors
error_log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" >&2
}

# Function to check deployment status and display logs on failure
check_deployment() {
  local deployment=$1
  local timeout=${2:-30}
  
  log "Waiting for $deployment deployment to be ready (timeout: ${timeout}s)..."
  
  # Use timeout command to ensure the kubectl rollout status doesn't hang indefinitely
  if ! timeout ${timeout}s kubectl rollout status deployment/$deployment; then
    error_log "$deployment deployment failed or timed out after ${timeout} seconds!"
    
    # Display pod status for this deployment
    log "Pod status for $deployment:"
    kubectl get pods -l app=$deployment
    
    # Display logs for the failed pods
    log "Logs from $deployment pods:"
    kubectl logs -l app=$deployment --tail=50 || true
    
    # For specific deployments, show additional diagnostics
    if [[ "$deployment" == "cache" ]]; then
      log "Redis config details:"
      kubectl describe configmap redis-config || true
    elif [[ "$deployment" == "db" ]]; then
      log "Database PVC status:"
      kubectl get pvc db-pvc || true
    fi
    
    # Return failure
    return 1
  fi
  
  log "$deployment deployment is ready!"
  return 0
}

# Function to clear Redis data from the persistent volume
clear_redis_data() {
  log "Checking for existing Redis data..."
  
  # Create a temporary pod to access the PV
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: redis-data-cleaner
spec:
  containers:
  - name: redis-data-cleaner
    image: busybox
    command: ["sleep", "600"]
    volumeMounts:
    - name: redis-data
      mountPath: /data
  volumes:
  - name: redis-data
    persistentVolumeClaim:
      claimName: cache-pvc
EOF

  # Wait for the pod to be ready
  log "Waiting for data cleaner pod to be ready..."
  kubectl wait --for=condition=Ready pod/redis-data-cleaner --timeout=30s
  
  if [ $? -eq 0 ]; then
    # Check if there are Redis data files
    log "Checking Redis data files..."
    HAS_DATA=$(kubectl exec redis-data-cleaner -- ls -la /data | grep -E 'appendonly.aof|dump.rdb' || echo "")
    
    if [ ! -z "$HAS_DATA" ]; then
      log "Found Redis data files. Creating backup and clearing..."
      kubectl exec redis-data-cleaner -- sh -c "mkdir -p /data/backup-$(date +%Y%m%d%H%M%S) && mv /data/appendonly.aof* /data/dump.rdb* /data/backup-$(date +%Y%m%d%H%M%S)/ 2>/dev/null || true"
      log "Redis data backup created and old files cleared."
    else
      log "No Redis data files found."
    fi
  else
    error_log "Failed to start data cleaner pod. Cannot clear Redis data."
  fi
  
  # Clean up the temporary pod
  kubectl delete pod redis-data-cleaner --grace-period=0 --force
  log "Temporary data cleaner pod removed."
}

# Default to prod if no branch is specified
BRANCH=${1:-prod}

# Validate branch parameter
if [[ "$BRANCH" != "prod" && "$BRANCH" != "dev" ]]; then
  error_log "Invalid branch specified: $BRANCH. Must be either 'prod' or 'dev'."
  echo "Usage: $0 [prod|dev]"
  exit 1
fi

# Set environment variables for Kubernetes manifests
export DOCKER_USERNAME=${DOCKER_USERNAME:-billin19}
export IMAGE_TAG=$BRANCH
export NGINX_IMAGE=${NGINX_IMAGE:-k8s.gcr.io/ingress-nginx/controller:v1.2.1}
export CLOUDFLARED_IMAGE=${CLOUDFLARED_IMAGE:-cloudflare/cloudflared:2023.8.0}
export CLOUDFLARE_TUNNEL_TOKEN=${CLOUDFLARE_TUNNEL_TOKEN:-"your-tunnel-token"}

log "Starting deployment fixes for branch: $BRANCH..."
log "Using Docker username: $DOCKER_USERNAME"
log "Using image tag: $IMAGE_TAG"

# Create a temporary directory for processed YAML files
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

# Create a helper script to do the substitutions using bash
cat > "$TEMP_DIR/substitute.sh" << 'EOFSUBST'
#!/bin/bash
input_file="$1"
output_file="$2"

# Use cat with parameter expansion to replace variables directly
eval "cat << EOFEVAL
$(cat "$input_file")
EOFEVAL
" > "$output_file"
EOFSUBST

chmod +x "$TEMP_DIR/substitute.sh"

# Process YAML files with direct variable substitution
for yaml_file in config/*.yaml; do
  base_name=$(basename "$yaml_file")
  processed_file="$TEMP_DIR/$base_name"
  
  log "Processing $yaml_file with variable substitution"
  
  # Use bash directly to perform the substitutions
  bash "$TEMP_DIR/substitute.sh" "$yaml_file" "$processed_file"
  
  # Verify processed file for debugging
  if [[ "$base_name" == "db.yaml" ]]; then
    log "Checking processed DB file image reference:"
    grep -A 2 "image:" "$processed_file" || true
  fi
done

# Track failures
FAILED_DEPLOYMENTS=()

# Step 1: Fix the Redis cache deployment
log "Fixing Redis cache deployment..."
kubectl delete deployment cache --ignore-not-found=true
# Clear Redis data before redeploying
clear_redis_data
kubectl apply -f "$TEMP_DIR/cache.yaml"
check_deployment "cache" 60 || FAILED_DEPLOYMENTS+=("cache")

# Step 2: Fix the database deployment
log "Fixing database deployment..."
kubectl delete deployment db --ignore-not-found=true
kubectl apply -f "$TEMP_DIR/db.yaml"
check_deployment "db" 30 || FAILED_DEPLOYMENTS+=("db")

# Step 3: Fix the TensorFlow deployment
log "Fixing TensorFlow deployment..."
kubectl delete deployment tf --ignore-not-found=true
kubectl apply -f "$TEMP_DIR/tf.yaml"
check_deployment "tf" 30 || FAILED_DEPLOYMENTS+=("tf")

# Step 4: Fix the worker deployment
log "Fixing worker deployment..."
kubectl delete deployment worker --ignore-not-found=true
kubectl apply -f "$TEMP_DIR/worker.yaml"
check_deployment "worker" 30 || FAILED_DEPLOYMENTS+=("worker")

# Step 5: Fix the backend deployment
log "Fixing backend deployment..."
kubectl delete deployment backend --ignore-not-found=true
kubectl apply -f "$TEMP_DIR/backend.yaml"
check_deployment "backend" 30 || FAILED_DEPLOYMENTS+=("backend")

# Step 6: Fix the frontend deployment
log "Fixing frontend deployment..."
kubectl delete deployment frontend --ignore-not-found=true
kubectl apply -f "$TEMP_DIR/frontend.yaml"
check_deployment "frontend" 30 || FAILED_DEPLOYMENTS+=("frontend")

# Step 7: Fix the nginx deployment
log "Fixing nginx deployment..."
kubectl delete deployment ingress-nginx-controller --ignore-not-found=true
kubectl apply -f "$TEMP_DIR/nginx.yaml"
check_deployment "ingress-nginx-controller" 30 || FAILED_DEPLOYMENTS+=("ingress-nginx-controller")

# Step 8: Fix the cloudflared deployment
log "Fixing cloudflared deployment..."
kubectl delete deployment cloudflared --ignore-not-found=true
kubectl apply -f "$TEMP_DIR/cloudflared.yaml"
check_deployment "cloudflared" 30 || FAILED_DEPLOYMENTS+=("cloudflared")

# Step 9: Check the status of all deployments
log "Checking final deployment status..."
timeout 10s kubectl get deployments || log "Failed to retrieve deployments"

# Step 10: Check the status of all pods
log "Checking pod status..."
timeout 10s kubectl get pods || log "Failed to retrieve pods"

# Report failed deployments
if [ ${#FAILED_DEPLOYMENTS[@]} -gt 0 ]; then
  error_log "The following deployments failed or timed out:"
  for deployment in "${FAILED_DEPLOYMENTS[@]}"; do
    error_log "  - $deployment"
  done
  error_log "Check the logs above for more details."
  exit 1
else
  log "All deployments completed successfully!"
fi

log "Deployment fixes completed for branch: $BRANCH."
exit 0 