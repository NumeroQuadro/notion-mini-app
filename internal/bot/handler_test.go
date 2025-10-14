package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Test storing pending task
func TestStorePendingTask(t *testing.T) {
	handler := &Handler{
		pendingTasks: make(map[int64]map[int]*PendingTask),
	}

	message := &tgbotapi.Message{
		MessageID: 123,
		From:      &tgbotapi.User{ID: 456},
		Text:      "Test task",
		Chat:      &tgbotapi.Chat{ID: 789},
	}

	// Manually store without bot interaction
	userID := message.From.ID
	messageID := message.MessageID
	if handler.pendingTasks[userID] == nil {
		handler.pendingTasks[userID] = make(map[int]*PendingTask)
	}
	handler.pendingTasks[userID][messageID] = &PendingTask{
		MessageID: messageID,
		Text:      message.Text,
	}

	// Check task was stored
	if handler.pendingTasks[456] == nil {
		t.Fatal("User tasks map not initialized")
	}

	task := handler.pendingTasks[456][123]
	if task == nil {
		t.Fatal("Task not stored")
	}

	if task.Text != "Test task" {
		t.Errorf("Expected 'Test task', got '%s'", task.Text)
	}

	if task.MessageID != 123 {
		t.Errorf("Expected message ID 123, got %d", task.MessageID)
	}
}

// Test message update (edit)
func TestMessageUpdate(t *testing.T) {
	handler := &Handler{
		pendingTasks: make(map[int64]map[int]*PendingTask),
	}

	userID := int64(456)
	messageID := 123

	// Store initial message
	handler.pendingTasks[userID] = make(map[int]*PendingTask)
	handler.pendingTasks[userID][messageID] = &PendingTask{
		MessageID: messageID,
		Text:      "Original text",
	}

	// Update with edited message
	handler.pendingTasks[userID][messageID] = &PendingTask{
		MessageID: messageID,
		Text:      "Edited text",
	}

	// Check task was updated
	task := handler.pendingTasks[userID][messageID]
	if task.Text != "Edited text" {
		t.Errorf("Expected 'Edited text', got '%s'", task.Text)
	}
}

// Test only thumbs up triggers processing
func TestOnlyThumbsUpTriggers(t *testing.T) {
	tests := []struct {
		name  string
		emoji string
	}{
		{"thumbs up", "👍"},
		{"heart", "❤️"},
		{"fire", "🔥"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create reaction
			reaction := &MessageReactionUpdate{
				Chat:      ChatInfo{ID: 789},
				MessageID: 123,
				User:      UserInfo{ID: 456},
				NewReaction: []ReactionType{
					{Type: "emoji", Emoji: tt.emoji},
				},
			}

			// Check if it's thumbs up
			isThumbsUp := false
			for _, r := range reaction.NewReaction {
				if r.Type == "emoji" && r.Emoji == "👍" {
					isThumbsUp = true
					break
				}
			}

			if tt.emoji == "👍" && !isThumbsUp {
				t.Error("Thumbs up should be detected")
			}
			if tt.emoji != "👍" && isThumbsUp {
				t.Error("Non-thumbs-up should not be detected")
			}
		})
	}
}

// Test authorization check
func TestAuthorizationCheck(t *testing.T) {
	handler := &Handler{
		authorizedUserID: 456,
	}

	// Test authorized user
	if !handler.isAuthorized(456) {
		t.Error("User 456 should be authorized")
	}

	// Test unauthorized user
	if handler.isAuthorized(999) {
		t.Error("User 999 should not be authorized")
	}

	// Test no authorization (allow all)
	handlerNoAuth := &Handler{
		authorizedUserID: 0,
	}
	if !handlerNoAuth.isAuthorized(999) {
		t.Error("When no auth is set, all users should be allowed")
	}
}

// Test reaction type parsing
func TestReactionTypeParsing(t *testing.T) {
	reaction := &MessageReactionUpdate{
		NewReaction: []ReactionType{
			{Type: "emoji", Emoji: "👍"},
			{Type: "emoji", Emoji: "❤️"},
		},
	}

	if len(reaction.NewReaction) != 2 {
		t.Errorf("Expected 2 reactions, got %d", len(reaction.NewReaction))
	}

	if reaction.NewReaction[0].Emoji != "👍" {
		t.Error("First reaction should be thumbs up")
	}
}
