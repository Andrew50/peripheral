package tasks

import (
	"api/data"
	"context"
	"encoding/json"
	"fmt"
)

type NewInstanceArgs struct {
	Ticker    string `json:"a1"`
	Timestamp string `json:"a2"`
}

func NewInstance(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewInstanceArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("NewInstance invalid args: %v", err)
	}
	_, err := conn.DB.Exec(context.Background(), "insert into instances (security_id, timestamp, user_id) values ($1, $2, $3) ", args.SecurityId, args.Timestamp, userId)
	if err != nil {
		return nil, fmt.Errorf("NewIstance execution failed: %v", err)
	}
	return nil, nil
}
