#!/bin/bash
set -e

# Quick Database Fix Script
# This script will fix the schema_versions table issue without a complete reset

echo "=== Quick Database Fix Script ==="
echo "This will fix the schema_versions table issue."
echo ""

# Get the current database pod
DB_POD=$(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$DB_POD" ]; then
    echo "No database pod found. Please ensure the database deployment is running."
    exit 1
fi

echo "Current database pod: $DB_POD"
echo "Pod status:"
kubectl get pod $DB_POD

# Check if pod is in CrashLoopBackOff
POD_STATUS=$(kubectl get pod $DB_POD -o jsonpath='{.status.phase}')
if [ "$POD_STATUS" != "Running" ]; then
    echo ""
    echo "Pod is not running (status: $POD_STATUS). Let's try to scale down and up..."
    
    echo "Scaling down database deployment..."
    kubectl scale deployment db --replicas=0
    
    echo "Waiting for pod to terminate..."
    kubectl wait --for=delete pod -l app=db --timeout=60s || true
    
    echo "Scaling back up..."
    kubectl scale deployment db --replicas=1
    
    echo "Waiting for new pod..."
    sleep 20
    
    DB_POD=$(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}')
    echo "New database pod: $DB_POD"
fi

echo ""
echo "Attempting to connect and diagnose the database..."

# Wait a bit for the pod to potentially start
sleep 30

# Check if we can connect
if kubectl exec $DB_POD -- pg_isready -U postgres 2>/dev/null; then
    echo "Database is responding! Checking schema_versions table..."
    
    # Check current schema versions
    echo "Current schema versions in table:"
    kubectl exec $DB_POD -- psql -U postgres -d postgres -c "SELECT version, applied_at, description FROM schema_versions ORDER BY version;" || {
        echo "Could not read schema_versions table."
    }
    
    # Check what migration files exist
    echo ""
    echo "Available migration files on the pod:"
    kubectl exec $DB_POD -- find /migrations -name "*.sql" | sort -V || echo "Migration directory not accessible"
    
    # Get the highest migration file number
    HIGHEST_MIGRATION=$(kubectl exec $DB_POD -- find /migrations -name "*.sql" -exec basename {} \; | grep -o "^[0-9]*" | sort -n | tail -1 || echo "")
    
    if [ -n "$HIGHEST_MIGRATION" ]; then
        echo "Highest migration file found: $HIGHEST_MIGRATION"
        
        # Check if there's a mismatch
        HIGHEST_VERSION=$(kubectl exec $DB_POD -- psql -U postgres -d postgres -t -c "SELECT COALESCE(MAX(version), 0) FROM schema_versions;" | tr -d ' ')
        echo "Highest version in schema_versions table: $HIGHEST_VERSION"
        
        if [ "$HIGHEST_VERSION" -ge "$HIGHEST_MIGRATION" ]; then
            echo ""
            echo "Issue found: schema_versions table has version $HIGHEST_VERSION but highest migration file is $HIGHEST_MIGRATION"
            echo "This suggests the migration system is trying to apply a non-existent migration."
            echo ""
            echo "Fixing by removing future migration entries..."
            
            kubectl exec $DB_POD -- psql -U postgres -d postgres -c "DELETE FROM schema_versions WHERE version > $HIGHEST_MIGRATION;" || {
                echo "Could not clean schema_versions table."
                exit 1
            }
            
            echo "Fixed! Restarting database pod..."
            kubectl delete pod $DB_POD
            
            echo "Waiting for new pod to start..."
            kubectl wait --for=condition=Ready pod -l app=db --timeout=300s
            
            NEW_POD=$(kubectl get pods -l app=db -o jsonpath='{.items[0].metadata.name}')
            echo "New pod: $NEW_POD"
            
            # Test connection
            sleep 30
            if kubectl exec $NEW_POD -- pg_isready -U postgres; then
                echo ""
                echo "SUCCESS! Database is now healthy."
                echo "Final schema versions:"
                kubectl exec $NEW_POD -- psql -U postgres -d postgres -c "SELECT version, applied_at, description FROM schema_versions ORDER BY version;"
            else
                echo "Database still having issues. Check logs:"
                kubectl logs $NEW_POD --tail=20
            fi
        else
            echo "Schema versions look normal. The issue might be elsewhere."
        fi
    else
        echo "Could not determine migration file numbers."
    fi
    
else
    echo "Database is not responding. Checking recent logs..."
    kubectl logs $DB_POD --tail=30
    
    echo ""
    echo "Database appears to be in a crash loop. You may need to use the complete reset script:"
    echo "./config/dev/scripts/db-complete-reset.sh"
fi

echo ""
echo "=== Quick Fix Complete ===" 