package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	Messages  []ChatMessage `json:"messages"`
	Timestamp time.Time     `json:"timestamp"`
}

// ChatMessage represents a single message in the conversation
type ChatMessage struct {
	Query         string          `json:"query"`
	ResponseText  string          `json:"response_text"`
	FunctionCalls []FunctionCall  `json:"function_calls"`
	ToolResults   []ExecuteResult `json:"tool_results"`
	Timestamp     time.Time       `json:"timestamp"`
	ExpiresAt     time.Time       `json:"expires_at"` // When this message should expire
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
	fmt.Println("GetQuery", args)

	// Use the standardized Redis connectivity test
	ctx := context.Background()
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		fmt.Printf("WARNING: %s\n", message)
	} else {
		fmt.Println(message)
	}

	var query Query
	if err := json.Unmarshal(args, &query); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	// If the query is empty, return an error
	if query.Query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Check for existing conversation history
	conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	conversationData, err := getConversationFromCache(ctx, conn, userID, conversationKey)

	// If we have existing conversation data, append the new message
	fmt.Println("Accessing conversation for key:", conversationKey)

	// If no conversation exists, create a new one
	if err != nil || conversationData == nil {
		fmt.Printf("Creating new conversation. Error: %v\n", err)
		conversationData = &ConversationData{
			Messages:  []ChatMessage{},
			Timestamp: time.Now(),
		}
	} else {
		fmt.Printf("Found existing conversation with %d messages\n", len(conversationData.Messages))
	}

	// Get function calls from the LLM with context from previous messages
	var prompt string
	if len(conversationData.Messages) > 0 {
		// Build context from previous messages for Gemini
		prompt = buildConversationContext(conversationData.Messages, query.Query)
	} else {
		prompt = query.Query
	}

	geminiFuncResponse, err := getGeminiFunctionResponse(ctx, conn, prompt)
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

	// Create new message
	newMessage := ChatMessage{
		Query:         query.Query,
		ResponseText:  responseText,
		FunctionCalls: functionCalls,
		ToolResults:   []ExecuteResult{},
		Timestamp:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour), // Message expires after 24 hours
	}

	// If no function calls were returned, fall back to the text response
	if len(functionCalls) == 0 {
		// If we already have a text response, use it
		if responseText != "" {
			result := map[string]interface{}{
				"type": "text",
				"text": responseText,
			}

			// Add new message to conversation history
			conversationData.Messages = append(conversationData.Messages, newMessage)
			conversationData.Timestamp = time.Now()
			if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
				fmt.Printf("Error saving updated conversation: %v\n", err)
			}

			return result, nil
		}

		// Otherwise, get a direct text response
		textResponse, err := getGeminiResponse(ctx, conn, prompt)
		if err != nil {
			return nil, fmt.Errorf("error getting text response: %w", err)
		}

		result := map[string]interface{}{
			"type": "text",
			"text": textResponse,
		}

		// Update message and add to conversation history
		newMessage.ResponseText = textResponse
		conversationData.Messages = append(conversationData.Messages, newMessage)
		conversationData.Timestamp = time.Now()
		if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
			fmt.Printf("Error saving updated conversation with text response: %v\n", err)
		}

		return result, nil
	}

	// Execute the functions in order and collect results

	var results []ExecuteResult

	for _, fc := range functionCalls {
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

	// Update message with tool results and add to conversation
	newMessage.ToolResults = results
	conversationData.Messages = append(conversationData.Messages, newMessage)
	conversationData.Timestamp = time.Now()
	if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
		fmt.Printf("Error saving conversation with function calls: %v\n", err)
	}

	// Return both the function call results and the text response
	return map[string]interface{}{
		"type":    "function_calls",
		"results": results,
		"text":    responseText,
		"history": conversationData,
	}, nil
}

// buildConversationContext formats the conversation history for Gemini
func buildConversationContext(messages []ChatMessage, currentQuery string) string {
	var context strings.Builder

	// Include up to last 10 messages for context
	startIdx := 0
	if len(messages) > 10 {
		startIdx = len(messages) - 10
	}

	for i := startIdx; i < len(messages); i++ {
		context.WriteString("User: ")
		context.WriteString(messages[i].Query)
		context.WriteString("\n")

		context.WriteString("Assistant: ")
		context.WriteString(messages[i].ResponseText)
		context.WriteString("\n\n")
	}

	// Add current query
	context.WriteString("User: ")
	context.WriteString(currentQuery)

	return context.String()
}

// saveConversationToCache saves the conversation data to Redis
func saveConversationToCache(ctx context.Context, conn *utils.Conn, userID int, cacheKey string, data *ConversationData) error {
	if data == nil {
		return fmt.Errorf("cannot save nil conversation data")
	}

	// Test Redis connectivity before saving
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		fmt.Printf("WARNING: %s\n", message)
		return fmt.Errorf("redis connectivity test failed: %s", message)
	}

	// Filter out expired messages
	now := time.Now()
	var validMessages []ChatMessage
	for _, msg := range data.Messages {
		if msg.ExpiresAt.After(now) {
			validMessages = append(validMessages, msg)
		} else {
			fmt.Printf("Removing expired message from %s\n", msg.Timestamp.Format(time.RFC3339))
		}
	}

	// Update the data with only valid messages
	data.Messages = validMessages

	if len(data.Messages) == 0 {
		fmt.Println("Warning: Saving empty conversation data to cache (all messages expired)")
	}

	// Print details about what we're saving
	fmt.Printf("Saving conversation with %d valid messages to key: %s\n", len(data.Messages), cacheKey)

	// Serialize the conversation data
	serialized, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Failed to serialize conversation data: %v\n", err)
		return fmt.Errorf("failed to serialize conversation data: %w", err)
	}

	// Save to Redis without an expiration - we'll handle expiration at the message level
	err = conn.Cache.Set(ctx, cacheKey, serialized, 0).Err()
	if err != nil {
		fmt.Printf("Failed to save conversation to Redis: %v\n", err)
		return fmt.Errorf("failed to save conversation to cache: %w", err)
	}

	// Verify the data was saved correctly
	verification, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		fmt.Printf("Failed to verify saved conversation: %v\n", err)
		return fmt.Errorf("failed to verify saved conversation: %w", err)
	}

	var verifiedData ConversationData
	if err := json.Unmarshal([]byte(verification), &verifiedData); err != nil {
		fmt.Printf("Failed to parse verified conversation: %v\n", err)
		return fmt.Errorf("failed to parse verified conversation: %w", err)
	}

	fmt.Printf("Successfully saved and verified conversation to Redis. Verified %d messages.\n",
		len(verifiedData.Messages))

	return nil
}

// SetMessageExpiration allows setting a custom expiration time for a message
func SetMessageExpiration(message *ChatMessage, duration time.Duration) {
	message.ExpiresAt = time.Now().Add(duration)
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

	// Filter out expired messages
	now := time.Now()
	originalCount := len(conversationData.Messages)
	var validMessages []ChatMessage

	for _, msg := range conversationData.Messages {
		if msg.ExpiresAt.After(now) {
			validMessages = append(validMessages, msg)
		} else {
			fmt.Printf("Filtering out expired message from %s during retrieval\n",
				msg.Timestamp.Format(time.RFC3339))
		}
	}

	// Update with only valid messages
	conversationData.Messages = validMessages

	// If we filtered out messages, update the cache
	if len(validMessages) < originalCount {
		fmt.Printf("Removed %d expired messages from conversation\n",
			originalCount-len(validMessages))

		// Save the updated conversation back to cache if we have at least one valid message
		if len(validMessages) > 0 {
			go func() {
				// Create a new context for the goroutine
				bgCtx := context.Background()
				if err := saveConversationToCache(bgCtx, conn, userID, cacheKey, &conversationData); err != nil {
					fmt.Printf("Failed to update cache after filtering expired messages: %v\n", err)
				}
			}()
		} else if originalCount > 0 {
			// All messages expired, so we should delete the conversation entirely
			go func() {
				bgCtx := context.Background()
				if err := conn.Cache.Del(bgCtx, cacheKey).Err(); err != nil {
					fmt.Printf("Failed to delete empty conversation after all messages expired: %v\n", err)
				} else {
					fmt.Printf("Deleted conversation %s as all messages expired\n", cacheKey)
				}
			}()
		}
	}

	return &conversationData, nil
}

// GetUserConversation retrieves the conversation for a user
func GetUserConversation(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	ctx := context.Background()

	// Test Redis connectivity before attempting to retrieve conversation
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		fmt.Printf("WARNING: %s\n", message)
	} else {
		fmt.Println(message)
	}

	conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	fmt.Println("GetUserConversation", conversationKey)

	conversation, err := getConversationFromCache(ctx, conn, userID, conversationKey)
	if err != nil {
		// Handle the case when conversation doesn't exist in cache
		if strings.Contains(err.Error(), "redis: nil") {
			fmt.Println("No conversation found in cache, returning empty history")
			// Return empty conversation history instead of error
			return &ConversationData{
				Messages:  []ChatMessage{},
				Timestamp: time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to get user conversation: %w", err)
	}

	// Log the conversation data for debugging
	fmt.Printf("Retrieved conversation: %+v\n", conversation)
	if conversation != nil {
		fmt.Printf("Number of messages: %d\n", len(conversation.Messages))
	}

	// Ensure we're returning valid data
	if conversation == nil || len(conversation.Messages) == 0 {
		fmt.Println("Conversation was retrieved but has no messages, returning empty history")
		return &ConversationData{
			Messages:  []ChatMessage{},
			Timestamp: time.Now(),
		}, nil
	}

	return conversation, nil
}

// GetLLMParsedQuery is kept for backward compatibility
func GetLLMParsedQuery(conn *utils.Conn, query Query) (string, error) {
	ctx := context.Background()
	llmResponse, err := getGeminiResponse(ctx, conn, query.Query)
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
	queryFilePath := filepath.Join(currentDir, "defaultSystemPrompt.txt")

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

func getGeminiResponse(ctx context.Context, conn *utils.Conn, query string) (string, error) {
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
func getGeminiFunctionResponse(ctx context.Context, conn *utils.Conn, query string) (*GeminiFunctionResponse, error) {
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

	// Check if query has conversation history format ("User: ... Assistant: ...")
	// If it does, use that directly as the prompt
	if strings.Contains(query, "User:") && strings.Contains(query, "Assistant:") {
		// Use the formatted query directly since it already contains the conversation history
		result, err := client.Models.GenerateContent(
			ctx,
			"gemini-2.0-flash-001",
			genai.Text(query),
			config,
		)
		if err != nil {
			return nil, fmt.Errorf("error generating content with history: %w", err)
		}

		// Process the result similarly to the single-query case
		responseText := ""
		if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
			for _, part := range result.Candidates[0].Content.Parts {
				if part.Text != "" {
					responseText = part.Text
					break
				}
			}
		}

		// Extract function calls
		var functionCalls []FunctionCall
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

	// Fall back to the original implementation for simple queries
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
