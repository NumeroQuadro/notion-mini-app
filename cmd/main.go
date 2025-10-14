package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/jomei/notionapi"
	"github.com/numero_quadro/notion-mini-app/internal/bot"
	"github.com/numero_quadro/notion-mini-app/internal/database"
	"github.com/numero_quadro/notion-mini-app/internal/gemini"
	"github.com/numero_quadro/notion-mini-app/internal/notion"
	"github.com/numero_quadro/notion-mini-app/internal/scheduler"
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

	// Get authorized user ID
	authorizedUserID := os.Getenv("AUTHORIZED_USER_ID")
	if authorizedUserID == "" {
		log.Printf("Warning: AUTHORIZED_USER_ID not set, bot will be accessible to anyone")
	} else {
		log.Printf("Bot will be allowed only to user with IDs: %s", authorizedUserID)
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

	// Initialize Gemini client
	geminiClient := gemini.NewClient()

	// Initialize database
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/tasks.db" // Default path
	}
	db, err := database.NewDB(dbPath)
	if err != nil {
		log.Printf("Warning: Failed to initialize database: %v", err)
		log.Printf("Task tagging and notifications will be disabled")
		db = nil
	}
	if db != nil {
		defer db.Close()
	}

	// Initialize Telegram bot
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	botAPI.Debug = true
	log.Printf("Authorized on account %s", botAPI.Self.UserName)

	// Initialize bot handler
	handler := bot.NewHandler(botAPI, notionClient, geminiClient, db)

	// Get authorized user ID for scheduler
	authorizedUserIDInt, err := strconv.ParseInt(authorizedUserID, 10, 64)
	if err != nil {
		log.Printf("Warning: Invalid authorized user ID, scheduler will be disabled")
		authorizedUserIDInt = 0
	}

	// Start scheduler if database and user ID are configured
	if db != nil && authorizedUserIDInt != 0 {
		schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
		defer schedulerCancel()

		schedulerInstance := scheduler.NewScheduler(db, notionClient, botAPI, authorizedUserIDInt, "23:00")
		globalScheduler = schedulerInstance

		// Link scheduler to handler for /cron command
		handler.SetScheduler(schedulerInstance)

		go schedulerInstance.Start(schedulerCtx)
		log.Printf("Scheduler started")
	} else {
		log.Printf("Scheduler disabled (database or authorized user not configured)")
	}

	// Set global variables for webhook handler
	globalHandler = handler
	globalBot = botAPI

	// Check if we should use webhook or polling
	webhookURL := os.Getenv("WEBHOOK_URL")
	useWebhook := webhookURL != ""

	if useWebhook {
		log.Printf("Running in WEBHOOK mode: %s", webhookURL)
		log.Printf("Bot will receive updates via webhook at /telegram/webhook")
		log.Printf("Make sure webhook is configured with: ./setup-webhook.sh")

		// Serve static files and start webhook server
		serveStaticFiles()
	} else {
		log.Printf("Running in POLLING mode (webhook URL not set)")
		log.Printf("WARNING: Reactions will NOT work in polling mode!")
		log.Printf("To enable reactions, set WEBHOOK_URL and run ./setup-webhook.sh")

		// Serve static files for mini app in background
		go serveStaticFiles()

		// Use polling for development
		updateConfig := tgbotapi.NewUpdate(0)
		updateConfig.Timeout = 60
		updateConfig.AllowedUpdates = []string{"message", "callback_query"}

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
				log.Printf("Received callback query: %s", update.CallbackQuery.Data)
			}
		}
	}
}

// handleMessageReactionFromUpdate extracts and handles message_reaction from update
func handleMessageReactionFromUpdate(update tgbotapi.Update, handler *bot.Handler) error {
	// Try to parse message_reaction from raw JSON
	// The tgbotapi library v5.5.1 doesn't natively support message_reaction,
	// but we can access it through reflection or custom unmarshaling

	// For debugging: log the update ID to see if we're receiving reactions
	if update.Message == nil && update.CallbackQuery == nil {
		log.Printf("Received non-standard update: UpdateID=%d", update.UpdateID)
	}

	return nil
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
	http.HandleFunc("/notion/mini-app/api/projects", handleProjects)
	http.HandleFunc("/notion/mini-app/api/update-task-status", handleUpdateTaskStatus)
	http.HandleFunc("/notion/mini-app/api/trigger-check", handleTriggerCheck)

	// Telegram webhook endpoint for receiving reaction updates
	http.HandleFunc("/telegram/webhook", createWebhookHandler())

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
	hasJournalDb := notionClient.GetJournalDatabaseID() != ""
	hasProjectsDb := notionClient.GetProjectsDatabaseID() != ""

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

	if hasJournalDb {
		config["HAS_JOURNAL_DB"] = "true"
	} else {
		config["HAS_JOURNAL_DB"] = "false"
	}

	if hasProjectsDb {
		config["HAS_PROJECTS_DB"] = "true"
	} else {
		config["HAS_PROJECTS_DB"] = "false"
	}

	// Add sensitive info only in non-production environments
	if !isProd {
		notionKey := os.Getenv("NOTION_API_KEY")
		tasksDbID := notionClient.GetTasksDatabaseID()
		notesDbID := notionClient.GetNotesDatabaseID()
		journalDbID := notionClient.GetJournalDatabaseID()
		projectsDbID := notionClient.GetProjectsDatabaseID()

		// Log available keys (without exposing their values)
		log.Printf("Config: NOTION_API_KEY available: %v", notionKey != "")
		log.Printf("Config: NOTION_TASKS_DATABASE_ID available: %v", tasksDbID != "")
		log.Printf("Config: NOTION_NOTES_DATABASE_ID available: %v", notesDbID != "")
		log.Printf("Config: NOTION_JOURNAL_DATABASE_ID available: %v", journalDbID != "")
		log.Printf("Config: NOTION_PROJECTS_DATABASE_ID available: %v", projectsDbID != "")

		config["NOTION_API_KEY"] = notionKey
		config["NOTION_TASKS_DATABASE_ID"] = tasksDbID
		config["NOTION_NOTES_DATABASE_ID"] = notesDbID
		config["NOTION_JOURNAL_DATABASE_ID"] = journalDbID
		config["NOTION_PROJECTS_DATABASE_ID"] = projectsDbID
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
	taskID, err := notionClient.CreateTask(ctx, taskReq.Title, taskReq.Properties, dbType)
	if err != nil {
		log.Printf("Error creating task in Notion: %v", err)
		sendJSONError(http.StatusInternalServerError, "Failed to create task: "+err.Error())
		return
	}

	elapsed := time.Since(start)
	log.Printf("Task created successfully in %v with ID: %s", elapsed, taskID)

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

	_, err := notionClient.CreateTask(ctx, req.Title, req.Properties, req.DbType)
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

// API handler for updating task status
func handleUpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	log.Printf("Update task status API called from: %s", r.RemoteAddr)

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only handle POST requests for updating task status
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request
	var req struct {
		TaskID     string                 `json:"task_id"`
		Status     string                 `json:"status"`
		Properties map[string]interface{} `json:"properties"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding update task status request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.TaskID == "" || req.Status == "" {
		http.Error(w, "Task ID and status are required", http.StatusBadRequest)
		return
	}

	// Initialize Notion client
	notionClient := notion.NewClient()

	// Update task status in Notion
	err := notionClient.UpdateTaskStatus(req.TaskID, req.Status, req.Properties)
	if err != nil {
		log.Printf("Error updating task status: %v", err)
		http.Error(w, "Failed to update task status", http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Task status updated successfully",
	})
}

// Handler for fetching projects
func handleProjects(w http.ResponseWriter, r *http.Request) {
	log.Printf("Projects API called from: %s", r.RemoteAddr)

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

	// Initialize Notion client
	notionClient := notion.NewClient()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get projects from Notion
	projects, err := notionClient.GetProjects(ctx)
	if err != nil {
		sendJSONError(http.StatusInternalServerError, fmt.Sprintf("Failed to get projects: %v", err))
		return
	}

	// Return projects as JSON
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(projects); err != nil {
		log.Printf("Error encoding projects: %v", err)
	}
}

// Handler for manually triggering the daily task check
func handleTriggerCheck(w http.ResponseWriter, r *http.Request) {
	log.Printf("Manual trigger check API called from: %s", r.RemoteAddr)

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

	// Check if scheduler is available
	if globalScheduler == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Scheduler not available",
		})
		return
	}

	// Trigger manual check
	globalScheduler.RunManualCheck()

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Task check triggered successfully",
	})
}

// Global variable to store the bot handler for webhook
var globalHandler *bot.Handler
var globalBot *tgbotapi.BotAPI
var globalScheduler *scheduler.Scheduler

// createWebhookHandler creates a handler for Telegram webhook updates
func createWebhookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the webhook update
		var updateData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			log.Printf("Error decoding webhook data: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		log.Printf("Received webhook update: %+v", updateData)

		// Handle different update types

		// 1. Handle regular messages
		if messageData, ok := updateData["message"]; ok {
			log.Printf("Received message update via webhook")

			// Parse the message
			messageJSON, _ := json.Marshal(messageData)
			var message tgbotapi.Message
			if err := json.Unmarshal(messageJSON, &message); err == nil && globalHandler != nil {
				if err := globalHandler.HandleMessage(&message); err != nil {
					log.Printf("Error handling message: %v", err)
				}
			}
		}

		// 1.5. Handle edited messages (update pending task)
		if editedMessageData, ok := updateData["edited_message"]; ok {
			log.Printf("Received edited message update via webhook")

			// Parse the edited message
			messageJSON, _ := json.Marshal(editedMessageData)
			var message tgbotapi.Message
			if err := json.Unmarshal(messageJSON, &message); err == nil && globalHandler != nil {
				// Update the pending task with new text
				if err := globalHandler.HandleMessage(&message); err != nil {
					log.Printf("Error handling edited message: %v", err)
				}
			}
		}

		// 2. Handle callback queries
		if callbackData, ok := updateData["callback_query"]; ok {
			log.Printf("Received callback query via webhook: %+v", callbackData)
		}

		// 3. Handle message reactions
		if messageReactionData, ok := updateData["message_reaction"]; ok {
			log.Printf("Received message_reaction update: %+v", messageReactionData)

			// Parse the reaction update
			reactionJSON, err := json.Marshal(messageReactionData)
			if err != nil {
				log.Printf("Error marshaling reaction data: %v", err)
				w.WriteHeader(http.StatusOK)
				return
			}

			var reaction bot.MessageReactionUpdate
			if err := json.Unmarshal(reactionJSON, &reaction); err != nil {
				log.Printf("Error unmarshaling reaction: %v", err)
				w.WriteHeader(http.StatusOK)
				return
			}

			// Handle the reaction
			if globalHandler != nil {
				if err := globalHandler.HandleMessageReaction(&reaction); err != nil {
					log.Printf("Error handling reaction: %v", err)
				}
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}
