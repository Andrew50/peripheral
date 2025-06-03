package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ConversationSwitchRequest represents the request for switching conversations
type ConversationSwitchRequest struct {
	ConversationID string `json:"conversation_id"`
}

// ConversationCreateRequest represents the request for creating a new conversation
type ConversationCreateRequest struct {
	Query string `json:"query,omitempty"` // Optional first message
}

// ConversationDeleteRequest represents the request for deleting a conversation
type ConversationDeleteRequest struct {
	ConversationID string `json:"conversation_id"`
}

// NewConversation frontend endpoint to create a new conversation
// Note: The frontend "New Chat" button creates conversations lazily when the first message is sent.
// This endpoint is still available for API clients or explicit conversation creation.
func NewConversation(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var req ConversationCreateRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	// Generate title from query if provided, otherwise use default
	title := "New Conversation"
	if req.Query != "" {
		if len(req.Query) > 40 {
			title = req.Query[:40]
		} else {
			title = req.Query
		}
	}

	conversationID, err := CreateConversationInDB(context.Background(), conn, userID, title)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	// Set as active conversation
	if err := SetActiveConversationID(context.Background(), conn, userID, conversationID); err != nil {
		log.Printf("Warning: failed to set active conversation: %v", err)
	}

	// If there's an initial query, we could optionally handle it here
	// For now, just return the conversation ID
	return map[string]interface{}{
		"conversation_id": conversationID,
		"title":           title,
	}, nil
}

// SwitchConversation frontend endpoint to switch to a different conversation
func SwitchConversation(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var req ConversationSwitchRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	if req.ConversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}

	// Use cached switch function
	conversationData, err := SwitchActiveConversationWithCache(context.Background(), conn, userID, req.ConversationID)
	if err != nil {
		return nil, err
	}

	return conversationData, nil
}

// DeleteConversation frontend endpoint to delete a conversation
func DeleteConversation(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var req ConversationDeleteRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	if req.ConversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}

	// Check if this is the active conversation
	activeConversationID, err := GetActiveConversationIDCached(context.Background(), conn, userID)
	if err != nil {
		log.Printf("Warning: failed to get active conversation ID: %v", err)
	}

	// Delete the conversation
	if err := DeleteConversationInDB(conn, req.ConversationID, userID); err != nil {
		return nil, fmt.Errorf("failed to delete conversation: %w", err)
	}

	// If we deleted the active conversation, clear all cached data
	if activeConversationID == req.ConversationID {
		if err := ClearActiveConversationCache(context.Background(), conn, userID); err != nil {
			log.Printf("Warning: failed to clear active conversation cache: %v", err)
		}
	}

	return map[string]interface{}{
		"success": true,
	}, nil
}

// Helper function to convert DB messages to ConversationData format
func convertDBMessagesToConversationData(dbMessages []DBConversationMessage) *ConversationData {
	var chatMessages []ChatMessage
	var totalTokenCount int32

	for _, dbMsg := range dbMessages {
		chatMsg := ChatMessage{
			Query:            dbMsg.Query,
			ContentChunks:    dbMsg.ContentChunks,
			ResponseText:     dbMsg.ResponseText,
			FunctionCalls:    dbMsg.FunctionCalls,
			ToolResults:      dbMsg.ToolResults,
			ContextItems:     dbMsg.ContextItems,
			SuggestedQueries: dbMsg.SuggestedQueries,
			Timestamp:        dbMsg.CreatedAt,
			Citations:        dbMsg.Citations,
			TokenCount:       int32(dbMsg.TokenCount),
			Status:           dbMsg.Status,
		}

		if dbMsg.CompletedAt != nil {
			chatMsg.CompletedAt = *dbMsg.CompletedAt
		}

		chatMessages = append(chatMessages, chatMsg)
		totalTokenCount += int32(dbMsg.TokenCount)
	}

	// Use latest message timestamp or current time
	timestamp := time.Now()
	if len(dbMessages) > 0 {
		timestamp = dbMessages[len(dbMessages)-1].CreatedAt
	}

	return &ConversationData{
		Messages:   chatMessages,
		TokenCount: totalTokenCount,
		Timestamp:  timestamp,
	}
}
