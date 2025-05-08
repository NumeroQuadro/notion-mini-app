package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/jomei/notionapi"
	"github.com/numero_quadro/notion-mini-app/internal/bot"
	"github.com/numero_quadro/notion-mini-app/internal/notion"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	// Get bot token from environment
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is not set")
	}

	// Get mini app URL
	miniAppURL := os.Getenv("MINI_APP_URL")
	if miniAppURL == "" {
		log.Printf("Warning: MINI_APP_URL not set, using default")
		miniAppURL = "https://tralalero-tralala.ru/notion/mini-app"
	}
	log.Printf("Mini App URL: %s", miniAppURL)

	// Initialize Notion client
	notionClient := notion.NewClient()

	// Initialize Telegram bot
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	botAPI.Debug = true
	log.Printf("Authorized on account %s", botAPI.Self.UserName)

	// Initialize bot handler
	handler := bot.NewHandler(botAPI, notionClient)

	// Serve static files for mini app
	go serveStaticFiles()

	// Use polling for development
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := botAPI.GetUpdatesChan(updateConfig)

	// Handle updates
	for update := range updates {
		if update.Message != nil {
			// Handle incoming messages
			if err := handler.HandleMessage(update.Message); err != nil {
				log.Printf("Error handling message: %v", err)
			}
		} else if update.CallbackQuery != nil {
			// Handle callback queries (button clicks)
			if err := handler.HandleCallback(update.CallbackQuery); err != nil {
				log.Printf("Error handling callback: %v", err)
			}
		}
	}
}

// Serve static files for the mini app
func serveStaticFiles() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	// Create a file server handler for the root
	fs := http.FileServer(http.Dir("./web"))

	// For the mini app path
	http.Handle("/notion/mini-app/", http.StripPrefix("/notion/mini-app/", fs))

	// API endpoints
	http.HandleFunc("/notion/mini-app/api/tasks", handleTasks)
	http.HandleFunc("/notion/mini-app/api/properties", handleProperties)
	http.HandleFunc("/notion/mini-app/api/log", handleLogs)

	// Also serve files at the root for local development
	http.Handle("/", fs)

	// Start the server
	server := &http.Server{
		Addr:         host + ":" + port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Starting mini app server on %s", server.Addr)
	log.Printf("Mini app available at: http://%s:%s/notion/mini-app/", host, port)
	log.Fatal(server.ListenAndServe())
}

type TaskRequest struct {
	Title      string                 `json:"title"`
	Properties map[string]interface{} `json:"properties"`
}

// API handler for tasks
func handleTasks(w http.ResponseWriter, r *http.Request) {
	// Only handle POST requests for task creation
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var taskReq TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&taskReq); err != nil {
		log.Printf("Error decoding task request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create the task in Notion
	notionClient := notion.NewClient()
	ctx := context.Background()
	if err := notionClient.CreateTask(ctx, taskReq.Title, taskReq.Properties); err != nil {
		log.Printf("Error creating task in Notion: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Task created successfully",
	})
}

// API handler for database properties
func handleProperties(w http.ResponseWriter, r *http.Request) {
	// Only handle GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get properties from Notion
	notionClient := notion.NewClient()
	ctx := context.Background()
	properties, err := notionClient.GetDatabaseProperties(ctx)
	if err != nil {
		log.Printf("Error fetching properties from Notion: %v", err)
		http.Error(w, "Failed to fetch properties", http.StatusInternalServerError)
		return
	}

	// Convert properties to a more frontend-friendly format
	simplifiedProps := make(map[string]map[string]interface{})

	for name, prop := range properties {
		propInfo := map[string]interface{}{
			"type": string(prop.GetType()),
		}

		// Add more specific info based on property type
		switch prop.GetType() {
		case "multi_select":
			if multiSelect, ok := prop.(*notionapi.MultiSelectPropertyConfig); ok {
				options := make([]string, 0)
				for _, opt := range multiSelect.MultiSelect.Options {
					options = append(options, opt.Name)
				}
				propInfo["options"] = options
			}
		case "select":
			if selectProp, ok := prop.(*notionapi.SelectPropertyConfig); ok {
				options := make([]string, 0)
				for _, opt := range selectProp.Select.Options {
					options = append(options, opt.Name)
				}
				propInfo["options"] = options
			}
		}

		simplifiedProps[name] = propInfo
	}

	// Return the properties
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(simplifiedProps)
}

// API handler for logs
func handleLogs(w http.ResponseWriter, r *http.Request) {
	// Only handle POST requests for logging
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the log data
	var logData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&logData); err != nil {
		log.Printf("Error decoding log data: %v", err)
		http.Error(w, "Invalid log data", http.StatusBadRequest)
		return
	}

	// Log the data server-side
	log.Printf("Client log: %v", logData)

	// Return success
	w.WriteHeader(http.StatusOK)
}
