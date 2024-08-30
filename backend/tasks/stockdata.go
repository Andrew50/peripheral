package tasks

import (
	"api/data"
	"encoding/json"
	"fmt"
	"time"
)

type GetChartDataArgs struct {
	Ticker    string `json:"ticker"`
	Timeframe string `json:"timeframe"`
	//StartTime string `json:"starttime"`
	//EndTime   string `json:"endtime"`
}
type GetChartDataResults struct {
	Datetime string  `json:"time"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
}

func GetChartData(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetChartDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("getChartData invalid args: %v", err)
	}
	iter := data.GetAggsData(conn.Polygon, args.Ticker, 1, "day", data.MillisFromDatetimeString("2023-01-01"),
		data.MillisFromDatetimeString("2024-08-30"), 1000)
	var barDataList []GetChartDataResults
	for iter.Next() {
		var barData GetChartDataResults
		barData.Datetime = time.Time(iter.Item().Timestamp).Format(time.DateOnly)
		barData.Open = iter.Item().Open
		barData.High = iter.Item().High
		barData.Low = iter.Item().Low
		barData.Close = iter.Item().Close

		barDataList = append(barDataList, barData)
	}
	return barDataList, nil
}
