#!/usr/bin/env bash
set -euo pipefail

# Script to scale worker instances
# Usage: ./scripts/scale-workers.sh [dev|prod|stage] [number_of_workers]

ENVIRONMENT=${1:-dev}
WORKER_COUNT=${2:-3}

case $ENVIRONMENT in
    dev)
        echo "Scaling workers in development environment..."
        
        # Check if docker-compose is available
        if ! command -v docker-compose &> /dev/null; then
            echo "Error: docker-compose is not installed or not in PATH"
            exit 1
        fi
        
        echo "Scaling worker service to $WORKER_COUNT replicas..."
        docker-compose -f config/dev/docker-compose.yaml up -d --scale worker="$WORKER_COUNT"
        
        echo "Current worker status:"
        docker-compose -f config/dev/docker-compose.yaml ps worker
        
        echo "Worker scaling completed successfully!"
        ;;
    
    prod|stage)
        echo "Scaling workers in $ENVIRONMENT environment..."
        
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
        
        # Scale the deployment
        echo "Scaling worker deployment to $WORKER_COUNT replicas..."
        kubectl scale deployment worker \
            --replicas="$WORKER_COUNT" \
            --namespace="$NAMESPACE" \
            --context="$MINIKUBE_PROFILE"
        
        # Wait for the scaling to complete
        echo "Waiting for scaling to complete..."
        kubectl rollout status deployment/worker \
            --namespace="$NAMESPACE" \
            --context="$MINIKUBE_PROFILE" \
            --timeout=300s
        
        # Show current status
        echo "Current worker status:"
        kubectl get pods -l app=worker \
            --namespace="$NAMESPACE" \
            --context="$MINIKUBE_PROFILE"
        
        echo "Worker scaling completed successfully!"
        ;;
    
    *)
        echo "Usage: $0 [dev|prod|stage] [number_of_workers]"
        echo "Examples:"
        echo "  $0 dev 5          # Scale dev workers (manual process)"
        echo "  $0 prod 8         # Scale prod workers to 8 replicas"
        echo "  $0 stage 4        # Scale stage workers to 4 replicas"
        exit 1
        ;;
esac 