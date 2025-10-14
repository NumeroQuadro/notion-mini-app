# Testing the Reaction-Based Task Creation

## Testing Locally

### 1. Start the bot
```bash
go run cmd/main.go
```

### 2. Expose local server to internet (for webhook)

Using ngrok:
```bash
ngrok http 8080
```

Or using localhost.run:
```bash
ssh -R 80:localhost:8080 localhost.run
```

### 3. Set webhook URL in .env
```
WEBHOOK_URL=https://your-ngrok-url.ngrok.io/telegram/webhook
```

### 4. Setup webhook
```bash
./setup-webhook.sh
```

### 5. Test the bot

1. Send a message to your bot: "Buy milk"
2. React to your message with any emoji (👍, ❤️, etc.)
3. The bot should:
   - Create a task in Notion with title "Buy milk"
   - Add a ✅ reaction to confirm

## Troubleshooting

### Reactions not working?

Check webhook status:
```bash
curl https://api.telegram.org/bot<YOUR_TOKEN>/getWebhookInfo
```

Expected response should show:
- `url`: Your webhook URL
- `pending_update_count`: 0 (if everything is processed)
- `allowed_updates`: ["message", "message_reaction"]

### Check logs

The bot logs all incoming updates. Look for:
```
Received webhook update: ...
Received message_reaction update: ...
```

### Delete webhook (use polling instead)

```bash
curl -X POST https://api.telegram.org/bot<YOUR_TOKEN>/deleteWebhook
```

Note: Reactions won't work with polling, only regular messages.

## How it Works

1. **Message sent**: User sends a text message to the bot
   - Bot receives it via long polling
   - Bot stores it as a "pending task" in memory
   - Bot does NOT send any confirmation message

2. **Reaction added**: User adds a reaction to their message
   - Telegram sends a `message_reaction` update to webhook
   - Bot receives it via webhook endpoint
   - Bot checks if this message is in pending tasks
   - If yes, creates a task in Notion

3. **Task created**: Bot confirms by adding ✅ reaction
   - Uses direct Telegram API call to add reaction
   - Removes task from pending tasks

## Architecture

```
User Message → Long Polling → Bot stores in memory
                                    ↓
User Reaction → Webhook → Bot creates task in Notion
                                    ↓
                          Bot adds ✅ reaction
```

This hybrid approach (polling + webhook) allows us to:
- Use simple long polling for messages (no HTTPS needed for development)
- Use webhook for reactions (required by Telegram API)
- Keep the chat clean (no confirmation messages)
