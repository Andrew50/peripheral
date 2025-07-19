#!/bin/bash
set -e

# Database Recovery Script
# This script will reset the corrupted database by clearing the data directory

echo "=== PostgreSQL Database Recovery Script ==="
echo "WARNING: This will delete all existing database data!"
echo "Make sure you have backups if needed."
echo ""

read -p "Are you sure you want to proceed? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
    echo "Recovery cancelled."
    exit 0
fi

echo ""
echo "Step 1: Scaling down the database deployment..."
kubectl scale deployment db --replicas=0

echo "Waiting for pod to terminate..."
kubectl wait --for=delete pod -l app=db --timeout=120s || true

echo ""
echo "Step 2: Creating temporary pod to clear corrupted data..."

# Create a temporary pod to access the PVC and clear corrupted data
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: db-recovery-pod
  labels:
    app: db-recovery
spec:
  containers:
  - name: recovery
    image: postgres:17
    command: ["/bin/bash"]
    args: ["-c", "while true; do sleep 30; done"]
    volumeMounts:
    - name: db-pvc
      mountPath: /home/postgres/pgdata
  volumes:
  - name: db-pvc
    persistentVolumeClaim:
      claimName: db-pvc
  restartPolicy: Never
EOF

echo "Waiting for recovery pod to start..."
kubectl wait --for=condition=Ready pod/db-recovery-pod --timeout=120s

echo ""
echo "Step 3: Clearing corrupted database files..."
kubectl exec db-recovery-pod -- bash -c "
    echo 'Listing current contents of pgdata:'
    ls -la \$PGDATA/ || true
    echo ''
    echo 'Removing corrupted data directory contents...'
    rm -rf \$PGDATA/* || true
    rm -rf \$PGDATA/.* 2>/dev/null || true
    echo 'Data directory cleared.'
    echo 'Contents after cleanup:'
    ls -la \$PGDATA/ || true
"

echo ""
echo "Step 4: Cleaning up recovery pod..."
kubectl delete pod db-recovery-pod

echo ""
echo "Step 5: Restarting database deployment..."
kubectl scale deployment db --replicas=1

echo ""
echo "Step 6: Waiting for database to start..."
echo "This may take a few minutes as PostgreSQL initializes fresh..."

# Wait for pod to be running
kubectl wait --for=condition=Ready pod -l app=db --timeout=300s

echo ""
echo "Step 7: Verifying database health..."
DB_POD=$(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}')
echo "Database pod: $DB_POD"

# Wait a bit more for database to fully initialize
sleep 30

# Check if database is accepting connections
kubectl exec $DB_POD -- pg_isready -U postgres || {
    echo "Database not ready yet, checking logs..."
    kubectl logs $DB_POD --tail=20
    echo ""
    echo "Database may still be initializing. Please check logs with:"
    echo "kubectl logs $DB_POD"
    exit 1
}

echo ""
echo "=== Recovery Complete ==="
echo "Database has been reset and is now running with fresh data."
echo "You may need to run your application migrations to recreate the schema."
echo ""
echo "To check database status:"
echo "  kubectl logs $DB_POD"
echo "  kubectl exec $DB_POD -- pg_isready -U postgres"
echo ""
echo "To connect to the database:"
echo "  kubectl exec -it $DB_POD -- psql -U postgres" 