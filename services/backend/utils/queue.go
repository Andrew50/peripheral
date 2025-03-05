package utils

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)
// Poll performs operations related to Poll functionality.
func Poll(conn *Conn, taskId string) (json.RawMessage, error) {
	task := conn.Cache.Get(context.Background(), taskId).Val()
	if task == "" {
		return nil, fmt.Errorf("weh3")
	}
	result := json.RawMessage([]byte(task)) //its already json and you dont care about its contents utnil frontend so just push the json
	return result, nil
}
// QueueArgs represents a structure for handling QueueArgs data.
type QueueArgs struct {
	ID   string      `json:"id"`
	Func string      `json:"func"`
	Args interface{} `json:"args"`
}

type queueResponse struct {
	TaskID string `json:"taskId"`
}
// Queue performs operations related to Queue functionality.
func Queue(conn *Conn, funcName string, arguments interface{}) (string, error) {
	id := uuid.New().String()
	taskArgs := QueueArgs{
		ID:   id,
		Func: funcName,
		Args: arguments,
	}
	serializedTask, err := json.Marshal(taskArgs)
	if err != nil {
		return "", err
	}

	if err := conn.Cache.LPush(context.Background(), "queue", serializedTask).Err(); err != nil {
		return "", err
	}
	serializedStatus, err := json.Marshal("queued")
	if err != nil {
		return "", fmt.Errorf("error marshaling task status: %w", err)
	}
	if err := conn.Cache.Set(context.Background(), id, serializedStatus, 0).Err(); err != nil {
		return "", fmt.Errorf("error setting task status: %w", err)
	}
	return id, nil
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

	// Check if untrained samples exceed 20 or a certain percentage of sampleSize
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
