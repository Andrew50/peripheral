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
	_, err := conn.DB.Exec(context.Background(), "insert into instances (user_id, cik, timestamp) values ($1, $2, $3)", userId, args.Cik, args.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("NewInstance execution failed: %v", err)
	}
	var instanceID int
	qrErr := conn.DB.QueryRow(context.Background(), "SELECT instance_id FROM instances WHERE user_id = $1 AND cik = $2 and timestamp = $3", userId, args.Cik, args.Timestamp).Scan(&instanceID)
	if err != nil {
		return nil, fmt.Errorf("NewInstance execution failed: %v", qrErr)
	}
	fmt.Print(instanceID)

	return instanceID, err
}
