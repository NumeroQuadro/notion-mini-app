# Notion Task Manager Telegram Mini App

A Telegram mini app that allows users to create and manage tasks in Notion databases directly from Telegram.

## Features

- Create new tasks in Notion databases
- Set custom properties for tasks
- View and manage existing tasks
- Seamless integration with Telegram

## Prerequisites

- Go 1.21 or higher
- Telegram Bot Token
- Notion API Key
- Notion Database ID

## Setup

1. Clone the repository
2. Create a `.env` file with the following variables:
   ```
   TELEGRAM_BOT_TOKEN=your_telegram_bot_token
   NOTION_API_KEY=your_notion_api_key
   NOTION_DATABASE_ID=your_database_id
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Run the application:
   ```bash
   go run cmd/main.go
   ```

## Project Structure

```
.
├── cmd/
│   └── main.go           # Application entry point
├── internal/
│   ├── bot/             # Telegram bot handlers
│   ├── notion/          # Notion API integration
│   └── config/          # Configuration management
├── web/                 # Frontend for Telegram mini app
├── .env                 # Environment variables
├── go.mod              # Go module file
└── README.md           # Project documentation
```

## Development

The project uses:
- Go for the backend
- Telegram Bot API for bot interactions
- Notion API for database operations
- HTML/CSS/JavaScript for the mini app frontend

## License

MIT 