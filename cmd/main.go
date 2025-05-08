package main

import (
	"context"
	"encoding/json"
	"flag"
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
	// Define command-line flags
	debugMode := flag.Bool("debug", false, "Run in debug mode to diagnose database properties")
	debugTaskMode := flag.Bool("debug-task", false, "Run in debug task mode to test task creation directly")
	flag.Parse()

	// Run debug mode if requested
	if *debugMode {
		log.Println("Debug mode not yet implemented - run 'go run cmd/debug.go' separately")
		return
	}

	// Run debug task mode if requested
	if *debugTaskMode {
		log.Println("Debug task mode not yet implemented - run 'go run cmd/debug-task.go' separately")
		return
	}

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
		log.Printf("Missing task title")
		sendJSONError(http.StatusBadRequest, "Task title is required")
		return
	}

	// Create Notion client to get database properties
	notionClient := notion.NewClient()
	ctx := context.Background()

	// Filter properties to only include supported Notion types
	filteredProperties := make(map[string]interface{})
	for key, value := range taskReq.Properties {
		// Skip properties that are not supported by Notion
		if key == "button" || key == "complete" || key == "status" || key == "done" {
			continue
		}
		filteredProperties[key] = value
	}

	log.Printf("After filtering: %d properties remaining of %d original properties",
		len(filteredProperties), len(taskReq.Properties))

	// Create the task in Notion with timeout context
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := notionClient.CreateTask(ctx, taskReq.Title, filteredProperties); err != nil {
		log.Printf("Error creating task in Notion: %v", err)
		sendJSONError(http.StatusInternalServerError, "Failed to create task: "+err.Error())
		return
	}

	// Return success
	w.WriteHeader(http.StatusCreated)
	err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Task created successfully",
	})
	if err != nil {
		log.Printf("Error encoding success response: %v", err)
	}
}

// API handler for database properties
func handleProperties(w http.ResponseWriter, r *http.Request) {
	// Only handle GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Handling properties request from %s", r.RemoteAddr)

	// Set CORS headers to allow requests from any origin
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Try to get properties from Notion
	notionClient := notion.NewClient()
	ctx := context.Background()
	properties, err := notionClient.GetDatabaseProperties(ctx)

	// Prepare a map for the response
	simplifiedProps := make(map[string]map[string]interface{})

	if err != nil {
		// Log the error
		log.Printf("Error fetching properties from Notion: %v", err)

		// Create default properties
		simplifiedProps = map[string]map[string]interface{}{
			"Name": {
				"type":     "title",
				"required": true,
			},
			"Tags": {
				"type":    "multi_select",
				"options": []string{"sometimes-later"},
			},
			"project": {
				"type":    "select",
				"options": []string{"household-tasks", "the-wellness-hub"},
			},
			"Date": {
				"type": "date",
			},
			"Complete": {
				"type": "checkbox",
			},
		}
	} else {
		// Convert properties to a more frontend-friendly format
		for name, prop := range properties {
			propType := string(prop.GetType())
			propInfo := map[string]interface{}{
				"type": propType,
			}

			// Add more specific info based on property type
			switch propType {
			case "multi_select":
				if multiSelect, ok := prop.(*notionapi.MultiSelectPropertyConfig); ok {
					var options []string
					for _, option := range multiSelect.MultiSelect.Options {
						options = append(options, option.Name)
					}
					propInfo["options"] = options
				}
			case "select":
				if sel, ok := prop.(*notionapi.SelectPropertyConfig); ok {
					var options []string
					for _, option := range sel.Select.Options {
						options = append(options, option.Name)
					}
					propInfo["options"] = options
				}
			case "title":
				propInfo["required"] = true
			case "checkbox":
				propInfo["type"] = "checkbox"
			case "date":
				propInfo["type"] = "date"
			}

			simplifiedProps[name] = propInfo
		}
	}

	if err := json.NewEncoder(w).Encode(simplifiedProps); err != nil {
		log.Printf("Error encoding properties response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
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

// Debug endpoint for creating a task with minimal properties
func handleDebugTask(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling debug task request from %s", r.RemoteAddr)

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Only allow in non-production environments
	if os.Getenv("ENVIRONMENT") == "production" {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Debug endpoints not available in production",
		})
		return
	}

	// Create a task with minimal properties
	notionClient := notion.NewClient()
	ctx := context.Background()

	// Get the title from query parameter or use default
	title := r.URL.Query().Get("title")
	if title == "" {
		title = "Debug Task - " + time.Now().Format(time.RFC3339)
	}

	// Minimal properties (just a date)
	properties := map[string]interface{}{
		"Date": time.Now().Format("2006-01-02"),
	}

	// Create the task
	err := notionClient.CreateTask(ctx, title, properties)

	if err != nil {
		log.Printf("Error creating debug task: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
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
