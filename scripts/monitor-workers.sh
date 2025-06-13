#!/usr/bin/env bash
set -euo pipefail

# Script to monitor worker queue and performance
# Usage: ./scripts/monitor-workers.sh [dev|prod|stage]

ENVIRONMENT=${1:-dev}

echo "=== Worker Queue Monitor ==="
echo "Environment: $ENVIRONMENT"
echo "Timestamp: $(date)"
echo

case $ENVIRONMENT in
    dev)
        echo "Monitoring development environment..."
        
        # Check if docker-compose is running
        if ! docker-compose -f config/dev/docker-compose.yaml ps | grep -q "worker"; then
            echo "Error: Workers are not running in development environment"
            echo "Start them with: docker-compose -f config/dev/docker-compose.yaml up -d"
            exit 1
        fi
        
        echo "=== Docker Compose Worker Status ==="
        docker-compose -f config/dev/docker-compose.yaml ps worker
        echo
        
        echo "=== Worker Container Logs (last 10 lines each) ==="
        # Get all worker container names
        WORKER_CONTAINERS=$(docker-compose -f config/dev/docker-compose.yaml ps -q worker)
        if [ -n "$WORKER_CONTAINERS" ]; then
            for container_id in $WORKER_CONTAINERS; do
                CONTAINER_NAME=$(docker inspect --format='{{.Name}}' "$container_id" | sed 's/^.//')
                echo "--- $CONTAINER_NAME ---"
                docker logs --tail=10 "$container_id" 2>/dev/null || echo "Could not get logs for $CONTAINER_NAME"
                echo
            done
        else
            echo "No worker containers found"
        fi
        
        # Check Redis queue length
        echo "=== Redis Queue Status ==="
        REDIS_CONTAINER=$(docker-compose -f config/dev/docker-compose.yaml ps -q cache)
        if [ -n "$REDIS_CONTAINER" ]; then
            echo "Python execution queue length:"
            docker exec "$REDIS_CONTAINER" redis-cli LLEN python_execution_queue 2>/dev/null || echo "Could not connect to Redis"
            echo
            echo "Recent queue activity (last 10 items):"
            docker exec "$REDIS_CONTAINER" redis-cli LRANGE python_execution_queue 0 9 2>/dev/null || echo "Could not fetch queue items"
        else
            echo "Redis container not found"
        fi
        ;;
    
    prod|stage)
        echo "Monitoring $ENVIRONMENT environment..."
        
        # Check if kubectl is available
        if ! command -v kubectl &> /dev/null; then
            echo "Error: kubectl is not installed or not in PATH"
            exit 1
        fi
        
        # Set the appropriate minikube profile
        MINIKUBE_PROFILE="minikube-${ENVIRONMENT}"
        NAMESPACE="$ENVIRONMENT"
        
        echo "Using minikube profile: $MINIKUBE_PROFILE"
        echo "Using namespace: $NAMESPACE"
        echo
        
        echo "=== Worker Deployment Status ==="
        kubectl get deployment worker \
            --namespace="$NAMESPACE" \
            --context="$MINIKUBE_PROFILE" \
            -o wide 2>/dev/null || echo "Worker deployment not found"
        echo
        
        echo "=== Worker Pods Status ==="
        kubectl get pods -l app=worker \
            --namespace="$NAMESPACE" \
            --context="$MINIKUBE_PROFILE" \
            -o wide 2>/dev/null || echo "No worker pods found"
        echo
        
        echo "=== Worker Pod Resource Usage ==="
        kubectl top pods -l app=worker \
            --namespace="$NAMESPACE" \
            --context="$MINIKUBE_PROFILE" 2>/dev/null || echo "Metrics not available (metrics-server may not be installed)"
        echo
        
        echo "=== Horizontal Pod Autoscaler Status ==="
        kubectl get hpa worker-hpa \
            --namespace="$NAMESPACE" \
            --context="$MINIKUBE_PROFILE" 2>/dev/null || echo "HPA not found"
        echo
        
        echo "=== Recent Worker Events ==="
        kubectl get events --namespace="$NAMESPACE" --context="$MINIKUBE_PROFILE" \
            --field-selector="involvedObject.kind=Pod" \
            --sort-by='.lastTimestamp' | grep worker | tail -10 2>/dev/null || echo "No recent worker events"
        echo
        
        # Check Redis queue through a temporary pod
        echo "=== Redis Queue Status ==="
        echo "Checking queue length..."
        kubectl run redis-check --rm -i --restart=Never \
            --namespace="$NAMESPACE" \
            --context="$MINIKUBE_PROFILE" \
            --image=redis:alpine \
            --command -- redis-cli -h cache -p 6379 LLEN python_execution_queue 2>/dev/null || echo "Could not check Redis queue"
        ;;
    
    *)
        echo "Usage: $0 [dev|prod|stage]"
        echo "Examples:"
        echo "  $0 dev            # Monitor development workers"
        echo "  $0 prod           # Monitor production workers"
        echo "  $0 stage          # Monitor staging workers"
        exit 1
        ;;
esac

echo "=== Monitoring Complete ===" 