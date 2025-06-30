package marketdata

import (
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/postgres"
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/polygon-io/client-go/rest/models"
	"golang.org/x/sync/semaphore"
)

// Update1HourOHLCV fetches and stores 1-hour OHLCV data from Polygon API
func Update1HourOHLCV(conn *data.Conn) error {
	defer func() {
		// Log completion time for monitoring
	}()

	today := time.Now().Format("2006-01-02")
	if time.Now().Hour() < 17 {
		today = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}

	// Check latest timestamp in database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	maxDateRows, err := conn.DB.Query(ctx, "SELECT MAX(timestamp) FROM ohlcv_1h")
	if err != nil {
		return fmt.Errorf("error getting max date in ohlcv_1h table: %w", err)
	}
	defer maxDateRows.Close()

	var maxDate time.Time
	var nullableMaxDate *time.Time
	hasRows := false

	for maxDateRows.Next() {
		hasRows = true
		err = maxDateRows.Scan(&nullableMaxDate)
		if err != nil {
			return fmt.Errorf("error scanning max date: %w", err)
		}
		if nullableMaxDate != nil {
			maxDate = *nullableMaxDate
		}
	}

	// Set default start date if no existing data
	if !hasRows || nullableMaxDate == nil || maxDate.IsZero() {
		// Start from default date for initial hourly data load
		maxDate = time.Date(2003, 10, 1, 0, 0, 0, 0, time.UTC)
	}

	if maxDate.Format("2006-01-02") == today {
		return nil // Already up to date
	}

	// Collect all dates that need to be processed
	dates := []time.Time{}
	currentDate := maxDate.AddDate(0, 0, 1)
	todayTime, _ := time.Parse("2006-01-02", today)

	for currentDate.Before(todayTime) || currentDate.Equal(todayTime) {
		// Skip weekends (no market data)
		if currentDate.Weekday() != time.Saturday && currentDate.Weekday() != time.Sunday {
			dates = append(dates, currentDate)
		}
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	if len(dates) == 0 {
		return nil // No dates to process
	}

	// Use thread-safe security cache
	var securityCache sync.Map

	// Moderate concurrency for hourly data
	maxConcurrency := 3
	sem := semaphore.NewWeighted(int64(maxConcurrency))
	var wg sync.WaitGroup
	errorCh := make(chan error, len(dates))

	// Global context for all goroutines
	globalCtx, globalCancel := context.WithCancel(context.Background())
	defer globalCancel()

	for _, date := range dates {
		// Acquire semaphore
		if err := sem.Acquire(globalCtx, 1); err != nil {
			break
		}

		wg.Add(1)
		go func(date time.Time) {
			defer func() {
				if r := recover(); r != nil {
					errorCh <- fmt.Errorf("panic processing 1-hour data for %s: %v",
						date.Format("2006-01-02"), r)
				}
			}()
			defer wg.Done()
			defer sem.Release(1)

			dateStr := date.Format("2006-01-02")

			// Create context with timeout for this date's processing
			ctx, cancel := context.WithTimeout(globalCtx, 45*time.Second) // Moderate timeout for hourly data
			defer cancel()

			// Get 1-hour OHLCV data for this date
			ohlcvResponse, err := polygon.GetAllStocks1HourOHLCV(ctx, conn.Polygon, dateStr)
			if err != nil {
				errorCh <- fmt.Errorf("error getting 1-hour OHLCV for %s: %w", dateStr, err)
				return
			}

			if ohlcvResponse == nil || ohlcvResponse.ResultsCount == 0 {
				return // No data for this date
			}

			// Process and store the data
			err = store1HourOHLCVParallel(conn, ohlcvResponse, date, &securityCache)
			if err != nil {
				errorCh <- fmt.Errorf("error storing 1-hour OHLCV for %s: %w", dateStr, err)
				return
			}
		}(date)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorCh)

	// Check if any errors occurred
	for err := range errorCh {
		return err // Return first error
	}

	return nil
}

// store1HourOHLCVParallel stores 1-hour OHLCV data using parallel batch processing
func store1HourOHLCVParallel(conn *data.Conn, ohlcvResponse *models.GetGroupedDailyAggsResponse, date time.Time, securityCache *sync.Map) error {
	results := ohlcvResponse.Results
	if len(results) == 0 {
		return nil
	}

	// Use moderate batch size for hourly data
	const batchSize = 400

	// Calculate number of batches
	batchCount := int(math.Ceil(float64(len(results)) / float64(batchSize)))

	// Process batches with controlled concurrency
	var wg sync.WaitGroup
	maxConcurrency := 1 // REDUCED to 1 to prevent deadlocks in TimescaleDB
	sem := semaphore.NewWeighted(int64(maxConcurrency))
	errorCh := make(chan error, batchCount)

	// Global context for all goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second) // Longer timeout due to reduced concurrency
	defer cancel()

	// Pre-collect all tickers for this date
	allTickers := make(map[string]bool)
	for _, record := range results {
		allTickers[record.Ticker] = true
	}

	// Batch preload securities
	dateSecurities := &sync.Map{}
	batchPreload1HourTickers(conn, allTickers, date, dateSecurities)

	for i := 0; i < len(results); i += batchSize {
		// Acquire semaphore
		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire semaphore: %w", err)
		}

		wg.Add(1)
		go func(startIdx int) {
			defer wg.Done()
			defer sem.Release(1)

			// Thread-local cache
			localCache := make(map[string]int)

			// Determine end of current batch
			endIdx := startIdx + batchSize
			if endIdx > len(results) {
				endIdx = len(results)
			}

			currentBatch := results[startIdx:endIdx]

			// Start transaction for this batch
			batchCtx, batchCancel := context.WithTimeout(ctx, 45*time.Second)
			defer batchCancel()

			tx, err := conn.DB.Begin(batchCtx)
			if err != nil {
				errorCh <- fmt.Errorf("error beginning transaction for 1-hour batch %d-%d: %w",
					startIdx, endIdx, err)
				return
			}

			// Ensure transaction cleanup
			committed := false
			defer func() {
				if !committed {
					if rbErr := tx.Rollback(context.Background()); rbErr != nil {
						// Log rollback error
						_ = rbErr
					}
				}
			}()

			// Collect security IDs and build records
			recordsToProcess := make([]struct {
				record     models.Agg
				securityID int
			}, 0, len(currentBatch))

			for _, record := range currentBatch {
				ticker := record.Ticker
				cacheKey := securityCacheKey(ticker, date)

				// Check local cache first
				securityID, exists := localCache[cacheKey]

				// Check shared cache
				if !exists {
					if cachedID, found := securityCache.Load(cacheKey); found {
						securityID = cachedID.(int)
						localCache[cacheKey] = securityID
						exists = true
					}
				}

				// Database lookup if not in cache
				if !exists {
					securityID, err = postgres.GetSecurityID(conn, ticker, date)
					if err != nil {
						continue // Skip invalid securities
					}

					localCache[cacheKey] = securityID
					securityCache.Store(cacheKey, securityID)
				}

				recordsToProcess = append(recordsToProcess, struct {
					record     models.Agg
					securityID int
				}{record: record, securityID: securityID})
			}

			// Skip if no valid records
			if len(recordsToProcess) == 0 {
				return
			}

			// Build batch insert query
			valueStrings := make([]string, 0, len(recordsToProcess))
			valueArgs := make([]interface{}, 0, len(recordsToProcess)*7)
			argPosition := 1

			for _, item := range recordsToProcess {
				record := item.record
				securityID := item.securityID

				// Convert Polygon Millis timestamp to time.Time (UTC)
				tsUTC := time.Time(record.Timestamp).UTC()

				valueStrings = append(valueStrings,
					fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
						argPosition, argPosition+1, argPosition+2, argPosition+3,
						argPosition+4, argPosition+5, argPosition+6))

				valueArgs = append(valueArgs,
					tsUTC, securityID, record.Open, record.High,
					record.Low, record.Close, record.Volume)

				argPosition += 7
			}

			// Execute batch insert for 1-hour table
			query := fmt.Sprintf(
				"INSERT INTO ohlcv_1h (timestamp, securityid, open, high, low, close, volume) VALUES %s",
				valueStrings[0])

			for i := 1; i < len(valueStrings); i++ {
				query += ", " + valueStrings[i]
			}

			// Add conflict resolution
			query += " ON CONFLICT (timestamp, securityid) DO NOTHING"

			_, err = tx.Exec(batchCtx, query, valueArgs...)
			if err != nil {
				errorCh <- fmt.Errorf("error executing 1-hour batch insert for records %d-%d: %w",
					startIdx, endIdx, err)
				return
			}

			// Commit transaction
			if err = tx.Commit(batchCtx); err != nil {
				errorCh <- fmt.Errorf("error committing 1-hour transaction for batch %d-%d: %w",
					startIdx, endIdx, err)
				return
			}
			committed = true

		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorCh)

	// Check for errors
	for err := range errorCh {
		return err
	}

	return nil
}

// batchPreload1HourTickers preloads securities for 1-hour data processing
func batchPreload1HourTickers(conn *data.Conn, tickers map[string]bool, date time.Time, securityCache *sync.Map) {
	tickerList := make([]string, 0, len(tickers))
	for ticker := range tickers {
		tickerList = append(tickerList, ticker)
	}

	if len(tickerList) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	dateStr := date.Format("2006-01-02")

	query := `
		SELECT ticker, securityid 
		FROM securities 
		WHERE ticker = ANY($1)
		AND (minDate <= $2)
		AND (maxDate >= $2 OR maxDate IS NULL)
	`

	rows, err := conn.DB.Query(ctx, query, tickerList, dateStr)
	if err != nil {
		return // Log warning but continue
	}
	defer rows.Close()

	// Store results in thread-safe map
	for rows.Next() {
		var ticker string
		var id int
		if err := rows.Scan(&ticker, &id); err != nil {
			continue
		}

		cacheKey := securityCacheKey(ticker, date)
		securityCache.Store(cacheKey, id)
	}
}
