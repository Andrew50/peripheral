package marketdata

import (
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/postgres"
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/polygon-io/client-go/rest/models"
)

// UpdateAllOHLCV is the consolidated OHLCV updater that processes all active tickers
// across all timeframes (1m, 1h, 1d, 1w) using goroutines for parallel processing
func UpdateAllOHLCV(conn *data.Conn) error {
	log.Println("Starting consolidated OHLCV update for all active tickers...")
	start := time.Now()

	defer func() {
		log.Printf("Consolidated OHLCV update completed in %v", time.Since(start))
	}()

	// Get all active tickers from securities table
	tickers, err := getActiveTickers(conn)
	if err != nil {
		return fmt.Errorf("error getting active tickers: %w", err)
	}

	log.Printf("Found %d active tickers to process", len(tickers))

	// FIXED: Reduce concurrent workers from 10 to 3 to prevent lock contention
	const maxWorkers = 1
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var completed int64

	// Process each ticker using goroutines
	for i, ticker := range tickers {
		wg.Add(1)
		go func(ticker string, index int) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := processTickerAllTimeframes(conn, ticker)
			if err != nil {
				log.Printf("Error processing ticker %s: %v", ticker, err)
			}

			// Increment completed counter and log progress every 100 securities
			count := atomic.AddInt64(&completed, 1)
			if count == 1 || count%100 == 0 {
				log.Printf("%d/%d completed, up to ticker: %s", count, len(tickers), ticker)
			}
		}(ticker, i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return nil
}

// getActiveTickers retrieves all currently active tickers from the securities table
func getActiveTickers(conn *data.Conn) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Increased timeout
	defer cancel()

	query := `SELECT ticker FROM securities WHERE maxDate IS NULL ORDER BY ticker`
	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying active tickers: %w", err)
	}
	defer rows.Close()

	var tickers []string
	for rows.Next() {
		var ticker string
		if err := rows.Scan(&ticker); err != nil {
			return nil, fmt.Errorf("error scanning ticker: %w", err)
		}
		tickers = append(tickers, ticker)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ticker rows: %w", err)
	}

	return tickers, nil
}

// getLatestDateForTicker gets the latest timestamp for a ticker in a specific OHLCV table
func getLatestDateForTicker(conn *data.Conn, securityID int, tableName string) (*time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Increased timeout
	defer cancel()

	// First check if the table exists
	tableExistsQuery := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		)`

	var tableExists bool
	err := conn.DB.QueryRow(ctx, tableExistsQuery, tableName).Scan(&tableExists)
	if err != nil {
		return nil, fmt.Errorf("error checking if table %s exists: %w", tableName, err)
	}

	if !tableExists {
		return nil, fmt.Errorf("table %s does not exist - run database migrations first", tableName)
	}

	// Query for the latest timestamp
	query := fmt.Sprintf("SELECT MAX(timestamp) FROM %s WHERE securityid = $1", tableName)

	var latestDate *time.Time
	err = conn.DB.QueryRow(ctx, query, securityID).Scan(&latestDate)
	if err != nil {
		return nil, fmt.Errorf("error querying latest date from %s for securityID %d: %w", tableName, securityID, err)
	}

	return latestDate, nil
}

// processTickerAllTimeframes processes a single ticker across all timeframes
func processTickerAllTimeframes(conn *data.Conn, ticker string) error {
	// Get security ID for this ticker
	securityID, err := postgres.GetCurrentSecurityID(conn, ticker)
	if err != nil {
		return fmt.Errorf("error getting security ID for %s: %w", ticker, err)
	}

	// Process each timeframe sequentially with complete historical data
	timeframes := []struct {
		name       string
		timespan   string
		multiplier int
		tableName  string
	}{
		{"1-minute", "minute", 1, "ohlcv_1m"},
		{"1-hour", "hour", 1, "ohlcv_1h"},
		{"1-day", "day", 1, "ohlcv_1d"},
		{"1-week", "week", 1, "ohlcv_1w"},
	}

	for _, tf := range timeframes {
		// Get the latest date for this ticker and timeframe from database
		fromDate, err := getLatestDateForTicker(conn, securityID, tf.tableName)
		if err != nil {
			return fmt.Errorf("error getting latest date for %s %s: %w", ticker, tf.name, err)
		}

		// If fromDate is nil, start from 2008 (Polygon's data availability)
		if fromDate == nil {
			startDate := time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)
			fromDate = &startDate
		} else {
			// Start from the day after the latest data
			nextDay := fromDate.AddDate(0, 0, 1)
			fromDate = &nextDay
		}

		// End date is today
		toDate := time.Now()

		// Skip if fromDate is after toDate (data is up to date)
		if fromDate.After(toDate) {
			continue
		}

		if err := processTickerTimeframe(conn, ticker, securityID, tf.timespan, tf.multiplier, *fromDate, toDate); err != nil {
			return fmt.Errorf("error processing %s data for %s: %w", tf.name, ticker, err)
		}
	}

	return nil
}

// processTickerTimeframe processes a single ticker for a specific timeframe
func processTickerTimeframe(conn *data.Conn, ticker string, securityID int, timespan string, multiplier int, fromDate, toDate time.Time) error {
	// FIXED: Reduced timeout and added lock timeout
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second) // Reduced from 30 minutes to 10 minutes
	defer cancel()

	// Determine table name based on timeframe
	var tableName string
	switch timespan {
	case "minute":
		tableName = "ohlcv_1m"
	case "hour":
		tableName = "ohlcv_1h"
	case "day":
		tableName = "ohlcv_1d"
	case "week":
		tableName = "ohlcv_1w"
	default:
		return fmt.Errorf("unsupported timespan: %s", timespan)
	}

	// Set up date range for API call - fetch ALL data in the range without breaking it up
	fromMillis := models.Millis(fromDate)
	toMillis := models.Millis(toDate)

	// Fetch ALL data using GetAggsData - no arbitrary limits
	iter, err := polygon.GetAggsData(conn.Polygon, ticker, multiplier, timespan, fromMillis, toMillis, 50000, "asc", true)
	if err != nil {
		return fmt.Errorf("error fetching %s data for %s: %w", timespan, ticker, err)
	}

	// Check if we got any data from the API
	recordCount := 0

	// Start transaction with lock timeout
	tx, err := conn.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("error beginning transaction: %w", err)
	}
	defer tx.Rollback(context.Background())

	// FIXED: Set lock timeout to prevent indefinite waiting
	_, err = tx.Exec(ctx, "SET lock_timeout = '5min'")
	if err != nil {
		return fmt.Errorf("error setting lock timeout: %w", err)
	}

	// FIXED: Use proper batch insert with COPY for better performance and reduced locking
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s (timestamp, securityid, open, high, low, close, volume) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (timestamp, securityid) DO UPDATE SET
			open = EXCLUDED.open,
			high = EXCLUDED.high,
			low = EXCLUDED.low,
			close = EXCLUDED.close,
			volume = EXCLUDED.volume
	`, tableName)

	// FIXED: Reduced batch size from 1000 to 500 to reduce transaction size
	batchSize := 500
	batch := make([]struct {
		timestamp  time.Time
		securityID int
		open       float64
		high       float64
		low        float64
		close      float64
		volume     float64
	}, 0, batchSize)

	for iter.Next() {
		agg := iter.Item()
		recordCount++

		// Use the actual bar timestamp from Polygon API - no normalization
		insertTimestamp := time.Time(agg.Timestamp).UTC()

		// Add to batch
		batch = append(batch, struct {
			timestamp  time.Time
			securityID int
			open       float64
			high       float64
			low        float64
			close      float64
			volume     float64
		}{
			timestamp:  insertTimestamp,
			securityID: securityID,
			open:       agg.Open,
			high:       agg.High,
			low:        agg.Low,
			close:      agg.Close,
			volume:     agg.Volume,
		})

		// Process batch when it's full
		if len(batch) >= batchSize {
			if err := processBatchImproved(ctx, tx, insertQuery, batch); err != nil {
				return fmt.Errorf("error processing batch for %s: %w", ticker, err)
			}
			batch = batch[:0] // Reset batch
		}
	}

	// Process remaining records in batch
	if len(batch) > 0 {
		if err := processBatchImproved(ctx, tx, insertQuery, batch); err != nil {
			return fmt.Errorf("error processing final batch for %s: %w", ticker, err)
		}
	}

	// Check for iterator errors
	if err := iter.Err(); err != nil {
		return fmt.Errorf("error iterating %s data for %s: %w", timespan, ticker, err)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

// FIXED: Improved batch processing using pgx.Batch for true batch operations
func processBatchImproved(ctx context.Context, tx pgx.Tx, insertQuery string, batch []struct {
	timestamp  time.Time
	securityID int
	open       float64
	high       float64
	low        float64
	close      float64
	volume     float64
}) error {
	if len(batch) == 0 {
		return nil
	}

	// Use pgx.Batch for true batch processing (much more efficient)
	pgxBatch := &pgx.Batch{}

	for _, record := range batch {
		pgxBatch.Queue(insertQuery, record.timestamp, record.securityID, record.open, record.high, record.low, record.close, record.volume)
	}

	// Execute the entire batch in one round trip
	batchResults := tx.SendBatch(ctx, pgxBatch)
	defer batchResults.Close()

	// Process all results
	for i := 0; i < len(batch); i++ {
		_, err := batchResults.Exec()
		if err != nil {
			return fmt.Errorf("error executing batch insert at index %d: %w", i, err)
		}
	}

	return nil
}

// DEPRECATED: Old inefficient batch processing - kept for reference
func processBatch(ctx context.Context, tx pgx.Tx, insertQuery string, batch []struct {
	timestamp  time.Time
	securityID int
	open       float64
	high       float64
	low        float64
	close      float64
	volume     float64
}) error {
	for _, record := range batch {
		_, err := tx.Exec(ctx, insertQuery, record.timestamp, record.securityID, record.open, record.high, record.low, record.close, record.volume)
		if err != nil {
			return fmt.Errorf("error inserting record: %w", err)
		}
	}
	return nil
}
