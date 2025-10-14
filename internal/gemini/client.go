package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Client struct {
	apiKey string
	model  string
}

type GeminiRequest struct {
	Contents []Content `json:"contents"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GeminiResponse struct {
	Candidates []Candidate `json:"candidates"`
}

type Candidate struct {
	Content ContentResponse `json:"content"`
}

type ContentResponse struct {
	Parts []PartResponse `json:"parts"`
}

type PartResponse struct {
	Text string `json:"text"`
}

// NewClient creates a new Gemini API client
func NewClient() *Client {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Printf("WARNING: GEMINI_API_KEY not set")
	}

	return &Client{
		apiKey: apiKey,
		model:  "gemini-2.0-flash-exp", // Using latest flash model
	}
}

// TagTask analyzes the task content and returns an appropriate tag
func (c *Client) TagTask(taskContent string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not configured")
	}

	// Create the prompt
	prompt := fmt.Sprintf(`Analyze the following task entry and categorize it with a single tag.

Rules:
- If the entry is ONLY a URL/link (starts with http, https, or looks like a web link), respond with exactly: "link"
- If the entry mentions thoughts, emotions, observations, feelings, reflections, or is a personal journal-style entry, respond with exactly: "journal"
- If the entry mentions a deadline, date reference (like "today", "tomorrow", "next week", "23 october", "by friday", "due on", etc.), respond with exactly: "date"
- If none of the above apply, respond with exactly: "task"

Task entry: "%s"

Respond with ONLY ONE WORD from: link, journal, date, or task`, taskContent)

	// Create request body
	reqBody := GeminiRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make API request
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.model, c.apiKey)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to call Gemini API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract tag from response
	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini API")
	}

	tag := strings.TrimSpace(strings.ToLower(geminiResp.Candidates[0].Content.Parts[0].Text))

	// Validate tag
	validTags := map[string]bool{
		"link":    true,
		"journal": true,
		"date":    true,
		"task":    true,
	}

	if !validTags[tag] {
		log.Printf("Invalid tag received from Gemini: %s, defaulting to 'task'", tag)
		tag = "task"
	}

	log.Printf("Gemini tagged task as: %s", tag)
	return tag, nil
}
