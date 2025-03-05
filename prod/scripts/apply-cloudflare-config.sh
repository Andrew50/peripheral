#!/bin/bash

# Exit on error
set -e

# Function to log messages
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Check if the Cloudflare tunnel token is provided
if [ -z "$1" ]; then
  log "Error: Cloudflare tunnel token is required"
  log "Usage: $0 <cloudflare-tunnel-token>"
  exit 1
fi

TUNNEL_TOKEN=$1
TUNNEL_TOKEN_BASE64=$(echo -n "$TUNNEL_TOKEN" | base64 -w 0)

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
log "Created temporary directory: $TEMP_DIR"

# Create the secrets file with the tunnel token
log "Creating secrets file with the tunnel token..."
cat > "$TEMP_DIR/cloudflare-secret.yaml" << EOF
apiVersion: v1
kind: Secret
metadata:
  name: cloudflare-secret
type: Opaque
data:
  tunnel-token: $TUNNEL_TOKEN_BASE64
EOF

# Apply the secret
log "Applying the Cloudflare secret..."
kubectl apply -f "$TEMP_DIR/cloudflare-secret.yaml"

# Apply the updated cloudflared configuration
log "Applying the updated cloudflared configuration..."
kubectl apply -f ../../config/prod/config/cloudflared.yaml

# Delete any existing cloudflared pods to force a restart with the new configuration
log "Restarting cloudflared pods..."
kubectl delete pods -l app=cloudflared

# Clean up
rm -rf "$TEMP_DIR"
log "Temporary files cleaned up"

# Wait for the new pod to be ready
log "Waiting for cloudflared pod to be ready..."
kubectl wait --for=condition=ready pod -l app=cloudflared --timeout=60s

# Check the logs of the new pod
log "Checking logs of the new cloudflared pod..."
POD_NAME=$(kubectl get pods -l app=cloudflared -o jsonpath='{.items[0].metadata.name}')
kubectl logs "$POD_NAME" --tail=20

log "Cloudflare configuration applied successfully!"
log "If you're still seeing 502 errors, check the logs with: kubectl logs $POD_NAME" 