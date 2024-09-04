package tasks

import (
	"api/data"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode"
)

type GetChartDataArgs struct {
	SecurityId    int    `json:"securityId"`
	Timeframe     string `json:"timeframe"`
	Datetime      string `json:"datetime"`  // If this datetime is just a date, it needs to grab the end of the day as opposed to the beginning of the day
	Direction     string `json:"direction"` // to ensure that we get the data from that date
	Bars          int    `json:"bars"`
	ExtendedHours bool   `json:"extendedhours"`
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
	// CHECK TO MAKE SURE EndDateTime > StartDateTime ***********
	var args GetChartDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("0sjh getChartData invalid args: %v", err)
	}
	multiplier, timespan, _, _, err := getTimeframe(args.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("getChartData invalid timeframe: %v", err)
	}
	fmt.Printf("Passed Datetime: {%s}, Passed bars :{%v}", args.Datetime, args.Bars)
	var query string
	var polyResultOrder string
	var minDate time.Time
	var maxDate time.Time
	// This probably could be optimized to just having a variable for the comparison sign
	// but its fine for now
	if args.Datetime == "" {
		query = `SELECT ticker, minDate, maxDate 
                 FROM securities 
                 WHERE securityid = $1 AND 
				 $2 = $2
				 ORDER BY minDate DESC`
		polyResultOrder = "desc"
	} else if args.Direction == "backward" {
		query = `SELECT ticker, minDate, maxDate
				 FROM securities 
				 WHERE securityid = $1 AND (maxDate > $2 OR maxDate IS NULL)
				 ORDER BY minDate DESC`
		polyResultOrder = "desc"
		maxDate, err = data.StringToTime(args.Datetime)
		if err != nil {
			return nil, fmt.Errorf("zdk4g: Datetime parse error %v", err)
		}
	} else if args.Direction == "forward" {
		query = `SELECT ticker, minDate, maxDate
				 FROM securities 
				 WHERE securityid = $1 AND (minDate < $2)
				 ORDER BY minDate ASC`
		polyResultOrder = "asc"
		minDate, err = data.StringToTime(args.Datetime)
		if err != nil {
			return nil, fmt.Errorf("3ktng: Datetime parse error %v", err)
		}
	} else {
		return nil, fmt.Errorf("9d83j: Incorrect direction passed")
	}
	fmt.Println(query)
	rows, err := conn.DB.Query(context.Background(), query, args.SecurityId, args.Datetime)
	if err != nil {
		return nil, fmt.Errorf("2fg0 %w", err)
	}
	// we will iterate through each entry in ticker db for the given security id
	// until we have completed the request, starting with the most recent.
	// this allows us to handle ticker changes if the data request requires pulling across
	// two different tickers.
	var barDataList []GetChartDataResults
	numBarsRemaining := args.Bars
	for rows.Next() {
		var ticker string
		var minDateFromSQL *time.Time
		var maxDateFromSQL *time.Time
		err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL)
		if err != nil {
			return nil, fmt.Errorf("3tyl %w", err)
		}
		if maxDateFromSQL == nil {
			now := time.Now()
			maxDateFromSQL = &now
		}
		fmt.Println(minDateFromSQL, maxDateFromSQL, maxDate)
		// Estimate the start date to be sent to polygon. This should ideally overestimate the amount of data
		// Needed to fulfill the number of requested bars
		var queryStartTime time.Time // Used solely as the start date for polygon query
		var queryEndTime time.Time   // Used solely as the end date for polygon query
		if args.Direction == "backward" {
			if maxDate.Compare(*maxDateFromSQL) == 1 { // if maxdate from the securities is before the current max date
				maxDate = *maxDateFromSQL
			}

			// var estimatedStartTime time.Time
			// if dataConsolidationType == "d" {
			// 	estimatedStartTime = maxDate.AddDate(0, 0, -(numBarsRemaining * baseAggregateMultiplier * multiplier))
			// } else if dataConsolidationType == "m" {
			// 	estimatedStartTime = maxDate.Add(time.Duration(-numBarsRemaining*baseAggregateMultiplier*multiplier) * time.Minute)
			// 	// estimatedStartTime = maxDate.AddDate(0, 0, 0, -(numBarsRemaining * baseAggregateMultiplier * multiplier), 0, 0, 0)
			// } else if dataConsolidationType == "s" {
			// 	// ESTIMATE THE start date
			// 	estimatedStartTime = maxDate.Add(time.Duration(-numBarsRemaining*baseAggregateMultiplier*multiplier) * time.Second)
			// } else {
			// 	return nil, fmt.Errorf("34kgf Invalid dataConsolidationType {%v}", dataConsolidationType)
			// }
			// // if estimated Start time is greater than the ticker min date, then use the estimatedStartTime as the request start
			// if estimatedStartTime.Compare(minDate) == 1 {
			// 	queryStartTime = estimatedStartTime
			// } else {
			// 	queryStartTime = minDate
			// }
			queryStartTime = *minDateFromSQL
			queryEndTime = maxDate
			maxDate = queryStartTime
		} else if args.Direction == "forward" { // MIGHT NOT NEED THIS CHECK AS INCORRECT DIRCTIONS GET FILTERED OUT ABOVE
			if minDate.Compare(*minDateFromSQL) == -1 {
				minDate = *minDateFromSQL
			}
			// var estimatedEndTime time.Time
			// if dataConsolidationType == "d" {
			// 	estimatedEndTime = minDate.AddDate(0, 0, (numBarsRemaining * baseAggregateMultiplier * multiplier)) // going to need some flexibilty for weekends
			// } else if dataConsolidationType == "m" {
			// 	// ESTIMATE MINUTE END TIME
			// } else if dataConsolidationType == "s" {
			// 	// ESTIMATE SECOND END TIME
			// }
			// if estimatedEndTime.Compare(maxDate) == -1 {
			// 	queryEndTime = estimatedEndTime
			// } else {
			// 	queryEndTime = maxDate
			// }
			queryEndTime = *maxDateFromSQL
			queryStartTime = minDate
			minDate = queryEndTime
		}
		// SIDE NOTE: We need to figure out how often GetAggs is updated
		// within polygon to see what endpoint we need to call
		// for live intraday data.
		fmt.Printf("Query Start Date: %s, Query End Date: %s \n", queryStartTime, queryEndTime)
		iter, err := data.GetAggsData(conn.Polygon, ticker, multiplier, timespan,
			data.MillisFromDatetimeString(queryStartTime.Format(time.DateTime)), data.MillisFromDatetimeString(queryEndTime.Format(time.DateTime)),
			5000, polyResultOrder)
		if err != nil {
			return nil, fmt.Errorf("rfk3f, %v", err)
		}
		for iter.Next() {
			if numBarsRemaining <= 0 {
				if args.Direction == "forward" {
					return barDataList, nil
				} else {
					left, right := 0, len(barDataList)-1
					for left < right {
						barDataList[left], barDataList[right] = barDataList[right], barDataList[left]
						left++
						right--
					}
					return barDataList, nil
				}
			}
			var barData GetChartDataResults
			barData.Datetime = float64(time.Time(iter.Item().Timestamp).Unix())
			barData.Open = iter.Item().Open
			barData.High = iter.Item().High
			barData.Low = iter.Item().Low
			barData.Close = iter.Item().Close
			barData.Volume = iter.Item().Volume
			barDataList = append(barDataList, barData)
			numBarsRemaining--
		}
		// if we have undershot with the current row of information in security db

	}
	if len(barDataList) != 0 {
		return barDataList, nil
	}

	return nil, fmt.Errorf("c34lg: Did not return bar data for securityid {%v}, timeframe {%v}, datetime {%v}, direction {%v}, Bars {%v}, extendedHours {%v}",
		args.SecurityId, args.Timeframe, args.Datetime, args.Direction, args.Bars, args.ExtendedHours)
}

/*
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

		rows, err := conn.DB.Query(context.Background(), "SELECT ticker, minDate, maxDate FROM securities WHERE securityid = $1 AND (maxDate >= $2 OR maxDate is null) ORDER BY minDate desc", args.SecurityId, args.EndDateTime)
		if err != nil {
			return nil, fmt.Errorf("3srg %v", err)
		}
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
*/
func getTimeframe(timeframeString string) (int, string, string, int, error) {
	// if no identifer is passed, it means that it should be minute data
	lastChar := rune(timeframeString[len(timeframeString)-1])
	if unicode.IsDigit(lastChar) {
		num, err := strconv.Atoi(timeframeString)
		if err != nil {
			return 0, "", "", 0, err
		}
		return num, "minute", "m", 1, nil
	}
	// else, there is an identifier and not minute

	// add .toLower() or toUpper to not have to check two different cases
	identifier := string(timeframeString[len(timeframeString)-1])
	num, err := strconv.Atoi(timeframeString[:len(timeframeString)-1])
	if err != nil {
		return 0, "", "", 0, err
	}
	// add .toLower() or toUpper to not have to check two different cases
	if identifier == "s" {
		return num, "second", "s", 1, nil
	} else if identifier == "h" {
		return num, "hour", "m", 60, nil
	} else if identifier == "d" {
		return num, "day", "d", 1, nil
	} else if identifier == "w" {
		return num, "week", "d", 7, nil
	} else if identifier == "m" {
		return num, "month", "d", 30, nil
	} else if identifier == "y" {
		return num, "year", "d", 365, nil
	}
	return 0, "", "", 0, fmt.Errorf("incorrect timeframe passed")
}
