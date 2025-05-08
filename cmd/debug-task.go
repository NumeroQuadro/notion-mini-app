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

// Debug script that directly tests creating a task with various property combinations
func debugTaskCreation() {
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

	// Direct Notion client
	client := notionapi.NewClient(notionapi.Token(apiKey))
	ctx := context.Background()

	// Get database properties
	db, err := client.Database.Get(ctx, notionapi.DatabaseID(dbID))
	if err != nil {
		log.Fatalf("Error getting database: %v", err)
	}

	// Display database properties
	fmt.Printf("Database '%s' has %d properties:\n", db.Title[0].PlainText, len(db.Properties))

	// Map to store types for each property
	propTypes := map[string]string{}

	// Store button properties for later reference
	var buttonProps []string

	for name, prop := range db.Properties {
		propType := string(prop.GetType())
		propTypes[name] = propType
		fmt.Printf("- %s (%s)\n", name, propType)

		// Identify button properties
		if propType == "button" {
			buttonProps = append(buttonProps, name)
		}
	}

	fmt.Printf("\nDetected %d button properties: %v\n", len(buttonProps), buttonProps)

	// Now create a task with minimal properties to verify it works
	fmt.Println("\nTesting task creation with minimal properties...")

	// Create a simple page request with just the title
	page := &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			Type:       notionapi.ParentTypeDatabaseID,
			DatabaseID: notionapi.DatabaseID(dbID),
		},
		Properties: notionapi.Properties{
			"Name": notionapi.TitleProperty{
				Title: []notionapi.RichText{
					{
						Text: &notionapi.Text{
							Content: "Debug Task - Minimal",
						},
					},
				},
			},
		},
	}

	// Try to create the page
	createPage, err := client.Page.Create(ctx, page)
	if err != nil {
		fmt.Printf("Error creating minimal task: %v\n", err)
	} else {
		fmt.Printf("Created minimal task with ID: %s\n", createPage.ID)
	}

	// Now try creating a task using our internal client
	fmt.Println("\nTesting task creation using internal client...")
	notionClient := notion.NewClient()

	properties := map[string]interface{}{
		"Date": "2023-05-01",
	}

	// Add a multi-select property if one exists
	for name, propType := range propTypes {
		if propType == "multi_select" {
			properties[name] = []string{"Test Tag"}
			fmt.Printf("Adding multi-select property %s\n", name)
			break
		}
	}

	// Add a select property if one exists
	for name, propType := range propTypes {
		if propType == "select" {
			// Get the first available option
			if sel, ok := db.Properties[name].(*notionapi.SelectPropertyConfig); ok && len(sel.Select.Options) > 0 {
				properties[name] = sel.Select.Options[0].Name
				fmt.Printf("Adding select property %s with value %s\n", name, sel.Select.Options[0].Name)
			}
			break
		}
	}

	// Try to create a task using our client
	err = notionClient.CreateTask(ctx, "Debug Task - Using Client", properties)
	if err != nil {
		fmt.Printf("Error creating task with client: %v\n", err)
	} else {
		fmt.Println("Successfully created task using internal client")
	}

	// If there are button properties, verify handling
	if len(buttonProps) > 0 {
		fmt.Println("\nTesting explicit button property handling...")

		// Deliberately try to set a button property
		testProps := map[string]interface{}{
			buttonProps[0]: "true", // This should be filtered out
			"Date":         "2023-05-02",
		}

		err = notionClient.CreateTask(ctx, "Debug Task - Button Test", testProps)
		if err != nil {
			fmt.Printf("Error with button test: %v\n", err)
		} else {
			fmt.Println("Successfully created task despite including button property (should have been filtered)")
		}
	}

	// Dump JSON representation of all properties
	fmt.Println("\nJSON representation of database properties:")
	jsonData, err := json.MarshalIndent(db.Properties, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling to JSON: %v\n", err)
	} else {
		fmt.Println(string(jsonData))
	}
}
