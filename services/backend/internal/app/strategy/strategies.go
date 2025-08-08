package strategy

import (
	"backend/internal/data"
	"backend/internal/queue"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"backend/internal/app/limits"
)

// CreateStrategyFromPromptArgs contains the user's natural language prompt
type CreateStrategyFromPromptArgs struct {
	Query      string `json:"query"`      // Changed from Prompt to Query to match tool args
	StrategyID int    `json:"strategyId"` // Added StrategyID field to match tool args
}

// ScreeningArgs contains arguments for strategy screening
type ScreeningArgs struct {
	StrategyID int      `json:"strategyId"`
	Universe   []string `json:"universe,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

// ScreeningResponse represents the screening results
type ScreeningResponse struct {
	RankedResults []ScreeningResult  `json:"rankedResults"`
	Scores        map[string]float64 `json:"scores"`
	UniverseSize  int                `json:"universeSize"`
}

type ScreeningResult struct {
	Symbol       string                 `json:"symbol"`
	Score        float64                `json:"score"`
	CurrentPrice float64                `json:"currentPrice,omitempty"`
	Sector       string                 `json:"sector,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
}

// AlertArgs contains arguments for strategy alerts

// RunScreening executes a complete strategy screening using the new worker architecture
func RunScreening(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args ScreeningArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	log.Printf("Starting complete screening for strategy %d using new worker architecture", args.StrategyID)

	// Verify strategy exists and user has permission
	var strategyExists bool
	err := conn.DB.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM strategies WHERE strategyid = $1 AND userid = $2)`,
		args.StrategyID, userID).Scan(&strategyExists)
	if err != nil {
		return nil, fmt.Errorf("error checking strategy: %v", err)
	}
	if !strategyExists {
		return nil, fmt.Errorf("strategy not found or access denied")
	}

	// Build arguments for the new typed-queue screening task
	qArgs := map[string]interface{}{
		"user_id":      userID,
		"strategy_ids": []string{fmt.Sprintf("%d", args.StrategyID)},
	}
	if len(args.Universe) > 0 {
		qArgs["universe"] = args.Universe
	}
	/*if args.Limit > 0 {
		qArgs["limit"] = args.Limit
	}*/

	// Submit task via the unified queue system
	qResult, err := queue.ScreeningTyped(ctx, conn, qArgs)
	if err != nil {
		return nil, fmt.Errorf("error executing worker screening: %v", err)
	}

	// Convert instances returned by the worker to API compatible structure
	rankedResults := convertScreeningInstances(qResult.Instances)

	response := ScreeningResponse{
		RankedResults: rankedResults,
		Scores:        nil, // Worker currently doesn't supply aggregated scores
		UniverseSize:  len(rankedResults),
	}

	log.Printf("Complete screening finished for strategy %d: %d opportunities found",
		args.StrategyID, len(rankedResults))

	return response, nil
}

// Worker screening types and functions
type WorkerScreeningResult struct {
	Success         bool                 `json:"success"`
	StrategyID      int                  `json:"strategy_id"`
	ExecutionMode   string               `json:"execution_mode"`
	RankedResults   []WorkerRankedResult `json:"ranked_results"`
	Scores          map[string]float64   `json:"scores"`
	UniverseSize    int                  `json:"universe_size"`
	ExecutionTimeMs int                  `json:"execution_time_ms"`
	ErrorMessage    string               `json:"error_message,omitempty"`
}

type WorkerRankedResult struct {
	Symbol       string                 `json:"symbol"`
	Score        float64                `json:"score"`
	CurrentPrice float64                `json:"current_price,omitempty"`
	Sector       string                 `json:"sector,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
}

// callWorkerScreening calls the worker's run_screener function via Redis queue
/*func callWorkerScreening(ctx context.Context, conn *data.Conn, strategyID int, universe []string, limit int) (*WorkerScreeningResult, error) {
	// Set defaults
	if limit == 0 {
		limit = 100
	}

	// Generate unique task ID
	taskID := fmt.Sprintf("screening_%d_%d", strategyID, time.Now().UnixNano())

	// Prepare screening task payload - worker expects strategy_ids as array
	task := map[string]interface{}{
		"task_id":   taskID,
		"task_type": "screening",
		"args": map[string]interface{}{
			"strategy_ids": []string{fmt.Sprintf("%d", strategyID)},
		},
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}

	// Add optional parameters
	if len(universe) > 0 {
		task["args"].(map[string]interface{})["universe"] = universe
	}
	if limit > 0 {
		task["args"].(map[string]interface{})["limit"] = limit
	}

	// Submit task to Redis queue
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("error marshaling task: %v", err)
	}

	// Push task to worker queue
	err = conn.Cache.RPush(ctx, "strategy_queue", string(taskJSON)).Err()
	if err != nil {
		return nil, fmt.Errorf("error submitting task to queue: %v", err)
	}

	// Wait for result with timeout
	result, err := waitForScreeningResult(ctx, conn, taskID, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("error waiting for screening result: %v", err)
	}

	return result, nil
}

// waitForScreeningResult waits for a screening result via Redis pubsub
func waitForScreeningResult(ctx context.Context, conn *data.Conn, taskID string, timeout time.Duration) (*WorkerScreeningResult, error) {
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
			return nil, fmt.Errorf("timeout waiting for screening result")
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
					// Convert task result to WorkerScreeningResult
					var result WorkerScreeningResult
					if resultData, exists := taskUpdate["result"]; exists {
						resultJSON, err := json.Marshal(resultData)
						if err != nil {
							return nil, fmt.Errorf("error marshaling task result: %v", err)
						}

						err = json.Unmarshal(resultJSON, &result)
						if err != nil {
							return nil, fmt.Errorf("error unmarshaling screening result: %v", err)
						}
					}

					if status == "failed" {
						errorMsg, _ := taskUpdate["error_message"].(string)
						result.Success = false
						result.ErrorMessage = errorMsg
					} else {
						result.Success = true
					}

					return &result, nil
				}
			}
		}
	}
}

// Conversion functions
func convertWorkerRankedResults(workerResults []WorkerRankedResult) []ScreeningResult {
	results := make([]ScreeningResult, len(workerResults))

	for i, wr := range workerResults {
		results[i] = ScreeningResult(wr)
	}

	return results
}*/

func convertScreeningInstances(instances []map[string]interface{}) []ScreeningResult {
	results := make([]ScreeningResult, len(instances))
	for i, inst := range instances {
		sr := ScreeningResult{
			Data: inst,
		}
		if v, ok := inst["symbol"].(string); ok {
			sr.Symbol = v
		}
		if v, ok := inst["score"].(float64); ok {
			sr.Score = v
		}
		if v, ok := inst["current_price"].(float64); ok {
			sr.CurrentPrice = v
		}
		if v, ok := inst["sector"].(string); ok {
			sr.Sector = v
		}
		results[i] = sr
	}
	return results
}

type CreateStrategyFromPromptResult struct {
	StrategyID int    `json:"strategyId"`
	Name       string `json:"name"`
	Version    int    `json:"version"`
}

func AgentCreateStrategyFromPrompt(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := CreateStrategyFromPrompt(ctx, conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}
	/*go func() {
		strategyData := map[string]interface{}{
			"strategyId": res.StrategyID,
			"name":       res.Name,
			"version":    res.Version,
		}
		socket.SendStrategyUpdate(userID, "add", strategyData)
	}()*/
	return res, nil
}
func CreateStrategyFromPrompt(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	log.Printf("=== STRATEGY CREATION START (WORKER QUEUE) ===")
	log.Printf("UserID: %d", userID)
	log.Printf("Raw args: %s", string(rawArgs))

	var args CreateStrategyFromPromptArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		log.Printf("ERROR: Failed to unmarshal args: %v", err)
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	log.Printf("Parsed args - Query: %q, StrategyID: %d", args.Query, args.StrategyID)
	log.Printf("Delegating strategy creation to Python worker...")

	// Call the worker to create the strategy
	result, err := callWorkerCreateStrategy(ctx, conn, userID, args.Query, args.StrategyID)
	if err != nil {
		log.Printf("ERROR: Worker strategy creation failed: %v", err)
		return nil, fmt.Errorf("strategy creation failed: %v", err)
	}

	log.Printf("=== STRATEGY CREATION COMPLETED ===")
	log.Printf("Success: %t", result.Success)
	if result.Success && result.Strategy != nil {
		log.Printf("Created strategy ID: %d", result.Strategy.StrategyID)
		log.Printf("Strategy name: %s", result.Strategy.Name)
	}

	// Check if the strategy creation was successful
	if !result.Success {
		log.Printf("ERROR: Strategy creation failed: %s", result.Error)
		return nil, fmt.Errorf("strategy creation failed: %s", result.Error)
	}

	// Check if strategy object is nil
	if result.Strategy == nil {
		log.Printf("ERROR: Strategy creation succeeded but strategy object is nil")
		return nil, fmt.Errorf("strategy creation succeeded but strategy object is nil")
	}

	// Sync strategy universe to Redis for per-ticker alert processing
	// This happens after the strategy is created to ensure the universe is available
	if err := syncStrategyUniverseToRedis(conn, result.Strategy.StrategyID); err != nil {
		log.Printf("⚠️ Failed to sync strategy %d universe to Redis: %v", result.Strategy.StrategyID, err)
		// Don't fail the operation for Redis sync errors, just log them
	}

	return CreateStrategyFromPromptResult{
		StrategyID: result.Strategy.StrategyID,
		Name:       result.Strategy.Name,
		Version:    result.Strategy.Version,
	}, nil
}

// callWorkerCreateStrategy calls the worker's create_strategy function via the new queue system
func callWorkerCreateStrategy(ctx context.Context, conn *data.Conn, userID int, prompt string, strategyID int) (*queue.CreateStrategyResult, error) {
	messageID, ok := ctx.Value("messageID").(string)
	if !ok {
		messageID = ""
	}
	conversationID, ok := ctx.Value("conversationID").(string)
	if !ok {
		conversationID = ""
	}

	// Prepare strategy creation task arguments
	args := map[string]interface{}{
		"user_id":         userID,
		"prompt":          prompt,
		"strategy_id":     strategyID,
		"conversation_id": conversationID,
		"message_id":      messageID,
	}

	// Queue the task using the new typed queue system and return result directly
	result, err := queue.CreateStrategyTyped(ctx, conn, args)
	if err != nil {
		return nil, fmt.Errorf("error queuing strategy creation task: %v", err)
	}

	log.Printf("Submitted strategy creation task to PRIORITY worker queue via new queue system")

	return result, nil
}

func GetStrategies(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT strategyid, name, 
		       COALESCE(description, '') as description,
		       COALESCE(prompt, '') as prompt,
		       COALESCE(pythoncode, '') as pythoncode,
		       COALESCE(score, 0) as score,
		       COALESCE(version, 1) as version,
		       COALESCE(createdat, NOW()) as createdat,
		       alertactive as alertactive,
		       alert_threshold,
		       alert_universe,
		       COALESCE(min_timeframe, '') as min_timeframe,
		       alert_last_trigger_at
		FROM strategies WHERE userid = $1 ORDER BY createdat DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []queue.Strategy
	for rows.Next() {
		var strategy queue.Strategy
		var createdAt time.Time
		var alertLastTriggerAt *time.Time

		if err := rows.Scan(
			&strategy.StrategyID,
			&strategy.Name,
			&strategy.Description,
			&strategy.Prompt,
			&strategy.PythonCode,
			&strategy.Score,
			&strategy.Version,
			&createdAt,
			&strategy.IsAlertActive,
			&strategy.AlertThreshold,
			&strategy.AlertUniverse,
			&strategy.MinTimeframe,
			&alertLastTriggerAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning strategy: %v", err)
		}

		strategy.UserID = userID
		strategy.CreatedAt = createdAt.Format(time.RFC3339)

		// Convert alert_last_trigger_at to string if not null
		if alertLastTriggerAt != nil {
			triggerTime := alertLastTriggerAt.Format(time.RFC3339)
			strategy.AlertLastTriggerAt = &triggerTime
		}

		strategies = append(strategies, strategy)
	}

	return strategies, nil
}

type SetAlertArgs struct {
	StrategyID int      `json:"strategyId"`
	Active     bool     `json:"active"`
	Threshold  *float64 `json:"threshold,omitempty"`
	Universe   []string `json:"universe,omitempty"`
}

func SetAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetAlertArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Get current alert status and configuration before doing anything
	var currentActive bool
	var currentThreshold *float64
	var currentUniverse []string
	err := conn.DB.QueryRow(context.Background(), `
		SELECT COALESCE(alertactive, false), alert_threshold, alert_universe
		FROM strategies 
		WHERE strategyid = $1 AND userid = $2`,
		args.StrategyID, userID).Scan(&currentActive, &currentThreshold, &currentUniverse)
	if err != nil {
		return nil, fmt.Errorf("error checking current alert status: %v", err)
	}

	// If enabling the alert, check if user can create more strategy alerts
	if args.Active && !currentActive {
		allowed, remaining, err := limits.CheckUsageAllowed(conn, userID, limits.UsageTypeStrategyAlert, 0)
		if err != nil {
			return nil, fmt.Errorf("checking strategy alert limits: %w", err)
		}
		if !allowed {
			return nil, fmt.Errorf("strategy alert limit reached - you have %d strategy alerts remaining", remaining)
		}
	}

	// Update the alert status and configuration
	_, err = conn.DB.Exec(context.Background(), `
		UPDATE strategies 
		SET alertactive = $1, alert_threshold = $2, alert_universe = $3
		WHERE strategyid = $4 AND userid = $5`,
		args.Active, args.Threshold, args.Universe, args.StrategyID, userID)

	if err != nil {
		return nil, fmt.Errorf("error updating alert configuration: %v", err)
	}

	// Update the strategy alert counter based on the change
	if args.Active && !currentActive {
		// Enabling alert - increment counter
		if err := limits.RecordUsage(conn, userID, limits.UsageTypeStrategyAlert, 1, map[string]interface{}{
			"strategyId": args.StrategyID,
			"action":     "enabled",
		}); err != nil {
			// If we can't record usage, rollback the alert activation
			if _, rollbackErr := conn.DB.Exec(context.Background(), `
				UPDATE strategies 
				SET alertactive = false, alert_threshold = $1, alert_universe = $2
				WHERE strategyid = $3 AND userid = $4`,
				currentThreshold, currentUniverse, args.StrategyID, userID); rollbackErr != nil {
				log.Printf("Warning: failed to rollback strategy alert activation: %v", rollbackErr)
			}
			return nil, fmt.Errorf("recording strategy alert usage: %w", err)
		}
	} else if !args.Active && currentActive {
		// Disabling alert - decrement counter
		if err := limits.DecrementActiveStrategyAlerts(conn, userID, 1); err != nil {
			// Log the error but don't fail the operation since the alert is already disabled
			log.Printf("Warning: failed to decrement active strategy alerts counter for user %d: %v", userID, err)
		}
	}

	log.Printf("Strategy %d alert configuration updated - active: %v, threshold: %v, universe: %v",
		args.StrategyID, args.Active, args.Threshold, args.Universe)

	// Sync strategy universe to Redis for per-ticker alert processing
	// This happens after the database update to ensure consistency
	if err := syncStrategyUniverseToRedis(conn, args.StrategyID); err != nil {
		log.Printf("⚠️ Failed to sync strategy %d universe to Redis: %v", args.StrategyID, err)
		// Don't fail the operation for Redis sync errors, just log them
	}

	return map[string]interface{}{
		"success":        true,
		"strategyId":     args.StrategyID,
		"alertActive":    args.Active,
		"alertThreshold": args.Threshold,
		"alertUniverse":  args.Universe,
	}, nil
}

// syncStrategyUniverseToRedis syncs a strategy's universe from the database to Redis
func syncStrategyUniverseToRedis(conn *data.Conn, strategyID int) error {
	ctx := context.Background()

	// Query the strategy's alert_universe_full from the database
	var alertUniverseFull []string
	query := `SELECT COALESCE(alert_universe_full, ARRAY[]::TEXT[]) FROM strategies WHERE strategyId = $1`
	err := conn.DB.QueryRow(ctx, query, strategyID).Scan(&alertUniverseFull)
	if err != nil {
		return fmt.Errorf("failed to query strategy %d universe: %w", strategyID, err)
	}

	// Only sync to Redis if we have a non-empty universe (global strategies are not stored)
	if len(alertUniverseFull) > 0 {
		if err := data.SetStrategyUniverse(conn, strategyID, alertUniverseFull); err != nil {
			return fmt.Errorf("failed to set strategy %d universe in Redis: %w", strategyID, err)
		}
		log.Printf("📝 Synced strategy %d universe to Redis: %d tickers", strategyID, len(alertUniverseFull))
	} else {
		log.Printf("📝 Strategy %d has global universe, not syncing to Redis", strategyID)
	}

	return nil
}

type DeleteStrategyArgs struct {
	StrategyID int `json:"strategyId"`
}

func DeleteStrategy(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}

	// Check if the strategy has an active alert before deleting
	var isAlertActive bool
	err := conn.DB.QueryRow(context.Background(), `
		SELECT COALESCE(alertactive, false) 
		FROM strategies 
		WHERE strategyid = $1 AND userid = $2`,
		args.StrategyID, userID).Scan(&isAlertActive)
	if err != nil {
		return nil, fmt.Errorf("error checking strategy alert status: %v", err)
	}

	result, err := conn.DB.Exec(context.Background(), `
		DELETE FROM strategies 
		WHERE strategyid = $1 AND userid = $2`, args.StrategyID, userID)

	if err != nil {
		return nil, fmt.Errorf("error deleting strategy: %v", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("strategy not found or you don't have permission to delete it")
	}

	// If the strategy had an active alert, decrement the counter
	if isAlertActive {
		if err := limits.DecrementActiveStrategyAlerts(conn, userID, 1); err != nil {
			// Log the error but don't fail the deletion since the strategy is already removed
			log.Printf("Warning: failed to decrement active strategy alerts counter for user %d: %v", userID, err)
		}
	}

	return map[string]interface{}{"success": true}, nil
}
