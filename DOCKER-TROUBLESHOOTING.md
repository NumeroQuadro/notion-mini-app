# Docker Troubleshooting Guide

## Issue: Container Not Running After `make docker-run`

### Symptoms
```bash
$ docker ps
CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES
# Empty - no container running
```

### Common Causes

#### 1. Container Crashed Immediately

**Diagnosis:**
```bash
# Check if container exists (even if stopped)
docker ps -a | grep notion-mini-app

# If found, check logs
docker logs notion-mini-app
```

**Common crash reasons:**
- Missing `web/` directory in Docker image
- Missing environment variables
- Application error (check logs)

#### 2. Missing `web` Directory

**Problem:** The old Dockerfile didn't copy the `web/` directory to the final image.

**Solution:** Already fixed! The new Dockerfile now includes:
```dockerfile
# Copy the web directory (CRITICAL - bot needs this!)
COPY --from=builder /src/web /app/web
```

**Rebuild:**
```bash
make docker-build
make docker-run
```

#### 3. Missing or Invalid `.env` File

**Check:**
```bash
cat .env
```

**Required variables:**
```bash
TELEGRAM_BOT_TOKEN=your_token
NOTION_API_KEY=your_key
NOTION_TASKS_DATABASE_ID=your_db_id
WEBHOOK_URL=https://tralalero-tralala.ru/telegram/webhook
AUTHORIZED_USER_ID=your_user_id
```

#### 4. Port Already in Use

**Check:**
```bash
netstat -tlnp | grep 8081
# or
lsof -i :8081
```

**Solution:**
- Stop the conflicting service
- Or change port in Makefile

#### 5. Volume Mount Issues

The Makefile tries to mount SSL certificates:
```bash
-v /etc/letsencrypt/live/tralalero-tralala.ru/fullchain.pem:/app/certs/fullchain.pem:ro
```

**If certificates don't exist:**
```bash
# Check if files exist
ls -la /etc/letsencrypt/live/tralalero-tralala.ru/
```

**Temporary fix (if certs not needed):**
Edit Makefile and remove the `-v` mount lines.

## Makefile Commands

### Build Docker image
```bash
make docker-build
```

### Run container (with automatic crash detection)
```bash
make docker-run
```

New behavior:
- Starts container
- Waits 3 seconds
- Checks if still running
- Shows logs if crashed

### Check container status
```bash
make docker-status
```

Shows:
- Container status (running/stopped)
- Last logs if crashed
- Port mappings

### View logs
```bash
make docker-logs
```

### Stop container
```bash
make docker-stop
```

### Remove container
```bash
make docker-rm
```

## Step-by-Step Debugging

### 1. Build fresh image
```bash
make docker-build
```

### 2. Run diagnostic script (before starting)
```bash
./diagnose-docker.sh
```

This checks:
- .env file exists
- Required variables set
- web/ directory exists

### 3. Start container
```bash
make docker-run
```

Should now show:
```
✅ Container is running!
View logs with: make docker-logs
```

### 4. If container crashes

Check status:
```bash
make docker-status
```

View logs:
```bash
docker logs notion-mini-app
```

### 5. Run container in foreground (for debugging)
```bash
docker run --rm --name notion-mini-app-debug \
  --env-file .env \
  -p 8081:8080 \
  notion-mini-app
```

This will show logs directly in the terminal.

## Expected Behavior

### Successful Start

```bash
$ make docker-run
Building from Dockerfile...
Successfully built 1ff394166509
Successfully tagged notion-mini-app:latest
Starting Docker container...
a08819af99d3...
Waiting for container to start...
✅ Container is running!
View logs with: make docker-logs
```

### Container Logs (Healthy)

```bash
$ make docker-logs
2025/10/14 13:56:07 Warning: .env file not found, using environment variables
2025/10/14 13:56:07 Bot restricted to user ID: 123456789
2025/10/14 13:56:07 Mini App URL: https://tralalero-tralala.ru/notion/mini-app
2025/10/14 13:56:08 Authorized on account chat_gpt_killer_bot
2025/10/14 13:56:08 Running in WEBHOOK mode: https://tralalero-tralala.ru/telegram/webhook
2025/10/14 13:56:08 Starting mini app server on 0.0.0.0:8080
2025/10/14 13:56:08 Mini app available at: http://0.0.0.0:8080/notion/mini-app/
```

### Container Status (Healthy)

```bash
$ make docker-status
Docker container status:
CONTAINER ID   STATUS          NAMES              PORTS
a08819af99d3   Up 2 minutes    notion-mini-app    0.0.0.0:8081->8080/tcp

✅ Container is RUNNING
```

## Quick Fix Checklist

- [ ] `.env` file exists with all required variables
- [ ] `web/` directory exists in project
- [ ] Dockerfile updated (includes web directory copy)
- [ ] Docker image rebuilt: `make docker-build`
- [ ] Ports 8081/8443 are not in use
- [ ] SSL certificate files exist (or mount removed from Makefile)
- [ ] Container started: `make docker-run`
- [ ] Container is running: `docker ps | grep notion-mini-app`
- [ ] Logs look healthy: `make docker-logs`

## Still Having Issues?

Run full diagnostics:
```bash
./diagnose-docker.sh
```

Check webhook setup:
```bash
./diagnose-webhook.sh
```

Check container status:
```bash
make docker-status
```

View detailed logs:
```bash
docker logs --tail 100 notion-mini-app
```
