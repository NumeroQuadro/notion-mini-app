package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/jomei/notionapi"
	"github.com/numero_quadro/notion-mini-app/internal/notion"
)

// Simple script to dump all database properties to help diagnose issues
func debugDatabaseProperties() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	// Get Notion API key and database ID
	apiKey := os.Getenv("NOTION_API_KEY")
	dbID := os.Getenv("NOTION_DATABASE_ID")

	if apiKey == "" || dbID == "" {
		log.Fatal("NOTION_API_KEY and NOTION_DATABASE_ID must be set")
	}

	fmt.Printf("Using database ID: %s\n", dbID)

	// Create Notion client
	client := notionapi.NewClient(notionapi.Token(apiKey))

	// Get database details
	ctx := context.Background()
	db, err := client.Database.Get(ctx, notionapi.DatabaseID(dbID))
	if err != nil {
		log.Fatalf("Error getting database: %v", err)
	}

	fmt.Printf("Database found: %s\n", db.Title[0].PlainText)
	fmt.Printf("Total properties: %d\n\n", len(db.Properties))

	// Print detailed info about each property
	fmt.Println("PROPERTY DETAILS:")
	fmt.Println("----------------")

	// First, print as original JSON to see raw data
	for name, prop := range db.Properties {
		propType := prop.GetType()
		fmt.Printf("Property: %s (Type: %s)\n", name, propType)

		// Convert to JSON to see all fields
		jsonData, err := json.MarshalIndent(prop, "  ", "  ")
		if err != nil {
			fmt.Printf("  Error marshaling to JSON: %v\n", err)
			continue
		}
		fmt.Printf("  Raw data: %s\n\n", string(jsonData))
	}

	// Now try using our internal client
	fmt.Println("\nUSING INTERNAL CLIENT:")
	fmt.Println("--------------------")
	notionClient := notion.NewClient()
	properties, err := notionClient.GetDatabaseProperties(ctx)
	if err != nil {
		log.Fatalf("Error using internal client: %v", err)
	}

	for name, prop := range properties {
		propType := prop.GetType()
		fmt.Printf("Property: %s (Type: %s)\n", name, propType)
	}
}

// Add a debug command to main.go
func runDebugMode() {
	debugDatabaseProperties()
}
