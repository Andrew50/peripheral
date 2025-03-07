package main

import (
	"backend/utils"
	"fmt"
	"time"
)

// TestJobFunction is a simple job function that simulates work
func TestJobFunction(conn *utils.Conn) error {
	fmt.Println("Starting test job...")

	// Simulate some work
	time.Sleep(2 * time.Second)
	fmt.Println("Test job step 1 completed")

	// Simulate more work
	time.Sleep(2 * time.Second)
	fmt.Println("Test job step 2 completed")

	// Create a task that will be completed by the worker
	taskID, err := utils.Queue(conn, "test_function", map[string]interface{}{
		"test": "value",
	})
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	fmt.Printf("Created task with ID: %s\n", taskID)

	// Simulate final work
	time.Sleep(1 * time.Second)
	fmt.Println("Test job completed successfully")

	return nil
}
