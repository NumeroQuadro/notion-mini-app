# Deployment Guide for tralalero-tralala.ru

## Current Setup

Your bot is already running with:
- **Domain**: tralalero-tralala.ru
- **Mini-app**: https://tralalero-tralala.ru/notion/mini-app
- **SSL**: Let's Encrypt certificates
- **Server**: nginx + Go bot on port 8080

## Adding Webhook Support (for reactions)

### Option 1: Update Existing Nginx (Recommended)

If nginx is already configured and running:

```bash
# 1. Add webhook endpoint to nginx
sudo ./add-webhook-to-nginx.sh

# 2. Update .env file
echo "WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook" >> .env

# 3. Restart the bot
sudo systemctl restart notion-bot  # or however you run the bot

# 4. Configure Telegram webhook
./setup-webhook.sh
```

### Option 2: Full Nginx Reconfiguration

If you want to regenerate the entire nginx config:

```bash
# 1. Run the full setup (includes webhook endpoint)
sudo ./nginx-setup.sh

# 2. Update .env file
echo "WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook" >> .env

# 3. Restart the bot
sudo systemctl restart notion-bot

# 4. Configure Telegram webhook
./setup-webhook.sh
```

## Verification

### 1. Check nginx configuration
```bash
sudo nginx -t
```

### 2. Check if webhook endpoint is accessible
```bash
curl https://tralalero-tralala.ru/telegram/webhook
# Should return 405 Method Not Allowed (because it expects POST)
```

### 3. Check Telegram webhook status
```bash
# Set your token first
export TELEGRAM_BOT_TOKEN="your_token_here"

# Check webhook info
curl "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo"
```

Should show:
```json
{
  "ok": true,
  "result": {
    "url": "https://tralalero-tralala.ru/telegram/webhook",
    "has_custom_certificate": false,
    "pending_update_count": 0,
    "allowed_updates": ["message", "message_reaction"]
  }
}
```

### 4. Test the bot
1. Send a message to your bot
2. Add a reaction (👍, ❤️, etc.)
3. Bot should:
   - Create task in Notion
   - Add ✅ reaction to your message

## Troubleshooting

## Troubleshooting

### 502 Bad Gateway Error

If webhook info shows:
```json
{
  "last_error_message": "Wrong response from the webhook: 502 Bad Gateway"
}
```

**This means the bot is not running!**

Quick fix:
```bash
# 1. Run diagnostics
./diagnose-webhook.sh

# 2. Ensure .env has WEBHOOK_URL
echo "WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook" >> .env

# 3. Start the bot
./start-production.sh
# or with Docker:
docker-compose up -d

# 4. Verify it's running
ps aux | grep notion-bot
curl http://localhost:8080/telegram/webhook  # Should return 405
```

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for detailed fixes.

### Webhook not receiving updates

Check nginx logs:
```bash
sudo tail -f /var/log/nginx/access.log | grep webhook
sudo tail -f /var/log/nginx/error.log
```

Check bot logs:
```bash
sudo journalctl -u notion-bot -f
# or
sudo tail -f /var/log/notion-bot.log
```

### SSL certificate issues

Renew Let's Encrypt certificate:
```bash
sudo certbot renew
sudo systemctl reload nginx
```

### Bot not responding

Check if bot is running:
```bash
sudo systemctl status notion-bot
# or
ps aux | grep notion-bot
```

Restart the bot:
```bash
sudo systemctl restart notion-bot
```

## Your Complete .env Configuration

```bash
# Telegram
TELEGRAM_BOT_TOKEN=your_actual_token
AUTHORIZED_USER_ID=your_telegram_id

# Notion
NOTION_API_KEY=your_notion_key
NOTION_TASKS_DATABASE_ID=your_tasks_db_id
NOTION_NOTES_DATABASE_ID=your_notes_db_id

# Server
HOST=0.0.0.0
PORT=8080
ENVIRONMENT=production

# URLs (same domain!)
MINI_APP_URL=https://tralalero-tralala.ru/notion/mini-app
WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook
```

## Final Nginx Configuration

Your nginx will have these endpoints:

- **/** → Static files (if any)
- **/notion/mini-app/** → Mini-app interface (HTML/CSS/JS)
- **/notion/mini-app/api/** → API for mini-app (Notion operations)
- **/telegram/webhook** → Webhook for Telegram reactions ⭐ NEW

All running on the same domain with the same SSL certificate!
