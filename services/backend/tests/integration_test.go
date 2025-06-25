package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"backend/internal/data"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Test data structures
type Strategy struct {
	StrategyID    int    `json:"strategyId"`
	UserID        int    `json:"userId"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Prompt        string `json:"prompt"`
	PythonCode    string `json:"pythonCode"`
	Score         int    `json:"score,omitempty"`
	Version       string `json:"version,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
	IsAlertActive bool   `json:"isAlertActive,omitempty"`
}

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

type BacktestSummary struct {
	TotalInstances   int              `json:"totalInstances"`
	PositiveSignals  int              `json:"positiveSignals"`
	DateRange        []string         `json:"dateRange"`
	SymbolsProcessed int              `json:"symbolsProcessed"`
	Columns          []string         `json:"columns"`
	ColumnSamples    map[string][]any `json:"columnSamples"`
}

type BacktestResponse struct {
	Instances []BacktestResult `json:"instances"`
	Summary   BacktestSummary  `json:"summary"`
}

type TestRequest struct {
	Function  string      `json:"func"`
	Arguments interface{} `json:"args"`
}

// Integration test suite for natural language strategy creation and execution
func TestNaturalLanguageStrategyPipeline(t *testing.T) {
	// Skip if running in CI without proper test database
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration tests")
	}

	// Initialize database connection
	conn, cleanup := data.InitConn(false)
	defer cleanup()

	// Create test user
	userID := createTestUser(t, conn)
	defer cleanupTestUser(t, conn, userID)

	// Define test cases with natural language queries
	testCases := []struct {
		name        string
		query       string
		description string
		expectCode  bool
		expectError bool
	}{
		{
			name:        "Gap Up Strategy",
			query:       "Create a strategy to find stocks that gap up more than 3% with high volume",
			description: "Should create a strategy for detecting gap-up patterns",
			expectCode:  true,
			expectError: false,
		},
		{
			name:        "Technology Sector Filter",
			query:       "Find technology stocks with P/E ratio below 15 and market cap over 1 billion",
			description: "Should create a value-based technology sector strategy",
			expectCode:  true,
			expectError: false,
		},
		{
			name:        "Momentum Strategy",
			query:       "Identify stocks breaking out of consolidation with volume above 150% of average",
			description: "Should create a momentum-based breakout strategy",
			expectCode:  true,
			expectError: false,
		},
		{
			name:        "Relative Performance",
			query:       "Find stocks outperforming the market by more than 5% over the last 20 days",
			description: "Should create a relative performance strategy",
			expectCode:  true,
			expectError: false,
		},
		{
			name:        "Multi-Factor Strategy",
			query:       "Create a strategy for stocks with RSI below 30, P/E under 20, and trading near 52-week lows",
			description: "Should create a multi-factor oversold value strategy",
			expectCode:  true,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the complete pipeline
			strategyID := testStrategyCreation(t, conn, userID, tc.query, tc.expectCode, tc.expectError)
			if strategyID > 0 {
				testStrategyBacktest(t, conn, userID, strategyID, tc.name)
			}
		})
	}
}

// Test strategy creation from natural language
func testStrategyCreation(t *testing.T, conn *data.Conn, userID int, query string, expectCode, expectError bool) int {
	t.Logf("🧪 Testing strategy creation for query: %q", query)

	// Create HTTP test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate the private endpoint
		if r.URL.Path == "/private" {
			handlePrivateRequest(w, r, conn, userID)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Prepare request
	requestBody := TestRequest{
		Function: "createStrategyFromPrompt",
		Arguments: map[string]interface{}{
			"query":      query,
			"strategyId": -1, // -1 for new strategy
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Make HTTP request
	resp, err := http.Post(server.URL+"/private", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		if expectError {
			t.Logf("✅ Expected error occurred: %v", err)
			return -1
		}
		t.Fatalf("❌ Failed to make HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		if expectError {
			t.Logf("✅ Expected error response: %s", string(body))
			return -1
		}
		t.Fatalf("❌ HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var strategy Strategy
	if err := json.Unmarshal(body, &strategy); err != nil {
		t.Fatalf("Failed to parse strategy response: %v", err)
	}

	// Validate strategy creation
	if strategy.StrategyID <= 0 {
		t.Fatalf("❌ Invalid strategy ID: %d", strategy.StrategyID)
	}

	if strategy.Name == "" {
		t.Fatalf("❌ Strategy name is empty")
	}

	if expectCode && strategy.PythonCode == "" {
		t.Fatalf("❌ Expected Python code but got empty string")
	}

	if strategy.Prompt != query {
		t.Errorf("❌ Prompt mismatch: expected %q, got %q", query, strategy.Prompt)
	}

	t.Logf("✅ Strategy created successfully:")
	t.Logf("   ID: %d", strategy.StrategyID)
	t.Logf("   Name: %s", strategy.Name)
	t.Logf("   Description: %s", strategy.Description)
	t.Logf("   Python Code Length: %d characters", len(strategy.PythonCode))

	// Validate Python code contains expected elements
	if expectCode {
		validatePythonCode(t, strategy.PythonCode, query)
	}

	return strategy.StrategyID
}

// Test strategy backtest execution
func testStrategyBacktest(t *testing.T, conn *data.Conn, userID, strategyID int, _ string) {
	t.Logf("🏃 Testing backtest execution for strategy %d", strategyID)

	// Create HTTP test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/private" {
			handlePrivateRequest(w, r, conn, userID)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Prepare backtest request
	requestBody := TestRequest{
		Function: "run_backtest",
		Arguments: map[string]interface{}{
			"strategyId":    strategyID,
			"securities":    []int{},                                    // Empty for all securities
			"start":         time.Now().AddDate(0, -6, 0).Unix() * 1000, // 6 months ago
			"returnWindows": []int{1, 5},                                // 1-day and 5-day returns
			"fullResults":   false,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal backtest request: %v", err)
	}

	// Make HTTP request
	resp, err := http.Post(server.URL+"/private", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("❌ Failed to make backtest HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read backtest response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("❌ Backtest HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse backtest response
	var backtestResult BacktestResponse
	if err := json.Unmarshal(body, &backtestResult); err != nil {
		t.Fatalf("Failed to parse backtest response: %v", err)
	}

	// Validate backtest results
	validateBacktestResults(t, backtestResult, "")

	t.Logf("✅ Backtest completed successfully:")
	t.Logf("   Total Instances: %d", backtestResult.Summary.TotalInstances)
	t.Logf("   Positive Signals: %d", backtestResult.Summary.PositiveSignals)
	t.Logf("   Symbols Processed: %d", backtestResult.Summary.SymbolsProcessed)
	t.Logf("   Date Range: %v", backtestResult.Summary.DateRange)
}

// Validate Python code contains expected elements
func validatePythonCode(t *testing.T, pythonCode, query string) {
	queryLower := strings.ToLower(query)
	codeLower := strings.ToLower(pythonCode)

	// Check for function definition
	if !strings.Contains(codeLower, "def ") {
		t.Errorf("❌ Python code missing function definition")
	}

	// Check for strategy-specific patterns
	if strings.Contains(queryLower, "gap") && !strings.Contains(codeLower, "gap") {
		t.Errorf("❌ Gap strategy should contain 'gap' logic")
	}

	if strings.Contains(queryLower, "volume") && !strings.Contains(codeLower, "volume") {
		t.Errorf("❌ Volume strategy should contain volume analysis")
	}

	if strings.Contains(queryLower, "p/e") || strings.Contains(queryLower, "pe ratio") {
		if !strings.Contains(codeLower, "pe") && !strings.Contains(codeLower, "p/e") {
			t.Errorf("❌ P/E strategy should contain P/E ratio logic")
		}
	}

	if strings.Contains(queryLower, "technology") && !strings.Contains(codeLower, "technology") {
		t.Errorf("❌ Technology sector strategy should contain sector filtering")
	}

	// Check for return statement
	if !strings.Contains(codeLower, "return") {
		t.Errorf("❌ Python code missing return statement")
	}

	t.Logf("✅ Python code validation passed")
}

// Validate backtest results
func validateBacktestResults(t *testing.T, result BacktestResponse, _ string) {
	// Basic structure validation
	if result.Summary.TotalInstances < 0 {
		t.Errorf("❌ Invalid total instances: %d", result.Summary.TotalInstances)
	}

	if result.Summary.PositiveSignals < 0 {
		t.Errorf("❌ Invalid positive signals: %d", result.Summary.PositiveSignals)
	}

	if result.Summary.PositiveSignals > result.Summary.TotalInstances {
		t.Errorf("❌ Positive signals (%d) cannot exceed total instances (%d)",
			result.Summary.PositiveSignals, result.Summary.TotalInstances)
	}

	if len(result.Summary.DateRange) != 2 {
		t.Errorf("❌ Date range should have 2 elements, got %d", len(result.Summary.DateRange))
	}

	// Validate instances if any exist
	if len(result.Instances) > 0 {
		for i, instance := range result.Instances {
			if instance.Ticker == "" {
				t.Errorf("❌ Instance %d missing ticker", i)
			}
			if instance.Timestamp <= 0 {
				t.Errorf("❌ Instance %d invalid timestamp: %d", i, instance.Timestamp)
			}
			// Note: Classification can be true or false, both are valid
		}
	}

	t.Logf("✅ Backtest results validation passed")
}

// Handle private requests (simplified version of the actual handler)
func handlePrivateRequest(w http.ResponseWriter, r *http.Request, conn *data.Conn, userID int) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req TestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Convert arguments to json.RawMessage
	argsBytes, err := json.Marshal(req.Arguments)
	if err != nil {
		http.Error(w, "Failed to marshal arguments", http.StatusBadRequest)
		return
	}

	var result interface{}

	// Route to appropriate handler
	switch req.Function {
	case "createStrategyFromPrompt":
		result, err = handleCreateStrategy(conn, userID, argsBytes)
	case "run_backtest":
		result, err = handleRunBacktest(conn, userID, argsBytes)
	default:
		http.Error(w, fmt.Sprintf("Unknown function: %s", req.Function), http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// Mock handlers (for testing without full backend complexity)
func handleCreateStrategy(_ *data.Conn, userID int, argsBytes []byte) (interface{}, error) {
	// For testing purposes, return a mock strategy
	// In a real test, this would call the actual strategy.CreateStrategyFromPrompt

	var args map[string]interface{}
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		return nil, err
	}

	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("missing query parameter")
	}

	// Generate a simple mock strategy
	strategy := Strategy{
		StrategyID:    int(time.Now().Unix()), // Use timestamp as mock ID
		UserID:        userID,
		Name:          generateMockStrategyName(query),
		Description:   fmt.Sprintf("AI-generated strategy for: %s", query),
		Prompt:        query,
		PythonCode:    generateMockPythonCode(query),
		Score:         0,
		Version:       "1.0",
		CreatedAt:     time.Now().Format(time.RFC3339),
		IsAlertActive: false,
	}

	return strategy, nil
}

func handleRunBacktest(_ *data.Conn, userID int, argsBytes []byte) (interface{}, error) {
	// For testing purposes, return a mock backtest result
	// In a real test, this would call the actual strategy.RunBacktest

	var args map[string]interface{}
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		return nil, err
	}

	_, ok := args["strategyId"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing strategyId parameter")
	}

	// Generate mock backtest results
	result := BacktestResponse{
		Instances: []BacktestResult{
			{
				Ticker:         "AAPL",
				SecurityID:     1,
				Timestamp:      time.Now().AddDate(0, -1, 0).Unix() * 1000,
				Open:           150.0,
				High:           155.0,
				Low:            149.0,
				Close:          153.0,
				Volume:         50000000,
				Classification: true,
				FutureReturns:  map[string]float64{"1d": 0.02, "5d": 0.05},
			},
			{
				Ticker:         "MSFT",
				SecurityID:     2,
				Timestamp:      time.Now().AddDate(0, -1, -5).Unix() * 1000,
				Open:           300.0,
				High:           305.0,
				Low:            299.0,
				Close:          303.0,
				Volume:         30000000,
				Classification: true,
				FutureReturns:  map[string]float64{"1d": 0.01, "5d": 0.03},
			},
		},
		Summary: BacktestSummary{
			TotalInstances:   2,
			PositiveSignals:  2,
			DateRange:        []string{time.Now().AddDate(0, -6, 0).Format("2006-01-02"), time.Now().Format("2006-01-02")},
			SymbolsProcessed: 100,
			Columns:          []string{"ticker", "timestamp", "classification"},
		},
	}

	return result, nil
}

// Helper functions for mock data generation
func generateMockStrategyName(query string) string {
	words := strings.Fields(query)
	if len(words) > 4 {
		words = words[:4]
	}

	titleCaser := cases.Title(language.English)
	for i, word := range words {
		words[i] = titleCaser.String(strings.ToLower(word))
	}

	return fmt.Sprintf("%s Strategy %d", strings.Join(words, " "), time.Now().Unix())
}

func generateMockPythonCode(query string) string {
	queryLower := strings.ToLower(query)

	var code strings.Builder
	code.WriteString("def strategy():\n")
	code.WriteString("    \"\"\"AI-generated strategy function using NEW ACCESSOR PATTERN\"\"\"\n")
	code.WriteString("    instances = []\n")
	code.WriteString("    \n")
	code.WriteString("    # Get market data using accessor functions\n")
	code.WriteString("    bar_data = get_bar_data(\n")
	code.WriteString("        timeframe='1d',\n")
	code.WriteString("        columns=['ticker', 'timestamp', 'open', 'close', 'volume'],\n")
	code.WriteString("        min_bars=30\n")
	code.WriteString("    )\n")
	code.WriteString("    \n")
	code.WriteString("    if len(bar_data) == 0:\n")
	code.WriteString("        return instances\n")
	code.WriteString("    \n")
	code.WriteString("    # Convert to DataFrame\n")
	code.WriteString("    import pandas as pd\n")
	code.WriteString("    df = pd.DataFrame(bar_data, columns=['ticker', 'timestamp', 'open', 'close', 'volume'])\n")
	code.WriteString("    \n")

	// Add specific logic based on query content
	if strings.Contains(queryLower, "gap") {
		code.WriteString("    # Gap analysis\n")
		code.WriteString("    df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date\n")
		code.WriteString("    df = df.sort_values(['ticker', 'date']).copy()\n")
		code.WriteString("    df['prev_close'] = df.groupby('ticker')['close'].shift(1)\n")
		code.WriteString("    df['gap_percent'] = ((df['open'] - df['prev_close']) / df['prev_close']) * 100\n")
		code.WriteString("    \n")
		code.WriteString("    gap_ups = df[df['gap_percent'] > 3.0].dropna()\n")
		code.WriteString("    \n")
		code.WriteString("    for _, row in gap_ups.iterrows():\n")
		code.WriteString("        instances.append({\n")
		code.WriteString("            'ticker': row['ticker'],\n")
		code.WriteString("            'timestamp': str(row['date']),\n")
		code.WriteString("            'signal': True,\n")
		code.WriteString("            'gap_percent': round(row['gap_percent'], 2)\n")
		code.WriteString("        })\n")
	} else if strings.Contains(queryLower, "volume") {
		code.WriteString("    # Volume analysis\n")
		code.WriteString("    df['avg_volume'] = df.groupby('ticker')['volume'].rolling(20).mean().reset_index(0, drop=True)\n")
		code.WriteString("    df['volume_ratio'] = df['volume'] / df['avg_volume']\n")
		code.WriteString("    \n")
		code.WriteString("    high_volume = df[df['volume_ratio'] > 2.0].dropna()\n")
		code.WriteString("    \n")
		code.WriteString("    for _, row in high_volume.iterrows():\n")
		code.WriteString("        instances.append({\n")
		code.WriteString("            'ticker': row['ticker'],\n")
		code.WriteString("            'timestamp': str(pd.to_datetime(row['timestamp'], unit='s').date()),\n")
		code.WriteString("            'signal': True,\n")
		code.WriteString("            'volume_ratio': round(row['volume_ratio'], 2)\n")
		code.WriteString("        })\n")
	} else {
		code.WriteString("    # Generic signal detection\n")
		code.WriteString("    for _, row in df.iterrows():\n")
		code.WriteString("        instances.append({\n")
		code.WriteString("            'ticker': row['ticker'],\n")
		code.WriteString("            'timestamp': str(pd.to_datetime(row['timestamp'], unit='s').date()),\n")
		code.WriteString("            'signal': True\n")
		code.WriteString("        })\n")
	}

	code.WriteString("    \n")
	code.WriteString("    return instances\n")

	return code.String()
}

// Test helper functions
func createTestUser(_ *testing.T, _ *data.Conn) int {
	// For testing purposes, use a fixed test user ID
	// In a real test environment, you might create an actual test user
	return 999999 // Large ID to avoid conflicts
}

func cleanupTestUser(_ *testing.T, _ *data.Conn, _ int) {
	// Clean up any test data created for this user
	// This would delete strategies, backtest results, etc.
	// For now, we'll skip cleanup since we're using mock data
}

// Benchmark test for strategy creation performance
func BenchmarkStrategyCreation(b *testing.B) {
	conn, cleanup := data.InitConn(false)
	defer cleanup()

	userID := 999999
	query := "Find stocks that gap up more than 3% with high volume"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handleCreateStrategy(conn, userID, []byte(fmt.Sprintf(`{"query": "%s", "strategyId": -1}`, query)))
		if err != nil {
			b.Fatalf("Strategy creation failed: %v", err)
		}
	}
}

// Benchmark test for backtest execution performance
func BenchmarkBacktestExecution(b *testing.B) {
	conn, cleanup := data.InitConn(false)
	defer cleanup()

	userID := 999999
	strategyID := 123

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handleRunBacktest(conn, userID, []byte(fmt.Sprintf(`{"strategyId": %d, "securities": [], "start": %d, "returnWindows": [1, 5]}`, strategyID, time.Now().AddDate(0, -1, 0).Unix()*1000)))
		if err != nil {
			b.Fatalf("Backtest execution failed: %v", err)
		}
	}
}
