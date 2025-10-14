# Fix Guide: 502 Bad Gateway Error

## Your Situation

✅ Webhook is configured in Telegram  
✅ Domain is accessible (tralalero-tralala.ru)  
✅ Webhook URL is correct  
❌ **Bot is not running** → causes 502 error

## The Error You Saw

```json
{
  "last_error_message": "Wrong response from the webhook: 502 Bad Gateway",
  "pending_update_count": 2
}
```

This means Telegram tried to send 2 updates but couldn't reach your bot.

## Step-by-Step Fix

### 1. Ensure .env has WEBHOOK_URL

On your server:
```bash
cd /home/notion-mini-app

# Add WEBHOOK_URL if not present
echo "WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook" >> .env

# Also ensure AUTHORIZED_USER_ID is set
echo "AUTHORIZED_USER_ID=your_telegram_user_id" >> .env

# Verify
cat .env
```

### 2. Make sure nginx has webhook endpoint

```bash
# Check if webhook location exists
grep "/telegram/webhook" /etc/nginx/nginx.conf

# If not found, add it:
sudo ./add-webhook-to-nginx.sh
```

### 3. Start the bot

**Option A: Using the production start script (recommended)**
```bash
./start-production.sh
```

**Option B: Using Docker**
```bash
docker-compose up -d
docker-compose logs -f
```

**Option C: Using systemd**
```bash
sudo systemctl start notion-bot
sudo systemctl status notion-bot
```

**Option D: Manual**
```bash
# Rebuild first
go build -o notion-bot cmd/main.go

# Run in background
nohup ./notion-bot > bot.log 2>&1 &

# Or run in foreground (see logs)
./notion-bot
```

### 4. Verify it's working

```bash
# Check if bot is running
ps aux | grep notion-bot

# Check if port 8080 is listening
netstat -tlnp | grep 8080

# Test local endpoint
curl http://localhost:8080/telegram/webhook
# Should return: 405 Method Not Allowed (this is GOOD!)

# Test public endpoint
curl https://tralalero-tralala.ru/telegram/webhook
# Should also return: 405 Method Not Allowed

# Check webhook status
curl "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo" | jq .
```

### 5. Expected Results

**Healthy webhook info:**
```json
{
  "ok": true,
  "result": {
    "url": "https://tralalero-tralala.ru/telegram/webhook",
    "pending_update_count": 0,  // ← Should be 0 or decreasing
    "allowed_updates": ["message", "message_reaction"]
    // NO last_error_date or last_error_message
  }
}
```

**Bot logs should show:**
```
Running in WEBHOOK mode: https://tralalero-tralala.ru/telegram/webhook
Bot will receive updates via webhook at /telegram/webhook
Authorized on account chat_gpt_killer_bot
Bot restricted to user ID: 123456789
Starting mini app server on 0.0.0.0:8080
```

## Testing the Bot

1. **Send a message** to @chat_gpt_killer_bot
   - Example: "Buy groceries"
   
2. **Check bot logs** - should see:
   ```
   Received webhook update: ...
   Received message update via webhook
   Stored message 123 as pending task: Buy groceries
   ```

3. **Add a reaction** to your message (any emoji: 👍 ❤️ ✨)

4. **Check bot logs** - should see:
   ```
   Received message_reaction update: ...
   Received reaction update for message 123 from user 123456789
   Task created successfully: Buy groceries
   Successfully set reaction on message 123
   ```

5. **Bot adds ✅ reaction** to confirm task was created

6. **Check Notion** - task should appear in your database!

## Common Issues

### Bot crashes immediately

**Check logs:**
```bash
./notion-bot
# Look for error messages
```

**Common causes:**
- Missing environment variables → Check .env
- Invalid Notion API key → Verify in Notion settings
- Database ID incorrect → Double-check NOTION_TASKS_DATABASE_ID

### Port 8080 already in use

**Find what's using it:**
```bash
lsof -i :8080
```

**Either:**
- Stop the other service
- Change PORT in .env to different port (e.g., 8081)
- Update nginx config to match new port

### Still getting 502 after starting

**Wait 30 seconds** for webhook to retry, then:
```bash
# Run full diagnostics
./diagnose-webhook.sh

# Check all services
systemctl status nginx
systemctl status notion-bot  # if using systemd

# Check nginx can reach bot
curl -v http://localhost:8080/telegram/webhook
```

## Quick Reference

```bash
# Start bot
./start-production.sh

# Diagnose issues
./diagnose-webhook.sh

# Check webhook status
./setup-webhook.sh  # Shows status at the end

# View logs
tail -f bot.log                    # Manual
docker-compose logs -f             # Docker
sudo journalctl -u notion-bot -f   # Systemd
```

## Need More Help?

See detailed troubleshooting: [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
