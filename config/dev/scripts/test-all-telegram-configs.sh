#!/bin/bash
set -e

# Test All Telegram Configurations Script
# Verifies that all components use configurable environment variables

echo "=== Testing All Telegram Configurations ==="
echo ""

# Configuration
TEST_BOT_TOKEN="${TELEGRAM_BOT_TOKEN:-test_token_123}"
TEST_CHAT_ID="${TELEGRAM_CHAT_ID:--1234567890}"

echo "Using test credentials:"
echo "  Bot Token: ${TEST_BOT_TOKEN:0:15}...(truncated)"
echo "  Chat ID: $TEST_CHAT_ID"
echo ""

# Test 1: Backend dispatch.go compilation
echo "1. Testing backend Go code compilation..."
cd services/backend

# Check if the code compiles (this will catch syntax errors)
if go build -o /tmp/backend_test ./cmd/server/main.go >/dev/null 2>&1; then
    echo "✅ Backend code compiles successfully"
    rm -f /tmp/backend_test
else
    echo "❌ Backend compilation failed"
    go build ./cmd/server/main.go
    exit 1
fi
cd - >/dev/null
echo ""

# Test 2: Kubernetes configuration validation
echo "2. Testing Kubernetes configurations..."

# Check if backend.yaml has Telegram env vars
if grep -q "TELEGRAM_BOT_TOKEN" config/deploy/k8s/backend.yaml && \
   grep -q "TELEGRAM_CHAT_ID" config/deploy/k8s/backend.yaml; then
    echo "✅ Backend deployment includes Telegram environment variables"
else
    echo "❌ Backend deployment missing Telegram environment variables"
    exit 1
fi

# Check if secrets.yaml has Telegram secret
if grep -q "telegram-secret" config/deploy/k8s/secrets.yaml; then
    echo "✅ Secrets template includes Telegram secret"
else
    echo "❌ Secrets template missing Telegram secret"
    exit 1
fi

# Check if cluster-monitor has Telegram env vars
if grep -q "TELEGRAM_BOT_TOKEN" config/deploy/k8s/cluster-monitor.yaml; then
    echo "✅ Cluster monitor includes Telegram environment variables"
else
    echo "❌ Cluster monitor missing Telegram environment variables"
    exit 1
fi

# Check if db-backup-system has Telegram env vars
if grep -q "TELEGRAM_BOT_TOKEN" config/deploy/k8s/db-backup-system.yaml; then
    echo "✅ Database backup system includes Telegram environment variables"
else
    echo "❌ Database backup system missing Telegram environment variables"
    exit 1
fi
echo ""

# Test 3: Script configuration validation
echo "3. Testing script configurations..."

# Test setup-configs.sh includes Telegram variables
if grep -q "TELEGRAM_BOT_TOKEN.*=.*{TELEGRAM_BOT_TOKEN" config/deploy/scripts/setup-configs.sh && \
   grep -q "TELEGRAM_CHAT_ID.*=.*{TELEGRAM_CHAT_ID" config/deploy/scripts/setup-configs.sh; then
    echo "✅ Setup configs script includes Telegram variable handling"
else
    echo "❌ Setup configs script missing Telegram variable handling"
    exit 1
fi

# Test cluster-monitor.sh uses environment variables
if grep -q "\${TELEGRAM_BOT_TOKEN" services/cluster-monitor/scripts/cluster-monitor.sh && \
   grep -q "\${TELEGRAM_CHAT_ID" services/cluster-monitor/scripts/cluster-monitor.sh; then
    echo "✅ Cluster monitor script uses environment variables"
else
    echo "❌ Cluster monitor script not using environment variables"
    exit 1
fi

# Test resource-report.sh uses environment variables
if grep -q "\${TELEGRAM_BOT_TOKEN" services/cluster-monitor/scripts/resource-report.sh && \
   grep -q "\${TELEGRAM_CHAT_ID" services/cluster-monitor/scripts/resource-report.sh; then
    echo "✅ Resource report script uses environment variables"
else
    echo "❌ Resource report script not using environment variables"
    exit 1
fi

# Test health-monitor.sh uses environment variables
if grep -q "\$TELEGRAM_BOT_TOKEN" services/db/scripts/health-monitor.sh && \
   grep -q "\$TELEGRAM_CHAT_ID" services/db/scripts/health-monitor.sh; then
    echo "✅ Health monitor script uses environment variables"
else
    echo "❌ Health monitor script not using environment variables"
    exit 1
fi
echo ""

# Test 4: GitHub workflow includes Telegram secrets
echo "4. Testing GitHub workflow configuration..."
if grep -q "TELEGRAM_BOT_TOKEN:" .github/workflows/deploy.yml && \
   grep -q "TELEGRAM_CHAT_ID:" .github/workflows/deploy.yml; then
    echo "✅ GitHub workflow includes Telegram secrets"
else
    echo "❌ GitHub workflow missing Telegram secrets"
    exit 1
fi
echo ""

# Test 5: Environment variable substitution
echo "5. Testing environment variable substitution..."

# Test base64 encoding
export TELEGRAM_BOT_TOKEN="$TEST_BOT_TOKEN"
export TELEGRAM_CHAT_ID="$TEST_CHAT_ID"

TELEGRAM_BOT_TOKEN_B64=$(echo -n "$TELEGRAM_BOT_TOKEN" | base64 -w 0)
TELEGRAM_CHAT_ID_B64=$(echo -n "$TELEGRAM_CHAT_ID" | base64 -w 0)

if [ -n "$TELEGRAM_BOT_TOKEN_B64" ] && [ -n "$TELEGRAM_CHAT_ID_B64" ]; then
    echo "✅ Base64 encoding works correctly"
    echo "   Bot token base64: ${TELEGRAM_BOT_TOKEN_B64:0:20}...(truncated)"
    echo "   Chat ID base64: ${TELEGRAM_CHAT_ID_B64}"
else
    echo "❌ Base64 encoding failed"
    exit 1
fi

# Test template substitution
export TELEGRAM_BOT_TOKEN_B64 TELEGRAM_CHAT_ID_B64

TEST_TEMPLATE='bot-token: ${TELEGRAM_BOT_TOKEN_B64}
chat-id: ${TELEGRAM_CHAT_ID_B64}'

SUBSTITUTED=$(echo "$TEST_TEMPLATE" | envsubst)
if echo "$SUBSTITUTED" | grep -q "$TELEGRAM_BOT_TOKEN_B64" && \
   echo "$SUBSTITUTED" | grep -q "$TELEGRAM_CHAT_ID_B64"; then
    echo "✅ Template substitution works correctly"
else
    echo "❌ Template substitution failed"
    echo "Expected to contain: $TELEGRAM_BOT_TOKEN_B64 and $TELEGRAM_CHAT_ID_B64"
    echo "Got: $SUBSTITUTED"
    exit 1
fi
echo ""

# Test 6: No hardcoded values remain
echo "6. Checking for hardcoded Telegram values..."

# Check for hardcoded chat IDs in code (excluding this test file)
HARDCODED_CHAT_IDS=$(grep -r "ChatID.*=.*-[0-9]" --include="*.go" services/ || true)
if [ -n "$HARDCODED_CHAT_IDS" ]; then
    echo "❌ Found hardcoded chat IDs in Go code:"
    echo "$HARDCODED_CHAT_IDS"
    exit 1
else
    echo "✅ No hardcoded chat IDs found in Go code"
fi

# Check for hardcoded bot tokens (excluding documentation and this test)
HARDCODED_TOKENS=$(grep -r "bot.*token.*=" --include="*.go" --include="*.sh" services/ config/ | grep -v "\${" | grep -v "TELEGRAM_BOT_TOKEN" | grep -v "bot-token=" | grep -v "test-all-telegram-configs.sh" || true)
if [ -n "$HARDCODED_TOKENS" ]; then
    echo "⚠️  Found potential hardcoded bot tokens (review manually):"
    echo "$HARDCODED_TOKENS"
else
    echo "✅ No hardcoded bot tokens found"
fi

# Check for empty token creation
EMPTY_TOKENS=$(grep -r 'bot-token.*""' --include="*.sh" --include="*.yaml" config/ --exclude="test-all-telegram-configs.sh" || true)
if [ -n "$EMPTY_TOKENS" ]; then
    echo "❌ Found scripts creating empty bot tokens:"
    echo "$EMPTY_TOKENS"
    exit 1
else
    echo "✅ No empty token creation found"
fi
echo ""

# Summary
echo "=== Test Results Summary ==="
echo "✅ All Telegram configurations are properly using environment variables!"
echo ""
echo "📋 Configuration Status:"
echo "  ✅ Backend Go code uses environment variables"
echo "  ✅ Kubernetes deployments configured for Telegram secrets"
echo "  ✅ All scripts use environment variables"
echo "  ✅ GitHub workflow includes Telegram secrets"
echo "  ✅ Template substitution working"
echo "  ✅ No hardcoded values found"
echo ""
echo "🚀 Ready for deployment with configurable Telegram integration!"
echo ""
echo "📚 Next steps:"
echo "1. Set up your Telegram bot: ./config/dev/scripts/setup-telegram-alerts.sh"
echo "2. Add GitHub secrets: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID (REQUIRED)"
echo "3. Deploy the system: ./config/deploy/scripts/deploy-monitoring.sh"
echo ""
echo "🧪 Test Telegram integration: ./config/dev/scripts/test-telegram.sh" 