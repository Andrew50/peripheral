package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// ActiveConversationCache represents the cached conversation data
type ActiveConversationCache struct {
	ConversationID string        `json:"conversation_id"`
	Title          string        `json:"title"`
	Messages       []ChatMessage `json:"messages"`
	TokenCount     int32         `json:"token_count"`
	MessageCount   int           `json:"message_count"`
	LastAccessed   time.Time     `json:"last_accessed"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

const (
	// Cache key patterns
	activeConversationIDKey   = "user:%d:active_conversation_id"
	activeConversationDataKey = "user:%d:active_conversation_data"

	// Cache TTL settings
	activeConversationTTL   = 24 * time.Hour     // 24 hours for conversation data
	activeConversationIDTTL = 7 * 24 * time.Hour // 7 days for conversation ID

	// Maximum messages to cache (to prevent memory bloat)
	maxCachedMessages = 15
)

// GetActiveConversationFromCache retrieves the active conversation from Redis cache
func GetActiveConversationFromCache(ctx context.Context, conn *data.Conn, userID int) (*ActiveConversationCache, error) {
	cacheKey := fmt.Sprintf(activeConversationDataKey, userID)

	data, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss, not an error
		}
		return nil, fmt.Errorf("failed to get conversation from cache: %w", err)
	}

	var conversation ActiveConversationCache
	if err := json.Unmarshal([]byte(data), &conversation); err != nil {
		// Cache corruption, delete the invalid entry
		conn.Cache.Del(ctx, cacheKey)
		return nil, fmt.Errorf("failed to unmarshal cached conversation: %w", err)
	}

	// Update last accessed time
	conversation.LastAccessed = time.Now()

	return &conversation, nil
}

// SetActiveConversationCache stores the active conversation in Redis cache
func SetActiveConversationCache(ctx context.Context, conn *data.Conn, userID int, conversation *ActiveConversationCache) error {
	// Update timestamp
	conversation.LastAccessed = time.Now()

	// Limit messages to prevent memory bloat
	if len(conversation.Messages) > maxCachedMessages {
		// Keep the most recent messages
		conversation.Messages = conversation.Messages[len(conversation.Messages)-maxCachedMessages:]
	}

	cacheKey := fmt.Sprintf(activeConversationDataKey, userID)

	data, err := json.Marshal(conversation)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation for cache: %w", err)
	}

	return conn.Cache.Set(ctx, cacheKey, data, activeConversationTTL).Err()
}

// InvalidateActiveConversationCache removes the active conversation from cache
func InvalidateActiveConversationCache(ctx context.Context, conn *data.Conn, userID int) error {
	cacheKey := fmt.Sprintf(activeConversationDataKey, userID)
	return conn.Cache.Del(ctx, cacheKey).Err()
}

// GetActiveConversationIDCached gets the active conversation ID from Redis
func GetActiveConversationIDCached(ctx context.Context, conn *data.Conn, userID int) (string, error) {
	cacheKey := fmt.Sprintf(activeConversationIDKey, userID)
	conversationID, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // No active conversation
		}
		return "", fmt.Errorf("failed to get active conversation ID: %w", err)
	}
	return conversationID, nil
}

// SetActiveConversationIDCached sets the active conversation ID in Redis
func SetActiveConversationIDCached(ctx context.Context, conn *data.Conn, userID int, conversationID string) error {
	cacheKey := fmt.Sprintf(activeConversationIDKey, userID)
	return conn.Cache.Set(ctx, cacheKey, conversationID, activeConversationIDTTL).Err()
}

// GetActiveConversationWithCache retrieves the active conversation with caching
func GetActiveConversationWithCache(ctx context.Context, conn *data.Conn, userID int) (*ConversationData, error) {
	// First check if we have an active conversation ID
	activeConversationID, err := GetActiveConversationIDCached(ctx, conn, userID)
	if err != nil || activeConversationID == "" {
		// No active conversation
		return &ConversationData{
			Messages:   []ChatMessage{},
			TokenCount: 0,
			Timestamp:  time.Now(),
		}, nil
	}

	// Cache miss or ID mismatch - load from database
	messagesInterface, err := GetConversationMessages(ctx, conn, activeConversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load active conversation from database: %w", err)
	}

	// Type assert to get the actual messages
	messages, ok := messagesInterface.([]DBConversationMessage)
	if !ok {
		return nil, fmt.Errorf("unexpected type returned from GetConversationMessages")
	}

	// Convert DB messages to chat messages
	conversationData := convertDBMessagesToConversationData(messages)

	// Cache the conversation for future access
	cachedConv := &ActiveConversationCache{
		ConversationID: activeConversationID,
		Messages:       conversationData.Messages,
		TokenCount:     conversationData.TokenCount,
		MessageCount:   len(conversationData.Messages),
		UpdatedAt:      conversationData.Timestamp,
		LastAccessed:   time.Now(),
	}

	// Get conversation title from database
	var title string
	err = conn.DB.QueryRow(ctx, "SELECT title FROM conversations WHERE conversation_id = $1", activeConversationID).Scan(&title)
	if err == nil {
		cachedConv.Title = title
	}

	// Store in cache (best effort - don't fail if caching fails)
	if err := SetActiveConversationCache(ctx, conn, userID, cachedConv); err != nil {
		// Log but don't fail the request
		fmt.Printf("Warning: failed to cache conversation: %v\n", err)
	}

	return conversationData, nil
}

// SaveMessageToActiveConversationWithCache saves a message and updates cache
func SaveMessageToActiveConversationWithCache(ctx context.Context, conn *data.Conn, userID int, message ChatMessage) error {
	// Get or create active conversation
	activeConversationID, err := GetActiveConversationIDCached(ctx, conn, userID)
	if err != nil {
		return fmt.Errorf("failed to get active conversation: %w", err)
	}

	// If no active conversation, create a new one
	if activeConversationID == "" {
		title := AutoGenerateConversationTitle(message.Query)
		activeConversationID, err = CreateConversationInDB(ctx, conn, userID, title)
		if err != nil {
			return fmt.Errorf("failed to create new conversation: %w", err)
		}

		if err := SetActiveConversationIDCached(ctx, conn, userID, activeConversationID); err != nil {
			// Log but don't fail - the conversation is created
			fmt.Printf("Warning: failed to set new conversation as active: %v\n", err)
		}
	}

	// Save to database
	if err := SaveConversationMessage(ctx, conn, activeConversationID, userID, message); err != nil {
		return fmt.Errorf("failed to save message to database: %w", err)
	}

	// Update cache - get current cached conversation
	cachedConv, err := GetActiveConversationFromCache(ctx, conn, userID)
	if err == nil && cachedConv != nil {
		// Add message to cache
		cachedConv.Messages = append(cachedConv.Messages, message)
		cachedConv.TokenCount += message.TokenCount
		cachedConv.MessageCount = len(cachedConv.Messages)
		cachedConv.UpdatedAt = time.Now()

		// Update cache
		if err := SetActiveConversationCache(ctx, conn, userID, cachedConv); err != nil {
			fmt.Printf("Warning: failed to update cached conversation: %v\n", err)
		}
	} else {
		// Cache miss or error - invalidate to force reload next time
		InvalidateActiveConversationCache(ctx, conn, userID)
	}

	return nil
}

// UpdateMessageInActiveConversationCache updates a specific message in cache
func UpdateMessageInActiveConversationCache(ctx context.Context, conn *data.Conn, userID int, query string, updateFunc func(*ChatMessage)) error {
	cachedConv, err := GetActiveConversationFromCache(ctx, conn, userID)
	if err != nil || cachedConv == nil {
		// Cache miss - message is already updated in database, invalidate cache
		return InvalidateActiveConversationCache(ctx, conn, userID)
	}

	// Find and update the message
	messageUpdated := false
	for i := len(cachedConv.Messages) - 1; i >= 0; i-- {
		if cachedConv.Messages[i].Query == query {
			updateFunc(&cachedConv.Messages[i])
			messageUpdated = true
			break
		}
	}

	if !messageUpdated {
		// Message not found in cache - invalidate to force reload
		return InvalidateActiveConversationCache(ctx, conn, userID)
	}

	// Update cache metadata
	cachedConv.UpdatedAt = time.Now()

	// Save updated cache
	return SetActiveConversationCache(ctx, conn, userID, cachedConv)
}

// SwitchActiveConversationWithCache switches to a different conversation and updates cache
func SwitchActiveConversationWithCache(ctx context.Context, conn *data.Conn, userID int, conversationID string) (*ConversationData, error) {
	// Verify the conversation exists and belongs to the user
	messagesInterface, err := GetConversationMessages(ctx, conn, conversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to access conversation: %w", err)
	}

	// Type assert to get the actual messages
	messages, ok := messagesInterface.([]DBConversationMessage)
	if !ok {
		return nil, fmt.Errorf("unexpected type returned from GetConversationMessages")
	}

	// Clear old cached conversation
	if err := InvalidateActiveConversationCache(ctx, conn, userID); err != nil {
		fmt.Printf("Warning: failed to invalidate old conversation cache: %v\n", err)
	}

	// Set as active conversation
	if err := SetActiveConversationIDCached(ctx, conn, userID, conversationID); err != nil {
		return nil, fmt.Errorf("failed to set active conversation: %w", err)
	}

	// Convert DB messages to the format expected by frontend
	conversationData := convertDBMessagesToConversationData(messages)

	// Cache the new active conversation
	cachedConv := &ActiveConversationCache{
		ConversationID: conversationID,
		Messages:       conversationData.Messages,
		TokenCount:     conversationData.TokenCount,
		MessageCount:   len(conversationData.Messages),
		UpdatedAt:      conversationData.Timestamp,
		LastAccessed:   time.Now(),
	}

	// Get conversation title
	var title string
	err = conn.DB.QueryRow(ctx, "SELECT title FROM conversations WHERE conversation_id = $1", conversationID).Scan(&title)
	if err == nil {
		cachedConv.Title = title
	}

	// Store in cache (best effort)
	if err := SetActiveConversationCache(ctx, conn, userID, cachedConv); err != nil {
		fmt.Printf("Warning: failed to cache switched conversation: %v\n", err)
	}

	return conversationData, nil
}

// ClearActiveConversationCache clears all cached data for a user
func ClearActiveConversationCache(ctx context.Context, conn *data.Conn, userID int) error {
	// Clear both conversation data and ID
	dataKey := fmt.Sprintf(activeConversationDataKey, userID)
	idKey := fmt.Sprintf(activeConversationIDKey, userID)

	pipe := conn.Cache.Pipeline()
	pipe.Del(ctx, dataKey)
	pipe.Del(ctx, idKey)

	_, err := pipe.Exec(ctx)
	return err
}
