package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jomei/notionapi"
	"github.com/numero_quadro/notion-mini-app/internal/notion"
)

type Scheduler struct {
	notionClient     *notion.Client
	bot              *tgbotapi.BotAPI
	authorizedUserID int64
	checkTime        string // Format: "15:04" (HH:MM in 24-hour format)
	timezone         *time.Location
}

// NewScheduler creates a new scheduler instance
func NewScheduler(notionClient *notion.Client, bot *tgbotapi.BotAPI, authorizedUserID int64, checkTime string) *Scheduler {
	if checkTime == "" {
		checkTime = "23:00" // Default to 11 PM
	}

	// Load timezone from environment variable or default to MSK
	tzName := os.Getenv("TZ")
	if tzName == "" {
		tzName = "Europe/Moscow" // Default to Moscow timezone
	}

	location, err := time.LoadLocation(tzName)
	if err != nil {
		log.Printf("Warning: Failed to load timezone '%s': %v. Using UTC.", tzName, err)
		location = time.UTC
	} else {
		log.Printf("Scheduler timezone set to: %s", tzName)
	}

	return &Scheduler{
		notionClient:     notionClient,
		bot:              bot,
		authorizedUserID: authorizedUserID,
		checkTime:        checkTime,
		timezone:         location,
	}
}

// Start begins the scheduler loop
func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("Starting scheduler with daily check at %s (timezone: %s)", s.checkTime, s.timezone.String())

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Scheduler stopped")
			return
		case now := <-ticker.C:
			// Convert current time to configured timezone
			localNow := now.In(s.timezone)
			currentTime := localNow.Format("15:04")

			// Check if current time matches check time
			if currentTime == s.checkTime {
				log.Printf("Running scheduled task check at %s %s", currentTime, s.timezone.String())
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

	// Query ALL non-done tasks from Notion (not just last 24h from local DB)
	tasks, err := s.notionClient.GetRecentTasks(ctx, "tasks", 1000) // Get up to 1000 tasks
	if err != nil {
		log.Printf("Error retrieving tasks from Notion: %v", err)
		return
	}

	log.Printf("Found %d non-done tasks to check", len(tasks))

	for _, task := range tasks {
		// Check if task has llm_tag property in Notion
		llmTag, hasTag := task.Properties["llm_tag"].(string)
		if !hasTag || llmTag == "" {
			log.Printf("Task %s has no llm_tag, skipping", task.ID)
			continue
		}

		// Check if task has Date property
		hasDate := false
		if _, ok := task.Properties["Date"]; ok {
			if dateStr, ok := task.Properties["Date"].(string); ok && dateStr != "" {
				hasDate = true
			}
		}

		// Send notifications based on tag
		if err := s.sendNotification(task, hasDate); err != nil {
			log.Printf("Error sending notification for task %s: %v", task.ID, err)
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
func (s *Scheduler) sendNotification(task notion.Task, hasDate bool) error {
	var message string
	taskPreview := truncateString(task.Title, 50)
	// Fix Notion URL format - remove hyphens from ID
	cleanID := strings.ReplaceAll(task.ID, "-", "")
	taskURL := fmt.Sprintf("https://notion.so/%s", cleanID)
	llmTag := task.Properties["llm_tag"].(string)

	switch llmTag {
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

	log.Printf("Sent notification for task %s (tag: %s)", task.ID, llmTag)
	return nil
}

// truncateString truncates a string to maxLen characters (UTF-8 safe)
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
