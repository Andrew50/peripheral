#!/bin/bash

# Script to fix Redis deployment issues in Kubernetes

echo "Checking Redis deployment status..."
kubectl get pods -l app=cache

# Check if there are multiple Redis pods running
REDIS_PODS=$(kubectl get pods -l app=cache -o name | wc -l)

if [ "$REDIS_PODS" -gt 1 ]; then
    echo "Multiple Redis pods detected. This may cause connection issues."
    echo "Cleaning up Redis deployment..."
    
    # Scale down the deployment to 0 replicas
    echo "Scaling down Redis deployment to 0 replicas..."
    kubectl scale deployment cache --replicas=0
    
    # Wait for pods to terminate
    echo "Waiting for Redis pods to terminate..."
    kubectl wait --for=delete pods -l app=cache --timeout=60s
    
    # Scale back up to 1 replica
    echo "Scaling Redis deployment back to 1 replica..."
    kubectl scale deployment cache --replicas=1
    
    # Wait for the pod to be ready
    echo "Waiting for Redis pod to be ready..."
    kubectl wait --for=condition=ready pods -l app=cache --timeout=120s
else
    echo "Single Redis pod detected. Checking its status..."
    
    # Check if the pod is in CrashLoopBackOff
    CRASH_LOOP=$(kubectl get pods -l app=cache -o jsonpath='{.items[0].status.containerStatuses[0].state.waiting.reason}' | grep -c "CrashLoopBackOff")
    
    if [ "$CRASH_LOOP" -eq 1 ]; then
        echo "Redis pod is in CrashLoopBackOff state. Restarting the pod..."
        
        # Delete the pod to force a restart
        POD_NAME=$(kubectl get pods -l app=cache -o jsonpath='{.items[0].metadata.name}')
        kubectl delete pod $POD_NAME
        
        # Wait for the new pod to be ready
        echo "Waiting for new Redis pod to be ready..."
        kubectl wait --for=condition=ready pods -l app=cache --timeout=120s
    else
        echo "Redis pod appears to be running normally."
    fi
fi

# Check Redis service
echo "Checking Redis service..."
kubectl get service cache

# Verify Redis connectivity from worker pod
echo "Verifying Redis connectivity from worker pod..."
WORKER_POD=$(kubectl get pods -l app=worker -o jsonpath='{.items[0].metadata.name}')
kubectl exec $WORKER_POD -- redis-cli -h cache ping

echo "Redis deployment check complete."
echo "If issues persist, consider checking Redis logs:"
echo "kubectl logs -l app=cache"
echo "Or restart the worker pod:"
echo "kubectl delete pod -l app=worker" 