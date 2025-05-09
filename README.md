# Notion Task Manager Telegram Mini App

A Telegram mini app that serves only for one user (its me hahaha) and allows to create and manage tasks in Notion databases directly from Telegram.

## Features

- Create new tasks in Notion databases
- Set custom properties for tasks
- View and manage existing tasks
- Seamless integration with Telegram
- Support for multiple database types (tasks and notes)
- Graceful handling of button properties in Notion databases
- Streamlined, simplified interface for reliable performance

## Prerequisites

- Go 1.21 or higher
- Telegram Bot Token
- Notion API Key
- Notion Database IDs (tasks and/or notes)

## Setup

1. Clone the repository
2. Create a `.env` file with the following variables:
   ```
   TELEGRAM_BOT_TOKEN=your_telegram_bot_token
   NOTION_API_KEY=your_notion_api_key
   NOTION_TASKS_DATABASE_ID=your_tasks_database_id
   NOTION_NOTES_DATABASE_ID=your_notes_database_id
   MINI_APP_URL=https://your-domain.com/notion/mini-app
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
│   └── notion/          # Notion API integration
├── web/                 # Frontend for Telegram mini app
│   ├── index.html       # HTML structure
│   ├── app.js           # JavaScript functionality
│   └── styles.css       # Styling
├── .env                 # Environment variables
├── go.mod              # Go module file
└── README.md           # Project documentation
```

## Multiple Database Support

The app supports both tasks and notes databases:

1. Configure both database IDs in the `.env` file
2. Switch between databases using the tabs in the UI
3. Each database can have its own unique properties

## Error Handling

The app includes robust error handling to ensure reliability:
1. User-friendly error messages
2. Graceful handling of API limitations
3. Clean recovery from network issues

## Development

The project uses:
- Go for the backend
- Telegram Bot API for bot interactions
- Notion API for database operations
- HTML/CSS/JavaScript for the mini app frontend

## Deployment

A simple nginx setup script is included to help with deployment:

```bash
./nginx-setup.sh
```

This will set up the necessary configuration for serving the mini app through Nginx.

## License

MIT 