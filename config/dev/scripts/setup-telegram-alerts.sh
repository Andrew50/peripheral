#!/bin/bash
set -e

# Telegram Alerts Setup Script for Database Backup & Recovery System
# This script helps configure Telegram notifications for backup and health monitoring

echo "=== Telegram Alerts Setup for Database Backup & Recovery ==="
echo ""

# Check if we're in the right directory
if [ ! -f "services/db/Dockerfile.dev" ]; then
    echo "Error: Please run this script from the project root directory"
    exit 1
fi

echo "This script will help you set up Telegram alerts for your database backup system."
echo ""

# Instructions for creating a bot
echo "📱 Step 1: Create a Telegram Bot"
echo "1. Open Telegram and search for @BotFather"
echo "2. Send /start to BotFather"
echo "3. Send /newbot and follow the instructions"
echo "4. Choose a name and username for your bot"
echo "5. Copy the bot token (it looks like: 123456789:ABCdefGHIjklMNOpqrsTUVwxyz)"
echo ""

# Get bot token
while true; do
    read -p "Enter your Telegram bot token: " BOT_TOKEN
    if [[ $BOT_TOKEN =~ ^[0-9]+:[A-Za-z0-9_-]+$ ]]; then
        break
    else
        echo "Invalid bot token format. Please try again."
    fi
done

echo ""
echo "📱 Step 2: Get Chat ID"
echo "You can send alerts to:"
echo "  • A private chat with your bot"
echo "  • A group chat (add your bot to the group)"
echo "  • A channel (add your bot as admin)"
echo ""

# Test bot and get chat info
echo "Testing bot connection..."
RESPONSE=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/getMe")
if echo "$RESPONSE" | grep -q '"ok":true'; then
    BOT_NAME=$(echo "$RESPONSE" | grep -o '"first_name":"[^"]*"' | cut -d'"' -f4)
    BOT_USERNAME=$(echo "$RESPONSE" | grep -o '"username":"[^"]*"' | cut -d'"' -f4)
    echo "✅ Bot connection successful!"
    echo "   Bot Name: $BOT_NAME"
    echo "   Bot Username: @$BOT_USERNAME"
else
    echo "❌ Failed to connect to bot. Please check your token."
    exit 1
fi

echo ""
echo "To get your chat ID:"
echo "1. Send a message to your bot: @$BOT_USERNAME"
echo "2. Or add the bot to your group/channel"
echo "3. Send any message (like 'hello')"
echo ""
read -p "Press Enter after you've sent a message to the bot..."

# Get updates to find chat ID
echo ""
echo "Getting chat information..."
UPDATES=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/getUpdates")

if echo "$UPDATES" | grep -q '"chat"'; then
    echo "Found the following chats:"
    echo "$UPDATES" | grep -o '"chat":{"id":[^,]*,"[^"]*":"[^"]*"' | while read chat; do
        CHAT_ID=$(echo "$chat" | grep -o '"id":[^,]*' | cut -d':' -f2)
        CHAT_TYPE=$(echo "$chat" | grep -o '"type":"[^"]*"' | cut -d'"' -f4)
        CHAT_TITLE=$(echo "$chat" | grep -o '"title":"[^"]*"' | cut -d'"' -f4 2>/dev/null || echo "Private Chat")
        echo "  Chat ID: $CHAT_ID ($CHAT_TYPE - $CHAT_TITLE)"
    done
    echo ""
else
    echo "No messages found. Please send a message to your bot first."
    exit 1
fi

# Get chat ID from user
while true; do
    read -p "Enter the Chat ID you want to use for alerts: " CHAT_ID
    if [[ $CHAT_ID =~ ^-?[0-9]+$ ]]; then
        break
    else
        echo "Invalid chat ID format. Should be a number (can be negative)."
    fi
done

echo ""
echo "🧪 Step 3: Testing Telegram Alert"

# Test message
TEST_MESSAGE="🧪 *Database Backup System Test*

This is a test message from your PostgreSQL backup and recovery system.

*System:* Test Environment
*Time:* $(date '+%Y-%m-%d %H:%M:%S UTC')

If you receive this message, Telegram alerts are working correctly! ✅

*Next Steps:*
• Deploy the backup system with Telegram integration
• Monitor for backup success/failure notifications
• Get alerted for database health issues"

echo "Sending test message..."
RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot$BOT_TOKEN/sendMessage" \
    -H "Content-Type: application/json" \
    -d "{
        \"chat_id\": \"$CHAT_ID\",
        \"text\": \"$TEST_MESSAGE\",
        \"parse_mode\": \"Markdown\",
        \"disable_web_page_preview\": true
    }")

if echo "$RESPONSE" | grep -q '"ok":true'; then
    echo "✅ Test message sent successfully!"
    echo "   Check your Telegram to confirm you received it."
else
    echo "❌ Failed to send test message:"
    echo "$RESPONSE"
    exit 1
fi

echo ""
echo "🔐 Step 4: Creating Kubernetes Secret"

# Create base64 encoded values
BOT_TOKEN_B64=$(echo -n "$BOT_TOKEN" | base64 -w 0)
CHAT_ID_B64=$(echo -n "$CHAT_ID" | base64 -w 0)

# Create secret file
cat > telegram-secret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: telegram-secret
  labels:
    app: db-backup-system
type: Opaque
data:
  bot-token: $BOT_TOKEN_B64
  chat-id: $CHAT_ID_B64
EOF

echo "Created telegram-secret.yaml with your credentials."
echo ""

# Apply the secret
echo "🚀 Step 5: Applying Configuration"
echo "Applying Telegram secret to Kubernetes..."
kubectl apply -f telegram-secret.yaml

# Clean up the secret file for security
rm -f telegram-secret.yaml
echo "Secret applied and temporary file removed for security."

echo ""
echo "✅ Telegram alerts setup complete!"
echo ""
echo "📋 Configuration Summary:"
echo "  Bot: @$BOT_USERNAME"
echo "  Chat ID: $CHAT_ID"
echo "  Secret: telegram-secret (applied to Kubernetes)"
echo ""
echo "🔄 Next Steps:"
echo "1. Redeploy the backup system to use Telegram alerts:"
echo "   ./config/dev/scripts/deploy-backup-system.sh"
echo ""
echo "2. You will now receive notifications for:"
echo "   • Backup successes/failures"
echo "   • Database health issues"
echo "   • Recovery operations"
echo "   • System alerts"
echo ""
echo "📱 Alert Types:"
echo "   🚨 CRITICAL - Database failures requiring immediate attention"
echo "   ❌ ERROR - Backup failures or system errors"
echo "   ⚠️  WARNING - Potential issues or warnings"
echo "   ✅ SUCCESS - Backup completions and recovery successes"
echo "   ℹ️  INFO - General information and status updates" 