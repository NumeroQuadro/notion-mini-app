#!/bin/bash

# Quick script to add webhook endpoint to existing nginx config
# Use this if you already have nginx running and just want to add the webhook

NGINX_CONF="/etc/nginx/nginx.conf"
BACKUP_FILE="/etc/nginx/nginx.conf.backup.$(date +%Y%m%d%H%M%S)"

# Check if webhook endpoint already exists
if grep -q "/telegram/webhook" $NGINX_CONF; then
    echo "✅ Webhook endpoint already configured in nginx!"
    echo "Current configuration includes /telegram/webhook"
    exit 0
fi

# Create backup
sudo cp $NGINX_CONF $BACKUP_FILE
echo "📦 Created backup at $BACKUP_FILE"

# Add webhook endpoint before the closing brace of the HTTPS server block
# This adds it after the last location block
echo "➕ Adding webhook endpoint to nginx configuration..."

sudo sed -i '/location \/notion\/mini-app\/api\//,/^        }/a\
\
        # Telegram webhook endpoint for receiving reactions\
        location /telegram/webhook {\
            proxy_pass http://localhost:8080/telegram/webhook;\
            proxy_set_header Host $host;\
            proxy_set_header X-Real-IP $remote_addr;\
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\
            proxy_set_header X-Forwarded-Proto $scheme;\
            proxy_redirect off;\
            \
            # Telegram webhook timeouts\
            proxy_connect_timeout 10s;\
            proxy_send_timeout 10s;\
            proxy_read_timeout 10s;\
        }' $NGINX_CONF

# Test the configuration
echo "🔍 Testing nginx configuration..."
if sudo nginx -t; then
    echo "✅ Configuration test successful!"
    echo "🔄 Reloading nginx..."
    sudo systemctl reload nginx
    echo "✅ Nginx reloaded successfully!"
    echo ""
    echo "Webhook endpoint is now available at:"
    echo "  https://tralalero-tralala.ru/telegram/webhook"
    echo ""
    echo "Next steps:"
    echo "  1. Update WEBHOOK_URL in .env file"
    echo "  2. Run ./setup-webhook.sh to configure Telegram"
else
    echo "❌ Configuration test failed!"
    echo "🔙 Restoring backup..."
    sudo cp $BACKUP_FILE $NGINX_CONF
    echo "⚠️  Backup restored. Please check the error messages above."
    exit 1
fi
