package strategy

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Strategy represents a simplified strategy with natural language description and generated Python code
type Strategy struct {
	StrategyID    int    `json:"strategyId"`
	UserID        int    `json:"userId"`
	Name          string `json:"name"`
	Description   string `json:"description"` // Human-readable description
	Prompt        string `json:"prompt"`      // Original user prompt
	PythonCode    string `json:"pythonCode"`  // Generated Python classifier
	Score         int    `json:"score,omitempty"`
	Version       string `json:"version,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
	IsAlertActive bool   `json:"isAlertActive,omitempty"`
}

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
func RunScreening(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
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

	// Call the worker's run_screener function
	result, err := callWorkerScreening(context.Background(), conn, args.StrategyID, args.Universe, args.Limit)
	if err != nil {
		return nil, fmt.Errorf("error executing worker screening: %v", err)
	}

	// Convert worker result to ScreeningResponse format for API compatibility
	rankedResults := convertWorkerRankedResults(result.RankedResults)

	response := ScreeningResponse{
		RankedResults: rankedResults,
		Scores:        result.Scores,
		UniverseSize:  result.UniverseSize,
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
func callWorkerScreening(ctx context.Context, conn *data.Conn, strategyID int, universe []string, limit int) (*WorkerScreeningResult, error) {
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
	defer pubsub.Close()

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
}

type CreateStrategyFromPromptResult struct {
	StrategyID int    `json:"strategyId"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}

func CreateStrategyFromPrompt(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
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
	result, err := callWorkerCreateStrategy(context.Background(), conn, userID, args.Query, args.StrategyID)
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

	return CreateStrategyFromPromptResult{
		StrategyID: result.Strategy.StrategyID,
		Name:       result.Strategy.Name,
		Version:    result.Strategy.Version,
	}, nil
}

// WorkerCreateStrategyResult represents the result from the Python worker
type WorkerCreateStrategyResult struct {
	Success  bool      `json:"success"`
	Strategy *Strategy `json:"strategy,omitempty"`
	Error    string    `json:"error,omitempty"`
	TaskID   string    `json:"task_id,omitempty"`
}

// callWorkerCreateStrategy calls the worker's create_strategy function via Redis queue
func callWorkerCreateStrategy(ctx context.Context, conn *data.Conn, userID int, prompt string, strategyID int) (*WorkerCreateStrategyResult, error) {
	// Generate unique task ID
	taskID := fmt.Sprintf("create_strategy_%d_%d", userID, time.Now().UnixNano())

	// Prepare strategy creation task payload
	task := map[string]interface{}{
		"task_id":   taskID,
		"task_type": "create_strategy",
		"args": map[string]interface{}{
			"user_id":     userID,
			"prompt":      prompt,
			"strategy_id": strategyID,
		},
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"priority":   "high", // Mark strategy creation as high priority
	}

	// Submit task to Redis queue
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("error marshaling task: %v", err)
	}

	// Push task to PRIORITY worker queue for strategy creation/editing
	err = conn.Cache.RPush(ctx, "strategy_queue_priority", string(taskJSON)).Err()
	if err != nil {
		return nil, fmt.Errorf("error submitting task to priority queue: %v", err)
	}

	log.Printf("Submitted strategy creation task %s to PRIORITY worker queue", taskID)

	// Wait for result with timeout
	result, err := waitForCreateStrategyResult(ctx, conn, taskID, 3*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("error waiting for strategy creation result: %v", err)
	}

	return result, nil
}

// waitForCreateStrategyResult waits for a strategy creation result via Redis pubsub
func waitForCreateStrategyResult(ctx context.Context, conn *data.Conn, taskID string, timeout time.Duration) (*WorkerCreateStrategyResult, error) {
	// Subscribe to task updates
	pubsub := conn.Cache.Subscribe(ctx, "worker_task_updates")
	defer pubsub.Close()

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := pubsub.Channel()

	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timeout waiting for strategy creation result")
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
				if status == "completed" || status == "error" {
					// Convert task result to WorkerCreateStrategyResult
					var result WorkerCreateStrategyResult
					result.TaskID = taskID

					if status == "error" {
						result.Success = false
						if errorMsg, exists := taskUpdate["error_message"]; exists {
							result.Error = fmt.Sprintf("%v", errorMsg)
						} else if resultData, exists := taskUpdate["result"]; exists {
							if resultMap, ok := resultData.(map[string]interface{}); ok {
								if errorMsg, exists := resultMap["error"]; exists {
									result.Error = fmt.Sprintf("%v", errorMsg)
								}
							}
						}
					} else {
						// Parse successful result
						if resultData, exists := taskUpdate["result"]; exists {
							resultJSON, err := json.Marshal(resultData)
							if err != nil {
								return nil, fmt.Errorf("error marshaling task result: %v", err)
							}

							var workerResult map[string]interface{}
							err = json.Unmarshal(resultJSON, &workerResult)
							if err != nil {
								return nil, fmt.Errorf("error unmarshaling worker result: %v", err)
							}

							if success, exists := workerResult["success"]; exists {
								result.Success = success.(bool)
							}

							if result.Success {
								if strategyData, exists := workerResult["strategy"]; exists {
									// Convert strategy data to Strategy struct
									strategyJSON, err := json.Marshal(strategyData)
									if err != nil {
										return nil, fmt.Errorf("error marshaling strategy data: %v", err)
									}

									var strategy Strategy
									err = json.Unmarshal(strategyJSON, &strategy)
									if err != nil {
										return nil, fmt.Errorf("error unmarshaling strategy: %v", err)
									}

									result.Strategy = &strategy
								}
							} else {
								if errorMsg, exists := workerResult["error"]; exists {
									result.Error = fmt.Sprintf("%v", errorMsg)
								}
							}
						}
					}

					return &result, nil
				}
			}
		}
	}
}

func GetStrategies(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT strategyid, name, 
		       COALESCE(description, '') as description,
		       COALESCE(prompt, '') as prompt,
		       COALESCE(pythoncode, '') as pythoncode,
		       COALESCE(score, 0) as score,
		       COALESCE(version, '1.0') as version,
		       COALESCE(createdat, NOW()) as createdat,
		       COALESCE(isalertactive, false) as isalertactive
		FROM strategies WHERE userid = $1 ORDER BY createdat DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []Strategy
	for rows.Next() {
		var strategy Strategy
		var createdAt time.Time

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
		); err != nil {
			return nil, fmt.Errorf("error scanning strategy: %v", err)
		}

		strategy.UserID = userID
		strategy.CreatedAt = createdAt.Format(time.RFC3339)
		strategies = append(strategies, strategy)
	}

	return strategies, nil
}

type SetAlertArgs struct {
	StrategyID int  `json:"strategyId"`
	Active     bool `json:"active"`
}

func SetAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetAlertArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	_, err := conn.DB.Exec(context.Background(), `
		UPDATE strategies 
		SET isalertactive = $1 
		WHERE strategyid = $2 AND userid = $3`,
		args.Active, args.StrategyID, userID)

	if err != nil {
		return nil, fmt.Errorf("error updating alert status: %v", err)
	}

	log.Printf("Strategy %d alert status updated to: %v", args.StrategyID, args.Active)

	return map[string]interface{}{
		"success":     true,
		"strategyId":  args.StrategyID,
		"alertActive": args.Active,
	}, nil
}

type DeleteStrategyArgs struct {
	StrategyID int `json:"strategyId"`
}

func DeleteStrategy(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
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

	return map[string]interface{}{"success": true}, nil
}
