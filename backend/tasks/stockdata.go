package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

type GetSecurityDateBoundsArgs struct {
	SecurityId int `json:"securityId"`
}

type GetSecurityDateBoundsResults struct {
	MinDate float64 `json:"minDate"`
	MaxDate float64 `json:"maxDate"`
}

// 1m should only load ~4 days worth of data at a time, at maximum scroll out
func GetSecurityDateBounds(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetSecurityDateBoundsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("3k5pv GetSecurityDateBounds invalid args: %v", err)
	}
	query := `SELECT MIN(minDate) as minDate, 
			CASE WHEN COUNT(maxDate) = COUNT(*) THEN MAX(maxDate)
				ELSE NULL 
			END AS maxDate
				FROM SECURITIES 
				WHERE securityid = $1 
				GROUP BY securityid`
	rows, err := conn.DB.Query(context.Background(), query, args.SecurityId)
	if err != nil {
		return nil, fmt.Errorf("2j6kld: %v", err)
	}
	defer rows.Close()
	easternTimeLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, err
	}
	var result GetSecurityDateBoundsResults
	for rows.Next() {
		var tableMinDate *time.Time
		var tableMaxDate *time.Time
		err := rows.Scan(&tableMinDate, &tableMaxDate)
		if err != nil {
			return nil, fmt.Errorf("43lg: %v", err)
		}
		result.MinDate = float64(tableMinDate.Unix())
		if tableMaxDate == nil {
			var ticker string
			tickerQuery := `SELECT ticker FROM SECURITIES WHERE securityId = $1 ORDER BY minDate DESC`
			err := conn.DB.QueryRow(context.Background(), tickerQuery).Scan(&ticker)
			if err != nil {
				return nil, fmt.Errorf("3pgkv: %v", err)
			}
			queryStart, err := utils.MillisFromDatetimeString(time.Now().Add(-24 * time.Hour).In(easternTimeLocation).Format(time.DateTime))
			if err != nil {
				return nil, fmt.Errorf("k4lvm, %v", err)
			}
			queryEnd, err := utils.MillisFromDatetimeString(time.Now().In(easternTimeLocation).Format(time.DateTime))
			if err != nil {
				return nil, fmt.Errorf("4lgkv, %v", err)
			}
			iter, err := utils.GetAggsData(conn.Polygon, ticker, 1, "day", queryStart, queryEnd, 1000, "desc", false)
			if err != nil {
				return nil, fmt.Errorf("5jk4lv, %v", err)
			}
			for iter.Next() {
				result.MaxDate = float64(time.Time(iter.Item().Timestamp).Unix())
				return result, nil
			}
		} else {
			result.MaxDate = float64(tableMaxDate.Unix())
			return result, nil
		}
	}
	return nil, fmt.Errorf("did not return bounds for securityId {%v}", args.SecurityId)
}

/*type GetChartDataArgs struct {
	SecurityId    int    `json:"securityId"`
	Timeframe     string `json:"timeframe"`
	Timestamp     int64  `json:"timestamp"` // If this datetime is just a date, it needs to grab the end of the day as opposed to the beginning of the day
	Direction     string `json:"direction"` // to ensure that we get the data from that date
	Bars          int    `json:"bars"`
	ExtendedHours bool   `json:"extendedHours"`
	IsReplay      bool   `json:"isreplay"`
	//EndTime   string `json:"endtime"`
}
type GetChartDataResults struct {
	Timestamp float64 `json:"time"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}*/

/*func GetChartData(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	// CHECK TO MAKE SURE EndDateTime > StartDateTime ***********
	var args GetChartDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("0sjh getChartData invalid args: %v", err)
	}
	multiplier, timespan, _, _, err := utils.GetTimeFrame(args.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("getChartData invalid timeframe: %v", err)
	}
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location 3klffk")

	}
	// if !strings.Contains(args.Datetime, "-") && args.Datetime != "" {
	// 	seconds, err := strconv.ParseInt(args.Datetime, 10, 64)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("3k5lv: Error converting string to int: %v", err)
	// 	}
	// 	t := time.Unix(seconds, 0)
	// 	args.Datetime = t.Format(time.DateTime)
	// }
	inputTimestamp := time.Unix(args.Timestamp/1000, (args.Timestamp%1000)*1e6).UTC()
	var query string
	var polyResultOrder string
	var minDate time.Time
	var maxDate time.Time
	// This probably could be optimized to just having a variable for the comparison sign
	// but its fine for now
	if args.Timestamp == 0 { // default value
		query = `SELECT ticker, minDate, maxDate 
                 FROM securities 
                 WHERE securityid = $1 AND 
				 ticker != $2
				 ORDER BY minDate DESC`
		polyResultOrder = "desc"
	} else if args.Direction == "backward" {
		query = `SELECT ticker, minDate, maxDate
				 FROM securities 
				 WHERE securityid = $1 AND (maxDate > $2 OR maxDate IS NULL)
				 ORDER BY minDate DESC limit 1`
		polyResultOrder = "desc"
		maxDate = inputTimestamp
		fmt.Println(maxDate)
	} else if args.Direction == "forward" {
		query = `SELECT ticker, minDate, maxDate
				 FROM securities 
				 WHERE securityid = $1 AND  (minDate < $2)
				 ORDER BY minDate ASC`
		polyResultOrder = "asc"
		minDate = inputTimestamp

	} else {
		return nil, fmt.Errorf("9d83j: Incorrect direction passed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := conn.DB.Query(ctx, query, args.SecurityId, inputTimestamp.In(easternLocation).Format(time.DateTime))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("2fg0 %w", err)
	}
	defer rows.Close()

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
		var minDateSQL time.Time
		var maxDateSQL time.Time
		if minDateFromSQL != nil {
			minDateET := (*minDateFromSQL).In(easternLocation)
			minDateSQL = time.Date(minDateET.Year(), minDateET.Month(), minDateET.Day(), 0, 0, 0, 0, easternLocation).AddDate(0, 0, 1)
		}
		if maxDateFromSQL != nil {
			maxDateET := (*maxDateFromSQL).In(easternLocation)
			maxDateSQL = time.Date(maxDateET.Year(), maxDateET.Month(), maxDateET.Day(), 0, 0, 0, 0, easternLocation).AddDate(0, 0, 1)
		}

		// Estimate the start date to be sent to polygon. This should ideally overestimate the amount of data
		// Needed to fulfill the number of requested bars
		var queryStartTime time.Time // Used solely as the start date for polygon query
		var queryEndTime time.Time   // Used solely as the end date for polygon query
		if args.Direction == "backward" {
			if maxDate.Compare(maxDateSQL) == 1 || maxDate.IsZero() { // if maxdate from the securities is before the current max date
				maxDate = maxDateSQL
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
			queryStartTime = minDateSQL
			queryEndTime = maxDate
			maxDate = queryStartTime
		} else if args.Direction == "forward" { // MIGHT NOT NEED THIS CHECK AS INCORRECT DIRCTIONS GET FILTERED OUT ABOVE
			if minDate.Compare(minDateSQL) == -1 {
				minDate = minDateSQL
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
			queryEndTime = maxDateSQL
			queryStartTime = minDate
			minDate = queryEndTime
		}
		// SIDE NOTE: We need to figure out how often GetAggs is updated
		// within polygon to see what endpoint we need to call
		// for live intraday data.
		//fmt.Printf("Query Params Start Date: %s, Query End Date: %s \n", time.Time(queryStartTime), time.Time(queryEndTime))
		date1, err := utils.MillisFromUTCTime(queryStartTime)
		if err != nil {
			return nil, fmt.Errorf("1n0f %v", err)
		}
		date2, err := utils.MillisFromUTCTime(queryEndTime)
		if err != nil {
			return nil, fmt.Errorf("n91ve2n0 %v", err)
		}
		//fmt.Printf("Query Start Date: %s, Query End Date: %s \n", time.Time(date1), time.Time(date2))
		iter, err := utils.GetAggsData(conn.Polygon, ticker, multiplier, timespan,
			date1, date2,
			5000, polyResultOrder, !args.IsReplay)
		if err != nil {
			return nil, fmt.Errorf("rfk3f, %v", err)
		}
		for iter.Next() {
            if !args.ExtendedHours {
                timestamp := time.Time(iter.Item().Timestamp).In(easternLocation)
                hour := timestamp.Hour()
                minute := timestamp.Minute()
                if hour < 9 || (hour == 9 && minute < 30) || hour >= 16 {
                    continue
                }
            }
			if numBarsRemaining <= 0 {
				if args.Direction == "forward" {
					fmt.Println("forward working")
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
			barData.Timestamp = float64(time.Time(iter.Item().Timestamp).Unix())
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
		if args.Direction == "forward" {
			fmt.Println("forward working")
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

	return nil, fmt.Errorf("c34lg")
}*/


type GetTradeDataArgs struct {
	SecurityID    int64 `json:"securityId"`
	Timestamp     int64 `json:"time"`
	LengthOfTime  int64 `json:"lengthOfTime"` //length of time in milliseconds
	ExtendedHours bool  `json:"extendedHours"`
}

type GetTradeDataResults struct {
	Timestamp  int64   `json:"timestamp"`
	Price      float64 `json:"price"`
	Size       float64 `json:"size"`
	Exchange   int     `json:"exchange"`
	Conditions []int32 `json:"conditions"`
}

func GetTradeData(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTradeDataArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("0sj33gh getTradeData invalid args: %v", err)
	}
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location 3355767uh")

	}
	inputTime := time.Unix(args.Timestamp/1000, (args.Timestamp%1000)*1e6).UTC()

	query := `SELECT ticker, minDate, maxDate FROM securities WHERE securityid=$1 AND (minDate <= $2 AND (maxDate IS NULL or maxDate >= $2)) ORDER BY minDate ASC`

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, query, args.SecurityID, inputTime.In(easternLocation).Format(time.DateTime))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("45lgkv %w", err)
	}
	defer rows.Close()

	var tradeDataList []GetTradeDataResults
	windowStartTime := args.Timestamp                   // milliseconds
	windowEndTime := args.Timestamp + args.LengthOfTime // milliseconds
	for rows.Next() {
		var ticker string
		var minDateFromSQL *time.Time
		var maxDateFromSQL *time.Time
		err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL)
		if err != nil {
			return nil, fmt.Errorf("vfm4l %w", err)
		}
		//fmt.Println(ticker)
		windowStartTimeNanos, err := utils.NanosFromUTCTime(time.Unix(windowStartTime/1000, (windowStartTime % 1000 * 1e6)).UTC())
		if err != nil {
			return nil, fmt.Errorf("45l6k6lkgjl, %v", err)
		}
		//fmt.Printf("\nWindow Start: {%v}, Window End: {%v}", windowStartTime, windowEndTime)
		//fmt.Printf("Ticker {%v}", ticker)
		iter, err := utils.GetTrade(conn.Polygon, ticker, windowStartTimeNanos, "asc", models.GTE, 30000)
		if err != nil {
			return nil, fmt.Errorf("4lyoh, %v", err)
		}
		for iter.Next() {
			if int64(time.Time(iter.Item().ParticipantTimestamp).Unix())*1000 > windowEndTime {
				return tradeDataList, nil
			}
            if !args.ExtendedHours {
                timestamp := time.Time(iter.Item().ParticipantTimestamp).In(easternLocation)
                hour := timestamp.Hour()
                minute := timestamp.Minute()
                if hour < 9 || (hour == 9 && minute < 30) || hour >= 16 {
                    continue
                }
            }
			var tradeData GetTradeDataResults
			tradeData.Timestamp = time.Time(iter.Item().ParticipantTimestamp).UnixNano() / int64(time.Millisecond)
			tradeData.Price = iter.Item().Price
			tradeData.Size = iter.Item().Size
			tradeData.Exchange = iter.Item().Exchange
			tradeData.Conditions = iter.Item().Conditions
			tradeDataList = append(tradeDataList, tradeData)
		}
		windowStartTime = tradeDataList[len(tradeDataList)-1].Timestamp
	}
	if len(tradeDataList) != 0 {
		return tradeDataList, nil
	}
    return nil, fmt.Errorf("on0fi01in0f")
}

type GetQuoteDataArgs struct {
	SecurityID    int64 `json:"securityId"`
	Timestamp     int64 `json:"time"`
	LengthOfTime  int64 `json:"lengthOfTime"`
	ExtendedHours bool  `json:"extendedHours"`
}

type GetQuoteDataResults struct {
	Timestamp int64   `json:"timestamp"`
	BidPrice  float64 `json:"bidPrice"`
	AskPrice  float64 `json:"askPrice"`
	BidSize   float64 `json:"bidSize"`
	AskSize   float64 `json:"askSize"`
}

func GetQuoteData(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetQuoteDataArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("getQuoteData invalid args: %v", err)
	}
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}
	inputTime := time.Unix(args.Timestamp/1000, (args.Timestamp%1000)*1e6).UTC()
	query := `SELECT ticker, minDate, maxDate FROM securities WHERE securityid=$1 AND (minDate <= $2 AND (maxDate IS NULL or maxDate >= $2)) ORDER BY minDate ASC`
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rows, err := conn.DB.Query(ctx, query, args.SecurityID, inputTime.In(easternLocation).Format(time.DateTime))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()
	var quoteDataList []GetQuoteDataResults
	windowStartTime := args.Timestamp                   // milliseconds
	windowEndTime := args.Timestamp + args.LengthOfTime // milliseconds
	for rows.Next() {
		var ticker string
		var minDateFromSQL *time.Time
		var maxDateFromSQL *time.Time
		err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		windowStartTimeNanos, err := utils.NanosFromUTCTime(time.Unix(windowStartTime/1000, (windowStartTime%1000)*1e6).UTC())
		if err != nil {
			return nil, fmt.Errorf("error converting time: %v", err)
		}
		iter := utils.GetQuote(conn.Polygon, ticker, windowStartTimeNanos, "asc", models.GTE, 30000)
		for iter.Next() {
			if int64(time.Time(iter.Item().ParticipantTimestamp).Unix())*1000 > windowEndTime {
				return quoteDataList, nil
			}
            if !args.ExtendedHours {
                timestamp := time.Time(iter.Item().ParticipantTimestamp).In(easternLocation)
                hour := timestamp.Hour()
                minute := timestamp.Minute()
                if hour < 9 || (hour == 9 && minute < 30) || hour >= 16 {
                    continue
                }
            }
			var quoteData GetQuoteDataResults
			quoteData.Timestamp = time.Time(iter.Item().ParticipantTimestamp).UnixNano() / int64(time.Millisecond)
			quoteData.BidPrice = iter.Item().BidPrice
			quoteData.AskPrice = iter.Item().AskPrice
			quoteData.BidSize = iter.Item().BidSize
			quoteData.AskSize = iter.Item().AskSize
			quoteDataList = append(quoteDataList, quoteData)
		}
		windowStartTime = quoteDataList[len(quoteDataList)-1].Timestamp
	}
	if len(quoteDataList) != 0 {
		return quoteDataList, nil
	}
	return nil, fmt.Errorf("kn20vke0")
}
