package tasks

import (
	"api/data"
	"context"
	"encoding/json"
	"fmt"
)

type GetCikArgs struct {
	Ticker string `json:"a1"`
}

func GetCik(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetCikArgs

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	cik, err := data.GetCIK(conn.Polygon, args.Ticker)
	if err != nil {
		return nil, err
	}
	return cik, err

}

type NewInstanceArgs struct {
	Cik       int    `json:"a1"`
	Timestamp string `json:"a2"`
}

func NewInstance(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewInstanceArgs

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("NewInstance invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "insert into instances (cik, timestamp, user_id) values ($1, $2, $3) ", args.Cik, args.Timestamp, userId)
	if err != nil {
		return nil, fmt.Errorf("NewInstance execution failed: %v", err)
	}

	return arg1, err
}
