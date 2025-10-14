package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type TaskMetadata struct {
	ID        int       `json:"id"`
	TaskID    string    `json:"task_id"`
	TaskTitle string    `json:"task_title"`
	LLMTag    string    `json:"llm_tag"`
	CreatedAt time.Time `json:"created_at"`
}

type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection and initializes the schema
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Printf("Database initialized successfully at %s", dbPath)
	return db, nil
}

// initSchema creates the necessary tables if they don't exist
func (db *DB) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS task_metadata (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL UNIQUE,
		task_title TEXT NOT NULL,
		llm_tag TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_created_at ON task_metadata(created_at);
	CREATE INDEX IF NOT EXISTS idx_llm_tag ON task_metadata(llm_tag);
	`

	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

// StoreTaskMetadata stores task metadata in the database
func (db *DB) StoreTaskMetadata(taskID, taskTitle, llmTag string) error {
	query := `
		INSERT INTO task_metadata (task_id, task_title, llm_tag, created_at)
		VALUES (?, ?, ?, ?)
	`

	_, err := db.conn.Exec(query, taskID, taskTitle, llmTag, time.Now())
	if err != nil {
		return fmt.Errorf("failed to store task metadata: %w", err)
	}

	log.Printf("Stored task metadata: ID=%s, Tag=%s", taskID, llmTag)
	return nil
}

// GetTasksSince retrieves all tasks created since the specified time
func (db *DB) GetTasksSince(since time.Time) ([]TaskMetadata, error) {
	query := `
		SELECT id, task_id, task_title, llm_tag, created_at
		FROM task_metadata
		WHERE created_at >= ?
		ORDER BY created_at DESC
	`

	rows, err := db.conn.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []TaskMetadata
	for rows.Next() {
		var task TaskMetadata
		err := rows.Scan(&task.ID, &task.TaskID, &task.TaskTitle, &task.LLMTag, &task.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// DeleteTask removes task metadata from the database
func (db *DB) DeleteTask(taskID string) error {
	query := `DELETE FROM task_metadata WHERE task_id = ?`
	_, err := db.conn.Exec(query, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}
