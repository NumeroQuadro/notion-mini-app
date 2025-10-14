# Changelog - Reaction-Based Task Creation

## Summary

Изменена логика бота для уменьшения спама в чате:
- ❌ Убрано: Диалоги с подтверждением "да/нет"
- ✅ Добавлено: Создание задач через реакции на сообщения
- ✅ Добавлено: Бот подтверждает создание задачи своей реакцией ✅

## Как теперь работает

1. **Отправьте сообщение** боту с текстом задачи (например: "Купить молоко")
2. **Поставьте реакцию** (любой эмодзи) на своё сообщение
3. **Бот создаст задачу** в Notion
4. **Бот поставит ✅** на ваше сообщение в подтверждение

## Изменённые файлы

### 1. `/internal/bot/handler.go`
**Изменения:**
- Удалены состояния `StateAwaitingConfirmation` и структура `UserState`
- Добавлена структура `PendingTask` для хранения сообщений в ожидании реакции
- Метод `HandleMessage()` теперь просто сохраняет сообщение без ответа
- Добавлены типы для работы с реакциями: `MessageReactionUpdate`, `ChatInfo`, `UserInfo`, `ReactionType`
- Добавлен метод `HandleMessageReaction()` для обработки реакций
- Добавлен метод `setMessageReaction()` для установки реакции бота через прямой API вызов
- Удалены методы подтверждения: `askForConfirmation()`, `createTaskFromPending()`, `HandleCallback()` и связанные

### 2. `/cmd/main.go`
**Изменения:**
- Добавлены глобальные переменные `globalHandler` и `globalBot` для webhook
- Обновлён `updateConfig.AllowedUpdates` для получения `message_reaction`
- Добавлена функция `handleMessageReactionFromUpdate()` для обработки реакций из updates
- Добавлен webhook endpoint `/telegram/webhook` для получения реакций
- Добавлена функция `createWebhookHandler()` для обработки webhook запросов от Telegram

### 3. `/README.md`
**Изменения:**
- Обновлено описание функциональности
- Добавлена секция "How It Works" с объяснением работы реакций
- Добавлены инструкции по настройке webhook
- Добавлены новые переменные окружения: `AUTHORIZED_USER_ID`, `WEBHOOK_URL`
- Добавлены важные замечания о требовании webhook для реакций

### 4. Новые файлы

#### `/setup-webhook.sh`
Скрипт для автоматической настройки webhook:
- Читает конфигурацию из .env
- Настраивает webhook на Telegram API
- Проверяет статус webhook
- Выводит инструкции по удалению webhook

#### `/TESTING.md`
Документация по тестированию:
- Инструкции для локального тестирования
- Примеры использования ngrok/localhost.run
- Troubleshooting guide
- Объяснение архитектуры (polling + webhook)

## Технические детали

### Почему webhook для реакций?

Telegram Bot API отправляет обновления реакций (`message_reaction`) **только через webhook**, не через long polling. Поэтому используется гибридный подход:

- **Long Polling**: Для обычных сообщений (проще для разработки, не требует HTTPS)
- **Webhook**: Для реакций (требуется Telegram API)

### Архитектура

```
[User sends message]
        ↓
[Long Polling → Bot receives message]
        ↓
[Bot stores in pendingTasks map] ← No response!
        ↓
[User adds reaction]
        ↓
[Webhook receives message_reaction update]
        ↓
[Bot checks pendingTasks]
        ↓
[Bot creates task in Notion]
        ↓
[Bot adds ✅ reaction]
```

### Хранение в памяти

Pending tasks хранятся в памяти в структуре:
```go
map[int64]map[int]*PendingTask
// userID → messageID → task
```

**Ограничение**: При перезапуске бота pending tasks теряются. Это приемлемо, так как:
1. Задачи создаются обычно быстро (в течение минут)
2. Пользователь всегда может отправить сообщение заново
3. Нет необходимости в персистентном хранилище для temporary state

## Требования для деплоя

1. **HTTPS endpoint**: Webhook требует публичного HTTPS URL
2. **Переменные окружения**:
   - `WEBHOOK_URL` - URL для webhook (e.g., https://yourdomain.com/telegram/webhook)
   - `AUTHORIZED_USER_ID` - ID Telegram пользователя (для ограничения доступа)
3. **Настроенный webhook**: Выполнить `./setup-webhook.sh` после деплоя

## Преимущества нового подхода

✅ **Меньше спама**: Нет сообщений-подтверждений  
✅ **Быстрее**: Одна реакция вместо диалога  
✅ **Интуитивнее**: Естественный интерфейс через реакции  
✅ **Чище чат**: Только ваши сообщения и реакция бота  

## Backward Compatibility

- Mini App продолжает работать как раньше
- `/start` команда работает
- Авторизация по AUTHORIZED_USER_ID сохранена
- Все API endpoints для mini app без изменений

## Следующие шаги

1. Задеплоить на сервер с HTTPS
2. Установить WEBHOOK_URL в .env
3. Запустить `./setup-webhook.sh`
4. Протестировать создание задач через реакции
