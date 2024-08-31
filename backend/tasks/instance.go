package tasks

import (
	"api/data"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type GetRelatedTickersArgs struct {
	Ticker string `json:"ticker"`
}

func GetRelatedTickers(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetRelatedTickersArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	tickers, err := data.GetPolygonRelatedTickers(conn.Polygon, args.Ticker)
	return tickers, err
}

type GetCikArgs struct {
	TickerString string `json:"ticker"`
}
type GetCikResults struct {
	Cik string `json:"cik"`
}

func GetCik(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetCikArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	cik, cikErr := data.GetCIK(conn, args.TickerString, "")
	if cikErr != nil {
		return nil, cikErr
	}
	res := GetCikResults{Cik: cik}
	return res, err
}

type Security struct {
	Ticker string `json:"ticker"`
	Cik    string `json:"cik"`
}

type Instance struct {
	InstanceId int       `json:"instanceId"`
	Security   Security  `json:"security"`
	Timestamp  time.Time `json:"timestamp"`
}

func GetInstances(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), "SELECT instanceId, cik, timestamp FROM instances WHERE userId = $1", userId)
	if err != nil {
		return nil, fmt.Errorf("358dg: %v", err)
	}
	var instances []Instance
	for rows.Next() {
		var instance Instance
		if err := rows.Scan(&instance.InstanceId, &instance.Security.Cik, &instance.Timestamp); err != nil {
			return nil, fmt.Errorf("dfwb3: %v", err)
		}
		instance.Security.Ticker, err = data.GetTickerFromCIK(conn.Polygon, instance.Security.Cik)
		if err != nil {
			return nil, fmt.Errorf("245jd: %v", err)
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

type NewInstanceArgs struct {
	Cik       string `json:"cik"`
	Timestamp string `json:"timestamp"`
}
type NewInstanceResults struct {
	InstanceID int `json:"instanceId"`
}

func NewInstance(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewInstanceArgs
	fmt.Print("NewInstance hit")
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("NewInstance invalid args: %v", err)
	}
	_, err := conn.DB.Exec(context.Background(), "insert into instances (userId, cik, timestamp) values ($1, $2, $3)", userId, args.Cik, args.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("NewInstance execution failed: %v", err)
	}
	var instanceID int
	err = conn.DB.QueryRow(context.Background(), "SELECT instanceId FROM instances WHERE userId = $1 AND cik = $2 and timestamp = $3", userId, args.Cik, args.Timestamp).Scan(&instanceID)
	if err != nil {
		return nil, fmt.Errorf("NewInstance execution failed: %v", err)
	}
	return NewInstanceResults{InstanceID: instanceID}, err
}
