/*
GOAL: 10k ticker refreshed in 10s

MIGRATION 78 OPTIMIZATION:
- Consolidated 9 continuous aggregates into 2 static reference tables
- Only cagg_pre_market and cagg_extended_hours remain as continuous aggregates
- All other metrics now calculated on-demand in refresh_static_refs() and refresh_static_refs_1m()
- This reduces complexity from 11 caggs to 2, with better batch performance

bucket / metric	rows per bucket	how often queried	CA?	Rationale
Pre‑market daily stats (pre_market_stats)	330 rows / ticker	Every screen refresh	Yes — keep CA	One daily bucket, refresh touches ≤ 10k new rows/minute, huge scan avoided
Extended‑hours daily stats	330 rows / ticker	Every screen refresh	Yes — keep CA	Time-sensitive session data
Historical daily references + moving averages + volatility	1 row / ticker	Built periodically	No (static_refs_daily)	Batch calculation more efficient than continuous updates
Intraday ranges + volume averages	1 row / ticker	Built frequently	No (static_refs_1m)	Batch calculation more efficient than continuous updates
Final screener view (one row per ticker)	1 row	Always queried	No	You already up‑sert; CA adds no value

OPTIMIZATION TO TRY:
-- parallelize the screener update with golang workers

OPTIMIZATION LOG: -- baseline 3s for 1 ticker
*/

package screener

import (
	"backend/internal/data"
	"context" // Added fmt import
	"fmt"
	"log"
	"sync"
	"time"
)

// ScreenerUpdaterService encapsulates the screener updater and its state
type ScreenerUpdaterService struct {
	conn      *data.Conn
	isRunning bool
	stopChan  chan struct{}
	mutex     sync.RWMutex
	wg        sync.WaitGroup
	loc       *time.Location
}

// Global instance of the service
var screenerService *ScreenerUpdaterService
var serviceInitMutex sync.Mutex

// GetScreenerService returns the singleton instance of ScreenerUpdaterService
func GetScreenerService() *ScreenerUpdaterService {
	serviceInitMutex.Lock()
	defer serviceInitMutex.Unlock()

	if screenerService == nil {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			log.Fatalf("❌ cannot load ET timezone: %v", err)
		}

		screenerService = &ScreenerUpdaterService{
			stopChan: make(chan struct{}),
			loc:      loc,
		}
	}
	return screenerService
}

// Start initializes and starts the screener updater service (idempotent)
func (s *ScreenerUpdaterService) Start(conn *data.Conn) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isRunning {
		log.Printf("⚠️ Screener updater already running")
		return nil
	}

	log.Printf("🚀 Starting Screener updater service")
	s.conn = conn

	// Add logging before optimizing the database connection
	log.Printf("🔄 Calling optimizeDatabaseConnection")
	// Optimize database connection settings for better performance
	if err := optimizeDatabaseConnection(conn); err != nil {
		log.Printf("⚠️  Failed to optimize database connection: %v", err)
	}

	// Add tickers with null maxdate to screener_stale table
	insertStaleQuery := `
		INSERT INTO screener_stale (ticker, last_update_time, stale)
		SELECT DISTINCT ticker, '1970-01-01 00:00:00'::timestamp, true
		FROM securities
		WHERE active = TRUE
		ON CONFLICT (ticker) DO UPDATE SET
			last_update_time = EXCLUDED.last_update_time,
			stale = EXCLUDED.stale;
	`

	// Add logging before performing initial data refresh
	log.Printf("🔄 Calling initialRefresh")
	// Perform initial data refresh on startup
	if err := initialRefresh(conn); err != nil {
		log.Printf("⚠️  Initial data refresh failed: %v. Continuing...", err)
	}

	_, err := conn.DB.Exec(context.Background(), insertStaleQuery)
	if err != nil {
		log.Printf("⚠️  Failed to add stale tickers: %v. Continuing...", err)
	} else {
		log.Println("✅ Successfully added tickers with null maxdate to screener_stale table")
	}

	screenerRefreshCmd := fmt.Sprintf("SELECT refresh_screener(%d);", maxTickersPerBatch)
	log.Printf("Executing initial screener refresh: %s", screenerRefreshCmd)
	_, err = conn.DB.Exec(context.Background(), screenerRefreshCmd)
	if err != nil {
		return err
	}

	// Add logging before starting the updater loop
	log.Printf("🔄 Starting updater loop")
	// Create new stop channel for this session
	s.stopChan = make(chan struct{})
	s.isRunning = true

	// Start the updater loop goroutine
	s.wg.Add(1)
	go s.runUpdaterLoop()

	log.Printf("✅ Screener updater service started")
	return nil
}

// Stop gracefully shuts down the screener updater service (idempotent)
func (s *ScreenerUpdaterService) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		log.Printf("⚠️ Screener updater is not running")
		return nil
	}

	log.Printf("🛑 Stopping Screener updater service")

	// Signal the updater goroutine to stop
	close(s.stopChan)

	s.isRunning = false

	// Wait for the updater goroutine to finish
	s.wg.Wait()

	log.Printf("✅ Screener updater service stopped")
	return nil
}

// IsRunning returns whether the service is currently running
func (s *ScreenerUpdaterService) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.isRunning
}

// runUpdaterLoop is the main updater loop that runs in a goroutine
func (s *ScreenerUpdaterService) runUpdaterLoop() {
	defer s.wg.Done()

	// Create tickers for different refresh intervals
	screenerTicker := time.NewTicker(refreshInterval)
	staticRefs1mTicker := time.NewTicker(staticRefs1mInterval)
	staticRefsDailyTicker := time.NewTicker(staticRefsDailyInterval)
	latestBarViewsTicker := time.NewTicker(latestBarViewsInterval)

	defer screenerTicker.Stop()
	defer staticRefs1mTicker.Stop()
	defer staticRefsDailyTicker.Stop()
	defer latestBarViewsTicker.Stop()

	// Add counters for monitoring
	var updateCount int
	var totalDuration time.Duration

	log.Printf("🚀 Screener updater loop started with optimized static refs refresh (Migration 78)")
	log.Printf("📊 Consolidated from 11 continuous aggregates to 2 (pre-market + extended-hours only)")
	log.Printf("📅 Static refs 1m: every %v during market hours (4am-8pm ET, weekdays) - now includes ranges & volume", staticRefs1mInterval)
	log.Printf("📅 Static refs daily: every %v during regular market hours (9:30am-4pm ET, weekdays) - now includes moving averages & volatility", staticRefsDailyInterval)

	for {
		select {
		case <-s.stopChan:
			log.Printf("📡 Screener updater loop stopped by stop signal")
			return

		case <-screenerTicker.C:
			updateStart := time.Now()
			updateStaleScreenerValues(s.conn)
			updateDuration := time.Since(updateStart)

			updateCount++
			totalDuration += updateDuration

			if updateCount%10 == 0 {
				avgDuration := totalDuration / time.Duration(updateCount)
				log.Printf("📊 Screener update stats: %d updates, avg duration: %v", updateCount, avgDuration)
			}

		case <-staticRefs1mTicker.C:
			// Refresh static_refs_1m every minute during market hours (4am-8pm ET, weekdays)
			if isMarketHours(time.Now(), s.loc) {
				go refreshStaticRefs1m(s.conn)
			}

		case <-staticRefsDailyTicker.C:
			// Refresh static_refs every 5 minutes during regular market hours (9:30am-4pm ET, weekdays)
			if isRegularMarketHours(time.Now(), s.loc) {
				go refreshStaticRefsDaily(s.conn)
			}

		case <-latestBarViewsTicker.C:
			// Refresh latest bar materialized views every 30 seconds (CRITICAL for screener performance)
			if isMarketHours(time.Now(), s.loc) {
				go refreshLatestBarViews(s.conn)
			}
		}
	}
}

const (
	refreshInterval         = 60 * time.Second   // full screener top-off frequency (fallback)
	refreshTimeout          = 600 * time.Second  // per-refresh SQL timeout (increased from 60s)
	staticRefsTimeout       = 1200 * time.Second // timeout for static refs functions (increased due to more computation)
	maxTickersPerBatch      = 50000              // max tickers to process per batch (0 = no limit), increased from 1 for better efficiency
	staticRefs1mInterval    = 1 * time.Minute    // refresh static_refs_1m every minute (was 5 minutes)
	staticRefsDailyInterval = 5 * time.Minute    // refresh static_refs every 5 minutes (was 20 minutes)
	latestBarViewsInterval  = 30 * time.Second   // refresh latest bar materialized views every 30 seconds (CRITICAL)
	useAnalysis             = false              // enable performance analysis
)

var (
	dailyStaticRefsMu   sync.Mutex // guards refresh_static_refs()
	oneMinStaticRefsMu  sync.Mutex // guards refresh_static_refs_1m()
	preMarketCaggMu     sync.Mutex // guards cagg_pre_market refresh
	extendedHoursCaggMu sync.Mutex // guards cagg_extended_hours refresh
)

// Legacy function maintained for backward compatibility
// StartScreenerUpdaterLoop starts the screener updater service
func StartScreenerUpdaterLoop(conn *data.Conn) error {
	log.Printf("🚀 StartScreenerUpdaterLoop called (using service-based approach)")
	service := GetScreenerService()
	return service.Start(conn)
}

// StopScreenerUpdaterLoop stops the screener updater service
func StopScreenerUpdaterLoop() error {
	log.Printf("🛑 StopScreenerUpdaterLoop called (using service-based approach)")
	service := GetScreenerService()
	return service.Stop()
}

// initialRefresh performs a one-time refresh of all aggregates and static data at startup
func initialRefresh(conn *data.Conn) error {
	ctx := context.Background() // No timeout - let index updaters run as long as needed

	log.Println("🚀 Starting initial data refresh...")

	start := time.Now()

	refreshCommands := []string{
		// Latest bar materialized views (CRITICAL for screener performance)
		"SELECT refresh_latest_bar_views();",
		// Clean up dead tuple bloat in static_refs tables before refresh
		"VACUUM (ANALYZE) static_refs_1m;",
		"VACUUM (ANALYZE) static_refs_daily;",
	}

	for _, cmd := range refreshCommands {
		log.Printf("🔄 Executing: %s", cmd)
		if _, err := conn.DB.Exec(ctx, cmd); err != nil {
			// Log error but continue, some might fail if already running or not needed
			log.Printf("⚠️  Initial refresh command failed for '%s': %v", cmd, err)
		}
	}

	// Refresh static reference tables with mutex protection
	//log.Println("🔄 Refreshing static reference tables with mutex protection...")
	refreshStaticRefsDaily(conn)
	refreshStaticRefs1m(conn)

	// Refresh continuous aggregates with mutex protection (pre-market and extended-hours)
	log.Println("🔄 Refreshing continuous aggregates with mutex protection...")
	refreshPreMarketCagg(conn)
	refreshExtendedHoursCagg(conn)

	log.Printf("✅ Initial data refresh completed in %v", time.Since(start))
	return nil
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
	/*if cwd, err := os.Getwd(); err == nil {
		log.Printf("📊 Current working directory: %s", cwd)
	}*/

	//log.Printf("🔄 Updating screener values (timeout: %v)...", refreshTimeout)
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
	if useAnalysis {
		go func() {
			if err := RunPerformanceAnalysis(conn, screenerAnalysisConfig); err != nil {
				log.Printf("⚠️  Background performance analysis failed: %v", err)
			}
		}()
	}

	log.Printf("🔄 updateStaleScreenerValues: %v", duration)
}

// refreshStaticRefs1m refreshes the static_refs_1m table
func refreshStaticRefs1m(conn *data.Conn) {
	if !oneMinStaticRefsMu.TryLock() {
		log.Printf("⏭️ static_refs_1M refresh skipped – already running")
		return
	}
	defer oneMinStaticRefsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), staticRefsTimeout)
	defer cancel()

	//log.Printf("🔄 Refreshing static_refs_1m (now includes range and volume calculations)...")
	start := time.Now()

	// Apply aggressive vacuum settings for high-frequency update table
	vacuumOptimizations := []string{
		"ALTER TABLE static_refs_1m SET (autovacuum_vacuum_threshold = 100, autovacuum_vacuum_scale_factor = 0.02)",
		"ALTER TABLE static_refs_1m SET (autovacuum_analyze_threshold = 50, autovacuum_analyze_scale_factor = 0.01)",
		"ALTER TABLE static_refs_1m SET (autovacuum_vacuum_cost_delay = 2, autovacuum_vacuum_cost_limit = 2000)",
	}

	// Apply vacuum optimizations (ignore errors if already set)
	for _, opt := range vacuumOptimizations {
		if _, err := conn.DB.Exec(ctx, opt); err != nil {
			log.Printf("Warning: vacuum optimization failed: %v", err)
		}
	}

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
	if !dailyStaticRefsMu.TryLock() {
		log.Printf("⏭️ static_refs DAILY refresh skipped – already running")
		return
	}
	defer dailyStaticRefsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), staticRefsTimeout)
	defer cancel()

	//log.Printf("🔄 Refreshing static_refs_daily (now includes moving averages, volatility, and volume calculations)...")
	start := time.Now()

	// Apply aggressive vacuum settings for frequent update table
	vacuumOptimizations := []string{
		"ALTER TABLE static_refs_daily SET (autovacuum_vacuum_threshold = 200, autovacuum_vacuum_scale_factor = 0.03)",
		"ALTER TABLE static_refs_daily SET (autovacuum_analyze_threshold = 100, autovacuum_analyze_scale_factor = 0.02)",
		"ALTER TABLE static_refs_daily SET (autovacuum_vacuum_cost_delay = 2, autovacuum_vacuum_cost_limit = 2000)",
	}

	// Apply vacuum optimizations (ignore errors if already set)
	for _, opt := range vacuumOptimizations {
		if _, err := conn.DB.Exec(ctx, opt); err != nil {
			log.Printf("Warning: vacuum optimization failed: %v", err)
		}
	}

	_, err := conn.DB.Exec(ctx, refreshStaticRefsQuery)

	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ refreshStaticRefsDaily: failed to refresh static_refs: %v", err)
		return
	}

	log.Printf("✅ static_refs_daily refresh completed in %v", duration)
}

// refreshLatestBarViews refreshes the latest bar materialized views (CRITICAL for screener performance)
func refreshLatestBarViews(conn *data.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()

	log.Printf("🔄 Refreshing latest bar materialized views...")
	start := time.Now()

	_, err := conn.DB.Exec(ctx, "SELECT refresh_latest_bar_views()")

	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ refreshLatestBarViews: failed to refresh latest bar views: %v", err)
		return
	}

	log.Printf("✅ Latest bar views refresh completed in %v", duration)
}

// refreshPreMarketCagg refreshes the cagg_pre_market continuous aggregate
func refreshPreMarketCagg(conn *data.Conn) {
	if !preMarketCaggMu.TryLock() {
		log.Printf("⏭️ cagg_pre_market refresh skipped – already running")
		return
	}
	defer preMarketCaggMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), staticRefsTimeout)
	defer cancel()

	log.Printf("🔄 Refreshing cagg_pre_market continuous aggregate...")
	start := time.Now()

	_, err := conn.DB.Exec(ctx, "CALL refresh_continuous_aggregate('cagg_pre_market', now() - INTERVAL '3 days', NULL)")

	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ refreshPreMarketCagg: failed to refresh cagg_pre_market: %v", err)
		return
	}

	log.Printf("✅ cagg_pre_market refresh completed in %v", duration)
}

// refreshExtendedHoursCagg refreshes the cagg_extended_hours continuous aggregate
func refreshExtendedHoursCagg(conn *data.Conn) {
	if !extendedHoursCaggMu.TryLock() {
		log.Printf("⏭️ cagg_extended_hours refresh skipped – already running")
		return
	}
	defer extendedHoursCaggMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), staticRefsTimeout)
	defer cancel()

	log.Printf("🔄 Refreshing cagg_extended_hours continuous aggregate...")
	start := time.Now()

	_, err := conn.DB.Exec(ctx, "CALL refresh_continuous_aggregate('cagg_extended_hours', now() - INTERVAL '3 days', NULL)")

	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ refreshExtendedHoursCagg: failed to refresh cagg_extended_hours: %v", err)
		return
	}

	log.Printf("✅ cagg_extended_hours refresh completed in %v", duration)
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
	QueryPatterns:    []string{"screener", "ohlcv", "refresh_screener", "refresh_static_refs", "refresh_static_refs_1m", "refresh_continuous_aggregate"},
	TestFunctions: []TestQuery{
		// Core screener operations with actual batch size
		{Name: "refresh_screener_actual_batch", Query: fmt.Sprintf("SELECT refresh_screener(%d)", maxTickersPerBatch)},
		{Name: "refresh_screener_single", Query: "SELECT refresh_screener(1)"},
		{Name: "refresh_screener_large_batch", Query: "SELECT refresh_screener(50)"},

		// Static reference refreshes (now do much more work after migration 78)
		{Name: "refresh_static_refs_daily", Query: "SELECT refresh_static_refs()"},
		{Name: "refresh_static_refs_1m", Query: "SELECT refresh_static_refs_1m()"},

		// Individual refresh operations (matching initialRefresh operations)
		{Name: "refresh_latest_bar_views", Query: "SELECT refresh_latest_bar_views()"},

		// Only remaining continuous aggregates after migration 78
		{Name: "refresh_cagg_pre_market", Query: "CALL refresh_continuous_aggregate('cagg_pre_market', now() - INTERVAL '3 days', NULL)"},
		{Name: "refresh_cagg_extended_hours", Query: "CALL refresh_continuous_aggregate('cagg_extended_hours', now() - INTERVAL '3 days', NULL)"},
	},
	ComponentTests: []TestQuery{
		// Core data lookups using real stale tickers
		{Name: "screener_stale_lookup", Query: "SELECT ticker, last_update_time, stale FROM screener_stale WHERE ticker = ANY($1)"},
		{Name: "screener_final_view", Query: "SELECT ticker, price, volume, market_cap FROM screener WHERE ticker = ANY($1)"},

		// OHLCV data access patterns (what refresh_screener actually queries)
		{Name: "ohlcv_1m_latest_batch", Query: "SELECT ticker, timestamp, close/1000.0 as close FROM ohlcv_1m WHERE ticker = ANY($1) AND timestamp >= now() - INTERVAL '1 hour' ORDER BY timestamp DESC"},
		{Name: "ohlcv_1d_latest_batch", Query: "SELECT ticker, timestamp, close/1000.0 as close FROM ohlcv_1d WHERE ticker = ANY($1) AND timestamp >= now() - INTERVAL '30 days' ORDER BY timestamp DESC"},

		// Static reference lookups (what the screener depends on) - updated for migration 78
		{Name: "static_refs_daily_batch", Query: "SELECT ticker, price_1d, price_1w, price_1m, dma_50, dma_200, volatility_1w_pct, volatility_1m_pct, avg_volume_14d FROM static_refs_daily WHERE ticker = ANY($1)"},
		{Name: "static_refs_1m_batch", Query: "SELECT ticker, price_1m, price_15m, price_1h, price_4h, range_15m_pct, range_1h_pct, avg_volume_1m_14 FROM static_refs_1m WHERE ticker = ANY($1)"},

		// Securities metadata (active ticker filtering)
		{Name: "securities_active_batch", Query: "SELECT ticker, market_cap, sector, active FROM securities WHERE ticker = ANY($1) AND active = TRUE"},

		// Performance-critical joins (simulating screener calculation logic) - updated for migration 78
		{Name: "screener_join_simulation", Query: `
			SELECT s.ticker, s.market_cap, sr.price_1d, sr.dma_50, sr.dma_200, sr.volatility_1w_pct, 
				   sr1m.price_1m, sr1m.range_15m_pct, sr1m.range_1h_pct, o1d.close/1000.0 as current_price
			FROM securities s
			LEFT JOIN static_refs_daily sr ON s.ticker = sr.ticker
			LEFT JOIN static_refs_1m sr1m ON s.ticker = sr1m.ticker
			LEFT JOIN LATERAL (
				SELECT close FROM ohlcv_1d WHERE ticker = s.ticker ORDER BY timestamp DESC LIMIT 1
			) o1d ON true
			WHERE s.ticker = ANY($1) AND s.active = TRUE
		`},

		// Batch processing efficiency test
		{Name: "batch_stale_processing", Query: `
			WITH stale_batch AS (
				SELECT ticker FROM screener_stale WHERE stale = TRUE LIMIT $2
			)
			SELECT COUNT(*) as stale_count, 
				   COUNT(DISTINCT s.ticker) as active_count,
				   AVG(EXTRACT(EPOCH FROM now() - ss.last_update_time)) as avg_staleness_seconds
			FROM stale_batch sb
			JOIN screener_stale ss ON sb.ticker = ss.ticker
			JOIN securities s ON sb.ticker = s.ticker AND s.active = TRUE
		`},
	},
}
