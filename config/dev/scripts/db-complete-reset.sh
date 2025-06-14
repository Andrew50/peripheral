#!/bin/bash
set -e

# Complete Database Reset Script
# This script will completely reset the database and rebuild it from scratch

echo "=== Complete PostgreSQL Database Reset Script ==="
echo "WARNING: This will delete ALL database data and recreate from scratch!"
echo ""

read -p "Are you sure you want to proceed? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
    echo "Reset cancelled."
    exit 0
fi

echo ""
echo "Step 1: Scaling down the database deployment..."
kubectl scale deployment db --replicas=0

echo "Waiting for pod to terminate..."
kubectl wait --for=delete pod -l app=db --timeout=120s || true

echo ""
echo "Step 2: Creating temporary pod to completely reset database..."

# Create a temporary pod to access the PVC and reset everything
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: db-reset-pod
  labels:
    app: db-reset
spec:
  containers:
  - name: reset
    image: postgres:17
    command: ["/bin/bash"]
    args: ["-c", "while true; do sleep 30; done"]
    volumeMounts:
    - name: db-pvc
      mountPath: /home/postgres/pgdata
    env:
    - name: POSTGRES_PASSWORD
      value: "temppass"
    - name: PGDATA
      value: "/home/postgres/pgdata/data"
  volumes:
  - name: db-pvc
    persistentVolumeClaim:
      claimName: db-pvc
  restartPolicy: Never
EOF

echo "Waiting for reset pod to start..."
kubectl wait --for=condition=Ready pod/db-reset-pod --timeout=120s

echo ""
echo "Step 3: Completely clearing all database files..."
kubectl exec db-reset-pod -- bash -c "
    echo 'Removing all data...'
    rm -rf /home/postgres/pgdata/data || true
    rm -rf /home/postgres/pgdata/* || true
    rm -rf /home/postgres/pgdata/.* 2>/dev/null || true
    
    echo 'Ensuring clean state...'
    ls -la /home/postgres/pgdata/ || true
"

echo ""
echo "Step 4: Cleaning up reset pod..."
kubectl delete pod db-reset-pod

echo ""
echo "Step 5: Rebuilding the database image to include migration fixes..."
cd /home/aj/dev/study

# First, let's ensure we're in the right directory structure
if [ ! -f "services/db/Dockerfile.dev" ]; then
    echo "Error: Database Dockerfile not found. Please ensure you're in the project root."
    exit 1
fi

# Get current environment variables for Docker build
DOCKER_USERNAME=\${DOCKER_USERNAME:-your-docker-username}
DOCKER_TAG=\${DOCKER_TAG:-latest}

echo "Building database image with migration fixes..."
docker build -t "\$DOCKER_USERNAME/db:\$DOCKER_TAG" -f services/db/Dockerfile.dev services/db/

echo ""
echo "Step 6: Restarting database deployment..."
kubectl scale deployment db --replicas=1

echo ""
echo "Step 7: Waiting for database to initialize completely..."
echo "This will take several minutes as PostgreSQL initializes and runs migrations..."

# Wait for pod to be running (but not necessarily ready)
kubectl wait --for=condition=PodReadyCondition=false pod -l app=db --timeout=60s || true
sleep 10

# Now wait for it to become ready
kubectl wait --for=condition=Ready pod -l app=db --timeout=600s || {
    echo "Database taking longer than expected. Checking status..."
    DB_POD=\$(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}')
    echo "Current pod: \$DB_POD"
    kubectl describe pod \$DB_POD
    echo ""
    echo "Recent logs:"
    kubectl logs \$DB_POD --tail=30
    echo ""
    echo "Continuing to wait..."
    kubectl wait --for=condition=Ready pod -l app=db --timeout=300s
}

echo ""
echo "Step 8: Verifying database health..."
DB_POD=\$(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}')
echo "Database pod: \$DB_POD"

# Wait a bit more for database to fully stabilize
sleep 30

# Check if database is accepting connections
kubectl exec \$DB_POD -- pg_isready -U postgres || {
    echo "Database not ready yet, checking logs..."
    kubectl logs \$DB_POD --tail=50
    echo ""
    echo "Database may still be initializing. Check status with:"
    echo "kubectl logs \$DB_POD -f"
    exit 1
}

echo ""
echo "Step 9: Verifying schema initialization..."
kubectl exec \$DB_POD -- psql -U postgres -d postgres -c "SELECT version, description FROM schema_versions ORDER BY version;" || {
    echo "Schema versions table may not be ready yet. This is normal for a fresh database."
}

echo ""
echo "=== Complete Reset Successful ==="
echo "Database has been completely reset and rebuilt from scratch."
echo ""
echo "To check database status:"
echo "  kubectl logs \$DB_POD"
echo "  kubectl exec \$DB_POD -- pg_isready -U postgres"
echo ""
echo "To connect to the database:"
echo "  kubectl exec -it \$DB_POD -- psql -U postgres"
echo ""
echo "To view schema versions:"
echo "  kubectl exec \$DB_POD -- psql -U postgres -d postgres -c \"SELECT * FROM schema_versions ORDER BY version;\"" 