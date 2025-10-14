# Quick Start Guide

## Быстрый старт для локального тестирования

### 1. Подготовка

```bash
# Клонировать репозиторий
git clone <your-repo>
cd notion-mini-app

# Установить зависимости
go mod download
```

### 2. Создать .env файл

```bash
cat > .env << 'EOF'
# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN=your_bot_token_from_botfather
AUTHORIZED_USER_ID=your_telegram_user_id

# Notion Configuration
NOTION_API_KEY=your_notion_api_key
NOTION_TASKS_DATABASE_ID=your_tasks_database_id
NOTION_NOTES_DATABASE_ID=your_notes_database_id

# Server Configuration
HOST=0.0.0.0
PORT=8080
MINI_APP_URL=https://your-domain.com/notion/mini-app

# Webhook Configuration (for reactions)
WEBHOOK_URL=https://your-domain.com/telegram/webhook
EOF
```

**Как получить необходимые ID:**
- `TELEGRAM_BOT_TOKEN`: Создайте бота через [@BotFather](https://t.me/BotFather)
- `AUTHORIZED_USER_ID`: Узнайте свой ID через [@userinfobot](https://t.me/userinfobot)
- `NOTION_API_KEY`: Создайте интеграцию на [notion.so/my-integrations](https://www.notion.so/my-integrations)
- `NOTION_TASKS_DATABASE_ID`: ID вашей базы данных задач в Notion

### 3. Запустить бот локально

```bash
go run cmd/main.go
```

**На этом этапе работают:**
- ✅ Обычные сообщения (хранятся в памяти)
- ✅ Mini App интерфейс (доступен на http://localhost:8080)
- ❌ Реакции (требуют webhook с HTTPS)

### 4. Настроить webhook для реакций (опционально)

Для тестирования реакций локально используйте ngrok:

```bash
# В отдельном терминале запустите ngrok
ngrok http 8080

# Скопируйте https URL (например: https://abc123.ngrok.io)
# Обновите WEBHOOK_URL в .env:
WEBHOOK_URL=https://abc123.ngrok.io/telegram/webhook

# Настройте webhook
./setup-webhook.sh
```

### 5. Протестировать

1. Откройте Telegram и найдите вашего бота
2. Отправьте `/start`
3. Отправьте любое сообщение, например: "Протестировать бота"
4. Поставьте любую реакцию (👍, ❤️, etc.) на своё сообщение
5. Бот должен:
   - Создать задачу в Notion
   - Поставить ✅ реакцию в подтверждение

## Деплой в продакшн

### Вариант 1: VPS с nginx

```bash
# 1. Склонировать на сервер
git clone <your-repo>
cd notion-mini-app

# 2. Настроить .env с production значениями
nano .env

# 3. Собрать
go build -o notion-bot cmd/main.go

# 4. Настроить systemd service
sudo nano /etc/systemd/system/notion-bot.service
```

**notion-bot.service:**
```ini
[Unit]
Description=Notion Telegram Bot
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/path/to/notion-mini-app
ExecStart=/path/to/notion-mini-app/notion-bot
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
# 5. Запустить сервис
sudo systemctl daemon-reload
sudo systemctl enable notion-bot
sudo systemctl start notion-bot

# 6. Настроить nginx для webhook
sudo ./nginx-setup.sh

# 7. Настроить webhook
./setup-webhook.sh
```

### Вариант 2: Docker

```bash
# TODO: Добавить Dockerfile
```

## Troubleshooting

### Бот не отвечает
```bash
# Проверьте логи
journalctl -u notion-bot -f

# Проверьте, запущен ли бот
ps aux | grep notion-bot
```

### Реакции не работают
```bash
# Проверьте статус webhook
curl "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo"

# Должно показать:
# - url: ваш webhook URL
# - allowed_updates: ["message", "message_reaction"]
```

### Задачи не создаются в Notion
```bash
# Проверьте права интеграции Notion
# База данных должна быть расшарена с вашей интеграцией

# Проверьте логи на ошибки Notion API
journalctl -u notion-bot | grep -i "notion"
```

## Полезные команды

```bash
# Пересобрать бот
go build -o notion-bot cmd/main.go

# Проверить webhook
curl "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo"

# Удалить webhook (вернуться на polling)
curl -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/deleteWebhook"

# Установить webhook заново
./setup-webhook.sh

# Просмотреть логи
tail -f /var/log/notion-bot/bot.log  # or
journalctl -u notion-bot -f
```

## Дальнейшие улучшения

- [ ] Добавить Docker support
- [ ] Добавить персистентное хранилище для pending tasks
- [ ] Добавить метрики и мониторинг
- [ ] Добавить rate limiting
- [ ] Добавить поддержку нескольких пользователей
