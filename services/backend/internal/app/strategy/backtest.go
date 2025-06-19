package strategy

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
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

// RunBacktest executes a prompt-based strategy backtest
func RunBacktest(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (any, error) {
	var args BacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Get the strategy from the database
	var strategy Strategy
	err := conn.DB.QueryRow(context.Background(), `
		SELECT strategyid, name, 
		       COALESCE(description, '') as description,
		       COALESCE(prompt, '') as prompt,
		       COALESCE(pythoncode, '') as pythoncode,
		       COALESCE(version, '1.0') as version
		FROM strategies WHERE strategyid = $1 AND userid = $2`, args.StrategyID, userID).Scan(
		&strategy.StrategyID,
		&strategy.Name,
		&strategy.Description,
		&strategy.Prompt,
		&strategy.PythonCode,
		&strategy.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("error retrieving strategy: %v", err)
	}

	if strategy.PythonCode == "" {
		return nil, fmt.Errorf("strategy has no Python code to execute")
	}

	// Default return windows if not specified
	returnWindows := args.ReturnWindows
	if len(returnWindows) == 0 {
		returnWindows = []int{1, 5, 10} // Default to 1, 5, and 10 day returns
	}

	// Get historical data for backtesting (last 2 years)
	endDate := time.Now()
	startDate := endDate.AddDate(-2, 0, 0) // 2 years ago

	// Get all active stocks for backtesting
	symbols, err := getActiveSymbols(conn)
	if err != nil {
		return nil, fmt.Errorf("error getting symbols: %v", err)
	}

	// Limit symbols for performance (can be made configurable)
	if len(symbols) > 500 {
		symbols = symbols[:500]
	}

	log.Printf("Running backtest for strategy %d with %d symbols from %s to %s",
		args.StrategyID, len(symbols), startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	// Run the backtest
	results, summary, err := executeBacktest(ctx, conn, strategy, symbols, startDate, endDate, returnWindows)
	if err != nil {
		return nil, fmt.Errorf("error executing backtest: %v", err)
	}

	// Prepare response
	response := BacktestResponse{
		Instances: results,
		Summary:   summary,
	}

	// Cache the results
	if err := SaveBacktestToCache(ctx, conn, userID, args.StrategyID, response); err != nil {
		log.Printf("Warning: Failed to cache backtest results: %v", err)
		// Don't return error, just log warning
	}

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
