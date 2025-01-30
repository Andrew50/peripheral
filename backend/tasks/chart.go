package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

type GetChartDataArgs struct {
	SecurityId    int    `json:"securityId"`
	Timeframe     string `json:"timeframe"`
	Timestamp     int64  `json:"timestamp"`
	Direction     string `json:"direction"`
	Bars          int    `json:"bars"`
	ExtendedHours bool   `json:"extendedHours"`
	IsReplay      bool   `json:"isreplay"`
}

type GetChartDataResults struct {
	Timestamp float64 `json:"time"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

func MaxDivisorOf30(n int) int {
	for k := n; k >= 1; k-- {
		if 30%k == 0 && n%k == 0 {
			return k
		}
	}
	return 1 // 1 divides all integers, so we return 1 if no other common divisor is found.
}

func GetChartData(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetChartDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	fmt.Printf("\nDebug: Received request for SecurityId: %d, Timeframe: %s, Direction: %s\n",
		args.SecurityId, args.Timeframe, args.Direction)

	multiplier, timespan, _, _, err := utils.GetTimeFrame(args.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("invalid timeframe: %v", err)
	}

	fmt.Printf("Debug: Parsed timeframe - Multiplier: %d, Timespan: %s\n", multiplier, timespan)

	var queryTimespan string
	var queryMultiplier int
	var queryBars int
	var tickerForIncompleteAggregate string
	haveToAggregate := false
	if (timespan == "second" || timespan == "minute") && (30%multiplier != 0) {
		queryTimespan = timespan
		queryMultiplier = MaxDivisorOf30(multiplier)
		queryMultiplier = 1
		queryBars = args.Bars * multiplier / queryMultiplier
		haveToAggregate = true
	} else if timespan == "hour" { //&& !args.ExtendedHours { this was commented out idk if it does anything but it "works" now
		queryTimespan = "minute"
		queryMultiplier = 30
		queryBars = multiplier * 2 * args.Bars
		timespan = "minute"
		multiplier *= 60
		haveToAggregate = true
	} else {
		queryTimespan = timespan
		queryMultiplier = multiplier
		queryBars = args.Bars
	}

	if timespan != "minute" && timespan != "second" && timespan != "hour" {
		args.ExtendedHours = false
	}

	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}

	var inputTimestamp time.Time
	if args.Timestamp == 0 {
		inputTimestamp = time.Now().In(easternLocation)
	} else {
		inputTimestamp = time.Unix(args.Timestamp/1000, (args.Timestamp%1000)*1e6).UTC()
	}

	var query string
	var queryParams []interface{}
	var polyResultOrder string

	if args.Timestamp == 0 {
		query = `SELECT ticker, minDate, maxDate 
                 FROM securities 
                 WHERE securityid = $1
                 ORDER BY minDate DESC NULLS FIRST`
		queryParams = []interface{}{args.SecurityId}
		polyResultOrder = "desc"
	} else if args.Direction == "backward" {
		query = `SELECT ticker, minDate, maxDate
                 FROM securities 
                 WHERE securityid = $1 AND (maxDate > $2 OR maxDate IS NULL)
                 ORDER BY minDate DESC NULLS FIRST LIMIT 1`
		queryParams = []interface{}{args.SecurityId, inputTimestamp}
		polyResultOrder = "desc"
	} else if args.Direction == "forward" {
		query = `SELECT ticker, minDate, maxDate
                 FROM securities 
                 WHERE securityid = $1 AND (minDate < $2 OR minDate IS NULL)
                 ORDER BY minDate ASC NULLS LAST`
		queryParams = []interface{}{args.SecurityId, inputTimestamp}
		polyResultOrder = "asc"
	} else {
		return nil, fmt.Errorf("Incorrect direction passed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := conn.DB.Query(ctx, query, queryParams...)
	if err != nil {
		fmt.Printf("Debug: Database query failed: %v\n", err)
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("error querying data: %w", err)
	}
	defer rows.Close()

	var barDataList []GetChartDataResults
	numBarsRemaining := args.Bars

	fmt.Printf("Debug: Starting to process database rows\n")

	for rows.Next() {
		var ticker string
		var minDateFromSQL *time.Time
		var maxDateFromSQL *time.Time
		err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL)
		if err != nil {
			fmt.Printf("Debug: Error scanning row: %v\n", err)
			return nil, fmt.Errorf("error scanning data: %w", err)
		}
		tickerForIncompleteAggregate = ticker
		// Handle NULL dates from the database
		if maxDateFromSQL == nil {
			now := time.Now()
			maxDateFromSQL = &now
		}
		var minDateSQL, maxDateSQL time.Time
		if minDateFromSQL != nil {
			minDateSQL = minDateFromSQL.In(easternLocation)
		} else {
			// Default to a very early date if minDate is NULL
			minDateSQL = time.Date(1970, 1, 1, 0, 0, 0, 0, easternLocation)
		}

		if maxDateFromSQL != nil {
			maxDateSQL = (maxDateFromSQL).In(easternLocation)
		} else {
			// Default to current time if maxDate is NULL
			maxDateSQL = time.Now().In(easternLocation)
		}

		var queryStartTime, queryEndTime time.Time
		if args.Direction == "backward" {
			queryEndTime = inputTimestamp
			if maxDateSQL.Before(queryEndTime) {
				queryEndTime = maxDateSQL
			}
			queryStartTime = minDateSQL
			if queryStartTime.After(queryEndTime) {
				//fmt.Printf("\n%v, %v", queryStartTime, queryEndTime, args.Direction, args.Timestamp, args.Timeframe)
				return nil, fmt.Errorf("i10i0v")
			}
		} else if args.Direction == "forward" {
			queryStartTime = inputTimestamp
			if minDateSQL.After(queryStartTime) {
				queryStartTime = minDateSQL
			}
			queryEndTime = maxDateSQL
			if queryEndTime.Before(queryStartTime) {
				continue
			}
		}

		date1, date2, err := utils.GetRequestStartEndTime(queryStartTime, queryEndTime, args.Direction, timespan, multiplier, queryBars)
		if err != nil {
			fmt.Printf("Debug: GetRequestStartEndTime failed: %v\n", err)
			return nil, fmt.Errorf("dkn0 %v", err)
		}

		fmt.Printf("Debug: Requesting data from Polygon - Start: %v, End: %v\n", date1, date2)

		if haveToAggregate {
			fmt.Printf("Debug: Using aggregation path\n")
			iter, err := utils.GetAggsData(conn.Polygon, ticker, queryMultiplier, queryTimespan, date1, date2, 50000, "asc", !args.IsReplay)
			if err != nil {
				return nil, fmt.Errorf("error fetching data from Polygon: %v", err)
			}
			aggregatedData, err := buildHigherTimeframeFromLower(iter, multiplier, timespan, args.ExtendedHours, easternLocation, &numBarsRemaining, args.Direction)
			if err != nil {
				return nil, err
			}
			barDataList = append(barDataList, aggregatedData...)
			if numBarsRemaining <= 0 {
				break
			}
		} else {
			fmt.Printf("Debug: Using direct data path\n")
			iter, err := utils.GetAggsData(conn.Polygon, ticker, queryMultiplier, queryTimespan, date1, date2, 50000, polyResultOrder, !args.IsReplay)
			if err != nil {
				fmt.Printf("Debug: Polygon API error: %v\n", err)
				return nil, fmt.Errorf("error fetching data from Polygon: %v", err)
			}
			var processedBars int
			for iter.Next() {
				processedBars++
				item := iter.Item()
				if iter.Err() != nil {
					fmt.Printf("Debug: Iterator error: %v\n", iter.Err())
					return nil, fmt.Errorf("dkn0w")
				}
				timestamp := time.Time(item.Timestamp).In(easternLocation)
				
				if queryTimespan == "week" || queryTimespan == "month" || queryTimespan == "year" {
					for timestamp.Weekday() == time.Saturday || timestamp.Weekday() == time.Sunday {
						timestamp = timestamp.AddDate(0, 0, 1)
					}
				}

				// Skip if we need regular hours and timestamp is outside trading hours
				if (timespan == "minute" || timespan == "second" || timespan == "hour") && 
				   !args.ExtendedHours && !utils.IsTimestampRegularHours(timestamp) {
					continue
				}

				barData := GetChartDataResults{
					Timestamp: float64(timestamp.Unix()),
					Open:      item.Open,
					High:      item.High,
					Low:       item.Low,
					Close:     item.Close,
					Volume:    item.Volume,
				}
				barDataList = append(barDataList, barData)
				
				numBarsRemaining--
				if numBarsRemaining <= 0 {
					break
				}
			}
			if numBarsRemaining <= 0 {
				break
			}
		}
	}
	if len(barDataList) != 0 {
		if haveToAggregate || args.Direction == "forward" {
			if args.Direction == "backward" {
				fmt.Println("hit first one")
				marketStatus, err := utils.GetMarketStatus(conn)
				if err != nil {
					return nil, fmt.Errorf("issue with market status")
				}
				if (args.Timestamp == 0 && marketStatus != "closed") || args.IsReplay {
					incompleteAggregate, err := requestIncompleteBar(conn, tickerForIncompleteAggregate, args.Timestamp, multiplier, timespan,
						args.ExtendedHours, args.IsReplay, easternLocation)
					if err != nil {
						return nil, fmt.Errorf("issue with incomplete aggregate 65k5lhgfk, %v", err)
					}
					if len(barDataList) > 0 && incompleteAggregate.Timestamp == barDataList[len(barDataList)-1].Timestamp {
						barDataList = barDataList[:len(barDataList)-1]
					}
					if incompleteAggregate.Open != 0 {
						barDataList = append(barDataList, incompleteAggregate)
					}
				}
			}
			return barDataList, nil
		} else {
			reverse(barDataList)
			marketStatus, err := utils.GetMarketStatus(conn)
			if err != nil {
				return nil, fmt.Errorf("issue with market status")
			}
			if (args.Timestamp == 0 && marketStatus != "closed") || args.IsReplay {
				incompleteAggregate, err := requestIncompleteBar(conn, tickerForIncompleteAggregate, args.Timestamp, multiplier, timespan,
					args.ExtendedHours, args.IsReplay, easternLocation)
				fmt.Printf("\nagg:%v", incompleteAggregate)
				if err != nil {
					return nil, fmt.Errorf("issue with incomplete aggregate 65k5lhgfk, %v", err)
				}
				if len(barDataList) > 0 && incompleteAggregate.Timestamp == barDataList[len(barDataList)-1].Timestamp {
					barDataList = barDataList[:len(barDataList)-1]
				}
				if incompleteAggregate.Open != 0 {
					if utils.IsTimestampRegularHours(time.Unix(int64(incompleteAggregate.Timestamp), 0)) && !args.ExtendedHours || timespan == "day" || timespan == "week" || timespan == "month" {
						barDataList = append(barDataList, incompleteAggregate)
					}
				}
			}
			/*starTim := int64(barDataList[0].Timestamp)
			endTim := int64(barDataList[len(barDataList)-1].Timestamp)
			strt := time.Unix(starTim, (starTim)*1e6).UTC()
			end := time.Unix(endTim, (endTim)*1e6).UTC()*/
			return barDataList, nil
		}
	} else {
		fmt.Printf("Debug: No bars collected. NumBarsRemaining: %d\n", numBarsRemaining)
	}
	return nil, fmt.Errorf("no data found")
}

func reverse(data []GetChartDataResults) {
	left, right := 0, len(data)-1
	for left < right {
		data[left], data[right] = data[right], data[left]
		left++
		right--
	}
}

func requestIncompleteBar(conn *utils.Conn, ticker string, timestamp int64, multiplier int, timespan string, extendedHours bool, isReplay bool, easternLocation *time.Location) (GetChartDataResults, error) {
	var incompleteBar GetChartDataResults
	timestampEnd := timestamp
	if timestamp == 0 {
		timestampEnd = time.Now().UnixMilli()
	}
	timestampTime := time.Unix(0, timestampEnd*int64(time.Millisecond)).UTC()
	var timestampStart int64
	var currentDayStart int64
	if timespan == "second" || timespan == "minute" || timespan == "hour" {
		currentDayStart = utils.GetReferenceStartTime(timestampEnd, extendedHours, easternLocation)
		timeframeInSeconds := utils.GetTimeframeInSeconds(multiplier, timespan)
		elapsedTime := timestampEnd - currentDayStart
		fmt.Printf("\nElapsed Time:%v", elapsedTime)
		if elapsedTime < 0 {
			return incompleteBar, nil
		}
		timestampStart = currentDayStart + (elapsedTime/(timeframeInSeconds*1000))*timeframeInSeconds*1000
	} else {
		currentDayStart = utils.GetReferenceStartTime(timestampEnd, false, easternLocation)
		if timespan == "day" {
			timestampStart = utils.GetReferenceStartTimeForDays(timestampEnd, multiplier, easternLocation)
		} else if timespan == "week" {
			timestampStart = utils.GetReferenceStartTimeForWeeks(timestampEnd, multiplier, easternLocation)
		} else if timespan == "month" {
			timestampStart = utils.GetReferenceStartTimeForMonths(timestampEnd, multiplier, easternLocation)
		}
	}
	fmt.Printf("\nTimestampStart:%v", timestampStart)
	incompleteBar.Timestamp = math.Floor(float64(timestampStart) / 1000)
	if timespan == "day" || timespan == "week" || timespan == "month" {
		lastCompleteDayUTC := time.Date(timestampTime.Year(), timestampTime.Month(), timestampTime.Day(), 0, 0, 0, 0, time.UTC).UnixMilli()
		dailyBarsDurationMs := lastCompleteDayUTC - timestampStart
		numDailyBars := int(dailyBarsDurationMs / (86400000))
		if numDailyBars > 0 {
			iter, err := utils.GetAggsData(conn.Polygon, ticker, 1, "day",
				models.Millis(time.Unix(0, timestampStart*int64(time.Millisecond)).UTC()),
				models.Millis(time.Unix(0, (lastCompleteDayUTC-86400000)*int64(time.Millisecond)).UTC()),
				10000, "asc", !isReplay)
			if err != nil {
				return incompleteBar, fmt.Errorf("error creating incomplete bar 64krjglvk %v", err)
			}
			var count int
			for iter.Next() {
				if count >= numDailyBars {
					break
				}
				if incompleteBar.Open == 0 && iter.Item().Open != 0 {
					fmt.Println(iter.Item().Open)
					incompleteBar.Open = iter.Item().Open
				}
				if iter.Item().High > incompleteBar.High {
					incompleteBar.High = iter.Item().High
				}
				if iter.Item().Low < incompleteBar.Low || incompleteBar.Low == 0 {
					incompleteBar.Low = iter.Item().Low
				}
				incompleteBar.Close = iter.Item().Close
				incompleteBar.Volume += iter.Item().Volume
				count++
			}
		}
		timestampStart = currentDayStart
	}
	if timespan != "second" {
		lastCompleteMinuteUTC := timestampTime.Truncate(time.Minute).UnixMilli()
		if !extendedHours || timespan == "day" || timespan == "week" || timespan == "month" {
			marketClose := time.Date(timestampTime.Year(), timestampTime.Month(), timestampTime.Day(), 16, 0, 0, 0, easternLocation)
			if timestampTime.After(marketClose) {
				lastCompleteMinuteUTC = time.Date(timestampTime.Year(), timestampTime.Month(), timestampTime.Day(), 15, 59, 0, 0, easternLocation).UnixMilli()
			}
		}
		minuteBarsEndTimeUTC := lastCompleteMinuteUTC
		if lastCompleteMinuteUTC <= timestampStart {
			minuteBarsEndTimeUTC = timestampStart
		}
		numMinuteBars := (minuteBarsEndTimeUTC - timestampStart) / (60 * 1000)
		fmt.Printf("numMinuteBars: %d\n", numMinuteBars)
		if numMinuteBars > 0 {
			iter, err := utils.GetAggsData(conn.Polygon, ticker, 1, "minute",
				models.Millis(time.Unix(0, timestampStart*int64(time.Millisecond)).UTC()),
				models.Millis(time.Unix(0, (lastCompleteMinuteUTC-60000)*int64(time.Millisecond)).UTC()),
				10000, "asc", !isReplay)
			if err != nil {
				return incompleteBar, fmt.Errorf("error while pulling minute data for incomplete bar 56kly7lg %v", err)
			}
			var count int64
			for iter.Next() {
				if count >= numMinuteBars {
					break
				}
				timestamp := time.Time(iter.Item().Timestamp).In(easternLocation)
				if !utils.IsTimestampRegularHours(timestamp) && !extendedHours {
					continue
				}
				if incompleteBar.Open == 0 {
					incompleteBar.Open = iter.Item().Open
				}
				if iter.Item().High > incompleteBar.High {
					incompleteBar.High = iter.Item().High
				}
				if iter.Item().Low < incompleteBar.Low || incompleteBar.Low == 0 {
					incompleteBar.Low = iter.Item().Low
				}
				incompleteBar.Close = iter.Item().Close
				incompleteBar.Volume += iter.Item().Volume
				count++
			}
		}
		timestampStart = lastCompleteMinuteUTC
	}
	lastCompleteSecondUTC := timestampTime.Truncate(time.Second).UnixMilli()
	secondBarsEndTimeUTC := lastCompleteSecondUTC
	if lastCompleteSecondUTC <= timestampStart {
		secondBarsEndTimeUTC = timestampStart
	}
	numSecondBars := (secondBarsEndTimeUTC - timestampStart) / 1000
	fmt.Printf("\nnumSecondBars: %d\n", numSecondBars)
	fmt.Printf("\nstarttime:%v", time.Unix(0, timestampStart*int64(time.Millisecond)).UTC().UnixMilli())
	fmt.Printf("\nendtime:%v", time.Unix(0, (lastCompleteSecondUTC)*int64(time.Millisecond)).UTC().UnixMilli())
	if numSecondBars > 0 {
		iter, err := utils.GetAggsData(conn.Polygon, ticker, 1, "second",
			models.Millis(time.Unix(0, timestampStart*int64(time.Millisecond)).UTC()),
			models.Millis(time.Unix(0, (lastCompleteSecondUTC)*int64(time.Millisecond)).UTC()),
			10000, "asc", !isReplay)
		if err != nil {
			return incompleteBar, fmt.Errorf("error while pulling minute data for incomplete bar 56kly7lg %v", err)
		}
		var count int64
		for iter.Next() {
			if count >= numSecondBars {
				break
			}
			timestamp := time.Time(iter.Item().Timestamp).In(easternLocation)
			if !utils.IsTimestampRegularHours(timestamp) && !extendedHours {
				continue
			}
			if incompleteBar.Open == 0 {
				incompleteBar.Open = iter.Item().Open
			}
			if iter.Item().High > incompleteBar.High {
				incompleteBar.High = iter.Item().High
			}
			if iter.Item().Low < incompleteBar.Low || incompleteBar.Low == 0 {
				incompleteBar.Low = iter.Item().Low
			}
			incompleteBar.Close = iter.Item().Close
			incompleteBar.Volume += iter.Item().Volume
			count++
		}
		fmt.Printf("COUNT:%v", count)
	}
	var tradeConditionsToCheck = map[int32]struct{}{
		2: {}, 5: {}, 10: {}, 15: {}, 16: {},
		20: {}, 21: {}, 22: {}, 29: {}, 33: {},
		38: {}, 52: {}, 53: {},
	}
	var volumeConditionsToCheck = map[int32]struct{}{
		15: {}, 16: {}, 38: {},
	}
	timestampStart = lastCompleteSecondUTC
	iter, err := utils.GetTrade(conn.Polygon, ticker, models.Nanos(time.Unix(timestampStart/1000, (timestampStart%1000*1e6)).UTC()),
		"asc", models.GTE, 30000)
	if err != nil {
		return incompleteBar, fmt.Errorf("error pulling tick data 56l5kykgk, %v", err)
	}
	for iter.Next() {
		if time.Time(iter.Item().ParticipantTimestamp).UnixMilli() > timestampEnd {
			break
		}
		foundCondition := false
		for _, condition := range iter.Item().Conditions {
			if _, found := tradeConditionsToCheck[condition]; found {
				foundCondition = true
				break
			}
		}
		if !foundCondition {
			if incompleteBar.Open == 0 {
				incompleteBar.Open = iter.Item().Price
			}
			if iter.Item().Price > incompleteBar.High {
				incompleteBar.High = iter.Item().Price
			} else if iter.Item().Price < incompleteBar.Low || incompleteBar.Low == 0 {
				incompleteBar.Low = iter.Item().Price
			}
			incompleteBar.Close = iter.Item().Price
		}
		foundTradeCondition := false
		for _, condition := range iter.Item().Conditions {
			if _, found := volumeConditionsToCheck[condition]; found {
				foundTradeCondition = true
				break
			}
		}
		if !foundTradeCondition {
			incompleteBar.Volume += iter.Item().Size
		}
	}
	return incompleteBar, nil
}

func buildHigherTimeframeFromLower(iter *iter.Iter[models.Agg], multiplier int, timespan string, extendedHours bool, easternLocation *time.Location, numBarsRemaining *int, direction string) ([]GetChartDataResults, error) {
	var barDataList []GetChartDataResults
	var currentBar GetChartDataResults
	var barStartTime time.Time

	b := 0

	for iter.Next() {
		item := iter.Item()
		err := iter.Err()
		if err != nil {
			return nil, fmt.Errorf("din0wi %v", err)
		}
		timestamp := time.Time(item.Timestamp).In(easternLocation)
		if extendedHours || (utils.IsTimestampRegularHours(timestamp)) {
			diff := timestamp.Sub(barStartTime)
			if barStartTime.IsZero() || diff >= time.Duration(multiplier)*utils.TimespanStringToDuration(timespan) {
				if !barStartTime.IsZero() {
					barDataList = append(barDataList, currentBar)
					if direction == "forwards" {
						*numBarsRemaining--
						if *numBarsRemaining <= 0 {
							break
						}
					}
				}
				currentBar = GetChartDataResults{
					Timestamp: float64(timestamp.Unix()),
					Open:      item.Open,
					High:      item.High,
					Low:       item.Low,
					Close:     item.Close,
					Volume:    item.Volume,
				}
				barStartTime = timestamp
			} else {
				currentBar.High = max(currentBar.High, item.High)
				currentBar.Low = min(currentBar.Low, item.Low)
				currentBar.Close = item.Close
				currentBar.Volume += item.Volume
			}
		}
		b++
	}
	if direction == "forwards" {
		if !barStartTime.IsZero() && *numBarsRemaining > 0 {
			barDataList = append(barDataList, currentBar)
			*numBarsRemaining--
		}
	} else {
		barsToKeep := len(barDataList) - *numBarsRemaining
		if barsToKeep < 0 {
			barsToKeep = 0
			*numBarsRemaining -= len(barDataList)
		} else {
			*numBarsRemaining = 0
		}
		barDataList = barDataList[barsToKeep:]
	}

	return barDataList, nil
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
