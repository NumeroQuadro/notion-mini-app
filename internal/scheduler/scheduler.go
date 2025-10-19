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
	"github.com/numero_quadro/notion-mini-app/internal/gemini"
	"github.com/numero_quadro/notion-mini-app/internal/notion"
)

type Scheduler struct {
	notionClient     *notion.Client
	bot              *tgbotapi.BotAPI
	authorizedUserID int64
	checkTime        string // Format: "15:04" (HH:MM in 24-hour format)
	timezone         *time.Location
	geminiClient     *gemini.Client
}

// NewScheduler creates a new scheduler instance
func NewScheduler(notionClient *notion.Client, bot *tgbotapi.BotAPI, authorizedUserID int64, checkTime string, geminiClient *gemini.Client) *Scheduler {
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
		geminiClient:     geminiClient,
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

	// Step 0: Ensure all undone tasks (excluding 'sometimes-later') have llm_tag set
	// This covers tasks added directly in Notion bypassing the bot.
	if err := s.ensureTagsForUndoneTasks(ctx); err != nil {
		log.Printf("Error ensuring tags for undone tasks: %v", err)
		errorMsg := tgbotapi.NewMessage(s.authorizedUserID,
			fmt.Sprintf("❌ Error preparing tasks for check: %v", err))
		s.bot.Send(errorMsg)
		return
	}

	// Send header message to separate this batch from previous ones
	checkTime := time.Now().In(s.timezone)
	headerMsg := tgbotapi.NewMessage(s.authorizedUserID,
		fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━\n📋 **Daily Task Check**\n🕐 %s\n━━━━━━━━━━━━━━━━━━━━",
			checkTime.Format("Mon, 02 Jan 2006 15:04 MST")))
	headerMsg.ParseMode = "Markdown"
	s.bot.Send(headerMsg)

	// Query ALL non-done tasks from Notion (not just last 24h from local DB)
	tasks, err := s.notionClient.GetRecentTasks(ctx, "tasks", 1000) // Get up to 1000 tasks
	if err != nil {
		log.Printf("Error retrieving tasks from Notion: %v", err)
		errorMsg := tgbotapi.NewMessage(s.authorizedUserID,
			fmt.Sprintf("❌ Error checking tasks: %v", err))
		s.bot.Send(errorMsg)
		return
	}

	log.Printf("Found %d non-done tasks to check", len(tasks))

	notificationCount := 0
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
		} else {
			// Only count if notification was actually sent
			if (llmTag == "date" && !hasDate) || llmTag == "journal" || llmTag == "link" {
				notificationCount++
			}
		}
	}

	// Send footer message with summary
	var footerText string
	if notificationCount == 0 {
		footerText = "✅ All tasks look good! No issues found."
	} else {
		footerText = fmt.Sprintf("📊 Found %d task(s) needing attention", notificationCount)
	}
	footerMsg := tgbotapi.NewMessage(s.authorizedUserID,
		fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━\n%s\n━━━━━━━━━━━━━━━━━━━━", footerText))
	s.bot.Send(footerMsg)

	log.Printf("Task check completed: %d notifications sent", notificationCount)
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

// ensureTagsForUndoneTasks tags all undone tasks (excluding 'sometimes-later') that lack an llm_tag
// Returns an error if the operation fails critically
func (s *Scheduler) ensureTagsForUndoneTasks(ctx context.Context) error {
	if s.geminiClient == nil {
		log.Printf("Gemini client not configured; skipping pre-tagging step")
		return nil
	}

	// Fetch a broad set of undone tasks without 'sometimes-later'
	tasks, err := s.notionClient.GetUndoneTasksExcludingSometimesLater(ctx, "tasks", 1000)
	if err != nil {
		log.Printf("Pre-tagging: failed to fetch tasks: %v", err)
		return fmt.Errorf("failed to fetch tasks: %w", err)
	}

	log.Printf("Found %d tasks to check for tagging", len(tasks))

	tagged := 0
	skipped := 0
	errors := 0

	for _, task := range tasks {
		// Skip if already tagged
		if existingTag, ok := task.Properties["llm_tag"].(string); ok && strings.TrimSpace(existingTag) != "" {
			skipped++
			continue
		}

		// Get tag from Gemini
		tag, err := s.geminiClient.TagTask(task.Title)
		if err != nil || strings.TrimSpace(tag) == "" {
			if err != nil {
				log.Printf("Pre-tagging: gemini failed for %s: %v", task.ID, err)
			}
			tag = "task"
		}

		if err := s.notionClient.UpdateTaskLLMTag(task.ID, tag); err != nil {
			log.Printf("Pre-tagging: failed to update llm_tag for %s: %v", task.ID, err)
			errors++
		} else {
			log.Printf("Pre-tagging: successfully tagged task %s with '%s'", task.ID, tag)
			tagged++
		}

		// Small delay to avoid rate limits
		time.Sleep(300 * time.Millisecond)
	}

	log.Printf("Pre-tagging complete. tagged=%d skipped=%d errors=%d", tagged, skipped, errors)

	// If we had critical errors, return an error
	if errors > 0 && errors == len(tasks) {
		return fmt.Errorf("failed to tag any tasks, %d errors occurred", errors)
	}

	return nil
}
