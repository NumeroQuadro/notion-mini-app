package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/numero_quadro/notion-mini-app/internal/notion"
)

// Store pending tasks waiting for reaction
type PendingTask struct {
	MessageID int
	Text      string
}

type Handler struct {
	bot              *tgbotapi.BotAPI
	notion           *notion.Client
	authorizedUserID int64                          // Only this user can interact with the bot
	pendingTasks     map[int64]map[int]*PendingTask // Track pending tasks by user ID and message ID
}

func NewHandler(bot *tgbotapi.BotAPI, notionClient *notion.Client) *Handler {
	// Get authorized user ID from environment variable
	authorizedUserIDStr := os.Getenv("AUTHORIZED_USER_ID")
	var authorizedUserID int64 = 0

	if authorizedUserIDStr != "" {
		id, err := strconv.ParseInt(authorizedUserIDStr, 10, 64)
		if err != nil {
			log.Printf("Warning: Invalid AUTHORIZED_USER_ID: %v", err)
		} else {
			authorizedUserID = id
		}
	}

	if authorizedUserID == 0 {
		log.Printf("Warning: No authorized user ID set, bot will be accessible to anyone")
	} else {
		log.Printf("Bot restricted to user ID: %d", authorizedUserID)
	}

	return &Handler{
		bot:              bot,
		notion:           notionClient,
		authorizedUserID: authorizedUserID,
		pendingTasks:     make(map[int64]map[int]*PendingTask),
	}
}

func (h *Handler) isAuthorized(userID int64) bool {
	// If no authorized user is set, allow anyone
	if h.authorizedUserID == 0 {
		return true
	}
	return userID == h.authorizedUserID
}

func (h *Handler) HandleMessage(message *tgbotapi.Message) error {
	// Check if user is authorized
	if !h.isAuthorized(message.From.ID) {
		// Only respond to /start, silently ignore other messages from unauthorized users
		if message.Text == "/start" {
			return h.handleUnauthorized(message)
		}
		log.Printf("Ignoring message from unauthorized user: %d", message.From.ID)
		return nil
	}

	// Handle regular commands
	switch message.Text {
	case "/start":
		return h.handleStart(message)
	case "Open Mini App":
		return h.handleMiniAppButton(message)
	default:
		// Any other text is treated as a potential task, stored and waiting for reaction
		h.storePendingTask(message)
		log.Printf("Stored message %d as pending task: %s", message.MessageID, message.Text)
		return nil // Don't send any response, just wait for reaction
	}
}

func (h *Handler) handleUnauthorized(message *tgbotapi.Message) error {
	msg := tgbotapi.NewMessage(message.Chat.ID, "Sorry, you are not authorized to use this bot.")
	_, err := h.bot.Send(msg)
	return err
}

func (h *Handler) storePendingTask(message *tgbotapi.Message) {
	userID := message.From.ID
	messageID := message.MessageID

	// Initialize map for user if it doesn't exist
	if h.pendingTasks[userID] == nil {
		h.pendingTasks[userID] = make(map[int]*PendingTask)
	}

	// Store the pending task
	h.pendingTasks[userID][messageID] = &PendingTask{
		MessageID: messageID,
		Text:      message.Text,
	}

	// Set thinking emoji when message is received
	if setErr := h.setMessageReaction(message.Chat.ID, messageID, "🤔"); setErr != nil {
		log.Printf("Warning: Failed to set 🤔 reaction: %v", setErr)
	}
}

func (h *Handler) handleStart(message *tgbotapi.Message) error {
	// Create a custom keyboard with a button
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Open Mini App"),
		),
	)
	msg := tgbotapi.NewMessage(message.Chat.ID, "Welcome to Notion Task Manager! Use the button below to open the mini app or simply send me any text to create a new task.")
	msg.ReplyMarkup = keyboard
	_, err := h.bot.Send(msg)
	return err
}

func (h *Handler) handleMiniAppButton(message *tgbotapi.Message) error {
	miniAppURL := os.Getenv("MINI_APP_URL")
	if miniAppURL == "" {
		miniAppURL = "https://tralalero-tralala.ru/notion/mini-app" // Default fallback
	}

	// Create a web app button
	webAppButton := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Open Mini App", miniAppURL),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, "Click the button below to open the mini app:")
	msg.ReplyMarkup = webAppButton
	_, err := h.bot.Send(msg)
	return err
}

// MessageReactionUpdate represents an update to message reactions
type MessageReactionUpdate struct {
	Chat        ChatInfo       `json:"chat"`
	MessageID   int            `json:"message_id"`
	User        UserInfo       `json:"user,omitempty"`
	ActorChat   ChatInfo       `json:"actor_chat,omitempty"`
	Date        int            `json:"date"`
	OldReaction []ReactionType `json:"old_reaction"`
	NewReaction []ReactionType `json:"new_reaction"`
}

type ChatInfo struct {
	ID int64 `json:"id"`
}

type UserInfo struct {
	ID int64 `json:"id"`
}

type ReactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
}

// HandleMessageReaction handles reactions added to messages
func (h *Handler) HandleMessageReaction(reaction *MessageReactionUpdate) error {
	// Check if user is authorized
	if reaction.User.ID == 0 || !h.isAuthorized(reaction.User.ID) {
		log.Printf("Ignoring reaction from unauthorized or unknown user")
		return nil
	}

	userID := reaction.User.ID
	messageID := reaction.MessageID
	chatID := reaction.Chat.ID

	log.Printf("Received reaction update for message %d from user %d", messageID, userID)

	// Check if this message has a pending task
	if h.pendingTasks[userID] == nil || h.pendingTasks[userID][messageID] == nil {
		log.Printf("No pending task found for message %d", messageID)
		return nil
	}

	// Check if reaction was added (not removed)
	if len(reaction.NewReaction) == 0 {
		log.Printf("Reaction was removed, ignoring")
		return nil
	}

	// Only process if reaction is 👍 (thumbs up)
	isThumbsUp := false
	for _, r := range reaction.NewReaction {
		if r.Type == "emoji" && r.Emoji == "👍" {
			isThumbsUp = true
			break
		}
	}

	if !isThumbsUp {
		log.Printf("Reaction is not thumbs up, ignoring")
		return nil
	}

	// Get the pending task
	pendingTask := h.pendingTasks[userID][messageID]

	// Set writing hand reaction to indicate processing
	if setErr := h.setMessageReaction(chatID, messageID, "✍️"); setErr != nil {
		log.Printf("Warning: Failed to set ✍️ reaction: %v", setErr)
	}

	// Try to create task with retries
	ctx := context.Background()
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Attempt %d/%d to create task: %s", attempt, maxRetries, pendingTask.Text)
		err = h.notion.CreateTask(ctx, pendingTask.Text, nil, "tasks")

		if err == nil {
			// Success!
			log.Printf("Task created successfully on attempt %d: %s", attempt, pendingTask.Text)
			break
		}

		log.Printf("Attempt %d failed: %v", attempt, err)

		if attempt < maxRetries {
			// Wait before retry (exponential backoff)
			sleep := time.Duration(attempt) * 2 * time.Second
			log.Printf("Waiting %v before retry...", sleep)
			time.Sleep(sleep)
		}
	}

	// Remove from pending tasks
	delete(h.pendingTasks[userID], messageID)

	if err != nil {
		// All retries failed - set crying emoji
		log.Printf("Failed to create task after %d attempts: %v", maxRetries, err)
		if setErr := h.setMessageReaction(chatID, messageID, "😢"); setErr != nil {
			log.Printf("Error setting 😢 reaction: %v", setErr)
		}
		return err
	}

	// Success - set thumbs up (try multiple times to ensure it's visible)
	for i := 0; i < 2; i++ {
		if setErr := h.setMessageReaction(chatID, messageID, "👍"); setErr != nil {
			log.Printf("Attempt %d: Failed to set 👍 reaction: %v", i+1, setErr)
			time.Sleep(500 * time.Millisecond)
		} else {
			log.Printf("Successfully set 👍 reaction")
			break
		}
	}
	return nil
}

// setMessageReaction sets a reaction on a message using direct API call
func (h *Handler) setMessageReaction(chatID int64, messageID int, emoji string) error {
	token := h.bot.Token
	url := fmt.Sprintf("https://api.telegram.org/bot%s/setMessageReaction", token)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"reaction": []map[string]string{
			{
				"type":  "emoji",
				"emoji": emoji,
			},
		},
		"is_big": false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("API returned status %d: %v", resp.StatusCode, result)
	}

	log.Printf("Successfully set reaction on message %d", messageID)
	return nil
}
