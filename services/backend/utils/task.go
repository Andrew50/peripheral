package utils

import (
	"time"
)

// Task state constants
const (
	TaskStateQueued    = "queued"
	TaskStateRunning   = "running"
	TaskStateCompleted = "completed"
	TaskStateFailed    = "failed"
	TaskStateCancelled = "cancelled"
)

// LogEntry represents a log message from a task
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Level     string    `json:"level"` // info, warn, error
}

// Task represents a background task that can be monitored
type Task struct {
	ID        string                 `json:"id"`
	Status    string                 `json:"status"`
	Function  string                 `json:"function"`
	Args      map[string]interface{} `json:"args"`
	Result    []byte                 `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Logs      []LogEntry             `json:"logs,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	StartedAt *time.Time             `json:"started_at,omitempty"`
	EndedAt   *time.Time             `json:"ended_at,omitempty"`
}
