package screener

import (
	"backend/internal/data"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Market hours constants (Eastern Time)
const (
	MarketOpenHour    = 9
	MarketOpenMinute  = 30
	MarketCloseHour   = 16
	MarketCloseMinute = 0
	ExtendedOpenHour  = 4
	ExtendedCloseHour = 20
)

var (
	screenerUpdaterRunning bool
	screenerUpdaterMutex   sync.Mutex
	stopScreenerUpdater    chan struct{}
)

// isMarketOpen checks if the current time is during market hours (including extended hours)
func isMarketOpen(now time.Time) bool {
	// Check if it's a weekend
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	// Check if it's during extended hours (4:00 AM to 8:00 PM ET)
	hour := now.Hour()
	return hour >= ExtendedOpenHour && hour < ExtendedCloseHour
}

// isPostMarketClosed checks if post-market has closed (after 8:00 PM ET)
func isPostMarketClosed(now time.Time) bool {
	hour := now.Hour()
	return hour >= ExtendedCloseHour
}

// refreshScreenerAggregate refreshes the screener continuous aggregate
func refreshScreenerAggregate(conn *data.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Call the TimescaleDB function to refresh the continuous aggregate
	_, err := conn.DB.Exec(ctx, "CALL refresh_continuous_aggregate('screener_ca', NULL, NULL)")
	if err != nil {
		return fmt.Errorf("failed to refresh screener continuous aggregate: %w", err)
	}

	return nil
}

// StartScreenerUpdater starts the screener updater service
// It refreshes the screener continuous aggregate every 10 seconds during market hours
// and stops when post-market closes (8:00 PM ET)
func StartScreenerUpdater(conn *data.Conn) error {
	screenerUpdaterMutex.Lock()
	defer screenerUpdaterMutex.Unlock()

	if screenerUpdaterRunning {
		log.Println("⚠️ Screener updater already running")
		return nil
	}

	// Load Eastern timezone
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return fmt.Errorf("failed to load Eastern timezone: %w", err)
	}

	// Create stop channel
	stopScreenerUpdater = make(chan struct{})
	screenerUpdaterRunning = true

	log.Println("🚀 Starting screener updater service")

	go func() {
		defer func() {
			screenerUpdaterMutex.Lock()
			screenerUpdaterRunning = false
			screenerUpdaterMutex.Unlock()
			log.Println("⏹️ Screener updater service stopped")
		}()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopScreenerUpdater:
				return
			case <-ticker.C:
				now := time.Now().In(easternLocation)

				// Check if market is open
				if !isMarketOpen(now) {
					// If post-market has closed, stop the service
					if isPostMarketClosed(now) {
						log.Println("🌙 Post-market closed, stopping screener updater")
						return
					}
					// If market is not open yet, continue waiting
					continue
				}

				// Refresh the screener aggregate
				if err := refreshScreenerAggregate(conn); err != nil {
					log.Printf("❌ Failed to refresh screener aggregate: %v", err)
					// Continue running even if refresh fails
					continue
				}

				log.Printf("✅ Screener aggregate refreshed at %s", now.Format("15:04:05"))
			}
		}
	}()

	return nil
}

// StopScreenerUpdater stops the screener updater service
func StopScreenerUpdater() {
	screenerUpdaterMutex.Lock()
	defer screenerUpdaterMutex.Unlock()

	if screenerUpdaterRunning && stopScreenerUpdater != nil {
		close(stopScreenerUpdater)
		screenerUpdaterRunning = false
		log.Println("🛑 Screener updater stop requested")
	}
}

// IsScreenerUpdaterRunning returns whether the screener updater is currently running
func IsScreenerUpdaterRunning() bool {
	screenerUpdaterMutex.Lock()
	defer screenerUpdaterMutex.Unlock()
	return screenerUpdaterRunning
}
