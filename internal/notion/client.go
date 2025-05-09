package notion

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jomei/notionapi"
)

// ButtonProperty represents a Notion button property
type ButtonProperty struct {
	Button map[string]interface{} `json:"button"`
}

// GetType returns the type of the property
func (p ButtonProperty) GetType() notionapi.PropertyType {
	return "button"
}

// ButtonPropertyConfig represents the configuration for a button property
type ButtonPropertyConfig struct {
	Type   notionapi.PropertyConfigType `json:"type"`
	Button map[string]interface{}       `json:"button"`
	ID     string                       `json:"id,omitempty"`
	Name   string                       `json:"name,omitempty"`
}

// GetType returns the type of the property config
func (p ButtonPropertyConfig) GetType() notionapi.PropertyConfigType {
	return p.Type
}

// Task represents a simplified Notion database item
type Task struct {
	ID         string                 `json:"id"`
	Title      string                 `json:"title"`
	URL        string                 `json:"url"`
	CreatedAt  time.Time              `json:"created_at"`
	Properties map[string]interface{} `json:"properties"`
}

type Client struct {
	client        *notionapi.Client
	taskDbID      string
	notesDbID     string
	dbCache       map[string]map[string]notionapi.PropertyConfig
	dbCacheExpiry map[string]time.Time
}

func NewClient() *Client {
	apiToken := os.Getenv("NOTION_API_KEY")
	taskDbID := os.Getenv("NOTION_TASKS_DATABASE_ID")
	notesDbID := os.Getenv("NOTION_NOTES_DATABASE_ID")

	if apiToken == "" {
		log.Printf("WARNING: NOTION_API_KEY environment variable is not set")
	}

	if taskDbID == "" {
		// For backward compatibility
		taskDbID = os.Getenv("NOTION_DATABASE_ID")
		if taskDbID == "" {
			log.Printf("WARNING: NOTION_TASKS_DATABASE_ID environment variable is not set")
		}
	}

	if notesDbID == "" {
		log.Printf("WARNING: NOTION_NOTES_DATABASE_ID environment variable is not set")
	}

	// Create standard Notion client
	client := notionapi.NewClient(notionapi.Token(apiToken))

	return &Client{
		client:        client,
		taskDbID:      taskDbID,
		notesDbID:     notesDbID,
		dbCache:       make(map[string]map[string]notionapi.PropertyConfig),
		dbCacheExpiry: make(map[string]time.Time),
	}
}

func (c *Client) GetTasksDatabaseID() string {
	return c.taskDbID
}

func (c *Client) GetNotesDatabaseID() string {
	return c.notesDbID
}

func (c *Client) CreateTask(ctx context.Context, title string, properties map[string]interface{}, dbType string) error {
	dbID := c.getDbIDForType(dbType)
	log.Printf("Creating task in %s database: %s with properties: %v", dbType, title, properties)

	if title == "" {
		return fmt.Errorf("task title cannot be empty")
	}

	if dbID == "" {
		return fmt.Errorf("database ID for %s not configured", dbType)
	}

	// Check if context has a deadline (timeout)
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		log.Printf("Context has deadline, %v remaining", remaining)
	} else {
		// If no timeout set, add one to prevent hanging
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		log.Printf("Added 10s timeout to context")
	}

	// Get database properties to check for button types
	dbProps, err := c.GetDatabaseProperties(ctx, dbType)
	if err != nil {
		log.Printf("Warning: Could not fetch database properties: %v", err)
		// Continue anyway but be more cautious
	}

	// Create the base request with title property
	page := &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			Type:       notionapi.ParentTypeDatabaseID,
			DatabaseID: notionapi.DatabaseID(dbID),
		},
		Properties: notionapi.Properties{
			"Name": notionapi.TitleProperty{
				Title: []notionapi.RichText{
					{
						Type: notionapi.ObjectType("text"),
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

				// Skip button and unsupported types
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

		// Check for button property error
		if strings.Contains(err.Error(), "unsupported property type: button") {
			log.Printf("Error due to button property - adding workaround code")
			return fmt.Errorf("database contains button properties which are not supported by Notion API. Please remove button properties from the request")
		}

		return fmt.Errorf("Notion API error: %w", err)
	}

	log.Printf("Task created successfully with ID: %s", createdPage.ID)
	return nil
}

func (c *Client) getDbIDForType(dbType string) string {
	switch dbType {
	case "notes":
		return c.notesDbID
	case "tasks":
		return c.taskDbID
	default:
		// Default to tasks database for backward compatibility
		return c.taskDbID
	}
}

// GetDatabaseProperties retrieves database properties and caches them for efficiency
func (c *Client) GetDatabaseProperties(ctx context.Context, dbType string) (map[string]notionapi.PropertyConfig, error) {
	dbID := c.getDbIDForType(dbType)

	// Check cache first
	if props, ok := c.dbCache[dbID]; ok {
		// Check if cache is still valid (10 minute cache)
		if expiryTime, ok := c.dbCacheExpiry[dbID]; ok && time.Now().Before(expiryTime) {
			log.Printf("Using cached database properties for %s", dbType)
			return props, nil
		}
	}

	if dbID == "" {
		return nil, fmt.Errorf("database ID for %s not configured", dbType)
	}

	// Add timeout to context if not already present
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	// Call Notion API to get database
	db, err := c.client.Database.Get(ctx, notionapi.DatabaseID(dbID))
	if err != nil {
		// Check if it's a button property error
		if strings.Contains(err.Error(), "unsupported property type: button") {
			log.Printf("Warning: Database has button properties which are not supported by the API library")
			// Try a different approach to get database properties
			return c.getPropertiesWithButtonWorkaround(ctx, dbID)
		}
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	// Create a copy of the properties to handle button type
	properties := make(map[string]notionapi.PropertyConfig)

	// Copy all properties from the DB response
	for key, prop := range db.Properties {
		// Skip button properties which might cause issues
		if propType := prop.GetType(); propType == "button" {
			log.Printf("Skipping button property '%s' to avoid API errors", key)
			continue
		}
		properties[key] = prop
	}

	// Cache the result with 10 minute expiry
	c.dbCache[dbID] = properties
	c.dbCacheExpiry[dbID] = time.Now().Add(10 * time.Minute)

	return properties, nil
}

// getPropertiesWithButtonWorkaround is a fallback method to get database properties
// when the standard approach fails due to button properties
func (c *Client) getPropertiesWithButtonWorkaround(ctx context.Context, dbID string) (map[string]notionapi.PropertyConfig, error) {
	log.Printf("Using workaround to retrieve database properties while ignoring button properties")

	// Query the database to get one page - this avoids the direct database fetch error
	queryRequest := &notionapi.DatabaseQueryRequest{
		PageSize: 1,
	}

	response, err := c.client.Database.Query(ctx, notionapi.DatabaseID(dbID), queryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}

	// Create a map to store property configurations
	properties := make(map[string]notionapi.PropertyConfig)

	// If we got a page, use its properties to determine the schema
	if len(response.Results) > 0 {
		page := response.Results[0]

		// Process each property to create a property config
		for key, prop := range page.Properties {
			// Skip button properties
			if prop.GetType() == "button" {
				log.Printf("Skipping button property '%s'", key)
				continue
			}

			// Create a basic property config based on the type
			var config notionapi.PropertyConfig

			switch prop.GetType() {
			case "title":
				config = &notionapi.TitlePropertyConfig{
					Type: notionapi.PropertyConfigTypeTitle,
				}
			case "rich_text":
				config = &notionapi.RichTextPropertyConfig{
					Type: notionapi.PropertyConfigTypeRichText,
				}
			case "number":
				config = &notionapi.NumberPropertyConfig{
					Type: notionapi.PropertyConfigTypeNumber,
				}
			case "select":
				config = &notionapi.SelectPropertyConfig{
					Type: notionapi.PropertyConfigTypeSelect,
					Select: notionapi.Select{
						Options: []notionapi.Option{},
					},
				}
			case "multi_select":
				config = &notionapi.MultiSelectPropertyConfig{
					Type: notionapi.PropertyConfigTypeMultiSelect,
					MultiSelect: notionapi.Select{
						Options: []notionapi.Option{},
					},
				}
			case "date":
				config = &notionapi.DatePropertyConfig{
					Type: notionapi.PropertyConfigTypeDate,
				}
			case "checkbox":
				config = &notionapi.CheckboxPropertyConfig{
					Type: notionapi.PropertyConfigTypeCheckbox,
				}
			case "url":
				config = &notionapi.URLPropertyConfig{
					Type: notionapi.PropertyConfigTypeURL,
				}
			case "email":
				config = &notionapi.EmailPropertyConfig{
					Type: notionapi.PropertyConfigTypeEmail,
				}
			case "phone_number":
				config = &notionapi.PhoneNumberPropertyConfig{
					Type: notionapi.PropertyConfigTypePhoneNumber,
				}
			default:
				// Skip unsupported property types
				log.Printf("Skipping unsupported property type: %s for property %s", prop.GetType(), key)
				continue
			}

			properties[key] = config
		}
	}

	// Cache the properties
	c.dbCache[dbID] = properties
	c.dbCacheExpiry[dbID] = time.Now().Add(10 * time.Minute)

	return properties, nil
}

// For backward compatibility
func (c *Client) CreateItem(ctx context.Context, title string, properties map[string]interface{}) error {
	return c.CreateTask(ctx, title, properties, "tasks")
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

// GetRecentTasks retrieves recent tasks from the specified Notion database
// with filtering for:
// - tasks should not have status "done"
// - tasks should have empty Date
// - tasks should not contain tag "sometimes-later"
func (c *Client) GetRecentTasks(ctx context.Context, dbType string, limit int) ([]Task, error) {
	dbID := c.getDbIDForType(dbType)
	if dbID == "" {
		return nil, fmt.Errorf("database ID for %s not configured", dbType)
	}

	// Create database query filter
	filter := &notionapi.DatabaseQueryRequest{
		Filter: notionapi.AndCompoundFilter{
			// Filter for status not being "done"
			notionapi.PropertyFilter{
				Property: "status",
				Select: &notionapi.SelectFilterCondition{
					DoesNotEqual: "done",
				},
			},
			// Filter for empty Date
			notionapi.PropertyFilter{
				Property: "Date",
				Date: &notionapi.DateFilterCondition{
					IsEmpty: true,
				},
			},
			// Filter for tags not containing "sometimes-later"
			notionapi.PropertyFilter{
				Property: "Tags",
				MultiSelect: &notionapi.MultiSelectFilterCondition{
					DoesNotContain: "sometimes-later",
				},
			},
		},
		Sorts: []notionapi.SortObject{
			{
				Property:  "Created time",
				Direction: "descending",
			},
		},
		PageSize: limit,
	}

	// Query the database
	response, err := c.client.Database.Query(ctx, notionapi.DatabaseID(dbID), filter)
	if err != nil {
		// Handle button property error gracefully
		if strings.Contains(err.Error(), "unsupported property type: button") {
			log.Printf("Warning: Button property detected during query. Using workaround...")
			return c.getRecentTasksWithButtonWorkaround(ctx, dbID, limit)
		}
		return nil, fmt.Errorf("failed to query database: %w", err)
	}

	// Transform the results
	tasks := make([]Task, 0, len(response.Results))
	for _, page := range response.Results {
		task, err := c.transformPageToTask(page)
		if err != nil {
			log.Printf("Warning: Could not transform page %s: %v", page.ID, err)
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// transformPageToTask converts a Notion page to a Task struct
func (c *Client) transformPageToTask(page notionapi.Page) (Task, error) {
	task := Task{
		ID:         string(page.ID),
		URL:        page.URL,
		CreatedAt:  page.CreatedTime,
		Properties: make(map[string]interface{}),
	}

	// Extract title from Name property
	if titleProp, ok := page.Properties["Name"]; ok {
		if title, ok := titleProp.(*notionapi.TitleProperty); ok && len(title.Title) > 0 {
			task.Title = title.Title[0].PlainText
		}
	}

	// Add other properties
	for key, prop := range page.Properties {
		if key == "Name" {
			continue // Already handled above
		}

		// Skip button properties
		if prop.GetType() == "button" {
			continue
		}

		switch prop.GetType() {
		case "select":
			if selectProp, ok := prop.(*notionapi.SelectProperty); ok && selectProp.Select.Name != "" {
				task.Properties[key] = selectProp.Select.Name
			}
		case "multi_select":
			if multiSelectProp, ok := prop.(*notionapi.MultiSelectProperty); ok {
				tags := make([]string, 0, len(multiSelectProp.MultiSelect))
				for _, opt := range multiSelectProp.MultiSelect {
					tags = append(tags, opt.Name)
				}
				task.Properties[key] = tags
			}
		case "date":
			if dateProp, ok := prop.(*notionapi.DateProperty); ok && dateProp.Date != nil {
				task.Properties[key] = dateProp.Date.Start.String()
			}
		case "checkbox":
			if checkboxProp, ok := prop.(*notionapi.CheckboxProperty); ok {
				task.Properties[key] = checkboxProp.Checkbox
			}
		case "rich_text":
			if textProp, ok := prop.(*notionapi.RichTextProperty); ok && len(textProp.RichText) > 0 {
				var text strings.Builder
				for _, t := range textProp.RichText {
					text.WriteString(t.PlainText)
				}
				task.Properties[key] = text.String()
			}
		default:
			// Skip other property types
		}
	}

	return task, nil
}

// getRecentTasksWithButtonWorkaround provides a fallback method for querying databases with button properties
func (c *Client) getRecentTasksWithButtonWorkaround(ctx context.Context, dbID string, limit int) ([]Task, error) {
	// Simple query without filters to get recent tasks
	queryRequest := &notionapi.DatabaseQueryRequest{
		Sorts: []notionapi.SortObject{
			{
				Property:  "Created time",
				Direction: "descending",
			},
		},
		PageSize: 100, // Get more to allow for manual filtering
	}

	response, err := c.client.Database.Query(ctx, notionapi.DatabaseID(dbID), queryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}

	// Manual filtering
	tasks := make([]Task, 0, limit)
	for _, page := range response.Results {
		// Skip if we already have enough tasks
		if len(tasks) >= limit {
			break
		}

		// Check status property
		if statusProp, ok := page.Properties["status"]; ok {
			if selectProp, ok := statusProp.(*notionapi.SelectProperty); ok {
				if selectProp.Select.Name == "done" {
					continue // Skip if status is done
				}
			}
		}

		// Check Date property
		if dateProp, ok := page.Properties["Date"]; ok {
			if dateValue, ok := dateProp.(*notionapi.DateProperty); ok {
				if dateValue.Date != nil && dateValue.Date.Start != nil {
					continue // Skip if date is not empty
				}
			}
		}

		// Check Tags property
		if tagsProp, ok := page.Properties["Tags"]; ok {
			if multiSelectProp, ok := tagsProp.(*notionapi.MultiSelectProperty); ok {
				hasSometimesLater := false
				for _, tag := range multiSelectProp.MultiSelect {
					if tag.Name == "sometimes-later" {
						hasSometimesLater = true
						break
					}
				}
				if hasSometimesLater {
					continue // Skip if has sometimes-later tag
				}
			}
		}

		// If we got here, the task passed all filters
		task, err := c.transformPageToTask(page)
		if err != nil {
			log.Printf("Warning: Could not transform page %s: %v", page.ID, err)
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}
