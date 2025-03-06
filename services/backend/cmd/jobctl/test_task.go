package main

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// TestTask creates a test task and simulates task monitoring
func TestTask() {
	// Create a connection
	inContainer := os.Getenv("IN_CONTAINER") == "true"
	conn, cleanup := utils.InitConn(inContainer)
	defer cleanup()

	fmt.Println("Running test task simulation...")

	// Create a unique task ID
	taskID := fmt.Sprintf("test-task-%s", time.Now().Format("20060102-150405"))

	// Create a test task
	task := utils.Task{
		ID:        taskID,
		Status:    utils.TaskStateQueued,
		Function:  "test_function",
		Args:      map[string]interface{}{"test": "value"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save the task to Redis
	taskJSON, _ := json.Marshal(task)
	conn.Cache.Set(context.Background(), taskID, taskJSON, 0)

	fmt.Printf("Created test task with ID: %s\n\n", taskID)

	// Print task details
	printTaskDetails(&task)

	// Print monitoring header
	fmt.Println("\n=== TASK MONITORING ===")
	fmt.Printf("Started at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Monitoring task %s\n", taskID)
	fmt.Println("-------------------------")

	// Simulate task starting
	time.Sleep(2 * time.Second)
	task.Status = utils.TaskStateRunning
	task.UpdatedAt = time.Now()
	task.Logs = append(task.Logs, utils.LogEntry{
		Timestamp: time.Now(),
		Message:   "Task started running",
		Level:     "info",
	})
	taskJSON, _ = json.Marshal(task)
	conn.Cache.Set(context.Background(), taskID, taskJSON, 0)
	fmt.Printf("[%s] Task %s: %s (Function: %s)\n",
		time.Now().Format("15:04:05"),
		taskID,
		task.Status,
		task.Function)
	fmt.Printf("[%s] Task %s | info: Task started running\n",
		time.Now().Format("15:04:05"),
		taskID)

	// Simulate task progress
	time.Sleep(2 * time.Second)
	task.Logs = append(task.Logs, utils.LogEntry{
		Timestamp: time.Now(),
		Message:   "Processing data...",
		Level:     "info",
	})
	taskJSON, _ = json.Marshal(task)
	conn.Cache.Set(context.Background(), taskID, taskJSON, 0)
	fmt.Printf("[%s] Task %s | info: Processing data...\n",
		time.Now().Format("15:04:05"),
		taskID)

	// Simulate more task progress
	time.Sleep(2 * time.Second)
	task.Logs = append(task.Logs, utils.LogEntry{
		Timestamp: time.Now(),
		Message:   "Data processed successfully",
		Level:     "info",
	})
	taskJSON, _ = json.Marshal(task)
	conn.Cache.Set(context.Background(), taskID, taskJSON, 0)
	fmt.Printf("[%s] Task %s | info: Data processed successfully\n",
		time.Now().Format("15:04:05"),
		taskID)

	// Simulate task completion
	time.Sleep(2 * time.Second)
	task.Status = utils.TaskStateCompleted
	task.UpdatedAt = time.Now()
	resultJSON, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data":   "test result",
	})
	task.Result = resultJSON
	task.Logs = append(task.Logs, utils.LogEntry{
		Timestamp: time.Now(),
		Message:   "Task completed successfully",
		Level:     "info",
	})
	taskJSON, _ = json.Marshal(task)
	conn.Cache.Set(context.Background(), taskID, taskJSON, 0)
	fmt.Printf("[%s] Task %s: %s\n",
		time.Now().Format("15:04:05"),
		taskID,
		task.Status)
	fmt.Printf("[%s] Task %s | info: Task completed successfully\n",
		time.Now().Format("15:04:05"),
		taskID)

	// Print completion message
	fmt.Printf("\n=== MONITORING COMPLETE ===\n")
	fmt.Printf("Duration: %v\n", time.Since(task.CreatedAt).Round(time.Millisecond))
	fmt.Printf("Task completed successfully.\n")

	// Print result
	fmt.Printf("\nResult:\n")
	var prettyResult interface{}
	if err := json.Unmarshal(task.Result, &prettyResult); err == nil {
		resultJSON, err := json.MarshalIndent(prettyResult, "", "  ")
		if err == nil {
			fmt.Printf("%s\n", string(resultJSON))
		}
	}
}

// printTaskDetails prints the details of a task

func printTaskDetails(task *utils.Task) {

	fmt.Printf("\n=== TASK DETAILS ===\n")
	fmt.Printf("ID:        %s\n", task.ID)
	fmt.Printf("Function:  %s\n", task.Function)
	fmt.Printf("Status:    %s\n", task.Status)
	fmt.Printf("Created:   %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:   %s\n", task.UpdatedAt.Format("2006-01-02 15:04:05"))

	// Print arguments if available
	if task.Args != nil {
		argsJSON, err := json.MarshalIndent(task.Args, "", "  ")
		if err == nil {
			fmt.Printf("Arguments: %s\n", string(argsJSON))
		}
	}
}
