package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
    "backend/utils"

	"github.com/go-redis/redis/v8"
)

// PersistentContextItem represents a single piece of data stored in the persistent context.
type PersistentContextItem struct {
	Key       string          `json:"key"`        // Unique identifier for the context item
	Value     json.RawMessage `json:"value"`      // The actual data, stored as raw JSON
	Timestamp time.Time       `json:"timestamp"`  // When the item was last updated
	ExpiresAt time.Time       `json:"expires_at"` // Optional: When this specific item should expire (zero time means no specific expiration)
}

// PersistentContextData holds all persistent context items for a user.
type PersistentContextData struct {
	Items     map[string]PersistentContextItem `json:"items"`     // Map for efficient key-based access
	Timestamp time.Time                        `json:"timestamp"` // Last update time of the entire context set
}

// --- Core Cache Functions ---

const persistentContextKeyFormat = "user:%d:persistent_context"
const defaultPersistentContextExpiration = 7 * 24 * time.Hour // Default expiration for the whole set
const maxPersistentContextItems = 20                          // Max number of items to keep (pruning)

// savePersistentContext saves the entire persistent context data block to Redis.
func savePersistentContext(ctx context.Context, conn *data.Conn, userID int, data *PersistentContextData) error {
	if data == nil {
		return fmt.Errorf("cannot save nil persistent context data")
	}
	cacheKey := fmt.Sprintf(persistentContextKeyFormat, userID)

	// --- Pruning Logic --- Implement before saving
	now := time.Now()
	validItems := make(map[string]PersistentContextItem)
	for key, item := range data.Items {
		// Remove items with specific expiration dates that have passed
		if !item.ExpiresAt.IsZero() && item.ExpiresAt.Before(now) {
			////fmt.Printf("Pruning expired persistent context item '%s' for user %d\n", key, userID)
			continue // Skip expired item
		}
		validItems[key] = item
	}
	data.Items = validItems

	// Prune by count if necessary (remove oldest items first)
	if len(data.Items) > maxPersistentContextItems {
		// Convert map to slice for sorting
		itemsSlice := make([]PersistentContextItem, 0, len(data.Items))
		for _, item := range data.Items {
			itemsSlice = append(itemsSlice, item)
		}

		// Sort by timestamp (oldest first)
		sort.Slice(itemsSlice, func(i, j int) bool {
			return itemsSlice[i].Timestamp.Before(itemsSlice[j].Timestamp)
		})

		// Keep only the newest 'maxPersistentContextItems'
		itemsToKeep := itemsSlice[len(itemsSlice)-maxPersistentContextItems:]

		// Rebuild the map with only the items to keep
		prunedItems := make(map[string]PersistentContextItem)
		for _, item := range itemsToKeep {
			prunedItems[item.Key] = item
		}
		data.Items = prunedItems
		////fmt.Printf("Pruned persistent context items for user %d to newest %d\n", userID, maxPersistentContextItems)
	}
	// --- End Pruning Logic ---

	data.Timestamp = time.Now() // Update last modified time

	serializedData, err := json.Marshal(data)
	if err != nil {
		////fmt.Printf("Failed to serialize persistent context for user %d: %v\n", userID, err)
		return fmt.Errorf("failed to serialize persistent context: %w", err)
	}

	////fmt.Printf("Saving %d persistent context items for user %d to cache key: %s\n", len(data.Items), userID, cacheKey)
	err = conn.Cache.Set(ctx, cacheKey, serializedData, defaultPersistentContextExpiration).Err()
	if err != nil {
		////fmt.Printf("Failed to save persistent context to Redis for user %d: %v\n", userID, err)
		return fmt.Errorf("failed to save persistent context to cache: %w", err)
	}

	////fmt.Printf("Successfully saved persistent context for user %d to Redis.\n", userID)
	return nil
}

// getPersistentContext retrieves the persistent context data block from Redis.
func getPersistentContext(ctx context.Context, conn *data.Conn, userID int) (*PersistentContextData, error) {
	cacheKey := fmt.Sprintf(persistentContextKeyFormat, userID)

	cachedValue, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss is not an error, just return an empty structure
			return &PersistentContextData{Items: make(map[string]PersistentContextItem), Timestamp: time.Time{}}, nil
		}
		////fmt.Printf("Error retrieving persistent context from Redis for user %d: %v\n", userID, err)
		return nil, fmt.Errorf("failed to retrieve persistent context from cache: %w", err)
	}

	var data PersistentContextData
	if err := json.Unmarshal([]byte(cachedValue), &data); err != nil {
		////fmt.Printf("Failed to deserialize persistent context for user %d: %v\n", userID, err)
		// If deserialization fails, return an empty structure to avoid breaking flows
		return &PersistentContextData{Items: make(map[string]PersistentContextItem), Timestamp: time.Time{}}, nil // Consider logging the error more prominently
	}

	// Ensure Items map is initialized if it was nil after unmarshalling (e.g., from empty JSON `"{}"`)
	if data.Items == nil {
		data.Items = make(map[string]PersistentContextItem)
	}

	// Optional: Filter out expired items during retrieval as well, although savePersistentContext should handle it.
	// This ensures consumers always get non-expired items even if pruning during save failed.
	now := time.Now()
	validItems := make(map[string]PersistentContextItem)
	needsResave := false
	for key, item := range data.Items {
		if !item.ExpiresAt.IsZero() && item.ExpiresAt.Before(now) {
			////fmt.Printf("Filtering expired persistent context item '%s' during retrieval for user %d\n", key, userID)
			needsResave = true
			continue
		}
		validItems[key] = item
	}

	// If items were filtered, update the data and potentially save back
	if needsResave {
		data.Items = validItems
		// Optional: Save the cleaned data back asynchronously
		//go func() {
			bgCtx := context.Background()
			if err := savePersistentContext(bgCtx, conn, userID, &data); err != nil {
                return nil, err
				////fmt.Printf("Error saving persistent context after filtering expired items during get for user %d: %v\n", userID, err)
			}
		//}()
	}

	////fmt.Printf("Retrieved %d persistent context items from cache for user %d.\n", len(data.Items), userID)
	return &data, nil
}

// --- Helper Functions for Modifying Context ---

// AddOrUpdatePersistentContextItem adds or updates a single item in the persistent context.
func AddOrUpdatePersistentContextItem(ctx context.Context, conn *data.Conn, userID int, key string, value interface{}, itemExpiration time.Duration) error {
	// 1. Get current context
	data, err := getPersistentContext(ctx, conn, userID)
	if err != nil {
		return fmt.Errorf("failed to get persistent context before update for key '%s': %w", key, err)
	}

	// Ensure Items map is initialized
	if data.Items == nil {
		data.Items = make(map[string]PersistentContextItem)
	}

	// 2. Marshal the new value to json.RawMessage
	rawValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value for persistent context key '%s': %w", key, err)
	}

	// 3. Create/Update the item
	now := time.Now()
	var expiresAt time.Time
	if itemExpiration > 0 {
		expiresAt = now.Add(itemExpiration)
	}

	data.Items[key] = PersistentContextItem{
		Key:       key,
		Value:     rawValue,
		Timestamp: now,
		ExpiresAt: expiresAt, // Set specific expiration if provided
	}

	// 4. Save the updated context
	if err := savePersistentContext(ctx, conn, userID, data); err != nil {
		return fmt.Errorf("failed to save persistent context after update for key '%s': %w", key, err)
	}

	////fmt.Printf("Successfully added/updated persistent context item '%s' for user %d\n", key, userID)
	return nil
}

// RemovePersistentContextItem removes a specific item from the persistent context by its key.
func RemovePersistentContextItem(ctx context.Context, conn *data.Conn, userID int, key string) error {
	// 1. Get current context
	data, err := getPersistentContext(ctx, conn, userID)
	if err != nil {
		// If context doesn't exist, the item isn't there anyway
		if err.Error() == "redis: nil" || data == nil {
			return nil
		}
		return fmt.Errorf("failed to get persistent context before removing key '%s': %w", key, err)
	}

	// 2. Check if item exists and remove it
	if _, exists := data.Items[key]; exists {
		delete(data.Items, key)
		////fmt.Printf("Removed persistent context item '%s' for user %d\n", key, userID)

		// 3. Save the updated context
		if err := savePersistentContext(ctx, conn, userID, data); err != nil {
			return fmt.Errorf("failed to save persistent context after removing key '%s': %w", key, err)
		}
	} 
    //else {
		////fmt.Printf("Persistent context item '%s' not found for removal for user %d\n", key, userID)
		// Not an error if the item wasn't there
	//}

	return nil
}
// ClearConversationHistory deletes the conversation for a user
func ClearConversationHistory(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	ctx := context.Background()
	conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	fmt.Printf("Attempting to delete conversation for key: %s\n", conversationKey)

	// Delete the conversation history key from Redis
	err := conn.Cache.Del(ctx, conversationKey).Err()
	if err != nil {
		fmt.Printf("Failed to delete conversation from Redis: %v\n", err)
		// Don't return immediately, still try to delete persistent context
	} else {
		fmt.Printf("Successfully deleted conversation for key: %s\n", conversationKey)
	}

	// Also delete the persistent context key
	persistentContextKey := fmt.Sprintf(persistentContextKeyFormat, userID) // Use constant from persistentContext.go
	pErr := conn.Cache.Del(ctx, persistentContextKey).Err()
	if pErr != nil {
		fmt.Printf("Failed to delete persistent context from Redis: %v\n", pErr)
		// If conversation deletion succeeded but this failed, maybe return a specific error?
		// For now, just log it and return the original error if it exists, or this one if not.
		if err == nil { // If conversation delete was ok, return this error
			return nil, fmt.Errorf("failed to clear persistent context: %w", pErr)
		}
	} else {
		fmt.Printf("Successfully deleted persistent context for key: %s\n", persistentContextKey)
	}

	// If the conversation deletion failed initially, return that error now
	if err != nil {
		return nil, fmt.Errorf("failed to clear conversation history: %w", err)
	}

	fmt.Printf("Successfully deleted conversation for key: %s\n", conversationKey)
	return map[string]string{"message": "Conversation history cleared successfully"}, nil
=======
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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
	Citations     []Citation               `json:"citations,omitempty"`
}

func saveMessageToConversation(conn *data.Conn, userID int, query string, contextItems []map[string]interface{}, contentChunks []ContentChunk, functionCalls []FunctionCall, toolResults []ExecuteResult) error {
	message := ChatMessage{
		Query:         query,
		ContextItems:  contextItems,
		ContentChunks: contentChunks,
		FunctionCalls: functionCalls,
		ToolResults:   toolResults,
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
		////fmt.Printf("WARNING: %s\n", message)
		return fmt.Errorf("redis connectivity test failed: %s", message)
	}

	// Filter out expired messages
	now := time.Now()
	var validMessages []ChatMessage
	for _, msg := range data.Messages {
		if msg.ExpiresAt.After(now) {
			validMessages = append(validMessages, msg)
		}
		// else {
		////fmt.Printf("Removing expired message from %s\n", msg.Timestamp.Format(time.RFC3339))
		//}
	}

	// Update the data with only valid messages
	data.Messages = validMessages

	//if len(data.Messages) == 0 {
	////fmt.Println("Warning: Saving empty conversation data to cache (all messages expired)")
	//}
	// Print details about what we're saving
	////fmt.Printf("Saving conversation with %d valid messages to key: %s\n", len(data.Messages), cacheKey)

	// Serialize the conversation data
	serialized, err := json.Marshal(data)
	if err != nil {
		////fmt.Printf("Failed to serialize conversation data: %v\n", err)
		return fmt.Errorf("failed to serialize conversation data: %w", err)
	}

	// Save to Redis without an expiration - we'll handle expiration at the message level
	err = conn.Cache.Set(ctx, cacheKey, serialized, 0).Err()
	if err != nil {
		////fmt.Printf("Failed to save conversation to Redis: %v\n", err)
		return fmt.Errorf("failed to save conversation to cache: %w", err)
	}

	// Verify the data was saved correctly
	verification, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		////fmt.Printf("Failed to verify saved conversation: %v\n", err)
		return fmt.Errorf("failed to verify saved conversation: %w", err)
	}

	var verifiedData ConversationData
	if err := json.Unmarshal([]byte(verification), &verifiedData); err != nil {
		////fmt.Printf("Failed to parse verified conversation: %v\n", err)
		return fmt.Errorf("failed to parse verified conversation: %w", err)
	}

	////fmt.Printf("Successfully saved and verified conversation to Redis. Verified %d messages.\n", len(verifiedData.Messages))

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

	// Filter out expired messages
	now := time.Now()
	originalCount := len(conversationData.Messages)
	var validMessages []ChatMessage

	for _, msg := range conversationData.Messages {
		if msg.ExpiresAt.After(now) {
			validMessages = append(validMessages, msg)
		} // else {
		////fmt.Printf("Filtering out expired message from %s during retrieval\n", msg.Timestamp.Format(time.RFC3339))
		//}
	}

	// Update with only valid messages
	conversationData.Messages = validMessages

	// If we filtered out messages, update the cache
	if len(validMessages) < originalCount {
		////fmt.Printf("Removed %d expired messages from conversation\n", originalCount-len(validMessages))

		// Save the updated conversation back to cache if we have at least one valid message
		if len(validMessages) > 0 {
			bgCtx := context.Background()
			if err := saveConversationToCache(bgCtx, conn, userID, cacheKey, &conversationData); err != nil {
				return nil, err

				////fmt.Printf("Failed to update cache after filtering expired messages: %v\n", err)
			}
		} else if originalCount > 0 {
			// All messages expired, so we should delete the conversation entirely
			//go func() {
			bgCtx := context.Background()
			if err := conn.Cache.Del(bgCtx, cacheKey).Err(); err != nil {
				return nil, err

				////fmt.Printf("Failed to delete empty conversation after all messages expired: %v\n", err)
			}
			//else {
			////fmt.Printf("Deleted conversation %s as all messages expired\n", cacheKey)
			//}
			//}()
		}
	}

	return &conversationData, nil
}

// GetUserConversation retrieves the conversation for a user
func GetUserConversation(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	ctx := context.Background()

	// Test Redis connectivity before attempting to retrieve conversation
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		return nil, fmt.Errorf("%s", message)
		////fmt.Printf("WARNING: %s\n", message)
	}
	//conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	////fmt.Println("GetUserConversation", conversationKey)

	conversation, err := GetConversationFromCache(ctx, conn, userID)
	if err != nil {
		// Handle the case when conversation doesn't exist in cache
		if strings.Contains(err.Error(), "redis: nil") {
			////fmt.Println("No conversation found in cache, returning empty history")
			// Return empty conversation history instead of error
			return &ConversationData{
				Messages:  []ChatMessage{},
				Timestamp: time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to get user conversation: %w", err)
	}

	// Log the conversation data for debugging
	////fmt.Printf("Retrieved conversation: %+v\n", conversation)
	/*if conversation != nil {
		fmt.Printf("Number of messages: %d\n", len(conversation.Messages))
	}*/

	// Ensure we're returning valid data
	if conversation == nil || len(conversation.Messages) == 0 {
		////fmt.Println("Conversation was retrieved but has no messages, returning empty history")
		return &ConversationData{
			Messages:  []ChatMessage{},
			Timestamp: time.Now(),
		}, nil
	}

	return conversation, nil
}
