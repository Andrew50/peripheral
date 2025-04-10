#!/usr/bin/env bash
set -Eeuo pipefail

echo "Running database migrations..."

# Scale down deployments that might conflict with DB ops
DEPLOYMENTS_TO_SCALE=( "worker" "backend" "tf" )
for dep in "${DEPLOYMENTS_TO_SCALE[@]}"; do
  if kubectl get deployment "$dep" &>/dev/null; then
    echo "Scaling down $dep to 0 replicas..."
    kubectl scale deployment "$dep" --replicas=0 || true
  fi
done

# (Optional) Wait for pods to actually terminate
sleep 15

# Example: you might run an actual "migration job" here. Or you can do a "rollout restart" of the DB so it picks up new migrations on startup.

if kubectl get deployment db &>/dev/null; then
  echo "Rolling out DB changes by restarting db deployment..."
  kubectl rollout restart deployment/db
else
  echo "No 'db' deployment found; possibly first-time deployment."
fi

# Wait for DB to become ready
echo "Waiting for DB pod to become ready..."
kubectl wait --for=condition=available deployment/db --timeout=180s || \
  echo "WARNING: DB did not become available in time."

# Scale services back up
for dep in "${DEPLOYMENTS_TO_SCALE[@]}"; do
  if kubectl get deployment "$dep" &>/dev/null; then
    # Example: We scale them back to 1, or detect the original number from an env var, etc.
    # For simplicity, scale to 1
    echo "Scaling up $dep to 1..."
    kubectl scale deployment "$dep" --replicas=1
  fi
done

echo "Database migrations complete."
