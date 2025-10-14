# Troubleshooting: Webhook vs Polling Conflict

## Error 1: "Conflict: can't use getUpdates method while webhook is active"

This error occurs when:
1. A webhook is configured in Telegram
2. Your bot tries to use polling (`getUpdates`)
3. Telegram doesn't allow both simultaneously

## Solution Options

### Option 1: Use Webhook Mode (Recommended for Production)

**Why:** Webhooks support reactions, better for production

**Steps:**
```bash
# 1. Make sure WEBHOOK_URL is set in .env
echo "WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook" >> .env

# 2. Rebuild and restart the bot
go build -o notion-bot cmd/main.go
docker-compose restart  # or however you run it

# 3. Bot will automatically detect WEBHOOK_URL and use webhook mode
```

The bot will log:
```
Running in WEBHOOK mode: https://tralalero-tralala.ru/telegram/webhook
Bot will receive updates via webhook at /telegram/webhook
```

### Option 2: Delete Webhook and Use Polling (Development Only)

**Why:** Simpler for local development, no HTTPS needed

**⚠️ WARNING:** Reactions will NOT work in polling mode!

**Steps:**
```bash
# 1. Delete the webhook
./delete-webhook.sh

# 2. Remove or comment out WEBHOOK_URL in .env
# WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook

# 3. Restart the bot
docker-compose restart
```

The bot will log:
```
Running in POLLING mode (webhook URL not set)
WARNING: Reactions will NOT work in polling mode!
```

## Quick Commands

### Check current webhook status
```bash
export TELEGRAM_BOT_TOKEN="your_token"
curl "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo" | jq .
```

### Delete webhook
```bash
./delete-webhook.sh
# or manually:
curl -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/deleteWebhook"
```

### Set webhook
```bash
./setup-webhook.sh
# or manually:
curl -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/setWebhook" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://tralalero-tralala.ru/telegram/webhook","allowed_updates":["message","message_reaction"]}'
```

## How the Bot Chooses Mode

The bot automatically detects which mode to use based on environment variables:

```go
webhookURL := os.Getenv("WEBHOOK_URL")
if webhookURL != "" {
    // Use webhook mode
    log.Printf("Running in WEBHOOK mode: %s", webhookURL)
} else {
    // Use polling mode
    log.Printf("Running in POLLING mode")
}
```

## Comparison

| Feature | Webhook Mode | Polling Mode |
|---------|-------------|--------------|
| **Reactions** | ✅ Works | ❌ Doesn't work |
| **Messages** | ✅ Works | ✅ Works |
| **Requires HTTPS** | ✅ Yes | ❌ No |
| **Best for** | Production | Development |
| **Setup complexity** | Medium | Easy |

## Your Current Setup

Based on your error, you currently have:
- ✅ Webhook configured in Telegram
- ❌ Bot trying to use polling
- 🔧 Need to set `WEBHOOK_URL` in .env

## Fix for Your Case

```bash
# 1. Add WEBHOOK_URL to your .env
echo "WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook" >> .env

# 2. Make sure AUTHORIZED_USER_ID is set (from the logs warning)
echo "AUTHORIZED_USER_ID=your_telegram_user_id" >> .env

# 3. Restart the bot
docker-compose restart notion-bot
```

The errors should stop, and you'll see:
```
Running in WEBHOOK mode: https://tralalero-tralala.ru/telegram/webhook
Authorized user ID: your_id
```

---

## Error 2: "Wrong response from the webhook: 502 Bad Gateway"

### What It Means

Telegram successfully reached your webhook URL, but got a 502 error. This means:
- ✅ Webhook URL is correct
- ✅ Domain/nginx is accessible
- ❌ Backend application (bot) is not running or unreachable

### Quick Diagnosis

Run the diagnostic script:
```bash
./diagnose-webhook.sh
```

### Common Causes & Fixes

#### 1. Bot is not running

**Check:**
```bash
ps aux | grep notion-bot
# or for Docker:
docker-compose ps
```

**Fix:**
```bash
# If using systemd:
sudo systemctl start notion-bot
sudo systemctl status notion-bot

# If using Docker:
docker-compose up -d
docker-compose logs -f

# If running manually:
./notion-bot
```

#### 2. Bot crashed or has errors

**Check logs:**
```bash
# Systemd:
sudo journalctl -u notion-bot -f

# Docker:
docker-compose logs -f notion-bot

# Manual:
./notion-bot 2>&1 | tee bot.log
```

**Common issues in logs:**
- Missing `.env` file → Create it with required variables
- Database connection errors → Check Notion API key
- Port already in use → Change PORT in .env or stop conflicting service

#### 3. WEBHOOK_URL not set in .env

**Check:**
```bash
cat .env | grep WEBHOOK_URL
```

**Fix:**
```bash
echo "WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook" >> .env
```

Then restart the bot.

#### 4. Nginx can't reach the bot (port 8080)

**Check if port 8080 is listening:**
```bash
netstat -tlnp | grep 8080
# or
ss -tlnp | grep 8080
```

**Test local endpoint:**
```bash
curl http://localhost:8080/telegram/webhook
# Should return: 405 Method Not Allowed (this is correct!)
```

**If port is not listening:**
- Bot is not running
- Bot is listening on wrong port (check PORT in .env)
- Firewall blocking the port

#### 5. Nginx webhook location not configured

**Check nginx config:**
```bash
grep -A 10 "/telegram/webhook" /etc/nginx/nginx.conf
```

**If not found, add it:**
```bash
sudo ./add-webhook-to-nginx.sh
# or
sudo ./nginx-setup.sh
```

**Reload nginx:**
```bash
sudo nginx -t  # Test configuration
sudo systemctl reload nginx
```

### Step-by-Step Fix

```bash
# 1. Ensure .env has WEBHOOK_URL
echo "WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook" >> .env
echo "AUTHORIZED_USER_ID=your_telegram_id" >> .env

# 2. Rebuild the bot (if code changed)
go build -o notion-bot cmd/main.go

# 3. Start/restart the bot
docker-compose up -d
# or
sudo systemctl restart notion-bot
# or
./notion-bot &

# 4. Wait a few seconds, then check if it's running
ps aux | grep notion-bot
netstat -tlnp | grep 8080

# 5. Test local webhook
curl http://localhost:8080/telegram/webhook
# Should return 405 Method Not Allowed

# 6. Test public webhook
curl https://tralalero-tralala.ru/telegram/webhook
# Should also return 405

# 7. Check Telegram webhook status
curl "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo" | jq .
# Should show no errors and pending_update_count should decrease
```

### Expected Webhook Info (Healthy)

```json
{
  "ok": true,
  "result": {
    "url": "https://tralalero-tralala.ru/telegram/webhook",
    "has_custom_certificate": false,
    "pending_update_count": 0,
    "max_connections": 40,
    "allowed_updates": ["message", "message_reaction"]
  }
}
```

Notice:
- ✅ `pending_update_count: 0` (or small number)
- ✅ No `last_error_date` or `last_error_message`
- ✅ `allowed_updates` includes `message_reaction`

### Verification

1. **Send a test message** to your bot
2. **Check bot logs** - you should see:
   ```
   Received webhook update: ...
   Received message update via webhook
   Stored message X as pending task: ...
   ```
3. **Add a reaction** to your message
4. **Check bot logs** - you should see:
   ```
   Received message_reaction update: ...
   Task created successfully: ...
   ```
5. **Bot should add ✅ reaction** to confirm

---
