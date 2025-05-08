package notion

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jomei/notionapi"
)

type Client struct {
	client *notionapi.Client
	dbID   string
}

func NewClient() *Client {
	client := notionapi.NewClient(notionapi.Token(os.Getenv("NOTION_API_KEY")))
	return &Client{
		client: client,
		dbID:   os.Getenv("NOTION_DATABASE_ID"),
	}
}

func (c *Client) CreateTask(ctx context.Context, title string, properties map[string]interface{}) error {
	log.Printf("Creating task: %s with properties: %v", title, properties)

	if title == "" {
		return fmt.Errorf("task title cannot be empty")
	}

	// Create the base request with title property
	page := &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			Type:       notionapi.ParentTypeDatabaseID,
			DatabaseID: notionapi.DatabaseID(c.dbID),
		},
		Properties: notionapi.Properties{
			"Name": notionapi.TitleProperty{
				Title: []notionapi.RichText{
					{
						Text: &notionapi.Text{
							Content: title,
						},
					},
				},
			},
		},
	}

	// Add custom properties
	for key, value := range properties {
		log.Printf("Processing property: %s = %v", key, value)

		switch key {
		case "Tags":
			// Handle multi-select property
			if tags, ok := value.([]interface{}); ok {
				var options []notionapi.Option
				for _, tag := range tags {
					if tagStr, ok := tag.(string); ok {
						options = append(options, notionapi.Option{
							Name: tagStr,
						})
					}
				}
				page.Properties[key] = notionapi.MultiSelectProperty{
					MultiSelect: options,
				}
			} else if tagStr, ok := value.(string); ok {
				// Handle single string
				page.Properties[key] = notionapi.MultiSelectProperty{
					MultiSelect: []notionapi.Option{
						{Name: tagStr},
					},
				}
			}

		case "project":
			// Handle select property
			if projectStr, ok := value.(string); ok {
				page.Properties[key] = notionapi.SelectProperty{
					Select: notionapi.Option{
						Name: projectStr,
					},
				}
			}

		case "Date":
			// Handle date property using a simpler approach for now
			if dateStr, ok := value.(string); ok && dateStr != "" {
				// Convert date string to the format expected by Notion
				dateStr = formatDateString(dateStr)

				// Use Select property to store the date as a string for now
				// This is a workaround until we can properly handle the date type
				page.Properties[key] = notionapi.SelectProperty{
					Select: notionapi.Option{
						Name: dateStr,
					},
				}
			}

		default:
			// Handle text properties as default
			if valueStr, ok := value.(string); ok {
				page.Properties[key] = notionapi.RichTextProperty{
					RichText: []notionapi.RichText{
						{
							Text: &notionapi.Text{
								Content: valueStr,
							},
						},
					},
				}
			}
		}
	}

	// Create the page in Notion
	createdPage, err := c.client.Page.Create(ctx, page)
	if err != nil {
		log.Printf("Error creating task in Notion: %v", err)
		return fmt.Errorf("Notion API error: %w", err)
	}

	log.Printf("Task created successfully with ID: %s", createdPage.ID)
	return nil
}

func (c *Client) GetDatabaseProperties(ctx context.Context) (map[string]notionapi.PropertyConfig, error) {
	log.Printf("Fetching database properties for database ID: %s", c.dbID)

	db, err := c.client.Database.Get(ctx, notionapi.DatabaseID(c.dbID))
	if err != nil {
		log.Printf("Error fetching database properties: %v", err)
		return nil, err
	}

	log.Printf("Successfully fetched database properties: %d properties found", len(db.Properties))

	// This helper will be useful for debugging
	for name, config := range db.Properties {
		propType := config.GetType()
		log.Printf("Property: %s, Type: %s", name, propType)

		// For date properties, check if we should handle them differently
		if propType == "date" {
			log.Printf("Found date property: %s", name)
		}
	}

	return db.Properties, nil
}

// formatDateString converts various date formats to a standard format
func formatDateString(date string) string {
	// Check if the date is already in YYYY-MM-DD format
	if len(date) >= 10 && date[4] == '-' && date[7] == '-' {
		// Already in YYYY-MM-DD format, return first 10 chars
		return date[:10]
	}

	// Check MM/DD/YYYY format (common in US)
	if len(date) >= 10 && date[2] == '/' && date[5] == '/' {
		month := date[0:2]
		day := date[3:5]
		year := date[6:10]
		return year + "-" + month + "-" + day
	}

	// For other formats, just return the original and log a warning
	log.Printf("Warning: Unrecognized date format: %s", date)
	return date
}
