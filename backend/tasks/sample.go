
package tasks

import (
	"backend/utils"
    "time"
    "fmt"
	"context"
	"encoding/json"
)
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
    utils.CheckSampleQueue(conn,args.SetupId,args.Label)
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

	utils.CheckSampleQueue(conn, args.SetupId,false)
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
type SetSampleArgs struct {
	SecurityId int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"` // Unix timestamp in milliseconds
	SetupId    int    `json:"setupId"`
}

func SetSample(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetSampleArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	_, err := conn.DB.Exec(context.Background(), `
		INSERT INTO samples (securityId, timestamp, setupId,label)
		VALUES ($1, to_timestamp($2), $3,true)`,
		args.SecurityId, args.Timestamp/1000, args.SetupId) // Convert timestamp from milliseconds to seconds
	if err != nil {
		return nil, fmt.Errorf("error inserting sample: %v", err)
	}
    utils.CheckSampleQueue(conn,args.SetupId,true)

	return map[string]string{"status": "success"}, nil
}
