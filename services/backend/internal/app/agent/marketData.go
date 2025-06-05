package agent

import (
	"backend/internal/app/chart"
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/utils"

	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

type GetOHLCVDataArgs struct {
	SecurityID    int      `json:"securityId"`
	Timeframe     string   `json:"timeframe"`
	From          int64    `json:"from"`
	To            int64    `json:"to,omitempty"`
	Bars          int      `json:"bars"`
	ExtendedHours bool     `json:"extended"`
	Columns       []string `json:"columns,omitempty"`
}

// Columnar response format for better token efficiency
type GetOHLCVDataResponse struct {
	Ticker    string    `json:"ticker,omitempty"`
	Timeframe string    `json:"tf,omitempty"`
	T         []float64 `json:"t"`           // timestamps
	O         []float64 `json:"o,omitempty"` // open
	H         []float64 `json:"h,omitempty"` // high
	L         []float64 `json:"l,omitempty"` // low
	C         []float64 `json:"c,omitempty"` // close
	V         []float64 `json:"v,omitempty"` // volume
}

// ColumnFilter handles which columns should be included in the response
type ColumnFilter struct {
	includeOpen   bool
	includeHigh   bool
	includeLow    bool
	includeClose  bool
	includeVolume bool
}

// NewColumnFilter creates a new ColumnFilter from the requested columns
func NewColumnFilter(columns []string) *ColumnFilter {
	// If no columns specified, include all
	if len(columns) == 0 {
		return &ColumnFilter{
			includeOpen:   true,
			includeHigh:   true,
			includeLow:    true,
			includeClose:  true,
			includeVolume: true,
		}
	}

	cf := &ColumnFilter{}
	for _, col := range columns {
		switch col {
		case "o":
			cf.includeOpen = true
		case "h":
			cf.includeHigh = true
		case "l":
			cf.includeLow = true
		case "c":
			cf.includeClose = true
		case "v":
			cf.includeVolume = true
		}
	}
	return cf
}

// AppendToResponse adds a bar's data to the columnar response arrays
func (cf *ColumnFilter) AppendToResponse(response *GetOHLCVDataResponse, timestamp time.Time, open, high, low, close, volume float64) {
	response.T = append(response.T, float64(timestamp.Unix()))

	if cf.includeOpen {
		response.O = append(response.O, open)
	}
	if cf.includeHigh {
		response.H = append(response.H, high)
	}
	if cf.includeLow {
		response.L = append(response.L, low)
	}
	if cf.includeClose {
		response.C = append(response.C, close)
	}
	if cf.includeVolume {
		response.V = append(response.V, volume)
	}
}

func GetOHLCVData(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetOHLCVDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	columnFilter := NewColumnFilter(args.Columns)

	multiplier, timespan, _, _, err := chart.GetTimeFrame(args.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("invalid timeframe: %v", err)
	}

	var queryTimespan string
	var queryMultiplier int
	var queryBars int
	var numBarsRequestedPolygon int
	haveToAggregate := false
	if args.Bars > 300 {
		args.Bars = 300
	}
	// Special logic for second/minute frames with 30-based constraints
	if (timespan == "second" || timespan == "minute") && (30%multiplier != 0) {
		queryTimespan = timespan
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

	// Convert timestamps to time.Time
	var fromTime, toTime time.Time
	if args.From > 0 {
		fromTime = time.Unix(args.From/1000, (args.From%1000)*1e6).UTC()
	}
	if args.To > 0 {
		toTime = time.Unix(args.To/1000, (args.To%1000)*1e6).UTC()
	} else {
		// If no end time provided, use current time
		toTime = time.Now().UTC()
	}

	// Build simplified DB query for chronological data
	query := `SELECT ticker, minDate, maxDate
             FROM securities 
             WHERE securityid = $1 AND (maxDate >= $2 OR maxDate IS NULL) AND (minDate <= $3 OR minDate IS NULL)
             ORDER BY minDate ASC`
	queryParams := []interface{}{args.SecurityID, fromTime, toTime}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, query, queryParams...)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out: %w", err)
		}
		return nil, fmt.Errorf("error querying data: %w", err)
	}
	defer rows.Close()

	type securityRecord struct {
		ticker         string
		minDateFromSQL *time.Time
		maxDateFromSQL *time.Time
	}

	var securityRecords []securityRecord
	for rows.Next() {
		var record securityRecord
		if err := rows.Scan(&record.ticker, &record.minDateFromSQL, &record.maxDateFromSQL); err != nil {
			return nil, fmt.Errorf("error scanning security data: %w", err)
		}
		securityRecords = append(securityRecords, record)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating security data rows: %w", err)
	}
	rows.Close()

	// Initialize columnar response
	response := &GetOHLCVDataResponse{
		Timeframe: args.Timeframe,
		T:         make([]float64, 0, args.Bars+10),
	}

	// Initialize arrays based on column filter
	if columnFilter.includeOpen {
		response.O = make([]float64, 0, args.Bars+10)
	}
	if columnFilter.includeHigh {
		response.H = make([]float64, 0, args.Bars+10)
	}
	if columnFilter.includeLow {
		response.L = make([]float64, 0, args.Bars+10)
	}
	if columnFilter.includeClose {
		response.C = make([]float64, 0, args.Bars+10)
	}
	if columnFilter.includeVolume {
		response.V = make([]float64, 0, args.Bars+10)
	}

	numBarsRemaining := args.Bars

	// Process security records chronologically
	for _, record := range securityRecords {
		if numBarsRemaining <= 0 {
			break
		}

		ticker := record.ticker
		response.Ticker = ticker

		minDateFromSQL := record.minDateFromSQL
		maxDateFromSQL := record.maxDateFromSQL

		// Handle NULL dates
		if maxDateFromSQL == nil {
			now := time.Now()
			maxDateFromSQL = &now
		}

		var minDateSQL time.Time
		if minDateFromSQL != nil {
			minDateSQL = minDateFromSQL.In(easternLocation)
		} else {
			minDateSQL = time.Date(1970, 1, 1, 0, 0, 0, 0, easternLocation)
		}
		maxDateSQL := maxDateFromSQL.In(easternLocation)

		// Determine query time range
		queryStartTime := fromTime
		if minDateSQL.After(queryStartTime) {
			queryStartTime = minDateSQL
		}

		queryEndTime := toTime
		if maxDateSQL.Before(queryEndTime) {
			queryEndTime = maxDateSQL
		}

		if queryEndTime.Before(queryStartTime) {
			continue
		}

		// Get request start/end times using chart package function
		date1, date2, err := chart.GetRequestStartEndTime(
			queryStartTime, queryEndTime, "forward", timespan, multiplier, queryBars,
		)
		if err != nil {
			return nil, fmt.Errorf("error calculating request times: %v", err)
		}

		// Fetch data from Polygon
		if haveToAggregate {
			numBarsRequestedPolygon = int(math.Ceil(float64(queryBars*multiplier)/float64(queryMultiplier))) + 10
			it, err := polygon.GetAggsData(
				conn.Polygon,
				ticker,
				queryMultiplier,
				queryTimespan,
				date1, date2,
				numBarsRequestedPolygon,
				"asc", // Always ascending for chronological data
				true,
			)
			if err != nil {
				return nil, fmt.Errorf("error fetching data from Polygon: %v", err)
			}

			err = buildColumnarAggregation(
				it, multiplier, timespan, args.ExtendedHours, easternLocation, &numBarsRemaining, columnFilter, response,
			)
			if err != nil {
				return nil, err
			}
		} else {
			// Direct fetch at desired timeframe
			numBarsRequestedPolygon = queryBars + 10
			it, err := polygon.GetAggsData(
				conn.Polygon,
				ticker,
				queryMultiplier,
				queryTimespan,
				date1,
				date2,
				numBarsRequestedPolygon,
				"asc", // Always ascending for chronological data
				true,
			)
			if err != nil {
				return nil, fmt.Errorf("error fetching data from Polygon: %v", err)
			}

			for it.Next() {
				item := it.Item()
				if it.Err() != nil {
					return nil, fmt.Errorf("iterator error: %v", it.Err())
				}

				ts := time.Time(item.Timestamp).In(easternLocation)
				// Skip out of hours if not extended hours
				if (timespan == "minute" || timespan == "second" || timespan == "hour") &&
					!args.ExtendedHours && !utils.IsTimestampRegularHours(ts) {
					continue
				}

				columnFilter.AppendToResponse(response, ts, item.Open, item.High, item.Low, item.Close, item.Volume)

				numBarsRemaining--
				if numBarsRemaining <= 0 {
					break
				}
			}
		}
	}

	// Return chronological data
	if len(response.T) > 0 {
		return response, nil
	}

	return nil, fmt.Errorf("no data found")
}

// buildColumnarAggregation creates higher timeframe bars from lower timeframe data in columnar format
func buildColumnarAggregation(
	it *iter.Iter[models.Agg],
	multiplier int,
	timespan string,
	extendedHours bool,
	easternLocation *time.Location,
	numBarsRemaining *int,
	columnFilter *ColumnFilter,
	response *GetOHLCVDataResponse,
) error {

	var currentBarValues struct {
		timestamp time.Time
		open      float64
		high      float64
		low       float64
		close     float64
		volume    float64
	}
	var barStartTime time.Time

	// Convert the timespan into duration
	unitDuration := chart.TimespanStringToDuration(timespan)
	requiredDuration := time.Duration(multiplier) * unitDuration

	for it.Next() {
		agg := it.Item()
		if it.Err() != nil {
			return fmt.Errorf("iterator error: %v", it.Err())
		}
		timestamp := time.Time(agg.Timestamp).In(easternLocation)

		// Filter out pre/post market if not extended
		if !extendedHours && (timespan == "minute" || timespan == "second" || timespan == "hour") {
			if !utils.IsTimestampRegularHours(timestamp) {
				continue
			}
		}

		// Check if we need to start a new bar
		diff := timestamp.Sub(barStartTime)
		if barStartTime.IsZero() || diff >= requiredDuration {
			// If we have a bar in progress, store it
			if !barStartTime.IsZero() {
				columnFilter.AppendToResponse(response, currentBarValues.timestamp, currentBarValues.open, currentBarValues.high, currentBarValues.low, currentBarValues.close, currentBarValues.volume)
				*numBarsRemaining--
				if *numBarsRemaining <= 0 {
					break
				}
			}
			// Start a new bar
			currentBarValues = struct {
				timestamp time.Time
				open      float64
				high      float64
				low       float64
				close     float64
				volume    float64
			}{
				timestamp: timestamp,
				open:      agg.Open,
				high:      agg.High,
				low:       agg.Low,
				close:     agg.Close,
				volume:    agg.Volume,
			}
			barStartTime = timestamp
		} else {
			// Continue aggregating into the current bar
			if agg.High > currentBarValues.high {
				currentBarValues.high = agg.High
			}
			if agg.Low < currentBarValues.low {
				currentBarValues.low = agg.Low
			}
			currentBarValues.close = agg.Close
			currentBarValues.volume += agg.Volume
		}
	}

	// Add the last bar if we have one in progress
	if !barStartTime.IsZero() && *numBarsRemaining > 0 {
		columnFilter.AppendToResponse(response, currentBarValues.timestamp, currentBarValues.open, currentBarValues.high, currentBarValues.low, currentBarValues.close, currentBarValues.volume)
	}

	return nil
}
