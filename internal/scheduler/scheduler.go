package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jomei/notionapi"
	"github.com/numero_quadro/notion-mini-app/internal/database"
	"github.com/numero_quadro/notion-mini-app/internal/notion"
)

type Scheduler struct {
	db               *database.DB
	notionClient     *notion.Client
	bot              *tgbotapi.BotAPI
	authorizedUserID int64
	checkTime        string // Format: "15:04" (HH:MM in 24-hour format)
}

// NewScheduler creates a new scheduler instance
func NewScheduler(db *database.DB, notionClient *notion.Client, bot *tgbotapi.BotAPI, authorizedUserID int64, checkTime string) *Scheduler {
	if checkTime == "" {
		checkTime = "23:00" // Default to 11 PM
	}

	return &Scheduler{
		db:               db,
		notionClient:     notionClient,
		bot:              bot,
		authorizedUserID: authorizedUserID,
		checkTime:        checkTime,
	}
}

// Start begins the scheduler loop
func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("Starting scheduler with daily check at %s", s.checkTime)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Scheduler stopped")
			return
		case now := <-ticker.C:
			// Check if current time matches check time
			if now.Format("15:04") == s.checkTime {
				log.Printf("Running scheduled task check at %s", now.Format("15:04"))
				go s.checkTasks(ctx)
			}
		}
	}
}

// RunManualCheck triggers a manual task check (for testing/recovery)
func (s *Scheduler) RunManualCheck() {
	log.Printf("Manual task check triggered")
	ctx := context.Background()
	go s.checkTasks(ctx)
}

// checkTasks performs the daily task check
func (s *Scheduler) checkTasks(ctx context.Context) {
	log.Printf("Starting task check...")

	// Get tasks from the last 24 hours
	since := time.Now().Add(-24 * time.Hour)
	tasks, err := s.db.GetTasksSince(since)
	if err != nil {
		log.Printf("Error retrieving tasks: %v", err)
		return
	}

	log.Printf("Found %d tasks to check", len(tasks))

	for _, task := range tasks {
		// Check if task still exists in Notion
		exists, hasDate, err := s.checkTaskInNotion(ctx, task.TaskID)
		if err != nil {
			log.Printf("Error checking task %s in Notion: %v", task.TaskID, err)
			continue
		}

		if !exists {
			// Task was deleted from Notion, remove from our database
			log.Printf("Task %s no longer exists in Notion, removing from database", task.TaskID)
			s.db.DeleteTask(task.TaskID)
			continue
		}

		// Send notifications based on tag
		if err := s.sendNotification(task, hasDate); err != nil {
			log.Printf("Error sending notification for task %s: %v", task.TaskID, err)
		}
	}

	log.Printf("Task check completed")
}

// checkTaskInNotion verifies if a task exists in Notion and checks if it has a date
func (s *Scheduler) checkTaskInNotion(ctx context.Context, taskID string) (exists bool, hasDate bool, err error) {
	// Query Notion to get the task
	page, err := s.notionClient.GetPage(ctx, taskID)
	if err != nil {
		// If task not found, it was deleted
		return false, false, nil
	}

	// Check if Date property exists and has a value
	if dateProp, ok := page.Properties["Date"]; ok {
		if date, ok := dateProp.(*notionapi.DateProperty); ok && date.Date != nil {
			hasDate = true
		}
	}

	return true, hasDate, nil
}

// sendNotification sends appropriate notification based on task tag
func (s *Scheduler) sendNotification(task database.TaskMetadata, hasDate bool) error {
	var message string
	taskPreview := truncateString(task.TaskTitle, 50)
	taskURL := fmt.Sprintf("https://notion.so/%s", task.TaskID)

	switch task.LLMTag {
	case "date":
		if !hasDate {
			message = fmt.Sprintf("⏰ Task with deadline has no date set!\n\n"+
				"Task: %s\n\n"+
				"You mentioned a deadline, but no date was added. Consider setting one.\n\n"+
				"[Open in Notion](%s)", taskPreview, taskURL)
		} else {
			// Task has date, no notification needed
			return nil
		}

	case "journal":
		message = fmt.Sprintf("📔 Possible journal entry in tasks!\n\n"+
			"Task: %s\n\n"+
			"This looks like a journal entry. Consider moving it to your journal database.\n\n"+
			"[Open in Notion](%s)", taskPreview, taskURL)

	case "link":
		message = fmt.Sprintf("🔗 Link-only task detected!\n\n"+
			"Task: %s\n\n"+
			"This task is just a link. Please give it a descriptive name.\n\n"+
			"[Open in Notion](%s)", taskPreview, taskURL)

	default:
		// No notification for regular tasks
		return nil
	}

	// Send message to authorized user
	msg := tgbotapi.NewMessage(s.authorizedUserID, message)
	msg.ParseMode = "Markdown"
	msg.DisableWebPagePreview = true

	_, err := s.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	log.Printf("Sent notification for task %s (tag: %s)", task.TaskID, task.LLMTag)
	return nil
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
