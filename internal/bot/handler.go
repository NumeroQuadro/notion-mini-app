package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/numero_quadro/notion-mini-app/internal/notion"
)

// State constants for tracking user conversation state
const (
	StateNone                 = 0
	StateAwaitingConfirmation = 1
)

// Store user states and pending tasks
type UserState struct {
	State       int
	PendingTask string
}

type Handler struct {
	bot              *tgbotapi.BotAPI
	notion           *notion.Client
	authorizedUserID int64                // Only this user can interact with the bot
	userStates       map[int64]*UserState // Track state for each user
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
		userStates:       make(map[int64]*UserState),
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

	// Get or initialize user state
	state, exists := h.userStates[message.From.ID]
	if !exists {
		state = &UserState{State: StateNone}
		h.userStates[message.From.ID] = state
	}

	// If we're awaiting confirmation, handle it
	if state.State == StateAwaitingConfirmation {
		// Check if message is a confirmation or decline
		switch strings.ToLower(message.Text) {
		case "yes", "y", "да", "д":
			return h.createTaskFromPending(message)
		case "no", "n", "нет", "н":
			state.State = StateNone
			msg := tgbotapi.NewMessage(message.Chat.ID, "Task creation cancelled.")
			_, err := h.bot.Send(msg)
			return err
		default:
			// Any other message cancels the previous task and starts a new one
			state.PendingTask = message.Text
			return h.askForConfirmation(message)
		}
	}

	// Handle regular commands
	switch message.Text {
	case "/start":
		return h.handleStart(message)
	case "Open Mini App":
		return h.handleMiniAppButton(message)
	default:
		// Any other text is treated as a potential task
		state.PendingTask = message.Text
		state.State = StateAwaitingConfirmation
		return h.askForConfirmation(message)
	}
}

func (h *Handler) handleUnauthorized(message *tgbotapi.Message) error {
	msg := tgbotapi.NewMessage(message.Chat.ID, "Sorry, you are not authorized to use this bot.")
	_, err := h.bot.Send(msg)
	return err
}

func (h *Handler) askForConfirmation(message *tgbotapi.Message) error {
	state := h.userStates[message.From.ID]

	// Create inline keyboard for confirmation
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Yes", "confirm_task"),
			tgbotapi.NewInlineKeyboardButtonData("No", "cancel_task"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Do you want to create a task with the following title?\n\n\"%s\"\n\nReply 'yes' or 'no', or send a new message to create a different task.", state.PendingTask))
	msg.ReplyMarkup = keyboard
	_, err := h.bot.Send(msg)
	return err
}

func (h *Handler) createTaskFromPending(message *tgbotapi.Message) error {
	state := h.userStates[message.From.ID]

	// Create a Notion task
	ctx := context.Background()
	err := h.notion.CreateTask(ctx, state.PendingTask, nil, "tasks")

	// Reset the state
	state.State = StateNone

	if err != nil {
		// Send error message
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Failed to create task: %v", err))
		_, sendErr := h.bot.Send(msg)
		return sendErr
	}

	// Send success message
	msg := tgbotapi.NewMessage(message.Chat.ID, "Task created successfully! ✅")
	_, sendErr := h.bot.Send(msg)
	return sendErr
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

func (h *Handler) HandleCallback(callback *tgbotapi.CallbackQuery) error {
	// Check if user is authorized
	if !h.isAuthorized(callback.From.ID) {
		log.Printf("Ignoring callback from unauthorized user: %d", callback.From.ID)
		callbackResponse := tgbotapi.NewCallback(callback.ID, "Unauthorized")
		_, err := h.bot.Request(callbackResponse)
		return err
	}

	// Always answer the callback to avoid the "loading" state in Telegram
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	h.bot.Request(callbackResponse)

	switch callback.Data {
	case "confirm_task":
		return h.handleConfirmTaskCallback(callback)
	case "cancel_task":
		return h.handleCancelTaskCallback(callback)
	default:
		return h.handleDefaultCallback(callback)
	}
}

func (h *Handler) handleConfirmTaskCallback(callback *tgbotapi.CallbackQuery) error {
	state, exists := h.userStates[callback.From.ID]
	if !exists || state.State != StateAwaitingConfirmation {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "No pending task to confirm.")
		_, err := h.bot.Send(msg)
		return err
	}

	// Create task in Notion
	ctx := context.Background()
	err := h.notion.CreateTask(ctx, state.PendingTask, nil, "tasks")

	// Reset the state
	state.State = StateNone

	if err != nil {
		// Send error message
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("Failed to create task: %v", err))
		_, sendErr := h.bot.Send(msg)
		return sendErr
	}

	// Update original message
	editMsg := tgbotapi.NewEditMessageText(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		fmt.Sprintf("Task created successfully! ✅\n\n\"%s\"", state.PendingTask),
	)
	_, err = h.bot.Send(editMsg)
	return err
}

func (h *Handler) handleCancelTaskCallback(callback *tgbotapi.CallbackQuery) error {
	state, exists := h.userStates[callback.From.ID]
	if exists {
		state.State = StateNone
	}

	// Update original message
	editMsg := tgbotapi.NewEditMessageText(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		"Task creation cancelled.",
	)
	_, err := h.bot.Send(editMsg)
	return err
}

func (h *Handler) handleDefaultCallback(callback *tgbotapi.CallbackQuery) error {
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Unknown action")
	_, err := h.bot.Send(msg)
	return err
}
