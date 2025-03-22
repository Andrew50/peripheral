package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// LabelTrainingQueueInstanceArgs represents a structure for handling LabelTrainingQueueInstanceArgs data.
type LabelTrainingQueueInstanceArgs struct {
	SampleID int  `json:"sampleId"`
	Label    bool `json:"label"` // Include the label to be assigned (true/false)
}

// LabelTrainingQueueInstance performs operations related to LabelTrainingQueueInstance functionality.
func LabelTrainingQueueInstance(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args LabelTrainingQueueInstanceArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
	var setupID int
	err := conn.DB.QueryRow(context.Background(), `
		UPDATE samples
		SET label = $1
		WHERE sampleId = $2
		RETURNING setupId`,
		args.Label, args.SampleID).Scan(&setupID)
	if err != nil {
		return nil, fmt.Errorf("error updating and retrieving setupId: %v", err)
	}
	utils.CheckSampleQueue(conn, setupID, args.Label)
	return nil, nil
}

// GetTrainingQueueArgs represents a structure for handling GetTrainingQueueArgs data.
type GetTrainingQueueArgs struct {
	SetupID int `json:"setupId"`
}

// GetTrainingQueueResult represents a structure for handling GetTrainingQueueResult data.
type GetTrainingQueueResult struct {
	SampleID   int    `json:"sampleId"`
	SecurityID int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"`
	Ticker     string `json:"ticker"`
}

// GetTrainingQueue performs operations related to GetTrainingQueue functionality.
func GetTrainingQueue(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTrainingQueueArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	utils.CheckSampleQueue(conn, args.SetupID, false)
	rows, err := conn.DB.Query(context.Background(), `
		SELECT s.sampleId, s.securityId, s.timestamp, sec.ticker
		FROM samples s
		JOIN securities sec ON s.securityId = sec.securityId
		WHERE s.setupId = $1 
		  AND s.label IS NULL 
		  AND sec.minDate <= s.timestamp 
		  AND (sec.maxDate IS NULL OR sec.maxDate >= s.timestamp)`, args.SetupID)
	if err != nil {
		return nil, fmt.Errorf("error getting training queue: %v", err)
	}
	defer rows.Close()
	var trainingQueue []GetTrainingQueueResult
	for rows.Next() {
		var sampleID, securityID int
		var timestamp time.Time
		var ticker string
		if err := rows.Scan(&sampleID, &securityID, &timestamp, &ticker); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}
		trainingQueue = append(trainingQueue, GetTrainingQueueResult{
			SampleID:   sampleID,
			SecurityID: securityID,
			Timestamp:  timestamp.Unix(), // Convert to Unix timestamp in milliseconds
			Ticker:     ticker,           // Add ticker to the struct
		})
	}

	return trainingQueue, nil
}

// SetSampleArgs represents a structure for handling SetSampleArgs data.
type SetSampleArgs struct {
	SecurityID int   `json:"securityId"`
	Timestamp  int64 `json:"timestamp"` // Unix timestamp in milliseconds
	SetupID    int   `json:"setupId"`
}

// SetSample performs operations related to SetSample functionality.
func SetSample(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetSampleArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	_, err := conn.DB.Exec(context.Background(), `
		INSERT INTO samples (securityId, timestamp, setupId,label)
		VALUES ($1, to_timestamp($2), $3,true)`,
		args.SecurityID, args.Timestamp/1000, args.SetupID) // Convert timestamp from milliseconds to seconds
	if err != nil {
		return nil, fmt.Errorf("error inserting sample: %v", err)
	}
	utils.CheckSampleQueue(conn, args.SetupID, true)

	return map[string]string{"status": "success"}, nil
}
