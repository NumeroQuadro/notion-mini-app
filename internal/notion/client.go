package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jomei/notionapi"
)

type Client struct {
	client *notionapi.Client
	dbID   string
}

func NewClient() *Client {
	apiToken := os.Getenv("NOTION_API_KEY")
	dbID := os.Getenv("NOTION_DATABASE_ID")

	if apiToken == "" {
		log.Printf("WARNING: NOTION_API_KEY environment variable is not set")
	}

	if dbID == "" {
		log.Printf("WARNING: NOTION_DATABASE_ID environment variable is not set")
	}

	// Create standard Notion client
	client := notionapi.NewClient(notionapi.Token(apiToken))

	return &Client{
		client: client,
		dbID:   dbID,
	}
}

func (c *Client) CreateTask(ctx context.Context, title string, properties map[string]interface{}) error {
	log.Printf("Creating task: %s with properties: %v", title, properties)

	if title == "" {
		return fmt.Errorf("task title cannot be empty")
	}

	// Check if context has a deadline (timeout)
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		log.Printf("Context has deadline, %v remaining", remaining)
	} else {
		// If no timeout set, add one to prevent hanging
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		log.Printf("Added 30s timeout to context")
	}

	// Get database properties to check for button types
	dbProps, err := c.GetDatabaseProperties(ctx)
	if err != nil {
		log.Printf("Warning: Could not fetch database properties: %v", err)
		// Continue anyway but be more cautious
	}

	// Log all database properties for debugging
	if dbProps != nil {
		log.Printf("Database properties found: %d", len(dbProps))
		for name, prop := range dbProps {
			propType := prop.GetType()
			log.Printf("DB Property: %s (Type: %s)", name, propType)
		}
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
		if key == "complete" || key == "status" || key == "done" || key == "button" ||
			key == "checkbox" || strings.Contains(strings.ToLower(key), "button") {
			log.Printf("Skipping known button-like property: %s", key)
			continue
		}

		// If we have database properties, check the property type
		if dbProps != nil {
			if prop, exists := dbProps[key]; exists {
				propType := prop.GetType()
				log.Printf("Property %s has type: %s", key, propType)

				// Skip button type properties and any other unsupported types
				if propType == "button" || propType == "unsupported" {
					log.Printf("Skipping unsupported property type: %s (type: %s)", key, propType)
					continue
				}

				// Handle property based on its type in the database
				switch propType {
				case "multi_select":
					c.handleMultiSelectProperty(page, key, value)
					continue
				case "select":
					c.handleSelectProperty(page, key, value)
					continue
				case "date":
					c.handleDateProperty(page, key, value)
					continue
				case "checkbox":
					c.handleCheckboxProperty(page, key, value)
					continue
				case "rich_text":
					c.handleTextProperty(page, key, value)
					continue
				case "number":
					c.handleNumberProperty(page, key, value)
					continue
				case "url":
					c.handleURLProperty(page, key, value)
					continue
				case "email":
					c.handleEmailProperty(page, key, value)
					continue
				case "phone_number":
					c.handlePhoneProperty(page, key, value)
					continue
				}
			} else {
				// Property doesn't exist in database schema
				log.Printf("Property %s does not exist in database schema, skipping", key)
				continue
			}
		}

		// Fallback logic for when we couldn't determine property type or don't have schema
		switch key {
		case "Tags":
			c.handleMultiSelectProperty(page, key, value)

		case "project":
			c.handleSelectProperty(page, key, value)

		case "Date":
			c.handleDateProperty(page, key, value)

		default:
			// Handle text properties as default
			c.handleTextProperty(page, key, value)
		}
	}

	log.Printf("Sending create page request to Notion API")
	creationStart := time.Now()

	// Create the page in Notion
	createdPage, err := c.client.Page.Create(ctx, page)

	elapsedTime := time.Since(creationStart)
	log.Printf("Notion API request took %v", elapsedTime)

	if err != nil {
		log.Printf("Error creating task in Notion: %v", err)

		// Check for context deadline exceeded
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("request to Notion API timed out after %v", elapsedTime)
		}

		// Check if it's a button property error
		if err.Error() == "unsupported property type: button" {
			log.Printf("CRITICAL: Button property error detected despite filtering!")
			log.Printf("Properties after filtering: %v", page.Properties)

			// Further diagnose which property might be causing it
			for key, prop := range page.Properties {
				propJSON, _ := json.Marshal(prop)
				log.Printf("Property %s: %s", key, string(propJSON))
			}
		}

		return fmt.Errorf("Notion API error: %w", err)
	}

	log.Printf("Task created successfully with ID: %s", createdPage.ID)
	return nil
}

// Helper methods for handling different property types

func (c *Client) handleMultiSelectProperty(page *notionapi.PageCreateRequest, key string, value interface{}) {
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
}

func (c *Client) handleSelectProperty(page *notionapi.PageCreateRequest, key string, value interface{}) {
	if projectStr, ok := value.(string); ok {
		page.Properties[key] = notionapi.SelectProperty{
			Select: notionapi.Option{
				Name: projectStr,
			},
		}
	}
}

func (c *Client) handleDateProperty(page *notionapi.PageCreateRequest, key string, value interface{}) {
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
}

func (c *Client) handleCheckboxProperty(page *notionapi.PageCreateRequest, key string, value interface{}) {
	var checked bool
	switch v := value.(type) {
	case bool:
		checked = v
	case string:
		checked = v == "true" || v == "yes" || v == "1"
	case float64:
		checked = v != 0
	case int:
		checked = v != 0
	default:
		checked = false
	}
	page.Properties[key] = notionapi.CheckboxProperty{
		Checkbox: checked,
	}
}

func (c *Client) handleTextProperty(page *notionapi.PageCreateRequest, key string, value interface{}) {
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

func (c *Client) handleNumberProperty(page *notionapi.PageCreateRequest, key string, value interface{}) {
	var number float64
	switch v := value.(type) {
	case float64:
		number = v
	case int:
		number = float64(v)
	case string:
		// Try to parse the string as a number
		var parsed float64
		if _, err := fmt.Sscanf(v, "%f", &parsed); err == nil {
			number = parsed
		} else {
			log.Printf("Could not parse string '%s' as number, skipping property %s", v, key)
			return
		}
	default:
		log.Printf("Unsupported type for number property %s, skipping", key)
		return
	}
	page.Properties[key] = notionapi.NumberProperty{
		Number: number,
	}
}

func (c *Client) handleURLProperty(page *notionapi.PageCreateRequest, key string, value interface{}) {
	if urlStr, ok := value.(string); ok {
		page.Properties[key] = notionapi.URLProperty{
			URL: urlStr,
		}
	}
}

func (c *Client) handleEmailProperty(page *notionapi.PageCreateRequest, key string, value interface{}) {
	if emailStr, ok := value.(string); ok {
		page.Properties[key] = notionapi.EmailProperty{
			Email: emailStr,
		}
	}
}

func (c *Client) handlePhoneProperty(page *notionapi.PageCreateRequest, key string, value interface{}) {
	if phoneStr, ok := value.(string); ok {
		page.Properties[key] = notionapi.PhoneNumberProperty{
			PhoneNumber: phoneStr,
		}
	}
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
