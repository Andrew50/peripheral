#!/usr/bin/env bash
set -Eeuo pipefail

# --- Environment Variable Validation ---
: "${DOCKER_TAG:?Error: DOCKER_TAG environment variable is required.}"
: "${K8S_NAMESPACE:?Error: K8S_NAMESPACE environment variable is required.}"
: "${SERVICES:?Error: SERVICES environment variable (space-separated list) is required.}"
: "${DOCKER_USERNAME:?Error: DOCKER_USERNAME environment variable is required.}"

# Convert the space-separated string of services into a bash array
read -r -a SERVICES_ARRAY <<< "$SERVICES"

SOURCE_DIR="config/deploy/k8s"
TMP_DIR="config/deploy/tmp"

echo "Deploying to Kubernetes with tag: $DOCKER_TAG, namespace: $K8S_NAMESPACE"
echo "Source directory: $SOURCE_DIR"
echo "Temporary directory: $TMP_DIR"
echo "Target services: ${SERVICES_ARRAY[@]}"
echo "Using Docker user: $DOCKER_USERNAME"

# 1. Check if source directory exists
if [[ ! -d "$SOURCE_DIR" ]]; then
  echo "Error: Source directory '$SOURCE_DIR' not found."
  exit 1
fi

# 2. Prepare temporary directory
echo "Preparing temporary directory: $TMP_DIR"
rm -rf "$TMP_DIR"
mkdir -p "$TMP_DIR"
# Copy contents of source dir to temp dir
cp -r "$SOURCE_DIR"/* "$TMP_DIR/"

# Remove the secrets template file from the temp directory as it's handled separately
echo "Removing secrets template from temporary directory..."
rm -f "$TMP_DIR/secrets.yaml"

# 3. Update image tags in temporary YAML files
echo "Updating image tags in temporary files..."
for dep in "${SERVICES_ARRAY[@]}"; do
  echo "Processing service: $dep"
  # Find all yaml files in the temp directory
  find "$TMP_DIR" -type f \( -name "*.yaml" -o -name "*.yml" \) -print0 | while IFS= read -r -d $'\0' file; do
    # Use sed to replace the image tag.
    # This assumes the image format is '<some-path>/<service-name>:<some-tag>'
    # and replaces it with '$DOCKER_USERNAME/$dep:$DOCKER_TAG'.
    # It targets lines starting with optional spaces followed by 'image:',
    # containing '/<service-name>:' later in the line.
    # Note: This sed command is based on common conventions but might be fragile
    # if YAML structure or image naming deviates significantly.
    sed -i -E "s|^( *)image:.*[/]${dep}:.*$|\1image: ${DOCKER_USERNAME}/${dep}:${DOCKER_TAG}|g" "$file"
  done
done
echo "Image tag update complete."

# 4. Apply all YAML files from the temporary directory
echo "Applying configurations from $TMP_DIR to namespace ${K8S_NAMESPACE}..."
if ! kubectl apply -f "$TMP_DIR" --recursive --validate=false --namespace="${K8S_NAMESPACE}"; then
  echo "Error: kubectl apply failed."
  # Consider whether to exit here or allow potential cleanup steps later
  exit 1
fi

# 5. Verify PVCs are correctly bound
echo "Verifying PersistentVolumeClaims in namespace ${K8S_NAMESPACE}..."

# Check if any PVCs exist in the namespace
PVC_COUNT=$(kubectl get pvc --namespace="${K8S_NAMESPACE}" -o name 2>/dev/null | wc -l)

if [[ "$PVC_COUNT" -gt 0 ]]; then
  echo "Found ${PVC_COUNT} PVCs in namespace ${K8S_NAMESPACE}. Verifying binding status..."
  
  # Get all PVC names
  PVC_NAMES=$(kubectl get pvc --namespace="${K8S_NAMESPACE}" -o jsonpath='{.items[*].metadata.name}')
  
  # Wait for PVCs to bind
  echo "Waiting up to 60s for PVCs to become Bound..."
  for pvc in $PVC_NAMES; do
    echo "Waiting for PVC: $pvc"
    if ! kubectl wait --for=condition=Bound pvc/"$pvc" --namespace="${K8S_NAMESPACE}" --timeout=60s; then
      echo "WARNING: PVC $pvc not bound within 60s. Checking status..."
      kubectl describe pvc "$pvc" --namespace="${K8S_NAMESPACE}"
      # Continue despite warning - don't fail the deployment
    else
      echo "PVC $pvc successfully bound"
    fi
  done
else
  echo "No PVCs found in namespace ${K8S_NAMESPACE}. Skipping PVC verification."
fi

# 6. Wait for deployments to complete
echo "Waiting for deployments to complete..."
for dep in "${SERVICES_ARRAY[@]}"; do
  echo "Checking rollout status for deployment: $dep in namespace ${K8S_NAMESPACE}"
  if ! kubectl rollout status "deployment/${dep}" --namespace="${K8S_NAMESPACE}" --timeout=5m; then
    echo "Error: Deployment rollout failed for service: $dep"
    # Get more diagnostic information
    echo "Deployment status:"
    kubectl describe deployment "${dep}" --namespace="${K8S_NAMESPACE}"
    echo "Recent pod events:"
    POD_SELECTOR=$(kubectl get deployment "${dep}" --namespace="${K8S_NAMESPACE}" -o jsonpath='{.spec.selector.matchLabels}' | tr -d '{}' | sed 's/:/=/g')
    kubectl get events --namespace="${K8S_NAMESPACE}" --field-selector="involvedObject.kind=Pod" | grep "$POD_SELECTOR" | tail -10
    exit 1
  fi
  echo "Deployment ${dep} successfully rolled out."
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
