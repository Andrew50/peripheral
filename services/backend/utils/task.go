package utils

import (
	"time"
)

// Task states
const (
	TaskStateQueued    = "queued"
	TaskStateRunning   = "running"
	TaskStateCompleted = "completed"
	TaskStateFailed    = "failed"
	TaskStateCancelled = "cancelled"
)

// LogEntry represents a log entry for a task
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Level     string    `json:"level"` // info, warning, error
}

// Task represents a background task
type Task struct {
	ID        string                 `json:"id"`
	Status    string                 `json:"status"`
	Function  string                 `json:"function"`
	Args      map[string]interface{} `json:"args,omitempty"`
	Result    []byte                 `json:"result,omitempty"`
	Logs      []LogEntry             `json:"logs,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Error     string                 `json:"error,omitempty"`
}
