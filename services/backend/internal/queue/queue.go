package queue

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// ResultUpdate represents a task status update
type ResultUpdate struct {
	TaskID    string                 `json:"task_id"`
	Status    string                 `json:"status"` // queued|running|completed|error|cancelled
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	UpdatedAt time.Time              `json:"updated_at"`
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
	ErrorMessage              string             `json:"error_message,omitempty"`
}

// StrategyPlotData represents plotly plot data
type StrategyPlotData struct {
	PlotID      int            `json:"plotID"`
	Data        map[string]any `json:"data"`
	TitleTicker string         `json:"titleTicker,omitempty"`
}

// ScreeningResult represents the result of a screening task
type ScreeningResult struct {
	Success   bool                     `json:"success"`
	Instances []map[string]interface{} `json:"instances"`
	Error     string                   `json:"error,omitempty"`
}

// AlertResult represents the result of an alert task
type AlertResult struct {
	Success      bool                     `json:"success"`
	Instances    []map[string]interface{} `json:"instances"`
	ErrorMessage string                   `json:"error_message,omitempty"`
}

// CreateStrategyResult represents the result of a strategy creation task
type CreateStrategyResult struct {
	Success  bool      `json:"success"`
	Strategy *Strategy `json:"strategy,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// Strategy represents a created strategy
type Strategy struct {
	StrategyID     int      `json:"strategyId"`
	UserID         int      `json:"userId"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Prompt         string   `json:"prompt"`
	PythonCode     string   `json:"pythonCode"`
	Score          int      `json:"score,omitempty"`
	Version        int      `json:"version,omitempty"`
	CreatedAt      string   `json:"createdAt,omitempty"`
	IsAlertActive  bool     `json:"isAlertActive,omitempty"`
	AlertThreshold *float64 `json:"alertThreshold,omitempty"`
	AlertUniverse  []string `json:"alertUniverse,omitempty"`
}

// PythonAgentResult represents the result of a general python agent task
type PythonAgentResult struct {
	Success        bool     `json:"success"`
	Result         []any    `json:"result"`
	Prints         string   `json:"prints"`
	Plots          []any    `json:"plots"`
	ResponseImages []string `json:"responseImages"`
	ExecutionID    string   `json:"executionID"`
	Error          string   `json:"error,omitempty"`
}

// UnifiedMessage represents the new format from worker context system
type UnifiedMessage struct {
	TaskID      string                 `json:"task_id"`
	MessageType string                 `json:"message_type"` // update | heartbeat | result
	Status      string                 `json:"status"`       // running | completed | error | cancelled | heartbeat
	Data        map[string]interface{} `json:"data,omitempty"`
}

// Handle provides control over a queued task
type Handle struct {
	Updates    <-chan ResultUpdate
	Cancel     func() error
	Await      func(ctx context.Context) (ResultUpdate, error)
	AwaitTyped func(ctx context.Context, resultType interface{}) (interface{}, error)

	// Internal fields for cleanup
	taskID     string
	taskType   string
	statusID   string
	conn       *data.Conn
	updatesCh  chan ResultUpdate
	cancelCh   chan struct{}
	doneCh     chan struct{}
	cancelOnce sync.Once
	mu         sync.RWMutex
	cancelled  bool
	pubsub     *redis.PubSub
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

// TaskAssignment represents a task currently assigned to a worker
type TaskAssignment struct {
	WorkerID  string `json:"worker_id"`
	TaskID    string `json:"task_id"`
	StartedAt string `json:"started_at"`
	Status    string `json:"status"`
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

	// Determine queue name
	queueName := "task_queue"
	if priority {
		queueName = "priority_task_queue"
	}

	// Push task to queue
	err = conn.Cache.RPush(ctx, queueName, string(taskJSON)).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to push task to queue %s: %w", queueName, err)
	}

	// Create handle with channels
	updatesCh := make(chan ResultUpdate, 10) // Buffered channel for updates
	cancelCh := make(chan struct{})
	doneCh := make(chan struct{})

	handle := &Handle{
		Updates:   updatesCh,
		taskID:    taskID,
		taskType:  taskType,
		statusID:  statusID,
		conn:      conn,
		updatesCh: updatesCh,
		cancelCh:  cancelCh,
		doneCh:    doneCh,
	}

	// Set up cancel function
	handle.Cancel = func() error {
		handle.cancelOnce.Do(func() {
			handle.mu.Lock()
			handle.cancelled = true
			handle.mu.Unlock()

			// Unsubscribe from status updates to signal cancellation to worker
			if handle.pubsub != nil {
				handle.pubsub.Close()
			}

			close(cancelCh)
		})
		return nil
	}

	// Set up await function
	handle.Await = func(ctx context.Context) (ResultUpdate, error) {
		for {
			select {
			case <-ctx.Done():
				return ResultUpdate{}, ctx.Err()
			case <-cancelCh:
				return ResultUpdate{TaskID: taskID, Status: "cancelled", UpdatedAt: time.Now()}, nil
			case update := <-updatesCh:
				if update.Status == "completed" || update.Status == "error" || update.Status == "cancelled" {
					return update, nil
				}
				// Continue waiting for final status
			case <-doneCh:
				// Task monitoring completed, return error
				return ResultUpdate{}, fmt.Errorf("task monitoring completed without final result")
			}
		}
	}

	// Set up typed await function
	handle.AwaitTyped = func(ctx context.Context, resultType interface{}) (interface{}, error) {
		result, err := handle.Await(ctx)
		if err != nil {
			return nil, err
		}

		// Handle error and cancelled cases
		if result.Status == "error" {
			return nil, fmt.Errorf("task failed: %s", result.Error)
		}
		if result.Status == "cancelled" {
			return nil, fmt.Errorf("task was cancelled")
		}

		// Handle success case - unmarshal the data to the provided type
		if result.Status == "completed" && result.Data != nil {
			// Convert map to JSON and then unmarshal to the provided type
			dataJSON, err := json.Marshal(result.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result data: %w", err)
			}

			if err := json.Unmarshal(dataJSON, resultType); err != nil {
				return nil, fmt.Errorf("failed to unmarshal result data to %T: %w", resultType, err)
			}

			return resultType, nil
		}

		return nil, fmt.Errorf("unexpected task status: %s", result.Status)
	}

	// Start monitoring goroutines
	go handle.subscribeToUpdates(ctx, statusID)
	go handle.watchdog(ctx, maxRetries, timeout, priority, statusID, 5) // Pass heartbeat interval

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

// AwaitTypedResult provides a generic typed await method
func AwaitTypedResult[T any](ctx context.Context, handle *Handle) (*T, error) {
	var result T
	_, err := handle.AwaitTyped(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// subscribeToUpdates subscribes to Redis updates for this task using the new unified system
func (h *Handle) subscribeToUpdates(ctx context.Context, statusID string) {
	defer close(h.doneCh)
	defer close(h.updatesCh)

	// Subscribe to unified task status channel
	statusChannel := fmt.Sprintf("task_status:%s", statusID)
	pubsub := h.conn.Cache.Subscribe(ctx, statusChannel)
	h.pubsub = pubsub
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Printf("🔔 Subscribed to status channel: %s", statusChannel)

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.cancelCh:
			return
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
				// Just log heartbeat for debugging, don't send to user updates channel
				log.Printf("💓 Heartbeat received for task %s", h.taskID)
				continue

			case "update":
				// Status update from task execution
				resultUpdate := ResultUpdate{
					TaskID:    h.taskID,
					Status:    unifiedMsg.Status,
					Data:      unifiedMsg.Data,
					UpdatedAt: time.Now(),
				}

				// Send update to channel (non-blocking)
				select {
				case h.updatesCh <- resultUpdate:
				default:
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

				// Handle error message if present
				if errorMsg, exists := unifiedMsg.Data["error"]; exists {
					if errorStr, ok := errorMsg.(string); ok {
						resultUpdate.Error = errorStr
					}
				}

				// Send final update to channel (non-blocking)
				select {
				case h.updatesCh <- resultUpdate:
				default:
					// Channel full, skip this update
				}

				// Stop monitoring if task is complete
				if unifiedMsg.Status == "completed" || unifiedMsg.Status == "error" || unifiedMsg.Status == "cancelled" {
					return
				}
			}
		}
	}
}

// watchdog monitors the task for worker failures and handles retries
func (h *Handle) watchdog(ctx context.Context, maxRetries int, timeout time.Duration, priority bool, statusID string, heartbeatInterval int) {
	retryCount := 0

	for retryCount <= maxRetries {
		select {
		case <-ctx.Done():
			return
		case <-h.cancelCh:
			return
		case <-h.doneCh:
			return
		default:
		}

		// Wait for task assignment (task to be picked up by worker)
		assignment, err := h.waitForAssignment(ctx, 30*time.Second)
		if err != nil {
			if retryCount >= maxRetries {
				h.markTaskAsFailed("failed to get task assignment after max retries")
				return
			}
			retryCount++
			continue
		}

		// Monitor worker health
		if h.monitorWorkerHealth(ctx, assignment, timeout, statusID, heartbeatInterval) {
			// Task completed successfully or was cancelled
			return
		}

		// Worker died or task timed out
		if retryCount >= maxRetries {
			h.markTaskAsFailed(fmt.Sprintf("max retries (%d) exceeded", maxRetries))
			return
		}

		log.Printf("🔄 Task %s worker %s failed, retrying (%d/%d)", h.taskID, assignment.WorkerID, retryCount+1, maxRetries)
		retryCount++

		// Requeue the task
		if err := h.requeueTask(ctx, retryCount, "worker failure or timeout", priority, statusID, heartbeatInterval); err != nil {
			log.Printf("❌ Failed to requeue task %s: %v", h.taskID, err)
			h.markTaskAsFailed(fmt.Sprintf("failed to requeue: %v", err))
			return
		}
	}
}

// waitForAssignment waits for the task to be assigned to a worker
func (h *Handle) waitForAssignment(ctx context.Context, timeout time.Duration) (*TaskAssignment, error) {
	assignmentKey := fmt.Sprintf("task_assignment:%s", h.taskID)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-h.cancelCh:
			return nil, fmt.Errorf("task cancelled")
		default:
		}

		assignmentJSON, err := h.conn.Cache.Get(ctx, assignmentKey).Result()
		if err == nil {
			var assignment TaskAssignment
			if err := json.Unmarshal([]byte(assignmentJSON), &assignment); err == nil {
				return &assignment, nil
			}
		}

		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("timeout waiting for task assignment")
}

// monitorWorkerHealth monitors the assigned worker's health via the unified messaging system
func (h *Handle) monitorWorkerHealth(ctx context.Context, assignment *TaskAssignment, timeout time.Duration, statusID string, heartbeatInterval int) bool {
	checkInterval := time.Duration(heartbeatInterval) * time.Second
	heartbeatTimeout := time.Duration(heartbeatInterval*3) * time.Second // 3 missed heartbeats = failure

	// Parse start time
	startTime, err := time.Parse(time.RFC3339, assignment.StartedAt)
	if err != nil {
		log.Printf("⚠️ Invalid start time for task %s: %v", h.taskID, err)
		return false
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	lastHeartbeat := time.Now() // Initialize to now
	taskStarted := false

	for {
		select {
		case <-ctx.Done():
			return false
		case <-h.cancelCh:
			return true // Cancelled by user
		case <-h.doneCh:
			return true // Task completed

		case update := <-h.updatesCh:
			// Monitor updates from the subscription
			if update.Status == "running" {
				taskStarted = true
				lastHeartbeat = time.Now()
				log.Printf("✅ Task %s started, heartbeat monitoring active", h.taskID)
			}
			// Update heartbeat time for any update (including heartbeats)
			lastHeartbeat = time.Now()

			// Check if task completed
			if update.Status == "completed" || update.Status == "error" || update.Status == "cancelled" {
				return true
			}

		case <-ticker.C:
			now := time.Now()

			// Check if task has been running too long
			if now.Sub(startTime) > timeout {
				log.Printf("⏰ Task %s timed out after %v", h.taskID, timeout)
				return false
			}

			// Only check heartbeats after task has started
			if taskStarted {
				// Check if we've missed heartbeats
				if now.Sub(lastHeartbeat) > heartbeatTimeout {
					log.Printf("💀 Task %s missed heartbeats - last heartbeat %v ago", h.taskID, now.Sub(lastHeartbeat))
					return false
				}
			} else {
				// Check if task assignment still exists (task may have started but we missed the update)
				assignmentKey := fmt.Sprintf("task_assignment:%s", h.taskID)
				exists, err := h.conn.Cache.Exists(ctx, assignmentKey).Result()
				if err != nil || exists == 0 {
					// Assignment removed, task likely completed
					return true
				}

				// If task hasn't started after reasonable time, consider it failed
				if now.Sub(startTime) > 2*time.Minute {
					log.Printf("⚠️ Task %s never started after 2 minutes", h.taskID)
					return false
				}
			}
		}
	}
}

// isWorkerAlive checks if a worker is still sending heartbeats
func (h *Handle) isWorkerAlive(ctx context.Context, workerID string) bool {
	heartbeatKey := fmt.Sprintf("worker_heartbeat:%s", workerID)
	heartbeatJSON, err := h.conn.Cache.Get(ctx, heartbeatKey).Result()
	if err != nil {
		return false
	}

	var heartbeat WorkerHeartbeat
	if err := json.Unmarshal([]byte(heartbeatJSON), &heartbeat); err != nil {
		return false
	}

	// Parse heartbeat timestamp
	var heartbeatTime time.Time
	heartbeatTime, err = time.Parse(time.RFC3339, heartbeat.Timestamp)
	if err != nil {
		// Try alternative formats
		heartbeatTime, err = time.Parse("2006-01-02T15:04:05.000000", heartbeat.Timestamp)
		if err != nil {
			heartbeatTime, err = time.Parse("2006-01-02T15:04:05", heartbeat.Timestamp[:19])
			if err != nil {
				return false
			}
		}
	}

	// Check if heartbeat is recent (within 15 seconds)
	return time.Since(heartbeatTime) <= 15*time.Second
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

	// Clear task assignment
	assignmentKey := fmt.Sprintf("task_assignment:%s", h.taskID)
	h.conn.Cache.Del(ctx, assignmentKey)

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

	log.Printf("❌ Task %s marked as failed: %s", h.taskID, reason)
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

	return AwaitTypedResult[BacktestResult](ctx, handle)
}

// QueueScreening queues a screening task with default settings
func QueueScreening(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*Handle, error) {
	return QueueTask(ctx, conn, "screening", args, false, 3, 5*time.Minute)
}

// QueueScreeningTyped queues a screening task and returns a typed result
func QueueScreeningTyped(ctx context.Context, conn *data.Conn, args map[string]interface{}) (*ScreeningResult, error) {
	handle, err := QueueTask(ctx, conn, "screening", args, false, 3, 5*time.Minute)
	if err != nil {
		return nil, err
	}

	return AwaitTypedResult[ScreeningResult](ctx, handle)
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

	return AwaitTypedResult[AlertResult](ctx, handle)
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

	return AwaitTypedResult[CreateStrategyResult](ctx, handle)
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

	return AwaitTypedResult[PythonAgentResult](ctx, handle)
}
