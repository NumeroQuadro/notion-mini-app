package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/numero_quadro/notion-mini-app/internal/gemini"
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
	gemini           *gemini.Client
	scheduler        Scheduler
	authorizedUserID int64                          // Only this user can interact with the bot
	pendingTasks     map[int64]map[int]*PendingTask // Track pending tasks by user ID and message ID
}

// Scheduler interface to avoid circular dependency
type Scheduler interface {
	RunManualCheck()
}

func NewHandler(bot *tgbotapi.BotAPI, notionClient *notion.Client, geminiClient *gemini.Client) *Handler {
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
		gemini:           geminiClient,
		scheduler:        nil, // Set later via SetScheduler
		authorizedUserID: authorizedUserID,
		pendingTasks:     make(map[int64]map[int]*PendingTask),
	}
}

// SetScheduler sets the scheduler after initialization
func (h *Handler) SetScheduler(scheduler Scheduler) {
	h.scheduler = scheduler
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

	// If it's a voice or audio message, transcribe it first
	if (message.Voice != nil || message.Audio != nil) && h.gemini != nil {
		var fileID string
		var mimeType string
		if message.Voice != nil {
			fileID = message.Voice.FileID
			mimeType = message.Voice.MimeType
			if mimeType == "" {
				mimeType = "audio/ogg"
			}
		} else if message.Audio != nil {
			fileID = message.Audio.FileID
			mimeType = message.Audio.MimeType
			if mimeType == "" {
				mimeType = "audio/mpeg"
			}
		}

		// Download file from Telegram
		url, err := h.bot.GetFileDirectURL(fileID)
		if err != nil {
			log.Printf("Failed to get file URL: %v", err)
			// Gracefully continue without storing
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Could not access audio file.")
			_, _ = h.bot.Send(msg)
			return nil
		}
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("Failed to download audio: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Download failed for audio.")
			_, _ = h.bot.Send(msg)
			return nil
		}
		defer resp.Body.Close()
		audioBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read audio bytes: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Could not read audio data.")
			_, _ = h.bot.Send(msg)
			return nil
		}

		// Transcribe via Gemini
		transcript, err := h.gemini.TranscribeAudio(audioBytes, mimeType)
		if err != nil {
			log.Printf("Gemini transcription failed: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Transcription failed.")
			_, _ = h.bot.Send(msg)
			return nil
		}

		// Store as pending task with the transcribed text
		message.Text = transcript
		h.storePendingTask(message)

		// Brief confirmation
		preview := transcript
		if len([]rune(preview)) > 200 {
			previewRunes := []rune(preview)
			preview = string(previewRunes[:200]) + "..."
		}
		confirm := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("📝 Transcribed. Add 👍 to save.\n%s", preview))
		_, _ = h.bot.Send(confirm)
		return nil
	}

	// Handle regular commands
	switch message.Text {
	case "/start":
		return h.handleStart(message)
	case "Open Mini App":
		return h.handleMiniAppButton(message)
	case "/cron":
		return h.handleCronCommand(message)
	case "/tags":
		return h.handleTagsCommand(message)
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

// handleCronCommand manually triggers the daily task check
func (h *Handler) handleCronCommand(message *tgbotapi.Message) error {
	if h.scheduler == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Scheduler not available")
		_, err := h.bot.Send(msg)
		return err
	}

	// Trigger the check
	h.scheduler.RunManualCheck()

	// Send confirmation
	msg := tgbotapi.NewMessage(message.Chat.ID, "✅ Task check triggered! Check logs for results.")
	_, err := h.bot.Send(msg)
	return err
}

// handleTagsCommand tags all existing tasks using Gemini AI
func (h *Handler) handleTagsCommand(message *tgbotapi.Message) error {
	if h.gemini == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Gemini AI not configured")
		_, err := h.bot.Send(msg)
		return err
	}

	// Send initial message
	msg := tgbotapi.NewMessage(message.Chat.ID, "🏷️ Starting to tag all tasks... This may take a while.")
	h.bot.Send(msg)

	// Run tagging in background
	go func() {
		ctx := context.Background()
		log.Printf("/tags command: Starting to tag all tasks")

		// Get all non-done tasks (up to 1000)
		tasks, err := h.notion.GetRecentTasks(ctx, "tasks", 1000)
		if err != nil {
			log.Printf("Error retrieving tasks for tagging: %v", err)
			errorMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ Failed to retrieve tasks: %v", err))
			h.bot.Send(errorMsg)
			return
		}

		log.Printf("/tags command: Found %d tasks to tag", len(tasks))

		taggedCount := 0
		skippedCount := 0
		errorCount := 0

		for i, task := range tasks {
			log.Printf("/tags command: Processing task %d/%d: %s", i+1, len(tasks), task.Title)

			// Check if task already has llm_tag
			if existingTag, ok := task.Properties["llm_tag"].(string); ok && existingTag != "" {
				log.Printf("/tags command: Task %s already has llm_tag '%s', skipping", task.ID, existingTag)
				skippedCount++
				continue
			}

			// Double-check: Skip if status is "done" (in case filter didn't catch it)
			if status, ok := task.Properties["status"].(string); ok && status == "done" {
				log.Printf("/tags command: Task %s has status 'done', skipping", task.ID)
				skippedCount++
				continue
			}

			// Double-check: Skip if task has "sometimes-later" tag (in case filter didn't catch it)
			if tags, ok := task.Properties["Tags"]; ok {
				if tagList, ok := tags.([]string); ok {
					hasSometimesLater := false
					for _, tag := range tagList {
						if tag == "sometimes-later" {
							hasSometimesLater = true
							break
						}
					}
					if hasSometimesLater {
						log.Printf("/tags command: Task %s has 'sometimes-later' tag, skipping", task.ID)
						skippedCount++
						continue
					}
				}
			}

			// Get tag from Gemini
			tag, err := h.gemini.TagTask(task.Title)
			if err != nil {
				log.Printf("/tags command: Failed to tag task %s: %v", task.ID, err)
				errorCount++
				// Use default tag on error
				tag = "task"
			}

			// Update task in Notion
			if err := h.notion.UpdateTaskLLMTag(task.ID, tag); err != nil {
				log.Printf("/tags command: Failed to update task %s in Notion: %v", task.ID, err)
				errorCount++
			} else {
				log.Printf("/tags command: Successfully tagged task %s as '%s'", task.ID, tag)
				taggedCount++
			}

			// Small delay to avoid rate limits
			time.Sleep(500 * time.Millisecond)
		}

		// Send summary
		summary := fmt.Sprintf(
			"✅ Tagging complete!\n\n"+
				"📊 Summary:\n"+
				"• Tagged: %d tasks\n"+
				"• Skipped (already tagged): %d\n"+
				"• Errors: %d\n"+
				"• Total processed: %d",
			taggedCount, skippedCount, errorCount, len(tasks))

		summaryMsg := tgbotapi.NewMessage(message.Chat.ID, summary)
		h.bot.Send(summaryMsg)
		log.Printf("/tags command: Completed. Tagged=%d, Skipped=%d, Errors=%d", taggedCount, skippedCount, errorCount)
	}()

	return nil
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
	var taskID string
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Attempt %d/%d to create task: %s", attempt, maxRetries, pendingTask.Text)
		taskID, err = h.notion.CreateTask(ctx, pendingTask.Text, nil, "tasks")

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

	// Task created successfully - now tag it with Gemini and store in Notion
	if h.gemini != nil {
		go func() {
			// Get LLM tag from Gemini
			tag, err := h.gemini.TagTask(pendingTask.Text)
			if err != nil {
				log.Printf("Warning: Failed to get LLM tag for task %s: %v", taskID, err)
				tag = "task" // Default tag on error
			}

			// Store tag in Notion's llm_tag property
			if err := h.notion.UpdateTaskLLMTag(taskID, tag); err != nil {
				log.Printf("Warning: Failed to update llm_tag in Notion for %s: %v", taskID, err)
			}
		}()
	} else {
		log.Printf("Gemini not configured, skipping task tagging")
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
