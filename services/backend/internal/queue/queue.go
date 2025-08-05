package queue

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// parseError extracts structured error information from various error sources
func parseError(errorSource interface{}) (*ErrorDetails, string) {
	if errorSource == nil {
		return nil, ""
	}

	// Try to parse as structured error object
	if errorMap, ok := errorSource.(map[string]interface{}); ok {
		errorDetails := &ErrorDetails{}

		if errType, ok := errorMap["type"].(string); ok {
			errorDetails.Type = errType
		}
		if errMsg, ok := errorMap["message"].(string); ok {
			errorDetails.Message = errMsg
		}
		if errTrace, ok := errorMap["traceback"].(string); ok {
			errorDetails.Traceback = errTrace
		}

		// Parse frames array if present
		if framesInterface, ok := errorMap["frames"]; ok {
			if framesArray, ok := framesInterface.([]interface{}); ok {
				frames := make([]string, 0, len(framesArray))
				for _, frame := range framesArray {
					if frameStr, ok := frame.(string); ok {
						frames = append(frames, frameStr)
					}
				}
				errorDetails.Frames = frames
			}
		}

		// Return structured error and formatted message
		if errorDetails.Type != "" || errorDetails.Message != "" {
			return errorDetails, fmt.Sprintf("%s: %s", errorDetails.Type, errorDetails.Message)
		}
	}

	// Fall back to string error
	if errorStr, ok := errorSource.(string); ok {
		return nil, errorStr
	}

	return nil, ""
}

// logError logs error information with traceback and optional frame details
func logError(taskID string, errorDetails *ErrorDetails, errorMsg string) {
	if errorDetails != nil {
		log.Printf("❌ Task %s failed with %s: %s", taskID, errorDetails.Type, errorDetails.Message)
		if errorDetails.Traceback != "" {
			log.Printf("📋 Task %s traceback:\n%s", taskID, errorDetails.Traceback)
		}

		// Optionally log individual frames for detailed debugging
		if len(errorDetails.Frames) > 0 {
			log.Printf("🔍 Task %s has %d traceback frames available for detailed analysis", taskID, len(errorDetails.Frames))
			// Uncomment the following lines to print individual frames (useful for debugging):
			// log.Printf("📝 Task %s detailed frames:", taskID)
			// for i, frame := range errorDetails.Frames {
			//     log.Printf("   Frame %d: %s", i+1, strings.TrimSpace(frame))
			// }
		}
	} else if errorMsg != "" {
		log.Printf("❌ Task %s failed: %s", taskID, errorMsg)
	}
}

// ErrorDetails represents structured error information from the Python worker
type ErrorDetails struct {
	Type      string   `json:"type"`
	Message   string   `json:"message"`
	Traceback string   `json:"traceback"`
	Frames    []string `json:"frames,omitempty"` // Individual traceback lines for programmatic access
}

// ResultUpdate represents a task status update
type ResultUpdate struct {
	TaskID       string                 `json:"task_id"`
	Status       string                 `json:"status"` // queued|running|completed|error|cancelled
	Data         map[string]interface{} `json:"data,omitempty"`
	Error        string                 `json:"error,omitempty"`         // Legacy string error
	ErrorDetails *ErrorDetails          `json:"error_details,omitempty"` // New structured error
	UpdatedAt    time.Time              `json:"updated_at"`
}

// BacktestResult represents the result of a backtest task
type BacktestResult struct {
	Success                   bool               `json:"success"`
	StrategyID                int                `json:"strategy_id"`
	Version                   int                `json:"version"`
	TotalInstances            int                `json:"total_instances"`
	PositiveInstances         int                `json:"positive_instances"`
	DateRange                 []string           `json:"date_range"`
	SymbolsProcessed          int                `json:"symbols_processed"`
	ExecutionType             string             `json:"execution_type,omitempty"`
	SuccessfulClassifications int                `json:"successful_classifications,omitempty"`
	Instances                 []map[string]any   `json:"instances"`
	StrategyPrints            string             `json:"strategy_prints,omitempty"`
	StrategyPlots             []StrategyPlotData `json:"strategy_plots,omitempty"`
	ResponseImages            []string           `json:"response_images,omitempty"`
	ErrorMessage              string             `json:"error_message,omitempty"` // Legacy field
	Error                     *ErrorDetails      `json:"error,omitempty"`         // New structured error
}

// StrategyPlotData represents plotly plot data
type StrategyPlotData struct {
	PlotID      int            `json:"plotID"`
	Data        map[string]any `json:"data"`
	TitleTicker string         `json:"titleTicker,omitempty"`
}

// ScreeningResult represents the result of a screening task
type ScreeningResult struct {
	Success      bool                     `json:"success"`
	Instances    []map[string]interface{} `json:"instances"`
	Error        string                   `json:"error,omitempty"`         // Legacy string error
	ErrorDetails *ErrorDetails            `json:"error_details,omitempty"` // New structured error
}

// AlertResult represents the result of an alert task
type AlertResult struct {
	Success      bool                     `json:"success"`
	Instances    []map[string]interface{} `json:"instances"`
	UsedSymbols  []string                 `json:"used_symbols,omitempty"`  // Tickers actually accessed during execution
	ErrorMessage string                   `json:"error_message,omitempty"` // Legacy field
	Error        *ErrorDetails            `json:"error,omitempty"`         // New structured error
}

// CreateStrategyResult represents the result of a strategy creation task
type CreateStrategyResult struct {
	Success      bool          `json:"success"`
	Strategy     *Strategy     `json:"strategy,omitempty"`
	Error        string        `json:"error,omitempty"`         // Legacy string error
	ErrorDetails *ErrorDetails `json:"error_details,omitempty"` // New structured error
}

// Strategy represents a created strategy
type Strategy struct {
	StrategyID         int      `json:"strategyId"`
	UserID             int      `json:"userId"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Prompt             string   `json:"prompt"`
	PythonCode         string   `json:"pythonCode"`
	Score              int      `json:"score,omitempty"`
	Version            int      `json:"version,omitempty"`
	CreatedAt          string   `json:"createdAt,omitempty"`
	IsAlertActive      bool     `json:"isAlertActive,omitempty"`
	AlertThreshold     *float64 `json:"alertThreshold,omitempty"`
	AlertUniverse      []string `json:"alertUniverse,omitempty"`
	MinTimeframe       string   `json:"minTimeframe,omitempty"`
	AlertLastTriggerAt *string  `json:"alertLastTriggerAt,omitempty"`
}

// PythonAgentResult represents the result of a general python agent task
type PythonAgentResult struct {
	Success        bool               `json:"success"`
	Result         any                `json:"result"`
	Prints         string             `json:"prints"`
	Plots          []StrategyPlotData `json:"plots"`
	ResponseImages []string           `json:"responseImages"`
	ExecutionID    string             `json:"executionID"`
	Error          string             `json:"error,omitempty"`         // Legacy string error
	ErrorDetails   *ErrorDetails      `json:"error_details,omitempty"` // New structured error
}

// UnifiedMessage represents the new format from worker context system
type UnifiedMessage struct {
	TaskID      string                 `json:"task_id"`
	MessageType string                 `json:"message_type"` // update | heartbeat | result
	Status      string                 `json:"status"`       // running | completed | error | cancelled | heartbeat
	Data        map[string]interface{} `json:"data,omitempty"`
	Error       interface{}            `json:"error,omitempty"` // Can be string or structured error object
}

// Handle provides control over a queued task
type Handle struct {
	Updates <-chan ResultUpdate
	Cancel  func() error

	// Internal fields for cleanup
	taskID     string
	taskType   string
	statusID   string
	conn       *data.Conn
	updatesCh  chan ResultUpdate
	cancelCh   chan struct{}
	cancelOnce sync.Once
	mu         sync.RWMutex
	cancelled  bool
}

// ProgressCallback is a function type for receiving progress updates
type ProgressCallback func(update ResultUpdate)

// Await waits for task completion and returns the typed result with optional progress callback
func (h *Handle) Await(ctx context.Context, resultType interface{}, progressCallback ProgressCallback) (interface{}, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-h.cancelCh:
			return nil, fmt.Errorf("task was cancelled")
		case update := <-h.updatesCh:
			// Call progress callback for non-terminal statuses
			if progressCallback != nil && (update.Status == "running" || update.Status == "queued") {
				progressCallback(update)
			}

			// Only process terminal statuses
			if update.Status == "completed" || update.Status == "error" || update.Status == "cancelled" {
				// Handle error and cancelled cases
				if update.Status == "error" {
					// Log detailed error information
					logError(update.TaskID, update.ErrorDetails, update.Error)

					// Return appropriate error message
					if update.ErrorDetails != nil {
						return nil, fmt.Errorf("task failed with %s: %s", update.ErrorDetails.Type, update.ErrorDetails.Message)
					}
					return nil, fmt.Errorf("task failed: %s", update.Error)
				}
				if update.Status == "cancelled" {
					return nil, fmt.Errorf("task was cancelled")
				}

				// Handle success case - unmarshal the data to the provided type
				if update.Status == "completed" && update.Data != nil {
					// Convert map to JSON and then unmarshal to the provided type
					dataJSON, err := json.Marshal(update.Data)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal result data: %w", err)
					}

					if err := json.Unmarshal(dataJSON, resultType); err != nil {
						return nil, fmt.Errorf("failed to unmarshal result data to %T: %w", resultType, err)
					}

					return resultType, nil
				}

				return nil, fmt.Errorf("unexpected task status: %s", update.Status)
			}
			// Continue waiting for terminal status
		}
	}
}

// AwaitTyped is deprecated, use Await instead
func (h *Handle) AwaitTyped(ctx context.Context, resultType interface{}) (interface{}, error) {
	return h.Await(ctx, resultType, nil)
}

// TaskData represents the structure of a task in the queue (matches worker expectation)
type TaskData struct {
	TaskID            string `json:"task_id"`
	TaskType          string `json:"task_type"`
	Kwargs            string `json:"kwargs"` // JSON string of arguments
	CreatedAt         string `json:"created_at"`
	Priority          string `json:"priority"`
	StatusID          string `json:"status_id"`          // Unique ID for status updates
	HeartbeatInterval int    `json:"heartbeat_interval"` // Heartbeat interval in seconds
}

// WorkerHeartbeat represents a worker's heartbeat data
type WorkerHeartbeat struct {
	WorkerID      string                 `json:"worker_id"`
	Status        string                 `json:"status"`
	Timestamp     string                 `json:"timestamp"`
	UptimeSeconds float64                `json:"uptime_seconds"`
	ActiveTask    *string                `json:"active_task"`
	QueueStats    map[string]interface{} `json:"queue_stats"`
}

// QueueTask enqueues a task and returns a handle for monitoring and control
func QueueTask(ctx context.Context, conn *data.Conn, taskType string, args map[string]interface{}, priority bool, maxRetries int, timeout time.Duration) (*Handle, error) {
	// Generate unique task ID and status ID
	taskID := uuid.New().String()
	statusID := uuid.New().String()

	// Create task data
	priorityStr := "normal"
	if priority {
		priorityStr = "high"
	}

	// Convert args to JSON string for kwargs
	kwargsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task args: %w", err)
	}

	taskData := TaskData{
		TaskID:            taskID,
		TaskType:          taskType,
		Kwargs:            string(kwargsJSON),
		CreatedAt:         time.Now().Format(time.RFC3339),
		Priority:          priorityStr,
		StatusID:          statusID,
		HeartbeatInterval: 5, // 5 second heartbeat interval
	}

	// Marshal task data
	taskJSON, err := json.Marshal(taskData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task data: %w", err)
	}

	// Create handle with channels BEFORE pushing to queue
	updatesCh := make(chan ResultUpdate, 10) // Buffered channel for updates
	cancelCh := make(chan struct{})

	handle := &Handle{
		Updates:   updatesCh,
		taskID:    taskID,
		taskType:  taskType,
		statusID:  statusID,
		conn:      conn,
		updatesCh: updatesCh,
		cancelCh:  cancelCh,
	}

	// Set up cancel function
	handle.Cancel = func() error {
		handle.cancelOnce.Do(func() {
			handle.mu.Lock()
			handle.cancelled = true
			handle.mu.Unlock()

			close(cancelCh)
		})
		return nil
	}

	// Create a channel to signal when subscription is ready
	subscriptionReady := make(chan struct{})

	// Start unified event loop BEFORE pushing to queue to ensure subscription is active
	go handle.eventLoop(ctx, maxRetries, timeout, priority, statusID, 5, subscriptionReady) // Pass heartbeat interval and ready signal

	// Wait for subscription to be established
	select {
	case <-subscriptionReady:
		// Subscription is ready
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for subscription to be established")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Determine queue name
	queueName := "task_queue"
	if priority {
		queueName = "priority_task_queue"
	}

	// Push task to queue AFTER subscription is established
	err = conn.Cache.RPush(ctx, queueName, string(taskJSON)).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to push task to queue %s: %w", queueName, err)
	}

	// Send initial queued update
	initialUpdate := ResultUpdate{
		TaskID:    taskID,
		Status:    "queued",
		Data:      map[string]interface{}{},
		UpdatedAt: time.Now(),
	}

	select {
	case updatesCh <- initialUpdate:
	default:
		// Channel full, skip initial update
	}

	log.Printf("✅ Task %s queued successfully to %s with status_id %s", taskID, queueName, statusID)
	return handle, nil
}

// AwaitTypedResult provides a generic typed await method with optional progress callback
func AwaitTypedResult[T any](ctx context.Context, handle *Handle, progressCallback ProgressCallback) (*T, error) {
	var result T
	_, err := handle.Await(ctx, &result, progressCallback)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// eventLoop combines subscription and watchdog functionality in a single goroutine
func (h *Handle) eventLoop(ctx context.Context, maxRetries int, timeout time.Duration, priority bool, statusID string, heartbeatInterval int, subscriptionReady chan struct{}) {
	// Subscribe to unified task status channel
	statusChannel := fmt.Sprintf("task_status:%s", statusID)
	pubsub := h.conn.Cache.Subscribe(ctx, statusChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Printf("🔔 Subscribed to status channel: %s", statusChannel)

	// Signal that subscription is ready
	if subscriptionReady != nil {
		close(subscriptionReady)
	}

	retryCount := 0
	lastHeartbeat := time.Now()
	taskStarted := false
	var startTime time.Time

	checkInterval := time.Duration(heartbeatInterval) * time.Second
	heartbeatTimeout := time.Duration(heartbeatInterval*3) * time.Second
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Timer for waiting for the first message (assignment indicator)
	const firstMsgTimeout = 60 * time.Second
	startTimer := time.NewTimer(firstMsgTimeout)
	defer startTimer.Stop()

retryLoop:
	for retryCount <= maxRetries {
		select {
		case <-ctx.Done():
			return
		case <-h.cancelCh:
			return
		case <-startTimer.C:
			if !taskStarted {
				log.Printf("⚠️ Task %s never produced a start message", h.taskID)
				break retryLoop // Break to retry logic
			}
		case msg := <-ch:
			if msg == nil {
				continue
			}

			var unifiedMsg UnifiedMessage
			if err := json.Unmarshal([]byte(msg.Payload), &unifiedMsg); err != nil {
				log.Printf("❌ Failed to unmarshal unified message: %v", err)
				continue
			}

			// Verify this is for our task
			if unifiedMsg.TaskID != h.taskID {
				continue
			}

			// Handle different message types
			switch unifiedMsg.MessageType {
			case "heartbeat":
				// Update heartbeat timestamp
				lastHeartbeat = time.Now()
				log.Printf("💓 Heartbeat received for task %s", h.taskID)
				continue

			case "progress":
				// Status update from task execution
				if unifiedMsg.Status == "running" && !taskStarted {
					taskStarted = true
					// Parse start time from message data or use current time
					if ts, ok := unifiedMsg.Data["started_at"].(string); ok {
						if parsedTime, err := time.Parse(time.RFC3339, ts); err == nil {
							startTime = parsedTime
						} else {
							startTime = time.Now()
						}
					} else {
						startTime = time.Now()
					}
					log.Printf("✅ Task %s started", h.taskID)
					// Stop the first message timer since we've received the start signal
					startTimer.Stop()
				}
				lastHeartbeat = time.Now()

				resultUpdate := ResultUpdate{
					TaskID:    h.taskID,
					Status:    unifiedMsg.Status,
					Data:      unifiedMsg.Data,
					UpdatedAt: time.Now(),
				}
				select {
				case h.updatesCh <- resultUpdate:
				default:
					log.Printf("❌ Channel full, skipping update for task %s", h.taskID)
					// Channel full, skip this update
				}

			case "result":
				// Final result from task execution
				resultUpdate := ResultUpdate{
					TaskID:    h.taskID,
					Status:    unifiedMsg.Status,
					Data:      unifiedMsg.Data,
					UpdatedAt: time.Now(),
				}

				// Handle structured error information
				var errorDetails *ErrorDetails
				var errorStr string

				// First check the unified message error field
				if unifiedMsg.Error != nil {
					errorDetails, errorStr = parseError(unifiedMsg.Error)
				}

				// Fall back to checking the Data field if no error found yet
				if errorDetails == nil && errorStr == "" && unifiedMsg.Data != nil {
					if errorObj, exists := unifiedMsg.Data["error"]; exists {
						errorDetails, errorStr = parseError(errorObj)
					}
				}

				resultUpdate.ErrorDetails = errorDetails
				resultUpdate.Error = errorStr

				// Log error details if this is an error status
				if unifiedMsg.Status == "error" {
					logError(h.taskID, errorDetails, errorStr)
				}

				// Send final update to channel (non-blocking)
				select {
				case h.updatesCh <- resultUpdate:
				default:
					// Channel full, skip this update
				}

				// Task completed successfully
				if unifiedMsg.Status == "completed" || unifiedMsg.Status == "error" || unifiedMsg.Status == "cancelled" {
					return
				}
			}

		case <-ticker.C:
			now := time.Now()

			// Only perform timeout checks if task has started
			if taskStarted {
				// Check if task has been running too long
				if now.Sub(startTime) > timeout {
					log.Printf("⏰ Task %s timed out after %v", h.taskID, timeout)
					break retryLoop // Break to retry logic
				}

				// Check if we've missed heartbeats
				if now.Sub(lastHeartbeat) > heartbeatTimeout {
					log.Printf("💀 Task %s missed heartbeats - last heartbeat %v ago", h.taskID, now.Sub(lastHeartbeat))
					break retryLoop // Break to retry logic
				}
			} else {
				// If task hasn't started after reasonable time, consider it failed
				if now.Sub(time.Now().Add(-firstMsgTimeout)) > 2*time.Minute {
					log.Printf("⚠️ Task %s never started after 2 minutes", h.taskID)
					break retryLoop // Break to retry logic
				}
			}
		}
	}

	// Worker died or task timed out - retry logic
	if retryCount >= maxRetries {
		failureReason := fmt.Sprintf("max retries (%d) exceeded", maxRetries)
		h.markTaskAsFailed(failureReason)
		log.Printf("❌ Task %s permanently failed after %d retries", h.taskID, maxRetries)
		return
	}

	log.Printf("🔄 Task %s failed, retrying (%d/%d)", h.taskID, retryCount+1, maxRetries)
	retryCount++

	// Requeue the task
	if err := h.requeueTask(ctx, retryCount, "worker failure or timeout", priority, statusID, heartbeatInterval); err != nil {
		log.Printf("❌ Failed to requeue task %s (retry %d): %v", h.taskID, retryCount, err)
		h.markTaskAsFailed(fmt.Sprintf("failed to requeue: %v", err))
		return
	}

	// Reset task state for retry
	taskStarted = false
	startTimer.Reset(firstMsgTimeout)
}

// requeueTask requeues the task with updated retry information
func (h *Handle) requeueTask(ctx context.Context, retryCount int, reason string, priority bool, statusID string, heartbeatInterval int) error {
	// Create requeue task data
	priorityStr := "normal"
	if priority {
		priorityStr = "high"
	}

	// Create retry args
	retryArgs := map[string]interface{}{
		"retry_count":  retryCount,
		"retry_reason": reason,
	}

	kwargsJSON, err := json.Marshal(retryArgs)
	if err != nil {
		return fmt.Errorf("failed to marshal retry args: %w", err)
	}

	taskData := TaskData{
		TaskID:            h.taskID,
		TaskType:          h.taskType, // Use the original task type
		Kwargs:            string(kwargsJSON),
		CreatedAt:         time.Now().Format(time.RFC3339),
		Priority:          priorityStr,
		StatusID:          statusID, // Use the same statusID for requeue
		HeartbeatInterval: heartbeatInterval,
	}

	// Determine queue name
	queueName := "task_queue"
	if priorityStr == "high" {
		queueName = "priority_task_queue"
	}

	// Marshal and push to queue
	taskJSON, err := json.Marshal(taskData)
	if err != nil {
		return fmt.Errorf("failed to marshal task data: %w", err)
	}

	err = h.conn.Cache.LPush(ctx, queueName, string(taskJSON)).Err()
	if err != nil {
		return fmt.Errorf("failed to push task to queue: %w", err)
	}

	log.Printf("🔄 Task %s requeued (retry %d)", h.taskID, retryCount)
	return nil
}

// markTaskAsFailed marks the task as permanently failed
func (h *Handle) markTaskAsFailed(reason string) {
	// Send error update to channel if possible
	errorUpdate := ResultUpdate{
		TaskID:    h.taskID,
		Status:    "error",
		Error:     reason,
		Data:      map[string]interface{}{"failure_type": "watchdog_failure"},
		UpdatedAt: time.Now(),
	}

	select {
	case h.updatesCh <- errorUpdate:
	default:
		// Channel full or closed
	}

	// Log the watchdog failure
	log.Printf("❌ Task %s marked as failed by watchdog: %s", h.taskID, reason)
}

// Convenience wrapper functions for common task types

// QueueBacktest queues a backtest task with default settings
func QueueBacktest(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*Handle, error) {
	return QueueTask(ctx, conn, "backtest", args, false, 3, 10*time.Minute)
}

// QueueBacktestTyped queues a backtest task and returns a typed result
func QueueBacktestTyped(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*BacktestResult, error) {
	handle, err := QueueTask(ctx, conn, "backtest", args, false, 3, 10*time.Minute)
	if err != nil {
		return nil, err
	}

	return AwaitTypedResult[BacktestResult](ctx, handle, nil)
}

// QueueScreening queues a screening task with default settings
func QueueScreening(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*Handle, error) {
	return QueueTask(ctx, conn, "screen", args, false, 3, 5*time.Minute)
}

// QueueScreeningTyped queues a screening task and returns a typed result
func QueueScreeningTyped(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*ScreeningResult, error) {
	handle, err := QueueTask(ctx, conn, "screen", args, false, 3, 5*time.Minute)
	if err != nil {
		return nil, err
	}

	return AwaitTypedResult[ScreeningResult](ctx, handle, nil)
}

// QueueAlert queues an alert task with default settings
func QueueAlert(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*Handle, error) {
	return QueueTask(ctx, conn, "alert", args, false, 3, 2*time.Minute)
}

// QueueAlertTyped queues an alert task and returns a typed result
func QueueAlertTyped(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*AlertResult, error) {
	handle, err := QueueTask(ctx, conn, "alert", args, false, 3, 2*time.Minute)
	if err != nil {
		return nil, err
	}

	return AwaitTypedResult[AlertResult](ctx, handle, nil)
}

// QueueCreateStrategy queues a strategy creation task with high priority
func QueueCreateStrategy(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*Handle, error) {
	return QueueTask(ctx, conn, "create_strategy", args, true, 2, 15*time.Minute)
}

// QueueCreateStrategyTyped queues a strategy creation task and returns a typed result
func QueueCreateStrategyTyped(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*CreateStrategyResult, error) {
	handle, err := QueueTask(ctx, conn, "create_strategy", args, true, 2, 15*time.Minute)
	if err != nil {
		return nil, err
	}

	return AwaitTypedResult[CreateStrategyResult](ctx, handle, nil)
}

// QueuePythonAgent queues a general python agent task with default settings
func QueuePythonAgent(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*Handle, error) {
	return QueueTask(ctx, conn, "python_agent", args, false, 3, 8*time.Minute)
}

// QueuePythonAgentTyped queues a general python agent task and returns a typed result
func QueuePythonAgentTyped(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*PythonAgentResult, error) {
	handle, err := QueueTask(ctx, conn, "python_agent", args, false, 3, 8*time.Minute)
	if err != nil {
		return nil, err
	}

	return AwaitTypedResult[PythonAgentResult](ctx, handle, nil)
}
