package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
)

type GetActiveArgs struct {
	Timeframe string `json:"timeframe"`
	Group     string `json:"group"`
	Metric    string `json:"metric"`
}

type StockResult struct {
	Ticker     string `json:"ticker"`
	SecurityId int    `json:"securityId"`
}

type GroupConstituent struct {
	Ticker     string `json:"ticker"`
	SecurityId int    `json:"securityId"`
}

type GroupResult struct {
	Group        string             `json:"group"`
	Constituents []GroupConstituent `json:"constituents"`
}

// ActiveResult is a union type that can be either StockResult or GroupResult
type ActiveResult struct {
	Ticker       string             `json:"ticker,omitempty"`
	SecurityId   int                `json:"securityId,omitempty"`
	Group        string             `json:"group,omitempty"`
	Constituents []GroupConstituent `json:"constituents,omitempty"`
}

func GetActive(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetActiveArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("error parsing args: %w", err)
	}

	cacheKey := fmt.Sprintf("active:%s:%s:%s", args.Timeframe, args.Group, args.Metric)

	// Try to get from cache
	cached, err := conn.Cache.Get(context.Background(), cacheKey).Result()
	if err != nil {
		return []ActiveResult{}, nil // Return empty array if not found
	}

	var results []ActiveResult
	err = json.Unmarshal([]byte(cached), &results)
	if err != nil {
		return []ActiveResult{}, nil // Return empty array if unmarshal fails
	}

	return results, nil
}
