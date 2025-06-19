#!/bin/bash
set -euo pipefail

# Deploy Cluster Monitoring System
# This script deploys the complete cluster monitoring and alerting infrastructure

echo "=== Cluster Monitoring System Deployment ==="
echo ""

# Check if we're in the right directory
if [ ! -f "services/cluster-monitor/Dockerfile" ]; then
    echo "Error: Please run this script from the project root directory"
    exit 1
fi

# Get environment variables
DOCKER_USERNAME=${DOCKER_USERNAME:-$(whoami)}
DOCKER_TAG=${DOCKER_TAG:-latest}
ENVIRONMENT=${ENVIRONMENT:-stage}
K8S_NAMESPACE=${K8S_NAMESPACE:-stage}
MINIKUBE_PROFILE="minikube-${ENVIRONMENT}"

echo "Deployment settings:"
echo "  Docker Username: $DOCKER_USERNAME"
echo "  Docker Tag: $DOCKER_TAG"
echo "  Environment: $ENVIRONMENT"
echo "  Namespace: $K8S_NAMESPACE"
echo "  Minikube Profile: $MINIKUBE_PROFILE"
echo ""

# Step 1: Build the cluster monitor image
echo "Step 1: Building cluster monitor image..."
docker build -t "$DOCKER_USERNAME/cluster-monitor:$DOCKER_TAG" services/cluster-monitor/

if command -v minikube >/dev/null 2>&1; then
    echo "Loading image into minikube..."
    minikube image load "$DOCKER_USERNAME/cluster-monitor:$DOCKER_TAG" -p "$MINIKUBE_PROFILE"
fi

echo "Cluster monitor image built successfully!"
echo ""

# Step 2: Check if Telegram secrets exist
echo "Step 2: Checking Telegram configuration..."
if ! kubectl get secret telegram-secret -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" >/dev/null 2>&1; then
    echo "❌ ERROR: telegram-secret not found!"
    echo ""
    echo "Telegram configuration is required for monitoring alerts."
    echo "Please configure Telegram alerts first:"
    echo "  ./config/dev/scripts/setup-telegram-alerts.sh"
    echo ""
    echo "Or if you have the credentials, create the secret manually:"
    echo "  kubectl create secret generic telegram-secret \\"
    echo "    --from-literal=bot-token=\"YOUR_BOT_TOKEN\" \\"
    echo "    --from-literal=chat-id=\"YOUR_CHAT_ID\" \\"
    echo "    -n \"$K8S_NAMESPACE\" \\"
    echo "    --context=\"$MINIKUBE_PROFILE\""
    echo ""
    exit 1
else
    echo "✅ Telegram secret found"
    
    # Verify the secret has actual values
    BOT_TOKEN=$(kubectl get secret telegram-secret -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" -o jsonpath='{.data.bot-token}' | base64 -d)
    CHAT_ID=$(kubectl get secret telegram-secret -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" -o jsonpath='{.data.chat-id}' | base64 -d)
    
    if [ -z "$BOT_TOKEN" ] || [ -z "$CHAT_ID" ]; then
        echo "❌ ERROR: Telegram secret exists but is empty!"
        echo "Please configure Telegram alerts with valid credentials:"
        echo "  ./config/dev/scripts/setup-telegram-alerts.sh"
        exit 1
    fi
    
    echo "✅ Telegram configuration validated"
fi

# Step 3: Apply the monitoring configuration
echo "Step 3: Deploying monitoring system infrastructure..."

# Create temporary directory for processed manifests
TEMP_DIR=$(mktemp -d)
echo "Using temporary directory: $TEMP_DIR"

# Process the manifest template
export DOCKER_USERNAME K8S_NAMESPACE DOCKER_TAG ENVIRONMENT MINIKUBE_PROFILE
envsubst < config/deploy/k8s/cluster-monitor.yaml > "$TEMP_DIR/cluster-monitor.yaml"

# Apply the configuration
kubectl apply -f "$TEMP_DIR/cluster-monitor.yaml" --context="$MINIKUBE_PROFILE"

# Cleanup temporary directory
rm -rf "$TEMP_DIR"

echo "Monitoring system infrastructure deployed!"
echo ""

# Step 4: Wait for monitoring PVC to be bound
echo "Step 4: Waiting for monitoring storage to be ready..."
kubectl wait --for=condition=Bound pvc/monitor-logs-pvc --timeout=120s -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE"
echo "✅ Monitoring storage ready!"
echo ""

# Step 5: Wait for cluster monitor deployment to be ready
echo "Step 5: Waiting for cluster monitor to start..."
kubectl wait --for=condition=available deployment/cluster-monitor --timeout=300s -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE"
echo "✅ Cluster monitor is running!"
echo ""

# Step 6: Verify node monitor daemonset
echo "Step 6: Checking node monitor status..."
kubectl rollout status daemonset/node-monitor --timeout=120s -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE"
echo "✅ Node monitor is running!"
echo ""

# Step 7: Show status
echo "Step 7: Monitoring system status..."
echo ""
echo "=== Cluster Monitor Pods ==="
kubectl get pods -l app=cluster-monitor -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE"
echo ""
echo "=== Node Monitor Pods ==="
kubectl get pods -l app=node-monitor -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE"
echo ""
echo "=== CronJobs ==="
kubectl get cronjobs -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE"
echo ""

# Step 8: Setup systemd auto-start (optional)
echo "Step 8: Setting up auto-start service..."
if [ "$EUID" -eq 0 ] || groups | grep -q sudo; then
    echo "Setting up systemd service for auto-start..."
    
    # Copy systemd service file
    sudo cp config/deploy/systemd/minikube-auto-start.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable minikube-auto-start.service
    
    echo "✅ Auto-start service enabled"
    echo "Minikube will automatically start on system boot"
else
    echo "⚠️  Cannot setup auto-start (need sudo access)"
    echo "To enable auto-start manually, run:"
    echo "  sudo cp config/deploy/systemd/minikube-auto-start.service /etc/systemd/system/"
    echo "  sudo systemctl daemon-reload"
    echo "  sudo systemctl enable minikube-auto-start.service"
fi
echo ""

# Show logs
echo "=== Recent Cluster Monitor Logs ==="
kubectl logs deployment/cluster-monitor --tail=10 -n "$K8S_NAMESPACE" --context="$MINIKUBE_PROFILE" || echo "No logs available yet"
echo ""

echo "=== Cluster Monitoring System Deployment Complete! ==="
echo ""
echo "📋 System Overview:"
echo "  ✅ Cluster monitoring: Every 30 seconds"
echo "  ✅ Resource reports: Every hour"
echo "  ✅ Node monitoring: Every minute"
echo "  ✅ Auto-restart: Enabled (after 3 failures)"
echo "  ✅ Telegram alerts: $([ -n "${TELEGRAM_BOT_TOKEN:-}" ] && echo "Configured" || echo "Pending setup")"
echo ""
echo "🔧 Management Commands:"
echo "  Check monitor status:"
echo "    kubectl get pods -l app=cluster-monitor -n $K8S_NAMESPACE --context=$MINIKUBE_PROFILE"
echo ""
echo "  View monitor logs:"
echo "    kubectl logs deployment/cluster-monitor -n $K8S_NAMESPACE --context=$MINIKUBE_PROFILE"
echo ""
echo "  Check resource reports:"
echo "    kubectl logs job/resource-report-[timestamp] -n $K8S_NAMESPACE --context=$MINIKUBE_PROFILE"
echo ""
echo "  Manual resource report:"
echo "    kubectl create job manual-resource-report --from=cronjob/resource-report -n $K8S_NAMESPACE --context=$MINIKUBE_PROFILE"
echo ""
echo "  Setup Telegram alerts:"
echo "    ./config/dev/scripts/setup-telegram-alerts.sh"
echo ""
echo "📊 Monitoring Features:"
echo "  • Cluster health monitoring (API, nodes, pods)"
echo "  • Resource usage tracking (CPU, memory, disk)"
echo "  • Automatic cluster restart on failures"
echo "  • Telegram notifications for all events"
echo "  • Hourly resource reports"
echo "  • Event tracking and recommendations"
echo ""
echo "🚨 Alert Types:"
echo "  🚨 CRITICAL - Cluster failures requiring restart"
echo "  ❌ ERROR - Service errors or connectivity issues"
echo "  ⚠️  WARNING - Resource pressure or pod issues"
echo "  🔄 RESTART - Cluster restart operations"
echo "  ✅ SUCCESS - Successful operations and recovery"
echo "  📊 REPORT - Hourly resource usage reports"
echo ""
echo "Monitor is now actively watching your cluster!"
echo "You should receive a startup notification if Telegram is configured." 