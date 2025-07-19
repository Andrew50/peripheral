/*
GOAL: 10k ticker refreshed in 10s

bucket / metric	rows per bucket	how often queried	CA?	Rationale
Pre‑market daily stats (pre_market_stats)	330 rows / ticker	Every screen refresh	Yes — keep CA	One daily bucket, refresh touches ≤ 10 k new rows/minute, huge scan avoided
Intraday 1 m / 15 m / 1 h deltas & ranges	≤ 16 min of raw data	Only needed for current screen	No — keep intraday_stats table	Bucket would be 1 minute → every insert invalidates it, refresh ≈ query cost
Historical daily references (1 w–10 y, 52 w high/low)	1 row / day	Built once per day	No (plain table is fine)	Refresh cost tiny, CA would duplicate storage
Final screener view (one row per ticker)	1 row	Always queried	No	You already up‑sert; CA adds no value


OPTIMIZATION TO TRY:

-- parallelize the screener update with golang workers



OPTIMIZATION LOG: -- baseline 3s for 1 ticker

*/

package screener

import (
	"backend/internal/data"
	"backend/internal/services/marketdata" // Add this import
	"context"
	"fmt" // Added fmt import
	"log"
	"os"
	"time"
)

const (
	refreshInterval         = 60 * time.Second  // full screener top-off frequency (fallback)
	refreshTimeout          = 300 * time.Second // per-refresh SQL timeout (increased from 60s)
	extendedCloseHour       = 20                // 8 PM Eastern – hard stop
	maxTickersPerBatch      = 100               // max tickers to process per batch (0 = no limit), increased from 1 for better efficiency
	staticRefs1mInterval    = 1 * time.Minute   // refresh static_refs_1m every minute
	staticRefsDailyInterval = 5 * time.Minute   // refresh static_refs every 5 minutes
	IgnoreMarketHours       = true              // ignore market hours
)

// initialRefresh performs a one-time refresh of all aggregates and static data at startup
func initialRefresh(conn *data.Conn) error {
	ctx := context.Background() // No timeout - let index updaters run as long as needed

	log.Println("🚀 Starting initial data refresh...")
	start := time.Now()

	refreshCommands := []string{
		"CALL refresh_continuous_aggregate('cagg_1440_minute', now() - INTERVAL '1440 minutes', NULL);",
		"CALL refresh_continuous_aggregate('cagg_15_minute', now() - INTERVAL '15 minutes', NULL);",
		"CALL refresh_continuous_aggregate('cagg_60_minute', now() - INTERVAL '60 minutes', NULL);",
		"CALL refresh_continuous_aggregate('cagg_4_hour', now() - INTERVAL '4 hours', NULL);",
		"CALL refresh_continuous_aggregate('cagg_7_day', now() - INTERVAL '7 days', NULL);",
		"CALL refresh_continuous_aggregate('cagg_30_day', now() - INTERVAL '30 days', NULL);",
		"CALL refresh_continuous_aggregate('cagg_50_day', now() - INTERVAL '50 days', NULL);",
		"CALL refresh_continuous_aggregate('cagg_200_day', now() - INTERVAL '200 days', NULL);",
		"CALL refresh_continuous_aggregate('cagg_14_day', now() - INTERVAL '14 days', NULL);",
		"CALL refresh_continuous_aggregate('cagg_14_minute', now() - INTERVAL '14 minutes', NULL);",
		//"CALL refresh_continuous_aggregate('cagg_rsi_14_day', now() - INTERVAL '3 weeks', NULL);", //3 weeks cause you need 14 bars to calculate rsi
		"SELECT refresh_static_refs();",
		"SELECT refresh_static_refs_1m();",
	}

	for _, cmd := range refreshCommands {
		log.Printf("Executing: %s", cmd)
		if _, err := conn.DB.Exec(ctx, cmd); err != nil {
			// Log error but continue, some might fail if already running or not needed
			log.Printf("⚠️  Initial refresh command failed for '%s': %v", cmd, err)
		}
	}

	// Create OHLCV indexes
	indexSQLs := marketdata.IndexSQLs()
	for _, idxCmd := range indexSQLs {
		log.Printf("Executing index creation: %s", idxCmd)
		if _, err := conn.DB.Exec(ctx, idxCmd); err != nil {
			log.Printf("⚠️  Index creation failed: %v", err)
		}
	}

	log.Printf("✅ Initial data refresh completed in %v", time.Since(start))
	return nil
}

func StartScreenerUpdaterLoop(conn *data.Conn) error {

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("❌ cannot load ET timezone: %v", err)
	}

	// Optimize database connection settings for better performance
	if err := optimizeDatabaseConnection(conn); err != nil {
		log.Printf("⚠️  Failed to optimize database connection: %v", err)
	} // Add tickers with null maxdate to screener_stale table
	log.Println("🔄 Adding tickers with null maxdate to screener_stale table...")
	insertStaleQuery := `
		INSERT INTO screener_stale (ticker, last_update_time, stale)
		SELECT DISTINCT ticker, '1970-01-01 00:00:00'::timestamp, true
		FROM securities 
		WHERE ticker NOT IN (
			SELECT DISTINCT ticker 
			FROM ohlcv_1d 
			WHERE ticker IS NOT NULL
		)
		ON CONFLICT (ticker) DO UPDATE SET
			last_update_time = EXCLUDED.last_update_time,
			stale = EXCLUDED.stale;
	`

	// Perform initial data refresh on startup
	if err := initialRefresh(conn); err != nil {
		log.Printf("⚠️  Initial data refresh failed: %v. Continuing...", err)
	} //might want to re-enable this before pr
	screenerRefreshCmd := fmt.Sprintf("SELECT refresh_screener(%d);", maxTickersPerBatch)
	log.Printf("Executing initial screener refresh: %s", screenerRefreshCmd)
	_, err = conn.DB.Exec(context.Background(), screenerRefreshCmd)
	if err != nil {
		log.Printf("⚠️  Initial screener refresh failed: %v. Continuing...", err)
	}

	_, err = conn.DB.Exec(context.Background(), insertStaleQuery)
	if err != nil {
		log.Printf("⚠️  Failed to add stale tickers: %v. Continuing...", err)
	} else {
		log.Println("✅ Successfully added tickers with null maxdate to screener_stale table")
	}

	// Create tickers for different refresh intervals
	screenerTicker := time.NewTicker(refreshInterval)
	staticRefs1mTicker := time.NewTicker(staticRefs1mInterval)
	staticRefsDailyTicker := time.NewTicker(staticRefsDailyInterval)

	defer screenerTicker.Stop()
	defer staticRefs1mTicker.Stop()
	defer staticRefsDailyTicker.Stop()

	// Add counters for monitoring
	var updateCount int
	var totalDuration time.Duration

	log.Printf("🚀 Screener updater started with static refs refresh")
	log.Printf("📅 Static refs 1m: every %v during market hours (4am-8pm ET, weekdays)", staticRefs1mInterval)
	log.Printf("📅 Static refs daily: every %v during regular market hours (9:30am-4pm ET, weekdays)", staticRefsDailyInterval)

	for {
		if !IgnoreMarketHours {
			now := time.Now().In(loc)
			if now.Hour() >= extendedCloseHour {
				log.Println("🌙 Post‑market closed — stopping screener updater")
				return nil
			}
		}

		select {
		case <-screenerTicker.C:
			updateStart := time.Now()
			updateStaleScreenerValues(conn)
			updateDuration := time.Since(updateStart)

			updateCount++
			totalDuration += updateDuration

			if updateCount%10 == 0 {
				avgDuration := totalDuration / time.Duration(updateCount)
				log.Printf("📊 Screener update stats: %d updates, avg duration: %v", updateCount, avgDuration)
			}

		case <-staticRefs1mTicker.C:
			// Refresh static_refs_1m every minute during market hours (4am-8pm ET, weekdays)
			if isMarketHours(time.Now(), loc) {
				go refreshStaticRefs1m(conn)
			}

		case <-staticRefsDailyTicker.C:
			// Refresh static_refs every 5 minutes during regular market hours (9:30am-4pm ET, weekdays)
			if isRegularMarketHours(time.Now(), loc) {
				go refreshStaticRefsDaily(conn)
			}
		}
	}
}

// optimizeDatabaseConnection applies performance optimizations to the database connection
func optimizeDatabaseConnection(conn *data.Conn) error {
	ctx := context.Background() // No timeout for database optimization settings

	// Apply connection-level optimizations
	optimizations := []string{
		"SET statement_timeout = '0'", // No statement timeout - let index updaters run as long as needed
		/* effective_cache_size = '4GB'",   // Adjust based on your system
		"SET random_page_cost = 1.1",         // Optimize for SSD storage
		"SET seq_page_cost = 1.0",            // Optimize for SSD storage
		"SET cpu_tuple_cost = 0.01",          // Optimize for modern CPUs
		"SET cpu_index_tuple_cost = 0.005",   // Optimize for modern CPUs
		"SET cpu_operator_cost = 0.0025",     // Optimize for modern CPUs
		"SET effective_io_concurrency = 200", // Optimize for SSD
		"SET synchronous_commit = off",       // Improve write performance (careful with this)
		"SET checkpoint_completion_target = 0.9",
		"SET wal_buffers = '16MB'",
		"SET shared_preload_libraries = 'pg_stat_statements'",*/
	}

	successCount := 0
	for _, opt := range optimizations {
		if _, err := conn.DB.Exec(ctx, opt); err != nil {
			// Some settings might not be changeable at runtime - that's OK
			continue
		}
		successCount++
	}

	// Add this after the loop
	var currentWorkMem string
	err := conn.DB.QueryRow(ctx, "SHOW work_mem;").Scan(&currentWorkMem)
	if err != nil {
		log.Printf("Failed to check work_mem: %v", err)
	} else {
		log.Printf("Effective work_mem in session: %s", currentWorkMem)
	}

	log.Printf("✅ Applied %d/%d database optimizations", successCount, len(optimizations))
	return nil
}

// SQL queries for reuse (avoid re-parsing)
var (
	refreshScreenerQuery     = `SELECT refresh_screener($1);`
	refreshStaticRefsQuery   = `SELECT refresh_static_refs();`
	refreshStaticRefs1mQuery = `SELECT refresh_static_refs_1m();`
)

func updateStaleScreenerValues(conn *data.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()

	// Log current working directory for debugging
	if cwd, err := os.Getwd(); err == nil {
		log.Printf("📊 Current working directory: %s", cwd)
	}

	log.Printf("🔄 Updating screener values (timeout: %v)...", refreshTimeout)
	start := time.Now()

	// Execute the main query
	_, err := conn.DB.Exec(ctx, refreshScreenerQuery, maxTickersPerBatch)

	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ updateStaleScreenerValues: failed to refresh screener data: %v", err)
		log.Printf("🔄 updateStaleScreenerValues: %v (failed)", duration)
		return
	}

	log.Printf("✅ Screener refresh completed successfully in %v", duration)

	// Only run detailed analysis if the operation took too long
	go func() {
		if err := RunPerformanceAnalysis(conn, screenerAnalysisConfig); err != nil {
			log.Printf("⚠️  Background performance analysis failed: %v", err)
		}
	}()

	log.Printf("🔄 updateStaleScreenerValues: %v", duration)
}

// refreshStaticRefs1m refreshes the static_refs_1m table
func refreshStaticRefs1m(conn *data.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()

	log.Printf("🔄 Refreshing static_refs_1m...")
	start := time.Now()

	_, err := conn.DB.Exec(ctx, refreshStaticRefs1mQuery)

	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ refreshStaticRefs1m: failed to refresh static_refs_1m: %v", err)
		return
	}

	log.Printf("✅ static_refs_1m refresh completed in %v", duration)
}

// refreshStaticRefsDaily refreshes the static_refs_daily table
func refreshStaticRefsDaily(conn *data.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()

	log.Printf("🔄 Refreshing static_refs_daily...")
	start := time.Now()

	_, err := conn.DB.Exec(ctx, refreshStaticRefsQuery)

	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ refreshStaticRefsDaily: failed to refresh static_refs: %v", err)
		return
	}

	log.Printf("✅ static_refs_daily refresh completed in %v", duration)
}

// isMarketHours checks if current time is within market hours (4am-8pm ET on weekdays)
func isMarketHours(now time.Time, loc *time.Location) bool {
	nowET := now.In(loc)
	weekday := nowET.Weekday()

	// Only weekdays (Monday-Friday)
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	hour := nowET.Hour()
	// 4am to 8pm ET
	return hour >= 4 && hour < 20
}

// isRegularMarketHours checks if current time is within regular market hours (9:30am-4pm ET on weekdays)
func isRegularMarketHours(now time.Time, loc *time.Location) bool {
	nowET := now.In(loc)
	weekday := nowET.Weekday()

	// Only weekdays (Monday-Friday)
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	hour := nowET.Hour()
	minute := nowET.Minute()

	// 9:30am to 4:00pm ET
	if hour < 9 || hour > 16 {
		return false
	}
	if hour == 9 && minute < 30 {
		return false
	}
	if hour == 16 && minute > 0 {
		return false
	}

	return true
}

var screenerAnalysisConfig = AnalysisConfig{
	LogFilePath:      "/app/screener_analysis.log",
	StaleQuery:       `SELECT ticker, last_update_time, stale FROM screener_stale WHERE stale = TRUE ORDER BY last_update_time ASC LIMIT $1`,
	StaleQueryParams: []interface{}{maxTickersPerBatch},
	Tables:           []string{"ohlcv_1m", "ohlcv_1d", "screener", "screener_stale", "securities", "static_refs_daily", "static_refs_1m"},
	QueryPatterns:    []string{"screener", "ohlcv", "refresh_screener", "refresh_static_refs", "refresh_static_refs_1m"},
	TestFunctions: []TestQuery{
		{Name: "refresh_screener", Query: "SELECT refresh_screener(10)"},
	},
	ComponentTests: []TestQuery{
		{Name: "OHLCV 1m latest", Query: "SELECT ticker, close/1000.0 FROM ohlcv_1m WHERE ticker = ANY($1) ORDER BY timestamp DESC LIMIT 10"},
		{Name: "OHLCV 1d latest", Query: "SELECT ticker, close/1000.0 FROM ohlcv_1d WHERE ticker = ANY($1) ORDER BY timestamp DESC LIMIT 10"},
		{Name: "Securities lookup", Query: "SELECT ticker, market_cap, sector FROM securities WHERE ticker = ANY($1)"},
		{Name: "Static refs daily", Query: "SELECT ticker, price_1d, price_1w FROM static_refs_daily WHERE ticker = ANY($1)"},
		{Name: "Static refs 1m", Query: "SELECT ticker, price_1m, price_15m FROM static_refs_1m WHERE ticker = ANY($1)"},
	},
}
