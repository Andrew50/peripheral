package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
)

type SetupResult struct {
    SetupId   int    `json:"setupId"`
    Name     string   `json:"name"`
    Timeframe  string  `json:"timeframe"`
    Bars   int   `json:"bars"`
    Threshold int `json:"threshold"`
    Dolvol   float64 `json:"dolvol"`
    Adr     float64  `json:"adr"`
    Mcap    float64   `json:"mcap"`
}

func GetSetups(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
    SELECT setupId, name, timeframe, bars, threshold, dolvol, adr, mcap 
    from setups where userId = $1`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var setupResults []SetupResult
	for rows.Next() {
		var setupResult SetupResult
		if err := rows.Scan(&setupResult.SetupId, &setupResult.Name, &setupResult.Timeframe,&setupResult.Bars,&setupResult.Threshold,&setupResult.Dolvol, &setupResult.Adr,&setupResult.Mcap); err != nil {
			return nil, err
		}
		setupResults = append(setupResults, setupResult)
	}
	return setupResults, nil
}
