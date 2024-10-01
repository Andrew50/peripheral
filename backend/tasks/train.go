package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Argument structs
type LabelTrainingQueueInstanceArgs struct {
	SetupId  int `json:"setupId"`
	SampleId int `json:"sampleId"`
	Label    bool `json:"label"` // Include the label to be assigned (true/false)
}



func LabelTrainingQueueInstance(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args LabelTrainingQueueInstanceArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
    checkQueueLength(conn,args.SetupId)
	_, err := conn.DB.Exec(context.Background(), `
		UPDATE samples
		SET label = $1
		WHERE sampleId = $2 AND setupId = $3`,
		args.Label, args.SampleId, args.SetupId)
	if err != nil {
		return nil, fmt.Errorf("error labeling sample: %v", err)
	}
	return map[string]string{"status": "success"}, nil
}
type GetTrainingQueueArgs struct {
	SetupId int `json:"setupId"`
}
type GetTrainingQueueResult struct {
    SampleId   int    `json:"sampleId"`
    SecurityId int    `json:"securityId"`
    Timestamp  int64  `json:"timestamp"`
    Ticker     string `json:"ticker"`
}


func GetTrainingQueue(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTrainingQueueArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	checkQueueLength(conn, args.SetupId)
	rows, err := conn.DB.Query(context.Background(), `
		SELECT s.sampleId, s.securityId, s.timestamp, sec.ticker
		FROM samples s
		JOIN securities sec ON s.securityId = sec.securityId
		WHERE s.setupId = $1 
		  AND s.label IS NULL 
		  AND sec.minDate <= s.timestamp 
		  AND (sec.maxDate IS NULL OR sec.maxDate >= s.timestamp)`, args.SetupId)
	if err != nil {
		return nil, fmt.Errorf("error getting training queue: %v", err)
	}
	defer rows.Close()
	var trainingQueue []GetTrainingQueueResult
	for rows.Next() {
		var sampleId, securityId int
		var timestamp time.Time
		var ticker string
		if err := rows.Scan(&sampleId, &securityId, &timestamp, &ticker); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}
		trainingQueue = append(trainingQueue, GetTrainingQueueResult{
			SampleId:   sampleId,
			SecurityId: securityId,
			Timestamp:  timestamp.Unix(), // Convert to Unix timestamp in milliseconds
			Ticker:     ticker,                // Add ticker to the struct
		})
	}

	return trainingQueue, nil
}
func checkQueueLength(conn *utils.Conn, setupId int) {
    var queueLength int
    err := conn.DB.QueryRow(context.Background(), `
        SELECT COUNT(*) 
        FROM samples 
        WHERE setupId = $1 AND label IS NULL`, setupId).Scan(&queueLength)
    if err != nil {
        fmt.Printf("Error checking queue length: %v\n", err)
        return
    }
    if queueLength < 20 {
        queueRunningKey := fmt.Sprintf("%d_queue_running", setupId)
        queueRunning := conn.Cache.Get(context.Background(), queueRunningKey).Val()
        if queueRunning != "true" {
            conn.Cache.Set(context.Background(), queueRunningKey, "true", 0)
            _, err := utils.Queue(conn, "refillTrainerQueue", map[string]interface{}{
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

