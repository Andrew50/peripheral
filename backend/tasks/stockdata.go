package tasks

import (
	"api/data"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode"

	"github.com/polygon-io/client-go/rest/models"
)

type GetChartDataArgs struct {
	SecurityId    string `json:"security"`
	Timeframe     string `json:"timeframe"`
	EndDateTime   string `json:"endtime"`
	NumBars       int    `json:"numbars"`
	StartDateTime string `json:"starttime"`
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

// 1m should only load ~4 days worth of data at a time, at maximum scroll out
func GetChartData(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetChartDataArgs
	// CHECK TO MAKE SURE EndDateTime > StartDateTime ***********
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("0sjh getChartData invalid args: %v", err)
	}
	multiplier, timespan, dataConsolidationType, baseAggregateMultiplier, err := getTimeframe(args.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("getChartData invalid timeframe: %v", err)
	}
	var ticker string
	var maxDatePtr *time.Time
	var maxDate time.Time
	var minDate time.Time

	// Prepare the query based on whether datetime is empty
	var query string
	if args.datetime == "" { //if no endDate provided then get the most recent ticker for that security, not necesarily null becuase could be delisted
		query = `SELECT ticker, minDate, maxDate 
                 FROM securities 
                 WHERE securityid = $1 
                 ORDER BY maxDate IS NULL DESC, maxDate DESC`
		err = conn.DB.QueryRow(context.Background(), query, args.SecurityId).Scan(&ticker, &minDate, &maxDatePtr)
		if maxDatePtr == nil {
			maxDate = time.Now()
		}
	} else {
		maxDate, err = data.StringToTime(args.datetime)
		if err != nil {
			return nil, fmt.Errorf("cd0f %v", err)
		}
		query = `SELECT ticker, minDate
                 FROM securities 
                 WHERE securityid = $1 
                 AND (maxDate > $2 OR maxDate IS NULL)
                 ORDER BY maxDate IS NULL DESC, maxDate DESC`
		err = conn.DB.QueryRow(context.Background(), query, args.SecurityId, args.datetime).Scan(&ticker, &minDate)
	}

	var barDataList []GetChartDataResults
	// First figure out request upper bound
	if args.EndDateTime == "" {

	}
	if dataConsolidationType == "daily" {

	}
	rows, err := conn.DB.Query(context.Background(), "SELECT ticker, minDate, maxDate FROM securities WHERE securityid = $1 AND (maxDate >= $2 OR maxDate is null) ORDER BY minDate desc", args.SecurityId, args.EndDateTime)
	if err != nil {
		return nil, fmt.Errorf("3srg %v", err)
	}
	/*for rows.Next() {
	//candleDataCount := 0
	for rows.Next() {
		var ticker string
		var maxDate time.Time
		var minDate time.Time
		err := rows.Scan(&ticker, &minDate, &maxDate)
		if err != nil {
			return nil, err
		}
	}*/
	easternTimeLocation, tzErr := time.LoadLocation("America/New_York")
	if tzErr != nil {
		return nil, err
	}
	start := models.Millis(maxDate.AddDate(0, 0, -20).In(easternTimeLocation))
	end := models.Millis(maxDate.In(easternTimeLocation))
	//fmt.Printf(ticker, multiplier,timespan,start,end)
	iter, err := data.GetAggsData(conn.Polygon, ticker, multiplier, timespan, start, end, 1000)
	if err != nil {
		return nil, err
	}
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
func getTimeframe(timeframeString string) (int, string, string, int, error) {
	// if no identifer is passed, it means that it should be minute data
	lastChar := rune(timeframeString[len(timeframeString)-1])
	if unicode.IsDigit(lastChar) {
		num, err := strconv.Atoi(timeframeString)
		if err != nil {
			return 0, "", "", 0, err
		}
		return num, "minute", "minute", 1, nil
	}
	// else, there is an identifier and not minute
	identifier := string(timeframeString[len(timeframeString)-1])
	num, err := strconv.Atoi(timeframeString[:len(timeframeString)-1])
	if err != nil {
		return 0, "", "", 0, err
	}
	if identifier == "s" || identifier == "S" {
		return num, "second", nil
	} else if identifier == "h" || identifier == "H" {
		return num, "hour", nil
	} else if identifier == "d" || identifier == "D" {
		return num, "day", nil
	} else if identifier == "w" || identifier == "W" {
		return num, "week", nil
	} else if identifier == "m" || identifier == "M" {
		return num, "month", nil
	} else if identifier == "y" || identifier == "Y" {
		return num, "year", nil
	}
	return 0, "", "", 0, fmt.Errorf("incorrect timeframe passed")
}
