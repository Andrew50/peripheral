#!/usr/bin/env bash
set -Eeuo pipefail

K8S_NAMESPACE="${1:-}"

# Require these secrets:
: "${DB_ROOT_PASSWORD:?Missing DB_ROOT_PASSWORD}"
: "${REDIS_PASSWORD:?Missing REDIS_PASSWORD}"
: "${POLYGON_API_KEY:?Missing POLYGON_API_KEY}"
: "${GEMINI_FREE_KEYS:?Missing GEMINI_FREE_KEYS}"
: "${GOOGLE_CLIENT_ID:?Missing GOOGLE_CLIENT_ID}"
: "${GOOGLE_CLIENT_SECRET:?Missing GOOGLE_CLIENT_SECRET}"
: "${JWT_SECRET:?Missing JWT_SECRET}"

echo "Updating Kubernetes Secrets in namespace: ${K8S_NAMESPACE}..."

# Encode secrets
DB_B64=$(echo -n "$DB_ROOT_PASSWORD" | base64 -w 0)
REDIS_B64=$(echo -n "$REDIS_PASSWORD" | base64 -w 0)
POLYGON_B64=$(echo -n "$POLYGON_API_KEY" | base64 -w 0)
GEMINI_B64=$(echo -n "$GEMINI_FREE_KEYS" | base64 -w 0)
GOOGLE_ID_B64=$(echo -n "$GOOGLE_CLIENT_ID" | base64 -w 0)
GOOGLE_SECRET_B64=$(echo -n "$GOOGLE_CLIENT_SECRET" | base64 -w 0)
JWT_B64=$(echo -n "$JWT_SECRET" | base64 -w 0)

TMP_DIR="$(mktemp -d)"
cat <<EOF > "$TMP_DIR/secrets.yaml"
apiVersion: v1
kind: Secret
metadata:
  name: db-secret
type: Opaque
data:
  DB_ROOT_PASSWORD: $DB_B64
---
apiVersion: v1
kind: Secret
metadata:
  name: redis-secret
type: Opaque
data:
  REDIS_PASSWORD: $REDIS_B64
---
apiVersion: v1
kind: Secret
metadata:
  name: polygon-secret
type: Opaque
data:
  api-key: $POLYGON_B64
---
apiVersion: v1
kind: Secret
metadata:
  name: gemini-secret
type: Opaque
data:
  GEMINI_FREE_KEYS: $GEMINI_B64
---
apiVersion: v1
kind: Secret
metadata:
  name: google-oauth-secret
type: Opaque
data:
  GOOGLE_CLIENT_ID: $GOOGLE_ID_B64
  GOOGLE_CLIENT_SECRET: $GOOGLE_SECRET_B64
---
apiVersion: v1
kind: Secret
metadata:
  name: jwt-secret
type: Opaque
data:
  JWT_SECRET: $JWT_B64
EOF

echo "Applying secrets to namespace ${K8S_NAMESPACE}..."
kubectl apply -f "$TMP_DIR/secrets.yaml" --validate=false --namespace=${K8S_NAMESPACE}

rm -rf "$TMP_DIR"

# (Optional) rollout restart to pick up new secrets in certain deployments
if kubectl get deployment backend --namespace=${K8S_NAMESPACE} &>/dev/null; then
  kubectl rollout restart deployment/backend --namespace=${K8S_NAMESPACE}
fi
if kubectl get deployment worker --namespace=${K8S_NAMESPACE} &>/dev/null; then
  kubectl rollout restart deployment/worker --namespace=${K8S_NAMESPACE}
fi

echo "Secrets updated in namespace ${K8S_NAMESPACE}."
