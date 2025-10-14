#!/bin/bash

# Docker Diagnostics Script
# Helps identify why the container isn't running

echo "🔍 Docker Container Diagnostics"
echo "================================"
echo ""

# 1. Check if container exists (running or stopped)
echo "1️⃣  Checking container status..."
CONTAINER_ID=$(docker ps -a --filter "name=notion-mini-app" --format "{{.ID}}")

if [ -z "$CONTAINER_ID" ]; then
    echo "   ❌ No container found (running or stopped)"
    echo "   → Container likely crashed immediately after starting"
else
    echo "   ℹ️  Container found: $CONTAINER_ID"
    
    # Check if it's running
    if docker ps --filter "name=notion-mini-app" --format "{{.ID}}" | grep -q .; then
        echo "   ✅ Container IS running"
    else
        echo "   ❌ Container exists but NOT running (crashed)"
    fi
fi
echo ""

# 2. Get container logs (even if stopped)
echo "2️⃣  Checking container logs..."
if [ -n "$CONTAINER_ID" ]; then
    echo "   Last 50 lines of logs:"
    echo "   ─────────────────────────────────────────"
    docker logs --tail 50 notion-mini-app 2>&1 || echo "   No logs available"
    echo "   ─────────────────────────────────────────"
else
    echo "   ⚠️  No container to check logs from"
    echo "   → Try running: make docker-run"
fi
echo ""

# 3. Check last exited containers
echo "3️⃣  Checking recently exited containers..."
docker ps -a --filter "name=notion-mini-app" --format "table {{.ID}}\t{{.Status}}\t{{.Names}}"
echo ""

# 4. Check .env file
echo "4️⃣  Checking .env file..."
if [ -f .env ]; then
    echo "   ✅ .env file exists"
    echo "   Required variables:"
    
    [ -n "$(grep TELEGRAM_BOT_TOKEN .env)" ] && echo "   ✅ TELEGRAM_BOT_TOKEN" || echo "   ❌ TELEGRAM_BOT_TOKEN"
    [ -n "$(grep NOTION_API_KEY .env)" ] && echo "   ✅ NOTION_API_KEY" || echo "   ❌ NOTION_API_KEY"
    [ -n "$(grep WEBHOOK_URL .env)" ] && echo "   ✅ WEBHOOK_URL" || echo "   ❌ WEBHOOK_URL"
    [ -n "$(grep AUTHORIZED_USER_ID .env)" ] && echo "   ✅ AUTHORIZED_USER_ID" || echo "   ❌ AUTHORIZED_USER_ID"
else
    echo "   ❌ .env file NOT found!"
fi
echo ""

# 5. Check web directory
echo "5️⃣  Checking web directory..."
if [ -d web ]; then
    echo "   ✅ web/ directory exists"
    ls -la web/
else
    echo "   ❌ web/ directory missing!"
fi
echo ""

# 6. Try to start container and capture immediate output
echo "6️⃣  Attempting to start container in foreground (for debugging)..."
echo "   Press Ctrl+C to stop"
echo "   ─────────────────────────────────────────"

if [ -f .env ]; then
    docker run --rm --name notion-mini-app-debug \
        --env-file .env \
        -p 8081:8080 \
        notion-mini-app
else
    echo "   ❌ Cannot start: .env file missing"
fi
