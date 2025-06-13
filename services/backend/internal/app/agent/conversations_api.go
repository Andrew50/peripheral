package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// ConversationSwitchRequest represents the request for switching conversations
type ConversationSwitchRequest struct {
	ConversationID string `json:"conversation_id"`
}

// ConversationDeleteRequest represents the request for deleting a conversation
type ConversationDeleteRequest struct {
	ConversationID string `json:"conversation_id"`
}

// ConversationCreateRequest represents the request for creating a new conversation
type ConversationCreateRequest struct {
	Query string `json:"query,omitempty"` // Optional first message
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
	var totalTokenCount int

	for _, dbMsg := range dbMessages {
		chatMsg := ChatMessage{
			MessageID:        dbMsg.MessageID,
			Query:            dbMsg.Query,
			ContentChunks:    dbMsg.ContentChunks,
			ResponseText:     dbMsg.ResponseText,
			FunctionCalls:    dbMsg.FunctionCalls,
			ToolResults:      dbMsg.ToolResults,
			ContextItems:     dbMsg.ContextItems,
			SuggestedQueries: dbMsg.SuggestedQueries,
			Timestamp:        dbMsg.CreatedAt,
			Citations:        dbMsg.Citations,
			TokenCount:       dbMsg.TokenCount,
			Status:           dbMsg.Status,
		}

		if dbMsg.CompletedAt != nil {
			chatMsg.CompletedAt = *dbMsg.CompletedAt
		}

		chatMessages = append(chatMessages, chatMsg)
		totalTokenCount += chatMsg.TokenCount
	}

	// Use latest message timestamp or current time
	timestamp := time.Now()
	if len(dbMessages) > 0 {
		timestamp = dbMessages[len(dbMessages)-1].CreatedAt
	}

	return &ConversationData{
		Messages:  chatMessages,
		Timestamp: timestamp,
	}
}

// EditMessageRequest represents the request for editing a message
type EditMessageRequest struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"` // Required: Message ID to edit
	NewQuery       string `json:"new_query"`  // New message content
}

// EditMessageResponse represents the response after editing a message
type EditMessageResponse struct {
	Success        bool   `json:"success"`
	ConversationID string `json:"conversation_id"`
}

// EditMessage frontend endpoint to edit a message and regenerate response
func EditMessage(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var req EditMessageRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	// Validate required fields
	if req.ConversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}
	if req.MessageID == "" {
		return nil, fmt.Errorf("message_id is required")
	}
	if req.NewQuery == "" {
		return nil, fmt.Errorf("new_query is required")
	}

	// Start transaction for atomic edit operation
	tx, err := conn.DB.Begin(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	// Validate user owns the conversation
	if err = VerifyConversationOwnership(conn, req.ConversationID, userID); err != nil {
		return nil, err
	}

	// Find the message to edit
	var messageOrder int
	var foundQuery string
	querySQL := `SELECT message_order, query FROM conversation_messages WHERE conversation_id = $1 AND message_id = $2 AND archived = FALSE`
	err = tx.QueryRow(context.Background(), querySQL, req.ConversationID, req.MessageID).Scan(&messageOrder, &foundQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to find message to edit: %w", err)
	}

	// Validate the message can be edited
	var status string
	validateSQL := `SELECT status FROM conversation_messages WHERE conversation_id = $1 AND message_order = $2 AND archived = FALSE`
	err = tx.QueryRow(context.Background(), validateSQL, req.ConversationID, messageOrder).Scan(&status)
	if err != nil {
		return nil, fmt.Errorf("failed to validate message: %w", err)
	}
	if status == "pending" {
		return nil, fmt.Errorf("cannot edit a message that is currently being processed")
	}

	// Archive all messages after this one (preserve them for logging)
	archiveSQL := `UPDATE conversation_messages SET archived = TRUE WHERE conversation_id = $1 AND message_order >= $2 AND archived = FALSE`
	_, err = tx.Exec(context.Background(), archiveSQL, req.ConversationID, messageOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to archive messages: %w", err)
	}

	// Update the message content - set status to completed since we're not regenerating here
	updateSQL := `
		UPDATE conversation_messages 
		SET query = $1, status = $2, completed_at = NULL, response_text = '', 
		    content_chunks = '[]', function_calls = '[]', tool_results = '[]', 
		    suggested_queries = '[]', citations = '[]', token_count = 0
		WHERE conversation_id = $3 AND message_order = $4`
	_, err = tx.Exec(context.Background(), updateSQL, req.NewQuery, "completed", req.ConversationID, messageOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}

	// Update conversation metadata
	metadataSQL := `
		UPDATE conversations 
		SET message_count = (
			SELECT COUNT(*) FROM conversation_messages 
			WHERE conversation_id = $1 AND archived = FALSE
		),
		total_token_count = (
			SELECT COALESCE(SUM(token_count), 0) FROM conversation_messages 
			WHERE conversation_id = $1 AND archived = FALSE
		),
		updated_at = $2
		WHERE conversation_id = $1`
	_, err = tx.Exec(context.Background(), metadataSQL, req.ConversationID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to update conversation metadata: %w", err)
	}

	// Get the context items from the original message for regeneration (before commit)
	contextItemsSQL := `SELECT context_items FROM conversation_messages WHERE conversation_id = $1 AND message_order = $2 AND archived = FALSE`
	var contextItemsJSON []byte
	err = tx.QueryRow(context.Background(), contextItemsSQL, req.ConversationID, messageOrder).Scan(&contextItemsJSON)
	var contextItems []map[string]interface{}
	if err == nil && len(contextItemsJSON) > 0 {
		if unmarshalErr := json.Unmarshal(contextItemsJSON, &contextItems); unmarshalErr != nil {
			log.Printf("Warning: failed to parse context items: %v", unmarshalErr)
			contextItems = []map[string]interface{}{} // Default to empty context
		}
	} else {
		contextItems = []map[string]interface{}{} // Default to empty context
	}

	// Invalidate cache BEFORE committing transaction to prevent stale data
	if err := InvalidateConversationCache(context.Background(), conn, userID, req.ConversationID); err != nil {
		log.Printf("Warning: failed to invalidate conversation cache: %v", err)
	}

	// Commit the transaction
	if err = tx.Commit(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to commit edit transaction: %w", err)
	}

	// Return success response with context items for frontend to use in regeneration
	return map[string]interface{}{
		"success":         true,
		"conversation_id": req.ConversationID,
		"context_items":   contextItems,
	}, nil
}

// GetUserConversation gets the active conversation
func GetUserConversation(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	return GetActiveConversationWithCache(context.Background(), conn, userID)
}

func checkIfConversationIsPublic(conn *data.Conn, conversationID string) (bool, int, string, error) {
	var isPublic bool
	var userID int
	var title string

	err := conn.DB.QueryRow(context.Background(), "SELECT is_public, user_id, title FROM conversations WHERE conversation_id = $1", conversationID).Scan(&isPublic, &userID, &title)
	if err != nil {
		return false, 0, "", err
	}
	return isPublic, userID, title, nil
}
func checkIfConversationIsPublicAndGrabPreview(conn *data.Conn, conversationId string) (bool, int, string, string, error) {
	var isPublic bool
	var userID int
	var firstUserMessage string
	var firstAssistantMessage string

	isPublic, userID, _, err := checkIfConversationIsPublic(conn, conversationId)
	// If not public, don't fetch messages
	if err != nil || !isPublic {
		return false, 0, "", "", err
	}

	// Get the first user message (query)
	err = conn.DB.QueryRow(context.Background(),
		"SELECT query FROM conversation_messages WHERE conversation_id = $1 AND archived = FALSE ORDER BY created_at ASC LIMIT 1",
		conversationId).Scan(&firstUserMessage)
	if err != nil && err.Error() != "no rows in result set" {
		return false, 0, "", "", err
	}

	// Get the first assistant message content_chunks
	var contentChunksJSON []byte
	err = conn.DB.QueryRow(context.Background(),
		"SELECT COALESCE(content_chunks, '[]') FROM conversation_messages WHERE conversation_id = $1 AND content_chunks IS NOT NULL AND archived = FALSE ORDER BY created_at ASC LIMIT 1",
		conversationId).Scan(&contentChunksJSON)
	if err != nil && err.Error() != "no rows in result set" {
		return false, 0, "", "", err
	}

	// Extract text from content_chunks
	if len(contentChunksJSON) > 0 {
		var contentChunks []ContentChunk
		if err := json.Unmarshal(contentChunksJSON, &contentChunks); err == nil {
			// Extract text from the first few text chunks
			var textParts []string
			for i, chunk := range contentChunks {
				if i >= 3 { // Limit to first 3 chunks
					break
				}
				if chunk.Type == "text" {
					if text, ok := chunk.Content.(string); ok {
						// Remove HTML tags and clean up the text
						cleanText := stripHTMLTags(text)
						if len(cleanText) > 0 {
							textParts = append(textParts, cleanText)
						}
					}
				}
			}
			if len(textParts) > 0 {
				firstAssistantMessage = strings.Join(textParts, " ")
				// Limit total length for OG preview
				if len(firstAssistantMessage) > 500 {
					firstAssistantMessage = firstAssistantMessage[:500] + "..."
				}
			}
		}
	}

	return isPublic, userID, firstUserMessage, firstAssistantMessage, nil
}

type GetPublicConversationRequest struct {
	ConversationID string `json:"conversation_id"`
}

func GetPublicConversation(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args GetPublicConversationRequest
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}
	fmt.Println("Getting public conversation for conversationID:", args.ConversationID)
	var isPublic bool
	var userID int
	var title string
	isPublic, userID, title, err := checkIfConversationIsPublic(conn, args.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("issue checking if convo is public: %w", err)
	}
	if !isPublic {
		return nil, fmt.Errorf("no access to conversation")
	}

	// Get the raw messages
	messagesInterface, err := GetConversationMessages(context.Background(), conn, args.ConversationID, userID)
	if err != nil {
		return nil, err
	}

	// Type assert to get the actual messages
	messages, ok := messagesInterface.([]DBConversationMessage)
	if !ok {
		return nil, fmt.Errorf("unexpected type returned from GetConversationMessages")
	}

	// Convert to the format expected by the frontend
	conversationData := convertDBMessagesToConversationData(messages)
	conversationData.ConversationID = args.ConversationID
	conversationData.Title = title

	return conversationData, nil
}

type SetConversationVisibilityRequest struct {
	ConversationID string `json:"conversation_id"`
	IsPublic       bool   `json:"is_public"`
}

func SetConversationVisibility(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var req SetConversationVisibilityRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	if req.ConversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}
	if err := VerifyConversationOwnership(conn, req.ConversationID, userID); err != nil {
		return nil, fmt.Errorf("no permission to edit this conversation")
	}
	_, err := conn.DB.Exec(context.Background(), "UPDATE conversations SET is_public = $1 WHERE conversation_id = $2", req.IsPublic, req.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to update conversation: %w", err)
	}
	return map[string]interface{}{
		"success": true,
	}, nil
}

type GetConversationSnippetArgs struct {
	ConversationID string `json:"conversation_id"`
}

type ConversationSnippetResponse struct {
	Title         string `json:"title"`
	FirstResponse string `json:"first_response"`
}

func GetConversationSnippet(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args GetConversationSnippetArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	if args.ConversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}

	// Use the updated function to get conversation preview data
	isPublic, _, title, firstResponse, err := checkIfConversationIsPublicAndGrabPreview(conn, args.ConversationID)
	if !isPublic {
		return nil, fmt.Errorf("conversation not public")
	}
	if err != nil {
		return nil, fmt.Errorf("error fetching conversation: %w", err)
	}

	return ConversationSnippetResponse{
		Title:         title,
		FirstResponse: firstResponse,
	}, nil
}

// stripHTMLTags removes HTML tags and ticker formatting from text content
func stripHTMLTags(text string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	cleaned := re.ReplaceAllString(text, "")

	// Remove ticker formatting $$$TICKER-TIMESTAMP$$$ and replace with just TICKER
	tickerRe := regexp.MustCompile(`\$\$\$([A-Z]+)-\d+\$\$\$`)
	cleaned = tickerRe.ReplaceAllString(cleaned, "$1")

	// Clean up extra whitespace
	cleaned = strings.TrimSpace(cleaned)
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")

	return cleaned
}
