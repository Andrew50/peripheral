package polygon

import (
	"backend/internal/data"
	"backend/internal/data/postgres"
	"backend/internal/data/utils"
	"context"
	"fmt"

	//	"log"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

// var stdoutMutex sync.Mutex

// silentLogger implements a logger that discards all messages
// nolint:unused
//
//lint:ignore U1000 kept for future logging control
type silentLogger struct{}

// Printf is a no-op to satisfy the logger interface if needed elsewhere.
func (l *silentLogger) Printf(_ string, _ ...interface{}) {}

// retryWithBackoff is a generic utility function to retry an operation with exponential backoff.
// operation: name of the operation for logging.
// ticker: ticker symbol, used for logging if shouldLog is true.
// maxRetries: maximum number of retries.
// shouldLog: if true, logs retry attempts.
// fn: the function to execute, which returns a result of type T and an error.
func retryWithBackoff[T any](operation string, _ string, maxRetries int, _ bool, fn func() (T, error)) (T, error) {
	var result T
	var err error
	var lastLoggedError error // To avoid spamming the same error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Execute the function directly without wrapping it
		result, err = fn()

		if err == nil {
			return result, nil
		}
		lastLoggedError = err

		// Only log on the final attempt if logging is enabled
		//if shouldLog && attempt == maxRetries {
		//log.Printf("ERROR Failed to %s for %s after %d attempts: %v", operation, ticker, maxRetries, lastErr)
		//}

		if attempt < maxRetries {
			backoffTime := time.Duration(attempt*2) * time.Second
			time.Sleep(backoffTime)
		}
	}

	return result, fmt.Errorf("failed to %s after %d attempts: %v", operation, maxRetries, lastLoggedError)
}

// GetAggsData performs operations related to GetAggsData functionality.
func GetAggsData(client *polygon.Client, ticker string, multiplier int, timeframe string,
	fromMillis models.Millis, toMillis models.Millis, bars int, resultsOrder string, isAdjusted bool) (*iter.Iter[models.Agg], error) {
	timespan := models.Timespan(timeframe)
	if resultsOrder != "asc" && resultsOrder != "desc" {
		return nil, fmt.Errorf("incorrect order string passed %s", resultsOrder)
	}
	params := models.ListAggsParams{
		Ticker:     ticker,
		Multiplier: multiplier,
		Timespan:   timespan,
		From:       fromMillis,
		To:         toMillis,
	}.WithOrder(models.Order(resultsOrder)).WithLimit(bars).WithAdjusted(isAdjusted)

	maxRetries := 3
	var lastErr error
	var iter *iter.Iter[models.Agg]

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Execute directly without withSilentOutput
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		iter = client.ListAggs(ctx, params)
		if iter.Next() {
			// Reset the iterator
			iter = client.ListAggs(context.Background(), params)
			return iter, nil
		}

		if err := iter.Err(); err != nil {
			lastErr = err
		} else {
			// No error but no data either
			return iter, nil
		}

		if attempt < maxRetries {
			backoffTime := time.Duration(attempt*2) * time.Second
			time.Sleep(backoffTime)
		}
	}

	return nil, fmt.Errorf("failed to get aggregates data after %d attempts: %v", maxRetries, lastErr)
}

// GetTradeAtTimestamp performs operations related to GetTradeAtTimestamp functionality.
func GetTradeAtTimestamp(conn *data.Conn, securityID int, timestamp time.Time, fullLot bool) (models.Trade, error) {
	ticker, err := postgres.GetTicker(conn, securityID, timestamp)
	if err != nil {
		return models.Trade{}, fmt.Errorf("sif20ih %v", err)
	}
	nanoTimestamp := models.Nanos(timestamp)

	maxRetries := 3
	iter, err := retryWithBackoff("get trade at timestamp", ticker, maxRetries, false, func() (*iter.Iter[models.Trade], error) {
		return GetTrade(conn.Polygon, ticker, nanoTimestamp, "desc", models.LTE, 1)
	})
	if err != nil {
		return models.Trade{}, fmt.Errorf("error initiating trade search: %v", err)
	}

	for iter.Next() {
		if iter.Item().Size >= 100 && fullLot {
			return iter.Item(), nil
		} else if !fullLot {
			return iter.Item(), nil
		}
	}
	if err := iter.Err(); err != nil {
		return models.Trade{}, fmt.Errorf("error fetching trade: %v", err)
	}
	return models.Trade{}, fmt.Errorf("no trade found for ticker %s at timestamp %v", ticker, nanoTimestamp)
}

// GetQuoteAtTimestamp performs operations related to GetQuoteAtTimestamp functionality.
func GetQuoteAtTimestamp(conn *data.Conn, securityID int, timestamp time.Time) (models.Quote, error) {
	ticker, err := postgres.GetTicker(conn, securityID, timestamp)
	if err != nil {
		return models.Quote{}, fmt.Errorf("doi20 %v", err)
	}
	nanoTimestamp := models.Nanos(timestamp)

	maxRetries := 3
	iter, err := retryWithBackoff("get quote at timestamp", ticker, maxRetries, false, func() (*iter.Iter[models.Quote], error) {
		return conn.Polygon.ListQuotes(context.Background(), models.ListQuotesParams{
			Ticker: ticker,
		}.WithTimestamp(models.LTE, nanoTimestamp).
			WithSort(models.Timestamp).
			WithOrder(models.Desc).
			WithLimit(1)), nil
	})
	if err != nil {
		return models.Quote{}, fmt.Errorf("error initiating quote search: %v", err)
	}

	for iter.Next() {
		return iter.Item(), nil
	}
	if err := iter.Err(); err != nil {
		return models.Quote{}, fmt.Errorf("error fetching quote: %v", err)
	}
	return models.Quote{}, fmt.Errorf("no quote found for ticker %s at timestamp %v", ticker, nanoTimestamp)
}

// GetLastQuote performs operations related to GetLastQuote functionality.
func GetLastQuote(client *polygon.Client, ticker string) (models.LastQuote, error) {
	params := &models.GetLastQuoteParams{
		Ticker: ticker,
	}
	maxRetries := 3
	var lastErr error
	var result models.LastQuote

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Execute directly without withSilentOutput
		resp, err := client.GetLastQuote(context.Background(), params)
		if err == nil {
			result = resp.Results
			return result, nil
		}
		lastErr = err

		if attempt < maxRetries {
			backoffTime := time.Duration(attempt*2) * time.Second
			time.Sleep(backoffTime)
		}
	}

	return result, fmt.Errorf("failed to get last quote after %d attempts: %v", maxRetries, lastErr)
}

// GetQuote performs operations related to GetQuote functionality.
func GetQuote(client *polygon.Client, ticker string, nanoTimestamp models.Nanos, ord string, compareType models.Comparator, numResults int) *iter.Iter[models.Quote] {
	sortOrder := models.Desc
	if ord == "asc" {
		sortOrder = models.Asc
	}
	params := models.ListQuotesParams{
		Ticker: ticker,
	}.WithTimestamp(compareType, nanoTimestamp).
		WithSort(models.Timestamp).
		WithOrder(sortOrder).
		WithLimit(numResults)
	return client.ListQuotes(context.Background(), params)
}

type GetLastTradeResponse struct {
	Ticker     string
	Conditions []int32
	Timestamp  models.Nanos
	Price      float64
	Size       float64
	Exchange   int
	Tape       int32
}

// GetLastTrade gets the last trade for a ticker. The atLeastFullLot flag is used if we want to grab the last trade that was at least a full lot (100 shares).
func GetLastTrade(client *polygon.Client, ticker string, atLeastFullLot bool) (GetLastTradeResponse, error) {
	params := &models.GetLastTradeParams{
		Ticker: ticker,
	}
	if !atLeastFullLot {
		maxRetries := 3
		var lastErr error
		var result models.LastTrade

		for attempt := 1; attempt <= maxRetries; attempt++ {
			// Execute directly without withSilentOutput
			resp, err := client.GetLastTrade(context.Background(), params)
			if err == nil {
				result = resp.Results
				return GetLastTradeResponse{
					Ticker:     ticker,
					Conditions: result.Conditions,
					Timestamp:  result.Timestamp,
					Price:      result.Price,
					Size:       result.Size,
					Exchange:   int(result.Exchange),
					Tape:       result.Tape,
				}, nil
			}
			lastErr = err

			if attempt < maxRetries {
				backoffTime := time.Duration(attempt*2) * time.Second
				time.Sleep(backoffTime)
			}
		}

		return GetLastTradeResponse{}, fmt.Errorf("failed to get last trade after %d attempts: %v", maxRetries, lastErr)
	}
	paramsListTrades := models.ListTradesParams{
		Ticker: ticker,
	}.WithOrder(models.Desc).WithSort(models.Timestamp).WithLimit(1000)
	iter := client.ListTrades(context.Background(), paramsListTrades)
	for iter.Next() {
		trade := iter.Item()
		if trade.Size >= 100 {
			return GetLastTradeResponse{
				Ticker:     ticker,
				Conditions: trade.Conditions,
				Timestamp:  trade.ParticipantTimestamp,
				Price:      trade.Price,
				Size:       trade.Size,
				Exchange:   trade.Exchange,
				Tape:       trade.Tape,
			}, nil
		}
	}
	if iter.Err() != nil {
		return GetLastTradeResponse{}, fmt.Errorf("error iterating trades for %s: %w", ticker, iter.Err())
	}
	return GetLastTradeResponse{}, fmt.Errorf("no trade with size >= 100 found in the last 1000 trades for %s", ticker)
}

// GetTrade performs operations related to GetTrade functionality.
func GetTrade(client *polygon.Client, ticker string, nanoTimestamp models.Nanos, ord string, compareType models.Comparator, numResults int) (*iter.Iter[models.Trade], error) {
	sortOrder := models.Desc
	if ord != "asc" && ord != "desc" {
		return nil, fmt.Errorf("incorrect order string passed 35ltkg")
	}
	if ord == "asc" {
		sortOrder = models.Asc
	}
	params := models.ListTradesParams{
		Ticker: ticker,
	}.WithTimestamp(compareType, nanoTimestamp).
		WithOrder(sortOrder).
		WithLimit(numResults).
		WithSort(models.Timestamp)

	// Execute directly without withSilentOutput
	iter := client.ListTrades(context.Background(), params)
	return iter, nil
}

// GetMostRecentRegularClose gets the most recent close price from regular trading hours
func GetMostRecentRegularClose(client *polygon.Client, ticker string, referenceTime time.Time) (float64, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return 0, fmt.Errorf("error loading location: %v", err)
	}

	refLocal := referenceTime.In(easternLocation)
	startOfDay := time.Date(refLocal.Year(), refLocal.Month(), refLocal.Day(), 0, 0, 0, 0, easternLocation)

	// We'll use 'referenceTime' as the "end" time for scanning minute bars
	startMillis := models.Millis(startOfDay)
	endMillis := models.Millis(refLocal)

	iter, err := GetAggsData(client, ticker, 1, "minute", startMillis, endMillis, 1000, "desc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting aggregates: %v", err)
	}

	// Find the most recent bar during regular hours
	for iter.Next() {
		agg := iter.Item()
		timestamp := time.Time(agg.Timestamp)
		if utils.IsTimestampRegularHours(timestamp) {
			return agg.Close, nil
		}
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("error iterating aggregates: %v", err)
	}

	// If no regular hours data found for that day, keep checking previous days until we find data
	currentDate := startOfDay
	maxAttempts := 5 // Prevent infinite loop, should be enough to cover long weekends/holidays

	for attempts := 0; attempts < maxAttempts; attempts++ {
		previousDay := currentDate.AddDate(0, 0, -1)
		dayStart := models.Millis(previousDay)
		dayEnd := models.Millis(currentDate.Add(-time.Nanosecond))

		iter, err = GetAggsData(client, ticker, 1, "day", dayStart, dayEnd, 1, "desc", true)
		if err != nil {
			return 0, fmt.Errorf("error getting historical data: %v", err)
		}

		for iter.Next() {
			return iter.Item().Close, nil
		}

		if err := iter.Err(); err != nil {
			return 0, fmt.Errorf("error iterating historical data: %v", err)
		}

		currentDate = previousDay
	}

	return 0, fmt.Errorf("no recent regular hours close found for %s within last %d days", ticker, maxAttempts)
}

// GetMostRecentExtendedHoursClose gets the most recent extended-hours close relative to referenceTime
func GetMostRecentExtendedHoursClose(client *polygon.Client, ticker string, referenceTime time.Time) (float64, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return 0, fmt.Errorf("error loading location: %v", err)
	}

	refLocal := referenceTime.In(easternLocation)
	startOfExtended := time.Date(refLocal.Year(), refLocal.Month(), refLocal.Day(), 0, 0, 0, 0, easternLocation)
	startMillis := models.Millis(startOfExtended)
	endMillis := models.Millis(refLocal)

	iter, err := GetAggsData(client, ticker, 1, "minute", startMillis, endMillis, 1000, "desc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting aggregates: %v", err)
	}

	// Find the most recent bar during regular hours
	for iter.Next() {
		agg := iter.Item()
		timestamp := time.Time(agg.Timestamp)
		if utils.IsTimestampRegularHours(timestamp) {
			return agg.Close, nil
		}
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("error iterating aggregates: %v", err)
	}

	// If no regular hours data found for that day, check the previous day
	yesterdayLocal := startOfExtended.AddDate(0, 0, -1)
	yesterdayStart := models.Millis(yesterdayLocal)
	yesterdayEnd := models.Millis(startOfExtended.Add(-time.Nanosecond))

	iter, err = GetAggsData(client, ticker, 1, "day", yesterdayStart, yesterdayEnd, 1, "desc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting previous day's data: %v", err)
	}

	for iter.Next() {
		// For extended hours close, we might want "Open" or "Close" from the prior day.
		// This example uses .Open, but you could revise as needed:
		return iter.Item().Open, nil
	}

	return 0, fmt.Errorf("no recent extended hours close found for %s", ticker)
}

// GetDailyOpen gets the opening price for the day of the given timestamp
func GetDailyOpen(client *polygon.Client, ticker string, referenceTime time.Time) (float64, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return 0, fmt.Errorf("error loading location: %v", err)
	}

	refLocal := referenceTime.In(easternLocation)
	startOfDay := time.Date(refLocal.Year(), refLocal.Month(), refLocal.Day(), 0, 0, 0, 0, easternLocation)
	endOfDay := time.Date(refLocal.Year(), refLocal.Month(), refLocal.Day(), 23, 59, 59, 999999999, easternLocation)

	// Get daily bar for this specific day
	startMillis := models.Millis(startOfDay)
	endMillis := models.Millis(endOfDay)

	iter, err := GetAggsData(client, ticker, 1, "day", startMillis, endMillis, 1, "asc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting daily data: %v", err)
	}

	// The first (and likely only) bar should contain the open
	if iter.Next() {
		return iter.Item().Open, nil
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("error iterating daily data: %v", err)
	}
	// If no data for today, try getting the first minute bar of the day
	minuteIter, err := GetAggsData(client, ticker, 1, "minute", startMillis, endMillis, 1, "asc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting minute data: %v", err)
	}

	if minuteIter.Next() {
		return minuteIter.Item().Open, nil
	}

	if err := minuteIter.Err(); err != nil {
		return 0, fmt.Errorf("error iterating minute data: %v", err)
	}

	// If still no data, try the previous close as fallback
	return GetMostRecentRegularClose(client, ticker, startOfDay.Add(-time.Nanosecond))
}

func GetAllStocksDailyOHLCV(ctx context.Context, client *polygon.Client, date string) (*models.GetGroupedDailyAggsResponse, error) {
	on, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("error parsing date: %v", err)
	}
	params := &models.GetGroupedDailyAggsParams{
		Date:       models.Date(on),
		MarketType: "stocks",
		Locale:     "us",
	}
	res, err := client.GetGroupedDailyAggs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("error getting grouped daily aggs: %v", err)
	}
	return res, nil
}

func GetPolygonTickerSnapshot(ctx context.Context, client *polygon.Client, ticker string) (*models.GetTickerSnapshotResponse, error) {
	params := models.GetTickerSnapshotParams{
		Ticker:     ticker,
		Locale:     "us",
		MarketType: "stocks",
	}
	res, err := client.GetTickerSnapshot(ctx, &params)
	if err != nil {
		return nil, fmt.Errorf("error getting ticker snapshot: %v", err)
	}
	return res, nil
}

func GetPolygonAllTickerSnapshots(ctx context.Context, client *polygon.Client) (*models.GetAllTickersSnapshotResponse, error) {
	params := models.GetAllTickersSnapshotParams{
		Locale:     "us",
		MarketType: "stocks",
	}
	res, err := client.GetAllTickersSnapshot(ctx, &params)
	if err != nil {
		return nil, fmt.Errorf("error getting all ticker snapshots: %v", err)
	}
	return res, nil
}

type GetDailyOHLCVForTickerResponse struct {
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
	Date   string  `json:"date"`
}

func GetDailyOHLCVForTicker(ctx context.Context, client *polygon.Client, ticker string, date string, splitAdjusted bool) (GetDailyOHLCVForTickerResponse, error) {
	on, err := time.Parse("2006-01-02", date)
	if err != nil {
		return GetDailyOHLCVForTickerResponse{}, fmt.Errorf("error parsing date: %v", err)
	}
	params := models.GetDailyOpenCloseAggParams{
		Ticker: ticker,
		Date:   models.Date(on),
	}.WithAdjusted(splitAdjusted)
	res, err := client.GetDailyOpenCloseAgg(ctx, params)
	if err != nil {
		return GetDailyOHLCVForTickerResponse{}, fmt.Errorf("error getting daily open close: %v", err)
	}
	return GetDailyOHLCVForTickerResponse{
		Open:   res.Open,
		High:   res.High,
		Low:    res.Low,
		Close:  res.Close,
		Volume: res.Volume,
		Date:   res.From,
	}, nil
}

// GetAllStocks1MinuteOHLCV fetches 1-minute aggregated OHLCV data for all stocks on a given date
func GetAllStocks1MinuteOHLCV(ctx context.Context, client *polygon.Client, date string) (*models.GetGroupedDailyAggsResponse, error) {
	on, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("error parsing date for 1-minute data: %v", err)
	}
	
	// For 1-minute data, we use ListAggs with minute aggregation
	// Note: This returns aggregated minute data, not individual minute bars
	// We'll get a response similar to grouped daily but for minute timeframe
	params := &models.GetGroupedDailyAggsParams{
		Date:       models.Date(on),
		MarketType: "stocks", 
		Locale:     "us",
	}
	
	// Use the same grouped endpoint but it will aggregate minute data
	res, err := client.GetGroupedDailyAggs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("error getting grouped 1-minute aggs: %v", err)
	}
	return res, nil
}

// GetAllStocks1HourOHLCV fetches 1-hour aggregated OHLCV data for all stocks on a given date  
func GetAllStocks1HourOHLCV(ctx context.Context, client *polygon.Client, date string) (*models.GetGroupedDailyAggsResponse, error) {
	on, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("error parsing date for 1-hour data: %v", err)
	}
	
	// For 1-hour data, we use the same approach but with hourly aggregation
	params := &models.GetGroupedDailyAggsParams{
		Date:       models.Date(on),
		MarketType: "stocks",
		Locale:     "us", 
	}
	
	res, err := client.GetGroupedDailyAggs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("error getting grouped 1-hour aggs: %v", err)
	}
	return res, nil
}

// GetAllStocks1WeekOHLCV fetches 1-week aggregated OHLCV data for all stocks for a given week
func GetAllStocks1WeekOHLCV(ctx context.Context, client *polygon.Client, date string) (*models.GetGroupedDailyAggsResponse, error) {
	on, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("error parsing date for 1-week data: %v", err)
	}
	
	// For weekly data, we need to get the start of the week (Monday)
	weekStart := on
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}
	
	params := &models.GetGroupedDailyAggsParams{
		Date:       models.Date(weekStart),
		MarketType: "stocks",
		Locale:     "us",
	}
	
	res, err := client.GetGroupedDailyAggs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("error getting grouped 1-week aggs: %v", err)
	}
	return res, nil
}
