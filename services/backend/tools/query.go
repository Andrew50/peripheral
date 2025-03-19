package tools

import (
	"backend/utils"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"google.golang.org/genai"
)

type Query struct {
	Query string `json:"query"`
}

// ParsedQuery represents the structured JSON output from the LLM
type ParsedQuery struct {
	Timeframes []string    `json:"timeframes"`
	Stocks     Stocks      `json:"stocks"`
	Indicators []Indicator `json:"indicators"`
	Conditions []Condition `json:"conditions"`
	Sequence   Sequence    `json:"sequence"`
	DateRange  DateRange   `json:"date_range"`
}

type Stocks struct {
	Universe string            `json:"universe"`
	Include  []string          `json:"include"`
	Exclude  []string          `json:"exclude"`
	Filters  map[string]Filter `json:"filters"`
}

type Filter struct {
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
}

type Indicator struct {
	Type      string `json:"type"`
	Period    int    `json:"period"`
	Timeframe string `json:"timeframe"`
}

type Condition struct {
	LHS       FieldWithOffset `json:"lhs"`
	Operation string          `json:"operation"`
	RHS       RHS             `json:"rhs"`
	Timeframe string          `json:"timeframe"`
}

type FieldWithOffset struct {
	Field  string `json:"field"`
	Offset int    `json:"offset"`
}

type RHS struct {
	Field     string  `json:"field,omitempty"`
	Offset    int     `json:"offset,omitempty"`
	Indicator string  `json:"indicator,omitempty"`
	Period    int     `json:"period,omitempty"`
	Value     float64 `json:"value,omitempty"`
}

type Sequence struct {
	Condition string `json:"condition"`
	Timeframe string `json:"timeframe"`
	Window    int    `json:"window"`
}

type DateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// ExecuteResult represents the result of executing a function
type ExecuteResult struct {
	FunctionName string      `json:"function_name"`
	Result       interface{} `json:"result"`
	Error        string      `json:"error,omitempty"`
}

// ConversationData represents the data structure for storing conversation context
type ConversationData struct {
	Query         string          `json:"query"`
	ResponseText  string          `json:"response_text"`
	FunctionCalls []FunctionCall  `json:"function_calls"`
	ToolResults   []ExecuteResult `json:"tool_results"`
	Timestamp     time.Time       `json:"timestamp"`
}

// inferDateRange determines appropriate date ranges when not explicitly provided
func inferDateRange(queryText string) DateRange {
	now := time.Now()

	// Default to last 90 days for "recent" queries
	defaultRange := DateRange{
		Start: now.AddDate(0, -3, 0).Format("2006-01-02"),
		End:   now.Format("2006-01-02"),
	}

	// For very recent queries, use last 30 days
	if containsAny(queryText, []string{"very recent", "last month", "past month", "last 30 days", "this month"}) {
		return DateRange{
			Start: now.AddDate(0, -1, 0).Format("2006-01-02"),
			End:   now.Format("2006-01-02"),
		}
	}

	// For recent/current queries, use last 90 days
	if containsAny(queryText, []string{"recent", "current", "lately", "now", "present"}) {
		return defaultRange
	}

	// For YTD queries
	if containsAny(queryText, []string{"ytd", "year to date", "this year"}) {
		return DateRange{
			Start: time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02"),
			End:   now.Format("2006-01-02"),
		}
	}

	// For 1-year lookback
	if containsAny(queryText, []string{"last year", "past year", "12 months", "one year"}) {
		return DateRange{
			Start: now.AddDate(-1, 0, 0).Format("2006-01-02"),
			End:   now.Format("2006-01-02"),
		}
	}

	// Default to 90 days if no time context is found
	return defaultRange
}

// containsAny checks if the text contains any of the provided phrases
func containsAny(text string, phrases []string) bool {
	lowerText := strings.ToLower(text)
	for _, phrase := range phrases {
		if strings.Contains(lowerText, phrase) {
			return true
		}
	}
	return false
}

// GetQuery processes a natural language query and returns the result
func GetQuery(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var query Query
	if err := json.Unmarshal(args, &query); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	// If the query is empty, return an error
	if query.Query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	ctx := context.Background()

	// Generate a unique key for this conversation based on user ID and query content
	cacheKey := fmt.Sprintf("conversation:%d:%s", userID, generateHashFromString(query.Query))

	// Check if we have this conversation cached
	cachedData, err := getConversationFromCache(ctx, conn, userID, cacheKey)
	if err == nil && cachedData != nil {
		// We found cached data, return it
		return map[string]interface{}{
			"type":    "function_calls",
			"results": cachedData.ToolResults,
			"text":    cachedData.ResponseText,
			"cached":  true,
		}, nil
	}

	// Get function calls from the LLM
	geminiFuncResponse, err := getGeminiFunctionResponse(conn, query.Query)
	if err != nil {
		return nil, fmt.Errorf("error getting function calls: %w", err)
	}

	functionCalls := geminiFuncResponse.FunctionCalls
	responseText := geminiFuncResponse.Text

	// Process function calls to add default date ranges if needed
	for i, fc := range functionCalls {
		// Check if this is a function that might need date ranges
		if fc.Name == "getStockData" || fc.Name == "getChartData" || strings.Contains(fc.Name, "Price") {
			var args map[string]interface{}
			if err := json.Unmarshal(fc.Args, &args); err == nil {
				// Check if date range is missing or incomplete
				_, hasStart := args["start_date"]
				_, hasEnd := args["end_date"]

				if !hasStart || !hasEnd {
					// Infer appropriate date range
					dateRange := inferDateRange(query.Query)

					// Add the inferred dates to the args
					if !hasStart {
						args["start_date"] = dateRange.Start
					}
					if !hasEnd {
						args["end_date"] = dateRange.End
					}

					// Update the function call with the new args
					updatedArgs, _ := json.Marshal(args)
					functionCalls[i].Args = updatedArgs
				}
			}
		}
	}

	// If no function calls were returned, fall back to the text response
	if len(functionCalls) == 0 {
		// If we already have a text response, use it
		if responseText != "" {
			result := map[string]interface{}{
				"type": "text",
				"text": responseText,
			}

			// Cache the text response
			conversationData := &ConversationData{
				Query:        query.Query,
				ResponseText: responseText,
				Timestamp:    time.Now(),
			}
			saveConversationToCache(ctx, conn, userID, cacheKey, conversationData)

			return result, nil
		}

		// Otherwise, get a direct text response
		textResponse, err := getGeminiResponse(conn, query.Query)
		if err != nil {
			return nil, fmt.Errorf("error getting text response: %w", err)
		}

		result := map[string]interface{}{
			"type": "text",
			"text": textResponse,
		}

		// Cache the text response
		conversationData := &ConversationData{
			Query:        query.Query,
			ResponseText: textResponse,
			Timestamp:    time.Now(),
		}
		saveConversationToCache(ctx, conn, userID, cacheKey, conversationData)

		return result, nil
	}

	// Execute the functions in order and collect results
	var results []ExecuteResult

	// Check cache for existing tool results
	toolResultsMap := make(map[string]ExecuteResult)
	if cachedData != nil {
		for _, result := range cachedData.ToolResults {
			toolResultsMap[result.FunctionName] = result
		}
	}

	for _, fc := range functionCalls {
		// Check if we already have the result for this function call in cache
		if cachedResult, exists := toolResultsMap[fc.Name]; exists {
			results = append(results, cachedResult)
			continue
		}

		// Check if the function exists in Tools map
		tool, exists := Tools[fc.Name]
		if !exists {
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Error:        fmt.Sprintf("function '%s' not found", fc.Name),
			})
			continue
		}

		// Execute the function
		result, err := tool.Function(conn, userID, fc.Args)
		if err != nil {
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Error:        err.Error(),
			})
		} else {
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Result:       result,
			})
		}
	}

	// Cache the conversation data
	conversationData := &ConversationData{
		Query:         query.Query,
		ResponseText:  responseText,
		FunctionCalls: functionCalls,
		ToolResults:   results,
		Timestamp:     time.Now(),
	}
	saveConversationToCache(ctx, conn, userID, cacheKey, conversationData)

	// Return both the function call results and the text response
	return map[string]interface{}{
		"type":    "function_calls",
		"results": results,
		"text":    responseText,
	}, nil
}

// generateHashFromString creates a SHA-256 hash from a string
func generateHashFromString(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// saveConversationToCache saves the conversation data to Redis
func saveConversationToCache(ctx context.Context, conn *utils.Conn, userID int, cacheKey string, data *ConversationData) error {
	// Serialize the conversation data
	serialized, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize conversation data: %w", err)
	}

	// Save to Redis with 24-hour expiration
	err = conn.Cache.Set(ctx, cacheKey, serialized, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to save conversation to cache: %w", err)
	}

	// Add to user's conversation list
	userConversationsKey := fmt.Sprintf("user:%d:conversations", userID)

	// Add to sorted set with timestamp as score for ordering
	score := float64(data.Timestamp.Unix())
	err = conn.Cache.ZAdd(ctx, userConversationsKey, &redis.Z{
		Score:  score,
		Member: cacheKey,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to add conversation to user's list: %w", err)
	}

	return nil
}

// getConversationFromCache retrieves conversation data from Redis
func getConversationFromCache(ctx context.Context, conn *utils.Conn, userID int, cacheKey string) (*ConversationData, error) {
	// Get the conversation data from Redis
	cachedValue, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, fmt.Errorf("conversation not found in cache: %w", err)
	}

	// Deserialize the conversation data
	var conversationData ConversationData
	err = json.Unmarshal([]byte(cachedValue), &conversationData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize conversation data: %w", err)
	}

	return &conversationData, nil
}

// GetUserConversations retrieves a list of recent conversations for a user
func GetUserConversations(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	ctx := context.Background()
	userConversationsKey := fmt.Sprintf("user:%d:conversations", userID)

	// Get conversation keys from the sorted set, sorted by timestamp (most recent first)
	conversationKeys, err := conn.Cache.ZRevRange(ctx, userConversationsKey, 0, 9).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user conversations: %w", err)
	}

	var conversations []*ConversationData
	for _, key := range conversationKeys {
		conversation, err := getConversationFromCache(ctx, conn, userID, key)
		if err != nil {
			continue // Skip conversations that can't be retrieved
		}
		conversations = append(conversations, conversation)
	}

	return conversations, nil
}

// GetLLMParsedQuery is kept for backward compatibility
func GetLLMParsedQuery(conn *utils.Conn, query Query) (string, error) {
	llmResponse, err := getGeminiResponse(conn, query.Query)
	if err != nil {
		return "", fmt.Errorf("error getting gemini response: %w", err)
	}

	return llmResponse, nil
}

// getSystemInstruction reads the content of query.txt to be used as system instruction
func getSystemInstruction() (string, error) {
	// Get the directory of the current file (gemini.go)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("error getting current file path")
	}
	currentDir := filepath.Dir(filename)

	// Construct path to query.txt
	queryFilePath := filepath.Join(currentDir, "query.txt")

	// Read the content of query.txt
	content, err := os.ReadFile(queryFilePath)
	if err != nil {
		return "", fmt.Errorf("error reading query.txt: %w", err)
	}

	// Replace the {{CURRENT_TIME}} placeholder with the actual current time
	currentTime := time.Now().Format(time.RFC3339)
	instruction := strings.Replace(string(content), "{{CURRENT_TIME}}", currentTime, -1)

	return instruction, nil
}

func getGeminiResponse(conn *utils.Conn, query string) (string, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return "", fmt.Errorf("error getting gemini key: %w", err)
	}

	// Create a new client using the API key
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("error creating gemini client: %w", err)
	}
	// Get the system instruction
	systemInstruction, err := getSystemInstruction()
	if err != nil {
		return "", fmt.Errorf("error getting system instruction: %w", err)
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
	}
	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-001", genai.Text(query), config)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}

	// Extract the response text
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	// Get the text from the response
	text := fmt.Sprintf("%v", result.Candidates[0].Content.Parts[0])
	return text, nil
}

// FunctionCall represents a function to be called with its arguments
type FunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// FunctionResponse represents the response from the LLM with function calls
type FunctionResponse struct {
	FunctionCalls []FunctionCall `json:"function_calls"`
}

type GeminiFunctionResponse struct {
	FunctionCalls []FunctionCall `json:"function_calls"`
	Text          string         `json:"text"`
}

// getGeminiFunctionResponse uses the Google Function API to return an ordered list of functions to execute
func getGeminiFunctionResponse(conn *utils.Conn, query string) (*GeminiFunctionResponse, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return nil, fmt.Errorf("error getting gemini key: %w", err)
	}

	// Create a new client using the API key
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating gemini client: %w", err)
	}
	// Get the system instruction
	systemInstruction, err := getSystemInstruction()
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %w", err)
	}
	systemInstruction = "You are a helpful assistant that can execute functions."
	var geminiTools []*genai.Tool
	for _, tool := range Tools {
		// Convert the FunctionDeclaration to a Tool
		geminiTools = append(geminiTools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        tool.FunctionDeclaration.Name,
					Description: tool.FunctionDeclaration.Description,
					Parameters:  tool.FunctionDeclaration.Parameters,
				},
			},
		})
	}
	config := &genai.GenerateContentConfig{
		Tools: geminiTools,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
	}
	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-001", genai.Text(query), config)
	if err != nil {
		return nil, fmt.Errorf("error generating content: %w", err)
	}

	// Extract the clean text response for display
	responseText := ""
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		for _, part := range result.Candidates[0].Content.Parts {
			if part.Text != "" {
				responseText = part.Text
				break
			}
		}
	}

	// Print the response for debugging
	fmt.Println("Gemini response:", responseText)

	// Extract function calls from response
	var functionCalls []FunctionCall

	// Process the response to extract function calls
	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			// Check if the part is a FunctionCall
			if fc := part.FunctionCall; fc != nil {
				// Convert arguments to JSON
				args, err := json.Marshal(fc.Args)
				if err != nil {
					return nil, fmt.Errorf("error marshaling function args: %w", err)
				}

				functionCalls = append(functionCalls, FunctionCall{
					Name: fc.Name,
					Args: args,
				})
			}
		}
	}

	return &GeminiFunctionResponse{
		FunctionCalls: functionCalls,
		Text:          responseText,
	}, nil
}
