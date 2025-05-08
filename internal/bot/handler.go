package bot

import (
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/numero_quadro/notion-mini-app/internal/notion"
)

type Handler struct {
	bot    *tgbotapi.BotAPI
	notion *notion.Client
}

func NewHandler(bot *tgbotapi.BotAPI, notionClient *notion.Client) *Handler {
	return &Handler{
		bot:    bot,
		notion: notionClient,
	}
}

func (h *Handler) HandleMessage(message *tgbotapi.Message) error {
	switch message.Text {
	case "/start":
		return h.handleStart(message)
	case "/newtask":
		return h.handleNewTask(message)
	case "Open Mini App":
		return h.handleMiniAppButton(message)
	default:
		return h.handleDefault(message)
	}
}

func (h *Handler) handleStart(message *tgbotapi.Message) error {
	// Create a custom keyboard with a button
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Open Mini App"),
		),
	)
	msg := tgbotapi.NewMessage(message.Chat.ID, "Welcome to Notion Task Manager! Use the button below to open the mini app or /newtask to create a new task.")
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

func (h *Handler) handleNewTask(message *tgbotapi.Message) error {
	// Create inline keyboard for task creation
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Create Task", "create_task"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, "Click the button below to create a new task:")
	msg.ReplyMarkup = keyboard
	_, err := h.bot.Send(msg)
	return err
}

func (h *Handler) handleDefault(message *tgbotapi.Message) error {
	msg := tgbotapi.NewMessage(message.Chat.ID, "I don't understand that command. Use /start to see available commands.")
	_, err := h.bot.Send(msg)
	return err
}

func (h *Handler) HandleCallback(callback *tgbotapi.CallbackQuery) error {
	switch callback.Data {
	case "create_task":
		return h.handleCreateTaskCallback(callback)
	default:
		return h.handleDefaultCallback(callback)
	}
}

func (h *Handler) handleCreateTaskCallback(callback *tgbotapi.CallbackQuery) error {
	// Send a message with a link to the mini app
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Click the button below to open the task creation form:")

	// Create a web app button
	webAppButton := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Create Task", "https://your-domain.com/webapp"),
		),
	)

	msg.ReplyMarkup = webAppButton
	_, err := h.bot.Send(msg)
	return err
}

func (h *Handler) handleDefaultCallback(callback *tgbotapi.CallbackQuery) error {
	callbackResponse := tgbotapi.NewCallback(callback.ID, "Unknown action")
	_, err := h.bot.Request(callbackResponse)
	return err
}
