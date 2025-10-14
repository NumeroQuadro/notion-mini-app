#!/bin/bash

# Delete Telegram Bot Webhook
# Use this script to remove the webhook and switch back to polling mode

# Load environment variables from .env file
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Check if required variables are set
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "❌ Error: TELEGRAM_BOT_TOKEN is not set in .env file"
    exit 1
fi

echo "🗑️  Deleting webhook for Telegram bot..."

# Delete the webhook
RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/deleteWebhook")

echo "Response from Telegram API:"
echo $RESPONSE | jq . 2>/dev/null || echo $RESPONSE

# Check if successful
if echo $RESPONSE | grep -q '"ok":true'; then
    echo ""
    echo "✅ Webhook deleted successfully!"
    echo ""
    echo "Bot is now ready to use polling mode."
    echo "Restart your bot to switch to polling."
    echo ""
    echo "⚠️  Note: Reactions will NOT work in polling mode."
    echo "To enable reactions again, run: ./setup-webhook.sh"
else
    echo ""
    echo "❌ Failed to delete webhook. Check the error above."
    exit 1
fi
