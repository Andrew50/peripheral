package strategy

import (
	"backend/internal/data"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"time"
)

// BacktestArgs represents arguments for backtesting (kept for API compatibility)
type BacktestArgs struct {
	StrategyID    int   `json:"strategyId"`
	Securities    []int `json:"securities"`
	Start         int64 `json:"start"`
	ReturnWindows []int `json:"returnWindows"`
	FullResults   bool  `json:"fullResults"`
}

// BacktestResult represents a single backtest instance
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
}

// BacktestSummary contains summary statistics of the backtest
type BacktestSummary struct {
	TotalInstances   int              `json:"totalInstances"`
	PositiveSignals  int              `json:"positiveSignals"`
	DateRange        []string         `json:"dateRange"`
	SymbolsProcessed int              `json:"symbolsProcessed"`
	Columns          []string         `json:"columns"`
	ColumnSamples    map[string][]any `json:"columnSamples"`
}

// BacktestResponse represents the complete backtest response
type BacktestResponse struct {
	Instances []BacktestResult `json:"instances"`
	Summary   BacktestSummary  `json:"summary"`
}

// WorkerBacktestResult represents the result from the worker's run_backtest function
type WorkerBacktestResult struct {
	Success            bool                   `json:"success"`
	StrategyID         int                    `json:"strategy_id"`
	ExecutionMode      string                 `json:"execution_mode"`
	Instances          []WorkerInstance       `json:"instances"`
	Summary            WorkerSummary          `json:"summary"`
	PerformanceMetrics map[string]interface{} `json:"performance_metrics"`
	ExecutionTimeMs    int                    `json:"execution_time_ms"`
	ErrorMessage       string                 `json:"error_message,omitempty"`
}

type WorkerInstance struct {
	Ticker          string                 `json:"ticker"`
	Timestamp       int64                  `json:"timestamp"`
	Classification  bool                   `json:"classification"`
	EntryPrice      float64                `json:"entry_price,omitempty"`
	StrategyResults map[string]interface{} `json:"strategy_results,omitempty"`
	FutureReturn    float64                `json:"future_return,omitempty"`
}

type WorkerSummary struct {
	TotalInstances            int      `json:"total_instances"`
	PositiveSignals           int      `json:"positive_signals"`
	DateRange                 []string `json:"date_range"`
	SymbolsProcessed          int      `json:"symbols_processed"`
	ExecutionType             string   `json:"execution_type,omitempty"`
	SuccessfulClassifications int      `json:"successful_classifications,omitempty"`
}

// RunBacktest executes a complete strategy backtest using the new worker architecture
func RunBacktest(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (any, error) {
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
	result, err := callWorkerBacktest(args.StrategyID)
	if err != nil {
		return nil, fmt.Errorf("error executing worker backtest: %v", err)
	}

	// Convert worker result to BacktestResponse format for API compatibility
	instances := convertWorkerInstancesToBacktestResults(result.Instances)
	summary := convertWorkerSummaryToBacktestSummary(result.Summary)

	response := BacktestResponse{
		Instances: instances,
		Summary:   summary,
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

// getActiveSymbols retrieves a list of active stock symbols for backtesting
func getActiveSymbols(conn *data.Conn) ([]string, error) {
	query := `
		SELECT DISTINCT ticker 
		FROM securities 
		WHERE active = true 
		AND locale = 'us'
		AND market = 'stocks'
		ORDER BY ticker
		LIMIT 1000`

	rows, err := conn.DB.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// executeBacktest runs the strategy against historical data
func executeBacktest(ctx context.Context, conn *data.Conn, strategy Strategy, symbols []string,
	startDate, endDate time.Time, returnWindows []int) ([]BacktestResult, BacktestSummary, error) {

	var results []BacktestResult
	var processedSymbols int
	var positiveSignals int

	// Process each symbol
	for _, symbol := range symbols {
		symbolResults, err := processSymbolBacktest(ctx, conn, strategy, symbol, startDate, endDate, returnWindows)
		if err != nil {
			log.Printf("Error processing symbol %s: %v", symbol, err)
			continue
		}

		if len(symbolResults) > 0 {
			processedSymbols++
			for _, result := range symbolResults {
				if result.Classification {
					positiveSignals++
				}
				results = append(results, result)
			}
		}
	}

	// Sort results by timestamp
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp < results[j].Timestamp
	})

	// Create summary
	summary := createBacktestSummary(results, processedSymbols, positiveSignals, returnWindows)

	log.Printf("Backtest completed: %d instances, %d positive signals across %d symbols",
		len(results), positiveSignals, processedSymbols)

	return results, summary, nil
}

// processSymbolBacktest processes a single symbol for the backtest
func processSymbolBacktest(ctx context.Context, conn *data.Conn, strategy Strategy, symbol string,
	startDate, endDate time.Time, returnWindows []int) ([]BacktestResult, error) {

	// Get historical price data for this symbol
	query := `
		SELECT d.timestamp, d.open, d.high, d.low, d.close, d.volume, s.securityid
		FROM ohlcv_1d d
		JOIN securities s ON s.securityid = d.securityid
		WHERE s.ticker = $1 
		AND d.timestamp >= $2 
		AND d.timestamp <= $3
		ORDER BY d.timestamp`

	rows, err := conn.DB.Query(ctx, query, symbol, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dayData []struct {
		Timestamp  time.Time
		Open       float64
		High       float64
		Low        float64
		Close      float64
		Volume     int64
		SecurityID int
	}

	for rows.Next() {
		var data struct {
			Timestamp  time.Time
			Open       float64
			High       float64
			Low        float64
			Close      float64
			Volume     int64
			SecurityID int
		}
		if err := rows.Scan(&data.Timestamp, &data.Open, &data.High, &data.Low, &data.Close, &data.Volume, &data.SecurityID); err != nil {
			return nil, err
		}
		dayData = append(dayData, data)
	}

	if len(dayData) < 30 { // Need at least 30 days of data
		return nil, nil
	}

	var results []BacktestResult

	// Test the strategy for each day (starting from day 30 to have enough history)
	for i := 30; i < len(dayData); i++ {
		currentDay := dayData[i]

		// Execute the strategy for this symbol/date
		classification, strategyResults, err := executeStrategyForDay(ctx, conn, strategy, symbol, currentDay.Timestamp)
		if err != nil {
			log.Printf("Error executing strategy for %s on %s: %v", symbol, currentDay.Timestamp.Format("2006-01-02"), err)
			continue
		}

		// Calculate future returns if strategy gave a positive signal
		var futureReturns map[string]float64
		if classification {
			futureReturns = calculateFutureReturns(dayData, i, returnWindows)
		}

		result := BacktestResult{
			Ticker:          symbol,
			SecurityID:      currentDay.SecurityID,
			Timestamp:       currentDay.Timestamp.UnixMilli(),
			Open:            currentDay.Open,
			High:            currentDay.High,
			Low:             currentDay.Low,
			Close:           currentDay.Close,
			Volume:          currentDay.Volume,
			Classification:  classification,
			FutureReturns:   futureReturns,
			StrategyResults: strategyResults,
		}

		results = append(results, result)
	}

	return results, nil
}

// executeStrategyForDay executes the Python strategy for a specific symbol and date
func executeStrategyForDay(ctx context.Context, conn *data.Conn, strategy Strategy, symbol string, date time.Time) (bool, map[string]any, error) {
	// Create Python executor
	executor := NewPythonExecutor(conn)

	// Execute the actual Python strategy code
	classification, strategyResults, err := executor.ExecuteStrategy(ctx, strategy.PythonCode, symbol, date)
	if err != nil {
		// Log the error but don't fail the entire backtest
		log.Printf("Python execution error for %s on %s: %v", symbol, date.Format("2006-01-02"), err)
		return false, make(map[string]any), nil
	}

	return classification, strategyResults, nil
}

// calculateFutureReturns calculates returns for specified forward-looking windows
func calculateFutureReturns(dayData []struct {
	Timestamp  time.Time
	Open       float64
	High       float64
	Low        float64
	Close      float64
	Volume     int64
	SecurityID int
}, currentIndex int, returnWindows []int) map[string]float64 {

	futureReturns := make(map[string]float64)
	currentClose := dayData[currentIndex].Close

	for _, window := range returnWindows {
		futureIndex := currentIndex + window
		if futureIndex < len(dayData) {
			futureClose := dayData[futureIndex].Close
			returnPct := ((futureClose - currentClose) / currentClose) * 100
			futureReturns[fmt.Sprintf("future_%dd_return", window)] = math.Round(returnPct*100) / 100
		}
	}

	return futureReturns
}

// createBacktestSummary creates a summary of the backtest results
func createBacktestSummary(results []BacktestResult, processedSymbols, positiveSignals int, returnWindows []int) BacktestSummary {
	var minDate, maxDate time.Time
	columns := []string{"ticker", "timestamp", "open", "high", "low", "close", "volume", "classification"}
	columnSamples := make(map[string][]any)

	// Add future return columns
	for _, window := range returnWindows {
		columns = append(columns, fmt.Sprintf("future_%dd_return", window))
	}

	if len(results) > 0 {
		minDate = time.UnixMilli(results[0].Timestamp)
		maxDate = time.UnixMilli(results[len(results)-1].Timestamp)

		// Create sample data for each column
		sampleSize := 3
		if len(results) < sampleSize {
			sampleSize = len(results)
		}

		for _, col := range columns {
			var samples []any
			for i := 0; i < sampleSize && i < len(results); i++ {
				result := results[i]
				switch col {
				case "ticker":
					samples = append(samples, result.Ticker)
				case "timestamp":
					samples = append(samples, result.Timestamp)
				case "open":
					samples = append(samples, result.Open)
				case "high":
					samples = append(samples, result.High)
				case "low":
					samples = append(samples, result.Low)
				case "close":
					samples = append(samples, result.Close)
				case "volume":
					samples = append(samples, result.Volume)
				case "classification":
					samples = append(samples, result.Classification)
				default:
					if result.FutureReturns != nil {
						if val, exists := result.FutureReturns[col]; exists {
							samples = append(samples, val)
						}
					}
				}
			}
			columnSamples[col] = samples
		}
	}

	dateRange := []string{}
	if !minDate.IsZero() && !maxDate.IsZero() {
		dateRange = []string{minDate.Format("2006-01-02"), maxDate.Format("2006-01-02")}
	}

	return BacktestSummary{
		TotalInstances:   len(results),
		PositiveSignals:  positiveSignals,
		DateRange:        dateRange,
		SymbolsProcessed: processedSymbols,
		Columns:          columns,
		ColumnSamples:    columnSamples,
	}
}

// callWorkerBacktest calls the worker's run_backtest function
func callWorkerBacktest(strategyID int) (*WorkerBacktestResult, error) {
	// Prepare request payload
	payload := map[string]interface{}{
		"function":    "run_backtest",
		"strategy_id": strategyID,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling payload: %v", err)
	}

	// Call worker service (assuming it's running on localhost:8080)
	// In production, this would be configured via environment variables
	workerURL := "http://localhost:8080/execute"

	resp, err := http.Post(workerURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("error calling worker: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("worker returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading worker response: %v", err)
	}

	var result WorkerBacktestResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling worker response: %v", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("worker execution failed: %s", result.ErrorMessage)
	}

	return &result, nil
}

// convertWorkerInstancesToBacktestResults converts worker instances to API format
func convertWorkerInstancesToBacktestResults(instances []WorkerInstance) []BacktestResult {
	results := make([]BacktestResult, len(instances))

	for i, instance := range instances {
		results[i] = BacktestResult{
			Ticker:          instance.Ticker,
			SecurityID:      0, // Will be populated if needed
			Timestamp:       instance.Timestamp,
			Open:            instance.EntryPrice,
			High:            instance.EntryPrice,
			Low:             instance.EntryPrice,
			Close:           instance.EntryPrice,
			Volume:          0,
			Classification:  instance.Classification,
			FutureReturns:   map[string]float64{},
			StrategyResults: instance.StrategyResults,
		}

		// Add future return if available
		if instance.FutureReturn != 0 {
			results[i].FutureReturns["1d"] = instance.FutureReturn
		}
	}

	return results
}

// convertWorkerSummaryToBacktestSummary converts worker summary to API format
func convertWorkerSummaryToBacktestSummary(summary WorkerSummary) BacktestSummary {
	return BacktestSummary{
		TotalInstances:   summary.TotalInstances,
		PositiveSignals:  summary.PositiveSignals,
		DateRange:        summary.DateRange,
		SymbolsProcessed: summary.SymbolsProcessed,
		Columns:          []string{"ticker", "timestamp", "classification", "entry_price"},
		ColumnSamples:    map[string][]any{},
	}
}
