package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
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

	// Create config object
	config := map[string]string{
		"MINI_APP_URL": os.Getenv("MINI_APP_URL"),
	}

	// Add sensitive info only in non-production environments
	if !isProd {
		notionKey := os.Getenv("NOTION_API_KEY")
		notionDBID := os.Getenv("NOTION_DATABASE_ID")

		// Log available keys (without exposing their values)
		log.Printf("Config: NOTION_API_KEY available: %v", notionKey != "")
		log.Printf("Config: NOTION_DATABASE_ID available: %v", notionDBID != "")

		config["NOTION_API_KEY"] = notionKey
		config["NOTION_DATABASE_ID"] = notionDBID
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
		log.Printf("Missing task title")
		sendJSONError(http.StatusBadRequest, "Task title is required")
		return
	}

	// Create Notion client to get database properties
	notionClient := notion.NewClient()
	ctx := context.Background()

	// Get database properties to validate against
	dbProps, err := notionClient.GetDatabaseProperties(ctx)
	if err != nil {
		log.Printf("Warning: Could not fetch database properties: %v", err)
		// Continue but be more cautious with property handling
	}

	// Filter properties to only include supported Notion types
	filteredProperties := make(map[string]interface{})

	// First pass: Convert properties to the correct type and filter out unsupported ones
	for key, value := range taskReq.Properties {
		// Skip known button properties or potentially problematic properties
		if key == "button" || key == "complete" || key == "status" || key == "done" || key == "checkbox" {
			log.Printf("Skipping known button property: %s", key)
			continue
		}

		// If we have database properties, check if this property exists in the database
		if dbProps != nil {
			if prop, exists := dbProps[key]; exists {
				propType := prop.GetType()
				log.Printf("Found property %s with type %s in database", key, propType)

				// Skip button type properties
				if propType == "button" {
					log.Printf("Skipping button property from database: %s", key)
					continue
				}

				// Process property based on its type in the database
				switch propType {
				case "checkbox":
					// Convert checkbox values to bool
					boolValue := false
					switch v := value.(type) {
					case bool:
						boolValue = v
					case string:
						boolValue = v == "true" || v == "yes" || v == "1"
					case float64:
						boolValue = v != 0
					case int:
						boolValue = v != 0
					}
					filteredProperties[key] = boolValue
					continue

				case "number":
					// Try to convert to number if not already
					switch v := value.(type) {
					case float64, int:
						filteredProperties[key] = v
					case string:
						// Try to parse as float
						if parsedFloat, err := strconv.ParseFloat(v, 64); err == nil {
							filteredProperties[key] = parsedFloat
						} else {
							log.Printf("Warning: Could not parse %s as number for property %s, skipping", v, key)
							continue
						}
					default:
						log.Printf("Warning: Unsupported type for number property %s, skipping", key)
						continue
					}
					continue
				}
			} else {
				log.Printf("Property %s does not exist in database schema, skipping", key)
				continue
			}
		}

		// Include the property in filtered set
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
	err = json.NewEncoder(w).Encode(map[string]string{
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
