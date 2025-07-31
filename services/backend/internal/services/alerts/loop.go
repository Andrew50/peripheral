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
	"sync"
	"time"
)

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
	a.wg.Add(2)
	log.Printf("🚀 Starting price alert loop")
	go a.priceAlertLoop()
	go a.strategyAlertLoop()

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
	// Legacy global variables for backward compatibility - these will be removed in future versions
	priceAlerts    sync.Map // DEPRECATED: use service instance instead
	strategyAlerts sync.Map // DEPRECATED: use service instance instead
	//ctx            context.Context    // DEPRECATED: use service instance instead
	//cancel         context.CancelFunc // DEPRECATED: use service instance instead
	//mu             sync.Mutex         // DEPRECATED: use service instance instead
)

// Legacy wrapper functions for backward compatibility
// StartAlertLoop starts the alert service (wrapper around service-based approach)
func StartAlertLoop(conn *data.Conn) error { //entrypoint
	log.Printf("🚀 StartAlertLoop called (using service-based approach)")
	service := GetAlertService()
	return service.Start(conn)
}

// StopAlertLoop stops the alert service (wrapper around service-based approach)
func StopAlertLoop() {
	log.Printf("🛑 StopAlertLoop called (using service-based approach)")
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

	var wg sync.WaitGroup
	var processed, succeeded, failed int
	var mu sync.Mutex

	a.strategyAlerts.Range(func(_, value interface{}) bool {
		alert := value.(StrategyAlert)
		wg.Add(1)
		go func(alert StrategyAlert) {
			defer wg.Done()
			log.Printf("Processing strategy alert %d: %s (threshold: %.2f)", alert.StrategyID, alert.Name, alert.Threshold)
			if err := executeStrategyAlert(context.Background(), a.conn, alert); err != nil {
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
	log.Printf("Strategy alert processing summary: %d total, %d succeeded, %d failed", processed, succeeded, failed)
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
		       COALESCE(alert_universe, ARRAY[]::TEXT[]) as alert_universe
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

		a.strategyAlerts.Store(alert.StrategyID, alert)

		// Also store in legacy global map for backward compatibility
		strategyAlerts.Store(alert.StrategyID, alert)
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
}

// executeStrategyAlert submits a strategy alert task and waits for results
func executeStrategyAlert(ctx context.Context, conn *data.Conn, strategy StrategyAlert) error {
	// Generate unique task ID
	taskID := fmt.Sprintf("strategy_alert_%d_%d", strategy.StrategyID, time.Now().UnixNano())
	log.Printf("Executing strategy alert %d (task: %s)", strategy.StrategyID, taskID)

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
			message = fmt.Sprintf("Strategy '%s' executed successfully", strategy.Name)
			log.Printf("Strategy alert %d executed successfully with no matches", strategy.StrategyID)
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
	} else {
		log.Printf("Strategy alert %d execution completed but marked as not successful", strategy.StrategyID)
	}

	return nil
}*/
// Legacy functions for backward compatibility - these will be removed in future versions
/*func priceAlertLoop(ctx context.Context, conn *data.Conn) {
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
}*/

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
func executeStrategyAlert(ctx context.Context, conn *data.Conn, strategy StrategyAlert) error {
	// Prepare arguments expected by the Python worker (see services/worker/src/alert.py)
	args := map[string]interface{}{
		"strategy_id": strategy.StrategyID,
		"user_id":     strategy.UserID,
	}

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

	log.Printf("🚀 Strategy %d (%s): queuing alert task with args: %+v", strategy.StrategyID, strategy.Name, args)
	// Submit the alert task through the unified queue system and wait for the typed result.
	result, err := queue.QueueAlertTyped(ctx, conn, args)
	if err != nil {
		log.Printf("❌ Strategy %d (%s): queue submission failed: %v", strategy.StrategyID, strategy.Name, err)
		return fmt.Errorf("queue alert error: %w", err)
	}

	log.Printf("📥 Strategy %d (%s): received result - Success: %t, Instances: %d", strategy.StrategyID, strategy.Name, result.Success, len(result.Instances))

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
