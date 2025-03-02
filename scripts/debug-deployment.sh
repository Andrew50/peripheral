#!/bin/bash

# Debug Kubernetes Deployment Issues
# Usage: ./debug-deployment.sh [namespace]

NAMESPACE=${1:-default}
echo "Debugging deployments in namespace: $NAMESPACE"

# Check node status
echo "===== Node Status ====="
kubectl get nodes -o wide

# Check pod status
echo -e "\n===== Pod Status ====="
kubectl get pods -n $NAMESPACE

# Check deployment status
echo -e "\n===== Deployment Status ====="
kubectl get deployments -n $NAMESPACE

# Check service status
echo -e "\n===== Service Status ====="
kubectl get services -n $NAMESPACE

# Check persistent volume status
echo -e "\n===== PersistentVolume Status ====="
kubectl get pv,pvc -n $NAMESPACE

# Check events
echo -e "\n===== Recent Events ====="
kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp' | tail -n 20

# Check logs for failing pods
echo -e "\n===== Logs for Failing Pods ====="
FAILING_PODS=$(kubectl get pods -n $NAMESPACE | grep -v Running | grep -v Completed | awk '{if(NR>1)print $1}')
for pod in $FAILING_PODS; do
  echo -e "\n--- Logs for $pod ---"
  kubectl logs -n $NAMESPACE $pod --tail=50 || echo "Could not get logs for $pod"
  
  echo -e "\n--- Describe for $pod ---"
  kubectl describe pod -n $NAMESPACE $pod | grep -A 10 "Events:"
done

# Check resource usage
echo -e "\n===== Resource Usage ====="
kubectl top nodes || echo "Metrics server not available"
kubectl top pods -n $NAMESPACE || echo "Metrics server not available"

echo -e "\nDebugging completed. Check the output above for issues." 