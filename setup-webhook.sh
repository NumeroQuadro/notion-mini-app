#!/bin/bash

# Setup Telegram Bot Webhook for receiving reactions
# This script configures the Telegram Bot API to send updates to your webhook endpoint

# Load environment variables from .env file
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Check if required variables are set
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "Error: TELEGRAM_BOT_TOKEN is not set in .env file"
    exit 1
fi

if [ -z "$WEBHOOK_URL" ]; then
    echo "Error: WEBHOOK_URL is not set in .env file"
    echo "Please set WEBHOOK_URL=https://your-domain.com/telegram/webhook in .env"
    exit 1
fi

echo "Setting up webhook for Telegram bot..."
echo "Webhook URL: $WEBHOOK_URL"

# Set the webhook
RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/setWebhook" \
    -H "Content-Type: application/json" \
    -d "{\"url\":\"${WEBHOOK_URL}\",\"allowed_updates\":[\"message\",\"message_reaction\"]}")

echo "Response from Telegram API:"
echo $RESPONSE | jq . 2>/dev/null || echo $RESPONSE

# Check webhook info
echo ""
echo "Checking webhook info..."
curl -s "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo" | jq . 2>/dev/null || \
    curl -s "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo"

echo ""
echo "Webhook setup complete!"
echo ""
echo "To remove the webhook and use polling instead, run:"
echo "  curl -X POST https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/deleteWebhook"
