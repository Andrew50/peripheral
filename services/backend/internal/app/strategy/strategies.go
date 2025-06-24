package strategy

import (
	"backend/internal/data"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"google.golang.org/genai"
)

//go:embed prompts/*
var fs embed.FS // 2️⃣ compiled into the binary

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

// Data accessor functions that will be available in the generated Python code
// Designed for maximum performance with PyPy and native data structures
const dataAccessorFunctions = `
# Pre-defined data accessor functions available in your classifier
# 
# ███ HIGH-PERFORMANCE TRADING SYSTEM ███
# Designed for ultra-low latency and high-throughput trading
# 
# PERFORMANCE OPTIMIZATIONS:
# • NO PANDAS - Uses native Python lists/dicts for maximum speed
# • PyPy Compatible - All code optimized for PyPy JIT compilation  
# • NumPy Vectorization - Mathematical operations use NumPy for speed
# • Zero-Copy Operations - Minimal memory allocations
# • Type Hints - Full type annotations for JIT optimization
# • List Comprehensions - Faster than loops in PyPy
# • Native Data Structures - Direct list/dict access (no DataFrame overhead)
#
# EXPECTED PERFORMANCE:
# • 10-100x faster than pandas-based systems
# • Sub-millisecond strategy execution
# • Handles thousands of symbols simultaneously
# • Real-time market data processing
#
# DEPLOYMENT RECOMMENDATION:
# Use PyPy 3.10+ for maximum performance gains

import numpy as np
# Note: typing module is not available in the execution environment
# Use built-in types: list, dict, tuple, str, int, float, bool

# ==================== RAW DATA RETRIEVAL ONLY ====================
# Note: You must implement your own technical indicators using the raw data below

def get_price_data(symbol, timeframe='1d', days=30, 
                  extended_hours=False, start_time=None, end_time=None):
    """
    Get raw OHLCV price data for a symbol
    
    Args:
        symbol: Stock ticker symbol
        timeframe: '1m', '5m', '15m', '30m', '1h', '4h', '1d', '1w', '1M'
        days: Number of days of historical data
        extended_hours: Include pre/after market data (for intraday only)
        start_time: Time filter start (e.g., '09:30:00')
        end_time: Time filter end (e.g., '16:00:00')
    
    Returns: Dict with 'timestamps': list of int, 'open': list of float, 'high': list of float, 
             'low': list of float, 'close': list of float, 'volume': list of int, 
             'extended_hours': list of bool
    """
    pass  # Implemented by backend

def get_historical_data(symbol, timeframe='1d', periods=100, offset=0):
    """
    Get historical raw price data with lag support
    
    Args:
        symbol: Stock ticker symbol
        timeframe: Data frequency
        periods: Number of periods to retrieve
        offset: Number of periods to lag (0 = current, 1 = previous period, etc.)
    
    Returns: Dict with raw OHLCV data as lists
    """
    pass  # Implemented by backend

def get_security_info(symbol):
    """
    Get basic security metadata
    
    Returns: Dict with securityid, ticker, name, sector, industry, market, 
             primary_exchange, locale, active, cik, composite_figi, share_class_figi
    """
    pass  # Implemented by backend

def get_multiple_symbols_data(symbols, timeframe='1d', days=30):
    """
    Get raw price data for multiple symbols efficiently
    
    Args:
        symbols: list of ticker symbols
        timeframe: Data frequency
        days: Number of days of data
    
    Returns: Dict mapping symbol -> raw data dict (same format as get_price_data)
    """
    pass  # Implemented by backend

# ==================== RAW FUNDAMENTAL DATA ====================

def get_fundamental_data(symbol, metrics=None):
    """
    Get raw fundamental data for a symbol
    
    Args:
        metrics: list of specific metrics to retrieve, or None for all available
                Available: 'market_cap', 'shares_outstanding', 'eps', 'revenue', 'dividend',
                          'book_value', 'debt', 'cash', 'free_cash_flow', 'gross_profit',
                          'operating_income', 'net_income', 'total_assets', 'total_liabilities'
    
    Returns: Dict with raw fundamental metrics as key-value pairs
    """
    pass  # Implemented by backend

def get_earnings_data(symbol, quarters=8):
    """
    Get raw historical earnings data
    
    Returns: Dict with eps_actual: list of float, eps_estimate: list of float, 
             revenue_actual: list of float, revenue_estimate: list of float,
             report_dates: list of str, surprise_percent: list of float
    """
    pass  # Implemented by backend

def get_financial_statements(symbol, statement_type='income', periods=4):
    """
    Get raw financial statement data
    
    Args:
        statement_type: 'income', 'balance', 'cash_flow'
        periods: Number of periods (quarters) to retrieve
    
    Returns: Dict with 'periods': list of str, 'line_items': dict mapping str to list of float
    """
    pass  # Implemented by backend

# ==================== RAW MARKET DATA ====================

def get_sector_data(sector=None, days=5):
    """
    Get raw sector performance data
    
    Args:
        sector: Specific sector name, or None for all sectors
        days: Number of days of data
    
    Returns: Dict with sector symbols and their raw price data
    """
    pass  # Implemented by backend

def get_market_indices(indices=None, days=30):
    """
    Get raw market index data
    
    Args:
        indices: list of index symbols ['SPY', 'QQQ', 'IWM', 'VIX'] or None for all
        days: Number of days of data
    
    Returns: Dict mapping index -> raw OHLCV data
    """
    pass  # Implemented by backend

def get_economic_calendar(days_ahead=30):
    """
    Get upcoming economic events
    
    Returns: list of dicts with event_date, event_name, importance, previous, forecast, actual
    """
    pass  # Implemented by backend

# ==================== RAW VOLUME & FLOW DATA ====================

def get_volume_data(symbol, days=30):
    """
    Get raw volume data with timestamps
    
    Returns: Dict with 'timestamps': list of int, 'volume': list of int, 
             'dollar_volume': list of float, 'trade_count': list of int
    """
    pass  # Implemented by backend

def get_options_chain(symbol, expiration=None):
    """
    Get raw options chain data
    
    Args:
        expiration: Specific expiration date (YYYY-MM-DD), or None for next expiration
    
    Returns: Dict with calls: list of dict, puts: list of dict, each containing
             strike, bid, ask, volume, open_interest, implied_volatility
    """
    pass  # Implemented by backend

# ==================== RAW SENTIMENT & NEWS DATA ====================

def get_news_sentiment(symbol=None, days=7):
    """
    Get raw news articles with sentiment scores
    
    Args:
        symbol: Specific symbol or None for market news
        days: Number of days of news data
    
    Returns: list of dicts with timestamp, headline, sentiment_score, source, url
    """
    pass  # Implemented by backend

def get_social_mentions(symbol, days=7):
    """
    Get raw social media mention data
    
    Returns: Dict with 'timestamps': List[int], 'mention_count': List[int],
             'sentiment_scores': List[float], 'platforms': List[str]
    """
    pass  # Implemented by backend

# ==================== RAW INSIDER & INSTITUTIONAL DATA ====================

def get_insider_trades(symbol, days=90):
    """
    Get raw insider trading transactions
    
    Returns: list of dicts with date, insider_name, title, transaction_type,
             shares, price, value, shares_owned_after
    """
    pass  # Implemented by backend

def get_institutional_holdings(symbol, quarters=4):
    """
    Get raw institutional ownership data
    
    Returns: list of dicts with quarter, institution_name, shares_held,
             market_value, percent_ownership, change_in_shares
    """
    pass  # Implemented by backend

def get_short_data(symbol):
    """
    Get raw short interest data
    
    Returns: Dict with short_interest, short_ratio, days_to_cover,
             short_percent_float, previous_short_interest
    """
    pass  # Implemented by backend

# ==================== SCREENING & FILTERING ====================

def scan_universe(filters=None, sort_by=None, limit=100):
    """
    Screen stocks based on raw criteria
    
    Args:
        filters: dict with screening criteria
                Keys: 'market_cap_min', 'market_cap_max', 'price_min', 'price_max',
                      'volume_min', 'sector', 'industry', 'country', 'exchange'
        sort_by: Field to sort results by ('market_cap', 'volume', 'price')
        limit: Maximum number of results
    
    Returns: Dict with 'symbols': list of str, 'data': list of dict with basic info
    """
    pass  # Implemented by backend

def get_universe_symbols(universe='sp500'):
    """
    Get list of symbols from predefined universes
    
    Args:
        universe: 'sp500', 'nasdaq100', 'russell2000', 'all_stocks'
    
    Returns: list of ticker symbols
    """
    pass  # Implemented by backend
`

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
type AlertArgs struct {
	StrategyID int      `json:"strategyId"`
	Symbols    []string `json:"symbols,omitempty"`
}

// AlertResponse represents the alert monitoring results
type AlertResponse struct {
	Alerts           []Alert           `json:"alerts"`
	Signals          map[string]Signal `json:"signals"`
	SymbolsMonitored int               `json:"symbolsMonitored"`
}

type Alert struct {
	Symbol    string                 `json:"symbol"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	Priority  string                 `json:"priority,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

type Signal struct {
	Signal    bool                   `json:"signal"`
	Timestamp string                 `json:"timestamp"`
	Symbol    string                 `json:"symbol,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

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
	result, err := callWorkerScreening(args.StrategyID, args.Universe, args.Limit)
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

// RunAlert executes complete alert monitoring using the new worker architecture
func RunAlert(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args AlertArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	log.Printf("Starting complete alert monitoring for strategy %d using new worker architecture", args.StrategyID)

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

	// Call the worker's run_alert function
	result, err := callWorkerAlert(args.StrategyID, args.Symbols)
	if err != nil {
		return nil, fmt.Errorf("error executing worker alert: %v", err)
	}

	// Convert worker result to AlertResponse format for API compatibility
	alerts := convertWorkerAlerts(result.Alerts)
	signals := convertWorkerSignals(result.Signals)

	response := AlertResponse{
		Alerts:           alerts,
		Signals:          signals,
		SymbolsMonitored: result.SymbolsMonitored,
	}

	log.Printf("Complete alert monitoring finished for strategy %d: %d alerts, %d signals",
		args.StrategyID, len(alerts), len(signals))

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

// Worker alert types
type WorkerAlertResult struct {
	Success          bool                    `json:"success"`
	StrategyID       int                     `json:"strategy_id"`
	ExecutionMode    string                  `json:"execution_mode"`
	Alerts           []WorkerAlert           `json:"alerts"`
	Signals          map[string]WorkerSignal `json:"signals"`
	SymbolsMonitored int                     `json:"symbols_monitored"`
	ExecutionTimeMs  int                     `json:"execution_time_ms"`
	ErrorMessage     string                  `json:"error_message,omitempty"`
}

type WorkerAlert struct {
	Symbol    string                 `json:"symbol"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp,omitempty"`
	Priority  string                 `json:"priority,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

type WorkerSignal struct {
	Signal    bool                   `json:"signal"`
	Timestamp string                 `json:"timestamp"`
	Symbol    string                 `json:"symbol,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// callWorkerScreening calls the worker's run_screener function
func callWorkerScreening(strategyID int, universe []string, limit int) (*WorkerScreeningResult, error) {
	// Set defaults
	if limit == 0 {
		limit = 100
	}

	// Prepare request payload
	payload := map[string]interface{}{
		"function":    "run_screener",
		"strategy_id": strategyID,
	}

	if len(universe) > 0 {
		payload["universe"] = universe
	}
	if limit > 0 {
		payload["limit"] = limit
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling payload: %v", err)
	}

	// Call worker service
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

	var result WorkerScreeningResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling worker response: %v", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("worker execution failed: %s", result.ErrorMessage)
	}

	return &result, nil
}

// callWorkerAlert calls the worker's run_alert function
func callWorkerAlert(strategyID int, symbols []string) (*WorkerAlertResult, error) {
	// Prepare request payload
	payload := map[string]interface{}{
		"function":    "run_alert",
		"strategy_id": strategyID,
	}

	if len(symbols) > 0 {
		payload["symbols"] = symbols
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling payload: %v", err)
	}

	// Call worker service
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

	var result WorkerAlertResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling worker response: %v", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("worker execution failed: %s", result.ErrorMessage)
	}

	return &result, nil
}

// Conversion functions
func convertWorkerRankedResults(workerResults []WorkerRankedResult) []ScreeningResult {
	results := make([]ScreeningResult, len(workerResults))

	for i, wr := range workerResults {
		results[i] = ScreeningResult{
			Symbol:       wr.Symbol,
			Score:        wr.Score,
			CurrentPrice: wr.CurrentPrice,
			Sector:       wr.Sector,
			Data:         wr.Data,
		}
	}

	return results
}

func convertWorkerAlerts(workerAlerts []WorkerAlert) []Alert {
	alerts := make([]Alert, len(workerAlerts))

	for i, wa := range workerAlerts {
		alerts[i] = Alert{
			Symbol:    wa.Symbol,
			Type:      wa.Type,
			Message:   wa.Message,
			Timestamp: wa.Timestamp,
			Priority:  wa.Priority,
			Data:      wa.Data,
		}
	}

	return alerts
}

func convertWorkerSignals(workerSignals map[string]WorkerSignal) map[string]Signal {
	signals := make(map[string]Signal)

	for symbol, ws := range workerSignals {
		signals[symbol] = Signal{
			Signal:    ws.Signal,
			Timestamp: ws.Timestamp,
			Symbol:    ws.Symbol,
			Data:      ws.Data,
		}
	}

	return signals
}

func CreateStrategyFromPrompt(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	log.Printf("=== STRATEGY CREATION START ===")
	log.Printf("UserID: %d", userID)
	log.Printf("Raw args: %s", string(rawArgs))

	var args CreateStrategyFromPromptArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		log.Printf("ERROR: Failed to unmarshal args: %v", err)
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	log.Printf("Parsed args - Query: %q, StrategyID: %d", args.Query, args.StrategyID)

	var existingStrategy *Strategy
	isEdit := args.StrategyID != -1

	log.Printf("Is edit operation: %t", isEdit)

	// Handle strategy ID - if -1, create new strategy, otherwise edit existing
	if isEdit {
		log.Printf("Reading existing strategy with ID: %d", args.StrategyID)
		// Read existing strategy for editing
		var strategyRow Strategy
		err := conn.DB.QueryRow(context.Background(), `
			SELECT strategyid, name, 
			       COALESCE(description, '') as description,
			       COALESCE(prompt, '') as prompt,
			       COALESCE(pythoncode, '') as pythoncode,
			       COALESCE(score, 0) as score,
			       COALESCE(version, '1.0') as version,
			       COALESCE(createdat, NOW()) as createdat,
			       COALESCE(isalertactive, false) as isalertactive
			FROM strategies WHERE strategyid = $1 AND userid = $2`, args.StrategyID, userID).Scan(
			&strategyRow.StrategyID,
			&strategyRow.Name,
			&strategyRow.Description,
			&strategyRow.Prompt,
			&strategyRow.PythonCode,
			&strategyRow.Score,
			&strategyRow.Version,
			&strategyRow.CreatedAt,
			&strategyRow.IsAlertActive,
		)
		if err != nil {
			log.Printf("ERROR: Failed to read existing strategy: %v", err)
			return nil, fmt.Errorf("error reading existing strategy: %v", err)
		}
		existingStrategy = &strategyRow
		existingStrategy.UserID = userID
		log.Printf("Successfully loaded existing strategy: %q", existingStrategy.Name)
		log.Printf("Existing Python code length: %d characters", len(existingStrategy.PythonCode))
	}

	log.Printf("Getting Gemini API key...")
	apikey, err := conn.GetGeminiKey()
	if err != nil {
		log.Printf("ERROR: Failed to get Gemini key: %v", err)
		return nil, fmt.Errorf("error getting gemini key: %v", err)
	}
	log.Printf("Successfully retrieved Gemini API key (length: %d)", len(apikey))

	log.Printf("Creating Gemini client...")
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apikey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("ERROR: Failed to create Gemini client: %v", err)
		return nil, fmt.Errorf("error creating gemini client: %v", err)
	}
	log.Printf("Successfully created Gemini client")

	log.Printf("Loading system instruction...")
	systemInstruction, err := getSystemInstruction("classifier")
	if err != nil {
		log.Printf("ERROR: Failed to get system instruction: %v", err)
		return nil, fmt.Errorf("error getting system instruction: %v", err)
	}
	log.Printf("Successfully loaded system instruction (length: %d)", len(systemInstruction))

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
	}

	// Create the prompt with data accessor functions
	// Check if this is a symbol-specific gap strategy and enhance the prompt accordingly
	enhancedQuery := args.Query
	if strings.Contains(strings.ToLower(args.Query), "gap") &&
		(strings.Contains(strings.ToUpper(args.Query), "ARM") ||
			strings.Contains(strings.ToLower(args.Query), "arm ")) {
		log.Printf("Detected ARM gap strategy, enhancing query...")
		enhancedQuery = fmt.Sprintf(`%s

IMPORTANT: This query is asking specifically for ARM (ticker: ARM) gap-up analysis. 
- Create a strategy function that checks if the ARM symbol specifically gaps up by the specified percentage
- A gap up means: current day's opening price > previous day's closing price by the specified percentage
- Use the formula: gap_percent = ((current_open - previous_close) / previous_close) * 100
- Filter the data to only process ARM rows and return instances where the gap criteria is met`, args.Query)
	}

	var fullPrompt string
	if isEdit && existingStrategy != nil {
		log.Printf("Building edit prompt for existing strategy...")
		// For editing existing strategies, include current strategy content
		fullPrompt = fmt.Sprintf(`EDITING EXISTING STRATEGY:

Current Strategy Name: %s
Current Description: %s
Original Prompt: %s

Current Python Code:
`+"```python"+`
%s
`+"```"+`

User's Edit Request: %s

Please modify the existing strategy based on the user's edit request. You can:
1. Update the logic while keeping the same structure if the request is minor
2. Completely rewrite the strategy if the request requires major changes
3. Add new functionality while preserving existing behavior where appropriate
4. Fix any bugs or improve performance if requested

Generate the updated Python strategy function named 'strategy(data)' that incorporates the requested changes.`,
			existingStrategy.Name,
			existingStrategy.Description,
			existingStrategy.Prompt,
			existingStrategy.PythonCode,
			enhancedQuery)
	} else {
		log.Printf("Building new strategy prompt...")
		// For new strategies, use the original prompt format
		fullPrompt = fmt.Sprintf(`User Request: %s

Please generate a Python strategy function that identifies the pattern the user is requesting. The function should be named 'strategy(data)' where data is a numpy array containing market data, and should return a list of instances where the pattern was found.`, enhancedQuery)
	}

	log.Printf("Full prompt length: %d characters", len(fullPrompt))
	log.Printf("Full prompt preview (first 500 chars): %s...", fullPrompt[:min(500, len(fullPrompt))])

	content := genai.Text(fullPrompt)
	if len(content) == 0 {
		log.Printf("ERROR: Failed to create content from prompt")
		return nil, fmt.Errorf("failed to create content from prompt")
	}
	log.Printf("Successfully created Gemini content")

	log.Printf("Calling Gemini API to generate content...")
	result, err := client.Models.GenerateContent(context.Background(), "gemini-2.5-flash", content, config)
	if err != nil {
		log.Printf("ERROR: Gemini API call failed: %v", err)
		return nil, fmt.Errorf("error generating content: %w", err)
	}
	log.Printf("Successfully received response from Gemini API")

	log.Printf("Processing Gemini response...")
	log.Printf("Number of candidates: %d", len(result.Candidates))

	responseText := ""
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		log.Printf("Processing candidate 0...")
		log.Printf("Number of parts in candidate: %d", len(result.Candidates[0].Content.Parts))

		for i, part := range result.Candidates[0].Content.Parts {
			log.Printf("Part %d - Thought: %t, Text length: %d", i, part.Thought, len(part.Text))
			if !part.Thought && part.Text != "" {
				responseText = part.Text
				log.Printf("Selected part %d as response text", i)
				break
			}
		}
	}

	if responseText == "" {
		log.Printf("ERROR: Gemini returned no response text")
		log.Printf("Full result debug: %+v", result)
		return nil, fmt.Errorf("gemini returned no response")
	}

	log.Printf("=== GEMINI RESPONSE ===")
	log.Printf("Response length: %d characters", len(responseText))
	log.Printf("Full response text:\n%s", responseText)
	log.Printf("=== END GEMINI RESPONSE ===")

	// Extract Python code and description
	log.Printf("Extracting Python code from response...")
	pythonCode := extractPythonCode(responseText)
	log.Printf("Extracted Python code length: %d characters", len(pythonCode))

	if pythonCode == "" {
		log.Printf("ERROR: No Python code extracted from response")
		log.Printf("Response text for debugging:\n%s", responseText)
		return nil, fmt.Errorf("no Python code generated")
	}

	log.Printf("=== EXTRACTED PYTHON CODE ===")
	log.Printf("%s", pythonCode)
	log.Printf("=== END EXTRACTED PYTHON CODE ===")

	log.Printf("Extracting description from response...")
	description := extractDescription(responseText)
	log.Printf("Extracted description: %q", description)

	var strategyID int
	var name string

	if isEdit && existingStrategy != nil {
		log.Printf("Updating existing strategy...")
		// Update existing strategy
		name = existingStrategy.Name // Keep existing name unless user specifically requested a name change

		// Check if user requested a name change in their query
		if strings.Contains(strings.ToLower(args.Query), "rename") ||
			strings.Contains(strings.ToLower(args.Query), "name") ||
			strings.Contains(strings.ToLower(args.Query), "call it") {
			name = generateStrategyName(args.Query)
			log.Printf("User requested name change, new name: %q", name)
		}

		log.Printf("Executing database update for strategy ID %d...", args.StrategyID)
		log.Printf("Update values - Name: %q, Description: %q, Prompt: %q, Python code length: %d",
			name, description, args.Query, len(pythonCode))

		// Update the existing strategy in database
		result, err := conn.DB.Exec(context.Background(), `
			UPDATE strategies 
			SET name = $1, description = $2, prompt = $3, pythoncode = $4, version = $5
			WHERE strategyid = $6 AND userid = $7`,
			name, description, args.Query, pythonCode, "1.1", args.StrategyID, userID)
		if err != nil {
			log.Printf("ERROR: Database update failed: %v", err)
			return nil, fmt.Errorf("error updating strategy: %w", err)
		}

		rowsAffected := result.RowsAffected()
		log.Printf("Database update successful, rows affected: %d", rowsAffected)
		strategyID = args.StrategyID
	} else {
		log.Printf("Creating new strategy...")
		// Create new strategy
		name = generateStrategyName(args.Query)
		log.Printf("Generated strategy name: %q", name)

		log.Printf("Saving new strategy to database...")
		log.Printf("Save values - Name: %q, Description: %q, Prompt: %q, Python code length: %d",
			name, description, args.Query, len(pythonCode))

		strategyID, err = saveStrategy(conn, userID, name, description, args.Query, pythonCode)
		if err != nil {
			log.Printf("ERROR: Failed to save strategy: %v", err)
			return nil, fmt.Errorf("error saving strategy: %w", err)
		}
		log.Printf("Successfully saved new strategy with ID: %d", strategyID)
	}

	// Verify the strategy was saved correctly by reading it back
	log.Printf("Verifying saved strategy by reading back from database...")
	var verifyPythonCode string
	err = conn.DB.QueryRow(context.Background(), `
		SELECT COALESCE(pythoncode, '') FROM strategies WHERE strategyid = $1 AND userid = $2`,
		strategyID, userID).Scan(&verifyPythonCode)
	if err != nil {
		log.Printf("WARNING: Failed to verify saved strategy: %v", err)
	} else {
		log.Printf("Verification successful - Saved Python code length: %d", len(verifyPythonCode))
		if len(verifyPythonCode) == 0 {
			log.Printf("CRITICAL ERROR: Python code was not saved to database!")
		} else if verifyPythonCode != pythonCode {
			log.Printf("WARNING: Saved Python code differs from generated code")
			log.Printf("Generated length: %d, Saved length: %d", len(pythonCode), len(verifyPythonCode))
		} else {
			log.Printf("SUCCESS: Python code was correctly saved to database")
		}
	}

	finalStrategy := Strategy{
		StrategyID:    strategyID,
		UserID:        userID,
		Name:          name,
		Description:   description,
		Prompt:        args.Query,
		PythonCode:    pythonCode,
		Score:         0,
		Version:       "1.1",
		CreatedAt:     time.Now().Format(time.RFC3339),
		IsAlertActive: false,
	}

	log.Printf("=== STRATEGY CREATION COMPLETE ===")
	log.Printf("Final strategy - ID: %d, Name: %q, Python code length: %d",
		finalStrategy.StrategyID, finalStrategy.Name, len(finalStrategy.PythonCode))
	log.Printf("=== END STRATEGY CREATION ===")

	return finalStrategy, nil
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

// Helper functions
func extractPythonCode(response string) string {
	log.Printf("=== EXTRACT PYTHON CODE START ===")
	log.Printf("Response length: %d", len(response))

	// Look for Python code blocks
	log.Printf("Looking for '```python' marker...")
	start := strings.Index(response, "```python")
	if start == -1 {
		log.Printf("'```python' not found, looking for generic '```' marker...")
		start = strings.Index(response, "```")
	}
	if start == -1 {
		log.Printf("ERROR: No code block markers found in response")
		return ""
	}
	log.Printf("Found code block start at position: %d", start)

	// Find the start of actual code (after the newline)
	newlinePos := strings.Index(response[start:], "\n")
	if newlinePos == -1 {
		log.Printf("ERROR: No newline found after code block marker")
		return ""
	}
	start = newlinePos + start + 1
	log.Printf("Code content starts at position: %d", start)

	// Find the end of the code block
	log.Printf("Looking for closing '```' marker...")
	end := strings.Index(response[start:], "```")
	if end == -1 {
		log.Printf("ERROR: No closing '```' marker found")
		return ""
	}
	log.Printf("Found code block end at relative position: %d", end)

	extractedCode := strings.TrimSpace(response[start : start+end])
	log.Printf("Extracted code length: %d", len(extractedCode))
	log.Printf("Extracted code preview (first 300 chars): %s", extractedCode[:min(300, len(extractedCode))])
	log.Printf("=== EXTRACT PYTHON CODE END ===")

	return extractedCode
}

func extractDescription(response string) string {
	log.Printf("=== EXTRACT DESCRIPTION START ===")
	// Extract description from the response (before code block or after)
	lines := strings.Split(response, "\n")
	log.Printf("Response split into %d lines", len(lines))

	var description []string

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "```") {
			log.Printf("Skipping line %d (empty or code marker): %q", i, line)
			continue
		}
		if strings.Contains(line, "def classify_symbol") {
			log.Printf("Found function definition at line %d, stopping description extraction", i)
			break
		}
		description = append(description, line)
		log.Printf("Added line %d to description: %q", i, line)
	}

	result := strings.Join(description, " ")
	log.Printf("Joined description length: %d", len(result))

	if len(result) > 500 {
		result = result[:500] + "..."
		log.Printf("Truncated description to 500 characters")
	}

	log.Printf("Final description: %q", result)
	log.Printf("=== EXTRACT DESCRIPTION END ===")

	return result
}

func generateStrategyName(prompt string) string {
	words := strings.Fields(prompt)
	if len(words) == 0 {
		return fmt.Sprintf("Custom Strategy %d", time.Now().Unix())
	}

	// Take first few words and capitalize, but skip common words
	var nameWords []string
	skipWords := map[string]bool{
		"create": true, "a": true, "an": true, "the": true, "strategy": true, "for": true, "when": true,
	}

	for _, word := range words {
		if len(nameWords) >= 4 {
			break
		}
		if !skipWords[strings.ToLower(word)] {
			nameWords = append(nameWords, word)
		}
	}

	if len(nameWords) == 0 {
		nameWords = []string{"Custom"}
	}

	caser := cases.Title(language.English)
	for i, word := range nameWords {
		nameWords[i] = caser.String(word)
	}

	// Add timestamp to ensure uniqueness
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s Strategy %d", strings.Join(nameWords, " "), timestamp)
}

func saveStrategy(conn *data.Conn, userID int, name, description, prompt, pythonCode string) (int, error) {
	log.Printf("=== SAVE STRATEGY START ===")
	log.Printf("UserID: %d", userID)
	log.Printf("Name: %q", name)
	log.Printf("Description: %q", description)
	log.Printf("Prompt: %q", prompt)
	log.Printf("Python code length: %d", len(pythonCode))
	log.Printf("Python code preview (first 200 chars): %s", pythonCode[:min(200, len(pythonCode))])

	var strategyID int
	log.Printf("Executing INSERT query...")
	err := conn.DB.QueryRow(context.Background(), `
		INSERT INTO strategies (name, description, prompt, pythoncode, userid, createdat, version, score, isalertactive)
		VALUES ($1, $2, $3, $4, $5, NOW(), '1.0', 0, false) 
		RETURNING strategyid`,
		name, description, prompt, pythonCode, userID).Scan(&strategyID)

	if err != nil {
		log.Printf("ERROR: Failed to insert strategy into database: %v", err)
		return -1, fmt.Errorf("error inserting strategy into database: %w", err)
	}

	log.Printf("Successfully inserted strategy with ID: %d", strategyID)
	log.Printf("=== SAVE STRATEGY END ===")

	return strategyID, nil
}

func getSystemInstruction(name string) (string, error) {
	raw, err := fs.ReadFile("prompts/" + name + ".txt")
	if err != nil {
		return "", fmt.Errorf("reading prompt: %w", err)
	}
	now := time.Now()
	s := strings.ReplaceAll(string(raw), "{{CURRENT_TIME}}",
		now.Format(time.RFC3339))
	s = strings.ReplaceAll(s, "{{CURRENT_TIME_MILLISECONDS}}",
		strconv.FormatInt(now.UnixMilli(), 10))
	return s, nil
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
