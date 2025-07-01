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

// Update1WeekOHLCV fetches and stores 1-week OHLCV data from Polygon API
func Update1WeekOHLCV(conn *data.Conn) error {
	defer func() {
		// Log completion time for monitoring
	}()

	// Get current time and determine target week range
	now := time.Now()
	// Start from last Monday (beginning of current/previous week)
	targetDate := now
	for targetDate.Weekday() != time.Monday {
		targetDate = targetDate.AddDate(0, 0, -1)
	}

	// If it's before market close on Monday, use previous week
	if now.Weekday() == time.Monday && now.Hour() < 17 {
		targetDate = targetDate.AddDate(0, 0, -7)
	}

	// Check latest timestamp in database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	maxDateRows, err := conn.DB.Query(ctx, "SELECT MAX(timestamp) FROM ohlcv_1w")
	if err != nil {
		return fmt.Errorf("error getting max date in ohlcv_1w table: %w", err)
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
		// Start from default date for initial weekly data load
		maxDate = time.Date(2003, 10, 1, 0, 0, 0, 0, time.UTC)
		// Align to Monday
		for maxDate.Weekday() != time.Monday {
			maxDate = maxDate.AddDate(0, 0, -1)
		}
	}

	// Check if we're already up to date
	if maxDate.Format("2006-01-02") == targetDate.Format("2006-01-02") {
		return nil // Already up to date
	}

	// Collect week starts to process
	weekStarts := []time.Time{}
	currentWeek := maxDate
	if !maxDate.IsZero() {
		currentWeek = maxDate.AddDate(0, 0, 7) // Next week
	}

	// Align current week to Monday
	for currentWeek.Weekday() != time.Monday {
		currentWeek = currentWeek.AddDate(0, 0, -1)
	}

	for currentWeek.Before(targetDate) || currentWeek.Equal(targetDate) {
		weekStarts = append(weekStarts, currentWeek)
		currentWeek = currentWeek.AddDate(0, 0, 7) // Next week
	}

	if len(weekStarts) == 0 {
		return nil // No weeks to process
	}

	// Use thread-safe security cache
	var securityCache sync.Map

	// Higher concurrency for weekly data (less API load)
	maxConcurrency := 5
	sem := semaphore.NewWeighted(int64(maxConcurrency))
	var wg sync.WaitGroup
	errorCh := make(chan error, len(weekStarts))

	// Global context for all goroutines
	globalCtx, globalCancel := context.WithCancel(context.Background())
	defer globalCancel()

	for _, weekStart := range weekStarts {
		// Acquire semaphore
		if err := sem.Acquire(globalCtx, 1); err != nil {
			break
		}

		wg.Add(1)
		go func(weekStart time.Time) {
			defer func() {
				if r := recover(); r != nil {
					errorCh <- fmt.Errorf("panic processing 1-week data for %s: %v",
						weekStart.Format("2006-01-02"), r)
				}
			}()
			defer wg.Done()
			defer sem.Release(1)

			weekStr := weekStart.Format("2006-01-02")

			// Create context with timeout for this week's processing
			ctx, cancel := context.WithTimeout(globalCtx, 30*time.Second) // Short timeout for weekly data
			defer cancel()

			// Get 1-week OHLCV data for this week
			ohlcvResponse, err := polygon.GetAllStocks1WeekOHLCV(ctx, conn.Polygon, weekStr)
			if err != nil {
				errorCh <- fmt.Errorf("error getting 1-week OHLCV for %s: %w", weekStr, err)
				return
			}

			if ohlcvResponse == nil || ohlcvResponse.ResultsCount == 0 {
				return // No data for this week
			}

			// Process and store the data
			err = store1WeekOHLCVParallel(conn, ohlcvResponse, weekStart, &securityCache)
			if err != nil {
				errorCh <- fmt.Errorf("error storing 1-week OHLCV for %s: %w", weekStr, err)
				return
			}
		}(weekStart)
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

// store1WeekOHLCVParallel stores 1-week OHLCV data using parallel batch processing
func store1WeekOHLCVParallel(conn *data.Conn, ohlcvResponse *models.GetGroupedDailyAggsResponse, weekStart time.Time, securityCache *sync.Map) error {
	results := ohlcvResponse.Results
	if len(results) == 0 {
		return nil
	}

	// Use large batch size for weekly data (low volume)
	const batchSize = 800

	// Calculate number of batches
	batchCount := int(math.Ceil(float64(len(results)) / float64(batchSize)))

	// Process batches with REDUCED concurrency to prevent deadlocks
	var wg sync.WaitGroup
	maxConcurrency := 1 // REDUCED to 1 to prevent deadlocks in TimescaleDB
	sem := semaphore.NewWeighted(int64(maxConcurrency))
	errorCh := make(chan error, batchCount)

	// Global context for all goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second) // Longer timeout due to reduced concurrency
	defer cancel()

	// Pre-collect all tickers for this week
	allTickers := make(map[string]bool)
	for _, record := range results {
		allTickers[record.Ticker] = true
	}

	// Batch preload securities
	dateSecurities := &sync.Map{}
	batchPreload1WeekTickers(conn, allTickers, weekStart, dateSecurities)

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

			// Start transaction for this batch with longer timeout
			batchCtx, batchCancel := context.WithTimeout(ctx, 60*time.Second)
			defer batchCancel()

			tx, err := conn.DB.Begin(batchCtx)
			if err != nil {
				errorCh <- fmt.Errorf("error beginning transaction for 1-week batch %d-%d: %w",
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
				cacheKey := securityCacheKey(ticker, weekStart)

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
					securityID, err = postgres.GetSecurityID(conn, ticker, weekStart)
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

				// For weekly data, use the week start timestamp
				weekStartTimestamp := weekStart

				valueStrings = append(valueStrings,
					fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
						argPosition, argPosition+1, argPosition+2, argPosition+3,
						argPosition+4, argPosition+5, argPosition+6))

				valueArgs = append(valueArgs,
					weekStartTimestamp, securityID, record.Open, record.High,
					record.Low, record.Close, record.Volume)

				argPosition += 7
			}

			// Execute batch insert for 1-week table
			query := fmt.Sprintf(
				"INSERT INTO ohlcv_1w (timestamp, securityid, open, high, low, close, volume) VALUES %s",
				valueStrings[0])

			for i := 1; i < len(valueStrings); i++ {
				query += ", " + valueStrings[i]
			}

			// Add conflict resolution with explicit locking to prevent deadlocks
			query += " ON CONFLICT (timestamp, securityid) DO NOTHING"

			_, err = tx.Exec(batchCtx, query, valueArgs...)
			if err != nil {
				errorCh <- fmt.Errorf("error executing 1-week batch insert for records %d-%d: %w",
					startIdx, endIdx, err)
				return
			}

			// Commit transaction
			if err = tx.Commit(batchCtx); err != nil {
				errorCh <- fmt.Errorf("error committing 1-week transaction for batch %d-%d: %w",
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

// batchPreload1WeekTickers preloads securities for 1-week data processing
func batchPreload1WeekTickers(conn *data.Conn, tickers map[string]bool, weekStart time.Time, securityCache *sync.Map) {
	tickerList := make([]string, 0, len(tickers))
	for ticker := range tickers {
		tickerList = append(tickerList, ticker)
	}

	if len(tickerList) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	dateStr := weekStart.Format("2006-01-02")

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

		cacheKey := securityCacheKey(ticker, weekStart)
		securityCache.Store(cacheKey, id)
	}
}
