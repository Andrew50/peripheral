// Package worker_monitor provides monitoring and recovery of worker tasks
// via Redis heartbeats and task status management.
package worker_monitor

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// WorkerHeartbeat represents a worker's heartbeat data
type WorkerHeartbeat struct {
	WorkerID      string                 `json:"worker_id"`
	Status        string                 `json:"status"`
	Timestamp     string                 `json:"timestamp"`
	UptimeSeconds float64                `json:"uptime_seconds"`
	ActiveTask    *string                `json:"active_task"`
	QueueStats    map[string]interface{} `json:"queue_stats"`
}

// TaskAssignment represents a task currently assigned to a worker (derived from heartbeats)
type TaskAssignment struct {
	WorkerID  string `json:"worker_id"`
	TaskID    string `json:"task_id"`
	StartedAt string `json:"started_at"`
	Status    string `json:"status"`
}

// TaskData represents the structure of a task in the queue
type TaskData struct {
	TaskID    string                 `json:"task_id"`
	TaskType  string                 `json:"task_type"`
	Args      map[string]interface{} `json:"args"`
	CreatedAt string                 `json:"created_at"`
	Priority  string                 `json:"priority"`
}

// WorkerMonitor provides worker health monitoring and task recovery
type WorkerMonitor struct {
	conn      *data.Conn
	isRunning bool
	stopChan  chan struct{}
	mu        sync.RWMutex

	// Configuration
	heartbeatTimeout time.Duration // How long before considering a worker dead
	taskTimeout      time.Duration // How long before considering a task stuck
	checkInterval    time.Duration // How often to check for issues
	maxRetries       int           // Maximum retries for a task

	// Statistics
	deadWorkersDetected int64 // Total dead workers detected
	tasksRecovered      int64 // Total tasks recovered
	stuckTasksRecovered int64 // Total stuck tasks recovered
	failedRecoveries    int64 // Total failed recovery attempts
}

// NewWorkerMonitor creates a new worker monitor instance
func NewWorkerMonitor(conn *data.Conn) *WorkerMonitor {
	return &WorkerMonitor{
		conn:             conn,
		stopChan:         make(chan struct{}),
		heartbeatTimeout: 10 * time.Second, // 10 seconds = 2 missed heartbeats at 5s interval
		taskTimeout:      5 * time.Minute,  // 5 minutes = stuck task (aggressive timeout)
		checkInterval:    5 * time.Second,  // Check every 5 seconds (ultra-responsive)
		maxRetries:       3,                // Maximum 3 retries per task
	}
}

// Start begins the worker monitoring service
func (wm *WorkerMonitor) Start() {
	wm.mu.Lock()
	if wm.isRunning {
		wm.mu.Unlock()
		log.Println("⚠️ Worker monitor already running")
		return
	}
	wm.isRunning = true
	wm.mu.Unlock()

	// Start monitoring goroutine
	go wm.monitorLoop()

	log.Println("✅ Worker monitor service started")
}

// Stop gracefully shuts down the worker monitor
func (wm *WorkerMonitor) Stop() {
	wm.mu.Lock()
	if !wm.isRunning {
		wm.mu.Unlock()
		return
	}
	wm.isRunning = false
	wm.mu.Unlock()

	log.Println("🛑 Stopping worker monitor service...")
	close(wm.stopChan)
	log.Println("✅ Worker monitor service stopped")
}

// monitorLoop is the main monitoring loop
func (wm *WorkerMonitor) monitorLoop() {
	ticker := time.NewTicker(wm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wm.performHealthCheck()
		case <-wm.stopChan:
			log.Println("🏁 Worker monitor loop exiting")
			return
		}
	}
}

// performHealthCheck checks worker health and recovers failed tasks
func (wm *WorkerMonitor) performHealthCheck() {
	ctx := context.Background()

	// Get all worker heartbeats
	activeWorkers, err := wm.getActiveWorkers(ctx)
	if err != nil {
		log.Printf("❌ Error getting active workers: %v", err)
		return
	}

	// Get all task assignments (derived from active workers)
	taskAssignments := wm.getTaskAssignments(activeWorkers)

	// Check for dead workers and stuck tasks
	deadWorkers := wm.findDeadWorkers(activeWorkers)
	stuckTasks := wm.findStuckTasks(taskAssignments, activeWorkers)

	// Log monitoring status - only log details when there are issues or every 10 checks
	if len(deadWorkers) > 0 || len(stuckTasks) > 0 {
		log.Printf("🚨 Worker Monitor ALERT: %d active workers, %d assignments, %d DEAD workers, %d STUCK tasks",
			len(activeWorkers), len(taskAssignments), len(deadWorkers), len(stuckTasks))
	} else {
		// Only log every 60 checks (5 minutes) when everything is healthy
		if time.Now().Unix()%300 < 5 { // Every 5 minutes (300 seconds), within 5 second window
			log.Printf("✅ Worker Monitor: %d active workers, %d active tasks - all healthy",
				len(activeWorkers), len(taskAssignments))
		}
	}

	// Recover tasks from dead workers
	for _, workerID := range deadWorkers {
		wm.recoverTasksFromDeadWorker(ctx, workerID, taskAssignments)
	}

	// Recover stuck tasks
	for _, assignment := range stuckTasks {
		wm.recoverStuckTask(ctx, assignment)
	}
}

// getActiveWorkers retrieves all worker heartbeats from Redis
func (wm *WorkerMonitor) getActiveWorkers(ctx context.Context) (map[string]WorkerHeartbeat, error) {
	// Get all worker heartbeat keys
	keys, err := wm.conn.Cache.Keys(ctx, "worker_heartbeat:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get worker heartbeat keys: %w", err)
	}

	activeWorkers := make(map[string]WorkerHeartbeat)

	for _, key := range keys {
		// Extract worker ID from key
		workerID := strings.TrimPrefix(key, "worker_heartbeat:")

		// Get heartbeat data
		heartbeatJSON, err := wm.conn.Cache.Get(ctx, key).Result()
		if err != nil {
			log.Printf("⚠️ Failed to get heartbeat for worker %s: %v", workerID, err)
			continue
		}

		var heartbeat WorkerHeartbeat
		if err := json.Unmarshal([]byte(heartbeatJSON), &heartbeat); err != nil {
			log.Printf("⚠️ Failed to parse heartbeat for worker %s: %v", workerID, err)
			continue
		}

		activeWorkers[workerID] = heartbeat
	}

	return activeWorkers, nil
}

// getTaskAssignments derives current task assignments from worker heartbeats
func (wm *WorkerMonitor) getTaskAssignments(activeWorkers map[string]WorkerHeartbeat) map[string]TaskAssignment {
	assignments := make(map[string]TaskAssignment)

	for workerID, heartbeat := range activeWorkers {
		// Skip workers that don't have an active task
		if heartbeat.ActiveTask == nil || *heartbeat.ActiveTask == "" {
			continue
		}

		taskID := *heartbeat.ActiveTask

		// Parse heartbeat timestamp to use as started time
		var startedAt string
		if heartbeatTime, err := time.Parse(time.RFC3339, heartbeat.Timestamp); err == nil {
			startedAt = heartbeatTime.Format(time.RFC3339)
		} else {
			// Fallback to current time if parsing fails
			startedAt = time.Now().Format(time.RFC3339)
		}

		assignment := TaskAssignment{
			WorkerID:  workerID,
			TaskID:    taskID,
			StartedAt: startedAt,
			Status:    "running", // All active tasks are considered running
		}

		assignments[taskID] = assignment
	}

	return assignments
}

// findDeadWorkers identifies workers that haven't sent heartbeats recently
func (wm *WorkerMonitor) findDeadWorkers(activeWorkers map[string]WorkerHeartbeat) []string {
	var deadWorkers []string
	now := time.Now()

	for workerID, heartbeat := range activeWorkers {
		// Parse heartbeat timestamp - try multiple formats
		var heartbeatTime time.Time
		var err error

		// Try RFC3339 first (standard format)
		heartbeatTime, err = time.Parse(time.RFC3339, heartbeat.Timestamp)
		if err != nil {
			// Try Python datetime format without timezone
			heartbeatTime, err = time.Parse("2006-01-02T15:04:05.000000", heartbeat.Timestamp)
			if err != nil {
				// Try Python datetime format with microseconds
				heartbeatTime, err = time.Parse("2006-01-02T15:04:05", heartbeat.Timestamp[:19])
				if err != nil {
					log.Printf("⚠️ Invalid timestamp for worker %s (%s): %v", workerID, heartbeat.Timestamp, err)
					continue
				}
			}
		}

		// Check if heartbeat is too old
		timeSinceHeartbeat := now.Sub(heartbeatTime)

		if timeSinceHeartbeat > wm.heartbeatTimeout {
			deadWorkers = append(deadWorkers, workerID)
			log.Printf("💀 DEAD WORKER DETECTED: %s (silent for %v, last heartbeat: %s) - IMMEDIATE TASK RECOVERY",
				workerID, timeSinceHeartbeat.Round(time.Second), heartbeat.Timestamp)
		}
	}

	return deadWorkers
}

// findStuckTasks identifies tasks that have been running too long
func (wm *WorkerMonitor) findStuckTasks(assignments map[string]TaskAssignment, activeWorkers map[string]WorkerHeartbeat) []TaskAssignment {
	var stuckTasks []TaskAssignment
	now := time.Now()

	for taskID, assignment := range assignments {
		// Parse task start time - try multiple formats
		var startTime time.Time
		var err error

		// Try RFC3339 first (standard format)
		startTime, err = time.Parse(time.RFC3339, assignment.StartedAt)
		if err != nil {
			// Try Python datetime format without timezone
			startTime, err = time.Parse("2006-01-02T15:04:05.000000", assignment.StartedAt)
			if err != nil {
				// Try Python datetime format with microseconds
				startTime, err = time.Parse("2006-01-02T15:04:05", assignment.StartedAt[:19])
				if err != nil {
					log.Printf("⚠️ Invalid start time for task %s (%s): %v", taskID, assignment.StartedAt, err)
					continue
				}
			}
		}

		// Check if task has been running too long
		if now.Sub(startTime) > wm.taskTimeout {
			// Check if the worker is still alive
			if _, exists := activeWorkers[assignment.WorkerID]; !exists {
				// Worker is dead, this task is definitely stuck
				stuckTasks = append(stuckTasks, assignment)
				log.Printf("🚫 Stuck task detected: %s (worker %s dead, running for %v)",
					taskID, assignment.WorkerID, now.Sub(startTime))
			} else {
				// Worker is alive but task is taking too long
				stuckTasks = append(stuckTasks, assignment)
				log.Printf("⏰ Long-running task detected: %s (worker %s alive, running for %v)",
					taskID, assignment.WorkerID, now.Sub(startTime))
			}
		}
	}

	return stuckTasks
}

// recoverTasksFromDeadWorker requeues all tasks from a dead worker
func (wm *WorkerMonitor) recoverTasksFromDeadWorker(ctx context.Context, workerID string, assignments map[string]TaskAssignment) {
	tasksToRecover := []string{}

	// Find all tasks assigned to this dead worker
	for taskID, assignment := range assignments {
		if assignment.WorkerID == workerID {
			tasksToRecover = append(tasksToRecover, taskID)
		}
	}

	// Always clean up the dead worker's heartbeat, even if no tasks to recover
	heartbeatKey := fmt.Sprintf("worker_heartbeat:%s", workerID)
	wm.conn.Cache.Del(ctx, heartbeatKey)
	log.Printf("🧹 Cleaned up heartbeat for dead worker %s", workerID)

	if len(tasksToRecover) == 0 {
		log.Printf("🔍 Dead worker %s had no assigned tasks - heartbeat cleaned up", workerID)
		return
	}

	// Update statistics
	wm.deadWorkersDetected++

	log.Printf("🚨 IMMEDIATE RECOVERY: %d tasks from dead worker %s", len(tasksToRecover), workerID)

	// Process all task recoveries immediately
	successCount := 0
	for _, taskID := range tasksToRecover {
		log.Printf("🔄 Recovering task %s from dead worker %s...", taskID, workerID)
		if err := wm.requeueTask(ctx, taskID, fmt.Sprintf("Worker %s died (heartbeat timeout)", workerID)); err != nil {
			log.Printf("❌ CRITICAL: Failed to requeue task %s from dead worker %s: %v", taskID, workerID, err)
			wm.failedRecoveries++
		} else {
			log.Printf("✅ SUCCESS: Task %s requeued from dead worker %s", taskID, workerID)
			successCount++
			wm.tasksRecovered++
		}
	}

	log.Printf("📊 Recovery summary for worker %s: %d/%d tasks successfully recovered",
		workerID, successCount, len(tasksToRecover))
}

// recoverStuckTask handles recovery of a stuck task
func (wm *WorkerMonitor) recoverStuckTask(ctx context.Context, assignment TaskAssignment) {
	startTime, _ := time.Parse(time.RFC3339, assignment.StartedAt)
	runtime := time.Since(startTime).Round(time.Second)

	log.Printf("⏰ STUCK TASK RECOVERY: %s from worker %s (running %v)",
		assignment.TaskID, assignment.WorkerID, runtime)

	reason := fmt.Sprintf("Task timeout after %v (limit: %v)", runtime, wm.taskTimeout)
	if err := wm.requeueTask(ctx, assignment.TaskID, reason); err != nil {
		log.Printf("❌ CRITICAL: Failed to requeue stuck task %s: %v", assignment.TaskID, err)
		wm.failedRecoveries++
	} else {
		log.Printf("✅ SUCCESS: Stuck task %s requeued after %v", assignment.TaskID, runtime)
		wm.stuckTasksRecovered++
		wm.tasksRecovered++
	}
}

// requeueTask moves a failed task back to the appropriate queue
func (wm *WorkerMonitor) requeueTask(ctx context.Context, taskID string, reason string) error {
	// Get the original task result to determine task type and priority
	resultKey := fmt.Sprintf("task_result:%s", taskID)
	resultJSON, err := wm.conn.Cache.Get(ctx, resultKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get task result for %s: %w", taskID, err)
	}

	var taskResult map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &taskResult); err != nil {
		return fmt.Errorf("failed to parse task result for %s: %w", taskID, err)
	}

	// Extract task data from the result
	data, ok := taskResult["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid task result format for %s", taskID)
	}

	// Check retry count
	retryCount := 0
	if count, exists := data["retry_count"]; exists {
		if countFloat, ok := count.(float64); ok {
			retryCount = int(countFloat)
		}
	}

	if retryCount >= wm.maxRetries {
		// Mark task as permanently failed
		log.Printf("❌ Task %s exceeded max retries (%d), marking as failed", taskID, wm.maxRetries)
		return wm.markTaskAsFailed(ctx, taskID, fmt.Sprintf("Max retries exceeded. Last failure: %s", reason))
	}

	// Increment retry count
	retryCount++

	// Try to reconstruct the original task data
	// This is a simplified approach - in production you might want to store the original task data separately
	taskData := TaskData{
		TaskID:   taskID,
		TaskType: "backtest", // Default, should be extracted from original task
		Args: map[string]interface{}{
			"retry_count":  retryCount,
			"retry_reason": reason,
		},
		CreatedAt: time.Now().Format(time.RFC3339),
		Priority:  "normal",
	}

	// Extract original task data if available
	if originalTask, exists := data["original_task"]; exists {
		if taskMap, ok := originalTask.(map[string]interface{}); ok {
			// Extract task type
			if taskType, ok := taskMap["task_type"].(string); ok {
				taskData.TaskType = taskType
			}

			// Extract priority
			if priority, ok := taskMap["priority"].(string); ok {
				taskData.Priority = priority
			}

			// Extract original args and merge with retry info
			if originalArgs, ok := taskMap["args"].(map[string]interface{}); ok {
				for k, v := range originalArgs {
					taskData.Args[k] = v
				}
				// Override retry info
				taskData.Args["retry_count"] = retryCount
				taskData.Args["retry_reason"] = reason
			}
		}
	}

	// Determine queue based on task type or priority - preserve original queue
	queueName := "strategy_queue" // Default to normal queue
	if taskData.TaskType == "create_strategy" || taskData.Priority == "high" {
		queueName = "strategy_queue_priority"
	}

	// Log the requeue decision
	log.Printf("📋 Requeuing task %s (type: %s, priority: %s, retry: %d/%d) to queue: %s",
		taskID, taskData.TaskType, taskData.Priority, retryCount, wm.maxRetries, queueName)

	// Marshal task data
	taskJSON, err := json.Marshal(taskData)
	if err != nil {
		return fmt.Errorf("failed to marshal task data for %s: %w", taskID, err)
	}

	// Add back to queue
	err = wm.conn.Cache.LPush(ctx, queueName, string(taskJSON)).Err()
	if err != nil {
		return fmt.Errorf("failed to push task %s to queue %s: %w", taskID, queueName, err)
	}

	// Update task status for retry (no need to clear assignment keys anymore)

	// Update task status to queued for retry
	if err := wm.updateTaskStatus(ctx, taskID, "queued", map[string]interface{}{
		"retry_count":  retryCount,
		"retry_reason": reason,
		"requeued_at":  time.Now().Format(time.RFC3339),
	}); err != nil {
		log.Printf("Warning: failed to update task status for retry: %v", err)
	}

	return nil
}

// markTaskAsFailed marks a task as permanently failed
func (wm *WorkerMonitor) markTaskAsFailed(ctx context.Context, taskID string, reason string) error {
	// Update task status to failed
	return wm.updateTaskStatus(ctx, taskID, "failed", map[string]interface{}{
		"error":        reason,
		"failed_at":    time.Now().Format(time.RFC3339),
		"failure_type": "recovery_failure",
	})
}

// updateTaskStatus updates a task's status in Redis
func (wm *WorkerMonitor) updateTaskStatus(ctx context.Context, taskID string, status string, data map[string]interface{}) error {
	result := map[string]interface{}{
		"task_id":    taskID,
		"status":     status,
		"data":       data,
		"updated_at": time.Now().Format(time.RFC3339),
		"updated_by": "worker_monitor",
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal task result: %w", err)
	}

	resultKey := fmt.Sprintf("task_result:%s", taskID)
	err = wm.conn.Cache.SetEX(ctx, resultKey, string(resultJSON), 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Publish update
	updateMessage := map[string]interface{}{
		"task_id":    taskID,
		"status":     status,
		"result":     data,
		"updated_at": time.Now().Format(time.RFC3339),
		"updated_by": "worker_monitor",
	}

	updateJSON, _ := json.Marshal(updateMessage)
	wm.conn.Cache.Publish(ctx, "worker_task_updates", string(updateJSON))

	return nil
}

// GetMonitoringStats returns current monitoring statistics
func (wm *WorkerMonitor) GetMonitoringStats(ctx context.Context) (map[string]interface{}, error) {
	activeWorkers, err := wm.getActiveWorkers(ctx)
	if err != nil {
		return nil, err
	}

	taskAssignments := wm.getTaskAssignments(activeWorkers)

	deadWorkers := wm.findDeadWorkers(activeWorkers)
	stuckTasks := wm.findStuckTasks(taskAssignments, activeWorkers)

	stats := map[string]interface{}{
		"active_workers": len(activeWorkers),
		"active_tasks":   len(taskAssignments), // Derived from worker heartbeats
		"dead_workers":   len(deadWorkers),
		"stuck_tasks":    len(stuckTasks),
		"is_running":     wm.isRunning,
		"last_check":     time.Now().Format(time.RFC3339),
		"config": map[string]interface{}{
			"heartbeat_timeout_seconds": wm.heartbeatTimeout.Seconds(),
			"task_timeout_minutes":      wm.taskTimeout.Minutes(),
			"check_interval_seconds":    wm.checkInterval.Seconds(),
			"max_retries":               wm.maxRetries,
		},
		"recovery_stats": map[string]interface{}{
			"dead_workers_detected": wm.deadWorkersDetected,
			"tasks_recovered":       wm.tasksRecovered,
			"stuck_tasks_recovered": wm.stuckTasksRecovered,
			"failed_recoveries":     wm.failedRecoveries,
			"success_rate":          float64(wm.tasksRecovered) / float64(wm.tasksRecovered+wm.failedRecoveries+1) * 100, // +1 to avoid division by zero
		},
	}

	return stats, nil
}
