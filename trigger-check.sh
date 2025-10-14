#!/bin/bash

# Script to manually trigger the daily task check
# Useful for testing or recovering from failed scheduled runs

set -e

# Configuration
HOST="${HOST:-localhost}"
PORT="${PORT:-8080}"
URL="http://${HOST}:${PORT}/notion/mini-app/api/trigger-check"

echo "🔄 Triggering manual task check..."
echo "Endpoint: $URL"
echo ""

# Make the API call
RESPONSE=$(curl -s -X POST "$URL" -H "Content-Type: application/json")

# Check if successful
if echo "$RESPONSE" | grep -q '"status":"success"'; then
    echo "✅ Task check triggered successfully!"
    echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
else
    echo "❌ Failed to trigger task check"
    echo "$RESPONSE"
    exit 1
fi

echo ""
echo "Check the bot logs for task check results"
