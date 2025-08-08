// Package utils provides task-related helpers.
// revive:disable:var-naming - allow conventional 'utils' package name
package utils

import (
	"encoding/json"
	"time"
)

// Task state constants for consistent status values
const (
	TaskStateQueued    = "queued"    // Task is waiting in queue
	TaskStateRunning   = "running"   // Task is currently executing
	TaskStateCompleted = "completed" // Task finished successfully
	TaskStateFailed    = "failed"    // Task failed with an error
	TaskStateCancelled = "cancelled" // Task was cancelled
)

// LogEntry represents a log message from a task with metadata
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"` // When the log entry was created
	Message   string    `json:"message"`   // Log message content
	Level     string    `json:"level"`     // Log level: info, warn, error
}

// Task represents a background task that can be monitored through the system
type Task struct {
	// Basic identification
	ID       string                 `json:"id"`       // Unique task identifier (UUID)
	Function string                 `json:"function"` // Name of the function to execute
	Args     map[string]interface{} `json:"args"`     // Arguments for the function

	// Execution status
	Status string `json:"status"`           // Current state (using TaskState constants)
	Error  string `json:"error,omitempty"`  // Error message if task failed
	Result []byte `json:"result,omitempty"` // Serialized result data if any

	// Logging
	Logs []LogEntry `json:"logs,omitempty"` // Log entries generated during execution

	// Timing information
	CreatedAt time.Time  `json:"created_at"`           // When task was created and added to queue
	StartedAt *time.Time `json:"started_at,omitempty"` // When task started execution
	EndedAt   *time.Time `json:"ended_at,omitempty"`   // When task finished execution
	UpdatedAt time.Time  `json:"updated_at"`           // Last time task status was updated
}

// NewTask creates a new task with the given function name and arguments
func NewTask(funcName string, args interface{}) *Task {
	now := time.Now()
	return &Task{
		Function:  funcName,
		Args:      convertToMap(args),
		Status:    TaskStateQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// revive:enable:var-naming
// convertToMap ensures an interface is converted to a map[string]interface{}
func convertToMap(data interface{}) map[string]interface{} {
	if data == nil {
		return map[string]interface{}{}
	}

	// If it's already a map, return it
	if m, ok := data.(map[string]interface{}); ok {
		return m
	}

	// Try to marshal and unmarshal to convert to map
	jsonData, err := json.Marshal(data)
	if err != nil {
		return map[string]interface{}{}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return map[string]interface{}{}
	}

	return result
}
