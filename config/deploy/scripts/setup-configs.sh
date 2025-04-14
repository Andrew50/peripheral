#!/usr/bin/env bash
set -Eeuo pipefail

# --- Environment Variable Validation ---
#: "${K8S_NAMESPACE:?Error: K8S_NAMESPACE environment variable is required.}"

# Require these secrets:
: "${DB_ROOT_PASSWORD:?Missing DB_ROOT_PASSWORD}"
: "${REDIS_PASSWORD:?Missing REDIS_PASSWORD}"
: "${POLYGON_API_KEY:?Missing POLYGON_API_KEY}"
: "${GEMINI_FREE_KEYS:?Missing GEMINI_FREE_KEYS}"
: "${GOOGLE_CLIENT_ID:?Missing GOOGLE_CLIENT_ID}"
: "${GOOGLE_CLIENT_SECRET:?Missing GOOGLE_CLIENT_SECRET}"
: "${JWT_SECRET:?Missing JWT_SECRET}"
: "${CLOUDFLARE_TUNNEL_TOKEN:?Missing CLOUDFLARE_TUNNEL_TOKEN}"
: "${CONFIG_DIR:?Missing CONFIG_DIR}"
: "${TMP_DIR:?Missing TMP_DIR}"
: "${INGRESS_HOST:?Error: INGRESS_HOST environment variable is required.}"
: "${DOCKER_USERNAME:?Error: DOCKER_USERNAME environment variable is required.}"
: "${DOCKER_TAG:?Error: DOCKER_TAG environment variable is required.}"

#echo "Updating Kubernetes Secrets in namespace: ${K8S_NAMESPACE}..."
# Encode secrets
DB_B64=$(echo -n "$DB_ROOT_PASSWORD" | base64 -w 0)
REDIS_B64=$(echo -n "$REDIS_PASSWORD" | base64 -w 0)
POLYGON_B64=$(echo -n "$POLYGON_API_KEY" | base64 -w 0)
GEMINI_B64=$(echo -n "$GEMINI_FREE_KEYS" | base64 -w 0)
GOOGLE_ID_B64=$(echo -n "$GOOGLE_CLIENT_ID" | base64 -w 0)
GOOGLE_SECRET_B64=$(echo -n "$GOOGLE_CLIENT_SECRET" | base64 -w 0)
JWT_B64=$(echo -n "$JWT_SECRET" | base64 -w 0)
CLOUDFLARE_TOKEN_B64=$(echo -n "$CLOUDFLARE_TUNNEL_TOKEN" | base64 -w 0)

#TMP_DIR="$(mktemp -d)"
#INGRESS_YAML_FILE="${TMP_DIR}/ingress.yaml"
#INGRESS_YML_FILE="${TMP_DIR}/ingress.yml"


if [[ ! -d "$CONFIG_DIR" ]]; then
  echo "Error: Source directory '$CONFIG_DIR' not found."
  exit 1
fi

echo "Preparing temporary directory: $TMP_DIR"
rm -rf "$TMP_DIR"
mkdir -p "$TMP_DIR"
# Copy contents of source dir to temp dir
cp -r "$CONFIG_DIR"/* "$TMP_DIR/"

for file in "$TMP_DIR"/*; do
    if [ -d "$file" ]; then
        continue
    fi

    echo "Processing: $file"
    
    # Check if this is the secrets.yaml file - if so, substitute all variables
    #if [[ "$(basename "$file")" == "secrets.yaml" ]]; then
        #echo "  - Full substitution for secrets file"
        envsubst < "$file" > "${file}.tmp"
    #else
        # For all other files, only substitute DOCKER_USERNAME, DOCKER_TAG, and INGRESS_HOST
        #echo "  - Limited substitution (DOCKER_USERNAME, DOCKER_TAG, INGRESS_HOST only)"
        #envsubst '$DOCKER_USERNAME $DOCKER_TAG $INGRESS_HOST' < "$file" > "${file}.tmp"
    #fi
    
    mv "${file}.tmp" "$file"
done

echo "Setup all k8s configs"




# Remove the secrets template file from the temp directory as it's handled separately
#echo "Removing secrets template from temporary directory..."
#rm -f "$TMP_DIR/secrets.yaml"

# 3. Update image tags in temporary YAML files
#echo "Updating image tags in temporary files..."
#for dep in "${SERVICES_ARRAY[@]}"; do
#  echo "Processing service: $dep"
#  # Find all yaml files in the temp directory
#  find "$TMP_DIR" -type f \( -name "*.yaml" -o -name "*.yml" \) -print0 | while IFS= read -r -d $'\0' file; do
#    # Use sed to replace the image tag.
#    # This assumes the image format is '<some-path>/<service-name>:<some-tag>'
#    # and replaces it with '$DOCKER_USERNAME/$dep:$DOCKER_TAG'.
#    # It targets lines starting with optional spaces followed by 'image:',
#    # containing '/<service-name>:' later in the line.
#    # Note: This sed command is based on common conventions but might be fragile
#    # if YAML structure or image naming deviates significantly.
#    sed -i -E "s|^( *)image:.*[/]${dep}:.*$|\1image: ${DOCKER_USERNAME}/${dep}:${DOCKER_TAG}|g" "$file"
#  done
#done
#echo "Image tag update complete."
#
## 3.5 Substitute environment variables in specific files (e.g., Ingress host)
#echo "Substituting environment variables in configuration files..."
## Substitute INGRESS_HOST in ingress.yaml or ingress.yml
#
#if [[ -f "$INGRESS_YAML_FILE" ]]; then
#    echo "Substituting INGRESS_HOST in $INGRESS_YAML_FILE..."
#    sed -i "s|\${INGRESS_HOST}|${INGRESS_HOST}|g" "$INGRESS_YAML_FILE"
#elif [[ -f "$INGRESS_YML_FILE" ]]; then
#    echo "Substituting INGRESS_HOST in $INGRESS_YML_FILE..."
#    sed -i "s|\${INGRESS_HOST}|${INGRESS_HOST}|g" "$INGRESS_YML_FILE"
#else
#    echo "Warning: Neither ingress.yaml nor ingress.yml found in $TMP_DIR. Skipping INGRESS_HOST substitution."
#fi

#modify to instead base secrets.yaml insterad

#cat <<EOF > "$TMP_DIR/secrets.yaml"
#apiVersion: v1
#kind: Secret
#metadata:
#  name: db-secret
#type: Opaque
#data:
#  DB_ROOT_PASSWORD: $DB_B64
#---
#apiVersion: v1
#kind: Secret
#metadata:
#  name: redis-secret
#type: Opaque
#data:
#  REDIS_PASSWORD: $REDIS_B64
#---
#apiVersion: v1
#kind: Secret
#metadata:
#  name: polygon-secret
#type: Opaque
#data:
#  api-key: $POLYGON_B64
#---
#apiVersion: v1
#kind: Secret
#metadata:
#  name: gemini-secret
#type: Opaque
#data:
#  GEMINI_FREE_KEYS: $GEMINI_B64
#---
#apiVersion: v1
#kind: Secret
#metadata:
#  name: google-oauth-secret
#type: Opaque
#data:
#  GOOGLE_CLIENT_ID: $GOOGLE_ID_B64
#  GOOGLE_CLIENT_SECRET: $GOOGLE_SECRET_B64
#---
#apiVersion: v1
#kind: Secret
#metadata:
#  name: jwt-secret
#type: Opaque
#data:
#  JWT_SECRET: $JWT_B64
#---
#apiVersion: v1
#kind: Secret
#metadata:
#  name: cloudflare-secret
#type: Opaque
#data:
#  CLOUDFLARE_TUNNEL_TOKEN: $CLOUDFLARE_TOKEN_B64
#EOF
#
#echo "Created secrets config file"

# REST OF THIS SHOULD BE HANLDED IN deploy-to-k8s.sh

#echo "Applying secrets to namespace ${K8S_NAMESPACE}..."
#kubectl apply -f "$TMP_DIR/secrets.yaml" --validate=false --namespace=${K8S_NAMESPACE}

#rm -rf "$TMP_DIR"

# (Optional) rollout restart to pick up new secrets in certain deployments
#if kubectl get deployment backend --namespace=${K8S_NAMESPACE} &>/dev/null; then
  #kubectl rollout restart deployment/backend --namespace=${K8S_NAMESPACE}
#fi
#if kubectl get deployment worker --namespace=${K8S_NAMESPACE} &>/dev/null; then
  #kubectl rollout restart deployment/worker --namespace=${K8S_NAMESPACE}
#fi

#echo "Secrets updated in namespace ${K8S_NAMESPACE}."
