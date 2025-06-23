#!/usr/bin/env bash
set -Eeuo pipefail

# --- Environment Variable Validation ---
: "${DOCKER_TAG:?Error: DOCKER_TAG environment variable is required.}"
: "${K8S_NAMESPACE:?Error: K8S_NAMESPACE environment variable is required.}"
: "${SERVICES:?Error: SERVICES environment variable (space-separated list) is required.}"
: "${DOCKER_USERNAME:?Error: DOCKER_USERNAME environment variable is required.}"
: "${TMP_DIR:?Error: TMP_DIR not set}"


# Convert the space-separated string of services into a bash array
read -r -a SERVICES_ARRAY <<< "$SERVICES"


echo "Deploying to Kubernetes with tag: $DOCKER_TAG, namespace: $K8S_NAMESPACE"
echo "Temporary directory: $TMP_DIR"
echo "Target services: ${SERVICES_ARRAY[@]}"
echo "Using Docker user: $DOCKER_USERNAME"


# 4. Apply all YAML files from the temporary directory
echo "Applying configurations from $TMP_DIR to namespace ${K8S_NAMESPACE}..."
if ! kubectl apply -f "$TMP_DIR" --recursive --validate=false --namespace="${K8S_NAMESPACE}"; then
  echo "Error: kubectl apply failed."
  # Consider whether to exit here or allow potential cleanup steps later
  exit 1
fi

echo "Forcing a fresh rollout because we reuse the :main tag"
for dep in "${SERVICES_ARRAY[@]}"; do
  kubectl rollout restart deployment/"$dep" \
    --namespace="$K8S_NAMESPACE"
done

# 5. Verify PVCs are correctly bound
echo "Verifying PersistentVolumeClaims in namespace ${K8S_NAMESPACE}..."

# Check if any PVCs exist in the namespace
PVC_COUNT=$(kubectl get pvc --namespace="${K8S_NAMESPACE}" -o name 2>/dev/null | wc -l)

if [[ "$PVC_COUNT" -gt 0 ]]; then
  echo "Found ${PVC_COUNT} PVCs in namespace ${K8S_NAMESPACE}. Verifying binding status..."
  
  # Get all PVC names
  PVC_NAMES=$(kubectl get pvc --namespace="${K8S_NAMESPACE}" -o jsonpath='{.items[*].metadata.name}')
  
  # Wait for PVCs to bind
  echo "Waiting up to 120s for PVCs to become Bound..."
  for pvc in $PVC_NAMES; do
    echo "Waiting for PVC: $pvc"
    bound=false
    for i in {1..24}; do # Check every 5 seconds for 120 seconds (24 * 5 = 120)
      status=$(kubectl get pvc "$pvc" --namespace="${K8S_NAMESPACE}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Error")
      if [[ "$status" == "Bound" ]]; then
        echo "PVC $pvc is Bound."
        bound=true
        break
      elif [[ "$status" == "Error" ]]; then
         echo "Error getting status for PVC $pvc. Retrying..."
      else
        echo "PVC $pvc status is $status. Waiting... ($i/24)"
      fi
      sleep 5
    done

    if [[ "$bound" != true ]]; then
      echo "WARNING: PVC $pvc did not become Bound within 120s. Checking final status..."
      kubectl describe pvc "$pvc" --namespace="${K8S_NAMESPACE}"
      # Continue despite warning - don't fail the deployment
    fi
  done
else
  echo "No PVCs found in namespace ${K8S_NAMESPACE}. Skipping PVC verification."
fi

# 6. Wait for deployments to complete
echo "Waiting for deployments to complete..."
for dep in "${SERVICES_ARRAY[@]}"; do
  echo "Checking rollout status for deployment: $dep in namespace ${K8S_NAMESPACE}"
  
  # First, check if the deployment exists
  if ! kubectl get deployment "${dep}" --namespace="${K8S_NAMESPACE}" &>/dev/null; then
    echo "Warning: Deployment ${dep} not found in namespace ${K8S_NAMESPACE}. Skipping rollout check."
    continue
  fi
  
  # Set maximum attempts and timeout per attempt
  MAX_ATTEMPTS=5  # Total of 12 attempts
  TIMEOUT_PER_ATTEMPT="1m"  # 1 minute per attempt (total 12 minutes max)
  
  # Try rollout status with multiple short attempts
  success=false
  for attempt in $(seq 1 $MAX_ATTEMPTS); do
    echo "Attempt $attempt/$MAX_ATTEMPTS for deployment: $dep"
    
    if kubectl rollout status "deployment/${dep}" --namespace="${K8S_NAMESPACE}" --timeout="${TIMEOUT_PER_ATTEMPT}"; then
      echo "Deployment ${dep} successfully rolled out on attempt $attempt."
      success=true
      break
    else
      echo "Rollout not complete after attempt $attempt. Checking deployment status..."
      
      # Show deployment status after each attempt
      echo "Current deployment status for ${dep}:"
      kubectl get deployment "${dep}" --namespace="${K8S_NAMESPACE}" -o wide
      
      # Show pod status
      echo "Current pod status for ${dep}:"
      POD_SELECTOR=$(kubectl get deployment "${dep}" --namespace="${K8S_NAMESPACE}" -o jsonpath='{.spec.selector.matchLabels.app}')
      kubectl get pods --namespace="${K8S_NAMESPACE}" -l "app=${POD_SELECTOR}" -o wide
      
      # If this is the last attempt, we'll do more detailed diagnostics
      if [[ $attempt -eq $MAX_ATTEMPTS ]]; then
        break
      fi
      
      echo "Waiting before next attempt..."
      sleep 10
    fi
  done
  
  # If all attempts failed, gather detailed diagnostics
  if [[ "$success" != true ]]; then
    echo "Error: Deployment rollout failed for service: $dep after $MAX_ATTEMPTS attempts"
    
    # Get more diagnostic information
    echo "Detailed deployment status:"
    kubectl describe deployment "${dep}" --namespace="${K8S_NAMESPACE}" 
    # Get detailed pod information
    echo "Detailed pod status for deployment ${dep}:"
    POD_SELECTOR=$(kubectl get deployment "${dep}" --namespace="${K8S_NAMESPACE}" -o jsonpath='{.spec.selector.matchLabels.app}')
    kubectl get pods --namespace="${K8S_NAMESPACE}" -l "app=${POD_SELECTOR}" -o wide
    
    # Get logs from failing pods
    echo "Checking logs from failing pods:"
    FAILING_PODS=$(kubectl get pods --namespace="${K8S_NAMESPACE}" -l "app=${POD_SELECTOR}" -o jsonpath='{.items[?(@.status.phase!="Running")].metadata.name}')
    if [[ -n "$FAILING_PODS" ]]; then
      for pod in $FAILING_PODS; do
        echo "=== Logs for pod $pod ==="
        # Check if the pod has init containers
        INIT_CONTAINERS=$(kubectl get pod "$pod" --namespace="${K8S_NAMESPACE}" -o jsonpath='{.spec.initContainers[*].name}' 2>/dev/null)
        if [[ -n "$INIT_CONTAINERS" ]]; then
          for init_container in $INIT_CONTAINERS; do
            echo "--- Init container $init_container logs ---"
            kubectl logs "$pod" --namespace="${K8S_NAMESPACE}" -c "$init_container" --previous 2>/dev/null || kubectl logs "$pod" --namespace="${K8S_NAMESPACE}" -c "$init_container"
          done
        fi
        
        # Get logs from the main container
        echo "--- Main container logs ---"
        kubectl logs "$pod" --namespace="${K8S_NAMESPACE}" --previous 2>/dev/null || kubectl logs "$pod" --namespace="${K8S_NAMESPACE}"
      done
    else
      echo "No failing pods found, checking events instead"
    fi
    
    echo "Recent pod events:"
    kubectl get events --namespace="${K8S_NAMESPACE}" --field-selector="involvedObject.kind=Pod" | grep -i "${dep}" | tail -20
    
    # Fail the deployment process for all services consistently
    echo "ERROR: Deployment ${dep} failed to roll out after $MAX_ATTEMPTS attempts."
    echo "You can check its status later with: kubectl rollout status deployment/${dep} -n ${K8S_NAMESPACE}"
    exit 1
  fi
done

# 7. Verify services are accessible
echo "Verifying services are accessible..."
for dep in "${SERVICES_ARRAY[@]}"; do
  # Check if a service exists for this deployment
  if kubectl get service "${dep}" --namespace="${K8S_NAMESPACE}" &>/dev/null; then
    echo "Service ${dep} exists. Checking endpoints..."
    ENDPOINTS=$(kubectl get endpoints "${dep}" --namespace="${K8S_NAMESPACE}" -o jsonpath='{.subsets[*].addresses}')
    if [[ -z "$ENDPOINTS" ]]; then
      echo "WARNING: Service ${dep} has no endpoints. Pods may not be ready or labeled correctly."
      kubectl describe service "${dep}" --namespace="${K8S_NAMESPACE}" 
      kubectl describe endpoints "${dep}" --namespace="${K8S_NAMESPACE}"
    else
      echo "Service ${dep} has active endpoints."
    fi
  else
    echo "No service found for ${dep}. Skipping service verification."
  fi
done

# 8. Check for any pods in error state
echo "Checking for pods in error state..."
ERROR_PODS=$(kubectl get pods --namespace="${K8S_NAMESPACE}" --field-selector="status.phase!=Running,status.phase!=Succeeded" -o name)
if [[ -n "$ERROR_PODS" ]]; then
  echo "WARNING: Found pods not in Running/Succeeded state:"
  echo "$ERROR_PODS"
  for pod in $ERROR_PODS; do
    echo "Details for $pod:"
    kubectl describe "$pod" --namespace="${K8S_NAMESPACE}" 
  done
  # Don't fail deployment, just warn
else
  echo "All pods are in Running or Succeeded state."
fi

# The temporary directory cleanup is handled by a subsequent script.

echo "Deploy-to-K8s script complete. All deployments successful in namespace ${K8S_NAMESPACE}."
