package agent

import (
	"backend/internal/app/chart"
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/postgres"
	"backend/internal/data/utils"
	"strings"

	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
	"google.golang.org/genai"
)

type GetOHLCVDataArgs struct {
	SecurityID    int      `json:"securityId"`
	Timeframe     string   `json:"timeframe"`
	From          int64    `json:"from"`         //seconds since epoch
	To            int64    `json:"to,omitempty"` //seconds since epoch
	Bars          int      `json:"bars"`
	ExtendedHours bool     `json:"extended"`
	SplitAdjusted *bool    `json:"splitAdjusted,omitempty"`
	Columns       []string `json:"columns,omitempty"`
	TimestampType string   `json:"timestampType,omitempty"` // "ts" (default) or "text"
}

// Columnar response format for better token efficiency
type GetOHLCVDataResponse struct {
	Ticker    string      `json:"ticker,omitempty"`
	Timeframe string      `json:"tf,omitempty"`
	T         interface{} `json:"t"`           // timestamps (Unix float64 or text string array)
	O         []float64   `json:"o,omitempty"` // open
	H         []float64   `json:"h,omitempty"` // high
	L         []float64   `json:"l,omitempty"` // low
	C         []float64   `json:"c,omitempty"` // close
	V         []float64   `json:"v,omitempty"` // volume
}

// ColumnFilter handles which columns should be included in the response
type ColumnFilter struct {
	includeOpen     bool
	includeHigh     bool
	includeLow      bool
	includeClose    bool
	includeVolume   bool
	timestampType   string
	easternLocation *time.Location
}

// NewColumnFilter creates a new ColumnFilter from the requested columns
func NewColumnFilter(columns []string, timestampType string, easternLocation *time.Location) *ColumnFilter {
	// Default timestampType to "ts" if not specified or invalid
	if timestampType != "text" {
		timestampType = "ts"
	}

	// If no columns specified, include all
	if len(columns) == 0 {
		return &ColumnFilter{
			includeOpen:     true,
			includeHigh:     true,
			includeLow:      true,
			includeClose:    true,
			includeVolume:   true,
			timestampType:   timestampType,
			easternLocation: easternLocation,
		}
	}

	cf := &ColumnFilter{
		timestampType:   timestampType,
		easternLocation: easternLocation,
	}
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
func (cf *ColumnFilter) AppendToResponse(response *GetOHLCVDataResponse, timestamp time.Time, open, high, low, closePrice, volume float64) {
	// Handle timestamp based on type
	if cf.timestampType == "text" {
		// Convert to America/New_York timezone and format as text
		easternTime := timestamp.In(cf.easternLocation)
		timestampStr := easternTime.Format("2006-01-02T15:04:05")

		// Ensure T is initialized as string slice
		if response.T == nil {
			response.T = []string{}
		}
		if timestamps, ok := response.T.([]string); ok {
			response.T = append(timestamps, timestampStr)
		}
	} else {
		// Default behavior - Unix timestamp
		timestampFloat := float64(timestamp.Unix())

		// Ensure T is initialized as float64 slice
		if response.T == nil {
			response.T = []float64{}
		}
		if timestamps, ok := response.T.([]float64); ok {
			response.T = append(timestamps, timestampFloat)
		}
	}

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
		response.C = append(response.C, closePrice)
	}
	if cf.includeVolume {
		response.V = append(response.V, volume)
	}
}

func GetOHLCVData(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetOHLCVDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Default SplitAdjusted to true if not provided
	splitAdjustedValue := true
	if args.SplitAdjusted != nil {
		splitAdjustedValue = *args.SplitAdjusted
	}

	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}

	columnFilter := NewColumnFilter(args.Columns, args.TimestampType, easternLocation)

	multiplier, timespan, _, _, err := chart.GetTimeFrame(args.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("invalid timeframe: %v", err)
	}

	var queryTimespan string
	var queryMultiplier int
	var queryBars int
	var numBarsRequestedPolygon int
	haveToAggregate := false
	if args.Bars > 500 {
		args.Bars = 500
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

	// Convert timestamps to time.Time
	var fromTime, toTime time.Time
	if args.From > 0 {
		fromTime = time.Unix(args.From, 0).UTC()
	}
	if args.To > 0 {
		toTime = time.Unix(args.To, 0).UTC()
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
	}

	// Initialize timestamp array based on type
	if args.TimestampType == "text" {
		response.T = make([]string, 0, args.Bars+10)
	} else {
		response.T = make([]float64, 0, args.Bars+10)
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
				splitAdjustedValue,
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
				splitAdjustedValue,
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
	hasData := false
	if response.T != nil {
		switch t := response.T.(type) {
		case []float64:
			hasData = len(t) > 0
		case []string:
			hasData = len(t) > 0
		}
	}

	if hasData {
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

type RunIntradayAgentArgs struct {
	SecurityID       int    `json:"securityId"`
	Timeframe        string `json:"timeframe"`
	From             int64  `json:"from"`
	To               int64  `json:"to"`
	ExtendedHours    bool   `json:"extended"`
	SplitAdjusted    *bool  `json:"splitAdjusted,omitempty"`
	AdditionalPrompt string `json:"additionalPrompt,omitempty"`
}

type RunIntradayAgentResponse struct {
	Analysis string `json:"analysis"`
}

func RunIntradayAgent(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args RunIntradayAgentArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Check if from timestamp is in the future (compared to current EST time)
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}

	currentTimeEST := time.Now().In(easternLocation)
	fromTimeEST := time.Unix(args.From, 0).In(easternLocation)
	if fromTimeEST.After(currentTimeEST) {
		return nil, fmt.Errorf("from time %s is in the future", fromTimeEST.Format("2006-01-02 15:04:05 MST"))
	}

	// Default SplitAdjusted to true if not provided
	splitAdjustedValue := true
	if args.SplitAdjusted != nil {
		splitAdjustedValue = *args.SplitAdjusted
	}

	// Create the args for GetOHLCVData
	ohlcvArgs := GetOHLCVDataArgs{
		SecurityID:    args.SecurityID,
		Timeframe:     args.Timeframe,
		From:          args.From,
		To:            args.To,
		Bars:          500,
		ExtendedHours: args.ExtendedHours,
		SplitAdjusted: &splitAdjustedValue,
		TimestampType: "text",
	}

	// Marshal to JSON bytes
	ohlcvArgsBytes, err := json.Marshal(ohlcvArgs)
	if err != nil {
		return nil, fmt.Errorf("error marshaling OHLCV args: %v", err)
	}

	ohlcvData, err := GetOHLCVData(conn, 0, json.RawMessage(ohlcvArgsBytes))
	if err != nil {
		return nil, fmt.Errorf("error getting OHLCV data: %v", err)
	}
	// Convert timestamp to date and get ticker from OHLCV response
	fromTime := time.Unix(args.From, 0).UTC()
	dateStr := fromTime.Format("2006-01-02")

	ohlcvResponse, ok := ohlcvData.(*GetOHLCVDataResponse)
	if !ok || ohlcvResponse.Ticker == "" {
		return nil, fmt.Errorf("unable to get ticker from OHLCV response")
	}

	ticker, err := postgres.GetTicker(conn, args.SecurityID, fromTime)
	if err != nil {
		return nil, fmt.Errorf("error getting ticker: %v", err)
	}

	dailyData, err := polygon.GetDailyOHLCVForTicker(context.Background(), conn.Polygon, ticker, dateStr, splitAdjustedValue)
	if err != nil {
		return nil, fmt.Errorf("error getting daily data: %v", err)
	}

	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return nil, fmt.Errorf("error getting gemini key: %v", err)
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("error creating gemini client: %w", err)
	}
	systemPrompt, err := GetSystemInstruction("IntradayAgentPrompt")
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %v", err)
	}
	thinkingBudget := int32(10000)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  &thinkingBudget,
		},
	}

	// Convert OHLCV data to JSON string for the prompt
	ohlcvDataBytes, err := json.Marshal(ohlcvData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling OHLCV data: %v", err)
	}

	// Add daily data to the prompt
	dailyDataBytes, err := json.Marshal(dailyData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling daily data: %v", err)
	}

	// Create the full prompt with OHLCV data, daily data, and additional prompt
	fullPrompt := fmt.Sprintf("Intraday Data:\n%s\n\nDaily Data:\n%s", string(ohlcvDataBytes), string(dailyDataBytes))
	if args.AdditionalPrompt != "" {
		fullPrompt += "\n\nAdditional Prompt/Context from model:\n" + args.AdditionalPrompt
	}
	fmt.Println("full prompt:", fullPrompt)
	result, err := client.Models.GenerateContent(context.Background(), planningModel, genai.Text(fullPrompt), config)
	if err != nil {
		return Plan{}, fmt.Errorf("gemini had an error generating plan : %w", err)
	}
	var sb strings.Builder
	if len(result.Candidates) <= 0 {
		return Plan{}, fmt.Errorf("no candidates found in result")
	}
	candidate := result.Candidates[0]
	if candidate != nil && candidate.Content != nil && candidate.Content.Parts != nil {
		for _, part := range candidate.Content.Parts {
			if part != nil {
				if part.Thought {
					continue
				}
				if part.Text != "" {
					sb.WriteString(part.Text)
					sb.WriteString("\n")
				}
			}
		}
	}
	fmt.Println("result", sb)
	return RunIntradayAgentResponse{
		Analysis: sb.String(),
	}, nil
}

type GetStockChangeArgs struct {
	SecurityID    int    `json:"securityId"`
	From          int64  `json:"from"` //seconds since epoch
	To            int64  `json:"to"`   //seconds since epoch
	FromPoint     string `json:"fromPoint,omitempty"`
	ToPoint       string `json:"toPoint,omitempty"`
	SplitAdjusted *bool  `json:"splitAdjusted,omitempty"`
}
type GetStockChangeResponse struct {
	Change        float64 `json:"chg"`
	ChangePercent float64 `json:"chgPct"`
}

func GetStockChange(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStockChangeArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Check if from timestamp is in the future (compared to current EST time)
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}

	currentTimeEST := time.Now().In(easternLocation)
	fromTimeEST := time.Unix(args.From, 0).In(easternLocation)
	if fromTimeEST.After(currentTimeEST) {
		return nil, fmt.Errorf("from time %s is in the future", fromTimeEST.Format("2006-01-02 15:04:05 MST"))
	}

	var fromDateString string
	var toDateString string
	// Validate fromPoint and toPoint values
	if args.FromPoint != "" {
		switch args.FromPoint {
		case "open", "high", "low", "close":
			fromDateString = time.Unix(args.From, 0).UTC().Format("2006-01-02")
		default:
			return nil, fmt.Errorf("invalid fromPoint '%s': must be one of 'open', 'high', 'low', 'close'", args.FromPoint)
		}
	}

	if args.ToPoint != "" {
		switch args.ToPoint {
		case "open", "high", "low", "close":
			toDateString = time.Unix(args.To, 0).UTC().Format("2006-01-02")
		default:
			return nil, fmt.Errorf("invalid toPoint '%s': must be one of 'open', 'high', 'low', 'close'", args.ToPoint)
		}
	}

	// Default SplitAdjusted to true if not provided
	splitAdjustedValue := true
	if args.SplitAdjusted != nil {
		splitAdjustedValue = *args.SplitAdjusted
	}

	var startPrice float64
	var endPrice float64
	ticker, err := postgres.GetTicker(conn, args.SecurityID, time.Unix(args.From, 0).UTC())
	if err != nil {
		return nil, fmt.Errorf("error getting ticker: %v", err)
	}
	if args.FromPoint == "" {
		fromTrade, err := polygon.GetTradeAtTimestamp(conn, args.SecurityID, time.Unix(args.From, 0).UTC(), true)
		if err != nil {
			return nil, fmt.Errorf("error getting from trade at timestamp: %v, %v", args.From, err)
		}
		startPrice = fromTrade.Price
	} else {
		fromBar, err := polygon.GetDailyOHLCVForTicker(context.Background(), conn.Polygon, ticker, fromDateString, splitAdjustedValue)
		if err != nil {
			return nil, fmt.Errorf("error getting from trade at timestamp: %v, %v", args.From, err)
		}
		switch args.FromPoint {
		case "open":
			startPrice = fromBar.Open
		case "high":
			startPrice = fromBar.High
		case "low":
			startPrice = fromBar.Low
		case "close":
			startPrice = fromBar.Close
		}
	}

	if args.ToPoint == "" {
		toTrade, err := polygon.GetTradeAtTimestamp(conn, args.SecurityID, time.Unix(args.To, 0).UTC(), true)
		if err != nil {
			return nil, fmt.Errorf("error getting to trade at timestamp: %v, %v", args.To, err)
		}
		endPrice = toTrade.Price
	} else {
		toBar, err := polygon.GetDailyOHLCVForTicker(context.Background(), conn.Polygon, ticker, toDateString, splitAdjustedValue)
		if err != nil {
			return nil, fmt.Errorf("error getting to trade at timestamp: %v, %v", args.To, err)
		}
		switch args.ToPoint {
		case "open":
			endPrice = toBar.Open
		case "high":
			endPrice = toBar.High
		case "low":
			endPrice = toBar.Low
		case "close":
			endPrice = toBar.Close
		}
	}
	change := endPrice - startPrice
	changePercent := (change / startPrice) * 100

	return GetStockChangeResponse{
		Change:        math.Round(change*1000) / 1000,
		ChangePercent: math.Round(changePercent*1000) / 1000,
	}, nil
}

type GetStockPriceAtTimeArgs struct {
	SecurityID    int   `json:"securityId"`
	Timestamp     int64 `json:"timestamp"`
	SplitAdjusted *bool `json:"splitAdjusted,omitempty"`
}

type GetStockPriceAtTimeResponse struct {
	Ticker     string  `json:"ticker"`
	SecurityID int     `json:"securityId"`
	Time       string  `json:"time"`
	Price      float64 `json:"price"`
}

func GetStockPriceAtTime(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStockPriceAtTimeArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("issue loading eastern location: %v", err)
	}

	// Check if timestamp is in the future (compared to current EST time)
	currentTimeEST := time.Now().In(easternLocation)
	timestamp := time.Unix(args.Timestamp, 0).In(easternLocation)
	if timestamp.After(currentTimeEST) {
		return nil, fmt.Errorf("requested time %s is in the future", timestamp.Format("2006-01-02 15:04:05 MST"))
	}
	ticker, err := postgres.GetTicker(conn, args.SecurityID, timestamp)
	if err != nil {
		return nil, fmt.Errorf("error getting ticker: %v", err)
	}
	lastTrade, err := polygon.GetTradeAtTimestamp(conn, args.SecurityID, timestamp, true)
	if err != nil {
		return nil, fmt.Errorf("error getting price at time: %v, %v", args.Timestamp, err)
	}
	return GetStockPriceAtTimeResponse{
		Ticker:     ticker,
		SecurityID: args.SecurityID,
		Time:       timestamp.Format("2006-01-02T15:04:05"),
		Price:      math.Round(lastTrade.Price*1000) / 1000,
	}, nil
}
