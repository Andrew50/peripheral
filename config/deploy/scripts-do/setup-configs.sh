#!/usr/bin/env bash
set -Eeuo pipefail

# --- Environment Variable Validation ---
#: "${K8S_NAMESPACE:?Error: K8S_NAMESPACE environment variable is required.}" 

# Require these secrets:
: "${DB_ROOT_PASSWORD:?Missing DB_ROOT_PASSWORD}"
: "${REDIS_PASSWORD:?Missing REDIS_PASSWORD}"
: "${POLYGON_API_KEY:?Missing POLYGON_API_KEY}"
: "${POLYGON_S3_KEY:?Missing POLYGON_S3_KEY}"
: "${POLYGON_S3_SECRET:?Missing POLYGON_S3_SECRET}"
: "${GEMINI_FREE_KEYS:?Missing GEMINI_FREE_KEYS}"
: "${GEMINI_API_KEY:?Missing GEMINI_API_KEY}"
: "${OPENAI_API_KEY:?Missing OPENAI_API_KEY}"
: "${GROK_API_KEY:?Missing GROK_API_KEY}"
: "${TWITTER_API_IO_KEY:?Missing TWITTER_API_IO_KEY}"
: "${X_API_KEY:?Missing X_API_KEY}"
: "${X_API_SECRET:?Missing X_API_SECRET}"
: "${X_ACCESS_TOKEN:?Missing X_ACCESS_TOKEN}"
: "${X_ACCESS_SECRET:?Missing X_ACCESS_SECRET}"
: "${GOOGLE_CLIENT_ID:?Missing GOOGLE_CLIENT_ID}"
: "${GOOGLE_CLIENT_SECRET:?Missing GOOGLE_CLIENT_SECRET}"
: "${JWT_SECRET:?Missing JWT_SECRET}"
: "${CLOUDFLARE_TUNNEL_TOKEN:?Missing CLOUDFLARE_TUNNEL_TOKEN}"
: "${STRIPE_SECRET_KEY:?Missing STRIPE_SECRET_KEY}"
: "${STRIPE_WEBHOOK_SECRET:?Missing STRIPE_WEBHOOK_SECRET}"
: "${STRIPE_PUBLISHABLE_KEY:?Missing STRIPE_PUBLISHABLE_KEY}"

# Optional Telegram secrets (for monitoring alerts) - already base64 encoded
TELEGRAM_BOT_TOKEN_B64=${TELEGRAM_BOT_TOKEN_B64:-} 
TELEGRAM_CHAT_ID_B64=${TELEGRAM_CHAT_ID_B64:-} 
: "${CONFIG_DIR:?Missing CONFIG_DIR}" 
: "${TMP_DIR:?Missing TMP_DIR}"
: "${INGRESS_HOST:?Error: INGRESS_HOST environment variable is required.}"
: "${DOCKER_USERNAME:?Error: DOCKER_USERNAME environment variable is required.}"
: "${DOCKER_TAG:?Error: DOCKER_TAG environment variable is required.}"

# Encode secrets
DB_B64=$(echo -n "$DB_ROOT_PASSWORD" | base64 -w 0)
REDIS_B64=$(echo -n "$REDIS_PASSWORD" | base64 -w 0)
POLYGON_B64=$(echo -n "$POLYGON_API_KEY" | base64 -w 0)
POLYGON_S3_KEY_B64=$(echo -n "$POLYGON_S3_KEY" | base64 -w 0)
POLYGON_S3_SECRET_B64=$(echo -n "$POLYGON_S3_SECRET" | base64 -w 0)
GEMINI_B64=$(echo -n "$GEMINI_FREE_KEYS" | base64 -w 0)
GEMINI_API_B64=$(echo -n "$GEMINI_API_KEY" | base64 -w 0)
OPENAI_B64=$(echo -n "$OPENAI_API_KEY" | base64 -w 0)
GROK_B64=$(echo -n "$GROK_API_KEY" | base64 -w 0)
TWITTER_API_IO_KEY_B64=$(echo -n "$TWITTER_API_IO_KEY" | base64 -w 0)
X_API_KEY_B64=$(echo -n "$X_API_KEY" | base64 -w 0)
X_API_SECRET_B64=$(echo -n "$X_API_SECRET" | base64 -w 0)
X_ACCESS_TOKEN_B64=$(echo -n "$X_ACCESS_TOKEN" | base64 -w 0)
X_ACCESS_SECRET_B64=$(echo -n "$X_ACCESS_SECRET" | base64 -w 0)
GOOGLE_ID_B64=$(echo -n "$GOOGLE_CLIENT_ID" | base64 -w 0)
GOOGLE_SECRET_B64=$(echo -n "$GOOGLE_CLIENT_SECRET" | base64 -w 0)
OPENAI_B64=$(echo -n "$OPENAI_API_KEY" | base64 -w 0)
JWT_B64=$(echo -n "$JWT_SECRET" | base64 -w 0)
CLOUDFLARE_TOKEN_B64=$(echo -n "$CLOUDFLARE_TUNNEL_TOKEN" | base64 -w 0)
STRIPE_SECRET_B64=$(echo -n "$STRIPE_SECRET_KEY" | base64 -w 0)
STRIPE_WEBHOOK_B64=$(echo -n "$STRIPE_WEBHOOK_SECRET" | base64 -w 0)
STRIPE_PUBLISHABLE_B64=$(echo -n "$STRIPE_PUBLISHABLE_KEY" | base64 -w 0)

# Telegram secrets are already base64 encoded from GitHub secrets
# No need to encode them again

export DB_B64 REDIS_B64 POLYGON_B64 POLYGON_S3_KEY_B64 POLYGON_S3_SECRET_B64 GEMINI_B64 GEMINI_API_B64 OPENAI_B64 GROK_B64 TWITTER_API_IO_KEY_B64 X_API_KEY_B64 X_API_SECRET_B64 X_ACCESS_TOKEN_B64 X_ACCESS_SECRET_B64 GOOGLE_ID_B64 GOOGLE_SECRET_B64 JWT_B64 CLOUDFLARE_TOKEN_B64 TELEGRAM_BOT_TOKEN_B64 TELEGRAM_CHAT_ID_B64 STRIPE_SECRET_B64 STRIPE_WEBHOOK_B64 STRIPE_PUBLISHABLE_B64


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

echo "Verifying template substitution completed..."
if grep -r '\${' "$TMP_DIR" 2>/dev/null; then
    echo "Warning: Found unsubstituted variables in processed files:"
    grep -r '\${' "$TMP_DIR" || true
    echo "This may indicate missing environment variables."
fi

echo "Setup all k8s configs"

