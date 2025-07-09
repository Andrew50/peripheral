package strategy

import (
	"backend/internal/app/limits"
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
)

const BacktestCacheKey = "backtest:userID:%d:strategyID:%d"

// RunBacktestArgs represents arguments for backtesting (API compatibility)
type RunBacktestArgs struct {
	StrategyID  int    `json:"strategyId"`
	Securities  []int  `json:"securities"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	FullResults bool   `json:"fullResults"`
}

// BacktestInstanceRow represents a single backtest instance (API compatibility)
type BacktestInstanceRow struct {
	Ticker         string             `json:"ticker"`
	SecurityID     int                `json:"securityId,omitempty"`
	Timestamp      int64              `json:"timestamp"`
	Volume         int64              `json:"volume,omitempty"`
	Classification bool               `json:"classification"`
	FutureReturns  map[string]float64 `json:"futureReturns,omitempty"`
	Instance       map[string]any     `json:"instance,omitempty"`
}

// BacktestSummary contains summary statistics of the backtest (API compatibility)
type BacktestSummary struct {
	TotalInstances    int      `json:"totalInstances"`
	PositiveInstances int      `json:"positiveInstances,omitempty"`
	DateRange         []string `json:"dateRange"`
	SymbolsProcessed  int      `json:"symbolsProcessed"`
	Columns           []string `json:"columns"`
}

// BacktestResponse represents the complete backtest response (API compatibility)
type BacktestResponse struct {
	Instances      []BacktestInstanceRow `json:"instances,omitempty"`
	Summary        BacktestSummary       `json:"summary"`
	StrategyPrints string                `json:"strategyPrints,omitempty"`
	StrategyPlots  []StrategyPlot        `json:"strategyPlots,omitempty"`
}

// StrategyPlot represents a captured plotly plot (lightweight version for API response)
type StrategyPlot struct {
	Data      []map[string]any `json:"data,omitempty"`      // traces of data
	PlotID    int              `json:"plotID"`              // Plot ID for the chart
	ChartType string           `json:"chartType,omitempty"` // "line", "bar", "scatter", "histogram", "heatmap"
	Length    int              `json:"length,omitempty"`    // Length of the data array
	Title     string           `json:"title,omitempty"`     // Chart title
	Layout    map[string]any   `json:"layout,omitempty"`    // Minimal layout (axis labels, dimensions)
}

// StrategyPlotData represents the full plotly data for caching
type StrategyPlotData struct {
	PlotID int            `json:"plotID"`
	Data   map[string]any `json:"data"` // Full plotly figure object
}

// WorkerBacktestResult represents the result from the worker's run_backtest function
type WorkerBacktestResult struct {
	Success         bool               `json:"success"`
	StrategyID      int                `json:"strategy_id"`
	ExecutionMode   string             `json:"execution_mode"`
	Instances       []map[string]any   `json:"instances"`
	Summary         WorkerSummary      `json:"summary"`
	ExecutionTimeMs int                `json:"execution_time_ms"`
	StrategyPrints  string             `json:"strategy_prints,omitempty"`
	StrategyPlots   []StrategyPlotData `json:"strategy_plots,omitempty"`
	ErrorMessage    string             `json:"error_message,omitempty"`
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

// ProgressCallback is a function type for sending progress updates during backtest execution
type ProgressCallback func(message string)

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

// checkForActiveBacktests performs a comprehensive check for active backtests for a specific user
func checkForActiveBacktests(ctx context.Context, conn *data.Conn, userID int) (bool, error) {
	// Check both queues for waiting backtest tasks
	queues := []string{"strategy_queue", "strategy_queue_priority"}

	for _, queueName := range queues {
		queueLen, err := conn.Cache.LLen(ctx, queueName).Result()
		if err != nil {
			return false, fmt.Errorf("error checking queue %s: %v", queueName, err)
		}

		if queueLen > 0 {
			// Read all items in the queue
			queueItems, err := conn.Cache.LRange(ctx, queueName, 0, queueLen-1).Result()
			if err != nil {
				return false, fmt.Errorf("error reading queue %s: %v", queueName, err)
			}

			// Check for backtest tasks for this user
			for _, item := range queueItems {
				var queuedTask map[string]interface{}
				if err := json.Unmarshal([]byte(item), &queuedTask); err != nil {
					continue // skip malformed
				}

				if queuedTask["task_type"] == "backtest" {
					if args, ok := queuedTask["args"].(map[string]interface{}); ok {
						if userIDStr, ok := args["user_id"].(string); ok && userIDStr != "" {
							// Check if this backtest belongs to the current user
							queuedUserID, err := strconv.Atoi(userIDStr)
							if err != nil {
								continue
							}

							if queuedUserID == userID {
								return true, nil // Found active backtest for this user
							}
						}
					}
				}
			}
		}
	}

	// Check task assignments for currently running backtest tasks
	assignmentKeys, err := conn.Cache.Keys(ctx, "task_assignment:*").Result()
	if err != nil {
		return false, fmt.Errorf("error getting task assignment keys: %v", err)
	}

	for _, assignmentKey := range assignmentKeys {
		assignmentJSON, err := conn.Cache.Get(ctx, assignmentKey).Result()
		if err != nil {
			continue // skip if assignment was deleted
		}

		var assignment map[string]interface{}
		if err := json.Unmarshal([]byte(assignmentJSON), &assignment); err != nil {
			continue // skip malformed
		}

		// Get the task result to check if it's a backtest for this user
		taskID, ok := assignment["task_id"].(string)
		if !ok {
			continue
		}

		resultKey := fmt.Sprintf("task_result:%s", taskID)
		resultJSON, err := conn.Cache.Get(ctx, resultKey).Result()
		if err != nil {
			continue // skip if result doesn't exist
		}

		var taskResult map[string]interface{}
		if err := json.Unmarshal([]byte(resultJSON), &taskResult); err != nil {
			continue // skip malformed
		}

		// Check if this is a backtest task
		data, ok := taskResult["data"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check original task data
		if originalTask, exists := data["original_task"]; exists {
			if taskMap, ok := originalTask.(map[string]interface{}); ok {
				if taskType, ok := taskMap["task_type"].(string); ok && taskType == "backtest" {
					// Check if this backtest belongs to the current user
					if args, ok := taskMap["args"].(map[string]interface{}); ok {
						if userIDStr, ok := args["user_id"].(string); ok && userIDStr != "" {
							assignedUserID, err := strconv.Atoi(userIDStr)
							if err != nil {
								continue
							}

							if assignedUserID == userID {
								return true, nil // Found running backtest for this user
							}
						}
					}
				}
			}
		}
	}

	return false, nil
}

// RunBacktest executes a complete strategy backtest using the new worker architecture
func RunBacktest(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (any, error) {
	return RunBacktestWithProgress(ctx, conn, userID, rawArgs, nil)
}

// RunBacktestWithProgress executes a complete strategy backtest with optional progress callbacks
func RunBacktestWithProgress(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage, progressCallback ProgressCallback) (any, error) {
	var args RunBacktestArgs
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
	result, err := callWorkerBacktestWithProgress(ctx, conn, userID, args, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("error executing worker backtest: %v", err)
	}

	summary := convertWorkerSummaryToBacktestSummary(result.Summary, result.Instances)

	// Extract plot attributes and prepare lightweight plots for API response
	lightweightPlots := make([]StrategyPlot, len(result.StrategyPlots))
	fullPlotData := make([]StrategyPlotData, len(result.StrategyPlots))

	for i, plot := range result.StrategyPlots {
		// Store full data for caching
		fullPlotData[i] = StrategyPlotData{
			PlotID: plot.PlotID,
			Data:   plot.Data,
		}

		// Create lightweight version for API response
		lightweightPlots[i] = StrategyPlot{
			PlotID: plot.PlotID,
		}

		// Extract metadata from the full plot data
		extractPlotAttributes(&lightweightPlots[i], plot.Data)
	}

	responseWithInstances := BacktestResponse{
		Summary:        summary,
		StrategyPrints: result.StrategyPrints,
		StrategyPlots:  lightweightPlots,
		Instances:      convertWorkerInstancesToBacktestResults(result.Instances),
	}
	// Cache the results
	if err := SetBacktestToCache(ctx, conn, userID, args.StrategyID, responseWithInstances); err != nil {
		log.Printf("Warning: Failed to cache backtest results: %v", err)
		// Don't return error, just log warning
	}

	// Log backtest usage for analytics (no credit consumption)
	metadata := map[string]interface{}{
		"strategy_id":       args.StrategyID,
		"instances_found":   len(result.Instances),
		"symbols_processed": responseWithInstances.Summary.SymbolsProcessed,
		"operation_type":    "backtest",
		"credits_consumed":  0, // Explicitly show no credits consumed
	}
	if err := limits.RecordUsage(conn, userID, limits.UsageTypeBacktest, 0, metadata); err != nil {
		log.Printf("Warning: Failed to log backtest usage: %v", err)
		// Don't fail the request since backtest was successful
	}

	// Remove data from plots to save memory
	for i := range responseWithInstances.StrategyPlots {
		responseWithInstances.StrategyPlots[i].Data = []map[string]any{}
	}
	response := &BacktestResponse{
		Summary:        summary,
		StrategyPrints: result.StrategyPrints,
		StrategyPlots:  responseWithInstances.StrategyPlots,
	}
	return response, nil
}

// callWorkerBacktestWithProgress calls the worker's run_backtest function via Redis queue with progress callbacks
func callWorkerBacktestWithProgress(ctx context.Context, conn *data.Conn, userID int, args RunBacktestArgs, progressCallback ProgressCallback) (*WorkerBacktestResult, error) {
	// Generate unique task ID
	taskID := fmt.Sprintf("backtest_%d_%d", args.StrategyID, time.Now().UnixNano())

	// Prepare backtest task payload
	task := map[string]interface{}{
		"task_id":   taskID,
		"task_type": "backtest",
		"args": map[string]interface{}{
			"strategy_id": fmt.Sprintf("%d", args.StrategyID),
			"user_id":     fmt.Sprintf("%d", userID), // Include user ID for ownership verification
			"start_date":  args.StartDate,
			"end_date":    args.EndDate,
		},
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}

	// Check for active backtests using comprehensive check
	hasActiveBacktest, err := checkForActiveBacktests(ctx, conn, userID)
	if err != nil {
		return nil, fmt.Errorf("error checking for active backtests: %v", err)
	}
	if hasActiveBacktest {
		return nil, fmt.Errorf("another backtest is already queued or running for your account. Please wait for it to complete before starting a new one")
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
func convertWorkerInstancesToBacktestResults(instances []map[string]any) []BacktestInstanceRow {
	results := make([]BacktestInstanceRow, len(instances))

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

		results[i] = BacktestInstanceRow{
			Ticker:         ticker,
			Timestamp:      timestamp,
			Classification: true,     // Since instance was returned, it met criteria
			Instance:       instance, // Include the complete original instance
		}
	}

	return results
}

// convertWorkerSummaryToBacktestSummary converts worker summary to API format
func convertWorkerSummaryToBacktestSummary(summary WorkerSummary, instances []map[string]any) BacktestSummary {
	// Extract unique column names from instances
	columnSet := make(map[string]bool)
	for _, instance := range instances {
		for key := range instance {
			columnSet[key] = true
		}
	}
	// Convert to sorted slice for consistent output
	columns := make([]string, 0, len(columnSet))
	for column := range columnSet {
		columns = append(columns, column)
	}

	return BacktestSummary{
		TotalInstances:   summary.TotalInstances,
		DateRange:        []string(summary.DateRange),
		SymbolsProcessed: summary.SymbolsProcessed,
		Columns:          columns,
	}
}

// extractPlotAttributes extracts chart attributes from plotly JSON data
func extractPlotAttributes(plot *StrategyPlot, plotData map[string]any) {
	if plotData == nil {
		return
	}
	// Safely convert plotData["data"] to []map[string]any
	if dataSlice, ok := plotData["data"].([]interface{}); ok {
		converted := make([]map[string]any, len(dataSlice))
		for i, v := range dataSlice {
			if m, ok := v.(map[string]any); ok {
				converted[i] = m
			} else {
				converted[i] = nil // or handle error as needed
			}
		}
		plot.Data = converted
	}
	// Extract chart title
	if layout, ok := plotData["layout"].(map[string]any); ok {
		if title, ok := layout["title"].(map[string]any); ok {
			if titleText, ok := title["text"].(string); ok {
				plot.Title = titleText
			}
		} else if titleStr, ok := layout["title"].(string); ok {
			plot.Title = titleStr
		}
	}

	// Extract chart type and length from data traces
	if dataTraces, ok := plotData["data"].([]interface{}); ok && len(dataTraces) > 0 {
		plot.Length = len(dataTraces)

		// Get chart type from first trace
		if firstTrace, ok := dataTraces[0].(map[string]any); ok {
			if traceType, ok := firstTrace["type"].(string); ok {
				plot.ChartType = mapPlotlyTypeToChartType(traceType, firstTrace)
			}
		}
	}

	// Extract minimal layout information
	if layout, ok := plotData["layout"].(map[string]any); ok {
		minimalLayout := make(map[string]any)

		// Extract axis titles
		if xaxis, ok := layout["xaxis"].(map[string]any); ok {
			if xaxisTitle, ok := xaxis["title"].(map[string]any); ok {
				if titleText, ok := xaxisTitle["text"].(string); ok {
					minimalLayout["xaxis"] = map[string]any{"title": titleText}
				}
			} else if titleStr, ok := xaxis["title"].(string); ok {
				minimalLayout["xaxis"] = map[string]any{"title": titleStr}
			}
		}

		if yaxis, ok := layout["yaxis"].(map[string]any); ok {
			if yaxisTitle, ok := yaxis["title"].(map[string]any); ok {
				if titleText, ok := yaxisTitle["text"].(string); ok {
					minimalLayout["yaxis"] = map[string]any{"title": titleText}
				}
			} else if titleStr, ok := yaxis["title"].(string); ok {
				minimalLayout["yaxis"] = map[string]any{"title": titleStr}
			}
		}

		// Extract dimensions
		if width, ok := layout["width"]; ok {
			minimalLayout["width"] = width
		}
		if height, ok := layout["height"]; ok {
			minimalLayout["height"] = height
		}

		plot.Layout = minimalLayout
	}
}

// mapPlotlyTypeToChartType converts plotly trace types to standard chart types
func mapPlotlyTypeToChartType(traceType string, trace map[string]any) string {
	switch traceType {
	case "scatter":
		if mode, ok := trace["mode"].(string); ok {
			if mode == "lines" {
				return "line"
			}
		}
		return "scatter"
	case "line":
		return "line"
	case "bar":
		return "bar"
	case "histogram":
		return "histogram"
	case "heatmap":
		return "heatmap"
	case "box":
		return "bar" // Fallback
	case "violin":
		return "bar" // Fallback
	case "pie":
		return "bar" // Fallback
	case "candlestick":
		return "line" // Fallback
	case "ohlc":
		return "line" // Fallback
	default:
		return "line"
	}
}
