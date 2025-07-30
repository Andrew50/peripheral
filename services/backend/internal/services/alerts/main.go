package alerts

import (
	"backend/internal/data"
	"backend/internal/data/postgres"
	"encoding/json"
	"strings"

	"backend/internal/app/limits"
	"backend/internal/services/socket"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

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
	StrategyID int
	UserID     int
	Name       string
	Threshold  float64
	Universe   string
	Active     bool
}

var (
	priceAlertFrequency    = time.Second * 1
	strategyAlertFrequency = time.Minute * 1
	priceAlerts            sync.Map // key: alertID, value: PriceAlert
	strategyAlerts         sync.Map // key: strategyID, value: StrategyAlert
	ctx                    context.Context
	cancel                 context.CancelFunc
	mu                     sync.Mutex
)

// AddPriceAlert adds a price alert to the in-memory store
func AddPriceAlert(conn *data.Conn, alert PriceAlert) {
	ticker, err := postgres.GetTicker(conn, *alert.SecurityID, time.Now())
	if err != nil {
		////fmt.Println("error getting ticker: %w", err)
		return
	}
	alert.Ticker = &ticker
	priceAlerts.Store(alert.AlertID, alert)
}

// AddStrategyAlert adds a strategy alert to the in-memory store
func AddStrategyAlert(alert StrategyAlert) {
	strategyAlerts.Store(alert.StrategyID, alert)
}

// RemovePriceAlert removes a price alert from the in-memory store and decrements the counter
func RemovePriceAlert(conn *data.Conn, alertID int) error {
	mu.Lock()
	defer mu.Unlock()

	// Get the alert before removing it to access user information
	if alertInterface, exists := priceAlerts.Load(alertID); exists {
		alert := alertInterface.(PriceAlert)

		// Only decrement counter for real alerts (not system alerts)
		if alert.UserID > 0 {
			// Decrement the active alerts counter for price alerts
			if err := limits.DecrementActiveAlerts(conn, alert.UserID, 1); err != nil {
				return fmt.Errorf("failed to decrement active alerts counter for user %d: %w", alert.UserID, err)
			}
		}
	}

	priceAlerts.Delete(alertID)
	return nil
}

// RemoveStrategyAlert removes a strategy alert from the in-memory store and decrements the counter
func RemoveStrategyAlert(conn *data.Conn, strategyID int) error {
	mu.Lock()
	defer mu.Unlock()

	// Get the alert before removing it to access user information
	if alertInterface, exists := strategyAlerts.Load(strategyID); exists {
		alert := alertInterface.(StrategyAlert)

		// Only decrement counter for real alerts
		if alert.UserID > 0 {
			// Decrement the active strategy alerts counter
			if err := limits.DecrementActiveStrategyAlerts(conn, alert.UserID, 1); err != nil {
				return fmt.Errorf("failed to decrement active strategy alerts counter for user %d: %w", alert.UserID, err)
			}
		}
	}

	strategyAlerts.Delete(strategyID)
	return nil
}

// RemovePriceAlertFromMemory removes a price alert from the in-memory store without decrementing counters
// This is used when the counter has already been decremented elsewhere
func RemovePriceAlertFromMemory(alertID int) {
	mu.Lock()
	defer mu.Unlock()
	priceAlerts.Delete(alertID)
}

// RemoveStrategyAlertFromMemory removes a strategy alert from the in-memory store without decrementing counters
// This is used when the counter has already been decremented elsewhere
func RemoveStrategyAlertFromMemory(strategyID int) {
	mu.Lock()
	defer mu.Unlock()
	strategyAlerts.Delete(strategyID)
}

// StartAlertLoop performs operations related to StartAlertLoop functionality.
func StartAlertLoop(conn *data.Conn) error { //entrypoint
	err := InitTelegramBot()
	if err != nil {
		return err
	}
	if err := initPriceAlerts(conn); err != nil {
		return err
	}
	if err := initStrategyAlerts(conn); err != nil {
		return err
	}

	ctx, cancel = context.WithCancel(context.Background())
	go priceAlertLoop(ctx, conn)
	go strategyAlertLoop(ctx, conn)
	return nil
}

// StopAlertLoop performs operations related to StopAlertLoop functionality.
func StopAlertLoop() {
	if cancel != nil {
		cancel()
	}
}

func priceAlertLoop(ctx context.Context, conn *data.Conn) {
	ticker := time.NewTicker(priceAlertFrequency)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processPriceAlerts(conn)
		}
	}
}

func strategyAlertLoop(ctx context.Context, conn *data.Conn) {
	ticker := time.NewTicker(strategyAlertFrequency)
	defer ticker.Stop()
	log.Printf("Starting strategy alert loop with frequency: %v", strategyAlertFrequency)
	for {
		select {
		case <-ctx.Done():
			log.Printf("Strategy alert loop stopped due to context cancellation")
			return
		case <-ticker.C:
			log.Printf("Processing strategy alerts - checking %d active alerts", getStrategyAlertCount())
			startTime := time.Now()
			processStrategyAlerts(conn)
			duration := time.Since(startTime)
			log.Printf("Strategy alert processing completed in %v", duration)
		}
	}
}

func processPriceAlerts(conn *data.Conn) {
	var wg sync.WaitGroup
	priceAlerts.Range(func(_, value interface{}) bool {
		alert := value.(PriceAlert)
		wg.Add(1)
		go func(a PriceAlert) {
			defer wg.Done()
			if err := processPriceAlert(conn, a); err != nil {
				//log.Printf("Error processing price alert %d: %v", a.AlertID, err)
			}
		}(alert)
		return true
	})
	wg.Wait()
}

func processStrategyAlerts(conn *data.Conn) {
	var wg sync.WaitGroup
	var processed, succeeded, failed int
	var mu sync.Mutex

	strategyAlerts.Range(func(_, value interface{}) bool {
		alert := value.(StrategyAlert)
		wg.Add(1)
		go func(a StrategyAlert) {
			defer wg.Done()
			log.Printf("Processing strategy alert %d: %s (threshold: %.2f)", a.StrategyID, a.Name, a.Threshold)
			if err := executeStrategyAlert(context.Background(), conn, a); err != nil {
				log.Printf("Error processing strategy alert %d: %v", a.StrategyID, err)
				mu.Lock()
				processed++
				failed++
				mu.Unlock()
			} else {
				log.Printf("Successfully processed strategy alert %d: %s", a.StrategyID, a.Name)
				mu.Lock()
				processed++
				succeeded++
				mu.Unlock()
			}
		}(alert)
		return true
	})
	wg.Wait()
	log.Printf("Strategy alert processing summary: %d total, %d succeeded, %d failed", processed, succeeded, failed)
}

func initPriceAlerts(conn *data.Conn) error {
	ctx := context.Background()

	// Load active price alerts
	query := `
        SELECT alertId, userId, price, direction, securityId
        FROM alerts
        WHERE active = true
    `
	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("querying active price alerts: %w", err)
	}
	defer rows.Close()

	priceAlerts = sync.Map{}
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

		ticker, err := postgres.GetTicker(conn, *alert.SecurityID, time.Now())
		if err != nil {
			return fmt.Errorf("getting ticker: %w", err)
		}
		alert.Ticker = &ticker

		priceAlerts.Store(alert.AlertID, alert)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating price alert rows: %w", err)
	}

	//log.Printf("Finished initializing %d price alerts", getPriceAlertCount())
	return nil
}

func initStrategyAlerts(conn *data.Conn) error {
	ctx := context.Background()

	// Load active strategy alerts with configuration
	query := `
		SELECT strategyId, userId, name, 
		       COALESCE(alert_threshold, 0.0) as alert_threshold,
		       COALESCE(alert_universe, ARRAY[]::TEXT[]) as alert_universe
		FROM strategies 
		WHERE alertactive = true 
		ORDER BY strategyId
	`
	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("querying active strategy alerts: %w", err)
	}
	defer rows.Close()

	strategyAlerts = sync.Map{}
	for rows.Next() {
		var alert StrategyAlert
		var alertUniverse []string
		err := rows.Scan(&alert.StrategyID, &alert.UserID, &alert.Name, &alert.Threshold, &alertUniverse)
		if err != nil {
			return fmt.Errorf("scanning strategy alert row: %w", err)
		}
		alert.Active = true

		// Convert universe array to string representation
		if len(alertUniverse) == 0 {
			alert.Universe = "all"
		} else {
			// For now, store as comma-separated string; could be enhanced later
			alert.Universe = fmt.Sprintf("%v", alertUniverse)
		}

		strategyAlerts.Store(alert.StrategyID, alert)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterating strategy alert rows: %w", err)
	}

	//log.Printf("Finished initializing %d strategy alerts", getStrategyAlertCount())
	return nil
}

// Helper functions to get alert counts
func getPriceAlertCount() int {
	count := 0
	priceAlerts.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func getStrategyAlertCount() int {
	count := 0
	strategyAlerts.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// waitForStrategyAlertResult waits for a strategy alert result via Redis pubsub
func waitForStrategyAlertResult(ctx context.Context, conn *data.Conn, taskID string, timeout time.Duration) (*WorkerStrategyAlertResult, error) {
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
}

// executeStrategyAlert submits a strategy alert task and waits for results
func executeStrategyAlert(ctx context.Context, conn *data.Conn, strategy StrategyAlert) error {
	// Generate unique task ID
	taskID := fmt.Sprintf("strategy_alert_%d_%d", strategy.StrategyID, time.Now().UnixNano())
	//log.Printf("Executing strategy alert %d (task: %s)", strategy.StrategyID, taskID)
	// Log the configured universe for additional debugging
	//log.Printf("Strategy alert %d universe string: %s", strategy.StrategyID, strategy.Universe)
	// Log the alert threshold as well for completeness
	//log.Printf("Strategy alert %d alert threshold: %.2f", strategy.StrategyID, strategy.Threshold)

	// Prepare strategy alert task payload
	task := map[string]interface{}{
		"task_id":   taskID,
		"task_type": "alert",
		"args": map[string]interface{}{
			"strategy_id": fmt.Sprintf("%d", strategy.StrategyID),
			"user_id":     fmt.Sprintf("%d", strategy.UserID),
		},
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}

	// Add universe parameter - convert "all" to nil to indicate default universe should be used
	if strategy.Universe == "all" {
		task["args"].(map[string]interface{})["universe"] = nil
		log.Printf("Strategy alert %d: using default universe (converted 'all' to nil)", strategy.StrategyID)
	} else {
		// Parse universe string back to array if it's not "all"
		// For now, assume it's a comma-separated string or array representation
		if strategy.Universe != "" {
			// Simple parsing - could be enhanced based on actual format
			var universe []string
			if strings.HasPrefix(strategy.Universe, "[") && strings.HasSuffix(strategy.Universe, "]") {
				// Handle array representation like "[AAPL MSFT TSLA]"
				universeStr := strings.Trim(strategy.Universe, "[]")
				if universeStr != "" {
					universe = strings.Fields(universeStr)
				}
			} else {
				// Handle comma-separated format
				universe = strings.Split(strategy.Universe, ",")
				for i := range universe {
					universe[i] = strings.TrimSpace(universe[i])
				}
			}
			task["args"].(map[string]interface{})["universe"] = universe
			//log.Printf("Strategy alert %d: using specific universe with %d symbols", strategy.StrategyID, len(universe))
		} else {
			task["args"].(map[string]interface{})["universe"] = nil
			//log.Printf("Strategy alert %d: empty universe string, using default universe", strategy.StrategyID)
		}
	}

	// Convert task to JSON
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("error marshaling task: %v", err)
	}

	// Submit task to Redis worker queue
	log.Printf("Submitting strategy alert task %s to Redis queue", taskID)
	err = conn.Cache.RPush(ctx, "strategy_queue", string(taskJSON)).Err()
	if err != nil {
		return fmt.Errorf("error submitting task to queue: %v", err)
	}

	// Wait for result with 2 minute timeout
	log.Printf("Waiting for strategy alert result for task %s (timeout: 2 minutes)", taskID)
	result, err := waitForStrategyAlertResult(ctx, conn, taskID, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("error waiting for strategy alert result: %v", err)
	}

	// If the strategy alert was successful, log it
	if result.Success {
		// Create a descriptive message
		numMatches := len(result.Matches)
		var message string
		if numMatches > 0 {
			message = fmt.Sprintf("Strategy '%s' triggered with %d matching securities", strategy.Name, numMatches)
			log.Printf("Strategy alert %d triggered: %s", strategy.StrategyID, message)
		} else {
			return nil
		}
		// Extract matched tickers for logging
		var hitTickers []string
		for _, match := range result.Matches {
			hitTickers = append(hitTickers, match.Symbol)
		}
		tickerCSV := strings.Join(hitTickers, ",")

		// Prepare additional data for the payload, including comma-separated tickers
		additionalData := map[string]interface{}{
			"execution_mode":    result.ExecutionMode,
			"execution_time_ms": result.ExecutionTimeMs,
			"num_matches":       numMatches,
			"ticker":            tickerCSV,
		}

		// Add match details if available (limit to prevent huge payloads)
		if numMatches > 0 && numMatches <= 50 {
			matches := make([]map[string]interface{}, 0, len(result.Matches))
			for _, match := range result.Matches {
				matchData := map[string]interface{}{
					"symbol": match.Symbol,
				}
				if match.Score != 0 {
					matchData["score"] = match.Score
				}
				if match.CurrentPrice != 0 {
					matchData["current_price"] = match.CurrentPrice
				}
				if match.Sector != "" {
					matchData["sector"] = match.Sector
				}
				matches = append(matches, matchData)
			}
			additionalData["matches"] = matches

			// Log a sample of the matches
			sampleSize := 3
			if numMatches < sampleSize {
				sampleSize = numMatches
			}
			log.Printf("Strategy alert %d sample matches: %+v", strategy.StrategyID, result.Matches[:sampleSize])
		} else if numMatches > 50 {
			additionalData["matches_note"] = fmt.Sprintf("Too many matches (%d) to include details", numMatches)
			log.Printf("Strategy alert %d has too many matches (%d) to log details", strategy.StrategyID, numMatches)
		}

		err = LogStrategyAlert(conn, strategy.UserID, strategy.StrategyID, strategy.Name, message, additionalData)
		if err != nil {
			log.Printf("Warning: failed to log strategy alert for strategy %d: %v", strategy.StrategyID, err)
			// Don't fail the entire alert processing if logging fails
		}
		// Dispatch Telegram and WebSocket notifications for strategy alert
		if err2 := SendTelegramMessage(message, chatID); err2 != nil {
			log.Printf("Warning: failed to send Telegram message for strategy %d: %v", strategy.StrategyID, err2)
		}
		socket.SendAlertToUser(strategy.UserID, socket.AlertMessage{
			AlertID:   strategy.StrategyID,
			Timestamp: time.Now().Unix() * 1000,
			Message:   message,
			Channel:   "alert",
			Type:      "strategy",
			Tickers:   hitTickers,
		})
	} else {
		log.Printf("Strategy alert %d execution completed but marked as not successful", strategy.StrategyID)
	}

	return nil
}
