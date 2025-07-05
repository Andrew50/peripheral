package strategy

import (
	"backend/internal/data"
	"backend/internal/app/limits"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

const BacktestCacheKey = "backtest:userID:%d:strategyID:%d"

// RunBacktestArgs represents arguments for backtesting (API compatibility)
type RunBacktestArgs struct {
	StrategyID  int   `json:"strategyId"`
	Securities  []int `json:"securities"`
	Start       int64 `json:"start"`
	FullResults bool  `json:"fullResults"`
}

// BacktestInstanceRow represents a single backtest instance (API compatibility)
type BacktestInstanceRow struct {
	Ticker          string             `json:"ticker"`
	SecurityID      int                `json:"securityId,omitempty"`
	Timestamp       int64              `json:"timestamp"`
	Volume          int64              `json:"volume,omitempty"`
	Classification  bool               `json:"classification"`
	FutureReturns   map[string]float64 `json:"futureReturns,omitempty"`
	StrategyResults map[string]any     `json:"strategyResults,omitempty"`
	Instance        map[string]any     `json:"instance,omitempty"`
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

// StrategyPlot represents a captured plotly plot
type StrategyPlot struct {
	ChartType string           `json:"chart_type"`       // "line", "bar", "scatter", "histogram", "heatmap"
	Data      []map[string]any `json:"data,omitempty"`   // Array of trace objects with x/y/z data arrays
	Length    int              `json:"length"`           // Length of the data array
	Title     string           `json:"title"`            // Chart title
	Layout    map[string]any   `json:"layout,omitempty"` // Minimal layout (axis labels, dimensions)
	PlotID    int              `json:"plotID"`           // Plot ID for the chart
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
	StrategyPlots      []StrategyPlot         `json:"strategy_plots,omitempty"`
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
	var args RunBacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	log.Printf("Starting complete backtest for strategy %d using new worker architecture", args.StrategyID)

	// Check if user has sufficient credits for backtest
	// TODO: This will need to be based on bars processed later (100k bars = 1 credit)
	// For now, backtests cost 1 credit regardless of size
	allowed, remainingCredits, err := limits.CheckUsageAllowed(conn, userID, limits.UsageTypeCredits, 1)
	if err != nil {
		return nil, fmt.Errorf("error checking credit usage limits: %v", err)
	}
	if !allowed {
		return nil, fmt.Errorf("insufficient credits to run backtest. You have %d credits remaining. Please add more credits to your account", remainingCredits)
	}

	// Verify strategy exists and user has permission
	var strategyExists bool
	err = conn.DB.QueryRow(context.Background(), `
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

	summary := convertWorkerSummaryToBacktestSummary(result.Summary, result.Instances)

	responseWithInstances := BacktestResponse{
		Summary:        summary,
		StrategyPrints: result.StrategyPrints,
		StrategyPlots:  result.StrategyPlots,
		Instances:      convertWorkerInstancesToBacktestResults(result.Instances),
	}
	// Cache the results
	if err := SetBacktestToCache(ctx, conn, userID, args.StrategyID, responseWithInstances); err != nil {
		log.Printf("Warning: Failed to cache backtest results: %v", err)
		// Don't return error, just log warning
	}

	// Record credit usage for successful backtest
	// TODO: This will need to be based on bars processed later (100k bars = 1 credit)
	// For now, backtests cost 1 credit regardless of size
	metadata := map[string]interface{}{
		"strategy_id":     args.StrategyID,
		"instances_found": len(instances),
		"symbols_processed": response.Summary.SymbolsProcessed,
		"operation_type":  "backtest",
	}
	if err := limits.RecordUsage(conn, userID, limits.UsageTypeCredits, 1, metadata); err != nil {
		log.Printf("Warning: Failed to record credit usage for backtest: %v", err)
		// Don't fail the request since backtest was successful
	}

	// Record credit usage for successful backtest
	// TODO: This will need to be based on bars processed later (100k bars = 1 credit)
	// For now, backtests cost 1 credit regardless of size
	metadata := map[string]interface{}{
		"strategy_id":     args.StrategyID,
		"instances_found": len(instances),
		"symbols_processed": response.Summary.SymbolsProcessed,
		"operation_type":  "backtest",
	}
	if err := limits.RecordUsage(conn, userID, limits.UsageTypeCredits, 1, metadata); err != nil {
		log.Printf("Warning: Failed to record credit usage for backtest: %v", err)
		// Don't fail the request since backtest was successful
	}

	// Record credit usage for successful backtest
	// TODO: This will need to be based on bars processed later (100k bars = 1 credit)
	// For now, backtests cost 1 credit regardless of size
	metadata := map[string]interface{}{
		"strategy_id":     args.StrategyID,
		"instances_found": len(instances),
		"symbols_processed": response.Summary.SymbolsProcessed,
		"operation_type":  "backtest",
	}
	if err := limits.RecordUsage(conn, userID, limits.UsageTypeCredits, 1, metadata); err != nil {
		log.Printf("Warning: Failed to record credit usage for backtest: %v", err)
		// Don't fail the request since backtest was successful
	}

	// Remove data from plots to save memory
	for i := range result.StrategyPlots {
		result.StrategyPlots[i].Data = nil
	}
	response := BacktestResponse{
		Summary:        summary,
		StrategyPrints: result.StrategyPrints,
		StrategyPlots:  result.StrategyPlots,
	}
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

type InstanceFilter struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}
type GetBacktestInstancesArgs struct {
	StrategyID int              `json:"strategyId"`
	Filters    []InstanceFilter `json:"filters"`
}

func GetBacktestInstances(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) ([]BacktestInstanceRow, error) {
	var args GetBacktestInstancesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	backtestResponse, err := GetBacktestData(ctx, conn, userID, args.StrategyID)
	if err != nil {
		return nil, fmt.Errorf("error getting backtest data: %v", err)
	}

	if len(args.Filters) == 0 {
		if len(backtestResponse.Instances) > 20 {
			return backtestResponse.Instances[:20], nil
		}
		return backtestResponse.Instances, nil
	}

	filteredInstances := FilterInstances(backtestResponse.Instances, args.Filters)

	return filteredInstances, nil
}

func FilterInstances(instances []BacktestInstanceRow, filters []InstanceFilter) []BacktestInstanceRow {
	if len(filters) == 0 {
		return instances
	}

	var filtered []BacktestInstanceRow
	for _, instance := range instances {
		if matchesAllFilters(instance, filters) {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}

// matchesAllFilters checks if an instance matches all provided filters (AND logic)
func matchesAllFilters(instance BacktestInstanceRow, filters []InstanceFilter) bool {
	for _, filter := range filters {
		if !matchesFilter(instance, filter) {
			return false
		}
	}
	return true
}

// matchesFilter checks if an instance matches a single filter
func matchesFilter(instance BacktestInstanceRow, filter InstanceFilter) bool {
	// Extract the value from the instance
	instanceValue := extractValueFromInstanceColumn(instance, filter.Column)
	if instanceValue == nil {
		return false // Field doesn't exist or is nil
	}
	// Apply the operator
	return applyOperator(instanceValue, filter.Operator, filter.Value)
}

// extractValueFromInstanceColumn gets the value of a field from a BacktestResult
func extractValueFromInstanceColumn(instance BacktestInstanceRow, column string) interface{} {
	// Check structured fields first
	switch column {
	case "ticker":
		return instance.Ticker
	case "timestamp":
		return instance.Timestamp
	case "volume":
		return instance.Volume
	case "classification":
		return instance.Classification
	}

	// Check dynamic fields in Instance map
	if instance.Instance != nil {
		if value, exists := instance.Instance[column]; exists {
			return value
		}
	}

	// Check StrategyResults map
	if instance.StrategyResults != nil {
		if value, exists := instance.StrategyResults[column]; exists {
			return value
		}
	}

	// Check FutureReturns map
	if instance.FutureReturns != nil {
		if value, exists := instance.FutureReturns[column]; exists {
			return value
		}
	}

	return nil
}

// applyOperator applies the comparison operator between instanceValue and filterValue
func applyOperator(instanceValue interface{}, operator string, filterValue interface{}) bool {
	switch operator {
	case "eq":
		return compareEqual(instanceValue, filterValue)
	case "gt", "gte", "lt", "lte":
		return compareNumbers(instanceValue, filterValue, operator)
	case "contains":
		return compareContains(instanceValue, filterValue)
	case "in":
		return compareIn(instanceValue, filterValue)
	default:
		return false
	}
}

// compareEqual checks equality with type conversion
func compareEqual(instanceValue, filterValue interface{}) bool {
	// Handle string comparisons
	if instStr, ok := instanceValue.(string); ok {
		if filtStr, ok := filterValue.(string); ok {
			return instStr == filtStr
		}
	}

	// Handle numeric comparisons using unified function
	if compareNumbers(instanceValue, filterValue, "eq") {
		return true
	}

	// Handle boolean comparisons
	if instBool, ok := instanceValue.(bool); ok {
		if filtBool, ok := filterValue.(bool); ok {
			return instBool == filtBool
		}
	}

	// Fallback to direct comparison
	return instanceValue == filterValue
}

// compareNumbers performs numeric comparison based on operator
func compareNumbers(instanceValue, filterValue interface{}, operator string) bool {
	instNum, instIsNum := convertToFloat64(instanceValue)
	filtNum, filtIsNum := convertToFloat64(filterValue)
	if !instIsNum || !filtIsNum {
		return false
	}

	switch operator {
	case "gt":
		return instNum > filtNum
	case "gte":
		return instNum >= filtNum
	case "lt":
		return instNum < filtNum
	case "lte":
		return instNum <= filtNum
	case "eq":
		return instNum == filtNum
	default:
		return false
	}
}

// compareContains checks if instanceValue contains filterValue (for strings)
func compareContains(instanceValue, filterValue interface{}) bool {
	instStr, instOk := instanceValue.(string)
	filtStr, filtOk := filterValue.(string)
	if instOk && filtOk && len(filtStr) > 0 {
		return strings.Contains(instStr, filtStr)
	}
	return false
}

// compareIn checks if instanceValue is in the filterValue array
func compareIn(instanceValue, filterValue interface{}) bool {
	// filterValue should be an array/slice
	switch filtArray := filterValue.(type) {
	case []interface{}:
		for _, val := range filtArray {
			if compareEqual(instanceValue, val) {
				return true
			}
		}
	case []string:
		instStr, ok := instanceValue.(string)
		if ok {
			for _, val := range filtArray {
				if instStr == val {
					return true
				}
			}
		}
	case []float64:
		instNum, ok := convertToFloat64(instanceValue)
		if ok {
			for _, val := range filtArray {
				if instNum == val {
					return true
				}
			}
		}
	}
	return false
}

// convertToFloat64 attempts to convert various numeric types to float64
func convertToFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}

func GetBacktestData(ctx context.Context, conn *data.Conn, userID int, strategyID int) (*BacktestResponse, error) {
	response, err := GetBacktestFromCache(ctx, conn, userID, strategyID)
	if err != nil {
		return nil, fmt.Errorf("error getting backtest from cache: %v", err)
	}
	return response, nil
}
