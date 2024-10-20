package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
)

type SetupResult struct {
	SetupId   int     `json:"setupId"`
	Name      string  `json:"name"`
	Timeframe string  `json:"timeframe"`
	Bars      int     `json:"bars"`
	Threshold int     `json:"threshold"`
	Dolvol    float64 `json:"dolvol"`
	Adr       float64 `json:"adr"`
	Mcap      float64 `json:"mcap"`
	Score     int     `json:"score"`
}

func GetSetups(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
    SELECT setupId, name, timeframe, bars, threshold, dolvol, adr, mcap, score
    from setups where userId = $1`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var setupResults []SetupResult
	for rows.Next() {
		var setupResult SetupResult
		if err := rows.Scan(&setupResult.SetupId, &setupResult.Name, &setupResult.Timeframe, &setupResult.Bars, &setupResult.Threshold, &setupResult.Dolvol, &setupResult.Adr, &setupResult.Mcap, &setupResult.Score); err != nil {
			return nil, fmt.Errorf("sdifn0 %v", err)
		}
		setupResults = append(setupResults, setupResult)
	}
	return setupResults, nil
}

type NewSetupArgs struct {
	Name      string  `json:"name"`
	Timeframe string  `json:"timeframe"`
	Bars      int     `json:"bars"`
	Threshold int     `json:"threshold"`
	Dolvol    float64 `json:"dolvol"`
	Adr       float64 `json:"adr"`
	Mcap      float64 `json:"mcap"`
}

func NewSetup(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewSetupArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}
	if args.Name == "" || args.Timeframe == "" {
		return nil, fmt.Errorf("dlkns")
	}
	var setupId int
	err := conn.DB.QueryRow(context.Background(), `
		INSERT INTO setups (name, timeframe, bars, threshold, dolvol, adr, mcap, userId) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING setupId`,
		args.Name, args.Timeframe, args.Bars, args.Threshold, args.Dolvol, args.Adr, args.Mcap, userId,
	).Scan(&setupId)

	if err != nil {
		return nil, fmt.Errorf("dkngvw0 %v", err)
	}
	utils.CheckSampleQueue(conn, setupId, false)
	return SetupResult{
		SetupId:   setupId,
		Name:      args.Name,
		Timeframe: args.Timeframe,
		Bars:      args.Bars,
		Threshold: args.Threshold,
		Dolvol:    args.Dolvol,
		Adr:       args.Adr,
		Mcap:      args.Mcap,
	}, nil
}

type DeleteSetupArgs struct {
	SetupId int `json:"setupId"`
}

func DeleteSetup(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteSetupArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}
	_, err := conn.DB.Exec(context.Background(), `
		DELETE FROM setups 
		WHERE setupId = $1`, args.SetupId)

	if err != nil {
		return nil, fmt.Errorf("error deleting setup: %v", err)
	}
	return nil, nil
}

type SetSetupArgs struct {
	SetupId   int     `json:"setupId"`
	Name      string  `json:"name"`
	Timeframe string  `json:"timeframe"`
	Bars      int     `json:"bars"`
	Threshold int     `json:"threshold"`
	Dolvol    float64 `json:"dolvol"`
	Adr       float64 `json:"adr"`
	Mcap      float64 `json:"mcap"`
}

func SetSetup(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetSetupArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
	if args.SetupId == 0 || args.Name == "" || args.Timeframe == "" {
		return nil, fmt.Errorf("missing required fields")
	}
	cmdTag, err := conn.DB.Exec(context.Background(), `
		UPDATE setups 
		SET name = $1, timeframe = $2, bars = $3, threshold = $4, dolvol = $5, adr = $6, mcap = $7 
		WHERE setupId = $8`,
		args.Name, args.Timeframe, args.Bars, args.Threshold, args.Dolvol, args.Adr, args.Mcap, args.SetupId)
	if err != nil {
		return nil, fmt.Errorf("error updating setup: %v", err)
	} else if cmdTag.RowsAffected() != 1 {
		return nil, fmt.Errorf("dkn0w")

	}
	return SetupResult{
		SetupId:   args.SetupId,
		Name:      args.Name,
		Timeframe: args.Timeframe,
		Bars:      args.Bars,
		Threshold: args.Threshold,
		Dolvol:    args.Dolvol,
		Adr:       args.Adr,
		Mcap:      args.Mcap,
	}, nil
}
