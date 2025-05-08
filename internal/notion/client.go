package notion

import (
	"context"
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
	// Create a new page in the database
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
		// Convert the value to appropriate Notion property type
		// This is a simplified version - you'll need to handle different property types
		switch v := value.(type) {
		case string:
			page.Properties[key] = notionapi.RichTextProperty{
				RichText: []notionapi.RichText{
					{
						Text: &notionapi.Text{
							Content: v,
						},
					},
				},
			}
		}
	}

	_, err := c.client.Page.Create(ctx, page)
	return err
}

func (c *Client) GetDatabaseProperties(ctx context.Context) (map[string]notionapi.PropertyConfig, error) {
	db, err := c.client.Database.Get(ctx, notionapi.DatabaseID(c.dbID))
	if err != nil {
		return nil, err
	}
	return db.Properties, nil
}
