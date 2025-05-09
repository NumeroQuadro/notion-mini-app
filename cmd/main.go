package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
	http.HandleFunc("/notion/mini-app/api/recent-tasks", handleRecentTasks)

	// Simple config endpoint that returns environment variables as JSON
	http.HandleFunc("/notion/mini-app/api/config", handleConfig)

	// Debug endpoints - disable in production
	http.HandleFunc("/notion/mini-app/api/debug/task", handleDebugTask)

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

// Handler for providing configuration to the frontend
func handleConfig(w http.ResponseWriter, r *http.Request) {
	log.Printf("Config endpoint called from: %s", r.RemoteAddr)

	// Set headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only handle GET requests
	if r.Method != http.MethodGet {
		log.Printf("Invalid method %s for config endpoint", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check environment (limit what's exposed in production)
	isProd := os.Getenv("ENVIRONMENT") == "production"

	// Initialize Notion client to get database IDs
	notionClient := notion.NewClient()

	// Create config object
	config := map[string]string{
		"MINI_APP_URL": os.Getenv("MINI_APP_URL"),
	}

	// Add boolean flags for available databases
	hasTasksDb := notionClient.GetTasksDatabaseID() != ""
	hasNotesDb := notionClient.GetNotesDatabaseID() != ""

	if hasTasksDb {
		config["HAS_TASKS_DB"] = "true"
	} else {
		config["HAS_TASKS_DB"] = "false"
	}

	if hasNotesDb {
		config["HAS_NOTES_DB"] = "true"
	} else {
		config["HAS_NOTES_DB"] = "false"
	}

	// Add sensitive info only in non-production environments
	if !isProd {
		notionKey := os.Getenv("NOTION_API_KEY")
		tasksDbID := notionClient.GetTasksDatabaseID()
		notesDbID := notionClient.GetNotesDatabaseID()

		// Log available keys (without exposing their values)
		log.Printf("Config: NOTION_API_KEY available: %v", notionKey != "")
		log.Printf("Config: NOTION_TASKS_DATABASE_ID available: %v", tasksDbID != "")
		log.Printf("Config: NOTION_NOTES_DATABASE_ID available: %v", notesDbID != "")

		config["NOTION_API_KEY"] = notionKey
		config["NOTION_TASKS_DATABASE_ID"] = tasksDbID
		config["NOTION_NOTES_DATABASE_ID"] = notesDbID
	}

	// Return configuration as JSON
	jsonData, err := json.Marshal(config)
	if err != nil {
		log.Printf("Error encoding config response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("Sending config response with %d bytes", len(jsonData))
	w.Write(jsonData)
}

type TaskRequest struct {
	Title      string                 `json:"title"`
	Properties map[string]interface{} `json:"properties"`
}

// API handler for tasks
func handleTasks(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling task request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Set content type for all other responses
	w.Header().Set("Content-Type", "application/json")

	// Helper to return JSON error responses
	sendJSONError := func(statusCode int, message string) {
		w.WriteHeader(statusCode)
		err := json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": message,
		})
		if err != nil {
			log.Printf("Error encoding error response: %v", err)
		}
	}

	// Only handle POST requests for task creation
	if r.Method != http.MethodPost {
		sendJSONError(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse the request body
	var taskReq TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&taskReq); err != nil {
		log.Printf("Error decoding task request: %v", err)
		sendJSONError(http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	log.Printf("Received task request: Title=%s, Properties=%+v", taskReq.Title, taskReq.Properties)

	// Validate request
	if taskReq.Title == "" {
		sendJSONError(http.StatusBadRequest, "Task title is required")
		return
	}

	// Get database type from query param or default to "tasks"
	dbType := r.URL.Query().Get("db_type")
	if dbType == "" {
		dbType = "tasks"
	}

	// Initialize Notion client
	notionClient := notion.NewClient()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()
	log.Printf("Creating task in %s database: %s", dbType, taskReq.Title)

	// Create the task in Notion
	err := notionClient.CreateTask(ctx, taskReq.Title, taskReq.Properties, dbType)
	if err != nil {
		log.Printf("Error creating task in Notion: %v", err)
		sendJSONError(http.StatusInternalServerError, "Failed to create task: "+err.Error())
		return
	}

	elapsed := time.Since(start)
	log.Printf("Task created successfully in %v", elapsed)

	// Return success response
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Task created successfully",
	}); err != nil {
		log.Printf("Error encoding success response: %v", err)
	}
}

// Handler for database properties API
func handleProperties(w http.ResponseWriter, r *http.Request) {
	log.Printf("Properties API called from: %s %s", r.RemoteAddr, r.URL.Path)

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Set content type for the response
	w.Header().Set("Content-Type", "application/json")

	// Helper function for error responses
	sendJSONError := func(statusCode int, message string, properties map[string]map[string]interface{}) {
		response := map[string]interface{}{
			"error": message,
		}

		// Include properties if available
		if properties != nil && len(properties) > 0 {
			response["properties"] = properties
		}

		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}

	// Only process GET requests
	if r.Method != http.MethodGet {
		sendJSONError(http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	// Get database type from query param or default to "tasks"
	dbType := r.URL.Query().Get("db_type")
	if dbType == "" {
		dbType = "tasks"
	}

	log.Printf("Fetching properties for database type: %s", dbType)

	// Initialize Notion client
	notionClient := notion.NewClient()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch database properties from Notion
	properties, err := notionClient.GetDatabaseProperties(ctx, dbType)

	// Transform the properties to a more frontend-friendly format
	result := make(map[string]map[string]interface{})

	// If there was an error but we still want to proceed
	var errorMessage string
	var buttonPropertyDetected bool
	var partialSuccess bool = false

	if err != nil {
		errorMessage = fmt.Sprintf("Failed to get full database properties: %v", err)
		log.Printf("Error getting database properties: %v", err)

		// Check if it's a button property error
		if strings.Contains(strings.ToLower(err.Error()), "button") ||
			strings.Contains(err.Error(), "unsupported property type") {
			buttonPropertyDetected = true
			log.Printf("Button property detected, will continue with partial properties")
			partialSuccess = properties != nil && len(properties) > 0
		} else if properties == nil || len(properties) == 0 {
			// For other errors with no properties, return the error
			sendJSONError(http.StatusInternalServerError, errorMessage, nil)
			return
		}
	}

	// Process properties if we have them
	if properties != nil {
		for name, prop := range properties {
			propType := prop.GetType()

			// Skip internal properties
			if strings.HasPrefix(name, "_") {
				continue
			}

			// Skip button properties to avoid API errors
			if propType == "button" || propType == "unsupported" {
				buttonPropertyDetected = true
				log.Printf("Skipping unsupported property: %s (type: %s)", name, propType)
				continue
			}

			propInfo := map[string]interface{}{
				"type": propType,
			}

			// Add options for select and multi_select types
			switch propType {
			case "select":
				if selectProp, ok := prop.(*notionapi.SelectPropertyConfig); ok && selectProp.Select.Options != nil {
					options := make([]string, 0)
					for _, opt := range selectProp.Select.Options {
						options = append(options, opt.Name)
					}
					propInfo["options"] = options
				}
			case "multi_select":
				if multiSelectProp, ok := prop.(*notionapi.MultiSelectPropertyConfig); ok && multiSelectProp.MultiSelect.Options != nil {
					options := make([]string, 0)
					for _, opt := range multiSelectProp.MultiSelect.Options {
						options = append(options, opt.Name)
					}
					propInfo["options"] = options
				}
			}

			result[name] = propInfo
		}
	}

	// If there was a button property, send a warning but still include properties
	if buttonPropertyDetected {
		statusCode := http.StatusOK
		if !partialSuccess {
			statusCode = http.StatusPartialContent
		}

		warningMsg := "Database contains button properties which are not fully supported. Some properties may not be shown."
		log.Printf("Returning properties with button property warning: %d properties available", len(result))
		sendJSONError(statusCode, warningMsg, result)
		return
	}

	// Send the property data as JSON
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("Error encoding properties: %v", err)
	}
}

// API handler for logs
func handleLogs(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

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

// Debug endpoint for testing task creation
func handleDebugTask(w http.ResponseWriter, r *http.Request) {
	log.Printf("Debug task endpoint called from: %s", r.RemoteAddr)

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Only handle POST requests
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	// Parse the request
	var req struct {
		Title      string                 `json:"title"`
		Properties map[string]interface{} `json:"properties"`
		DbType     string                 `json:"db_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding debug task request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Set default database type if not provided
	if req.DbType == "" {
		req.DbType = "tasks"
	}

	// Log the request
	log.Printf("Debug task: %+v", req)

	// Initialize Notion client
	notionClient := notion.NewClient()

	// Create task
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := notionClient.CreateTask(ctx, req.Title, req.Properties, req.DbType)
	if err != nil {
		log.Printf("Error creating debug task: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to create task: " + err.Error(),
		})
		return
	}

	// Return success
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Debug task created successfully",
	})
}

// Handler for fetching recent tasks with filtering
func handleRecentTasks(w http.ResponseWriter, r *http.Request) {
	log.Printf("Recent tasks API called from: %s", r.RemoteAddr)

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Set content type for the response
	w.Header().Set("Content-Type", "application/json")

	// Helper function for error responses
	sendJSONError := func(statusCode int, message string) {
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]string{
			"error": message,
		})
	}

	// Only process GET requests
	if r.Method != http.MethodGet {
		sendJSONError(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get database type from query param or default to "tasks"
	dbType := r.URL.Query().Get("db_type")
	if dbType == "" {
		dbType = "tasks"
	}

	// Initialize Notion client
	notionClient := notion.NewClient()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get the recent tasks
	tasks, err := notionClient.GetRecentTasks(ctx, dbType, 10)
	if err != nil {
		log.Printf("Error getting recent tasks: %v", err)
		sendJSONError(http.StatusInternalServerError, fmt.Sprintf("Failed to get recent tasks: %v", err))
		return
	}

	// Return tasks as JSON
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		log.Printf("Error encoding recent tasks: %v", err)
	}
}
