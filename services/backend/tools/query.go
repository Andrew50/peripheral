package tools

import (
	"context"
    "strconv"
    "encoding/json"
    "fmt"
    "log" // Import the log package
    "os"
    "strings"
    "time"

    "backend/utils"

    "github.com/tmc/langchaingo/chains"
    "github.com/tmc/langchaingo/llms"
    "github.com/tmc/langchaingo/llms/googleai"
    "github.com/tmc/langchaingo/outputparser"
    "github.com/tmc/langchaingo/prompts"

	"github.com/go-redis/redis/v8"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/schema" // Corrected import usage
	ltools "github.com/tmc/langchaingo/tools" // Using alias ltools consistently
)

// RedisHistoryTool exposes cached conversation context as a LangChain tool.
// Returns JSON so the LLM can parse or pass on.

type RedisHistoryTool struct {
    Conn   *utils.Conn
    UserID int
}

func (t RedisHistoryTool) Name() string        { return "redis_history" }
func (t RedisHistoryTool) Description() string { return "Return the user's last 10 conversation messages as JSON" }

func (t RedisHistoryTool) Call(ctx context.Context, _ string) (string, error) {
    convKey := "user:" + fmt.Sprint(t.UserID) + ":conversation"
    data, err := t.Conn.Cache.Get(ctx, convKey).Result()
    if err != nil {
        return "{}", nil // empty history is OK
    }
    return data, nil
}

// Generic wrapper for legacy tool funcs so you don’t have to hand‑code every schema.

func WrapFunc(name, desc string, fn func(*utils.Conn, int, json.RawMessage) (interface{}, error), conn *utils.Conn, uid int) ltools.Tool {
    return funcTool{name: name, desc: desc, inner: fn, conn: conn, uid: uid}
}

type funcTool struct {
    name        string
    desc        string
    inner       func(*utils.Conn, int, json.RawMessage) (interface{}, error)
    conn        *utils.Conn
    uid         int
    singleParam string // key of the only param ("" = multi‑param)
}

func (f funcTool) Name() string        { return f.name }
func (f funcTool) Description() string { return f.desc }

func (f funcTool) Call(ctx context.Context, arg string) (string, error) {
    log.Printf("DEBUG: %s received raw arg: %q", f.name, arg)

    // ① Fast‑path: already a JSON object/array → use as‑is
    trimmed := strings.TrimSpace(arg)
    if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
        return f.execute(ctx, json.RawMessage(trimmed))
    }

    // ② If this tool has exactly ONE parameter, wrap raw value into an object
    if f.singleParam != "" {
        // quote bare words so `"AAPL"` not `AAPL`
        // Also check for boolean and null literals
        if !(strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"")) &&
            !isNumber(trimmed) && trimmed != "true" && trimmed != "false" && trimmed != "null" {
            trimmed = strconv.Quote(trimmed) // Use strconv.Quote for proper string literal quoting
        }
        wrapped := fmt.Sprintf(`{"%s":%s}`, f.singleParam, trimmed)
        log.Printf("DEBUG: %s wrapped single param arg: %s", f.name, wrapped)
        return f.execute(ctx, json.RawMessage(wrapped))
    }

    // ③ Fallback: treat whatever we got as a JSON‑encoded string
    // This case might be less common if schemas are well-defined, but acts as a safety net.
    // Use strconv.Quote for robust JSON string encoding.
    wrapped := strconv.Quote(trimmed)
    log.Printf("DEBUG: %s wrapped fallback arg: %s", f.name, wrapped)
    // Note: Passing a simple JSON string might not be what most handlers expect if they
    // anticipate an object. Consider if this fallback is truly desired or if an error
    // should be returned if steps ① and ② fail. For now, implementing as requested.
    // If handlers expect objects, this might cause downstream unmarshal errors in f.inner.
    // A potentially safer fallback might be returning an error:
    // return "", fmt.Errorf("tool %q received non-JSON arg '%s' and is not a single-parameter tool", f.name, arg)
    return f.execute(ctx, json.RawMessage(wrapped)) // Passing the quoted string as RawMessage
}

// helper that runs the inner handler + marshals result
func (f funcTool) execute(ctx context.Context, raw json.RawMessage) (string, error) {
    // Validate the final JSON before passing to the inner function
    var dummy interface{}
    if err := json.Unmarshal(raw, &dummy); err != nil {
        // Log the raw JSON that failed validation
        log.Printf("ERROR: Tool %q argument failed final JSON validation: %s", f.name, string(raw))
        return "", fmt.Errorf("tool %q argument is not valid JSON: %w. Raw: %s", f.name, err, string(raw))
    }

    log.Printf("DEBUG: Tool %q executing with validated JSON args: %s", f.name, string(raw))
    out, err := f.inner(f.conn, f.uid, raw)
    if err != nil {
        // Log the error from the inner tool function execution, including the args used
        log.Printf("ERROR: Tool %q inner execution failed with args %s: %v", f.name, string(raw), err)
        return "", fmt.Errorf("tool %q execution failed: %w", f.name, err) // Propagate the error from the tool
    }

    // Marshal the output from the tool function into a JSON string for the agent.
    b, err := json.Marshal(out)
    if err != nil {
        log.Printf("ERROR: Failed to marshal output from tool %q: %v", f.name, err)
        return "", fmt.Errorf("failed to marshal output from tool %q: %w", f.name, err)
    }
    return string(b), nil
}

// very small numeric detector
func isNumber(s string) bool {
    // Trim spaces just in case
    s = strings.TrimSpace(s)
    if s == "" {
        return false
    }
    _, err := strconv.ParseFloat(s, 64)
    // Consider integer parsing as well for completeness, although ParseFloat handles integers
    // _, intErr := strconv.ParseInt(s, 10, 64)
    // return err == nil || intErr == nil
    return err == nil
}

// Runner wraps an agents.Executor.
type Runner struct{ exec *agents.Executor }

func NewRunner(conn *utils.Conn, userID int) (*Runner, error) {
    ctx := context.Background()

    apiKey, err := conn.GetGeminiKey()
    if err != nil { return nil, err }

    llm, err := googleai.New(
        ctx,                                 // <-- first arg must be ctx :contentReference[oaicite:2]{index=2}
        googleai.WithAPIKey(apiKey),         // auth option
        googleai.WithDefaultModel("gemini-2.0-flash-thinking-exp"), // correct helper :contentReference[oaicite:3]{index=3}
    )
    if err != nil { return nil, fmt.Errorf("init gemini: %w", err) }

    // Get the map of tool definitions filtered for query usage (IsQuery=true)
    toolDefinitions := GetTools(false) // false means get tools for query/agent use

    // Convert the map to a slice of ltools.Tool using the updated convertRegistry
    agentTools := convertRegistry(toolDefinitions, conn, userID)

    // Note: RedisHistoryTool is not defined using the standard Tool struct in tools.go,
    // so it won't be picked up by GetTools/convertRegistry.
    // We need to add it manually if it's required by the agent.
    // Check if RedisHistoryTool is intended for agent use. If so, uncomment:
    // agentTools = append(agentTools, RedisHistoryTool{Conn: conn, UserID: userID})
    // Based on its description ("Return the user's last 10 conversation messages as JSON"),
    // it seems plausible it might be useful for the agent.

    // Log the tools being provided to the agent
    toolNames := make([]string, len(agentTools))
    for i, tool := range agentTools {
        toolNames[i] = tool.Name()
    }
    log.Printf("DEBUG: Initializing OneShotAgent with tools: %v", toolNames)

    // Updated system prompt for better guidance
    systemPrompt := `You are a helpful financial assistant.
Your goal is to answer the user's query based on the information you can access through the available tools.
Use the tools provided when necessary to answer questions about stocks, market data, user trades, watchlists, etc.
If a query seems unrelated to finance or the available tools, or if it is too vague like "test", respond naturally or ask for clarification.
Do not make up information if the tools do not provide it.
If the user asks for the keyword, it is GOOGLE.`

    agent := agents.NewOneShotAgent(llm, agentTools, agents.WithPromptPrefix(systemPrompt)) // Removed trailing comma
    executor := agents.NewExecutor(agent)
    return &Runner{exec: executor}, nil
}

func (r *Runner) Run(ctx context.Context, q string) (string, error) {
	log.Printf("DEBUG: Agent Runner received query: %q", q) // Log the input query
	answer, err := chains.Run(ctx, r.exec, q)
	if err != nil {
		log.Printf("ERROR: Agent Runner chains.Run failed: %v", err) // Log the error if any
		// It's possible the error message itself explains the empty content
		// Return the error directly
		return "", err
	}
	// Log the raw answer *before* returning
	log.Printf("DEBUG: Agent Runner chains.Run raw answer: %q", answer)
	if answer == "" {
		log.Printf("WARN: Agent Runner chains.Run returned an empty answer for query: %q", q)
		// Potentially return a more specific error here if needed,
		// but for now, let's rely on the existing error handling in api.go
		// which seems to be catching this ("no content in generation response").
	}
	return answer, nil
}

// FunctionCall represents a function to be called with its arguments
type FunctionCall struct {
	Name   string          `json:"name"`
	CallID string          `json:"call_id,omitempty"`
	Args   json.RawMessage `json:"args,omitempty"`
}
// ChatMessage represents a single message in the conversation
type ChatMessage struct {
	Query         string          `json:"query"`
	ContentChunks []ContentChunk  `json:"content_chunks,omitempty"`
	ResponseText  string          `json:"response_text,omitempty"`
	FunctionCalls []FunctionCall  `json:"function_calls"`
	ToolResults   []ExecuteResult `json:"tool_results"`
	Timestamp     time.Time       `json:"timestamp"`
	ExpiresAt     time.Time       `json:"expires_at"` // When this message should expire
}

// ContentChunk represents a piece of content in the response sequence
type ContentChunk struct {
	Type    string      `json:"type"`    // "text" or "table" (or others later, e.g., "image")
	Content interface{} `json:"content"` // string for "text", TableData for "table"
}
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

type Query struct {
	Query string `json:"query"`
}
type QueryResponse struct {
	Type          string            `json:"type"` //"mixed_content", "function_calls", "simple_text"
	ContentChunks []ContentChunk    `json:"content_chunks,omitempty"`
	Text          string            `json:"text,omitempty"`
	Results       []ExecuteResult   `json:"results,omitempty"`
	History       *ConversationData `json:"history,omitempty"`
}


func GetQuery(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
    var req Query
    if err := json.Unmarshal(args, &req); err != nil {
        return nil, err
    }

    runner, err := NewRunner(conn, userID)
    if err != nil {
        return nil, err
    }
    answer, err := runner.Run(context.Background(), req.Query)
    if err != nil {
        return nil, err
    }
    return QueryResponse{Type: "text", Text: answer}, nil
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
func ClearConversationHistory(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	ctx := context.Background()
	conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	fmt.Printf("Attempting to delete conversation for key: %s\n", conversationKey)

	// Delete the key from Redis
	err := conn.Cache.Del(ctx, conversationKey).Err()
	if err != nil {
		fmt.Printf("Failed to delete conversation from Redis: %v\n", err)
		return nil, fmt.Errorf("failed to clear conversation history: %w", err)
	}

	fmt.Printf("Successfully deleted conversation for key: %s\n", conversationKey)
	return map[string]string{"message": "Conversation history cleared successfully"}, nil
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


// RedisChatHistory provides a langchaingo/memory.ChatMessageHistory implementation
// backed by Redis lists.error you listed. | Err msg group | Root cause | Minimal patch | |---------------|------------|---------------| | **`td.FunctionDeclaration undefined`** | Your `Tool` struct now uses th
type RedisChatHistory struct {
	Client *redis.Client
	Key    string        // Redis key for the chat history list, e.g., "user:42:chat"
	TTL    time.Duration // Time-to-live for the history key. 0 means no expiry.
}

// NewRedisChatHistory creates a new RedisChatHistory.
func NewRedisChatHistory(client *redis.Client, key string, ttl time.Duration) *RedisChatHistory {
	return &RedisChatHistory{
		Client: client,
		Key:    key,
		TTL:    ttl,
	}
}

// push adds a message to the end of the Redis list and updates TTL.
func (h *RedisChatHistory) push(ctx context.Context, msg llms.ChatMessage) error { // Use llms.ChatMessage
	// Marshal the message based on its type to preserve type information
	var dataToStore []byte
	var err error
	var typedMsg map[string]interface{}

	// Use llms types correctly
	switch m := msg.(type) {
	case llms.HumanChatMessage: // Use llms.HumanChatMessage
		typedMsg = map[string]interface{}{"type": "human", "content": m.GetContent()}
	case llms.AIChatMessage: // Use llms.AIChatMessage
		typedMsg = map[string]interface{}{"type": "ai", "content": m.GetContent()}
	case llms.SystemChatMessage: // Use llms.SystemChatMessage
		typedMsg = map[string]interface{}{"type": "system", "content": m.GetContent()}
	case llms.GenericChatMessage: // Use llms.GenericChatMessage
		typedMsg = map[string]interface{}{"type": "generic", "content": m.GetContent(), "role": m.Role}
	// TODO: Add cases for other types like FunctionChatMessage, ToolChatMessage if needed
	default:
		// Fallback for unknown types - attempt generic marshal, might lose specific type info
		fmt.Printf("Warning: marshaling unknown chat message type: %T\n", msg)
		dataToStore, err = json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal unknown chat message type: %w", err)
		}
		// Push the already marshaled data and return
		if err := h.Client.RPush(ctx, h.Key, dataToStore).Err(); err != nil {
			return fmt.Errorf("failed to push message to redis: %w", err)
		}
		// Update TTL if set (duplicate logic, consider refactoring)
		if h.TTL > 0 {
			if err := h.Client.Expire(ctx, h.Key, h.TTL).Err(); err != nil {
				fmt.Printf("Warning: failed to set TTL for key %s: %v\n", h.Key, err)
			}
		}
		return nil // Exit early for default case
	}

	// Marshal the structured message for known types
	dataToStore, err = json.Marshal(typedMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal chat message: %w", err)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal chat message: %w", err)
	}

	// Push the JSON string to the Redis list
	if err := h.Client.RPush(ctx, h.Key, dataToStore).Err(); err != nil {
		return fmt.Errorf("failed to push message to redis: %w", err)
	}

	// Update TTL if set
	if h.TTL > 0 {
		if err := h.Client.Expire(ctx, h.Key, h.TTL).Err(); err != nil {
			// Log error but don't fail the operation just because TTL update failed
			fmt.Printf("Warning: failed to set TTL for key %s: %v\n", h.Key, err)
		}
	}
	return nil
}

// AddUserMessage adds a human message to the history.
func (h *RedisChatHistory) AddUserMessage(ctx context.Context, text string) error {
	return h.push(ctx, llms.HumanChatMessage{Content: text}) // Use llms.HumanChatMessage
}

// AddAIMessage adds an AI message to the history.
func (h *RedisChatHistory) AddAIMessage(ctx context.Context, text string) error {
	return h.push(ctx, llms.AIChatMessage{Content: text}) // Use llms.AIChatMessage
}

// Messages retrieves all messages from the history.
func (h *RedisChatHistory) Messages(ctx context.Context) ([]llms.ChatMessage, error) { // Use llms.ChatMessage
	// Retrieve all elements from the list
	vals, err := h.Client.LRange(ctx, h.Key, 0, -1).Result()
	if err != nil {
		// If the key doesn't exist, return empty history, not an error
		if err == redis.Nil {
			return []llms.ChatMessage{}, nil // Use llms.ChatMessage
		}
		return nil, fmt.Errorf("failed to retrieve messages from redis: %w", err)
	}

	// Unmarshal each message back into the correct llms.ChatMessage type
	out := make([]llms.ChatMessage, 0, len(vals)) // Use llms.ChatMessage
	for _, v := range vals {
		var base map[string]interface{}
		if err := json.Unmarshal([]byte(v), &base); err != nil {
			fmt.Printf("Warning: failed to unmarshal base message structure from redis: %v\n", err)
			continue // Skip malformed messages
		}

		contentType, _ := base["type"].(string)
		contentValue, _ := base["content"].(string)

		var msg llms.ChatMessage // Use llms.ChatMessage
		switch contentType {
		case "human":
			msg = llms.HumanChatMessage{Content: contentValue} // Use llms.HumanChatMessage
		case "ai":
			msg = llms.AIChatMessage{Content: contentValue} // Use llms.AIChatMessage
		case "system":
			msg = llms.SystemChatMessage{Content: contentValue} // Use llms.SystemChatMessage
		case "generic":
			roleValue, _ := base["role"].(string)
			msg = llms.GenericChatMessage{Content: contentValue, Role: roleValue} // Use llms.GenericChatMessage
		// Add cases for other types if stored differently
		default:
			// Attempt a generic unmarshal if type is unknown or missing
			// Note: This generic unmarshal might not work correctly for llms.ChatMessage
			// as it's an interface. Consider logging an error or handling specific types.
			fmt.Printf("Warning: encountered unknown message type '%s' during retrieval\n", contentType)
			// Attempt to create a generic message if possible, otherwise skip
			if role, ok := base["role"].(string); ok {
				msg = llms.GenericChatMessage{Content: contentValue, Role: role}
			} else {
				fmt.Printf("Warning: skipping message of unknown type '%s' without a role\n", contentType)
				continue // Skip message if it cannot be represented generically
			}

			/* Original generic unmarshal attempt - likely problematic for interfaces
			if err := json.Unmarshal([]byte(v), &msg); err != nil {
				fmt.Printf("Warning: failed to unmarshal message of unknown type '%s' from redis: %v\n", contentType, err)
				continue // Skip message if completely unparsable
			}
			*/
			if err := json.Unmarshal([]byte(v), &msg); err != nil {
				fmt.Printf("Warning: failed to unmarshal message of unknown type '%s' from redis: %v\n", contentType, err)
				continue // Skip message if completely unparsable
			}
		}
		out = append(out, msg)
	}
	return out, nil
}

// Clear removes all messages from the history.
func (h *RedisChatHistory) Clear(ctx context.Context) error {
	err := h.Client.Del(ctx, h.Key).Err()
	if err != nil && err != redis.Nil { // Ignore error if key already doesn't exist
		return fmt.Errorf("failed to clear chat history from redis: %w", err)
	}
	return nil
}

// AddMessage stores an arbitrary llms.ChatMessage
func (h *RedisChatHistory) AddMessage(ctx context.Context, msg llms.ChatMessage) error { // Use llms.ChatMessage
	// we already wrote push(), so just call it:
	return h.push(ctx, msg)
}

// SetMessages replaces the entire history
func (h *RedisChatHistory) SetMessages(ctx context.Context, msgs []llms.ChatMessage) error { // Use llms.ChatMessage
	// Clear existing history first
	if err := h.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear history before setting messages: %w", err)
	}
	// Add new messages
	for _, m := range msgs {
		if err := h.AddMessage(ctx, m); err != nil {
			// Log error but attempt to continue adding other messages
			fmt.Printf("Warning: failed to add message during SetMessages: %v\n", err)
		}
	}
	return nil
}

// Ensure RedisChatHistory implements the interface
var _ schema.ChatMessageHistory = (*RedisChatHistory)(nil) // This interface is still in schema package

// funcTool adapts a standard Go function to the langchaingo tools.Tool interface.
// Name returns the name of the tool.

// convertRegistry turns local Tool specs into langchaingo tools.
func convertRegistry(reg map[string]Tool, conn *utils.Conn, uid int) []ltools.Tool {
    res := make([]ltools.Tool, 0, len(reg))
    for _, t := range reg {
        if t.Handler == nil {
            continue // nothing to wrap
        }

        name, desc := "unknown_tool", "no description"
        if t.Definition != nil {
            if t.Definition.Name != "" {
                name = t.Definition.Name
            }
            if t.Definition.Description != "" {
                desc = t.Definition.Description
            }
        }

        // --- NEW: figure out if schema has exactly one property -------------
        single := ""
        // Check if Definition and Parameters exist
        if t.Definition != nil && t.Definition.Parameters != nil {
            // Assert Parameters to json.RawMessage (which is []byte)
            paramsJSON, ok := t.Definition.Parameters.(json.RawMessage)
            if !ok {
                log.Printf("WARN: Tool '%s' Parameters field is not json.RawMessage, type is %T", name, t.Definition.Parameters)
            } else if len(paramsJSON) > 0 { // Check length only if it's valid json.RawMessage
                // Define a struct to capture the 'properties' field from the JSON schema
                var schema struct {
                    Properties map[string]json.RawMessage `json:"properties"`
                    // We might also need 'required' if we only want to trigger this
                    // for single *required* parameters, but for now, just checking
                    // the number of properties seems sufficient based on the request.
                }
                // Attempt to unmarshal the parameters JSON into our struct
                if err := json.Unmarshal(paramsJSON, &schema); err == nil {
                    // Check if exactly one property is defined
                    if len(schema.Properties) == 1 {
                        // If yes, iterate (only once) to get the key (parameter name)
                        for k := range schema.Properties {
                            single = k
                            break // Found the single key, no need to continue loop
                        }
                        log.Printf("DEBUG: Tool '%s' identified as single-parameter tool with param '%s'", name, single)
                    } else {
                        // Log if properties exist but not exactly one
                        if len(schema.Properties) > 1 {
                            log.Printf("DEBUG: Tool '%s' has multiple parameters (%d), not marking as single-parameter.", name, len(schema.Properties))
                        } else {
                            // len == 0
                            log.Printf("DEBUG: Tool '%s' has zero parameters defined in schema.", name)
                        }
                    }
                } else {
                    // Log if unmarshalling the schema failed
                    log.Printf("WARN: Failed to unmarshal parameters schema for tool '%s': %v. Raw schema: %s", name, err, string(paramsJSON))
                }
            } else {
                 // Parameters field exists but is empty
                 log.Printf("DEBUG: Tool '%s' has empty Parameters defined.", name)
            }
        } else {
            // No Definition or no Parameters field
            log.Printf("DEBUG: Tool '%s' has no Definition or Parameters field.", name)
        }
        // --------------------------------------------------------------------

        res = append(res, funcTool{
            name:        name,
            desc:        desc,
            inner:       t.Handler,
            conn:        conn,
            uid:         uid,
            singleParam: single, // <- pass it on
        })
    }
    return res
}
type GetSuggestedQueriesResponse struct {
	Suggestions []string `json:"suggestions"`
}

func GetSuggestedQueries(conn *utils.Conn, userID int, _ json.RawMessage) (interface{}, error) {
    ctx := context.Background()

    // ---- 1) Build chat history ------------------------------------------------
    conversationKey := fmt.Sprintf("user:%d:conversation", userID)
    conv, _ := getConversationFromCache(ctx, conn, userID, conversationKey) // ignore "not found" errors
    var history string
    if conv != nil && len(conv.Messages) > 0 {
        history = buildConversationContext(conv.Messages)
    }

    // ---- 2) Load the prompt template ------------------------------------------
    // Adjusted path relative to the /app working directory inside the container
    const promptPath = "tools/prompts/suggestedQueriesPrompt.txt"

    // Log the current working directory to debug file path issues
    wd, err := os.Getwd()
    if err != nil {
        log.Printf("Error getting working directory: %v", err)
    } else {
        log.Printf("Attempting to read prompt file from path: %s (relative to working directory: %s)", promptPath, wd)
    }

    tplBytes, err := os.ReadFile(promptPath)
    if err != nil {
        // Log the error with more context
        log.Printf("Error reading prompt file %q from working directory %q: %v", promptPath, wd, err)
        return nil, fmt.Errorf("read prompt template %q: %w", promptPath, err)
    }
    tpl := prompts.NewPromptTemplate(string(tplBytes), []string{"history"})

    // ---- 3) Initialise the Gemini LLM -----------------------------------------
    apiKey, err := conn.GetGeminiKey()
    if err != nil {
        return nil, fmt.Errorf("get Gemini key: %w", err)
    }
    llm, err := googleai.New(
        ctx,
        googleai.WithAPIKey(apiKey),
        googleai.WithDefaultModel("gemini-1.5-pro-latest"),
    )
    if err != nil {
        return nil, fmt.Errorf("init Gemini client: %w", err)
    }

    // ---- 4) Build & run the chain ---------------------------------------------
    chain := chains.NewLLMChain(llm, tpl) // simple LLMChain :contentReference[oaicite:0]{index=0}

    // Set the output parser and key directly on the chain instance
    chain.OutputParser = outputparser.NewSimple()
    chain.OutputKey = "text" // Assuming the LLM response key is "text"

    out, err := chains.Call(
        ctx,
        chain,
        map[string]any{"history": history},
        // chains.WithOutputParser(outputparser.NewSimple()), // Removed from here
        chains.WithTemperature(0.4),
    )
    if err != nil {
        return nil, fmt.Errorf("Gemini call failed: %w", err)
    }

    // ---- 5) Parse JSON block from the LLM response ----------------------------
    text := out["text"].(string)
    start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
    if start == -1 || end == -1 || end <= start {
        return GetSuggestedQueriesResponse{}, nil // model didn’t return JSON
    }

    var resp GetSuggestedQueriesResponse
    if err := json.Unmarshal([]byte(text[start:end+1]), &resp); err != nil {
        return GetSuggestedQueriesResponse{}, fmt.Errorf("unmarshal suggestions: %w", err)
    }
    return resp, nil
}
