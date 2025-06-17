#!/bin/bash
set -e

# Simple Telegram Bot Test Script
# Tests bot connectivity and sends a test message

echo "=== Telegram Bot Test Script ==="
echo ""

# Check if bot token and chat ID are provided as arguments or environment variables
BOT_TOKEN="${1:-${TELEGRAM_BOT_TOKEN:-}}"
CHAT_ID="${2:-${TELEGRAM_CHAT_ID:-}}"

if [ -z "$BOT_TOKEN" ] || [ -z "$CHAT_ID" ]; then
    echo "Usage: $0 <BOT_TOKEN> <CHAT_ID>"
    echo "   or set TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID environment variables"
    echo ""
    echo "To get these values:"
    echo "1. Create a bot: Message @BotFather on Telegram and send /newbot"
    echo "2. Get chat ID: Send a message to your bot, then visit:"
    echo "   https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates"
    echo ""
    exit 1
fi

echo "Testing with:"
echo "  Bot Token: ${BOT_TOKEN:0:10}...(truncated)"
echo "  Chat ID: $CHAT_ID"
echo ""

# Test 1: Check if Telegram API is accessible
echo "1. Testing Telegram API accessibility..."
API_RESPONSE=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/getMe" || echo "ERROR")

if echo "$API_RESPONSE" | grep -q '"ok":true'; then
    BOT_NAME=$(echo "$API_RESPONSE" | grep -o '"first_name":"[^"]*"' | cut -d'"' -f4)
    BOT_USERNAME=$(echo "$API_RESPONSE" | grep -o '"username":"[^"]*"' | cut -d'"' -f4)
    echo "✅ Bot connection successful!"
    echo "   Bot Name: $BOT_NAME"
    echo "   Bot Username: @$BOT_USERNAME"
else
    echo "❌ Bot connection failed!"
    echo "Response: $API_RESPONSE"
    exit 1
fi
echo ""

# Test 2: Send a test message
echo "2. Sending test message..."
TEST_MESSAGE="🧪 *Telegram Bot Test*

This is a test message from your cluster monitoring system.

*Time:* $(date '+%Y-%m-%d %H:%M:%S UTC')
*Status:* Connection successful ✅

If you receive this message, your Telegram integration is working correctly!

*Next steps:*
• Deploy the cluster monitoring system
• Configure GitHub secrets for automatic deployment
• Monitor your cluster health in real-time"

SEND_RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot$BOT_TOKEN/sendMessage" \
    -H "Content-Type: application/json" \
    -d "{
        \"chat_id\": \"$CHAT_ID\",
        \"text\": \"$TEST_MESSAGE\",
        \"parse_mode\": \"Markdown\",
        \"disable_web_page_preview\": true
    }")

if echo "$SEND_RESPONSE" | grep -q '"ok":true'; then
    echo "✅ Test message sent successfully!"
    echo "   Check your Telegram to confirm you received it."
else
    echo "❌ Failed to send test message!"
    echo "Response: $SEND_RESPONSE"
    exit 1
fi
echo ""

# Test 3: Update Kubernetes secret if we're in a cluster context
echo "3. Checking Kubernetes context..."
if command -v kubectl >/dev/null 2>&1; then
    if kubectl cluster-info >/dev/null 2>&1; then
        echo "✅ Kubernetes context available"
        
        # Ask if user wants to update the secret
        echo "Would you like to update the Kubernetes telegram-secret? (y/n)"
        read -r UPDATE_SECRET
        
        if [[ "$UPDATE_SECRET" =~ ^[Yy]$ ]]; then
            echo "Updating telegram-secret in current namespace..."
            
            # Get current namespace
            CURRENT_NAMESPACE=$(kubectl config view --minify --output 'jsonpath={..namespace}')
            CURRENT_NAMESPACE=${CURRENT_NAMESPACE:-default}
            
            # Create or update the secret
            kubectl create secret generic telegram-secret \
                --from-literal=bot-token="$BOT_TOKEN" \
                --from-literal=chat-id="$CHAT_ID" \
                --namespace="$CURRENT_NAMESPACE" \
                --dry-run=client -o yaml | kubectl apply -f -
            
            echo "✅ Secret updated in namespace: $CURRENT_NAMESPACE"
        else
            echo "Skipped secret update"
        fi
    else
        echo "⚠️  Kubernetes context not available or not connected"
    fi
else
    echo "⚠️  kubectl not found"
fi
echo ""

echo "=== Test Complete ==="
echo ""
echo "📋 Summary:"
echo "  ✅ Telegram API is accessible"
echo "  ✅ Bot connection working"
echo "  ✅ Message sending functional"
echo ""
echo "🚀 Ready to deploy monitoring system!"
echo "Run: ./config/deploy/scripts/deploy-monitoring.sh"
echo ""
echo "📚 For GitHub Actions integration, add these secrets:"
echo "  TELEGRAM_BOT_TOKEN: $BOT_TOKEN"
echo "  TELEGRAM_CHAT_ID: $CHAT_ID" 