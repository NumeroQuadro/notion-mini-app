# Troubleshooting: Webhook vs Polling Conflict

## Error: "Conflict: can't use getUpdates method while webhook is active"

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
