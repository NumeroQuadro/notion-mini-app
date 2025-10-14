#!/bin/bash

# Validate .env file for common mistakes

echo "🔍 Validating .env file"
echo "======================"
echo ""

# Check if .env exists
if [ ! -f .env ]; then
    echo "❌ Error: .env file not found!"
    echo ""
    echo "Create it from the example:"
    echo "  cp .env.production.example .env"
    echo "  nano .env"
    echo ""
    exit 1
fi

echo "✅ .env file exists"
echo ""

# Check required variables
echo "Checking required variables:"
echo "----------------------------"

check_var() {
    VAR_NAME=$1
    VAR_VALUE=$(grep "^${VAR_NAME}=" .env | cut -d '=' -f2-)
    
    if [ -z "$VAR_VALUE" ] || [ "$VAR_VALUE" = "your_${VAR_NAME,,}_here" ] || [[ "$VAR_VALUE" =~ ^your_ ]]; then
        echo "❌ $VAR_NAME: Missing or not set"
        return 1
    else
        # Show first 10 chars only (security)
        PREVIEW="${VAR_VALUE:0:10}..."
        echo "✅ $VAR_NAME: Set (${PREVIEW})"
        return 0
    fi
}

ERRORS=0

check_var "TELEGRAM_BOT_TOKEN" || ((ERRORS++))
check_var "AUTHORIZED_USER_ID" || ((ERRORS++))
check_var "NOTION_API_KEY" || ((ERRORS++))
check_var "NOTION_TASKS_DATABASE_ID" || ((ERRORS++))
check_var "WEBHOOK_URL" || ((ERRORS++))

echo ""

# Check HOST setting
echo "Checking Docker-specific settings:"
echo "----------------------------------"
HOST_VALUE=$(grep "^HOST=" .env | cut -d '=' -f2-)

if [ "$HOST_VALUE" = "0.0.0.0" ]; then
    echo "✅ HOST: Correctly set to 0.0.0.0 for Docker"
elif [ "$HOST_VALUE" = "185.92.182.65" ] || [[ "$HOST_VALUE" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "❌ HOST: Set to specific IP ($HOST_VALUE)"
    echo "   For Docker, change to: HOST=0.0.0.0"
    ((ERRORS++))
else
    echo "⚠️  HOST: Set to $HOST_VALUE (unusual, but might work)"
fi

echo ""

# Summary
echo "Summary:"
echo "--------"
if [ $ERRORS -eq 0 ]; then
    echo "✅ All checks passed!"
    echo ""
    echo "Your .env file is ready for Docker."
    echo "Run: make docker-run"
    exit 0
else
    echo "❌ Found $ERRORS issue(s)"
    echo ""
    echo "Please fix the issues above, then run:"
    echo "  ./validate-env.sh"
    echo ""
    echo "Quick fixes:"
    echo "  1. Get Telegram user ID: https://t.me/userinfobot"
    echo "  2. Get Notion API key: https://www.notion.so/my-integrations"
    echo "  3. Set HOST=0.0.0.0 (not your server IP!)"
    echo ""
    exit 1
fi
