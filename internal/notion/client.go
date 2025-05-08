package notion

import (
	"context"
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
			// Handle date property
			if dateStr, ok := value.(string); ok && dateStr != "" {
				// Skip the complex Date object for now, use a simple text property
				page.Properties[key] = notionapi.RichTextProperty{
					RichText: []notionapi.RichText{
						{
							Text: &notionapi.Text{
								Content: dateStr,
							},
						},
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
		log.Printf("Error creating task: %v", err)
		return err
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
		log.Printf("Property: %s, Type: %s", name, config.GetType())
	}

	return db.Properties, nil
}
