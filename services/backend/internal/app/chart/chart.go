package chart

import (
	"backend/internal/data"
	"backend/internal/data/utils"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"backend/internal/data/polygon"

	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

// GetChartDataArgs represents a structure for handling GetChartDataArgs data.
type GetChartDataArgs struct {
	SecurityID        int    `json:"securityId"`
	Timeframe         string `json:"timeframe"`
	Timestamp         int64  `json:"timestamp"`
	Direction         string `json:"direction"`
	Bars              int    `json:"bars"`
	ExtendedHours     bool   `json:"extendedHours"`
	IsReplay          bool   `json:"isreplay"`
	IncludeSECFilings bool   `json:"includeSECFilings,omitempty"`
}

// GetChartDataResults represents a structure for handling GetChartDataResults data.
type GetChartDataResults struct {
	Timestamp float64 `json:"time"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	Events    []Event `json:"events"`
}

// GetChartDataResponse represents a structure for handling GetChartDataResponse data.
type GetChartDataResponse struct {
	Bars           []GetChartDataResults `json:"bars"`
	IsEarliestData bool                  `json:"isEarliestData"`
}

// MaxDivisorOf30 returns the largest integer k such that k divides n and k also divides 30.
func MaxDivisorOf30(n int) int {
	for k := n; k >= 1; k-- {
		if 30%k == 0 && n%k == 0 {
			return k
		}
	}
	return 1
}

// GetChartData performs operations related to GetChartData functionality.
func GetChartData(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetChartDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// For public access (userID=0), disable premium features
	if userID == 0 {
		args.IncludeSECFilings = false
	}

	//	if debug {
	////fmt.Printf("[DEBUG] GetChartData: SecurityID=%d, Timeframe=%s, Direction=%s\n", args.SecurityID, args.Timeframe, args.Direction)
	//	}

	multiplier, timespan, _, _, err := GetTimeFrame(args.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("invalid timeframe: %v", err)
	}
	//if debug {
	////fmt.Printf("[DEBUG] Parsed timeframe => multiplier=%d, timespan=%s\n", multiplier, timespan)
	//}
	// Determine if we must build a higher TF from a lower TF
	var queryTimespan string
	var queryMultiplier int
	var queryBars int
	var tickerForIncompleteAggregate string
	var numBarsRequestedPolygon int
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
		query = `SELECT ticker, minDate, maxDate, false as has_earlier_data
                 FROM securities 
                 WHERE securityid = $1
                 ORDER BY maxDate DESC NULLS FIRST`
		queryParams = []interface{}{args.SecurityID}
		polyResultOrder = "desc"
	case args.Direction == "backward":
		// Use a conservative buffer to account for potential gaps between input timestamp
		// and actual bar data (market hours, holidays, etc.)
		bufferTime := inputTimestamp.Add(-72 * time.Hour) // 24-hour buffer for safety
		query = `SELECT ticker, minDate, maxDate,
                        EXISTS(SELECT 1 FROM securities s2 WHERE s2.securityid = $1 AND s2.minDate < $2) as has_earlier_data
                 FROM securities 
                 WHERE securityid = $1 AND (maxDate > $3 OR maxDate IS NULL)
                 ORDER BY minDate DESC NULLS FIRST LIMIT 1`
		queryParams = []interface{}{args.SecurityID, bufferTime, inputTimestamp}
		polyResultOrder = "desc"
	case args.Direction == "forward":
		query = `SELECT ticker, minDate, maxDate, false as has_earlier_data
                 FROM securities 
                 WHERE securityid = $1 AND (minDate < $2 OR minDate IS NULL)
                 ORDER BY minDate ASC NULLS LAST`
		queryParams = []interface{}{args.SecurityID, inputTimestamp}
		polyResultOrder = "asc"
	default:
		return nil, fmt.Errorf("incorrect direction passed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, query, queryParams...)
	if err != nil {
		//if debug {
		////fmt.Printf("[DEBUG] Database query failed: %v\n", err)
		//}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("error querying data: %w", err)
	}
	defer rows.Close()

	// Define a struct to hold the security data from the initial query
	type securityRecord struct {
		ticker         string
		minDateFromSQL *time.Time
		maxDateFromSQL *time.Time
		hasEarlierData bool
	}

	// Read all security records into a slice to get the count first
	var securityRecords []securityRecord
	for rows.Next() {
		var record securityRecord
		if err := rows.Scan(&record.ticker, &record.minDateFromSQL, &record.maxDateFromSQL, &record.hasEarlierData); err != nil {
			//if debug {
			////fmt.Printf("[DEBUG] Error scanning security record row: %v\n", err)
			//}
			return nil, fmt.Errorf("error scanning security data: %w", err)
		}
		securityRecords = append(securityRecords, record)
	}
	// Check for errors during row iteration
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating security data rows: %w", err)
	}
	rows.Close() // Close rows immediately after reading

	// Filter out security records that are more than a year before the most recent min date
	// This is especially useful for backward requests to avoid fetching very old data
	if args.Direction == "backward" && len(securityRecords) > 1 {
		// Find the most recent minDate among all records
		var mostRecentMinDate *time.Time
		for _, record := range securityRecords {
			if record.minDateFromSQL != nil {
				if mostRecentMinDate == nil || record.minDateFromSQL.After(*mostRecentMinDate) {
					mostRecentMinDate = record.minDateFromSQL
				}
			}
		}
		// THIS IS JUST A TEMP THING UNTIL WE FIX SECURITY TABLE
		// Filter out records where maxDate is more than 1 year before the most recent minDate
		if mostRecentMinDate != nil {
			oneYearBeforeMostRecent := mostRecentMinDate.AddDate(-1, 0, 0)
			var filteredRecords []securityRecord
			for _, record := range securityRecords {
				// Keep record if maxDate is NULL (current/ongoing) or if maxDate is within the year threshold
				if record.maxDateFromSQL == nil || !record.maxDateFromSQL.Before(oneYearBeforeMostRecent) {
					filteredRecords = append(filteredRecords, record)
				}
			}
			securityRecords = filteredRecords
		}
	}

	// Preallocate capacity for bar data. We'll at most fetch up to args.Bars + small overhead
	barDataList := make([]GetChartDataResults, 0, args.Bars+10)
	numBarsRemaining := args.Bars

	// Track whether earlier data exists (from the first record, since they're ordered)
	var isEarliestData bool
	if len(securityRecords) > 0 && args.Direction == "backward" {
		isEarliestData = !securityRecords[0].hasEarlierData
	}

	//if debug {
	////fmt.Printf("[DEBUG] Processing %d security record(s) from DB...\n", len(securityRecords))
	//}

	// Now iterate over the fetched security records
	for _, record := range securityRecords {
		// Use data from the record struct
		ticker := record.ticker
		minDateFromSQL := record.minDateFromSQL
		maxDateFromSQL := record.maxDateFromSQL

		tickerForIncompleteAggregate = ticker
		////fmt.Printf("\n [DEBUG]ticker: %s, minDateFromSQL: %v, maxDateFromSQL: %v\n", tickerForIncompleteAggregate, minDateFromSQL, maxDateFromSQL)
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
			if queryEndTime.Before(minDateSQL) {
				continue // Requested time is before the earliest known data for this security record.
			}
			if maxDateSQL.Before(queryEndTime) {
				queryEndTime = maxDateSQL
			}
			queryStartTime = minDateSQL
			if queryStartTime.After(queryEndTime) {
				// Requested time is before the earliest known data for this security record.
				// Return empty data, indicating the earliest point was reached.

				// Log chart query in goroutine
				go logChartQuery(conn, userID, args)

				return GetChartDataResponse{Bars: []GetChartDataResults{}, IsEarliestData: true}, nil
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
		//////fmt.Printf("\n\n%v, %v, %v, %v, %v, %v\n\n", ticker, timespan, multiplier, queryBars, queryStartTime, queryEndTime)
		date1, date2, err := GetRequestStartEndTime(
			queryStartTime, queryEndTime, args.Direction, timespan, multiplier, queryBars,
		)
		if err != nil {
			//if debug {
			////fmt.Printf("[DEBUG] GetRequestStartEndTime failed: %v\n", err)
			//}
			return nil, fmt.Errorf("dkn0 %v", err)
		}

		//if debug {
		////fmt.Printf("[DEBUG] Polygon request for %s: start=%v end=%v aggregator=%v\n", ticker, date1, date2, haveToAggregate)
		//}

		// If we have to aggregate (e.g., second->minute, or minute->hour), do so
		if haveToAggregate {
			numBarsRequestedPolygon = int(math.Ceil(float64(queryBars*multiplier)/float64(queryMultiplier))) + 10 // 10 bars margin
			it, err := polygon.GetAggsData(
				conn.Polygon,
				ticker,
				queryMultiplier,
				queryTimespan,
				date1, date2,
				numBarsRequestedPolygon,
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
			numBarsRequestedPolygon = queryBars + 10 // 10 bars margin
			it, err := polygon.GetAggsData(
				conn.Polygon,
				ticker,
				queryMultiplier,
				queryTimespan,
				date1,
				date2,
				numBarsRequestedPolygon,
				polyResultOrder,
				!args.IsReplay,
			)
			if err != nil {
				//if debug {
				////fmt.Printf("[DEBUG] Polygon API error: %v\n", err)
				//}
				return nil, fmt.Errorf("error fetching data from Polygon: %v", err)
			}

			for it.Next() {
				item := it.Item()
				if it.Err() != nil {
					return nil, fmt.Errorf("dkn0w")
				}

				ts := time.Time(item.Timestamp).In(easternLocation)
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
				marketStatus, err := polygon.GetMarketStatus(conn)
				if err != nil {
					return nil, fmt.Errorf("issue with market status")
				}

				if (args.Timestamp == 0 && marketStatus != "closed") || args.IsReplay {
					////fmt.Printf("\n\nrequesting incomplete bar\n\n")
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
			integrateChartEvents(&barDataList, conn, userID, args.SecurityID, args.IncludeSECFilings, multiplier, timespan, args.ExtendedHours, easternLocation)
			go logChartQuery(conn, userID, args)

			return GetChartDataResponse{
				Bars:           barDataList,
				IsEarliestData: isEarliestData,
			}, nil
		}

		// Otherwise, direction=backward with direct data—reverse the slice
		reverse(barDataList)

		// Possibly append incomplete bar
		marketStatus, err := polygon.GetMarketStatus(conn)
		if err != nil {
			return nil, fmt.Errorf("issue with market status")
		}
		if (args.Timestamp == 0 && marketStatus != "closed") || args.IsReplay {
			//if debug {
			////fmt.Printf("\n\nrequesting incomplete bar\n\n")
			//}
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
			//if debug {
			////fmt.Printf("\n\nincompleteAgg: %v\n\n", incompleteAgg)
			//}
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
				if (utils.IsTimestampRegularHours(incompleteTs)) ||
					timespan == "day" || timespan == "week" || timespan == "month" {
					barDataList = append(barDataList, incompleteAgg)
				}
			}
		}
		integrateChartEvents(&barDataList, conn, userID, args.SecurityID, args.IncludeSECFilings, multiplier, timespan, args.ExtendedHours, easternLocation)

		go logChartQuery(conn, userID, args)

		return GetChartDataResponse{
			Bars:           barDataList,
			IsEarliestData: isEarliestData,
		}, nil
	}

	//if debug {
	////fmt.Printf("[DEBUG] No data found. numBarsRemaining=%d\n", numBarsRemaining)
	//}

	// Log chart query in goroutine (even for failed requests)
	go logChartQuery(conn, userID, args)

	return nil, fmt.Errorf("no data found for %d, %s", args.SecurityID, tickerForIncompleteAggregate)
}

func reverse(data []GetChartDataResults) {
	for left, right := 0, len(data)-1; left < right; {
		data[left], data[right] = data[right], data[left]
		left++
		right--
	}
}

// requestIncompleteBars fetches daily, minute, second, and trade data
// in parallel, then merges all results in a single pass. This avoids stepwise merges.
func requestIncompleteBar(
	conn *data.Conn,
	ticker string,
	timestamp int64,
	multiplier int,
	timespan string,
	extendedHours bool,
	_ bool,
	easternLocation *time.Location,
) (GetChartDataResults, error) {

	// --------------------------
	// 1) Compute time boundaries
	// --------------------------
	var incompleteBar GetChartDataResults
	timestampEnd := timestamp
	if timestamp == 0 {
		timestampEnd = time.Now().UnixMilli()
	}

	timestampTime := time.Unix(timestampEnd/1000, (timestampEnd%1000)*1e6).UTC()
	var timestampStart int64
	var currentDayStart int64

	// Same logic as before
	if timespan == "second" || timespan == "minute" || timespan == "hour" {
		currentDayStart = GetReferenceStartTime(timestampEnd, extendedHours, easternLocation)
		elapsed := timestampEnd - currentDayStart
		timeframeInSeconds := GetTimeframeInSeconds(multiplier, timespan)
		if elapsed < 0 {
			// Means we're before the day start
			return incompleteBar, nil
		}
		// Snap to boundary
		timestampStart = currentDayStart + (elapsed/(timeframeInSeconds*1000))*timeframeInSeconds*1000
	} else {
		// Daily or above
		switch timespan {
		case "day":
			timestampStart = GetReferenceStartTimeForDays(timestampEnd, multiplier, easternLocation)
		case "week":
			timestampStart = GetReferenceStartTimeForWeeks(timestampEnd, multiplier, easternLocation)
		case "month":
			timestampStart = GetReferenceStartTimeForMonths(timestampEnd, multiplier, easternLocation)
		}
	}
	// This is the official bar "start" in seconds
	incompleteBar.Timestamp = math.Floor(float64(timestampStart) / 1000.0)

	// -------------------------
	// 2) Define sub-windows for daily/minute/second/trade, so no overlap
	// -------------------------
	// We'll fetch:
	//   (A) daily bars:   [timestampStart .. dailyEnd)
	//   (B) minute bars:  [dailyEnd       .. minuteEnd)
	//   (C) second bars:  [minuteEnd      .. secondEnd)
	//   (D) trades:       [secondEnd      .. timestampEnd)
	//
	// This ensures we don't double-aggregate.

	//
	// (A) daily up to last-complete-day
	//
	lastCompleteDayUTC := time.Date(timestampTime.Year(), timestampTime.Month(), timestampTime.Day(),
		0, 0, 0, 0, time.UTC).UnixMilli()
	dailyEnd := lastCompleteDayUTC - 86400000 // 1 day (ms)
	if dailyEnd < timestampStart {
		// means there's no partial day to fetch
		dailyEnd = timestampStart
	}

	//
	// (B) minute up to last-complete-minute
	//
	lastCompleteMinuteUTC := timestampTime.Truncate(time.Minute).UnixMilli()
	// If after market close, roll back to 15:59
	if !extendedHours || timespan == "day" || timespan == "week" || timespan == "month" {
		marketClose := time.Date(timestampTime.Year(), timestampTime.Month(), timestampTime.Day(),
			16, 0, 0, 0, easternLocation)
		if timestampTime.After(marketClose) {
			lastCompleteMinuteUTC = time.Date(timestampTime.Year(), timestampTime.Month(), timestampTime.Day(),
				15, 59, 0, 0, easternLocation).UnixMilli()
		}
	}
	minuteEnd := lastCompleteMinuteUTC - 60000 // subtract 1 minute
	if minuteEnd < dailyEnd {
		minuteEnd = dailyEnd
	}

	//
	// (C) second up to last-complete-second
	//
	lastCompleteSecondUTC := timestampTime.Truncate(time.Second).UnixMilli()
	secondEnd := lastCompleteSecondUTC
	if secondEnd < minuteEnd {
		secondEnd = minuteEnd
	}

	//
	// (D) trades up to timestampEnd
	//
	tradeEnd := timestampEnd
	if tradeEnd < secondEnd {
		tradeEnd = secondEnd
	}

	// ----------------------``--
	// 3) Concurrently fetch data
	// ------------------------
	var wg sync.WaitGroup
	wg.Add(4) // daily, minute, second, trades

	var dailyBars []models.Agg
	var minuteBars []models.Agg
	var secondBars []models.Agg
	var tradeList []models.Trade

	var dailyErr, minuteErr, secondErr, tradeErr error

	go func() {
		defer wg.Done()
		dailyBars, dailyErr = fetchAggData(
			conn, ticker,
			1, "day",
			timestampStart, dailyEnd,
			false,
			easternLocation,
		)
	}()
	go func() {
		defer wg.Done()
		minuteBars, minuteErr = fetchAggData(
			conn, ticker,
			1, "minute",
			dailyEnd, minuteEnd,
			extendedHours,
			easternLocation,
		)
	}()
	go func() {
		defer wg.Done()
		secondBars, secondErr = fetchAggData(
			conn, ticker,
			1, "second",
			minuteEnd, secondEnd,
			extendedHours,
			easternLocation,
		)
	}()
	go func() {
		defer wg.Done()
		tradeList, tradeErr = fetchTrades(
			conn, ticker,
			secondEnd, tradeEnd,
			extendedHours,
			false,
			easternLocation,
		)
	}()

	wg.Wait()

	// Check errors
	if dailyErr != nil {
		return incompleteBar, fmt.Errorf("daily fetch error: %v", dailyErr)
	}
	if minuteErr != nil {
		return incompleteBar, fmt.Errorf("minute fetch error: %v", minuteErr)
	}
	if secondErr != nil {
		return incompleteBar, fmt.Errorf("second fetch error: %v", secondErr)
	}
	if tradeErr != nil {
		return incompleteBar, fmt.Errorf("trade fetch error: %v", tradeErr)
	}

	// ------------------------
	// 4) Flatten all data into a single slice
	// ------------------------
	combined := make([]combinedData, 0, len(dailyBars)+len(minuteBars)+len(secondBars)+len(tradeList)+50)

	// (A) daily
	for _, agg := range dailyBars {
		combined = append(combined, combinedData{
			ts:      time.Time(agg.Timestamp),
			isTrade: false,
			open:    agg.Open,
			high:    agg.High,
			low:     agg.Low,
			close:   agg.Close,
			volume:  agg.Volume,
			// No conditions for aggregated bars
		})
	}
	// (B) minute
	for _, agg := range minuteBars {
		combined = append(combined, combinedData{
			ts:      time.Time(agg.Timestamp),
			isTrade: false,
			open:    agg.Open,
			high:    agg.High,
			low:     agg.Low,
			close:   agg.Close,
			volume:  agg.Volume,
		})
	}
	// (C) second
	for _, agg := range secondBars {
		combined = append(combined, combinedData{
			ts:      time.Time(agg.Timestamp),
			isTrade: false,
			open:    agg.Open,
			high:    agg.High,
			low:     agg.Low,
			close:   agg.Close,
			volume:  agg.Volume,
		})
	}
	// (D) trades
	for _, tr := range tradeList {
		// For trades, treat Price as O/H/L/C and Size as volume
		combined = append(combined, combinedData{
			ts:         time.Time(tr.ParticipantTimestamp),
			isTrade:    true,
			open:       tr.Price,
			high:       tr.Price,
			low:        tr.Price,
			close:      tr.Price,
			volume:     tr.Size,
			conditions: tr.Conditions,
		})
	}

	// ------------------------
	// 5) Sort combined by ascending timestamp
	// ------------------------
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].ts.Before(combined[j].ts)
	})

	// ------------------------
	// 6) Single-pass aggregator
	// ------------------------
	tradeConditionsToSkipOhlc := map[int32]struct{}{
		2: {}, 5: {}, 10: {}, 15: {}, 16: {}, 20: {}, 21: {}, 22: {}, 29: {}, 33: {}, 38: {}, 52: {}, 53: {},
	}
	tradeConditionsToSkipVolume := map[int32]struct{}{
		15: {}, 16: {}, 38: {},
	}

	for _, cd := range combined {
		// Only process data that's within the final time range
		if cd.ts.UnixMilli() < timestampStart {
			continue
		}
		if cd.ts.UnixMilli() > timestampEnd {
			break
		}

		if cd.isTrade {
			// Evaluate whether to skip O/H/L/C or volume based on conditions
			skipOhlc := false
			skipVol := false
			for _, cond := range cd.conditions {
				if _, found := tradeConditionsToSkipOhlc[cond]; found {
					skipOhlc = true
				}
				if _, found := tradeConditionsToSkipVolume[cond]; found {
					skipVol = true
				}
				if skipOhlc && skipVol {
					break
				}
			}
			if !skipOhlc {
				if incompleteBar.Open == 0 {
					incompleteBar.Open = cd.open
				}
				if cd.high > incompleteBar.High {
					incompleteBar.High = cd.high
				}
				if incompleteBar.Low == 0 || cd.low < incompleteBar.Low {
					incompleteBar.Low = cd.low
				}
				incompleteBar.Close = cd.close
			}
			if !skipVol {
				incompleteBar.Volume += cd.volume
			}
		} else {
			// It's aggregated data (daily/minute/second)
			// Incorporate it directly.
			if incompleteBar.Open == 0 && cd.open != 0 {
				incompleteBar.Open = cd.open
			}
			if cd.high > incompleteBar.High {
				incompleteBar.High = cd.high
			}
			if incompleteBar.Low == 0 || cd.low < incompleteBar.Low {
				incompleteBar.Low = cd.low
			}
			incompleteBar.Close = cd.close
			incompleteBar.Volume += cd.volume
		}
	}

	return incompleteBar, nil
}

// ----------------------------------------------------
//  Helper types and fetch functions used by the above
// ----------------------------------------------------

// combinedData is a unified structure for Agg or Trade data
type combinedData struct {
	ts         time.Time
	isTrade    bool
	open       float64
	high       float64
	low        float64
	close      float64
	volume     float64
	conditions []int32 // only used for trades
}

// fetchAggData pulls aggregated bars (day/minute/second) from Polygon,
// optionally filtering out non-regular-hour bars if `filterRegularOnly` is true.
func fetchAggData(
	conn *data.Conn,
	ticker string,
	multiplier int,
	timespan string,
	startMs, endMs int64,
	extendedHours bool,
	easternLocation *time.Location,
) ([]models.Agg, error) {

	if endMs <= startMs {
		return nil, nil
	}
	start := models.Millis(time.Unix(0, startMs*int64(time.Millisecond)).UTC())
	end := models.Millis(time.Unix(0, endMs*int64(time.Millisecond)).UTC())

	it, err := polygon.GetAggsData(conn.Polygon, ticker, multiplier, timespan, start, end, 10000, "asc", !extendedHours)
	if err != nil {
		return nil, err
	}

	var result []models.Agg
	for it.Next() {
		agg := it.Item()
		ts := time.Time(agg.Timestamp).In(easternLocation)

		if !extendedHours && timespan != "day" {
			if !utils.IsTimestampRegularHours(ts) {
				continue
			}
		}
		result = append(result, agg)
	}
	return result, it.Err()
}

// fetchTrades pulls trade data, filtering pre/post market if extendedHours=false.
func fetchTrades(
	conn *data.Conn,
	ticker string,
	startMs, endMs int64,
	extendedHours bool,
	_ bool,
	easternLocation *time.Location,
) ([]models.Trade, error) {

	if endMs <= startMs {
		return nil, nil
	}
	startNanos := models.Nanos(time.Unix(startMs/1000, (startMs%1000)*1e6).UTC())

	it, err := polygon.GetTrade(conn.Polygon, ticker, startNanos, "asc", models.GTE, 30000)
	if err != nil {
		return nil, err
	}

	var trades []models.Trade
	endTime := time.Unix(0, endMs*int64(time.Millisecond)).UTC()

	for it.Next() {
		tr := it.Item()
		if it.Err() != nil {
			return nil, it.Err()
		}
		tradeTs := time.Time(tr.ParticipantTimestamp).In(easternLocation)
		if tradeTs.After(endTime) {
			break
		}
		if extendedHours || utils.IsTimestampRegularHours(tradeTs) {
			trades = append(trades, tr)
		}
	}
	return trades, it.Err()
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
	unitDuration := TimespanStringToDuration(timespan)
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
	if !barStartTime.IsZero() {
		// Always add the last bar if we have one in progress,
		// regardless of direction - this fixes the missing latest bar
		barDataList = append(barDataList, currentBar)
	}

	// Now handle the direction-specific logic
	if direction == "forward" {
		// For forward direction, we've already added the last bar above
		if *numBarsRemaining > 0 {
			*numBarsRemaining--
		}
	} else {
		// For backward direction, we need to keep the most recent bars
		// If we have more bars than needed, take the most recent ones
		if len(barDataList) > *numBarsRemaining {
			barDataList = barDataList[len(barDataList)-*numBarsRemaining:]
			*numBarsRemaining = 0
		} else {
			// If we have fewer bars than needed, keep all and update remaining count
			*numBarsRemaining -= len(barDataList)
		}
	}

	return barDataList, nil
}

// Helper function to determine the start timestamp (Unix seconds) of the bar an event belongs to
func alignTimestampToStartOfBar(eventMs int64, multiplier int, timespan string, extendedHours bool, loc *time.Location) int64 {
	timeframeSeconds := GetTimeframeInSeconds(multiplier, timespan)
	var barStartMs int64

	// Ensure location is not nil, fallback to UTC if necessary to prevent panics
	if loc == nil {
		////fmt.Println("Warning: Eastern location was nil in alignTimestampToStartOfBar, falling back to UTC.")
		loc = time.UTC
	}

	if timespan == "second" || timespan == "minute" || timespan == "hour" {
		referenceStartMs := GetReferenceStartTime(eventMs, extendedHours, loc)
		elapsedMs := eventMs - referenceStartMs
		if elapsedMs < 0 {
			// Event is before the calculated reference start for the day.
			// This might happen for events near midnight or if reference logic has edge cases.
			// Aligning to the reference start might be the most reasonable approach.
			////fmt.Printf("Warning: Event timestamp %d ms is before its reference start %d ms for %s %d. Aligning to reference start.\\n", eventMs, referenceStartMs, timespan, multiplier)
			return referenceStartMs / 1000
		}

		// Ensure timeframeSeconds is valid before calculating milliseconds or dividing
		if timeframeSeconds <= 0 {
			////fmt.Printf("Warning: Invalid timeframeSeconds %d calculated for %s %d. Using 60s fallback for alignment.\\n", timeframeSeconds, timespan, multiplier)
			timeframeSeconds = 60 // Fallback to 60 seconds
		}
		tfMillis := int64(timeframeSeconds) * 1000
		if tfMillis <= 0 {
			////fmt.Printf("Warning: Invalid timeframeMillis %d. Using 60000ms fallback.\\n", tfMillis)
			tfMillis = 60000 // Prevent division by zero or negative values
		}

		barOffsetMs := (elapsedMs / tfMillis) * tfMillis
		barStartMs = referenceStartMs + barOffsetMs
	} else {
		// Daily or above - use the specific reference start functions based on UTC time
		switch timespan {
		case "day":
			barStartMs = GetReferenceStartTimeForDays(eventMs, multiplier, loc) // loc might be needed if utils func uses it
		case "week":
			barStartMs = GetReferenceStartTimeForWeeks(eventMs, multiplier, loc)
		case "month":
			barStartMs = GetReferenceStartTimeForMonths(eventMs, multiplier, loc)
		default:
			////fmt.Printf("Warning: Unknown timespan '%s' in alignTimestampToStartOfBar. Falling back to UTC day alignment.\\n", timespan)
			// Fallback: align to UTC day start
			t := time.UnixMilli(eventMs).UTC()
			barStartMs = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).UnixMilli()
		}
	}
	return barStartMs / 1000 // Return Unix seconds
}

// Helper function to fetch and integrate events into the bar list
func integrateChartEvents(
	barDataList *[]GetChartDataResults, // Pointer to modify the slice
	conn *data.Conn,
	userID int,
	securityID int,
	includeSECFilings bool,
	multiplier int,
	timespan string,
	extendedHours bool,
	easternLocation *time.Location,
) {
	// Ensure we have bars and necessary info to proceed
	if barDataList == nil || len(*barDataList) == 0 {
		return
	}

	bars := *barDataList // Dereference for easier use

	// 1. Determine Time Range from existing bars
	// Sort a temporary copy to reliably find min/max timestamps
	tempSortedBars := make([]GetChartDataResults, len(bars))
	copy(tempSortedBars, bars)
	sort.Slice(tempSortedBars, func(i, j int) bool {
		// Handle potential NaN or Inf values if they could occur
		if math.IsNaN(tempSortedBars[i].Timestamp) || math.IsNaN(tempSortedBars[j].Timestamp) {
			return false // Or handle according to desired NaN sorting
		}
		return tempSortedBars[i].Timestamp < tempSortedBars[j].Timestamp
	})

	// Check if sorting resulted in valid timestamps
	if len(tempSortedBars) == 0 {
		////fmt.Println("Warning: No valid bars after sorting for event fetching.")
		return
	}

	minTsSec := tempSortedBars[0].Timestamp
	maxTsSec := tempSortedBars[len(tempSortedBars)-1].Timestamp

	// Validate timestamps before proceeding
	if math.IsNaN(minTsSec) || math.IsNaN(maxTsSec) || math.IsInf(minTsSec, 0) || math.IsInf(maxTsSec, 0) {
		////fmt.Printf("Warning: Invalid min/max timestamps after sorting: min=%f, max=%f. Cannot fetch events.\\n", minTsSec, maxTsSec)
		return
	}

	chartTimeframeInSeconds := GetTimeframeInSeconds(multiplier, timespan)
	if chartTimeframeInSeconds <= 0 {
		////fmt.Printf("Warning: Cannot fetch events due to invalid timeframeSeconds: %d for %s %d\\n", chartTimeframeInSeconds, timespan, multiplier)
		return
	}

	// Calculate time range in milliseconds for the API call
	fromMs := int64(minTsSec * 1000)
	// Extend 'toMs' to cover the full duration of the last bar
	toMs := int64((maxTsSec + float64(chartTimeframeInSeconds)) * 1000)

	// 2. Fetch Events
	chartEvents, err := fetchChartEventsInRange(conn, userID, securityID, fromMs, toMs, includeSECFilings, true)
	if err != nil {
		// Log the error but don't fail the whole chart request
		////fmt.Printf("Warning: Failed to fetch chart events for secId %d between %d and %d: %v. Returning bars without events.\\n", securityID, fromMs, toMs, err)
		return // Continue without events
	}

	if len(chartEvents) == 0 {
		return // No events found for this range
	}

	// 3. Map Events to Bar Timestamps (using Unix seconds as key)
	eventsByTimestamp := make(map[int64][]Event)
	for _, event := range chartEvents {
		// Align the event timestamp (UTC ms) to its corresponding bar's start time (Unix seconds)
		barStartTimeSec := alignTimestampToStartOfBar(event.Timestamp, multiplier, timespan, extendedHours, easternLocation)
		eventsByTimestamp[barStartTimeSec] = append(eventsByTimestamp[barStartTimeSec], event)
	}

	// 4. Attach Events to the Original Bars (modifying the slice via pointer)
	for i := range bars {
		// Convert the bar's float64 seconds timestamp to int64 for map lookup
		barTimestampSec := int64(bars[i].Timestamp)
		if eventsToAdd, ok := eventsByTimestamp[barTimestampSec]; ok {
			// Ensure the Events slice is initialized before appending
			if bars[i].Events == nil {
				// Pre-allocate with approximate capacity if possible
				bars[i].Events = make([]Event, 0, len(eventsToAdd))
			}
			bars[i].Events = append(bars[i].Events, eventsToAdd...)
		}
	}
	// The modifications to 'bars' directly affect the slice pointed to by barDataList
}

// logChartQuery logs chart query parameters to the database for analytics
func logChartQuery(conn *data.Conn, userID int, args GetChartDataArgs) {
	// Use a separate context with timeout for the logging operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO chart_queries (
			securityid, timeframe, timestamp, direction, bars, 
			extended_hours, is_replay, include_sec_filings, user_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := conn.DB.Exec(ctx, query,
		args.SecurityID,
		args.Timeframe,
		args.Timestamp,
		args.Direction,
		args.Bars,
		args.ExtendedHours,
		args.IsReplay,
		args.IncludeSECFilings,
		userID,
	)

	if err != nil {
		// Log error but don't fail the main request
		fmt.Printf("Warning: Failed to log chart query: %v\n", err)
	}
}

// GetPublicChartData provides chart data for public/unauthenticated users
// This is a simple adapter that calls GetChartData with userID=0 to indicate public access
func GetPublicChartData(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	// Call the existing GetChartData function with userID=0 to indicate public access
	return GetChartData(conn, 0, rawArgs)
}
