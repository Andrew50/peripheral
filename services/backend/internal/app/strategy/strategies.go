package strategy

import (
	"backend/internal/data"
	"context"
	"embed"
	"encoding/json"
	"fmt"
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
	Prompt string `json:"prompt"`
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
from typing import List, Dict, Tuple, Union, Optional

# ==================== RAW DATA RETRIEVAL ONLY ====================
# Note: You must implement your own technical indicators using the raw data below

def get_price_data(symbol: str, timeframe: str = '1d', days: int = 30, 
                  extended_hours: bool = False, start_time: str = None, end_time: str = None) -> Dict:
    """
    Get raw OHLCV price data for a symbol
    
    Args:
        symbol: Stock ticker symbol
        timeframe: '1m', '5m', '15m', '30m', '1h', '4h', '1d', '1w', '1M'
        days: Number of days of historical data
        extended_hours: Include pre/after market data (for intraday only)
        start_time: Time filter start (e.g., '09:30:00')
        end_time: Time filter end (e.g., '16:00:00')
    
    Returns: Dict with 'timestamps': List[int], 'open': List[float], 'high': List[float], 
             'low': List[float], 'close': List[float], 'volume': List[int], 
             'extended_hours': List[bool]
    """
    pass  # Implemented by backend

def get_historical_data(symbol: str, timeframe: str = '1d', periods: int = 100, offset: int = 0) -> Dict:
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

def get_security_info(symbol: str) -> Dict:
    """
    Get basic security metadata
    
    Returns: Dict with securityid, ticker, name, sector, industry, market, 
             primary_exchange, locale, active, cik, composite_figi, share_class_figi
    """
    pass  # Implemented by backend

def get_multiple_symbols_data(symbols: List[str], timeframe: str = '1d', days: int = 30) -> Dict[str, Dict]:
    """
    Get raw price data for multiple symbols efficiently
    
    Args:
        symbols: List of ticker symbols
        timeframe: Data frequency
        days: Number of days of data
    
    Returns: Dict mapping symbol -> raw data dict (same format as get_price_data)
    """
    pass  # Implemented by backend

# ==================== RAW FUNDAMENTAL DATA ====================

def get_fundamental_data(symbol: str, metrics: Optional[List[str]] = None) -> Dict:
    """
    Get raw fundamental data for a symbol
    
    Args:
        metrics: List of specific metrics to retrieve, or None for all available
                Available: 'market_cap', 'shares_outstanding', 'eps', 'revenue', 'dividend',
                          'book_value', 'debt', 'cash', 'free_cash_flow', 'gross_profit',
                          'operating_income', 'net_income', 'total_assets', 'total_liabilities'
    
    Returns: Dict with raw fundamental metrics as key-value pairs
    """
    pass  # Implemented by backend

def get_earnings_data(symbol: str, quarters: int = 8) -> Dict:
    """
    Get raw historical earnings data
    
    Returns: Dict with eps_actual: List[float], eps_estimate: List[float], 
             revenue_actual: List[float], revenue_estimate: List[float],
             report_dates: List[str], surprise_percent: List[float]
    """
    pass  # Implemented by backend

def get_financial_statements(symbol: str, statement_type: str = 'income', periods: int = 4) -> Dict:
    """
    Get raw financial statement data
    
    Args:
        statement_type: 'income', 'balance', 'cash_flow'
        periods: Number of periods (quarters) to retrieve
    
    Returns: Dict with 'periods': List[str], 'line_items': Dict[str, List[float]]
    """
    pass  # Implemented by backend

# ==================== RAW MARKET DATA ====================

def get_sector_data(sector: str = None, days: int = 5) -> Dict:
    """
    Get raw sector performance data
    
    Args:
        sector: Specific sector name, or None for all sectors
        days: Number of days of data
    
    Returns: Dict with sector symbols and their raw price data
    """
    pass  # Implemented by backend

def get_market_indices(indices: List[str] = None, days: int = 30) -> Dict[str, Dict]:
    """
    Get raw market index data
    
    Args:
        indices: List of index symbols ['SPY', 'QQQ', 'IWM', 'VIX'] or None for all
        days: Number of days of data
    
    Returns: Dict mapping index -> raw OHLCV data
    """
    pass  # Implemented by backend

def get_economic_calendar(days_ahead: int = 30) -> List[Dict]:
    """
    Get upcoming economic events
    
    Returns: List of dicts with event_date, event_name, importance, previous, forecast, actual
    """
    pass  # Implemented by backend

# ==================== RAW VOLUME & FLOW DATA ====================

def get_volume_data(symbol: str, days: int = 30) -> Dict:
    """
    Get raw volume data with timestamps
    
    Returns: Dict with 'timestamps': List[int], 'volume': List[int], 
             'dollar_volume': List[float], 'trade_count': List[int]
    """
    pass  # Implemented by backend

def get_options_chain(symbol: str, expiration: str = None) -> Dict:
    """
    Get raw options chain data
    
    Args:
        expiration: Specific expiration date (YYYY-MM-DD), or None for next expiration
    
    Returns: Dict with calls: List[Dict], puts: List[Dict], each containing
             strike, bid, ask, volume, open_interest, implied_volatility
    """
    pass  # Implemented by backend

# ==================== RAW SENTIMENT & NEWS DATA ====================

def get_news_sentiment(symbol: str = None, days: int = 7) -> List[Dict]:
    """
    Get raw news articles with sentiment scores
    
    Args:
        symbol: Specific symbol or None for market news
        days: Number of days of news data
    
    Returns: List of dicts with timestamp, headline, sentiment_score, source, url
    """
    pass  # Implemented by backend

def get_social_mentions(symbol: str, days: int = 7) -> Dict:
    """
    Get raw social media mention data
    
    Returns: Dict with 'timestamps': List[int], 'mention_count': List[int],
             'sentiment_scores': List[float], 'platforms': List[str]
    """
    pass  # Implemented by backend

# ==================== RAW INSIDER & INSTITUTIONAL DATA ====================

def get_insider_trades(symbol: str, days: int = 90) -> List[Dict]:
    """
    Get raw insider trading transactions
    
    Returns: List of dicts with date, insider_name, title, transaction_type,
             shares, price, value, shares_owned_after
    """
    pass  # Implemented by backend

def get_institutional_holdings(symbol: str, quarters: int = 4) -> List[Dict]:
    """
    Get raw institutional ownership data
    
    Returns: List of dicts with quarter, institution_name, shares_held,
             market_value, percent_ownership, change_in_shares
    """
    pass  # Implemented by backend

def get_short_data(symbol: str) -> Dict:
    """
    Get raw short interest data
    
    Returns: Dict with short_interest, short_ratio, days_to_cover,
             short_percent_float, previous_short_interest
    """
    pass  # Implemented by backend

# ==================== SCREENING & FILTERING ====================

def scan_universe(filters: Dict = None, sort_by: str = None, limit: int = 100) -> Dict:
    """
    Screen stocks based on raw criteria
    
    Args:
        filters: dict with screening criteria
                Keys: 'market_cap_min', 'market_cap_max', 'price_min', 'price_max',
                      'volume_min', 'sector', 'industry', 'country', 'exchange'
        sort_by: Field to sort results by ('market_cap', 'volume', 'price')
        limit: Maximum number of results
    
    Returns: Dict with 'symbols': List[str], 'data': List[Dict] with basic info
    """
    pass  # Implemented by backend

def get_universe_symbols(universe: str = 'sp500') -> List[str]:
    """
    Get list of symbols from predefined universes
    
    Args:
        universe: 'sp500', 'nasdaq100', 'russell2000', 'all_stocks'
    
    Returns: List of ticker symbols
    """
    pass  # Implemented by backend
`

func CreateStrategyFromPrompt(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args CreateStrategyFromPromptArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	apikey, err := conn.GetGeminiKey()
	if err != nil {
		return nil, fmt.Errorf("error getting gemini key: %v", err)
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apikey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating gemini client: %v", err)
	}

	systemInstruction, err := getSystemInstruction("classifier")
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %v", err)
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
	}

	// Create the prompt with data accessor functions
	fullPrompt := fmt.Sprintf(`%s

User Request: %s

Please generate a Python classifier function that uses the above data accessor functions to identify the pattern the user is requesting. The function should be named 'classify_symbol(symbol)' and return a boolean indicating if the symbol matches the criteria.`, dataAccessorFunctions, args.Prompt)

	content := genai.Text(fullPrompt)
	if len(content) == 0 {
		return nil, fmt.Errorf("failed to create content from prompt")
	}

	result, err := client.Models.GenerateContent(context.Background(), "gemini-2.5-flash", content, config)
	if err != nil {
		return nil, fmt.Errorf("error generating content: %w", err)
	}

	responseText := ""
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		for _, part := range result.Candidates[0].Content.Parts {
			if !part.Thought && part.Text != "" {
				responseText = part.Text
				break
			}
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("gemini returned no response")
	}

	// Extract Python code and description
	pythonCode := extractPythonCode(responseText)
	description := extractDescription(responseText)

	if pythonCode == "" {
		return nil, fmt.Errorf("no Python code generated")
	}

	// Generate a name from the prompt
	name := generateStrategyName(args.Prompt)

	// Save to database
	strategyID, err := saveStrategy(conn, userID, name, description, args.Prompt, pythonCode)
	if err != nil {
		return nil, fmt.Errorf("error saving strategy: %w", err)
	}

	return Strategy{
		StrategyID:    strategyID,
		UserID:        userID,
		Name:          name,
		Description:   description,
		Prompt:        args.Prompt,
		PythonCode:    pythonCode,
		Score:         0,
		Version:       "1.0",
		CreatedAt:     time.Now().Format(time.RFC3339),
		IsAlertActive: false,
	}, nil
}

func GetStrategies(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
		SELECT strategyid, name, description, prompt, pythoncode, 
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
	// Look for Python code blocks
	start := strings.Index(response, "```python")
	if start == -1 {
		start = strings.Index(response, "```")
	}
	if start == -1 {
		return ""
	}

	start = strings.Index(response[start:], "\n") + start + 1
	end := strings.Index(response[start:], "```")
	if end == -1 {
		return ""
	}

	return strings.TrimSpace(response[start : start+end])
}

func extractDescription(response string) string {
	// Extract description from the response (before code block or after)
	lines := strings.Split(response, "\n")
	var description []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		if strings.Contains(line, "def classify_symbol") {
			break
		}
		description = append(description, line)
	}

	result := strings.Join(description, " ")
	if len(result) > 500 {
		result = result[:500] + "..."
	}

	return result
}

func generateStrategyName(prompt string) string {
	words := strings.Fields(prompt)
	if len(words) == 0 {
		return "Custom Strategy"
	}

	// Take first few words and capitalize
	nameWords := words
	if len(words) > 4 {
		nameWords = words[:4]
	}

	caser := cases.Title(language.English)
	for i, word := range nameWords {
		nameWords[i] = caser.String(word)
	}

	return strings.Join(nameWords, " ") + " Strategy"
}

func saveStrategy(conn *data.Conn, userID int, name, description, prompt, pythonCode string) (int, error) {
	var strategyID int
	err := conn.DB.QueryRow(context.Background(), `
		INSERT INTO strategies (name, description, prompt, pythoncode, userid, createdat, version, score, isalertactive)
		VALUES ($1, $2, $3, $4, $5, NOW(), '1.0', 0, false) 
		RETURNING strategyid`,
		name, description, prompt, pythonCode, userID).Scan(&strategyID)

	if err != nil {
		return -1, fmt.Errorf("error inserting strategy into database: %w", err)
	}

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
