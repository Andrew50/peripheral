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

var debug = false // Flip to `true` to enable verbose debugging output

func MaxDivisorOf30(n int) int {
	for k := n; k >= 1; k-- {
		if 30%k == 0 && n%k == 0 {
			return k
		}
	}
	return 1
}

func GetChartData(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetChartDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	if debug {
		fmt.Printf("[DEBUG] GetChartData: SecurityId=%d, Timeframe=%s, Direction=%s\n",
			args.SecurityId, args.Timeframe, args.Direction)
	}

	multiplier, timespan, _, _, err := utils.GetTimeFrame(args.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("invalid timeframe: %v", err)
	}
	if debug {
		fmt.Printf("[DEBUG] Parsed timeframe => multiplier=%d, timespan=%s\n", multiplier, timespan)
	}

	// Determine if we must build a higher TF from a lower TF
	var queryTimespan string
	var queryMultiplier int
	var queryBars int
	var tickerForIncompleteAggregate string
	haveToAggregate := false

	// Special logic for second/minute frames with 30-based constraints
	if (timespan == "second" || timespan == "minute") && (30%multiplier != 0) {
		queryTimespan = timespan
		// Overriding the typical logic with direct 1-min aggregator
		// (The original code forcibly sets queryMultiplier=1 anyway)
		_ = MaxDivisorOf30(multiplier)
		queryMultiplier = 1
		queryBars = args.Bars * multiplier / queryMultiplier
		haveToAggregate = true
	} else if timespan == "hour" {
		// Hour -> 30-min aggregator
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

	// For timeframes above day, there's no extended hours
	if timespan != "minute" && timespan != "second" && timespan != "hour" {
		args.ExtendedHours = false
	}

	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}

	// Convert the incoming timestamp (in ms) to time.Time
	var inputTimestamp time.Time
	if args.Timestamp == 0 {
		// If no timestamp provided, we assume "now" in Eastern Time
		inputTimestamp = time.Now().In(easternLocation)
	} else {
		inputTimestamp = time.Unix(args.Timestamp/1000, (args.Timestamp%1000)*1e6).UTC()
	}

	// Build the DB query depending on direction/timestamp
	var query string
	var queryParams []interface{}
	var polyResultOrder string

	switch {
	case args.Timestamp == 0:
		query = `SELECT ticker, minDate, maxDate 
                 FROM securities 
                 WHERE securityid = $1
                 ORDER BY minDate DESC NULLS FIRST`
		queryParams = []interface{}{args.SecurityId}
		polyResultOrder = "desc"
	case args.Direction == "backward":
		query = `SELECT ticker, minDate, maxDate
                 FROM securities 
                 WHERE securityid = $1 AND (maxDate > $2 OR maxDate IS NULL)
                 ORDER BY minDate DESC NULLS FIRST LIMIT 1`
		queryParams = []interface{}{args.SecurityId, inputTimestamp}
		polyResultOrder = "desc"
	case args.Direction == "forward":
		query = `SELECT ticker, minDate, maxDate
                 FROM securities 
                 WHERE securityid = $1 AND (minDate < $2 OR minDate IS NULL)
                 ORDER BY minDate ASC NULLS LAST`
		queryParams = []interface{}{args.SecurityId, inputTimestamp}
		polyResultOrder = "asc"
	default:
		return nil, fmt.Errorf("Incorrect direction passed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, query, queryParams...)
	if err != nil {
		if debug {
			fmt.Printf("[DEBUG] Database query failed: %v\n", err)
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("error querying data: %w", err)
	}
	defer rows.Close()

	// Preallocate capacity for bar data. We'll at most fetch up to args.Bars + small overhead
	barDataList := make([]GetChartDataResults, 0, args.Bars+10)
	numBarsRemaining := args.Bars

	if debug {
		fmt.Printf("[DEBUG] Processing rows from DB...\n")
	}

	for rows.Next() {
		var ticker string
		var minDateFromSQL, maxDateFromSQL *time.Time

		if err := rows.Scan(&ticker, &minDateFromSQL, &maxDateFromSQL); err != nil {
			if debug {
				fmt.Printf("[DEBUG] Error scanning row: %v\n", err)
			}
			return nil, fmt.Errorf("error scanning data: %w", err)
		}
		tickerForIncompleteAggregate = ticker

		// Handle NULL maxDate
		if maxDateFromSQL == nil {
			now := time.Now()
			maxDateFromSQL = &now
		}

		// If minDate is nil, pick a minimal date
		var minDateSQL time.Time
		if minDateFromSQL != nil {
			minDateSQL = minDateFromSQL.In(easternLocation)
		} else {
			minDateSQL = time.Date(1970, 1, 1, 0, 0, 0, 0, easternLocation)
		}
		maxDateSQL := maxDateFromSQL.In(easternLocation)

		var queryStartTime, queryEndTime time.Time
		switch args.Direction {
		case "backward":
			queryEndTime = inputTimestamp
			if maxDateSQL.Before(queryEndTime) {
				queryEndTime = maxDateSQL
			}
			queryStartTime = minDateSQL
			if queryStartTime.After(queryEndTime) {
				return nil, fmt.Errorf("i10i0v")
			}
		case "forward":
			queryStartTime = inputTimestamp
			if minDateSQL.After(queryStartTime) {
				queryStartTime = minDateSQL
			}
			queryEndTime = maxDateSQL
			if queryEndTime.Before(queryStartTime) {
				continue
			}
		}

		date1, date2, err := utils.GetRequestStartEndTime(
			queryStartTime, queryEndTime, args.Direction, timespan, multiplier, queryBars,
		)
		if err != nil {
			if debug {
				fmt.Printf("[DEBUG] GetRequestStartEndTime failed: %v\n", err)
			}
			return nil, fmt.Errorf("dkn0 %v", err)
		}

		if debug {
			fmt.Printf("[DEBUG] Polygon request for %s: start=%v end=%v aggregator=%v\n",
				ticker, date1, date2, haveToAggregate)
		}

		// If we have to aggregate (e.g., second->minute, or minute->hour), do so
		if haveToAggregate {
			it, err := utils.GetAggsData(
				conn.Polygon,
				ticker,
				queryMultiplier,
				queryTimespan,
				date1, date2,
				50000,
				// For intraday backward queries we typically want ascending data
				// to handle reaggregation easily (original code used "asc" for aggregator).
				"asc",
				!args.IsReplay,
			)
			if err != nil {
				return nil, fmt.Errorf("error fetching data from Polygon: %v", err)
			}

			aggregatedData, err := buildHigherTimeframeFromLower(
				it, multiplier, timespan, args.ExtendedHours, easternLocation, &numBarsRemaining, args.Direction,
			)
			if err != nil {
				return nil, err
			}
			barDataList = append(barDataList, aggregatedData...)
			if numBarsRemaining <= 0 {
				break
			}
		} else {
			// Otherwise, we can directly pull from Polygon at the desired timeframe
			it, err := utils.GetAggsData(
				conn.Polygon,
				ticker,
				queryMultiplier,
				queryTimespan,
				date1,
				date2,
				50000,
				polyResultOrder,
				!args.IsReplay,
			)
			if err != nil {
				if debug {
					fmt.Printf("[DEBUG] Polygon API error: %v\n", err)
				}
				return nil, fmt.Errorf("error fetching data from Polygon: %v", err)
			}

			for it.Next() {
				item := it.Item()
				if it.Err() != nil {
					if debug {
						fmt.Printf("[DEBUG] Iterator error: %v\n", it.Err())
					}
					return nil, fmt.Errorf("dkn0w")
				}

				ts := time.Time(item.Timestamp).In(easternLocation)
				// Skip weekends for big timespans
				if queryTimespan == "week" || queryTimespan == "month" || queryTimespan == "year" {
					if ts.Weekday() == time.Saturday || ts.Weekday() == time.Sunday {
						continue
					}
				}
				// Skip out of hours if not extended hours
				if (timespan == "minute" || timespan == "second" || timespan == "hour") &&
					!args.ExtendedHours && !utils.IsTimestampRegularHours(ts) {
					continue
				}

				barDataList = append(barDataList, GetChartDataResults{
					Timestamp: float64(ts.Unix()),
					Open:      item.Open,
					High:      item.High,
					Low:       item.Low,
					Close:     item.Close,
					Volume:    item.Volume,
				})

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

	// If we have some bars, reverse them if needed, then optionally fetch incomplete bar
	if len(barDataList) > 0 {
		// For aggregator logic or forward queries, we skip reversing.
		if haveToAggregate || args.Direction == "forward" {
			if args.Direction == "backward" {
				// Potentially fetch incomplete bar if market is open or in replay
				marketStatus, err := utils.GetMarketStatus(conn)
				if err != nil {
					return nil, fmt.Errorf("issue with market status")
				}

				if (args.Timestamp == 0 && marketStatus != "closed") || args.IsReplay {
					incompleteAgg, err := requestIncompleteBar(
						conn,
						tickerForIncompleteAggregate,
						args.Timestamp,
						multiplier,
						timespan,
						args.ExtendedHours,
						args.IsReplay,
						easternLocation,
					)
					if err != nil {
						return nil, fmt.Errorf("issue with incomplete aggregate: %v", err)
					}

					if len(barDataList) > 0 &&
						incompleteAgg.Timestamp == barDataList[len(barDataList)-1].Timestamp {
						barDataList = barDataList[:len(barDataList)-1]
					}
					if incompleteAgg.Open != 0 {
						barDataList = append(barDataList, incompleteAgg)
					}
				}
			}
			return barDataList, nil
		}

		// Otherwise, direction=backward with direct data—reverse the slice
		reverse(barDataList)

		// Possibly append incomplete bar
		marketStatus, err := utils.GetMarketStatus(conn)
		if err != nil {
			return nil, fmt.Errorf("issue with market status")
		}
		if (args.Timestamp == 0 && marketStatus != "closed") || args.IsReplay {
			incompleteAgg, err := requestIncompleteBar(
				conn,
				tickerForIncompleteAggregate,
				args.Timestamp,
				multiplier,
				timespan,
				args.ExtendedHours,
				args.IsReplay,
				easternLocation,
			)
			if err != nil {
				return nil, fmt.Errorf("issue with incomplete aggregate: %v", err)
			}
			if len(barDataList) > 0 &&
				incompleteAgg.Timestamp == barDataList[len(barDataList)-1].Timestamp {
				barDataList = barDataList[:len(barDataList)-1]
			}
			if incompleteAgg.Open != 0 {
				// Only add incomplete bar if it's within regular hours or daily+ timeframes
				incompleteTs := time.Unix(int64(incompleteAgg.Timestamp), 0)
				if (utils.IsTimestampRegularHours(incompleteTs) && !args.ExtendedHours) ||
					timespan == "day" || timespan == "week" || timespan == "month" {
					barDataList = append(barDataList, incompleteAgg)
				}
			}
		}
		return barDataList, nil
	}

	if debug {
		fmt.Printf("[DEBUG] No data found. numBarsRemaining=%d\n", numBarsRemaining)
	}
	return nil, fmt.Errorf("no data found")
}

func reverse(data []GetChartDataResults) {
	for left, right := 0, len(data)-1; left < right; {
		data[left], data[right] = data[right], data[left]
		left++
		right--
	}
}

// requestIncompleteBar attempts to build a partial bar for the current bar in progress
func requestIncompleteBar(
	conn *utils.Conn,
	ticker string,
	timestamp int64,
	multiplier int,
	timespan string,
	extendedHours bool,
	isReplay bool,
	easternLocation *time.Location,
) (GetChartDataResults, error) {

	var incompleteBar GetChartDataResults
	timestampEnd := timestamp
	if timestamp == 0 {
		timestampEnd = time.Now().UnixMilli()
	}

	timestampTime := time.Unix(timestampEnd/1000, (timestampEnd%1000)*1e6).UTC()
	var timestampStart int64
	var currentDayStart int64

	// Intraday logic
	if timespan == "second" || timespan == "minute" || timespan == "hour" {
		currentDayStart = utils.GetReferenceStartTime(timestampEnd, extendedHours, easternLocation)
		timeframeInSeconds := utils.GetTimeframeInSeconds(multiplier, timespan)
		elapsedTime := timestampEnd - currentDayStart
		if elapsedTime < 0 {
			return incompleteBar, nil
		}
		// Snap to boundary
		timestampStart = currentDayStart + (elapsedTime/(timeframeInSeconds*1000))*timeframeInSeconds*1000
	} else {
		// Daily or above
		currentDayStart = utils.GetReferenceStartTime(timestampEnd, false, easternLocation)
		switch timespan {
		case "day":
			timestampStart = utils.GetReferenceStartTimeForDays(timestampEnd, multiplier, easternLocation)
		case "week":
			timestampStart = utils.GetReferenceStartTimeForWeeks(timestampEnd, multiplier, easternLocation)
		case "month":
			timestampStart = utils.GetReferenceStartTimeForMonths(timestampEnd, multiplier, easternLocation)
		}
	}

	incompleteBar.Timestamp = math.Floor(float64(timestampStart) / 1000.0)

	// For daily or above, try to combine daily bars from open to "yesterday"
	if timespan == "day" || timespan == "week" || timespan == "month" {
		lastCompleteDayUTC := time.Date(
			timestampTime.Year(), timestampTime.Month(), timestampTime.Day(),
			0, 0, 0, 0, time.UTC,
		).UnixMilli()
		dailyBarsDurationMs := lastCompleteDayUTC - timestampStart
		numDailyBars := int(dailyBarsDurationMs / 86400000) // ms in a day
		if numDailyBars > 0 {
			startT := models.Millis(time.Unix(0, timestampStart*int64(time.Millisecond)).UTC())
			endT := models.Millis(time.Unix(0, (lastCompleteDayUTC-86400000)*int64(time.Millisecond)).UTC())

			it, err := utils.GetAggsData(conn.Polygon, ticker, 1, "day", startT, endT, 10000, "asc", !isReplay)
			if err != nil {
				return incompleteBar, fmt.Errorf("error creating incomplete bar (daily) %v", err)
			}

			var count int
			for it.Next() {
				if count >= numDailyBars {
					break
				}
				agg := it.Item()
				if incompleteBar.Open == 0 && agg.Open != 0 {
					incompleteBar.Open = agg.Open
				}
				if agg.High > incompleteBar.High {
					incompleteBar.High = agg.High
				}
				if agg.Low < incompleteBar.Low || incompleteBar.Low == 0 {
					incompleteBar.Low = agg.Low
				}
				incompleteBar.Close = agg.Close
				incompleteBar.Volume += agg.Volume
				count++
			}
		}
		timestampStart = currentDayStart
	}

	// For minute data
	if timespan != "second" {
		lastCompleteMinuteUTC := timestampTime.Truncate(time.Minute).UnixMilli()
		if !extendedHours || timespan == "day" || timespan == "week" || timespan == "month" {
			marketClose := time.Date(
				timestampTime.Year(), timestampTime.Month(), timestampTime.Day(),
				16, 0, 0, 0, easternLocation,
			)
			// If time is after market close, we roll back to the last minute before 16:00
			if timestampTime.After(marketClose) {
				lastCompleteMinuteUTC = time.Date(
					timestampTime.Year(), timestampTime.Month(), timestampTime.Day(),
					15, 59, 0, 0, easternLocation,
				).UnixMilli()
			}
		}
		minuteBarsEndTimeUTC := lastCompleteMinuteUTC
		if minuteBarsEndTimeUTC <= timestampStart {
			minuteBarsEndTimeUTC = timestampStart
		}
		numMinuteBars := (minuteBarsEndTimeUTC - timestampStart) / (60 * 1000)
		if debug {
			fmt.Printf("[DEBUG] # of minute bars to fetch: %d\n", numMinuteBars)
		}

		if numMinuteBars > 0 {
			startT := models.Millis(time.Unix(0, timestampStart*int64(time.Millisecond)).UTC())
			endT := models.Millis(time.Unix(0, (lastCompleteMinuteUTC-60000)*int64(time.Millisecond)).UTC())

			it, err := utils.GetAggsData(conn.Polygon, ticker, 1, "minute", startT, endT, 10000, "asc", !isReplay)
			if err != nil {
				return incompleteBar, fmt.Errorf("error while pulling minute data for incomplete bar: %v", err)
			}

			var count int64
			for it.Next() {
				if count >= numMinuteBars {
					break
				}
				agg := it.Item()
				barTs := time.Time(agg.Timestamp).In(easternLocation)
				if !utils.IsTimestampRegularHours(barTs) && !extendedHours {
					continue
				}
				if incompleteBar.Open == 0 {
					incompleteBar.Open = agg.Open
				}
				if agg.High > incompleteBar.High {
					incompleteBar.High = agg.High
				}
				if agg.Low < incompleteBar.Low || incompleteBar.Low == 0 {
					incompleteBar.Low = agg.Low
				}
				incompleteBar.Close = agg.Close
				incompleteBar.Volume += agg.Volume
				count++
			}
		}
		timestampStart = lastCompleteMinuteUTC
	}

	// For second data
	lastCompleteSecondUTC := timestampTime.Truncate(time.Second).UnixMilli()
	secondBarsEndTimeUTC := lastCompleteSecondUTC
	if secondBarsEndTimeUTC <= timestampStart {
		secondBarsEndTimeUTC = timestampStart
	}
	numSecondBars := (secondBarsEndTimeUTC - timestampStart) / 1000
	if debug {
		fmt.Printf("[DEBUG] # of second bars to fetch: %d\n", numSecondBars)
	}

	if numSecondBars > 0 {
		startT := models.Millis(time.Unix(0, timestampStart*int64(time.Millisecond)).UTC())
		endT := models.Millis(time.Unix(0, lastCompleteSecondUTC*int64(time.Millisecond)).UTC())

		it, err := utils.GetAggsData(conn.Polygon, ticker, 1, "second", startT, endT, 10000, "asc", !isReplay)
		if err != nil {
			return incompleteBar, fmt.Errorf("error while pulling second data for incomplete bar: %v", err)
		}

		var count int64
		for it.Next() {
			if count >= numSecondBars {
				break
			}
			agg := it.Item()
			barTs := time.Time(agg.Timestamp).In(easternLocation)
			if !extendedHours && !utils.IsTimestampRegularHours(barTs) {
				continue
			}
			if incompleteBar.Open == 0 {
				incompleteBar.Open = agg.Open
			}
			if agg.High > incompleteBar.High {
				incompleteBar.High = agg.High
			}
			if agg.Low < incompleteBar.Low || incompleteBar.Low == 0 {
				incompleteBar.Low = agg.Low
			}
			incompleteBar.Close = agg.Close
			incompleteBar.Volume += agg.Volume
			count++
		}
		timestampStart = lastCompleteSecondUTC
	}

	// For trade-level data
	tradeConditionsToCheck := map[int32]struct{}{
		2: {}, 5: {}, 10: {}, 15: {}, 16: {}, 20: {}, 21: {}, 22: {}, 29: {}, 33: {}, 38: {}, 52: {}, 53: {},
	}
	volumeConditionsToCheck := map[int32]struct{}{
		15: {}, 16: {}, 38: {},
	}

	startNanos := models.Nanos(time.Unix(timestampStart/1000, (timestampStart%1000)*1e6).UTC())
	it, err := utils.GetTrade(conn.Polygon, ticker, startNanos, "asc", models.GTE, 30000)
	if err != nil {
		return incompleteBar, fmt.Errorf("error pulling tick data: %v", err)
	}

	endUnix := time.Unix(0, timestampEnd*int64(time.Millisecond)).UTC()
	for it.Next() {
		trade := it.Item()
		// Check if the trade is empty by looking at the Price field
		if trade.Price == 0 {
			continue
		}

		tradeTs := time.Time(trade.ParticipantTimestamp).In(easternLocation)
		// Stop if we move past the target end
		if tradeTs.After(endUnix) {
			break
		}
		if !extendedHours && !utils.IsTimestampRegularHours(tradeTs) {
			break
		}

		// Skip if condition blacklists it from affecting O/H/L/C
		skipOhlc := false
		for _, condition := range trade.Conditions {
			if _, found := tradeConditionsToCheck[condition]; found {
				skipOhlc = true
				break
			}
		}
		if !skipOhlc {
			if incompleteBar.Open == 0 {
				incompleteBar.Open = trade.Price
			}
			if trade.Price > incompleteBar.High {
				incompleteBar.High = trade.Price
			} else if trade.Price < incompleteBar.Low || incompleteBar.Low == 0 {
				incompleteBar.Low = trade.Price
			}
			incompleteBar.Close = trade.Price
		}

		// Only add volume if not in volume condition skip set
		skipVol := false
		for _, condition := range trade.Conditions {
			if _, found := volumeConditionsToCheck[condition]; found {
				skipVol = true
				break
			}
		}
		if !skipVol {
			incompleteBar.Volume += trade.Size
		}
	}

	return incompleteBar, nil
}

// buildHigherTimeframeFromLower re-aggregates smaller timeframes into a bigger timeframe.
func buildHigherTimeframeFromLower(
	it *iter.Iter[models.Agg],
	multiplier int,
	timespan string,
	extendedHours bool,
	easternLocation *time.Location,
	numBarsRemaining *int,
	direction string,
) ([]GetChartDataResults, error) {

	var barDataList []GetChartDataResults
	barDataList = make([]GetChartDataResults, 0, *numBarsRemaining+10) // Prealloc

	var currentBar GetChartDataResults
	var barStartTime time.Time

	// Convert the "one unit" (minute/second) into time.Duration for aggregator logic
	unitDuration := utils.TimespanStringToDuration(timespan)
	// The length of each bar in lower timeframe
	requiredDuration := time.Duration(multiplier) * unitDuration

	for it.Next() {
		agg := it.Item()
		if it.Err() != nil {
			return nil, fmt.Errorf("iterator error: %v", it.Err())
		}
		timestamp := time.Time(agg.Timestamp).In(easternLocation)

		// Filter out pre/post market if not extended
		if extendedHours || utils.IsTimestampRegularHours(timestamp) {
			diff := timestamp.Sub(barStartTime)

			// If we're starting or we've exceeded the timeslice for the bar
			if barStartTime.IsZero() || diff >= requiredDuration {
				// If we have a bar in progress, store it
				if !barStartTime.IsZero() {
					barDataList = append(barDataList, currentBar)
					*numBarsRemaining--
					if *numBarsRemaining <= 0 {
						break
					}
				}
				// Start a new bar
				currentBar = GetChartDataResults{
					Timestamp: float64(timestamp.Unix()),
					Open:      agg.Open,
					High:      agg.High,
					Low:       agg.Low,
					Close:     agg.Close,
					Volume:    agg.Volume,
				}
				barStartTime = timestamp
			} else {
				// Continue aggregating into the current bar
				if agg.High > currentBar.High {
					currentBar.High = agg.High
				}
				if agg.Low < currentBar.Low {
					currentBar.Low = agg.Low
				}
				currentBar.Close = agg.Close
				currentBar.Volume += agg.Volume
			}
		}
	}

	// Flush last bar if needed
	// For forward direction, we add the last bar
	if direction == "forward" {
		if !barStartTime.IsZero() && *numBarsRemaining > 0 {
			barDataList = append(barDataList, currentBar)
			*numBarsRemaining--
		}
	} else {
		// For backward direction, we may need to slice off extra bars
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
