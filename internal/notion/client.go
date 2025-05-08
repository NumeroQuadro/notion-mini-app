package notion

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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

	// Get database properties to check for button types
	dbProps, err := c.GetDatabaseProperties(ctx)
	if err != nil {
		log.Printf("Warning: Could not fetch database properties: %v", err)
		// Continue anyway but be more cautious
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

	// Add custom properties - but filter out button properties
	for key, value := range properties {
		log.Printf("Processing property: %s = %v", key, value)

		// Skip known button properties or properties that might be buttons
		if key == "complete" || key == "status" {
			log.Printf("Skipping potential button property: %s", key)
			continue
		}

		// If we have database properties, check the property type
		if dbProps != nil {
			if prop, exists := dbProps[key]; exists {
				propType := prop.GetType()
				log.Printf("Property %s has type: %s", key, propType)

				// Skip button type properties
				if propType == "button" {
					log.Printf("Skipping button property: %s", key)
					continue
				}
			}
		}

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
			// Handle date property correctly using Notion's DateProperty
			if dateStr, ok := value.(string); ok && dateStr != "" {
				// Parse and convert to Notion's Date type
				parsedDate := parseToNotionDate(dateStr)

				// Create a DateProperty with the proper structure required by Notion
				page.Properties[key] = notionapi.DateProperty{
					Date: &notionapi.DateObject{
						Start: parsedDate,
						End:   nil, // End date is optional and can be nil
					},
				}

				log.Printf("Added Date property: %s", parsedDate.String())
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

// parseToNotionDate converts a string to a Notion Date pointer
func parseToNotionDate(dateStr string) *notionapi.Date {
	formattedStr := formatDateString(dateStr)

	// Parse the formatted date string to a time.Time
	t, err := time.Parse("2006-01-02", formattedStr)
	if err != nil {
		log.Printf("Error parsing date %s: %v", formattedStr, err)
		return nil
	}

	// Convert to Notion Date type
	notionDate := notionapi.Date(t)
	return &notionDate
}

// formatDateString converts various date formats to the format Notion expects
// Notion requires ISO 8601 date strings (YYYY-MM-DD) for dates
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

	// Try to parse with Go's time package
	layouts := []string{
		"01/02/2006", // MM/DD/YYYY
		"2006-01-02", // YYYY-MM-DD
		"02-01-2006", // DD-MM-YYYY
		"01-02-2006", // MM-DD-YYYY
		time.RFC3339, // YYYY-MM-DDTHH:MM:SSZ
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, date); err == nil {
			return t.Format("2006-01-02") // Return as YYYY-MM-DD
		}
	}

	// For other formats, just return the original and log a warning
	log.Printf("Warning: Unrecognized date format: %s", date)
	return date
}
