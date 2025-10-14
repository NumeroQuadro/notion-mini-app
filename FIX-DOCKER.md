# Fix: Docker Container Not Running

## The Problem

After running `make docker-run`, the container builds successfully but doesn't appear in `docker ps`:

```bash
Successfully built 1ff394166509
Successfully tagged notion-mini-app:latest
a08819af99d3...  # Container started

$ docker ps
CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES
# Empty! Container crashed immediately
```

## Root Causes Fixed

### 1. Missing `web/` Directory in Docker Image ⭐ MAIN ISSUE

**Problem:** The Dockerfile wasn't copying the `web/` directory to the final image. When the bot tried to serve static files from `./web`, it crashed because the directory didn't exist.

**Fixed in Dockerfile:**
```dockerfile
# Old - missing web directory
COPY --from=builder /notion-mini-app /app/notion-mini-app
# ❌ web/ directory NOT copied!

# New - includes web directory
COPY --from=builder /notion-mini-app /app/notion-mini-app
COPY --from=builder /src/web /app/web  # ✅ Fixed!
```

### 2. No Crash Detection in Makefile

**Problem:** `make docker-run` would start the container but not check if it crashed.

**Fixed in Makefile:**
```bash
# Now checks if container is still running after 3 seconds
# Shows crash logs if it failed
```

## How to Fix Your Server

### Step 1: Update Files

The following files have been updated:
- ✅ `Dockerfile` - Now copies web directory
- ✅ `Makefile` - Now detects crashes
- ✅ New: `diagnose-docker.sh` - Diagnostic script
- ✅ New: `DOCKER-TROUBLESHOOTING.md` - Complete guide

On your server:
```bash
cd /home/notion-mini-app

# Pull the latest changes
git pull

# Or manually update Dockerfile (see the changes above)
```

### Step 2: Rebuild Docker Image

```bash
# Remove old image
docker rmi notion-mini-app

# Build new image with web directory
make docker-build
```

You should see:
```
Successfully built XXXXXXXXXX
Successfully tagged notion-mini-app:latest
```

### Step 3: Ensure .env File Exists

```bash
# Check if .env exists
ls -la .env

# If not, create it
cp .env.example .env

# Edit with your values
nano .env
```

Required variables:
```bash
TELEGRAM_BOT_TOKEN=your_token
NOTION_API_KEY=your_key
NOTION_TASKS_DATABASE_ID=your_db_id
WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook
AUTHORIZED_USER_ID=your_user_id
```

### Step 4: Run Container

```bash
make docker-run
```

**Expected output (success):**
```
Starting Docker container...
a08819af99d3381ca1cef345a9577cc31b26882acc4b29940bfd4529d520f469
Waiting for container to start...
✅ Container is running!
View logs with: make docker-logs
```

**If it crashes:**
```
❌ Container crashed! Showing logs:
[crash logs will be displayed here]
```

### Step 5: Verify Container is Running

```bash
# Check status
make docker-status

# Should show:
✅ Container is RUNNING

# View logs
make docker-logs
```

**Expected logs:**
```
2025/10/14 XX:XX:XX Bot restricted to user ID: 123456789
2025/10/14 XX:XX:XX Running in WEBHOOK mode: https://tralalero-tralala.ru/telegram/webhook
2025/10/14 XX:XX:XX Starting mini app server on 0.0.0.0:8080
2025/10/14 XX:XX:XX Mini app available at: http://0.0.0.0:8080/notion/mini-app/
```

### Step 6: Test the Bot

1. Send message to bot
2. Check logs: `make docker-logs`
3. Should see: "Received webhook update..."
4. Add reaction to message
5. Should see: "Task created successfully..."
6. Bot adds ✅ reaction

## New Makefile Commands

```bash
make docker-build    # Build Docker image
make docker-run      # Run container (with crash detection)
make docker-status   # Check if container is running
make docker-logs     # View logs (follow mode)
make docker-stop     # Stop container
make docker-rm       # Remove container
```

## Quick Debugging

If container still doesn't run:

```bash
# 1. Run diagnostics
./diagnose-docker.sh

# 2. Check what's wrong
make docker-status

# 3. Try running in foreground (see errors directly)
docker run --rm --name notion-mini-app-debug \
  --env-file .env \
  -p 8081:8080 \
  notion-mini-app
```

## Common Issues After Fix

### Issue: "Cannot find web directory"

**Solution:**
```bash
# Make sure web/ exists in your project
ls -la web/

# If missing, you need to copy it from the repository
```

### Issue: "bind: address already in use"

**Solution:**
```bash
# Check what's using port 8081
lsof -i :8081

# Stop it or change port in Makefile
```

### Issue: Container runs but webhook still shows 502

**Solution:**
```bash
# 1. Check nginx is forwarding to port 8081 (not 8080)
grep 8081 /etc/nginx/nginx.conf

# 2. Update nginx to forward to 8081:
# location /telegram/webhook {
#     proxy_pass http://localhost:8081/telegram/webhook;
# }

# 3. Reload nginx
sudo nginx -t
sudo systemctl reload nginx
```

## Summary

**What was fixed:**
1. ✅ Dockerfile now copies `web/` directory
2. ✅ Makefile detects container crashes
3. ✅ Added diagnostic tools
4. ✅ Added comprehensive troubleshooting guide

**What you need to do:**
1. Pull latest code or update Dockerfile manually
2. Rebuild: `make docker-build`
3. Run: `make docker-run`
4. Verify: `make docker-status`

The container should now stay running! 🎉
