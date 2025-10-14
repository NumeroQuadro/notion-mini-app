#!/bin/bash

# Quick Docker Debug Script
# Runs the container in foreground to see crash logs

echo "🔍 Running Docker container in DEBUG mode"
echo "=========================================="
echo ""
echo "This will run the container in foreground mode"
echo "so you can see the exact error that causes it to crash."
echo ""
echo "Press Ctrl+C to stop when done."
echo ""
echo "=========================================="
echo ""

# Check if .env exists
if [ ! -f .env ]; then
    echo "❌ Error: .env file not found!"
    echo ""
    echo "Create .env with required variables:"
    echo "  TELEGRAM_BOT_TOKEN=your_token"
    echo "  NOTION_API_KEY=your_key"
    echo "  WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook"
    echo "  AUTHORIZED_USER_ID=your_user_id"
    echo ""
    exit 1
fi

# Clean up any existing containers
docker stop notion-mini-app-debug 2>/dev/null || true
docker rm notion-mini-app-debug 2>/dev/null || true

# Run in foreground (NOT detached)
docker run --rm --name notion-mini-app-debug \
    --env-file .env \
    -p 8081:8080 \
    notion-mini-app

echo ""
echo "Container stopped."
