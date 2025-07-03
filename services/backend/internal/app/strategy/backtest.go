package strategy

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// BacktestArgs represents arguments for backtesting (API compatibility)
type BacktestArgs struct {
	StrategyID int   `json:"strategyId"`
	Securities []int `json:"securities"`
	Start      int64 `json:"start"`

	FullResults bool `json:"fullResults"`
}

// BacktestResult represents a single backtest instance (API compatibility)
type BacktestResult struct {
	Ticker          string             `json:"ticker"`
	SecurityID      int                `json:"securityId"`
	Timestamp       int64              `json:"timestamp"`
	Open            float64            `json:"open"`
	High            float64            `json:"high"`
	Low             float64            `json:"low"`
	Close           float64            `json:"close"`
	Volume          int64              `json:"volume"`
	Classification  bool               `json:"classification"`
	FutureReturns   map[string]float64 `json:"futureReturns,omitempty"`
	StrategyResults map[string]any     `json:"strategyResults,omitempty"`
	Instance        map[string]any     `json:"instance,omitempty"`
}

// BacktestSummary contains summary statistics of the backtest (API compatibility)
type BacktestSummary struct {
	TotalInstances    int      `json:"totalInstances"`
	PositiveInstances int      `json:"positiveInstances"`
	DateRange         []string `json:"dateRange"`
	SymbolsProcessed  int      `json:"symbolsProcessed"`
	Columns           []string `json:"columns"`
}

// BacktestResponse represents the complete backtest response (API compatibility)
type BacktestResponse struct {
	Instances      []BacktestResult `json:"instances"`
	Summary        BacktestSummary  `json:"summary"`
	StrategyPrints string           `json:"strategyPrints,omitempty"`
}

// WorkerBacktestResult represents the result from the worker's run_backtest function
type WorkerBacktestResult struct {
	Success            bool                   `json:"success"`
	StrategyID         int                    `json:"strategy_id"`
	ExecutionMode      string                 `json:"execution_mode"`
	Instances          []map[string]any       `json:"instances"`
	Summary            WorkerSummary          `json:"summary"`
	PerformanceMetrics map[string]interface{} `json:"performance_metrics"`
	ExecutionTimeMs    int                    `json:"execution_time_ms"`
	StrategyPrints     string                 `json:"strategy_prints,omitempty"`
	ErrorMessage       string                 `json:"error_message,omitempty"`
}

// WorkerSummary represents worker summary statistics
type WorkerSummary struct {
	TotalInstances            int            `json:"total_instances"`
	PositiveInstances         int            `json:"positive_instances"`
	DateRange                 DateRangeField `json:"date_range"`
	SymbolsProcessed          int            `json:"symbols_processed"`
	ExecutionType             string         `json:"execution_type,omitempty"`
	SuccessfulClassifications int            `json:"successful_classifications,omitempty"`
}

// DateRangeField can handle both string and []string from JSON
type DateRangeField []string

func (d *DateRangeField) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as []string first
	var stringSlice []string
	if err := json.Unmarshal(data, &stringSlice); err == nil {
		*d = DateRangeField(stringSlice)
		return nil
	}

	// If that fails, try as string and convert to slice
	var singleString string
	if err := json.Unmarshal(data, &singleString); err == nil {
		*d = DateRangeField([]string{singleString})
		return nil
	}

	return fmt.Errorf("date_range must be either string or []string")
}

// RunBacktest executes a complete strategy backtest using the new worker architecture
func RunBacktest(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (any, error) {
	return RunBacktestWithProgress(ctx, conn, userID, rawArgs, nil)
}

// RunBacktestWithProgress executes a complete strategy backtest with optional progress callbacks
func RunBacktestWithProgress(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage, progressCallback ProgressCallback) (any, error) {
	var args BacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	log.Printf("Starting complete backtest for strategy %d using new worker architecture", args.StrategyID)

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

	// Call the worker's run_backtest function
	result, err := callWorkerBacktestWithProgress(ctx, conn, args.StrategyID, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("error executing worker backtest: %v", err)
	}

	// Convert worker result to BacktestResponse format for API compatibility
	instances := convertWorkerInstancesToBacktestResults(result.Instances)
	summary := convertWorkerSummaryToBacktestSummary(result.Summary)

	response := BacktestResponse{
		Instances:      instances,
		Summary:        summary,
		StrategyPrints: result.StrategyPrints,
	}

	// Cache the results
	if err := SaveBacktestToCache(ctx, conn, userID, args.StrategyID, response); err != nil {
		log.Printf("Warning: Failed to cache backtest results: %v", err)
		// Don't return error, just log warning
	}

	log.Printf("Complete backtest finished for strategy %d: %d instances found",
		args.StrategyID, len(instances))

	return response, nil
}

// callWorkerBacktestWithProgress calls the worker's run_backtest function via Redis queue with progress callbacks
func callWorkerBacktestWithProgress(ctx context.Context, conn *data.Conn, strategyID int, progressCallback ProgressCallback) (*WorkerBacktestResult, error) {
	// Generate unique task ID
	taskID := fmt.Sprintf("backtest_%d_%d", strategyID, time.Now().UnixNano())

	// Prepare backtest task payload
	task := map[string]interface{}{
		"task_id":   taskID,
		"task_type": "backtest",
		"args": map[string]interface{}{
			"strategy_id": fmt.Sprintf("%d", strategyID),
		},
		"created_at": time.Now().UTC().Format(time.RFC3339),
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

	// Wait for result with timeout and progress callbacks
	result, err := waitForBacktestResultWithProgress(ctx, conn, taskID, 5*time.Minute, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("error waiting for backtest result: %v", err)
	}

	return result, nil
}

// ProgressCallback is a function type for sending progress updates during backtest execution
type ProgressCallback func(message string)

// waitForBacktestResultWithProgress waits for a backtest result with optional progress callbacks
func waitForBacktestResultWithProgress(ctx context.Context, conn *data.Conn, taskID string, timeout time.Duration, progressCallback ProgressCallback) (*WorkerBacktestResult, error) {
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
			return nil, fmt.Errorf("timeout waiting for backtest result")
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
				if status == "progress" {
					// Handle progress updates
					stage, _ := taskUpdate["stage"].(string)
					message, _ := taskUpdate["message"].(string)
					log.Printf("Backtest progress [%s]: %s", stage, message)

					// Call progress callback if provided
					if progressCallback != nil {
						progressCallback(message)
					}
					continue
				}
				if status == "completed" || status == "failed" {
					// Convert task result to WorkerBacktestResult
					var result WorkerBacktestResult
					if resultData, exists := taskUpdate["result"]; exists {
						resultJSON, err := json.Marshal(resultData)
						if err != nil {
							return nil, fmt.Errorf("error marshaling task result: %v", err)
						}

						err = json.Unmarshal(resultJSON, &result)
						if err != nil {
							return nil, fmt.Errorf("error unmarshaling backtest result: %v", err)
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

// convertWorkerInstancesToBacktestResults converts worker instances to API format
func convertWorkerInstancesToBacktestResults(instances []map[string]any) []BacktestResult {
	results := make([]BacktestResult, len(instances))

	for i, instance := range instances {
		// Extract ticker (required field)
		ticker, _ := instance["ticker"].(string)

		// Extract timestamp and convert properly - Python worker returns Unix timestamps in seconds
		var timestamp int64
		if ts, ok := instance["timestamp"]; ok {
			switch v := ts.(type) {
			case int64:
				timestamp = v
			case float64:
				timestamp = int64(v)
			case int:
				timestamp = int64(v)
			}
		}

		// Convert from seconds to milliseconds if needed (for JavaScript Date compatibility)
		if timestamp > 0 && timestamp < 4000000000 { // Less than year 2096 in seconds
			timestamp = timestamp * 1000
		}

		results[i] = BacktestResult{
			Ticker:         ticker,
			SecurityID:     0, // Will be populated if needed
			Timestamp:      timestamp,
			Classification: true,     // Since instance was returned, it met criteria
			Instance:       instance, // Include the complete original instance
		}
	}

	return results
}

// convertWorkerSummaryToBacktestSummary converts worker summary to API format
func convertWorkerSummaryToBacktestSummary(summary WorkerSummary) BacktestSummary {
	return BacktestSummary{
		TotalInstances:    summary.TotalInstances,
		PositiveInstances: summary.PositiveInstances,
		DateRange:         []string(summary.DateRange),
		SymbolsProcessed:  summary.SymbolsProcessed,
		Columns:           []string{"ticker", "timestamp", "classification"},
	}
}
