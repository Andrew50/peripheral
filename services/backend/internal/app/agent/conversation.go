package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ConversationData represents the data structure for storing conversation context
type ConversationData struct {
	Messages   []ChatMessage `json:"messages"`
	TokenCount int32         `json:"token_count"`
	Timestamp  time.Time     `json:"timestamp"`
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
	Citations     []Citation               `json:"citations,omitempty"`
	TokenCount    int32                    `json:"token_count"`
}

func saveMessageToConversation(conn *data.Conn, userID int, query string, contextItems []map[string]interface{}, contentChunks []ContentChunk, functionCalls []FunctionCall, toolResults []ExecuteResult, tokenCount int32) error {
	message := ChatMessage{
		Query:         query,
		ContextItems:  contextItems,
		ContentChunks: contentChunks,
		FunctionCalls: functionCalls,
		ToolResults:   toolResults,
		TokenCount:    tokenCount,
		Timestamp:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}

	conversation, err := GetConversationFromCache(context.Background(), conn, userID)
	if err != nil {
		return fmt.Errorf("failed to get user conversation: %w", err)
	}
	conversation.Messages = append(conversation.Messages, message)
	conversation.Timestamp = time.Now()

	return saveConversationToCache(context.Background(), conn, userID, fmt.Sprintf("user:%d:conversation", userID), conversation)

}

// saveConversationToCache saves the conversation data to Redis
func saveConversationToCache(ctx context.Context, conn *data.Conn, userID int, cacheKey string, data *ConversationData) error {
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

// GetConversationFromCache retrieves conversation data from Redis
func GetConversationFromCache(ctx context.Context, conn *data.Conn, userID int) (*ConversationData, error) {
	// Get the conversation data from Redis
	cacheKey := fmt.Sprintf("user:%d:conversation", userID)
	cachedValue, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		return &ConversationData{Messages: []ChatMessage{}}, nil
	}

	// Deserialize the conversation data
	var conversationData ConversationData
	err = json.Unmarshal([]byte(cachedValue), &conversationData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize conversation data: %w", err)
	}

	var tokenCount int32 = 0
	// Filter out expired messages
	now := time.Now()
	originalCount := len(conversationData.Messages)
	var validMessages []ChatMessage

	for _, msg := range conversationData.Messages {
		if msg.ExpiresAt.After(now) {
			validMessages = append(validMessages, msg)
			tokenCount += msg.TokenCount
		} else {
			fmt.Printf("Filtering out expired message from %s during retrieval\n",
				msg.Timestamp.Format(time.RFC3339))
		}
	}

	// Update with only valid messages
	conversationData.Messages = validMessages
	conversationData.TokenCount = tokenCount

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
func GetUserConversation(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
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

	conversation, err := GetConversationFromCache(ctx, conn, userID)
	if err != nil {
		// Handle the case when conversation doesn't exist in cache
		if strings.Contains(err.Error(), "redis: nil") {
			fmt.Println("No conversation found in cache, returning empty history")
			// Return empty conversation history instead of error
			return &ConversationData{
				Messages:   []ChatMessage{},
				TokenCount: 0,
				Timestamp:  time.Now(),
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
			Messages:   []ChatMessage{},
			TokenCount: 0,
			Timestamp:  time.Now(),
		}, nil
	}

	return conversation, nil
}
