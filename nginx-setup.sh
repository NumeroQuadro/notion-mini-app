#!/bin/bash

# Set variables
NGINX_CONF="/etc/nginx/nginx.conf"
BACKUP_FILE="/etc/nginx/nginx.conf.backup.$(date +%Y%m%d%H%M%S)"
DOMAIN="tralalero-tralala.ru"
APP_PORT="8080"

# Create backup
sudo cp $NGINX_CONF $BACKUP_FILE
echo "Created backup at $BACKUP_FILE"

# Create new nginx.conf
sudo tee $NGINX_CONF > /dev/null << EOF
user www-data;
worker_processes auto;
pid /run/nginx.pid;
include /etc/nginx/modules-enabled/*.conf;

events {
    worker_connections 768;
    # multi_accept on;
}

http {
    # Basic Settings
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;

    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    # SSL Settings
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;

    # Logging Settings
    access_log /var/log/nginx/access.log;
    error_log /var/log/nginx/error.log;

    # Gzip Settings
    gzip on;

    # Virtual Host Configs
    include /etc/nginx/conf.d/*.conf;
    
    # Server block for HTTP (redirects to HTTPS)
    server {
        listen 80;
        server_name $DOMAIN;
        
        # Redirect all HTTP requests to HTTPS
        return 301 https://\$host\$request_uri;
    }

    # Server block for HTTPS
    server {
        listen 443 ssl;
        server_name $DOMAIN;
        
        # SSL certificate paths - UPDATE THESE WITH YOUR ACTUAL PATHS
        ssl_certificate /etc/letsencrypt/live/$DOMAIN/fullchain.pem;
        ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;
        
        # SSL configurations
        ssl_session_cache shared:SSL:10m;
        ssl_session_timeout 10m;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;
        ssl_prefer_server_ciphers on;
        
        # Root location
        location / {
            root /var/www/html;
            index index.html;
        }
        
        # Notion mini app location - static files
        location /notion/mini-app/ {
            proxy_pass http://localhost:$APP_PORT/notion/mini-app/;
            proxy_set_header Host \$host;
            proxy_set_header X-Real-IP \$remote_addr;
            proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto \$scheme;
            proxy_redirect off;
        }
        
        # Notion mini app API endpoints - need higher timeouts
        location /notion/mini-app/api/ {
            proxy_pass http://localhost:$APP_PORT/notion/mini-app/api/;
            proxy_set_header Host \$host;
            proxy_set_header X-Real-IP \$remote_addr;
            proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto \$scheme;
            proxy_redirect off;
            
            # Increase timeouts for API calls
            proxy_connect_timeout 60s;
            proxy_send_timeout 60s;
            proxy_read_timeout 60s;
            send_timeout 60s;
        }
        
        # Telegram webhook endpoint for receiving reactions
        location /telegram/webhook {
            proxy_pass http://localhost:$APP_PORT/telegram/webhook;
            proxy_set_header Host \$host;
            proxy_set_header X-Real-IP \$remote_addr;
            proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto \$scheme;
            proxy_redirect off;
            
            # Telegram webhook timeouts
            proxy_connect_timeout 10s;
            proxy_send_timeout 10s;
            proxy_read_timeout 10s;
        }
    }
}
EOF

echo "Created new nginx.conf file"

# Test the configuration
echo "Testing Nginx configuration..."
if sudo nginx -t; then
    echo "Configuration test successful, restarting Nginx..."
    sudo systemctl restart nginx
    echo "Nginx restarted successfully"
    echo "Your mini app should now be accessible at https://$DOMAIN/notion/mini-app/"
else
    echo "Configuration test failed. Restoring backup..."
    sudo cp $BACKUP_FILE $NGINX_CONF
    echo "Backup restored. Please check your SSL certificate paths and other settings."
fi