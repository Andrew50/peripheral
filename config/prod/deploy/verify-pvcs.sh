#!/usr/bin/env bash
set -Eeuo pipefail

ENVIRONMENT="${1:-}"

echo "Verifying PersistentVolumeClaims for environment: $ENVIRONMENT"

# Example path for PV/PVC files. Adjust as needed:
PV_FILES=$(find "config/$ENVIRONMENT" -type f -name "*pv*.yaml" 2>/dev/null || echo "")
PVC_FILES=$(find "config/$ENVIRONMENT" -type f -name "*pvc*.yaml" 2>/dev/null || echo "")

if [[ -z "$PV_FILES" && -z "$PVC_FILES" ]]; then
  echo "No PV/PVC files found for environment '$ENVIRONMENT'. Skipping..."
  exit 0
fi

if [[ -n "$PV_FILES" ]]; then
  echo "Applying PV files:"
  for file in $PV_FILES; do
    echo "  -> $file"
    kubectl apply -f "$file"
  done
fi

# Wait a few seconds to ensure PVs are registered
sleep 5

if [[ -n "$PVC_FILES" ]]; then
  echo "Applying PVC files:"
  for file in $PVC_FILES; do
    echo "  -> $file"
    kubectl apply -f "$file"
  done

  # (Optional) Wait for PVCs to bind:
  echo "Waiting up to 60s for PVCs to become Bound..."
  PVC_NAMES=$(kubectl get pvc -o jsonpath='{.items[*].metadata.name}')
  for pvc in $PVC_NAMES; do
    kubectl wait --for=condition=Bound pvc/"$pvc" --timeout=60s || \
      echo "WARNING: PVC $pvc not bound within 60s"
  done
fi

echo "PVC verification complete."
#!/usr/bin/env bash
set -Eeuo pipefail

ENVIRONMENT="${1:-}"

echo "Verifying PersistentVolumeClaims for environment: $ENVIRONMENT"

# Example path for PV/PVC files. Adjust as needed:
PV_FILES=$(find "config/$ENVIRONMENT" -type f -name "*pv*.yaml" 2>/dev/null || echo "")
PVC_FILES=$(find "config/$ENVIRONMENT" -type f -name "*pvc*.yaml" 2>/dev/null || echo "")

if [[ -z "$PV_FILES" && -z "$PVC_FILES" ]]; then
  echo "No PV/PVC files found for environment '$ENVIRONMENT'. Skipping..."
  exit 0
fi

if [[ -n "$PV_FILES" ]]; then
  echo "Applying PV files:"
  for file in $PV_FILES; do
    echo "  -> $file"
    kubectl apply -f "$file"
  done
fi

# Wait a few seconds to ensure PVs are registered
sleep 5

if [[ -n "$PVC_FILES" ]]; then
  echo "Applying PVC files:"
  for file in $PVC_FILES; do
    echo "  -> $file"
    kubectl apply -f "$file"
  done

  # (Optional) Wait for PVCs to bind:
  echo "Waiting up to 60s for PVCs to become Bound..."
  PVC_NAMES=$(kubectl get pvc -o jsonpath='{.items[*].metadata.name}')
  for pvc in $PVC_NAMES; do
    kubectl wait --for=condition=Bound pvc/"$pvc" --timeout=60s || \
      echo "WARNING: PVC $pvc not bound within 60s"
  done
fi

echo "PVC verification complete."
