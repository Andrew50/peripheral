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

func UpdateDailyOHLCV(conn *data.Conn) error {
	//start := time.Now()

	defer func() {
		////fmt.Printf("OHLCV update completed in %v\n", time.Since(start))
	}()

	today := time.Now().Format("2006-01-02")
	////fmt.Println("Starting daily ohlcv update for today:", today)
	if time.Now().Hour() < 17 {
		today = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}

	// Use a shorter timeout for the initial query
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	maxDateRows, err := conn.DB.Query(ctx, "SELECT MAX(timestamp) FROM ohlcv_1d")
	if err != nil {
		////fmt.Println("Error getting max date in ohlcv table:", err)
		return err
	}
	defer maxDateRows.Close()

	var maxDate time.Time
	var nullableMaxDate *time.Time
	hasRows := false

	for maxDateRows.Next() {
		hasRows = true
		err = maxDateRows.Scan(&nullableMaxDate)
		if err != nil {
			////fmt.Println("Error getting max date in ohlcv table:", err)
			return err
		}
		if nullableMaxDate != nil {
			maxDate = *nullableMaxDate
		}
	}

	// Set default date if no valid max date was found
	if !hasRows || nullableMaxDate == nil || maxDate.IsZero() {
		maxDate = time.Date(2003, 10, 1, 0, 0, 0, 0, time.UTC)
		////fmt.Println("No existing data found, starting from default date:", maxDate.Format("2006-01-02"))
	}

	if maxDate.Format("2006-01-02") == today {
		////fmt.Println("Max date in ohlcv table is today, skipping update")
		return nil
	}

	// Collect all dates that need to be processed
	dates := []time.Time{}
	currentDate := maxDate.AddDate(0, 0, 1)
	todayTime, _ := time.Parse("2006-01-02", today)

	for currentDate.Before(todayTime) || currentDate.Equal(todayTime) {
		if currentDate.Weekday() != time.Saturday && currentDate.Weekday() != time.Sunday {
			dates = append(dates, currentDate)
		}
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	if len(dates) == 0 {
		////fmt.Println("No dates to process")
		return nil
	}

	////fmt.Printf("Processing %d dates from %s to %s\n", len(dates), dates[0].Format("2006-01-02"), dates[len(dates)-1].Format("2006-01-02"))

	// Use a sync.Map for thread-safe access to the security cache
	var securityCache sync.Map

	// Use a semaphore to limit concurrent API calls
	maxConcurrency := 3 // Increase from 1 to 5
	sem := semaphore.NewWeighted(int64(maxConcurrency))
	var wg sync.WaitGroup
	errorCh := make(chan error, len(dates))

	// Global context for all goroutines
	globalCtx, globalCancel := context.WithCancel(context.Background())
	defer globalCancel()

	for _, date := range dates {
		// Acquire semaphore
		if err := sem.Acquire(globalCtx, 1); err != nil {
			////fmt.Printf("Failed to acquire semaphore: %v\n", err)
			break
		}

		wg.Add(1)
		go func(date time.Time) {
			// Add panic recovery
			defer func() {
				if r := recover(); r != nil {
					////fmt.Printf("Recovered from panic processing date %s: %v\n", date.Format("2006-01-02"), r)
					errorCh <- fmt.Errorf("panic processing date %s: %v",
						date.Format("2006-01-02"), r)
				}
			}()
			defer wg.Done()
			defer sem.Release(1)
			dateStr := date.Format("2006-01-02")

			// Create context with timeout for this date's processing
			ctx, cancel := context.WithTimeout(globalCtx, 20*time.Second)
			defer cancel()

			// Get OHLCV data for this date
			ohlcvResponse, err := polygon.GetAllStocksDailyOHLCV(ctx, conn.Polygon, dateStr)
			if err != nil {
				errorCh <- fmt.Errorf("error getting OHLCV for %s: %w", dateStr, err)
				return
			}

			if ohlcvResponse == nil || ohlcvResponse.ResultsCount == 0 {
				////fmt.Printf("No data found for date: %s\n", dateStr)
				return
			}

			// Process the data - using thread-safe sync.Map
			err = storeDailyOHLCVParallel(conn, ohlcvResponse, date, &securityCache)
			if err != nil {
				errorCh <- fmt.Errorf("error storing OHLCV for %s: %w", dateStr, err)
				return
			}
		}(date)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorCh)

	// Check if any errors occurred
	for err := range errorCh {
		////fmt.Println(err)
		return err // Return first error
	}

	return nil
}

// Create a time-aware key for the securities cache
func securityCacheKey(ticker string, date time.Time) string {
	return fmt.Sprintf("%s:%s", ticker, date.Format("2006-01-02"))
}

// Parallel version of storeDailyOHLCV - now using sync.Map for thread safety
func storeDailyOHLCVParallel(conn *data.Conn, ohlcvResponse *models.GetGroupedDailyAggsResponse, date time.Time, securityCache *sync.Map) error {
	results := ohlcvResponse.Results
	if len(results) == 0 {
		return nil
	}

	// Use a larger batch size for better performance
	const batchSize = 500

	// Calculate number of batches
	batchCount := int(math.Ceil(float64(len(results)) / float64(batchSize)))

	// Process batches in parallel with controlled concurrency
	var wg sync.WaitGroup
	maxConcurrency := 1 // Reduce back to 1 to prevent INSERT deadlocks
	sem := semaphore.NewWeighted(int64(maxConcurrency))
	errorCh := make(chan error, batchCount)

	// Global context for all goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First, pre-collect all tickers we'll need for this date
	allTickers := make(map[string]bool)
	for _, record := range results {
		allTickers[record.Ticker] = true
	}

	// Create a date-specific security cache
	dateSecurities := &sync.Map{}

	// Batch preload into the sync.Map
	batchPreloadTickers(conn, allTickers, date, dateSecurities)

	for i := 0; i < len(results); i += batchSize {
		// Acquire semaphore
		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire semaphore: %w", err)
		}

		wg.Add(1)
		go func(startIdx int) {
			defer wg.Done()
			defer sem.Release(1)

			// Create a thread-local cache to reduce lookups to the shared map
			localCache := make(map[string]int)

			// Determine end of current batch
			endIdx := startIdx + batchSize
			if endIdx > len(results) {
				endIdx = len(results)
			}

			currentBatch := results[startIdx:endIdx]

			// Start a transaction for this batch
			batchCtx, batchCancel := context.WithTimeout(ctx, 30*time.Second)
			defer batchCancel()

			tx, err := conn.DB.Begin(batchCtx)
			if err != nil {
				errorCh <- fmt.Errorf("error beginning transaction for batch %d-%d: %w",
					startIdx, endIdx, err)
				return
			}

			// Ensure transaction is handled properly
			//committed := false
			/*
							//defer func() {
								if !committed {
									if rbErr := tx.Rollback(context.Background()); rbErr != nil {
										////fmt.Printf("Error rolling back transaction: %v\n", rbErr)
				                        //return rbErr
									}
								}
							//}()
			*/

			// Collect all security IDs first
			recordsToProcess := make([]struct {
				record     models.Agg
				securityID int
			}, 0, len(currentBatch))

			// First pass - collect all the security IDs needed
			for _, record := range currentBatch {
				ticker := record.Ticker
				cacheKey := securityCacheKey(ticker, date)

				// Check local cache first (no sync needed)
				securityID, exists := localCache[cacheKey]

				// If not in local cache, check the shared thread-safe cache
				if !exists {
					if cachedID, found := securityCache.Load(cacheKey); found {
						securityID = cachedID.(int)
						// Update local cache
						localCache[cacheKey] = securityID
						exists = true
					}
				}

				// If not found in any cache, look it up
				if !exists {
					var err error
					securityID, err = postgres.GetSecurityID(conn, ticker, date)
					if err != nil {
						// Skip this record if security ID can't be found
						continue
					}

					// Store in local cache
					localCache[cacheKey] = securityID

					// Store in shared cache - sync.Map is thread-safe
					securityCache.Store(cacheKey, securityID)
				}

				// Add to the records we'll process
				recordsToProcess = append(recordsToProcess, struct {
					record     models.Agg
					securityID int
				}{record: record, securityID: securityID})
			}

			// Skip if no valid records to process
			if len(recordsToProcess) == 0 {
				return
			}

			// Build the batch insert
			valueStrings := make([]string, 0, len(recordsToProcess))
			valueArgs := make([]interface{}, 0, len(recordsToProcess)*7) // Changed from 10 to 7 for reduced columns
			argPosition := 1

			for _, item := range recordsToProcess {
				record := item.record
				securityID := item.securityID

				valueStrings = append(valueStrings,
					fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
						argPosition, argPosition+1, argPosition+2, argPosition+3,
						argPosition+4, argPosition+5, argPosition+6))

				valueArgs = append(valueArgs,
					record.Timestamp,
					securityID, record.Open, record.High,
					record.Low, record.Close, record.Volume)

				argPosition += 7 // Changed from 10 to 7 for reduced columns
			}

			// Execute batch insert
			query := fmt.Sprintf(
				"INSERT INTO ohlcv_1d (timestamp, securityid, open, high, low, close, volume) VALUES %s",
				valueStrings[0])

			for i := 1; i < len(valueStrings); i++ {
				query += ", " + valueStrings[i]
			}

			// Add ON CONFLICT clause to handle duplicates
			query += " ON CONFLICT (timestamp, securityid) DO NOTHING"

			_, err = tx.Exec(batchCtx, query, valueArgs...)
			if err != nil {
				errorCh <- fmt.Errorf("error executing batch insert for records %d-%d: %w",
					startIdx, endIdx, err)
				return
			}

			// Commit the transaction
			if err = tx.Commit(batchCtx); err != nil {
				errorCh <- fmt.Errorf("error committing transaction for batch %d-%d: %w",
					startIdx, endIdx, err)
				return
			}
			//committed = true

		}(i)
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

// Preload securities for a specific date in bulk to reduce database queries
func batchPreloadTickers(conn *data.Conn, tickers map[string]bool, date time.Time, securityCache *sync.Map) {
	// Convert ticker map to slice for query
	tickerList := make([]string, 0, len(tickers))
	for ticker := range tickers {
		tickerList = append(tickerList, ticker)
	}

	// Skip if no tickers to preload
	if len(tickerList) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Build the date condition for the query - we need securities as they were on this date
	dateStr := date.Format("2006-01-02")

	// Adjust query based on your database schema
	query := `
		SELECT ticker, securityid 
		FROM securities 
		WHERE ticker = ANY($1)
		AND (minDate <= $2)
		AND (maxDate >= $2 OR maxDate IS NULL)
	`

	rows, err := conn.DB.Query(ctx, query, tickerList, dateStr)
	if err != nil {
		////fmt.Printf("Warning: Failed to batch preload securities: %v\n", err)
		return
	}
	defer rows.Close()

	// Store results in the thread-safe sync.Map
	for rows.Next() {
		var ticker string
		var id int
		if err := rows.Scan(&ticker, &id); err != nil {
			////fmt.Printf("Warning: Error scanning security: %v\n", err)
			continue
		}

		// Store with time-aware key in the thread-safe map
		cacheKey := securityCacheKey(ticker, date)
		securityCache.Store(cacheKey, id)
	}
}
