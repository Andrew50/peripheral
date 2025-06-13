#!/bin/bash
set -e

# Deploy Database Backup and Recovery System
# This script deploys the complete backup and auto-recovery infrastructure

echo "=== Database Backup & Recovery System Deployment ==="
echo ""

# Check if we're in the right directory
if [ ! -f "services/db/Dockerfile.dev" ]; then
    echo "Error: Please run this script from the project root directory"
    exit 1
fi

# Get environment variables (you may need to set these)
DOCKER_USERNAME=${DOCKER_USERNAME:-$(whoami)}
DOCKER_TAG=${DOCKER_TAG:-latest}

echo "Docker settings:"
echo "  Username: $DOCKER_USERNAME"
echo "  Tag: $DOCKER_TAG"
echo ""

# Step 1: Rebuild the database image with new scripts
echo "Step 1: Building updated database image..."
docker build -t "$DOCKER_USERNAME/db:$DOCKER_TAG" -f services/db/Dockerfile.dev services/db/

if command -v minikube >/dev/null 2>&1; then
    echo "Loading image into minikube..."
    minikube image load "$DOCKER_USERNAME/db:$DOCKER_TAG"
fi

echo "Database image built successfully!"
echo ""

# Step 2: Apply the backup system configuration
echo "Step 2: Deploying backup system infrastructure..."

# Create temporary directory for processed manifests
TEMP_DIR=$(mktemp -d)
echo "Using temporary directory: $TEMP_DIR"

# Process the manifest template
envsubst < config/deploy/k8s/db-backup-system.yaml > "$TEMP_DIR/db-backup-system.yaml"

# Apply the configuration
kubectl apply -f "$TEMP_DIR/db-backup-system.yaml"

echo "Backup system infrastructure deployed!"
echo ""

# Step 3: Wait for backup PVC to be bound
echo "Step 3: Waiting for backup storage to be ready..."
kubectl wait --for=condition=Bound pvc/db-backups-pvc --timeout=120s
echo "Backup storage ready!"
echo ""

# Step 4: Restart the main database deployment to use new image
echo "Step 4: Updating main database deployment..."
kubectl patch deployment db -p "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"backup-system-deploy\":\"$(date)\"}}}}}"

echo "Waiting for database to restart..."
kubectl rollout status deployment/db --timeout=300s

# Verify database health
DB_POD=$(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}')
echo "Database pod: $DB_POD"

kubectl wait --for=condition=Ready pod/$DB_POD --timeout=180s
echo "Database is ready!"
echo ""

# Step 5: Wait for health monitor to start
echo "Step 5: Starting health monitoring system..."
kubectl wait --for=condition=Available deployment/db-health-monitor --timeout=120s

MONITOR_POD=$(kubectl get pods -l app=db-health-monitor -o jsonpath='{.items[0].metadata.name}')
echo "Health monitor pod: $MONITOR_POD"

kubectl wait --for=condition=Ready pod/$MONITOR_POD --timeout=120s
echo "Health monitor is ready!"
echo ""

# Step 6: Trigger an immediate backup to test the system
echo "Step 6: Creating initial backup to test the system..."
kubectl create job db-initial-backup --from=cronjob/db-backup-cronjob

echo "Waiting for backup job to complete..."
kubectl wait --for=condition=Complete job/db-initial-backup --timeout=600s

# Get backup job logs
echo "Backup job logs:"
kubectl logs job/db-initial-backup

# Clean up the test job
kubectl delete job db-initial-backup

echo ""

# Step 7: Verify the backup was created
echo "Step 7: Verifying backup creation..."

# Check if backup files exist
sleep 10  # Give some time for file system sync

# List recent backups through a temporary pod
kubectl run backup-check --image=busybox:latest --rm -i --restart=Never --overrides='
{
  "spec": {
    "containers": [{
      "name": "backup-check",
      "image": "busybox:latest",
      "command": ["sh", "-c", "ls -la /backups/backup_*.sql.gz 2>/dev/null | tail -5 || echo \"No backups found yet\""],
      "volumeMounts": [{
        "name": "backups-volume",
        "mountPath": "/backups"
      }]
    }],
    "volumes": [{
      "name": "backups-volume",
      "persistentVolumeClaim": {
        "claimName": "db-backups-pvc"
      }
    }]
  }
}'

echo ""

# Clean up temporary files
rm -rf "$TEMP_DIR"

echo "=== Backup & Recovery System Deployment Complete! ==="
echo ""
echo "📋 System Overview:"
echo "  ✅ Automated backups: 6 AM and 6 PM daily"
echo "  ✅ Health monitoring: Every 60 seconds"
echo "  ✅ Backup verification: 3 AM daily"
echo "  ✅ Cleanup/retention: 2 AM daily (30-day retention)"
echo "  ✅ Auto-recovery: Triggers after 3 health check failures"
echo ""
echo "🔧 Management Commands:"
echo "  Check backup status:"
echo "    kubectl get cronjobs"
echo "    kubectl get jobs"
echo ""
echo "  View backup logs:"
echo "    kubectl logs job/db-backup-cronjob-[timestamp]"
echo ""
echo "  Check health monitor:"
echo "    kubectl logs deployment/db-health-monitor"
echo ""
echo "  View backup files:"
echo "    kubectl exec -it \$(kubectl get pods -l app=db-health-monitor -o jsonpath='{.items[0].metadata.name}') -- ls -la /backups/"
echo ""
echo "  Manual backup:"
echo "    kubectl create job manual-backup-\$(date +%s) --from=cronjob/db-backup-cronjob"
echo ""
echo "  Manual recovery (if needed):"
echo "    kubectl exec -it \$(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}') -- /app/recovery-restore.sh /backups/backup_[timestamp].sql.gz"
echo ""
echo "📊 Monitoring:"
echo "  Health status: kubectl logs deployment/db-health-monitor --tail=10"
echo "  Backup status: kubectl logs cronjob/db-backup-cronjob"
echo "  Recovery alerts: kubectl exec [health-monitor-pod] -- ls /backups/alert-*" 