#!/bin/bash

# Production start script for Notion Telegram Bot
# This script ensures everything is properly configured before starting

set -e  # Exit on error

echo "🚀 Starting Notion Telegram Bot (Production Mode)"
echo "=================================================="
echo ""

# 1. Check if .env exists
if [ ! -f .env ]; then
    echo "❌ Error: .env file not found!"
    echo "Please create .env file with required variables."
    echo "See .env.example for reference."
    exit 1
fi

# 2. Load environment variables
export $(cat .env | grep -v '^#' | xargs)

# 3. Validate required environment variables
echo "1️⃣  Validating environment variables..."
MISSING_VARS=()

[ -z "$TELEGRAM_BOT_TOKEN" ] && MISSING_VARS+=("TELEGRAM_BOT_TOKEN")
[ -z "$NOTION_API_KEY" ] && MISSING_VARS+=("NOTION_API_KEY")
[ -z "$NOTION_TASKS_DATABASE_ID" ] && MISSING_VARS+=("NOTION_TASKS_DATABASE_ID")
[ -z "$WEBHOOK_URL" ] && MISSING_VARS+=("WEBHOOK_URL")
[ -z "$AUTHORIZED_USER_ID" ] && MISSING_VARS+=("AUTHORIZED_USER_ID")

if [ ${#MISSING_VARS[@]} -gt 0 ]; then
    echo "❌ Missing required environment variables:"
    for var in "${MISSING_VARS[@]}"; do
        echo "   - $var"
    done
    exit 1
fi

echo "   ✅ All required variables are set"
echo ""

# 4. Check if binary exists
echo "2️⃣  Checking bot binary..."
if [ ! -f "./notion-bot" ]; then
    echo "   ⚠️  Binary not found. Building..."
    go build -o notion-bot cmd/main.go
    echo "   ✅ Build complete"
else
    echo "   ✅ Binary found"
fi
echo ""

# 5. Verify webhook is configured
echo "3️⃣  Checking Telegram webhook..."
WEBHOOK_INFO=$(curl -s "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo")

if echo "$WEBHOOK_INFO" | grep -q "\"url\":\"$WEBHOOK_URL\""; then
    echo "   ✅ Webhook is configured correctly"
    
    # Check for errors
    if echo "$WEBHOOK_INFO" | grep -q "last_error_message"; then
        echo "   ⚠️  Warning: Webhook has errors:"
        echo "$WEBHOOK_INFO" | jq -r '.result.last_error_message' 2>/dev/null || echo "Unknown error"
    fi
else
    echo "   ⚠️  Webhook not configured or URL mismatch"
    echo "   Running setup-webhook.sh..."
    ./setup-webhook.sh
fi
echo ""

# 6. Check nginx
echo "4️⃣  Checking nginx..."
if systemctl is-active --quiet nginx 2>/dev/null; then
    echo "   ✅ Nginx is running"
    
    if grep -q "/telegram/webhook" /etc/nginx/nginx.conf 2>/dev/null; then
        echo "   ✅ Webhook endpoint configured in nginx"
    else
        echo "   ⚠️  Webhook endpoint not found in nginx config"
        echo "   Run: sudo ./add-webhook-to-nginx.sh"
    fi
else
    echo "   ⚠️  Nginx is not running or status unknown"
fi
echo ""

# 7. Stop any existing instance
echo "5️⃣  Checking for existing bot processes..."
if pgrep -f "notion-bot" > /dev/null; then
    echo "   ⚠️  Found existing process, stopping..."
    pkill -f "notion-bot"
    sleep 2
fi
echo "   ✅ Ready to start"
echo ""

# 8. Start the bot
echo "6️⃣  Starting bot..."
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Bot is starting. Press Ctrl+C to stop."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Run the bot
./notion-bot
