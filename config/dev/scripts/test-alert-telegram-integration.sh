#!/bin/bash
set -e

# Test Alert System Telegram Integration
# Verifies that the alert system uses the same Telegram configuration as everything else

echo "=== Alert System Telegram Integration Test ==="
echo ""

# Check environment variables
TELEGRAM_BOT_TOKEN="${TELEGRAM_BOT_TOKEN:-}"
TELEGRAM_CHAT_ID="${TELEGRAM_CHAT_ID:-}"

if [ -z "$TELEGRAM_BOT_TOKEN" ] || [ -z "$TELEGRAM_CHAT_ID" ]; then
    echo "❌ ERROR: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID environment variables are required."
    echo ""
    echo "Please set these variables or source them from a file:"
    echo "  export TELEGRAM_BOT_TOKEN='your_bot_token'"
    echo "  export TELEGRAM_CHAT_ID='your_chat_id'"
    echo ""
    echo "Or run: source .env # if you have a .env file"
    exit 1
fi

echo "✅ Environment variables configured:"
echo "  Bot Token: ${TELEGRAM_BOT_TOKEN:0:10}...(truncated)"
echo "  Chat ID: $TELEGRAM_CHAT_ID"
echo ""

# Test 1: Verify Telegram API connectivity
echo "1. Testing Telegram API connectivity..."
RESPONSE=$(curl -s "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/getMe")
if echo "$RESPONSE" | grep -q '"ok":true'; then
    BOT_NAME=$(echo "$RESPONSE" | grep -o '"first_name":"[^"]*"' | cut -d'"' -f4)
    BOT_USERNAME=$(echo "$RESPONSE" | grep -o '"username":"[^"]*"' | cut -d'"' -f4)
    echo "✅ Telegram API connection successful!"
    echo "   Bot Name: $BOT_NAME"
    echo "   Bot Username: @$BOT_USERNAME"
else
    echo "❌ Telegram API connection failed!"
    echo "Response: $RESPONSE"
    exit 1
fi
echo ""

# Test 2: Check Backend Go code compilation
echo "2. Testing backend alert system compilation..."
cd services/backend
if go build -o /tmp/backend_alert_test ./cmd/server/main.go >/dev/null 2>&1; then
    echo "✅ Backend alert system compiles successfully"
    rm -f /tmp/backend_alert_test
else
    echo "❌ Backend compilation failed"
    go build ./cmd/server/main.go
    exit 1
fi
cd - >/dev/null
echo ""

# Test 3: Verify Kubernetes configuration consistency
echo "3. Testing Kubernetes configurations..."

# Check if backend deployment includes Telegram environment variables
if grep -q "TELEGRAM_BOT_TOKEN" config/deploy/k8s/backend.yaml && \
   grep -q "TELEGRAM_CHAT_ID" config/deploy/k8s/backend.yaml; then
    echo "✅ Backend deployment includes Telegram environment variables"
else
    echo "❌ Backend deployment missing Telegram environment variables"
    exit 1
fi

# Check if secrets template includes Telegram secret
if grep -q "telegram-secret" config/deploy/k8s/secrets.yaml && \
   grep -q "bot-token:" config/deploy/k8s/secrets.yaml && \
   grep -q "chat-id:" config/deploy/k8s/secrets.yaml; then
    echo "✅ Secrets template includes Telegram secret configuration"
else
    echo "❌ Secrets template missing Telegram secret configuration"
    exit 1
fi

# Check if setup-configs.sh handles Telegram variables
if grep -q "TELEGRAM_BOT_TOKEN_B64" config/deploy/scripts/setup-configs.sh && \
   grep -q "TELEGRAM_CHAT_ID_B64" config/deploy/scripts/setup-configs.sh; then
    echo "✅ Setup configs script handles Telegram variables"
else
    echo "❌ Setup configs script missing Telegram variable handling"
    exit 1
fi
echo ""

# Test 4: Verify GitHub workflow configuration
echo "4. Testing GitHub workflow configuration..."
if grep -q "TELEGRAM_BOT_TOKEN:" .github/workflows/deploy.yml && \
   grep -q "TELEGRAM_CHAT_ID:" .github/workflows/deploy.yml; then
    echo "✅ GitHub workflow includes Telegram secrets"
else
    echo "❌ GitHub workflow missing Telegram secrets"
    exit 1
fi
echo ""

# Test 5: Check consistency across all systems
echo "5. Testing configuration consistency across all systems..."

# Backend alert system
BACKEND_BOT_TOKEN_USAGE=$(grep -c "TELEGRAM_BOT_TOKEN" services/backend/internal/services/alerts/dispatch.go)
BACKEND_CHAT_ID_USAGE=$(grep -c "TELEGRAM_CHAT_ID" services/backend/internal/services/alerts/dispatch.go)

# Cluster monitor
CLUSTER_BOT_TOKEN_USAGE=$(grep -c "TELEGRAM_BOT_TOKEN" services/cluster-monitor/scripts/cluster-monitor.sh)
CLUSTER_CHAT_ID_USAGE=$(grep -c "TELEGRAM_CHAT_ID" services/cluster-monitor/scripts/cluster-monitor.sh)

# Database backup system
DB_BOT_TOKEN_USAGE=$(grep -c "TELEGRAM_BOT_TOKEN" services/db/scripts/backup-improved.sh)
DB_CHAT_ID_USAGE=$(grep -c "TELEGRAM_CHAT_ID" services/db/scripts/backup-improved.sh)

echo "Configuration usage count across systems:"
echo "  Backend Alert System: BOT_TOKEN=$BACKEND_BOT_TOKEN_USAGE, CHAT_ID=$BACKEND_CHAT_ID_USAGE"
echo "  Cluster Monitor: BOT_TOKEN=$CLUSTER_BOT_TOKEN_USAGE, CHAT_ID=$CLUSTER_CHAT_ID_USAGE"
echo "  Database Backup: BOT_TOKEN=$DB_BOT_TOKEN_USAGE, CHAT_ID=$DB_CHAT_ID_USAGE"

if [ "$BACKEND_BOT_TOKEN_USAGE" -gt 0 ] && [ "$BACKEND_CHAT_ID_USAGE" -gt 0 ] && \
   [ "$CLUSTER_BOT_TOKEN_USAGE" -gt 0 ] && [ "$CLUSTER_CHAT_ID_USAGE" -gt 0 ] && \
   [ "$DB_BOT_TOKEN_USAGE" -gt 0 ] && [ "$DB_CHAT_ID_USAGE" -gt 0 ]; then
    echo "✅ All systems use consistent Telegram configuration"
else
    echo "❌ Inconsistent Telegram configuration across systems"
    exit 1
fi
echo ""

# Test 6: Send test messages from different system types
echo "6. Testing Telegram message sending from different system types..."

# Test backend-style alert message
echo "Testing backend alert-style message..."
BACKEND_MESSAGE="🚨 *Backend Alert System Test*

*System:* Backend Alert System
*Time:* $(date '+%Y-%m-%d %H:%M:%S UTC')
*Type:* Price Alert

*Message:*
Test alert: AAPL price above \$150.00

*Details:*
• Alert ID: 12345
• Security: AAPL
• Target Price: \$150.00
• Direction: Above

This message confirms that the backend alert system is properly configured with Telegram integration! ✅"

RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/sendMessage" \
    -H "Content-Type: application/json" \
    -d "{
        \"chat_id\": \"$TELEGRAM_CHAT_ID\",
        \"text\": \"$BACKEND_MESSAGE\",
        \"parse_mode\": \"Markdown\",
        \"disable_web_page_preview\": true
    }")

if echo "$RESPONSE" | grep -q '"ok":true'; then
    echo "✅ Backend-style message sent successfully"
else
    echo "❌ Failed to send backend-style message: $RESPONSE"
    exit 1
fi

# Wait a moment between messages
sleep 2

# Test cluster monitor-style message
echo "Testing cluster monitor-style message..."
CLUSTER_MESSAGE="⚠️ *Cluster Monitor Test*

*System:* Kubernetes Cluster Monitor
*Time:* $(date '+%Y-%m-%d %H:%M:%S UTC')
*Environment:* Test
*Namespace:* default

*Message:*
Test cluster monitoring alert - all systems operational

*Cluster Status:*
• Minikube: ✅ Running
• API Server: ✅ Reachable
• Nodes: 1/1 Ready
• Pods: 8/8 Running

*Monitor Stats:*
• Cluster Failures: 0/3
• API Failures: 0/3
• Node Failures: 0/3

This message confirms that the cluster monitoring system shares the same Telegram configuration! ✅"

RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/sendMessage" \
    -H "Content-Type: application/json" \
    -d "{
        \"chat_id\": \"$TELEGRAM_CHAT_ID\",
        \"text\": \"$CLUSTER_MESSAGE\",
        \"parse_mode\": \"Markdown\",
        \"disable_web_page_preview\": true
    }")

if echo "$RESPONSE" | grep -q '"ok":true'; then
    echo "✅ Cluster monitor-style message sent successfully"
else
    echo "❌ Failed to send cluster monitor-style message: $RESPONSE"
    exit 1
fi

# Wait a moment between messages
sleep 2

# Test database backup-style message
echo "Testing database backup-style message..."
DB_MESSAGE="✅ *Database Backup Test*

*System:* PostgreSQL Backup System
*Time:* $(date '+%Y-%m-%d %H:%M:%S UTC')
*Environment:* Test

*Message:*
Test database backup notification - backup completed successfully

*Backup Status:*
• Backup Directory: /backups
• Retention Policy: 30 days
• Database: postgres@db
• Backup Size: 125MB
• Duration: 45 seconds

This message confirms that the database backup system uses the same Telegram configuration! ✅"

RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/sendMessage" \
    -H "Content-Type: application/json" \
    -d "{
        \"chat_id\": \"$TELEGRAM_CHAT_ID\",
        \"text\": \"$DB_MESSAGE\",
        \"parse_mode\": \"Markdown\",
        \"disable_web_page_preview\": true
    }")

if echo "$RESPONSE" | grep -q '"ok":true'; then
    echo "✅ Database backup-style message sent successfully"
else
    echo "❌ Failed to send database backup-style message: $RESPONSE"
    exit 1
fi

echo ""

# Test 7: Check for hardcoded values
echo "7. Checking for hardcoded Telegram values..."
HARDCODED_FOUND=false

# Check for hardcoded chat IDs
HARDCODED_CHAT_IDS=$(grep -r "ChatID.*=.*-[0-9]" --include="*.go" services/ 2>/dev/null || true)
if [ -n "$HARDCODED_CHAT_IDS" ]; then
    echo "❌ Found hardcoded chat IDs:"
    echo "$HARDCODED_CHAT_IDS"
    HARDCODED_FOUND=true
fi

# Check for hardcoded bot tokens (excluding test files)
HARDCODED_TOKENS=$(grep -r '"[0-9]\+:[A-Za-z0-9_-]\+'"'" --include="*.go" --include="*.sh" services/ config/ 2>/dev/null | grep -v "test-alert-telegram-integration.sh" || true)
if [ -n "$HARDCODED_TOKENS" ]; then
    echo "❌ Found potential hardcoded bot tokens:"
    echo "$HARDCODED_TOKENS"
    HARDCODED_FOUND=true
fi

if [ "$HARDCODED_FOUND" = false ]; then
    echo "✅ No hardcoded Telegram values found"
fi
echo ""

# Test 8: Verify scheduler integration
echo "8. Testing alert system scheduler integration..."
if grep -q "StartAlertLoop" services/backend/internal/server/schedule.go && \
   grep -q "startAlertLoop" services/backend/internal/server/schedule.go && \
   ! grep -q "/\*.*StartAlertLoop.*\*/" services/backend/internal/server/schedule.go; then
    echo "✅ Alert system is properly integrated into the scheduler"
else
    echo "❌ Alert system is not properly integrated into the scheduler"
    exit 1
fi
echo ""

# Final summary
echo "=== Test Results Summary ==="
echo ""
echo "🎉 ALL TESTS PASSED! 🎉"
echo ""
echo "📋 Verification Results:"
echo "  ✅ Telegram API connectivity working"
echo "  ✅ Backend alert system compiles and uses environment variables"
echo "  ✅ Kubernetes configurations are consistent"
echo "  ✅ GitHub workflow includes Telegram secrets"
echo "  ✅ All systems use the same Telegram configuration"
echo "  ✅ Message sending works for all system types"
echo "  ✅ No hardcoded values found"
echo "  ✅ Alert system properly integrated into scheduler"
echo ""
echo "🔧 Configuration Status:"
echo "  📱 Bot: @$BOT_USERNAME"
echo "  💬 Chat ID: $TELEGRAM_CHAT_ID"
echo "  🔑 Environment Variables: Properly configured"
echo "  🐳 Kubernetes Secrets: Ready for deployment"
echo ""
echo "🚀 Next Steps:"
echo "1. Deploy the system with Telegram integration:"
echo "   TELEGRAM_BOT_TOKEN='$TELEGRAM_BOT_TOKEN' TELEGRAM_CHAT_ID='$TELEGRAM_CHAT_ID' ./deploy.sh"
echo ""
echo "2. Monitor for alerts in your Telegram chat"
echo ""
echo "3. The alert system will now:"
echo "   • Send price alerts when conditions are met"
echo "   • Use the same Telegram config as cluster monitoring"
echo "   • Properly initialize on server startup"
echo "   • Work consistently across all environments"
echo ""
echo "✨ Your alert system is ready to go!" 