package strategy

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// UnifiedStrategyArgs represents arguments for unified strategy execution
type UnifiedStrategyArgs struct {
	StrategyID      int      `json:"strategyId"`
	ExecutionMode   string   `json:"executionMode"` // "realtime", "backtest", "screening"
	StartDate       *string  `json:"startDate,omitempty"`
	EndDate         *string  `json:"endDate,omitempty"`
	Symbols         []string `json:"symbols,omitempty"`
	Universe        []string `json:"universe,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	IntervalSeconds int      `json:"intervalSeconds,omitempty"` // For realtime alerts
}

// UnifiedStrategyResult represents the result from unified strategy execution
type UnifiedStrategyResult struct {
	Mode            string `json:"mode"`
	Success         bool   `json:"success"`
	ExecutionTimeMs int    `json:"executionTimeMs"`
	ErrorMessage    string `json:"errorMessage,omitempty"`

	// Realtime results
	Alerts         []map[string]any `json:"alerts,omitempty"`
	CurrentSignals map[string]any   `json:"currentSignals,omitempty"`

	// Backtest results
	Instances          []BacktestResult `json:"instances,omitempty"`
	Summary            map[string]any   `json:"summary,omitempty"`
	PerformanceMetrics map[string]any   `json:"performanceMetrics,omitempty"`

	// Screening results
	RankedResults []map[string]any   `json:"rankedResults,omitempty"`
	Scores        map[string]float64 `json:"scores,omitempty"`
	UniverseSize  int                `json:"universeSize,omitempty"`

	// Raw strategy results
	StrategyResults map[string]any `json:"strategyResults"`
}

// RunUnifiedStrategy executes a strategy in the specified mode using the unified engine
func RunUnifiedStrategy(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (any, error) {
	var args UnifiedStrategyArgs
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

	// Execute using the unified Python engine
	result, err := executeUnifiedStrategy(ctx, conn, strategy, args)
	if err != nil {
		return nil, fmt.Errorf("error executing unified strategy: %v", err)
	}

	// Cache results if successful
	if result.Success {
		if err := cacheUnifiedStrategyResult(ctx, conn, userID, args.StrategyID, args.ExecutionMode, result); err != nil {
			log.Printf("Warning: Failed to cache unified strategy results: %v", err)
		}
	}

	return result, nil
}

// executeUnifiedStrategy executes the strategy using the Python unified engine
func executeUnifiedStrategy(ctx context.Context, conn *data.Conn, strategy Strategy, args UnifiedStrategyArgs) (*UnifiedStrategyResult, error) {
	// Create Python executor
	executor := NewPythonExecutor(conn)

	// Prepare execution job for unified engine
	executionID := fmt.Sprintf("unified_%s_%d_%d", args.ExecutionMode, strategy.StrategyID, time.Now().UnixNano())

	// Build input data based on execution mode
	inputData := map[string]interface{}{
		"execution_mode": args.ExecutionMode,
		"strategy_id":    strategy.StrategyID,
	}

	// Add mode-specific parameters
	switch args.ExecutionMode {
	case "realtime":
		inputData["symbols"] = args.Symbols
		if len(args.Symbols) == 0 {
			// Default symbols for realtime if none provided
			inputData["symbols"] = []string{"AAPL", "MSFT", "GOOGL", "AMZN", "TSLA"}
		}

	case "backtest":
		if args.StartDate != nil {
			inputData["start_date"] = *args.StartDate
		} else {
			// Default to 1 year ago
			inputData["start_date"] = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
		}
		if args.EndDate != nil {
			inputData["end_date"] = *args.EndDate
		} else {
			inputData["end_date"] = time.Now().Format("2006-01-02")
		}
		inputData["symbols"] = args.Symbols

	case "screening":
		inputData["universe"] = args.Universe
		if args.Limit > 0 {
			inputData["limit"] = args.Limit
		} else {
			inputData["limit"] = 50 // Default limit
		}

	default:
		return nil, fmt.Errorf("unsupported execution mode: %s", args.ExecutionMode)
	}

	// Wrap the strategy code for unified execution
	wrappedCode := wrapStrategyForUnifiedExecution(strategy.PythonCode, args.ExecutionMode)

	// Create execution job
	job := PythonExecutionJob{
		ExecutionID:    executionID,
		PythonCode:     wrappedCode,
		InputData:      inputData,
		TimeoutSeconds: getTimeoutForMode(args.ExecutionMode),
		MemoryLimitMB:  256, // Increased for unified execution
		Libraries:      []string{"numpy", "pandas"},
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	// Submit job to Redis queue
	err := executor.submitJob(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("failed to submit unified strategy job: %w", err)
	}

	// Wait for result with longer timeout for backtest/screening
	timeout := time.Duration(getTimeoutForMode(args.ExecutionMode)) * time.Second
	pythonResult, err := executor.waitForResult(ctx, executionID, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get unified strategy result: %w", err)
	}

	if pythonResult.Status != "completed" {
		return &UnifiedStrategyResult{
			Mode:         args.ExecutionMode,
			Success:      false,
			ErrorMessage: pythonResult.ErrorMessage,
		}, nil
	}

	// Parse the unified result
	result, err := parseUnifiedStrategyResult(pythonResult.OutputData, args.ExecutionMode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unified strategy result: %v", err)
	}

	result.ExecutionTimeMs = pythonResult.ExecutionTimeMs
	return result, nil
}

// wrapStrategyForUnifiedExecution wraps strategy code for the unified execution engine
func wrapStrategyForUnifiedExecution(strategyCode string, executionMode string) string {
	return fmt.Sprintf(`
# Unified Strategy Execution Wrapper for mode: %s
import json
from datetime import datetime

# Import the unified strategy engine components
try:
    from unified_strategy_engine import UnifiedStrategy, UnifiedStrategyEngine, ExecutionMode
    UNIFIED_ENGINE_AVAILABLE = True
except ImportError:
    UNIFIED_ENGINE_AVAILABLE = False

# Original strategy code
%s

# Unified execution logic
async def execute_unified_strategy():
    execution_mode = input_data.get('execution_mode', 'realtime')
    
    try:
        if UNIFIED_ENGINE_AVAILABLE:
            # Use the unified engine if available
            engine = UnifiedStrategyEngine()
            strategy = UnifiedStrategy("""
%s
""", strategy_id=input_data.get('strategy_id', 1))
            
            # Execute in the specified mode
            if execution_mode == 'realtime':
                symbols = input_data.get('symbols', [])
                result = await engine.execute_strategy(
                    strategy, ExecutionMode.REALTIME, symbols=symbols
                )
                
                save_result('alerts', result.alerts if result.success else [])
                save_result('signals', result.current_signals if result.success else {})
                save_result('success', result.success)
                save_result('error_message', result.error_message or '')
                
            elif execution_mode == 'backtest':
                start_date = input_data.get('start_date')
                end_date = input_data.get('end_date')
                symbols = input_data.get('symbols', [])
                
                result = await engine.execute_strategy(
                    strategy, ExecutionMode.BACKTEST,
                    start_date=start_date, end_date=end_date, symbols=symbols
                )
                
                save_result('instances', result.instances if result.success else [])
                save_result('summary', result.summary if result.success else {})
                save_result('performance_metrics', result.performance_metrics if result.success else {})
                save_result('success', result.success)
                save_result('error_message', result.error_message or '')
                
            elif execution_mode == 'screening':
                universe = input_data.get('universe', [])
                limit = input_data.get('limit', 50)
                
                result = await engine.execute_strategy(
                    strategy, ExecutionMode.SCREENING,
                    universe=universe, limit=limit
                )
                
                save_result('ranked_results', result.ranked_results if result.success else [])
                save_result('scores', result.scores if result.success else {})
                save_result('universe_size', result.universe_size if result.success else 0)
                save_result('success', result.success)
                save_result('error_message', result.error_message or '')
                
        else:
            # Fallback to legacy execution
            await execute_legacy_mode()
            
    except Exception as e:
        save_result('success', False)
        save_result('error_message', str(e))
        log(f"Unified execution error: {e}")

async def execute_legacy_mode():
    """Fallback execution using legacy approach"""
    execution_mode = input_data.get('execution_mode', 'realtime')
    
    if execution_mode == 'realtime':
        # Legacy realtime execution
        symbols = input_data.get('symbols', [])
        alerts = []
        signals = {}
        
        # Try to use run_realtime_scan if available
        if 'run_realtime_scan' in globals():
            result = run_realtime_scan(symbols)
            alerts = result.get('alerts', [])
            signals = result.get('signals', {})
        else:
            # Fallback: classify each symbol
            for symbol in symbols[:5]:  # Limit for performance
                try:
                    if 'classify_symbol' in globals() and classify_symbol(symbol):
                        signals[symbol] = {'signal': True, 'timestamp': datetime.utcnow().isoformat()}
                        alerts.append({'symbol': symbol, 'type': 'signal', 'message': f'{symbol} triggered'})
                except Exception:
                    continue
        
        save_result('alerts', alerts)
        save_result('signals', signals)
        save_result('success', True)
        
    elif execution_mode == 'backtest':
        # Legacy backtest execution
        start_date = input_data.get('start_date')
        end_date = input_data.get('end_date')
        symbols = input_data.get('symbols', [])
        
        instances = []
        
        # Try to use run_batch_backtest if available
        if 'run_batch_backtest' in globals():
            result = run_batch_backtest(start_date, end_date, symbols)
            instances = result.get('instances', [])
            performance_metrics = result.get('performance_metrics', {})
        else:
            # Fallback: simple classification
            for symbol in symbols[:10]:  # Limit for performance
                try:
                    if 'classify_symbol' in globals() and classify_symbol(symbol):
                        instances.append({
                            'ticker': symbol,
                            'timestamp': int(datetime.utcnow().timestamp() * 1000),
                            'classification': True
                        })
                except Exception:
                    continue
            
            performance_metrics = {'total_picks': len(instances)}
        
        save_result('instances', instances)
        save_result('summary', {'total_instances': len(instances)})
        save_result('performance_metrics', performance_metrics)
        save_result('success', True)
        
    elif execution_mode == 'screening':
        # Legacy screening execution
        universe = input_data.get('universe', [])
        limit = input_data.get('limit', 50)
        
        ranked_results = []
        scores = {}
        
        # Try to use run_screening if available
        if 'run_screening' in globals():
            result = run_screening(universe, limit)
            ranked_results = result.get('ranked_results', [])
            scores = result.get('scores', {})
        else:
            # Fallback: score each symbol
            symbol_scores = []
            for symbol in universe:
                try:
                    if 'score_symbol' in globals():
                        score = score_symbol(symbol)
                        if score > 0:
                            symbol_scores.append({'symbol': symbol, 'score': score})
                            scores[symbol] = score
                    elif 'classify_symbol' in globals() and classify_symbol(symbol):
                        symbol_scores.append({'symbol': symbol, 'score': 1.0})
                        scores[symbol] = 1.0
                except Exception:
                    continue
            
            symbol_scores.sort(key=lambda x: x['score'], reverse=True)
            ranked_results = symbol_scores[:limit]
        
        save_result('ranked_results', ranked_results)
        save_result('scores', scores)
        save_result('universe_size', len(universe))
        save_result('success', True)

# Execute the unified strategy
import asyncio
asyncio.run(execute_unified_strategy())
`, executionMode, strategyCode, strategyCode)
}

// parseUnifiedStrategyResult parses the Python result into Go structures
func parseUnifiedStrategyResult(outputData map[string]interface{}, executionMode string) (*UnifiedStrategyResult, error) {
	result := &UnifiedStrategyResult{
		Mode:            executionMode,
		Success:         false,
		StrategyResults: outputData,
	}

	// Check success
	if success, ok := outputData["success"].(bool); ok {
		result.Success = success
	}

	// Get error message if any
	if errorMsg, ok := outputData["error_message"].(string); ok {
		result.ErrorMessage = errorMsg
	}

	if !result.Success {
		return result, nil
	}

	// Parse mode-specific results
	switch executionMode {
	case "realtime":
		if alerts, ok := outputData["alerts"].([]interface{}); ok {
			result.Alerts = make([]map[string]any, len(alerts))
			for i, alert := range alerts {
				if alertMap, ok := alert.(map[string]interface{}); ok {
					result.Alerts[i] = alertMap
				}
			}
		}

		if signals, ok := outputData["signals"].(map[string]interface{}); ok {
			result.CurrentSignals = signals
		}

	case "backtest":
		if instances, ok := outputData["instances"].([]interface{}); ok {
			result.Instances = make([]BacktestResult, len(instances))
			for i, instance := range instances {
				if instanceMap, ok := instance.(map[string]interface{}); ok {
					// Convert to BacktestResult
					var backtestResult BacktestResult
					if ticker, ok := instanceMap["ticker"].(string); ok {
						backtestResult.Ticker = ticker
					}
					if timestamp, ok := instanceMap["timestamp"].(float64); ok {
						backtestResult.Timestamp = int64(timestamp)
					}
					if classification, ok := instanceMap["classification"].(bool); ok {
						backtestResult.Classification = classification
					}
					// Add other fields as needed
					backtestResult.StrategyResults = instanceMap
					result.Instances[i] = backtestResult
				}
			}
		}

		if summary, ok := outputData["summary"].(map[string]interface{}); ok {
			result.Summary = summary
		}

		if metrics, ok := outputData["performance_metrics"].(map[string]interface{}); ok {
			result.PerformanceMetrics = metrics
		}

	case "screening":
		if rankedResults, ok := outputData["ranked_results"].([]interface{}); ok {
			result.RankedResults = make([]map[string]any, len(rankedResults))
			for i, item := range rankedResults {
				if itemMap, ok := item.(map[string]interface{}); ok {
					result.RankedResults[i] = itemMap
				}
			}
		}

		if scores, ok := outputData["scores"].(map[string]interface{}); ok {
			result.Scores = make(map[string]float64)
			for symbol, score := range scores {
				if scoreFloat, ok := score.(float64); ok {
					result.Scores[symbol] = scoreFloat
				}
			}
		}

		if universeSize, ok := outputData["universe_size"].(float64); ok {
			result.UniverseSize = int(universeSize)
		}
	}

	return result, nil
}

// getTimeoutForMode returns appropriate timeout for each execution mode
func getTimeoutForMode(mode string) int {
	switch mode {
	case "realtime":
		return 30 // 30 seconds for realtime
	case "backtest":
		return 300 // 5 minutes for backtest
	case "screening":
		return 120 // 2 minutes for screening
	default:
		return 60 // 1 minute default
	}
}

// cacheUnifiedStrategyResult caches the result for future retrieval
func cacheUnifiedStrategyResult(ctx context.Context, conn *data.Conn, userID, strategyID int, mode string, result *UnifiedStrategyResult) error {
	// Implement caching logic similar to existing backtest caching
	// This would store results in Redis or database for quick retrieval

	cacheKey := fmt.Sprintf("unified_strategy:%d:%d:%s", userID, strategyID, mode)

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}

	// Store in Redis with appropriate TTL
	ttl := time.Hour * 24 // 24 hours for most results
	if mode == "realtime" {
		ttl = time.Minute * 10 // 10 minutes for realtime results
	}

	return conn.Cache.Set(ctx, cacheKey, resultJSON, ttl).Err()
}

// GetCachedUnifiedStrategyResult retrieves cached results
func GetCachedUnifiedStrategyResult(ctx context.Context, conn *data.Conn, userID, strategyID int, mode string) (*UnifiedStrategyResult, error) {
	cacheKey := fmt.Sprintf("unified_strategy:%d:%d:%s", userID, strategyID, mode)

	resultJSON, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, err
	}

	var result UnifiedStrategyResult
	err = json.Unmarshal([]byte(resultJSON), &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
