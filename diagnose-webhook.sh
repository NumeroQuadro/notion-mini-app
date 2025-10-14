#!/bin/bash

echo "🔍 Webhook Diagnostics"
echo "====================="
echo ""

# 1. Check if bot is running
echo "1️⃣  Checking if bot process is running..."
if pgrep -f "notion-bot" > /dev/null; then
    echo "   ✅ Bot process found:"
    ps aux | grep notion-bot | grep -v grep
else
    echo "   ❌ Bot process NOT running!"
    echo "   → Start the bot first"
fi
echo ""

# 2. Check if port 8080 is listening
echo "2️⃣  Checking if port 8080 is listening..."
if netstat -tlnp 2>/dev/null | grep :8080 > /dev/null || ss -tlnp 2>/dev/null | grep :8080 > /dev/null; then
    echo "   ✅ Port 8080 is listening"
    netstat -tlnp 2>/dev/null | grep :8080 || ss -tlnp 2>/dev/null | grep :8080
else
    echo "   ❌ Port 8080 is NOT listening!"
    echo "   → The bot needs to be running and listening on port 8080"
fi
echo ""

# 3. Test local webhook endpoint
echo "3️⃣  Testing local webhook endpoint..."
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/telegram/webhook)
if [ "$RESPONSE" = "405" ] || [ "$RESPONSE" = "200" ]; then
    echo "   ✅ Local webhook endpoint responds (HTTP $RESPONSE)"
else
    echo "   ❌ Local webhook endpoint not responding (HTTP $RESPONSE)"
    echo "   → Expected 405 (Method Not Allowed) for GET request"
fi
echo ""

# 4. Check nginx status
echo "4️⃣  Checking nginx status..."
if systemctl is-active --quiet nginx 2>/dev/null; then
    echo "   ✅ Nginx is running"
else
    echo "   ⚠️  Nginx status unknown or not running"
fi
echo ""

# 5. Test nginx configuration
echo "5️⃣  Testing nginx configuration..."
if nginx -t 2>&1 | grep -q "successful"; then
    echo "   ✅ Nginx configuration is valid"
else
    echo "   ❌ Nginx configuration has errors:"
    nginx -t 2>&1
fi
echo ""

# 6. Check if webhook location is in nginx config
echo "6️⃣  Checking nginx webhook configuration..."
if grep -q "/telegram/webhook" /etc/nginx/nginx.conf 2>/dev/null; then
    echo "   ✅ Webhook endpoint found in nginx config"
else
    echo "   ❌ Webhook endpoint NOT found in nginx config"
    echo "   → Run: sudo ./add-webhook-to-nginx.sh"
fi
echo ""

# 7. Test public webhook endpoint
echo "7️⃣  Testing public webhook endpoint..."
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" https://tralalero-tralala.ru/telegram/webhook)
if [ "$RESPONSE" = "405" ] || [ "$RESPONSE" = "200" ]; then
    echo "   ✅ Public webhook endpoint responds (HTTP $RESPONSE)"
else
    echo "   ❌ Public webhook endpoint error (HTTP $RESPONSE)"
    if [ "$RESPONSE" = "502" ]; then
        echo "   → 502 Bad Gateway: Backend (bot) is not running or unreachable"
    elif [ "$RESPONSE" = "404" ]; then
        echo "   → 404 Not Found: Nginx doesn't have webhook location configured"
    fi
fi
echo ""

# 8. Check environment variables
echo "8️⃣  Checking environment variables..."
if [ -f .env ]; then
    if grep -q "WEBHOOK_URL" .env; then
        echo "   ✅ WEBHOOK_URL is set in .env"
        grep "WEBHOOK_URL" .env
    else
        echo "   ⚠️  WEBHOOK_URL not found in .env"
    fi
    
    if grep -q "AUTHORIZED_USER_ID" .env; then
        echo "   ✅ AUTHORIZED_USER_ID is set in .env"
    else
        echo "   ⚠️  AUTHORIZED_USER_ID not set in .env"
    fi
else
    echo "   ⚠️  .env file not found"
fi
echo ""

# Summary
echo "📋 Summary"
echo "========="
echo ""
echo "Common fixes for 502 Bad Gateway:"
echo "  1. Start the bot: ./notion-bot"
echo "  2. Check bot logs for errors"
echo "  3. Ensure WEBHOOK_URL is set in .env"
echo "  4. Verify nginx has webhook location"
echo "  5. Restart nginx: sudo systemctl restart nginx"
echo ""
