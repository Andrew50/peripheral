package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
)

/*type Algo struct {
	AlgoID   int    `json:"algoId"`
	AlgoName string `json:"algoName"`
}
// GetAlgos performs operations related to GetAlgos functionality.
func GetAlgos(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT algoId, algoName
		FROM algos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var algos []Algo
	for rows.Next() {
		var algo Algo
		if err := rows.Scan(&algo.AlgoID, &algo.AlgoName); err != nil {
			return nil, err
		}
	}
	return algos, nil
}*/
// A SetupResult represents a setup configuration with its evaluation score.
type SetupResult struct {
	SetupID   int     `json:"setupId"`
	Name      string  `json:"name"`
	Timeframe string  `json:"timeframe"`
	Bars      int     `json:"bars"`
	Threshold int     `json:"threshold"`
	Dolvol    float64 `json:"dolvol"`
	Adr       float64 `json:"adr"`
	Mcap      float64 `json:"mcap"`
	Score     int     `json:"score"`
}

// GetSetups performs operations related to GetSetups functionality.
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
		if err := rows.Scan(&setupResult.SetupID, &setupResult.Name, &setupResult.Timeframe, &setupResult.Bars, &setupResult.Threshold, &setupResult.Dolvol, &setupResult.Adr, &setupResult.Mcap, &setupResult.Score); err != nil {
			return nil, fmt.Errorf("sdifn0 %v", err)
		}
		setupResults = append(setupResults, setupResult)
	}
	return setupResults, nil
}

// NewSetupArgs represents a structure for handling NewSetupArgs data.
type NewSetupArgs struct {
	Name      string  `json:"name"`
	Timeframe string  `json:"timeframe"`
	Bars      int     `json:"bars"`
	Threshold int     `json:"threshold"`
	Dolvol    float64 `json:"dolvol"`
	Adr       float64 `json:"adr"`
	Mcap      float64 `json:"mcap"`
}

// NewSetup performs operations related to NewSetup functionality.
func NewSetup(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewSetupArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}
	if args.Name == "" || args.Timeframe == "" {
		return nil, fmt.Errorf("dlkns")
	}
	var setupID int
	err := conn.DB.QueryRow(context.Background(), `
		INSERT INTO setups (name, timeframe, bars, threshold, dolvol, adr, mcap, userId) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING setupId`,
		args.Name, args.Timeframe, args.Bars, args.Threshold, args.Dolvol, args.Adr, args.Mcap, userId,
	).Scan(&setupID)

	if err != nil {
		return nil, fmt.Errorf("dkngvw0 %v", err)
	}
	utils.CheckSampleQueue(conn, setupID, false)
	return SetupResult{
		SetupID:   setupID,
		Name:      args.Name,
		Timeframe: args.Timeframe,
		Bars:      args.Bars,
		Threshold: args.Threshold,
		Dolvol:    args.Dolvol,
		Adr:       args.Adr,
		Mcap:      args.Mcap,
	}, nil
}

// DeleteSetupArgs represents a structure for handling DeleteSetupArgs data.
type DeleteSetupArgs struct {
	SetupID int `json:"setupId"`
}

// DeleteSetup performs operations related to DeleteSetup functionality.
func DeleteSetup(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteSetupArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}
	_, err := conn.DB.Exec(context.Background(), `
		DELETE FROM setups 
		WHERE setupId = $1`, args.SetupID)

	if err != nil {
		return nil, fmt.Errorf("error deleting setup: %v", err)
	}
	return nil, nil
}

// SetSetupArgs represents a structure for handling SetSetupArgs data.
type SetSetupArgs struct {
	SetupID   int     `json:"setupId"`
	Name      string  `json:"name"`
	Timeframe string  `json:"timeframe"`
	Bars      int     `json:"bars"`
	Threshold int     `json:"threshold"`
	Dolvol    float64 `json:"dolvol"`
	Adr       float64 `json:"adr"`
	Mcap      float64 `json:"mcap"`
}

// SetSetup performs operations related to SetSetup functionality.
func SetSetup(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetSetupArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
	if args.SetupID == 0 || args.Name == "" || args.Timeframe == "" {
		return nil, fmt.Errorf("missing required fields")
	}
	cmdTag, err := conn.DB.Exec(context.Background(), `
		UPDATE setups 
		SET name = $1, timeframe = $2, bars = $3, threshold = $4, dolvol = $5, adr = $6, mcap = $7 
		WHERE setupId = $8`,
		args.Name, args.Timeframe, args.Bars, args.Threshold, args.Dolvol, args.Adr, args.Mcap, args.SetupID)
	if err != nil {
		return nil, fmt.Errorf("error updating setup: %v", err)
	} else if cmdTag.RowsAffected() != 1 {
		return nil, fmt.Errorf("dkn0w")

	}
	return SetupResult{
		SetupID:   args.SetupID,
		Name:      args.Name,
		Timeframe: args.Timeframe,
		Bars:      args.Bars,
		Threshold: args.Threshold,
		Dolvol:    args.Dolvol,
		Adr:       args.Adr,
		Mcap:      args.Mcap,
	}, nil
}
