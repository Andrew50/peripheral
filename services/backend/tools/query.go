package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genai"
)

type Query struct {
	Query   string                   `json:"query"`
	Context []map[string]interface{} `json:"context,omitempty"`
}

// ExecuteResult represents the result of executing a function
type ExecuteResult struct {
	FunctionName string      `json:"function_name"`
	Result       interface{} `json:"result"`
	Error        string      `json:"error,omitempty"`
	Args         interface{} `json:"args,omitempty"`
}

// ConversationData represents the data structure for storing conversation context
type ConversationData struct {
	Messages  []ChatMessage `json:"messages"`
	Timestamp time.Time     `json:"timestamp"`
}

// ChatMessage represents a single message in the conversation
type ChatMessage struct {
	Query         string                   `json:"query"`
	ContentChunks []ContentChunk           `json:"content_chunks,omitempty"`
	ResponseText  string                   `json:"response_text,omitempty"`
	FunctionCalls []FunctionCall           `json:"function_calls"`
	ToolResults   []ExecuteResult          `json:"tool_results"`
	ContextItems  []map[string]interface{} `json:"context_items,omitempty"` // Store context sent with user message
	Timestamp     time.Time                `json:"timestamp"`
	ExpiresAt     time.Time                `json:"expires_at"` // When this message should expire
}

// ContentChunk represents a piece of content in the response sequence
type ContentChunk struct {
	Type    string      `json:"type"`    // "text" or "table" (or others later, e.g., "image")
	Content interface{} `json:"content"` // string for "text", TableData for "table"
}

type QueryResponse struct {
	Type          string            `json:"type"` //"mixed_content", "function_calls", "simple_text"
	ContentChunks []ContentChunk    `json:"content_chunks,omitempty"`
	Text          string            `json:"text,omitempty"`
	Results       []ExecuteResult   `json:"results,omitempty"`
	History       *ConversationData `json:"history,omitempty"`
}

// ThinkingResponse represents the JSON output from the thinking model with rounds
type ThinkingResponse struct {
	Rounds                  [][]FunctionCall `json:"rounds"`
	RequiresFurtherPlanning bool             `json:"requires_further_planning"`
	RequiresFinalResponse   bool             `json:"requires_final_response"`
	ContentChunks           []ContentChunk   `json:"content_chunks,omitempty"`
	PlanningContext         json.RawMessage  `json:"planning_context,omitempty"`
}

// buildContextPrompt formats incoming chart/filing context for the model
func buildContextPrompt(contextItems []map[string]interface{}) string {
	var sb strings.Builder
	for _, item := range contextItems {
		// Treat filing contexts first
		if _, ok := item["link"]; ok {
			ticker, _ := item["ticker"].(string)
			fType, _ := item["filingType"].(string)
			link, _ := item["link"].(string)
			sb.WriteString(fmt.Sprintf("Filing - Ticker: %s, Type: %s, Link: %s\n", ticker, fType, link))
		} else if _, ok := item["timestamp"]; ok {
			// Then treat instance contexts
			ticker, _ := item["ticker"].(string)
			secId := fmt.Sprint(item["securityId"])
			tsStr := fmt.Sprint(item["timestamp"])
			sb.WriteString(fmt.Sprintf("Instance - Ticker: %s, SecurityId: %s, TimestampMs: %s\n", ticker, secId, tsStr))
		}
	}
	return sb.String()
}

// replaceTickerPlaceholder is a helper function used by ReplaceAllStringFunc.
// It takes a matched placeholder string (e.g., "$$$TICKER-TIMESTAMP$$$"),
// looks up the security ID, and returns the replacement string
// (e.g., "$$$TICKER-ID-TIMESTAMP$$$") or the original match on error.
func replaceTickerPlaceholder(conn *utils.Conn, match string) string {
	// Use the same regex to extract parts from the *specific match*
	tickerTimestampRegex := regexp.MustCompile(`\$\$\$([A-Z]{1,5})-(\d+)\$\$\$`)
	submatches := tickerTimestampRegex.FindStringSubmatch(match)
	if len(submatches) != 3 {
		fmt.Printf("    [Helper] Error: Regex did not find 3 submatches in '%s'.\n", match)
		return match // Should not happen if called by ReplaceAllStringFunc, but safety first
	}
	ticker := submatches[1]
	timestampStr := submatches[2]

	timestampMs, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		fmt.Printf("    [Helper] Error parsing timestamp string '%s': %v. Skipping replacement.\n", timestampStr, err)
		return match
	}

	var timestamp time.Time
	if timestampMs == 0 {
		timestamp = time.Now()
	} else {
		timestamp = time.UnixMilli(timestampMs)
	}

	securityId, err := utils.GetSecurityID(conn, ticker, timestamp)
	if err != nil {
		fmt.Printf("    [Helper] Error getting security ID for %s at %v: %v. Skipping replacement.\n", ticker, timestamp, err)
		return match
	}

	replacement := fmt.Sprintf("$$$%s-%d-%s$$$", ticker, securityId, timestampStr)
	return replacement
}

// GetQuery processes a natural language query and returns the result
func GetQuery(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {

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

	// Build context prompt section
	contextSection := buildContextPrompt(query.Context)
	// If userQuery is empty, error
	userQuery := query.Query
	if userQuery == "" {
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
	var conversationHistory string
	var allResults []ExecuteResult
	var allThinkingResults []ThinkingResponse
	if len(conversationData.Messages) > 0 {
		// Build context from previous messages for Gemini
		conversationHistory = buildConversationContext(conversationData.Messages)
	} else {
		conversationHistory = ""
	}
	maxTurns := 5
	numTurns := 0
	for numTurns < maxTurns {
		var prompt strings.Builder
		if conversationHistory != "" {
			prompt.WriteString("Conversation History:\n")
			prompt.WriteString(conversationHistory)
			prompt.WriteString("\n\n")
		}
		if contextSection != "" {
			prompt.WriteString("User added context:\n")
			prompt.WriteString(contextSection)
			prompt.WriteString("\n")
		}
		prompt.WriteString("User Query:\n")
		prompt.WriteString(userQuery)
		prompt.WriteString("\n\n")
		if len(allThinkingResults) > 0 {
			prompt.WriteString("Results from all previous rounds:\n\n")
			resultsJSON, _ := json.Marshal(allResults)
			prompt.WriteString("```json\n")
			prompt.WriteString(string(resultsJSON))
			prompt.WriteString("\n```\n\n")
		}
		fmt.Println("prompt ", prompt.String())
		// This first passes the query to a thinking model
		geminiThinkingResponse, err := getGeminiFunctionThinking(ctx, conn, "defaultSystemPrompt", prompt.String())
		if err != nil {
			return nil, fmt.Errorf("error getting thinking response: %w", err)
		}

		responseText := geminiThinkingResponse.Text
		// Try to parse the thinking response as JSON
		var thinkingResp ThinkingResponse
		fmt.Println("thinking response ", thinkingResp)
		// Find the JSON block in the response
		jsonStartIdx := strings.Index(responseText, "{")
		jsonEndIdx := strings.LastIndex(responseText, "}")

		// If no valid JSON is found, just return the text response
		if jsonStartIdx == -1 || jsonEndIdx == -1 || jsonEndIdx <= jsonStartIdx {
			return QueryResponse{
				Type:    "text",
				Text:    responseText,
				History: conversationData,
			}, nil
		}

		jsonBlock := responseText[jsonStartIdx : jsonEndIdx+1]
		_ = json.Unmarshal([]byte(jsonBlock), &thinkingResp) // Ignore error for now, as the block was empty

		if len(thinkingResp.Rounds) == 0 && len(thinkingResp.ContentChunks) == 0 {
			newMessage := ChatMessage{
				Query:         query.Query,
				ResponseText:  responseText,
				FunctionCalls: []FunctionCall{},
				ToolResults:   []ExecuteResult{},
				ContextItems:  query.Context, // Store context with the user query message
				Timestamp:     time.Now(),
				ExpiresAt:     time.Now().Add(24 * time.Hour),
			}

			// Add new message to conversation history
			conversationData.Messages = append(conversationData.Messages, newMessage)
			conversationData.Timestamp = time.Now()
			if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
				fmt.Printf("Error saving updated conversation: %v\n", err)
			}

			return QueryResponse{
				Type:    "text",
				Text:    responseText,
				History: conversationData,
			}, nil
		}

		// If we have content chunks directly in the response, return them
		if len(thinkingResp.ContentChunks) > 0 {

			newMessage := ChatMessage{
				Query:         query.Query,
				ContentChunks: thinkingResp.ContentChunks,
				FunctionCalls: []FunctionCall{},
				ToolResults:   []ExecuteResult{},
				ContextItems:  query.Context, // Store context with the user query message
				Timestamp:     time.Now(),
				ExpiresAt:     time.Now().Add(24 * time.Hour),
			}

			// Add new message to conversation history
			conversationData.Messages = append(conversationData.Messages, newMessage)
			conversationData.Timestamp = time.Now()
			if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
				fmt.Printf("Error saving updated conversation: %v\n", err)
			}

			// Return both the original ContentChunks for the frontend to render
			// and the text version for systems that can't handle structured content
			return QueryResponse{
				Type:          "mixed_content",
				ContentChunks: thinkingResp.ContentChunks,
				History:       conversationData,
			}, nil
		}

		// Try to process the thinking response as rounds
		contentChunks, thinkingResults, err := processThinkingResponse(ctx, conn, userID, thinkingResp, query.Query)
		if err == nil && len(thinkingResults) > 0 {
			if len(contentChunks) > 0 {
				// Create new message with the content chunks response
				newMessage := ChatMessage{
					Query:         query.Query,
					ContentChunks: contentChunks,
					FunctionCalls: []FunctionCall{},
					ToolResults:   []ExecuteResult{},
					ContextItems:  query.Context, // Store context with the user query message
					Timestamp:     time.Now(),
					ExpiresAt:     time.Now().Add(24 * time.Hour),
				}
				conversationData.Messages = append(conversationData.Messages, newMessage)
				conversationData.Timestamp = time.Now()
				if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
					fmt.Printf("Error saving updated conversation: %v\n", err)
				}

				return QueryResponse{
					Type:          "mixed_content",
					ContentChunks: newMessage.ContentChunks,
					History:       conversationData,
				}, nil
			}
			if !thinkingResp.RequiresFurtherPlanning {
				// Create new message with the round results and formatted response
				newMessage := ChatMessage{
					Query:         query.Query,
					ResponseText:  "Successfully processed the following function calls:\n\n",
					FunctionCalls: []FunctionCall{}, // We don't store these as regular function calls
					ToolResults:   thinkingResults,
					ContextItems:  query.Context, // Store context with the user query message
					Timestamp:     time.Now(),
					ExpiresAt:     time.Now().Add(24 * time.Hour),
				}

				// Add new message to conversation history
				conversationData.Messages = append(conversationData.Messages, newMessage)
				conversationData.Timestamp = time.Now()
				if err := saveConversationToCache(ctx, conn, userID, conversationKey, conversationData); err != nil {
					fmt.Printf("Error saving updated conversation: %v\n", err)
				}

				return QueryResponse{
					Type:    "function_calls",
					Results: thinkingResults,
					Text:    "Successfully processed the following function calls:\n\n",
					History: conversationData,
				}, nil
			}
			allResults = append(allResults, thinkingResults...)
			allThinkingResults = append(allThinkingResults, thinkingResp)
		}

		numTurns++
	}
	return nil, fmt.Errorf("error getting gemini function response: %w", err)
}

// buildConversationContext formats the conversation history for Gemini
func buildConversationContext(messages []ChatMessage) string {
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
		// Include context items if they exist for the user message
		if len(messages[i].ContextItems) > 0 {
			context.WriteString("User Context:\n")
			context.WriteString(buildContextPrompt(messages[i].ContextItems)) // Reuse existing formatting function
			context.WriteString("\n")
		}

		context.WriteString("Assistant: ")
		if len(messages[i].ContentChunks) > 0 {
			for _, chunk := range messages[i].ContentChunks {
				// Safely handle different content types
				switch v := chunk.Content.(type) {
				case string:
					context.WriteString(v)
				case map[string]interface{}:
					// For table data or other structured content, convert to a simple text representation
					jsonData, err := json.Marshal(v)
					if err == nil {
						context.WriteString(fmt.Sprintf("[Table data: %s]", string(jsonData)))
					} else {
						context.WriteString("[Table data]")
					}
				default:
					// Handle any other type by converting to string
					context.WriteString(fmt.Sprintf("%v", v))
				}
			}
		} else {
			context.WriteString(messages[i].ResponseText)
		}
		context.WriteString("\n\n")
	}
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

	systemInstruction, err := getSystemInstruction("finalResponseSystemPrompt")
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
	text := fmt.Sprintf("%v", result.Candidates[0].Content.Parts[0].Text)
	return text, nil
}

// FunctionCall represents a function to be called with its arguments
type FunctionCall struct {
	Name   string          `json:"name"`
	CallID string          `json:"call_id,omitempty"`
	Args   json.RawMessage `json:"args,omitempty"`
}

// FunctionResponse represents the response from the LLM with function calls
type FunctionResponse struct {
	FunctionCalls []FunctionCall `json:"function_calls"`
}

type GeminiFunctionResponse struct {
	FunctionCalls []FunctionCall `json:"function_calls"`
	Text          string         `json:"text"`
}

func getGeminiFunctionThinking(ctx context.Context, conn *utils.Conn, systemPrompt string, query string) (*GeminiFunctionResponse, error) {
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
	baseSystemInstruction, err := getSystemInstruction(systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %w", err)
	}

	// Enhance the system instruction with tool descriptions
	enhancedSystemInstruction := enhanceSystemPromptWithTools(baseSystemInstruction)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: enhancedSystemInstruction},
			},
		},
	}

	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.0-flash-thinking-exp-01-21",
		genai.Text(query),
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("error generating content with thinking model: %w", err)
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
	response := &GeminiFunctionResponse{
		FunctionCalls: []FunctionCall{},
		Text:          responseText,
	}
	return response, nil
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

	systemInstruction := "You are a helpful assistant that can answer questions and run functions"
	var geminiTools []*genai.Tool
	for _, tool := range GetTools(false) {
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
			"gemini-2.0-flash",
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

// RoundResult stores the results of a round's function calls
type RoundResult struct {
	Results map[string]interface{} `json:"results"`
}

// processThinkingResponse attempts to parse and execute the thinking model's rounds
// and modifies the final response string *before* parsing into chunks.
func processThinkingResponse(ctx context.Context, conn *utils.Conn, userID int, thinkingResp ThinkingResponse, originalQuery string) ([]ContentChunk, []ExecuteResult, error) {

	// Check if this is an immediate content_chunks response (no rounds executed)
	if len(thinkingResp.Rounds) == 0 && len(thinkingResp.ContentChunks) > 0 {
		// Even though we inject later, we *could* inject here too if needed
		// For now, return as is, assuming injection happens after final response gen
		return thinkingResp.ContentChunks, []ExecuteResult{}, nil
	}

	// --- Round Execution Logic ---
	var allResults []ExecuteResult
	var allPreviousRoundResults []ExecuteResult
	for _, round := range thinkingResp.Rounds {
		// ... (existing round processing logic: build prompt, call Gemini, execute functions) ...

		// First, convert the round to JSON
		roundJSON, err := json.Marshal(round)
		if err != nil {
			fmt.Printf("Error marshaling round to JSON: %v\n", err)
			continue
		}

		// Create a prompt that includes the round and previous results
		var prompt strings.Builder
		prompt.WriteString("Process this round of function calls:\n\n")
		prompt.WriteString("```json\n")
		prompt.WriteString(string(roundJSON))
		prompt.WriteString("\n```\n\n")

		// Include ALL previous round results if available
		if len(allPreviousRoundResults) > 0 {
			prompt.WriteString("Results from all previous rounds:\n\n")
			resultsJSON, _ := json.Marshal(allPreviousRoundResults)
			prompt.WriteString("```json\n")
			prompt.WriteString(string(resultsJSON))
			prompt.WriteString("\n```\n\n")
		}

		prompt.WriteString("Please process this round of function calls.\n")
		// Send to Gemini for processing
		fmt.Printf("Sending round to Gemini for processing:\n%s\n", prompt.String())
		processedRound, err := processRoundWithGemini(ctx, conn, prompt.String())
		if err != nil {
			fmt.Printf("Error processing round with Gemini: %v\n", err)

			continue
		}

		// Execute the functions returned by Gemini
		roundResults, err := executeGeminiFunctions(ctx, conn, userID, processedRound)
		if err != nil {
			fmt.Printf("Error executing functions: %v\n", err)
			continue
		}

		// Add this round's results to the combined results
		allResults = append(allResults, roundResults...)
		// Accumulate results for the next round
		allPreviousRoundResults = append(allPreviousRoundResults, roundResults...)
	}
	if thinkingResp.RequiresFurtherPlanning {
		return []ContentChunk{}, allResults, nil // Return intermediate results
	}

	if thinkingResp.RequiresFinalResponse {
		// 1. Generate the final response text using accumulated results
		var finalPrompt strings.Builder
		finalPrompt.WriteString("Here is the original query: ")
		finalPrompt.WriteString(originalQuery)
		finalPrompt.WriteString("\n\nHere are the results from the function calls: ")
		resultsJSON, _ := json.Marshal(allResults)
		finalPrompt.WriteString(string(resultsJSON))
		finalPrompt.WriteString("\n\nPlease provide a final response to the original query based on the results from the function calls.")

		processedText, err := getGeminiResponse(ctx, conn, finalPrompt.String())
		if err != nil {
			return nil, nil, fmt.Errorf("error getting final gemini response: %w", err)
		}
		processedText = strings.TrimSpace(processedText)
		fmt.Printf("Raw final response text from Gemini:\n%s\n", processedText)

		// 2. Inject security IDs directly into the raw response string
		tickerTimestampRegex := regexp.MustCompile(`\$\$\$([A-Z]{1,5})-(\d+)\$\$\$`)
		processedTextWithIds := tickerTimestampRegex.ReplaceAllStringFunc(processedText, func(match string) string {
			return replaceTickerPlaceholder(conn, match)
		})

		// 3. Parse the modified text into ContentChunks
		var contentChunksResponse struct {
			ContentChunks []ContentChunk `json:"content_chunks"`
		}

		// Try parsing the entire modified string as JSON
		if err := json.Unmarshal([]byte(processedTextWithIds), &contentChunksResponse); err == nil && len(contentChunksResponse.ContentChunks) > 0 {
			return contentChunksResponse.ContentChunks, allResults, nil
		}

		// Try finding a JSON block within the modified string
		jsonStartIdx := strings.Index(processedTextWithIds, "{")
		jsonEndIdx := strings.LastIndex(processedTextWithIds, "}")
		if jsonStartIdx != -1 && jsonEndIdx != -1 && jsonEndIdx > jsonStartIdx {
			jsonBlock := processedTextWithIds[jsonStartIdx : jsonEndIdx+1]
			if err := json.Unmarshal([]byte(jsonBlock), &contentChunksResponse); err == nil && len(contentChunksResponse.ContentChunks) > 0 {
				return contentChunksResponse.ContentChunks, allResults, nil
			}
		}

		// Fallback: Treat the modified string as a single text chunk
		return []ContentChunk{{Type: "text", Content: processedTextWithIds}}, allResults, nil
	}

	// If no final response is needed
	return []ContentChunk{}, allResults, nil
}

// processRoundWithGemini sends a round to Gemini for processing and gets back the functions to execute
func processRoundWithGemini(ctx context.Context, conn *utils.Conn, prompt string) ([]FunctionCall, error) {
	// Get a response from Gemini with the processed functions
	response, err := getGeminiFunctionResponse(ctx, conn, prompt)
	if err != nil {
		return nil, fmt.Errorf("error getting function response from Gemini: %w", err)
	}

	// Return the function calls from the response
	return response.FunctionCalls, nil
}

// executeGeminiFunctions executes the function calls returned by Gemini
func executeGeminiFunctions(ctx context.Context, conn *utils.Conn, userID int, functionCalls []FunctionCall) ([]ExecuteResult, error) {
	var results []ExecuteResult

	for _, fc := range functionCalls {
		fmt.Printf("Executing function %s with args: %s\n", fc.Name, string(fc.Args))

		// Parse arguments into a map for storage
		var args interface{}
		if err := json.Unmarshal(fc.Args, &args); err != nil {
			fmt.Printf("Warning: Could not parse args for storage: %v\n", err)
		}

		// Check if the function exists in Tpols map
		tool, exists := GetTools(false)[fc.Name]
		if !exists {
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Error:        fmt.Sprintf("function '%s' not found", fc.Name),
				Args:         args,
			})
			continue
		}

		// Execute the function
		result, err := tool.Function(conn, userID, fc.Args)
		if err != nil {
			fmt.Printf("Function %s execution error: %v\n", fc.Name, err)
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Error:        err.Error(),
				Args:         args,
			})
		} else {
			fmt.Printf("Function %s executed successfully\n", fc.Name)
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Result:       result,
				Args:         args,
			})
		}
	}

	return results, nil
}

type GetSuggestedQueriesResponse struct {
	Suggestions []string `json:"suggestions"`
}

func GetSuggestedQueries(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {

	// Use the standardized Redis connectivity test
	ctx := context.Background()
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		fmt.Printf("WARNING: %s\n", message)
	} else {
		fmt.Println(message)
	}
	conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	conversationData, err := getConversationFromCache(ctx, conn, userID, conversationKey)
	if err != nil || conversationData == nil {
		return GetSuggestedQueriesResponse{}, nil
	}
	var conversationHistory string
	if len(conversationData.Messages) > 0 {
		conversationHistory = buildConversationContext(conversationData.Messages)
	}

	geminiRes, err := getGeminiFunctionThinking(ctx, conn, "suggestedQueriesPrompt", conversationHistory)
	if err != nil {
		return nil, fmt.Errorf("error getting suggested queries from Gemini: %w", err)
	}
	jsonStartIdx := strings.Index(geminiRes.Text, "{")
	jsonEndIdx := strings.LastIndex(geminiRes.Text, "}")
	if jsonStartIdx == -1 || jsonEndIdx == -1 {
		return GetSuggestedQueriesResponse{}, nil
	}
	jsonBlock := geminiRes.Text[jsonStartIdx : jsonEndIdx+1]
	var response GetSuggestedQueriesResponse
	if err := json.Unmarshal([]byte(jsonBlock), &response); err != nil {
		return GetSuggestedQueriesResponse{}, fmt.Errorf("error unmarshalling suggested queries: %w", err)
	}
	return response, nil

}
