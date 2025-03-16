package utils

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// QueueArgs represents the arguments required to enqueue a task
type QueueArgs struct {
	ID   string      `json:"id"`
	Func string      `json:"func"`
	Args interface{} `json:"args"`
}

// queueResponse represents the response from a queue operation
// nolint:unused
//
//lint:ignore U1000 kept for future queue response handling

type queueResponse struct {
	TaskID string `json:"taskId"`
}

// Queue adds a function to the Redis processing queue and returns the task ID
func Queue(conn *Conn, funcName string, arguments interface{}) (string, error) {
	// Create a new task
	taskID := uuid.New().String()
	task := NewTask(funcName, arguments)
	task.ID = taskID

	// Create queue arguments
	queueArgs := QueueArgs{
		ID:   taskID,
		Func: funcName,
		Args: arguments,
	}

	// Serialize and add to queue
	serializedArgs, err := json.Marshal(queueArgs)
	if err != nil {
		return "", fmt.Errorf("error serializing task arguments: %w", err)
	}

	if err := conn.Cache.LPush(context.Background(), "queue", serializedArgs).Err(); err != nil {
		return "", fmt.Errorf("error adding task to queue: %w", err)
	}

	// Store task in Redis
	if err := SaveTask(conn, task); err != nil {
		return "", fmt.Errorf("error storing task status: %w", err)
	}

	return taskID, nil
}

// Poll retrieves the current status of a task
func Poll(conn *Conn, taskID string) (json.RawMessage, error) {
	task, err := GetTask(conn, taskID)
	if err != nil {
		return nil, err
	}

	// Convert task to JSON
	serialized, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("error serializing task: %w", err)
	}

	return serialized, nil
}

// UpdateTaskStatus updates the status of a task in Redis
func UpdateTaskStatus(conn *Conn, taskID string, status string, err error) error {
	// Get task
	task, getErr := GetTask(conn, taskID)
	if getErr != nil {
		return fmt.Errorf("error getting task: %w", getErr)
	}

	// Update task status
	UpdateStatus(task, status)

	// Add error message if present
	if err != nil && status == TaskStateFailed {
		task.Error = err.Error()
	}

	// Save updated task
	if saveErr := SaveTask(conn, task); saveErr != nil {
		return fmt.Errorf("error saving updated task: %w", saveErr)
	}

	return nil
}

// AddTaskLog adds a log message to a task
func AddTaskLog(conn *Conn, taskID string, message string, level string) error {
	// Get task
	task, err := GetTask(conn, taskID)
	if err != nil {
		return fmt.Errorf("error getting task: %w", err)
	}

	// Add log entry
	AddLogEntry(task, message, level)

	// Save updated task
	if saveErr := SaveTask(conn, task); saveErr != nil {
		return fmt.Errorf("error saving updated task: %w", saveErr)
	}

	return nil
}

// CheckSampleQueue performs operations related to CheckSampleQueue functionality.
func CheckSampleQueue(conn *Conn, setupId int, addedSample bool) {
	if addedSample {
		// Update untrainedSamples and sampleSize if a new sample is added
		_, err := conn.DB.Exec(context.Background(), `
            UPDATE setups 
            SET untrainedSamples = untrainedSamples + 1, 
                sampleSize = sampleSize + 1
            WHERE setupId = $1`, setupId)
		if err != nil {
			fmt.Printf("Error updating sample counts: %v\n", err)
			return
		}
	}
	checkModel(conn, setupId)

	var queueLength int
	err := conn.DB.QueryRow(context.Background(), `
        SELECT COUNT(*) 
        FROM samples 
        WHERE setupId = $1 AND label IS NULL`, setupId).Scan(&queueLength)
	if err != nil {
		fmt.Printf("Error checking queue length: %v\n", err)
		return
	}
	if queueLength < 30 {
		queueRunningKey := fmt.Sprintf("%d_queue_running", setupId)
		queueRunning := conn.Cache.Get(context.Background(), queueRunningKey).Val()
		if queueRunning != "true" {
			conn.Cache.Set(context.Background(), queueRunningKey, "true", 0)
			_, err := Queue(conn, "refillTrainerQueue", map[string]interface{}{
				"setupId": setupId,
			})

			if err != nil {
				fmt.Printf("Error enqueuing refillQueue: %v\n", err)
				conn.Cache.Del(context.Background(), queueRunningKey)
				return
			}
			fmt.Printf("Enqueued refillQueue for setupId: %d\n", setupId)
		}
	}
}

func checkModel(conn *Conn, setupId int) {
	var untrainedSamples int
	var sampleSize int

	// Retrieve untrainedSamples and sampleSize from the database
	err := conn.DB.QueryRow(context.Background(), `
        SELECT untrainedSamples, sampleSize
        FROM setups
        WHERE setupId = $1`, setupId).Scan(&untrainedSamples, &sampleSize)
	if err != nil {
		fmt.Printf("Error retrieving model info: %v\n", err)
		return
	}

	// Check if untrained samples exceed threshold
	if untrainedSamples > 0 || float64(untrainedSamples)/float64(sampleSize) > 0.05 {
		trainRunningKey := fmt.Sprintf("%d_train_running", setupId)
		trainRunning := conn.Cache.Get(context.Background(), trainRunningKey).Val()

		// Add "train" to the queue if not already running
		if trainRunning != "true" {
			conn.Cache.Set(context.Background(), trainRunningKey, "true", 0)
			_, err := Queue(conn, "train", map[string]interface{}{
				"setupId": setupId,
			})
			if err != nil {
				fmt.Printf("Error enqueuing train task: %v\n", err)
				conn.Cache.Del(context.Background(), trainRunningKey)
				return
			}
			fmt.Printf("Enqueued train task for setupId: %d\n", setupId)
		}
	}
}
