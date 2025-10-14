# Quick Fix: Container Crashes Immediately

## What You're Seeing

```bash
❌ Container crashed! Showing logs:
Error response from daemon: No such container: notion-mini-app
```

This means the container **starts but crashes within 3 seconds**, and because it had `--rm` flag, it was auto-removed before we could check logs.

## Step 1: See the REAL Error

Run this to see why it's crashing:

```bash
./docker-debug.sh
```

OR manually:

```bash
docker run --rm --name notion-mini-app-debug \
  --env-file .env \
  -p 8081:8080 \
  notion-mini-app
```

This runs the container in **foreground mode** so you can see the actual error message.

## Common Crash Reasons & Fixes

### 1. Missing Environment Variables

**Error you'll see:**
```
2025/10/14 XX:XX:XX Warning: .env file not found
2025/10/14 XX:XX:XX Fatal error: ...
```

**Fix:**
```bash
# Check if .env exists inside container
cat .env

# Required variables:
TELEGRAM_BOT_TOKEN=your_token
NOTION_API_KEY=your_api_key
NOTION_TASKS_DATABASE_ID=your_db_id
WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook
AUTHORIZED_USER_ID=your_telegram_user_id
```

### 2. Cannot Open web/ Directory

**Error you'll see:**
```
open ./web: no such file or directory
panic: ...
```

**Check:**
```bash
# Verify web directory is in the image
docker run --rm notion-mini-app ls -la /app/web
```

**Should show:**
```
drwxr-xr-x    2 root     root          4096 Oct 14 13:56 .
drwxr-xr-x    1 root     root          4096 Oct 14 13:56 ..
-rw-r--r--    1 root     root          1234 Oct 14 13:56 app.js
-rw-r--r--    1 root     root          5678 Oct 14 13:56 index.html
-rw-r--r--    1 root     root           910 Oct 14 13:56 styles.css
```

**If empty or missing:**
```bash
# Rebuild with fresh clone
docker rmi notion-mini-app
make docker-build
```

### 3. Invalid Notion API Key

**Error you'll see:**
```
Error creating Notion client: unauthorized
```

**Fix:**
```bash
# Verify your Notion API key
# Go to: https://www.notion.so/my-integrations
# Make sure the integration has access to your database
```

### 4. Invalid Database ID

**Error you'll see:**
```
Failed to get database: Could not find database with ID: ...
```

**Fix:**
```bash
# Double-check your database ID in .env
# Should be 32 characters without dashes
```

### 5. Certificate Files Not Found (if mounted)

**Error you'll see:**
```
Error: no such file or directory: /app/certs/fullchain.pem
```

**Fix Option A - Remove cert mounts (temporary):**

Edit `Makefile`, remove these lines:
```bash
-v /etc/letsencrypt/live/tralalero-tralala.ru/fullchain.pem:/app/certs/fullchain.pem:ro \
-v /etc/letsencrypt/live/tralalero-tralala.ru/privkey.pem:/app/certs/privkey.pem:ro \
```

**Fix Option B - Verify certs exist:**
```bash
ls -la /etc/letsencrypt/live/tralalero-tralala.ru/
```

## Step 2: Fix the Issue

Based on the error from `docker-debug.sh`, apply the appropriate fix above.

## Step 3: Restart Container

```bash
# Clean up
make docker-clean

# Rebuild (if needed)
make docker-build

# Start
make docker-run
```

**Should now see:**
```
✅ Container is running!
View logs with: make docker-logs
```

## Step 4: Verify

```bash
# Check status
make docker-status

# Should show:
✅ Container is RUNNING

# View logs
make docker-logs

# Should show:
2025/10/14 XX:XX:XX Running in WEBHOOK mode: https://tralalero-tralala.ru/telegram/webhook
2025/10/14 XX:XX:XX Starting mini app server on 0.0.0.0:8080
```

## Updated Makefile Commands

```bash
make docker-run        # Start container (without --rm, so we can see logs)
make docker-run-debug  # Run in foreground to see errors
make docker-debug      # Alias for docker-run-debug
make docker-clean      # Clean up all containers
make docker-status     # Check container health
make docker-logs       # View logs
make docker-stop       # Stop and remove container
```

## Most Likely Issue

Based on the quick crash, it's probably **one of these**:

1. **Missing WEBHOOK_URL in .env** → Bot tries to use polling but webhook is active (409 error loop until crash)
2. **Invalid Notion credentials** → Bot can't connect to Notion API
3. **Missing .env file** → Bot has no configuration

## Quick Diagnostic Commands

```bash
# 1. Check .env exists and has required vars
cat .env | grep -E "TELEGRAM|NOTION|WEBHOOK|AUTHORIZED"

# 2. Check web directory is in image
docker run --rm notion-mini-app ls -la /app/

# 3. Run in debug mode to see exact error
./docker-debug.sh

# 4. Check if it's the webhook conflict issue
grep "409" ~/.docker/containers/*/$(docker ps -aq | head -1)*-json.log 2>/dev/null
```

## Need Help?

Run diagnostics:
```bash
./docker-debug.sh
```

This will show you the **exact error** that's causing the crash!
