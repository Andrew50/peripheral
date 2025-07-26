#!/bin/bash
set -e

# Database Backup Restoration Script
# This script will restore from a backup file
# COMMENTED OUT - Database restore functionality disabled

echo "=== PostgreSQL Database Backup Restoration Script ==="
echo "WARNING: Database restore functionality is currently disabled (commented out)"
echo ""

if [ $# -ne 1 ]; then
    echo "Usage: $0 <backup_file.sql.gz>"
    echo ""
    echo "Available backups in ./backups/:"
    ls -la ./backups/backup_*.sql.gz 2>/dev/null | tail -10 || echo "No backups found"
    echo ""
    echo "NOTE: Restore functionality is currently disabled"
    exit 1
fi

BACKUP_FILE="$1"

if [ ! -f "$BACKUP_FILE" ]; then
    echo "Error: Backup file '$BACKUP_FILE' not found"
    exit 1
fi

echo "Backup file: $BACKUP_FILE"
echo "File size: $(ls -lh "$BACKUP_FILE" | awk '{print $5}')"
echo ""
echo "NOTE: Restore functionality is currently disabled"
echo "To enable, uncomment the sections below in this script"
exit 0

# COMMENTED OUT - Backup file size check
# if [ $(stat -c%s "$BACKUP_FILE") -lt 100 ]; then
#     echo "WARNING: Backup file is very small ($(stat -c%s "$BACKUP_FILE") bytes)."
#     echo "This might indicate an empty or failed backup."
#     echo ""
#     read -p "Do you want to continue anyway? (yes/no): " confirm
#     if [ "$confirm" != "yes" ]; then
#         echo "Restoration cancelled."
#         exit 0
#     fi
# fi

# COMMENTED OUT - User confirmation
# echo "WARNING: This will replace all existing database data!"
# read -p "Are you sure you want to proceed? (yes/no): " confirm
# if [ "$confirm" != "yes" ]; then
#     echo "Restoration cancelled."
#     exit 0
# fi

# COMMENTED OUT - Database restoration process
# echo ""
# echo "Step 1: Scaling down the database deployment..."
# kubectl scale deployment db --replicas=0

# echo "Waiting for pod to terminate..."
# kubectl wait --for=delete pod -l app=db --timeout=120s || true

# echo ""
# echo "Step 2: Creating temporary pod to clear data and restore backup..."

# # Create a temporary pod to access the PVC and restore data
# cat <<EOF | kubectl apply -f -
# apiVersion: v1
# kind: Pod
# metadata:
#   name: db-restore-pod
#   labels:
#     app: db-restore
# spec:
#   containers:
#   - name: restore
#     image: postgres:17
#     command: ["/bin/bash"]
#     args: ["-c", "while true; do sleep 30; done"]
#     volumeMounts:
#     - name: db-pvc
#       mountPath: /home/postgres/pgdata
#     env:
#     - name: POSTGRES_PASSWORD
#       value: "temppass"
#     - name: PGDATA
#       value: "/home/postgres/pgdata"
#   volumes:
#   - name: db-pvc
#     persistentVolumeClaim:
#       claimName: db-pvc
#   restartPolicy: Never
# EOF

# echo "Waiting for restore pod to start..."
# kubectl wait --for=condition=Ready pod/db-restore-pod --timeout=120s

# echo ""
# echo "Step 3: Clearing existing data and initializing fresh database..."
# kubectl exec db-restore-pod -- bash -c "
#     echo 'Clearing existing data...'
#     rm -rf \$PGDATA/* || true
#     rm -rf \$PGDATA/.* 2>/dev/null || true
#     
#     echo 'Initializing fresh PostgreSQL cluster...'
#     initdb -D \$PGDATA --auth-local=trust --auth-host=md5
#     
#     echo 'Starting temporary PostgreSQL server...'
#     pg_ctl -D \$PGDATA -l /tmp/postgres.log start
#     
#     echo 'Waiting for PostgreSQL to start...'
#     sleep 5
#     
#     echo 'Creating postgres database if not exists...'
#     createdb postgres || true
# "

# echo ""
# echo "Step 4: Copying and restoring backup..."

# # Copy backup file to the pod
# kubectl cp "$BACKUP_FILE" db-restore-pod:/tmp/backup.sql.gz

# kubectl exec db-restore-pod -- bash -c "
#     echo 'Extracting backup...'
#     gunzip /tmp/backup.sql.gz
#     
#     echo 'Backup contents preview:'
#     head -20 /tmp/backup.sql || echo 'Backup appears to be empty or corrupted'
#     echo ''
#     
#     echo 'Restoring backup to postgres database...'
#     psql -U postgres -d postgres -f /tmp/backup.sql || {
#         echo 'Backup restoration failed. Creating empty database.'
#         # If restore fails, just ensure we have a working database
#     }
#     
#     echo 'Stopping temporary PostgreSQL server...'
#     pg_ctl -D \$PGDATA stop
# "

# echo ""
# echo "Step 5: Cleaning up restore pod..."
# kubectl delete pod db-restore-pod

# echo ""
# echo "Step 6: Restarting database deployment..."
# kubectl scale deployment db --replicas=1

# echo ""
# echo "Step 7: Waiting for database to start..."
# kubectl wait --for=condition=Ready pod -l app=db --timeout=300s

# echo ""
# echo "Step 8: Verifying database health..."
# DB_POD=$(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}')
# echo "Database pod: $DB_POD"

# # Wait a bit more for database to fully initialize
# sleep 30

# # Check if database is accepting connections
# kubectl exec $DB_POD -- pg_isready -U postgres || {
#     echo "Database not ready yet, checking logs..."
#     kubectl logs $DB_POD --tail=20
#     echo ""
#     echo "Database may still be initializing. Please check logs with:"
#     echo "kubectl logs $DB_POD"
#     exit 1
# }

# echo ""
# echo "=== Restoration Complete ==="
# echo "Database has been restored from backup: $BACKUP_FILE"
# echo ""
# echo "To check database status:"
# echo "  kubectl logs $DB_POD"
# echo "  kubectl exec $DB_POD -- pg_isready -U postgres"
# echo ""
# echo "To connect to the database:"
# echo "  kubectl exec -it $DB_POD -- psql -U postgres" 