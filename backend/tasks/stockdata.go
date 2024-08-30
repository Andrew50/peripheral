package tasks

import (
	"api/data"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode"
)

type GetChartDataArgs struct {
	Ticker    string `json:"ticker"`
	Timeframe string `json:"timeframe"`
	//StartTime string `json:"starttime"`
	//EndTime   string `json:"endtime"`
}
type GetChartDataResults struct {
	Datetime float64 `json:"time"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   float64 `json:"volume"`
}

func GetChartData(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetChartDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("getChartData invalid args: %v", err)
	}
	multiplier, timespan, err := getTimeframe(args.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("getChartData invalid timeframe: %v", err)
	}
	iter := data.GetAggsData(conn.Polygon, args.Ticker, multiplier, timespan, data.MillisFromDatetimeString("2024-08-25"),
		data.MillisFromDatetimeString("2024-08-30"), 1000)
	var barDataList []GetChartDataResults
	for iter.Next() {
		var barData GetChartDataResults
		barData.Datetime = float64(time.Time(iter.Item().Timestamp).Unix())
		barData.Open = iter.Item().Open
		barData.High = iter.Item().High
		barData.Low = iter.Item().Low
		barData.Close = iter.Item().Close
		barData.Volume = iter.Item().Volume
		barDataList = append(barDataList, barData)
	}
	return barDataList, nil
}
func getTimeframe(timeframeString string) (int, string, error) {
	// if no identifer is passed, it means that it should be minute data
	lastChar := rune(timeframeString[len(timeframeString)-1])
	if unicode.IsDigit(lastChar) {
		num, err := strconv.Atoi(timeframeString)
		if err != nil {
			return 0, "", err
		}
		return num, "minute", nil
	}
	// else, there is an identifier and not minute
	identifier := string(timeframeString[len(timeframeString)-1])
	num, err := strconv.Atoi(timeframeString[:len(timeframeString)-1])
	if err != nil {
		return 0, "", err
	}
	if identifier == "s" {
		return num, "second", nil
	} else if identifier == "h" {
		return num, "hour", nil
	} else if identifier == "d" {
		return num, "day", nil
	} else if identifier == "w" {
		return num, "week", nil
	} else if identifier == "m" {
		return num, "month", nil
	} else if identifier == "y" {
		return num, "year", nil
	}
	return 0, "", fmt.Errorf("incorrect timeframe passed")
}
