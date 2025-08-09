// Package alerts contains the core alert processing loops for price and
// strategy alerts, including throttling, metrics, and Redis integration.
package alerts

import (
	"backend/internal/data"
	"backend/internal/data/postgres"
	"backend/internal/queue"
	"strings"

	"backend/internal/app/limits"
	"backend/internal/services/socket"
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// bucketStart calculates the start time of the bucket that contains the given time
// for the specified timeframe, using calendar-aligned boundaries
func bucketStart(t time.Time, tf string) (time.Time, error) {
	if tf == "" {
		return time.Time{}, fmt.Errorf("empty timeframe")
	}

	re := regexp.MustCompile(`^(\d+)([mhdwqy]?)$`)
	matches := re.FindStringSubmatch(strings.ToLower(tf))
	if matches == nil {
		return time.Time{}, fmt.Errorf("invalid timeframe format: %s", tf)
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid number in timeframe: %s", tf)
	}

	unit := matches[2]
	switch unit {
	case "", "m": // minutes (no unit means minutes)
		dur := time.Duration(n) * time.Minute
		return t.UTC().Truncate(dur), nil
	case "h": // hours
		dur := time.Duration(n) * time.Hour
		return t.UTC().Truncate(dur), nil
	case "d": // days - align to midnight ET
		etLoc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to load ET timezone: %w", err)
		}
		et := t.In(etLoc)
		y, m, d := et.Date()
		// For multi-day periods, align to epoch and find the correct bucket
		if n > 1 {
			epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, etLoc)
			daysSinceEpoch := int(et.Sub(epoch).Hours() / 24)
			bucketNumber := daysSinceEpoch / n
			bucketStart := epoch.AddDate(0, 0, bucketNumber*n)
			return bucketStart, nil
		}
		return time.Date(y, m, d, 0, 0, 0, 0, etLoc), nil
	case "w": // weeks - align to Monday midnight ET
		etLoc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to load ET timezone: %w", err)
		}
		et := t.In(etLoc)
		// Find the Monday of this week
		daysFromMonday := int(et.Weekday()-time.Monday) % 7
		if daysFromMonday < 0 {
			daysFromMonday += 7
		}
		monday := et.AddDate(0, 0, -daysFromMonday)
		y, m, d := monday.Date()
		weekStart := time.Date(y, m, d, 0, 0, 0, 0, etLoc)

		// For multi-week periods, align to epoch
		if n > 1 {
			epoch := time.Date(1970, 1, 5, 0, 0, 0, 0, etLoc) // Jan 5, 1970 was a Monday
			weeksSinceEpoch := int(weekStart.Sub(epoch).Hours() / (24 * 7))
			bucketNumber := weeksSinceEpoch / n
			return epoch.AddDate(0, 0, bucketNumber*n*7), nil
		}
		return weekStart, nil
	case "q": // quarters - align to quarter start (Jan/Apr/Jul/Oct 1st) midnight ET
		etLoc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to load ET timezone: %w", err)
		}
		et := t.In(etLoc)
		y, m, _ := et.Date()
		// Find quarter start month (1, 4, 7, 10)
		quarterStartMonth := ((int(m)-1)/3)*3 + 1
		quarterStart := time.Date(y, time.Month(quarterStartMonth), 1, 0, 0, 0, 0, etLoc)

		// For multi-quarter periods
		if n > 1 {
			quartersSinceEpoch := (y-1970)*4 + (quarterStartMonth-1)/3
			bucketNumber := quartersSinceEpoch / n
			bucketYear := 1970 + (bucketNumber*n)/4
			bucketQuarter := ((bucketNumber * n) % 4)
			bucketMonth := bucketQuarter*3 + 1
			return time.Date(bucketYear, time.Month(bucketMonth), 1, 0, 0, 0, 0, etLoc), nil
		}
		return quarterStart, nil
	case "y": // years - align to January 1st midnight ET
		etLoc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to load ET timezone: %w", err)
		}
		et := t.In(etLoc)
		y, _, _ := et.Date()

		// For multi-year periods
		if n > 1 {
			bucketYear := ((y-1970)/n)*n + 1970
			return time.Date(bucketYear, 1, 1, 0, 0, 0, 0, etLoc), nil
		}
		return time.Date(y, 1, 1, 0, 0, 0, 0, etLoc), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported timeframe unit: %s", unit)
	}
}

// isPerTickerThrottleEnabled checks if the per-ticker throttling feature is enabled
func isPerTickerThrottleEnabled() bool {
	return true
	//return strings.ToLower(os.Getenv("PER_TICKER_THROTTLE")) == "true"
}

// AlertService encapsulates the alert system and its state
type AlertService struct {
	conn           *data.Conn
	isRunning      bool
	stopChan       chan struct{}
	mutex          sync.RWMutex
	wg             sync.WaitGroup
	priceAlerts    sync.Map // key: alertID, value: PriceAlert
	strategyAlerts sync.Map // key: strategyID, value: StrategyAlert
	alertsMutex    sync.Mutex
}

// Global instance of the service
var alertService *AlertService
var serviceInitMutex sync.Mutex

// GetAlertService returns the singleton instance of AlertService
func GetAlertService() *AlertService {
	serviceInitMutex.Lock()
	defer serviceInitMutex.Unlock()

	if alertService == nil {
		alertService = &AlertService{
			stopChan: make(chan struct{}),
		}
	}
	return alertService
}

// Start initializes and starts the alert service (idempotent)
func (a *AlertService) Start(conn *data.Conn) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.isRunning {
		log.Printf("⚠️ Alert service already running")
		return nil
	}

	log.Printf("🚀 Starting Alert service")
	a.conn = conn

	// Initialize Telegram bot
	err := InitTelegramBot()
	log.Printf("🚀 Telegram bot initialized")
	if err != nil {
		return fmt.Errorf("failed to initialize Telegram bot: %w", err)

	}

	// Initialize price and strategy alerts
	log.Printf("🚀 Initializing price alerts")
	if err := a.initPriceAlerts(); err != nil {
		return fmt.Errorf("failed to initialize price alerts: %w", err)
	}
	log.Printf("🚀 Initializing strategy alerts")
	if err := a.initStrategyAlerts(); err != nil {
		return fmt.Errorf("failed to initialize strategy alerts: %w", err)
	}

	log.Printf("🚀 Initializing alerts")

	// Create new stop channel for this session
	a.stopChan = make(chan struct{})
	a.isRunning = true

	// Start the alert processing goroutines
	a.wg.Add(4) // Adding one more for cleanup scheduling
	log.Printf("🚀 Starting price alert loop")
	go a.priceAlertLoop()
	go a.strategyAlertLoop()
	go a.metricsLoop() // Metrics logging goroutine
	go a.cleanupLoop() // New cleanup scheduling goroutine

	log.Printf("✅ Alert service started")
	return nil
}

// Stop gracefully shuts down the alert service (idempotent)
func (a *AlertService) Stop() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.isRunning {
		log.Printf("⚠️ Alert service is not running")
		return nil
	}

	log.Printf("🛑 Stopping Alert service")

	// Signal the alert processing goroutines to stop
	close(a.stopChan)

	a.isRunning = false

	// Wait for the alert processing goroutines to finish
	a.wg.Wait()

	log.Printf("✅ Alert service stopped")
	return nil
}

// IsRunning returns whether the service is currently running
func (a *AlertService) IsRunning() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.isRunning
}

// WorkerStrategyAlertResult represents the result from a strategy alert execution
type WorkerStrategyAlertResult struct {
	Success         bool               `json:"success"`
	StrategyID      int                `json:"strategy_id"`
	ExecutionMode   string             `json:"execution_mode"`
	Matches         []WorkerAlertMatch `json:"alerts"`
	ExecutionTimeMs int                `json:"execution_time_ms"`
	ErrorMessage    string             `json:"error,omitempty"`
}

type WorkerAlertMatch struct {
	Symbol       string                 `json:"symbol"`
	Score        float64                `json:"score,omitempty"`
	CurrentPrice float64                `json:"current_price,omitempty"`
	Sector       string                 `json:"sector,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
}

// PriceAlert represents a price-based alert for a single security.
type PriceAlert struct {
	AlertID    int
	UserID     int
	Price      *float64
	Direction  *bool
	SecurityID *int
	Ticker     *string
}

// StrategyAlert represents an alert condition for a user-defined strategy.
type StrategyAlert struct {
	StrategyID   int
	UserID       int
	Name         string
	Threshold    float64
	Universe     string
	Active       bool
	MinTimeframe string
	LastTrigger  time.Time
}

var (
	priceAlertFrequency    = time.Second * 1
	strategyAlertFrequency = time.Second * 10
	// Legacy global variables for backward compatibility - DEPRECATED in Stage 3
	// TODO: Remove these in next major version after per-ticker throttling is stable
	priceAlerts    sync.Map // DEPRECATED: use AlertService instance instead
	strategyAlerts sync.Map // DEPRECATED: use AlertService instance instead
)

// Legacy wrapper functions for backward compatibility - DEPRECATED in Stage 3
// StartAlertLoop starts the alert service (wrapper around service-based approach)
// DEPRECATED: Use GetAlertService().Start() directly. Will be removed in next major version.
func StartAlertLoop(conn *data.Conn) error { //entrypoint
	log.Printf("⚠️ DEPRECATED: StartAlertLoop called - use GetAlertService().Start() directly")
	service := GetAlertService()
	return service.Start(conn)
}

// StopAlertLoop stops the alert service (wrapper around service-based approach)
// DEPRECATED: Use GetAlertService().Stop() directly. Will be removed in next major version.
func StopAlertLoop() {
	log.Printf("⚠️ DEPRECATED: StopAlertLoop called - use GetAlertService().Stop() directly")
	service := GetAlertService()
	_ = service.Stop()
}

// AddPriceAlert adds a price alert to the service's in-memory store
func AddPriceAlert(conn *data.Conn, alert PriceAlert) {
	service := GetAlertService()
	ticker, err := postgres.GetTicker(conn, *alert.SecurityID, time.Now())
	if err != nil {
		////fmt.Println("error getting ticker: %w", err)
		return
	}
	alert.Ticker = &ticker
	service.priceAlerts.Store(alert.AlertID, alert)

	// Also update legacy global map for backward compatibility
	priceAlerts.Store(alert.AlertID, alert)
}

// AddStrategyAlert adds a strategy alert to the service's in-memory store
func AddStrategyAlert(alert StrategyAlert) {
	service := GetAlertService()
	service.strategyAlerts.Store(alert.StrategyID, alert)

	// Also update legacy global map for backward compatibility
	strategyAlerts.Store(alert.StrategyID, alert)
}

// RemovePriceAlert removes a price alert from the service's in-memory store and decrements the counter
func RemovePriceAlert(conn *data.Conn, alertID int) error {
	service := GetAlertService()
	service.alertsMutex.Lock()
	defer service.alertsMutex.Unlock()

	// Get the alert before removing it to access user information
	if alertInterface, exists := service.priceAlerts.Load(alertID); exists {
		alert := alertInterface.(PriceAlert)

		// Only decrement counter for real alerts (not system alerts)
		if alert.UserID > 0 {
			// Decrement the active alerts counter for price alerts
			if err := limits.DecrementActiveAlerts(conn, alert.UserID, 1); err != nil {
				return fmt.Errorf("failed to decrement active alerts counter for user %d: %w", alert.UserID, err)
			}
		}
	}

	service.priceAlerts.Delete(alertID)

	// Also remove from legacy global map for backward compatibility
	priceAlerts.Delete(alertID)
	return nil
}

// RemoveStrategyAlert removes a strategy alert from the service's in-memory store and decrements the counter
func RemoveStrategyAlert(conn *data.Conn, strategyID int) error {
	service := GetAlertService()
	service.alertsMutex.Lock()
	defer service.alertsMutex.Unlock()

	// Get the alert before removing it to access user information
	if alertInterface, exists := service.strategyAlerts.Load(strategyID); exists {
		alert := alertInterface.(StrategyAlert)

		// Only decrement counter for real alerts
		if alert.UserID > 0 {
			// Decrement the active strategy alerts counter
			if err := limits.DecrementActiveStrategyAlerts(conn, alert.UserID, 1); err != nil {
				return fmt.Errorf("failed to decrement active strategy alerts counter for user %d: %w", alert.UserID, err)
			}
		}
	}

	service.strategyAlerts.Delete(strategyID)

	// Also remove from legacy global map for backward compatibility
	strategyAlerts.Delete(strategyID)
	return nil
}

// RemovePriceAlertFromMemory removes a price alert from the service's in-memory store without decrementing counters
// This is used when the counter has already been decremented elsewhere
func RemovePriceAlertFromMemory(alertID int) {
	service := GetAlertService()
	service.alertsMutex.Lock()
	defer service.alertsMutex.Unlock()
	service.priceAlerts.Delete(alertID)

	// Also remove from legacy global map for backward compatibility
	priceAlerts.Delete(alertID)
}

// RemoveStrategyAlertFromMemory removes a strategy alert from the service's in-memory store without decrementing counters
// This is used when the counter has already been decremented elsewhere
func RemoveStrategyAlertFromMemory(strategyID int) {
	service := GetAlertService()
	service.alertsMutex.Lock()
	defer service.alertsMutex.Unlock()
	service.strategyAlerts.Delete(strategyID)

	// Also remove from legacy global map for backward compatibility
	strategyAlerts.Delete(strategyID)
}

// priceAlertLoop is the service method that runs the price alert processing loop
func (a *AlertService) priceAlertLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(priceAlertFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopChan:
			log.Printf("📡 Price alert loop stopped by stop signal")
			return
		case <-ticker.C:
			a.processPriceAlerts()
		}
	}
}

// strategyAlertLoop is the service method that runs the strategy alert processing loop
func (a *AlertService) strategyAlertLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(strategyAlertFrequency)
	defer ticker.Stop()
	log.Printf("Starting strategy alert loop with frequency: %v", strategyAlertFrequency)

	for {
		select {
		case <-a.stopChan:
			log.Printf("📡 Strategy alert loop stopped by stop signal")
			return
		case <-ticker.C:
			log.Printf("Processing strategy alerts - checking %d active alerts", a.getStrategyAlertCount())
			startTime := time.Now()
			a.processStrategyAlerts()
			duration := time.Since(startTime)
			log.Printf("Strategy alert processing completed in %v", duration)
		}
	}
}

// metricsLoop logs Redis operation metrics periodically
func (a *AlertService) metricsLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(5 * time.Minute) // Log every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-a.stopChan:
			log.Printf("📡 Metrics loop stopped by stop signal")
			return
		case <-ticker.C:
			// Use enhanced metrics if per-ticker throttling is enabled
			if isPerTickerThrottleEnabled() {
				detailedMetrics := data.GetDetailedAlertMetrics(a.conn)
				log.Printf("📊 Enhanced Redis metrics - Ticker updates: %v, Universe updates: %v, Total tracked: %v",
					detailedMetrics["ticker_updates"], detailedMetrics["universe_updates"], detailedMetrics["total_ticker_updates"])
				log.Printf("📊 Per-ticker throttling - Strategy runs: %v, Skipped (no update): %v, Skipped (bucket dup): %v",
					detailedMetrics["strategy_runs"], detailedMetrics["skipped_no_update"], detailedMetrics["skipped_bucket_dup"])
				log.Printf("📊 Advanced operations - Cleanup ops: %v, Lua intersections: %v, Universe discoveries: %v",
					detailedMetrics["cleanup_operations"], detailedMetrics["lua_intersections"], detailedMetrics["universe_discoveries"])

				// Log universe size distribution for performance analysis
				a.logUniverseSizeMetrics()
			} else {
				// Legacy metrics for backward compatibility
				metrics := data.GetAlertMetrics()
				log.Printf("📊 Redis metrics - Ticker updates: %d, Universe updates: %d",
					metrics["ticker_updates"], metrics["universe_updates"])
				log.Printf("📊 Per-ticker throttling - Strategy runs: %d, Skipped (no update): %d, Skipped (bucket dup): %d",
					metrics["strategy_runs"], metrics["skipped_no_update"], metrics["skipped_bucket_dup"])
			}
		}
	}
}

// cleanupLoop performs periodic Redis cleanup operations
func (a *AlertService) cleanupLoop() {
	defer a.wg.Done()

	// Run cleanup daily at 2 AM
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run initial cleanup after 1 hour to avoid startup congestion
	initialDelay := time.NewTimer(1 * time.Hour)
	defer initialDelay.Stop()

	for {
		select {
		case <-a.stopChan:
			log.Printf("📡 Cleanup loop stopped by stop signal")
			return
		case <-initialDelay.C:
			// First cleanup run
			a.performCleanup()
		case <-ticker.C:
			// Daily cleanup runs
			a.performCleanup()
		}
	}
}

// performCleanup executes Redis cleanup operations
func (a *AlertService) performCleanup() {
	log.Printf("🧹 Starting Redis cleanup operations")

	// Clean up ticker updates older than 90 days (handles longest possible bucket timeframes)
	maxDays := 90
	if err := data.CleanupTickerUpdates(a.conn, maxDays); err != nil {
		log.Printf("⚠️ Failed to cleanup ticker updates: %v", err)
	}

	// Log current Redis data sizes for monitoring
	if tickerCount, err := data.GetTickerUpdateCount(a.conn); err == nil {
		log.Printf("📊 Post-cleanup: %d ticker updates tracked in Redis", tickerCount)
	}

	log.Printf("✅ Redis cleanup operations completed")
}

// logUniverseSizeMetrics logs universe size distribution for performance analysis
func (a *AlertService) logUniverseSizeMetrics() {
	var small, medium, large, xlarge int
	var totalUniverse int

	a.strategyAlerts.Range(func(_, value interface{}) bool {
		alert := value.(StrategyAlert)
		if size, err := data.GetUniverseSize(a.conn, alert.StrategyID); err == nil {
			totalUniverse += size
			switch {
			case size <= 10:
				small++
			case size <= 100:
				medium++
			case size <= 1000:
				large++
			default:
				xlarge++
			}
		}
		return true
	})

	activeStrategies := a.getStrategyAlertCount()
	if activeStrategies > 0 {
		avgUniverse := totalUniverse / activeStrategies
		log.Printf("📈 Universe distribution - Small (≤10): %d, Medium (≤100): %d, Large (≤1000): %d, XLarge (>1000): %d",
			small, medium, large, xlarge)
		log.Printf("📈 Average universe size: %d tickers across %d active strategies",
			avgUniverse, activeStrategies)
	}
}

// processPriceAlerts processes all active price alerts
func (a *AlertService) processPriceAlerts() {
	var wg sync.WaitGroup
	a.priceAlerts.Range(func(_, value interface{}) bool {
		alert := value.(PriceAlert)
		wg.Add(1)
		go func(alert PriceAlert) {
			defer wg.Done()
			if err := processPriceAlert(a.conn, alert); err != nil {
				log.Printf("Error processing price alert %d: %v", alert.AlertID, err)
			}
		}(alert)
		return true
	})
	wg.Wait()
}

// processStrategyAlerts processes all active strategy alerts
func (a *AlertService) processStrategyAlerts() {
	// Log currently active strategy alerts
	var activeAlerts []string
	a.strategyAlerts.Range(func(_, value interface{}) bool {
		alert := value.(StrategyAlert)
		activeAlerts = append(activeAlerts, fmt.Sprintf("ID:%d(%s)", alert.StrategyID, alert.Name))
		return true
	})
	log.Printf("📊 Processing %d active strategy alerts: [%s]", len(activeAlerts), strings.Join(activeAlerts, ", "))

	// Check if per-ticker throttling is enabled
	usePerTickerThrottle := isPerTickerThrottleEnabled()
	if usePerTickerThrottle {
		log.Printf("🎯 Using per-ticker throttling mode")
		a.processStrategyAlertsPerTicker()
	} else {
		log.Printf("🎯 Using legacy throttling mode")
		a.processStrategyAlertsLegacy()
	}
}

// processStrategyAlertsLegacy implements the original strategy-level throttling
func (a *AlertService) processStrategyAlertsLegacy() {
	var wg sync.WaitGroup
	var processed, succeeded, failed, skipped int
	var mu sync.Mutex

	a.strategyAlerts.Range(func(_, value interface{}) bool {
		alert := value.(StrategyAlert)
		wg.Add(1)
		go func(alert StrategyAlert) {
			defer wg.Done()

			// Check if we should skip this alert based on timeframe throttling
			if !alert.LastTrigger.IsZero() && alert.MinTimeframe != "" {
				currBucket, err := bucketStart(time.Now(), alert.MinTimeframe)
				if err != nil {
					log.Printf("⚠️ Strategy %d (%s): invalid timeframe '%s', skipping throttling: %v",
						alert.StrategyID, alert.Name, alert.MinTimeframe, err)
				} else {
					lastBucket, err := bucketStart(alert.LastTrigger, alert.MinTimeframe)
					if err != nil {
						log.Printf("⚠️ Strategy %d (%s): error calculating last trigger bucket, skipping throttling: %v",
							alert.StrategyID, alert.Name, err)
					} else if currBucket.Equal(lastBucket) {
						log.Printf("⏩ Strategy %d (%s) skipped - same bucket (current: %v, last trigger: %v)",
							alert.StrategyID, alert.Name, currBucket.Format("2006-01-02 15:04:05 MST"),
							alert.LastTrigger.Format("2006-01-02 15:04:05 MST"))
						mu.Lock()
						processed++
						skipped++
						mu.Unlock()
						return
					}
				}
			}

			log.Printf("Processing strategy alert %d: %s (threshold: %.2f)", alert.StrategyID, alert.Name, alert.Threshold)
			if err := executeStrategyAlert(context.Background(), a.conn, alert, nil); err != nil {
				log.Printf("Error processing strategy alert %d: %v", alert.StrategyID, err)
				mu.Lock()
				processed++
				failed++
				mu.Unlock()
			} else {
				log.Printf("Successfully processed strategy alert %d: %s", alert.StrategyID, alert.Name)
				mu.Lock()
				processed++
				succeeded++
				mu.Unlock()
			}
		}(alert)
		return true
	})
	wg.Wait()
	log.Printf("Strategy alert processing summary: %d total, %d succeeded, %d failed, %d skipped", processed, succeeded, failed, skipped)
}

// intersectClientSide performs client-side intersection of two ticker slices
func intersectClientSide(updatedTickers, strategyUniverse []string) []string {
	updatedSet := make(map[string]bool)
	for _, ticker := range updatedTickers {
		updatedSet[ticker] = true
	}

	var result []string
	for _, ticker := range strategyUniverse {
		if updatedSet[ticker] {
			result = append(result, ticker)
		}
	}
	return result
}

// processStrategyAlertsPerTicker implements per-ticker throttling using Redis data
func (a *AlertService) processStrategyAlertsPerTicker() {
	now := time.Now()

	var wg sync.WaitGroup
	var processed, succeeded, failed, skippedNoUpdate, skippedBucketDup int
	var mu sync.Mutex

	a.strategyAlerts.Range(func(_, value interface{}) bool {
		alert := value.(StrategyAlert)
		wg.Add(1)
		go func(alert StrategyAlert) {
			defer wg.Done()
			// DEBUG: start evaluation
			log.Printf("🔎 Evaluating strategy %d '%s': universe='%s', lastTrigger=%v, minTimeframe='%s'",
				alert.StrategyID, alert.Name, alert.Universe, alert.LastTrigger, alert.MinTimeframe)

			// Skip strategies with invalid timeframes
			if alert.MinTimeframe == "" {
				log.Printf("⚠️ Strategy %d (%s): no min_timeframe set, skipping per-ticker throttling",
					alert.StrategyID, alert.Name)
				mu.Lock()
				processed++
				skippedNoUpdate++
				mu.Unlock()
				data.IncrementSkippedNoUpdate()
				return
			}

			// Calculate current bucket
			currBucket, err := bucketStart(now, alert.MinTimeframe)
			if err != nil {
				log.Printf("⚠️ Strategy %d (%s): invalid timeframe '%s', skipping: %v",
					alert.StrategyID, alert.Name, alert.MinTimeframe, err)
				mu.Lock()
				processed++
				skippedNoUpdate++
				mu.Unlock()
				data.IncrementSkippedNoUpdate()
				return
			}
			log.Printf("⌚ Strategy %d: computed bucket start = %v", alert.StrategyID, currBucket)

			// Get tickers updated since current bucket start
			updatedTickers, err := data.GetTickersUpdatedSince(a.conn, currBucket.UnixMilli())
			if err != nil {
				log.Printf("⚠️ Strategy %d (%s): failed GetTickersUpdatedSince: %v",
					alert.StrategyID, alert.Name, err)
				mu.Lock()
				processed++
				skippedNoUpdate++
				mu.Unlock()
				data.IncrementSkippedNoUpdate()
				return
			}
			log.Printf("📈 Strategy %d: %d tickers updated since bucket %v", alert.StrategyID, len(updatedTickers), currBucket)

			// Check if this is a global strategy (no specific universe)
			if alert.Universe == "all" || alert.Universe == "" {
				// For global strategies, fall back to legacy throttling logic
				if !alert.LastTrigger.IsZero() {
					lastBucket, err := bucketStart(alert.LastTrigger, alert.MinTimeframe)
					if err == nil && currBucket.Equal(lastBucket) {
						log.Printf("⏩ Global strategy %d (%s) skipped - same bucket",
							alert.StrategyID, alert.Name)
						mu.Lock()
						processed++
						skippedBucketDup++
						mu.Unlock()
						data.IncrementSkippedBucketDup()
						return
					}
				}

				// Run global strategy without ticker filtering
				log.Printf("🌍 Processing global strategy %d: %s", alert.StrategyID, alert.Name)
				data.IncrementStrategyRuns()
				if err := executeStrategyAlert(context.Background(), a.conn, alert, nil); err != nil {
					log.Printf("Error processing global strategy %d: %v", alert.StrategyID, err)
					mu.Lock()
					processed++
					failed++
					mu.Unlock()
				} else {
					log.Printf("Successfully processed global strategy %d: %s", alert.StrategyID, alert.Name)
					mu.Lock()
					processed++
					succeeded++
					mu.Unlock()
				}
				return
			}

			// Get strategy universe from Redis
			strategyUniverse, err := data.GetStrategyUniverse(a.conn, alert.StrategyID)
			if err != nil {
				log.Printf("⚠️ Strategy %d (%s): Redis SMEMBERS failed: %v",
					alert.StrategyID, alert.Name, err)
				mu.Lock()
				processed++
				skippedNoUpdate++
				mu.Unlock()
				data.IncrementSkippedNoUpdate()
				return
			}
			log.Printf("📊 Strategy %d: universe size from Redis = %d tickers", alert.StrategyID, len(strategyUniverse))

			if len(strategyUniverse) == 0 {
				log.Printf("⚠️ Strategy %d (%s): empty universe in Redis, skipping",
					alert.StrategyID, alert.Name)
				mu.Lock()
				processed++
				skippedNoUpdate++
				mu.Unlock()
				data.IncrementSkippedNoUpdate()
				return
			}

			// Find intersection of updated tickers and strategy universe
			// Use Lua script for large universes to reduce network overhead
			var changedTickers []string

			const luaThreshold = 1000 // Use Lua script for universes > 1000 tickers
			if len(strategyUniverse) > luaThreshold {
				log.Printf("🔧 Strategy %d: using Lua script for large universe (%d tickers)",
					alert.StrategyID, len(strategyUniverse))
				luaResult, luaErr := data.IntersectTickersServerSide(a.conn, alert.StrategyID, currBucket.UnixMilli())
				if luaErr != nil {
					log.Printf("⚠️ Strategy %d: Lua intersection failed, falling back to client-side: %v",
						alert.StrategyID, luaErr)
					// Fall back to client-side intersection
					changedTickers = intersectClientSide(updatedTickers, strategyUniverse)
				} else {
					changedTickers = luaResult
					data.IncrementLuaIntersections()
				}
			} else {
				// Client-side intersection for smaller universes
				changedTickers = intersectClientSide(updatedTickers, strategyUniverse)
			}
			log.Printf("🤝 Strategy %d: %d changed tickers after intersection", alert.StrategyID, len(changedTickers))

			if len(changedTickers) == 0 {
				log.Printf("⏩ Strategy %d (%s) skipped - no universe tickers updated (%d universe, %d updated)",
					alert.StrategyID, alert.Name, len(strategyUniverse), len(updatedTickers))
				mu.Lock()
				processed++
				skippedNoUpdate++
				mu.Unlock()
				data.IncrementSkippedNoUpdate()
				return
			}

			// Get last trigger buckets for changed tickers
			lastBuckets, err := data.GetStrategyLastBuckets(a.conn, alert.StrategyID, changedTickers)
			if err != nil {
				log.Printf("⚠️ Strategy %d (%s): Redis HMGET last buckets failed: %v",
					alert.StrategyID, alert.Name, err)
				// Continue with execution - assume no previous triggers
			}
			log.Printf("🗂️ Strategy %d: last trigger buckets = %v", alert.StrategyID, lastBuckets)

			// Filter out tickers that already triggered in current bucket
			currBucketMs := currBucket.UnixMilli()
			var finalTickers []string
			for _, ticker := range changedTickers {
				if lastBucketMs, exists := lastBuckets[ticker]; !exists || lastBucketMs != currBucketMs {
					finalTickers = append(finalTickers, ticker)
				}
			}

			if len(finalTickers) == 0 {
				log.Printf("⏩ Strategy %d (%s) skipped - all changed tickers already triggered in bucket (%d changed, 0 final)",
					alert.StrategyID, alert.Name, len(changedTickers))
				mu.Lock()
				processed++
				skippedBucketDup++
				mu.Unlock()
				data.IncrementSkippedBucketDup()
				return
			}

			data.IncrementStrategyRuns()
			if err := executeStrategyAlert(context.Background(), a.conn, alert, finalTickers); err != nil {
				log.Printf("Error processing strategy %d: %v", alert.StrategyID, err)
				mu.Lock()
				processed++
				failed++
				mu.Unlock()
			} else {
				log.Printf("Successfully processed strategy %d: %s", alert.StrategyID, alert.Name)

				// Update last trigger buckets for successful execution
				tickerBuckets := make(map[string]int64)
				for _, ticker := range finalTickers {
					tickerBuckets[ticker] = currBucketMs
				}
				if err := data.SetStrategyLastBuckets(a.conn, alert.StrategyID, tickerBuckets); err != nil {
					log.Printf("⚠️ Strategy %d: failed to update last buckets: %v", alert.StrategyID, err)
				}

				mu.Lock()
				processed++
				succeeded++
				mu.Unlock()
			}
		}(alert)
		return true
	})
	wg.Wait()
	log.Printf("Per-ticker strategy alert summary: %d total, %d succeeded, %d failed, %d skipped (no update), %d skipped (bucket dup)",
		processed, succeeded, failed, skippedNoUpdate, skippedBucketDup)
}

// initPriceAlerts initializes price alerts from the database
func (a *AlertService) initPriceAlerts() error {
	ctx := context.Background()

	// Load active price alerts
	query := `
        SELECT alertId, userId, price, direction, securityId
        FROM alerts
        WHERE active = true
    `
	rows, err := a.conn.DB.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("querying active price alerts: %w", err)
	}
	defer rows.Close()

	a.priceAlerts = sync.Map{}
	for rows.Next() {
		var alert PriceAlert
		err := rows.Scan(
			&alert.AlertID,
			&alert.UserID,
			&alert.Price,
			&alert.Direction,
			&alert.SecurityID,
		)
		if err != nil {
			return fmt.Errorf("scanning price alert row: %w", err)
		}

		ticker, err := postgres.GetTicker(a.conn, *alert.SecurityID, time.Now())
		if err != nil {
			return fmt.Errorf("getting ticker: %w", err)
		}
		alert.Ticker = &ticker

		a.priceAlerts.Store(alert.AlertID, alert)

		// Also store in legacy global map for backward compatibility
		priceAlerts.Store(alert.AlertID, alert)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating price alert rows: %w", err)
	}

	log.Printf("Finished initializing %d price alerts", a.getPriceAlertCount())
	return nil
}

// initStrategyAlerts initializes strategy alerts from the database
func (a *AlertService) initStrategyAlerts() error {
	ctx := context.Background()
	log.Printf("🚀 Initializing strategy alerts")

	// Load active strategy alerts with configuration
	query := `
		SELECT strategyId, userId, name, 
		       COALESCE(alert_threshold, 0.0) as alert_threshold,
		       COALESCE(alert_universe, ARRAY[]::TEXT[]) as alert_universe,
		       COALESCE(min_timeframe, '1d') as min_timeframe,
		       alert_last_trigger_at
		FROM strategies 
		WHERE alertActive = true 
		ORDER BY strategyId
	`
	rows, err := a.conn.DB.Query(ctx, query)
	log.Printf("🚀 Querying active strategy alerts")
	if err != nil {
		log.Printf("🚀 Error querying active strategy alerts: %v", err)
		return fmt.Errorf("querying active strategy alerts: %w", err)
	}
	defer rows.Close()

	a.strategyAlerts = sync.Map{}
	log.Printf("🚀 Iterating strategy alert rows")
	for rows.Next() {
		var alert StrategyAlert
		var alertUniverse []string
		var lastTrigger *time.Time
		err := rows.Scan(&alert.StrategyID, &alert.UserID, &alert.Name, &alert.Threshold, &alertUniverse, &alert.MinTimeframe, &lastTrigger)
		if err != nil {
			return fmt.Errorf("scanning strategy alert row: %w", err)
		}
		alert.Active = true

		// Handle nullable last trigger time
		if lastTrigger != nil {
			alert.LastTrigger = *lastTrigger
		}

		// Convert universe array to string representation
		if len(alertUniverse) == 0 {
			alert.Universe = "all"
		} else {
			// For now, store as comma-separated string; could be enhanced later
			alert.Universe = fmt.Sprintf("%v", alertUniverse)
		}

		a.strategyAlerts.Store(alert.StrategyID, alert)

		// Also store in legacy global map for backward compatibility
		strategyAlerts.Store(alert.StrategyID, alert)

		// Sync strategy universe to Redis for per-ticker alert processing
		if err := a.syncStrategyUniverseToRedis(alert.StrategyID); err != nil {
			log.Printf("⚠️ Failed to sync strategy %d universe to Redis: %v", alert.StrategyID, err)
			// Don't fail initialization for Redis sync errors
		}
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating strategy alert rows: %w", err)
	}

	log.Printf("Finished initializing %d strategy alerts", a.getStrategyAlertCount())
	return nil
}

// Helper methods to get alert counts from the service
func (a *AlertService) getPriceAlertCount() int {
	count := 0
	a.priceAlerts.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func (a *AlertService) getStrategyAlertCount() int {
	count := 0
	a.strategyAlerts.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// syncStrategyUniverseToRedis syncs a strategy's universe from the database to Redis
func (a *AlertService) syncStrategyUniverseToRedis(strategyID int) error {
	ctx := context.Background()

	// Query the strategy's alert_universe_full from the database
	var alertUniverseFull []string
	query := `SELECT COALESCE(alert_universe_full, ARRAY[]::TEXT[]) FROM strategies WHERE strategyId = $1`
	err := a.conn.DB.QueryRow(ctx, query, strategyID).Scan(&alertUniverseFull)
	if err != nil {
		return fmt.Errorf("failed to query strategy %d universe: %w", strategyID, err)
	}

	// Only sync to Redis if we have a non-empty universe (global strategies are not stored)
	if len(alertUniverseFull) > 0 {
		if err := data.SetStrategyUniverse(a.conn, strategyID, alertUniverseFull); err != nil {
			return fmt.Errorf("failed to set strategy %d universe in Redis: %w", strategyID, err)
		}
		log.Printf("📝 Synced strategy %d universe to Redis: %d tickers", strategyID, len(alertUniverseFull))
	} else {
		log.Printf("📝 Strategy %d has global universe, not syncing to Redis", strategyID)
	}

	return nil
}

// waitForStrategyAlertResult waits for a strategy alert result via Redis pubsub
/*func waitForStrategyAlertResult(ctx context.Context, conn *data.Conn, taskID string, timeout time.Duration) (*WorkerStrategyAlertResult, error) {
	// Subscribe to task updates
	pubsub := conn.Cache.Subscribe(ctx, "worker_task_updates")
	defer func() {
		if err := pubsub.Close(); err != nil {
			fmt.Printf("error closing pubsub: %v\n", err)
		}
	}()

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := pubsub.Channel()
	log.Printf("Listening for updates on worker_task_updates channel for task %s", taskID)

	for {
		select {
		case <-timeoutCtx.Done():
			log.Printf("Timeout waiting for strategy alert result for task %s", taskID)
			return nil, fmt.Errorf("timeout waiting for strategy alert result")
		case msg := <-ch:
			if msg == nil {
				continue
			}

			var taskUpdate map[string]interface{}
			err := json.Unmarshal([]byte(msg.Payload), &taskUpdate)
			if err != nil {
				log.Printf("Failed to unmarshal task update: %v", err)
				continue
			}

			if taskUpdate["task_id"] == taskID {
				status, _ := taskUpdate["status"].(string)
				log.Printf("Received update for task %s: status=%s", taskID, status)

				if status == "completed" || status == "failed" {
					// Convert task result to WorkerStrategyAlertResult
					var result WorkerStrategyAlertResult
					if resultData, exists := taskUpdate["result"]; exists {
						resultJSON, err := json.Marshal(resultData)
						if err != nil {
							return nil, fmt.Errorf("error marshaling task result: %v", err)
						}

						err = json.Unmarshal(resultJSON, &result)
						if err != nil {
							return nil, fmt.Errorf("error unmarshaling strategy alert result: %v", err)
						}
					}

					if status == "failed" {
						errorMsg := "unknown error"
						if result.ErrorMessage != "" {
							errorMsg = result.ErrorMessage
						} else if errorData, exists := taskUpdate["error_message"]; exists {
							if errorStr, ok := errorData.(string); ok {
								errorMsg = errorStr
							}
						}
						log.Printf("Strategy alert task %s failed: %s", taskID, errorMsg)
						return nil, fmt.Errorf("strategy alert execution failed: %s", errorMsg)
					}

					log.Printf("Strategy alert task %s completed successfully", taskID)
					return &result, nil
				}
			}
		}
	}
}*/

// executeStrategyAlert submits a strategy alert task and waits for results
func executeStrategyAlert(ctx context.Context, conn *data.Conn, strategy StrategyAlert, tickers []string) error {
	// Prepare arguments expected by the Python worker (see services/worker/src/alert.py)
	args := map[string]interface{}{
		"strategy_id": strategy.StrategyID,
		"user_id":     strategy.UserID,
	}

	// Use provided tickers if available (per-ticker throttling mode), otherwise parse universe
	if len(tickers) > 0 {
		args["symbols"] = tickers
		log.Printf("🎯 Strategy %d (%s): submitting alert task with per-ticker filtered symbols (%d): %v",
			strategy.StrategyID, strategy.Name, len(tickers), tickers)
	} else {
		// Convert the Universe string into a slice of symbols if it is not the special "all" keyword.
		if strategy.Universe != "" && strategy.Universe != "all" {
			var symbols []string
			if strings.HasPrefix(strategy.Universe, "[") && strings.HasSuffix(strategy.Universe, "]") {
				// Universe is in array representation like "[AAPL MSFT TSLA]" – split on whitespace
				universeStr := strings.Trim(strategy.Universe, "[]")
				if universeStr != "" {
					symbols = strings.Fields(universeStr)
				}
			} else {
				// Assume comma-separated list
				parts := strings.Split(strategy.Universe, ",")
				for _, p := range parts {
					if sym := strings.TrimSpace(p); sym != "" {
						symbols = append(symbols, sym)
					}
				}
			}
			if len(symbols) > 0 {
				args["symbols"] = symbols
				log.Printf("🎯 Strategy %d (%s): submitting alert task with %d symbols: %v", strategy.StrategyID, strategy.Name, len(symbols), symbols)
			} else {
				log.Printf("🎯 Strategy %d (%s): submitting alert task with default universe (no symbols filter)", strategy.StrategyID, strategy.Name)
			}
		} else {
			log.Printf("🎯 Strategy %d (%s): submitting alert task with default universe (no symbols filter)", strategy.StrategyID, strategy.Name)
		}
	}

	log.Printf("🚀 Strategy %d (%s): queuing alert task with args: %+v", strategy.StrategyID, strategy.Name, args)
	// Submit the alert task through the unified queue system and wait for the typed result.
	result, err := queue.AlertTyped(ctx, conn, args)
	if err != nil {
		log.Printf("❌ Strategy %d (%s): queue submission failed: %v", strategy.StrategyID, strategy.Name, err)
		return fmt.Errorf("queue alert error: %w", err)
	}

	log.Printf("📥 Strategy %d (%s): received result - Success: %t, Instances: %d", strategy.StrategyID, strategy.Name, result.Success, len(result.Instances))

	// Process used_symbols for universe discovery if available
	if len(result.UsedSymbols) > 0 {
		log.Printf("🔍 Strategy %d (%s): worker reported %d used symbols: %v",
			strategy.StrategyID, strategy.Name, len(result.UsedSymbols), result.UsedSymbols)

		// Update strategy universe in Redis with discovered symbols
		if err := data.SetStrategyUniverse(conn, strategy.StrategyID, result.UsedSymbols); err != nil {
			log.Printf("⚠️ Strategy %d: failed to update discovered universe in Redis: %v", strategy.StrategyID, err)
		} else {
			data.IncrementUniverseDiscoveries()
			log.Printf("📝 Strategy %d: updated Redis universe with %d discovered symbols",
				strategy.StrategyID, len(result.UsedSymbols))
		}

		// Optionally update database for persistence (could be done async)
		go func() {
			ctx := context.Background()
			_, updateErr := conn.DB.Exec(ctx,
				`UPDATE strategies SET alert_universe_full = $1 WHERE strategyid = $2`,
				result.UsedSymbols, strategy.StrategyID)
			if updateErr != nil {
				log.Printf("⚠️ Strategy %d: failed to update discovered universe in database: %v",
					strategy.StrategyID, updateErr)
			} else {
				log.Printf("💾 Strategy %d: updated database universe with discovered symbols", strategy.StrategyID)
			}
		}()
	}

	if !result.Success {
		// Prefer structured error details if available
		if result.Error != nil {
			log.Printf("❌ Strategy %d (%s): task failed with structured error - Type: %s, Message: %s", strategy.StrategyID, strategy.Name, result.Error.Type, result.Error.Message)
			return fmt.Errorf("alert task failed: %s: %s", result.Error.Type, result.Error.Message)
		}
		if result.ErrorMessage != "" {
			log.Printf("❌ Strategy %d (%s): task failed with error message: %s", strategy.StrategyID, strategy.Name, result.ErrorMessage)
			return fmt.Errorf("alert task failed: %s", result.ErrorMessage)
		}
		log.Printf("❌ Strategy %d (%s): task reported unsuccessful status without error details", strategy.StrategyID, strategy.Name)
		return fmt.Errorf("alert task reported unsuccessful status without details")
	}

	numInstances := len(result.Instances)
	if numInstances == 0 {
		// Nothing matched – nothing to notify
		log.Printf("📭 Strategy %d (%s): no instances matched, no notifications sent", strategy.StrategyID, strategy.Name)
		return nil
	}

	// Build notification message & extract tickers for logging / payload
	message := fmt.Sprintf("Strategy '%s' triggered with %d matching securities", strategy.Name, numInstances)

	var hitTickers []string
	for _, inst := range result.Instances {
		if symRaw, ok := inst["symbol"]; ok {
			if sym, ok := symRaw.(string); ok && sym != "" {
				hitTickers = append(hitTickers, sym)
				continue
			}
		}
		if symRaw, ok := inst["ticker"]; ok {
			if sym, ok := symRaw.(string); ok && sym != "" {
				hitTickers = append(hitTickers, sym)
			}
		}
	}

	tickerCSV := strings.Join(hitTickers, ",")
	log.Printf("🎉 Strategy %d (%s): %d instances matched, tickers: [%s]", strategy.StrategyID, strategy.Name, numInstances, tickerCSV)

	additionalData := map[string]interface{}{
		"num_matches": numInstances,
		"ticker":      tickerCSV,
	}

	// Include full instances payload if the size is reasonable
	if numInstances <= 50 {
		additionalData["instances"] = result.Instances
		log.Printf("📊 Strategy %d (%s): including full instances in log payload (%d instances)", strategy.StrategyID, strategy.Name, numInstances)
	} else {
		log.Printf("📊 Strategy %d (%s): too many instances (%d) to include in log payload", strategy.StrategyID, strategy.Name, numInstances)
	}

	if err := LogStrategyAlert(conn, strategy.UserID, strategy.StrategyID, strategy.Name, message, additionalData); err != nil {
		log.Printf("Warning: failed to log strategy alert for strategy %d: %v", strategy.StrategyID, err)
	} else {
		log.Printf("📝 Strategy %d (%s): successfully logged alert to database", strategy.StrategyID, strategy.Name)
	}

	// Update last trigger time in database and in-memory
	_, err = conn.DB.Exec(ctx,
		`UPDATE strategies SET alert_last_trigger_at = NOW() WHERE strategyid = $1`,
		strategy.StrategyID)
	if err != nil {
		log.Printf("Warning: failed to update last trigger time for strategy %d: %v", strategy.StrategyID, err)
	} else {
		// Update in-memory copy as well
		service := GetAlertService()
		strategy.LastTrigger = time.Now()
		service.strategyAlerts.Store(strategy.StrategyID, strategy)
		log.Printf("⏰ Strategy %d (%s): updated last trigger time", strategy.StrategyID, strategy.Name)
	}

	// Dispatch Telegram and WebSocket notifications (best-effort)
	if err := SendTelegramMessage(message, chatID); err != nil {
		log.Printf("Warning: failed to send Telegram message for strategy %d: %v", strategy.StrategyID, err)
	} else {
		log.Printf("📱 Strategy %d (%s): successfully sent Telegram notification", strategy.StrategyID, strategy.Name)
	}

	socket.SendAlertToUser(strategy.UserID, socket.AlertMessage{
		AlertID:   strategy.StrategyID,
		Timestamp: time.Now().Unix() * 1000,
		Message:   message,
		Channel:   "alert",
		Type:      "strategy",
		Tickers:   hitTickers,
	})
	log.Printf("🔔 Strategy %d (%s): sent WebSocket notification to user %d", strategy.StrategyID, strategy.Name, strategy.UserID)

	return nil
}
