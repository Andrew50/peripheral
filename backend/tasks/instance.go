package tasks

import (
	"api/data"
	"context"
	"encoding/json"
	"fmt"
)

type GetCikArgs struct {
	TickerString string `json:"ticker"`
}

func GetCik(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetCikArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	cik := data.GetCIK(conn.Polygon, args.TickerString)
	return cik, err

}

type NewInstanceArgs struct {
	Cik       string `json:"cik"`
	Timestamp string `json:"timestamp"`
}

func NewInstance(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewInstanceArgs
	fmt.Print("NewInstance hit")
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("NewInstance invalid args: %v", err)
	}
	var instanceID int
	err := conn.DB.QueryRow(context.Background(), "insert into instances (user_id, cik, timestamp) values ($1, $2, $3) RETURNING instance_id", userId, args.Cik, args.Timestamp).Scan(&instanceID)
	if err != nil {
		return nil, fmt.Errorf("NewInstance execution failed: %v", err)
	}
	fmt.Print(instanceID)

	return instanceID, err
}
