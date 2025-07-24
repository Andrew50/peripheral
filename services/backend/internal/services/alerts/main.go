package alerts

import (
	"backend/internal/data"
	"backend/internal/data/postgres"
	"encoding/json"

	"backend/internal/app/limits"
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
	Matches         []WorkerAlertMatch `json:"matches"`
	ExecutionTimeMs int                `json:"execution_time_ms"`
	ErrorMessage    string             `json:"error_message,omitempty"`
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
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processStrategyAlerts(conn)
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
	strategyAlerts.Range(func(_, value interface{}) bool {
		alert := value.(StrategyAlert)
		wg.Add(1)
		go func(a StrategyAlert) {
			defer wg.Done()
			if err := executeStrategyAlert(context.Background(), conn, a); err != nil {
				log.Printf("Error processing strategy alert %d: %v", a.StrategyID, err)
			}
		}(alert)
		return true
	})
	wg.Wait()
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

	log.Printf("Finished initializing %d price alerts", getPriceAlertCount())
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
		WHERE isAlertActive = true 
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

	log.Printf("Finished initializing %d strategy alerts", getStrategyAlertCount())
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

	for {
		select {
		case <-timeoutCtx.Done():
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
						return nil, fmt.Errorf("strategy alert execution failed: %s", errorMsg)
					}

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

	// Convert task to JSON
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("error marshaling task: %v", err)
	}

	// Submit task to Redis worker queue
	err = conn.Cache.RPush(ctx, "strategy_queue", string(taskJSON)).Err()
	if err != nil {
		return fmt.Errorf("error submitting task to queue: %v", err)
	}

	// Wait for result with 2 minute timeout
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
		} else {
			message = fmt.Sprintf("Strategy '%s' executed successfully", strategy.Name)
		}

		// Prepare additional data for the payload
		additionalData := map[string]interface{}{
			"execution_mode":    result.ExecutionMode,
			"execution_time_ms": result.ExecutionTimeMs,
			"num_matches":       numMatches,
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
		} else if numMatches > 50 {
			additionalData["matches_note"] = fmt.Sprintf("Too many matches (%d) to include details", numMatches)
		}

		err = LogStrategyAlert(conn, strategy.UserID, strategy.StrategyID, strategy.Name, message, additionalData)
		if err != nil {
			log.Printf("Warning: failed to log strategy alert for strategy %d: %v", strategy.StrategyID, err)
			// Don't fail the entire alert processing if logging fails
		}
	}

	return nil
}
