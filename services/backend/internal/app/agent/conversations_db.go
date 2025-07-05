package agent

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// ConversationSummary represents a conversation in the list view
type ConversationSummary struct {
	ConversationID   string    `json:"conversation_id"`
	Title            string    `json:"title"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	MessageCount     int       `json:"message_count"`
	LastMessageQuery string    `json:"last_message_query,omitempty"`
}

// VerifyConversationOwnership verifies that a user owns a conversation
func VerifyConversationOwnership(conn *data.Conn, conversationID string, userID int) error {
	var ownerID int
	err := conn.DB.QueryRow(context.Background(), "SELECT user_id FROM conversations WHERE conversation_id = $1", conversationID).Scan(&ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("conversation not found")
		}
		return fmt.Errorf("failed to verify conversation ownership: %w", err)
	}
	if ownerID != userID {
		return fmt.Errorf("unauthorized access to conversation")
	}
	return nil
}

// DBConversationMessage represents a message stored in the database
type DBConversationMessage struct {
	MessageID        string                   `json:"message_id"`
	ConversationID   string                   `json:"conversation_id"`
	Query            string                   `json:"query"`
	ResponseText     string                   `json:"response_text"`
	ContentChunks    []ContentChunk           `json:"content_chunks"`
	FunctionCalls    []FunctionCall           `json:"function_calls"`
	ToolResults      []ExecuteResult          `json:"tool_results"`
	ContextItems     []map[string]interface{} `json:"context_items"`
	SuggestedQueries []string                 `json:"suggested_queries"`
	Citations        []Citation               `json:"citations"`
	CreatedAt        time.Time                `json:"created_at"`
	CompletedAt      *time.Time               `json:"completed_at"`
	Status           string                   `json:"status"`
	TokenCount       int                      `json:"token_count"`
	MessageOrder     int                      `json:"message_order"`
}

// MessageCompletionData represents the data returned when completing a message
type MessageCompletionData struct {
	MessageID   string     `json:"message_id"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// CreateConversation creates a new conversation in the database
func CreateConversationInDB(ctx context.Context, conn *data.Conn, userID int, title string) (string, error) {
	conversationID := uuid.New().String()

	query := `
		INSERT INTO conversations (conversation_id, user_id, title, created_at, updated_at, metadata, total_token_count, message_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING conversation_id`

	now := time.Now()
	var returnedID string

	err := conn.DB.QueryRow(ctx, query,
		conversationID, userID, title, now, now, "{}", 0, 0,
	).Scan(&returnedID)

	if err != nil {
		return "", fmt.Errorf("failed to create conversation: %w", err)
	}

	return returnedID, nil
}

// GetConversationMessages retrieves all messages for a conversation
func GetConversationMessages(ctx context.Context, conn *data.Conn, conversationID string, userID int) (interface{}, error) {
	// First verify the user owns this conversation
	if err := VerifyConversationOwnership(conn, conversationID, userID); err != nil {
		return nil, err
	}

	query := `
		SELECT 
			message_id,
			conversation_id,
			query,
			response_text,
			content_chunks,
			function_calls,
			tool_results,
			context_items,
			suggested_queries,
			citations,
			created_at,
			completed_at,
			status,
			token_count,
			message_order
		FROM conversation_messages
		WHERE conversation_id = $1 AND archived = FALSE
		ORDER BY message_order ASC`

	rows, err := conn.DB.Query(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversation messages: %w", err)
	}
	defer rows.Close()

	var messages []DBConversationMessage
	for rows.Next() {
		var msg DBConversationMessage
		var contentChunksJSON, functionCallsJSON, toolResultsJSON []byte
		var contextItemsJSON, suggestedQueriesJSON, citationsJSON []byte
		var completedAt sql.NullTime

		err := rows.Scan(
			&msg.MessageID,
			&msg.ConversationID,
			&msg.Query,
			&msg.ResponseText,
			&contentChunksJSON,
			&functionCallsJSON,
			&toolResultsJSON,
			&contextItemsJSON,
			&suggestedQueriesJSON,
			&citationsJSON,
			&msg.CreatedAt,
			&completedAt,
			&msg.Status,
			&msg.TokenCount,
			&msg.MessageOrder,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}

		// Parse JSON fields
		if len(contentChunksJSON) > 0 {
			if err := json.Unmarshal(contentChunksJSON, &msg.ContentChunks); err != nil {
				return nil, fmt.Errorf("failed to parse content_chunks: %w", err)
			}
		}
		if len(functionCallsJSON) > 0 {
			if err := json.Unmarshal(functionCallsJSON, &msg.FunctionCalls); err != nil {
				return nil, fmt.Errorf("failed to parse function_calls: %w", err)
			}
		}
		if len(toolResultsJSON) > 0 {
			if err := json.Unmarshal(toolResultsJSON, &msg.ToolResults); err != nil {
				return nil, fmt.Errorf("failed to parse tool_results: %w", err)
			}
		}
		if len(contextItemsJSON) > 0 {
			if err := json.Unmarshal(contextItemsJSON, &msg.ContextItems); err != nil {
				return nil, fmt.Errorf("failed to parse context_items: %w", err)
			}
		}
		if len(suggestedQueriesJSON) > 0 {
			if err := json.Unmarshal(suggestedQueriesJSON, &msg.SuggestedQueries); err != nil {
				return nil, fmt.Errorf("failed to parse suggested_queries: %w", err)
			}
		}
		if len(citationsJSON) > 0 {
			if err := json.Unmarshal(citationsJSON, &msg.Citations); err != nil {
				return nil, fmt.Errorf("failed to parse citations: %w", err)
			}
		}

		if completedAt.Valid {
			msg.CompletedAt = &completedAt.Time
		}

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	return messages, nil
}

// SaveConversationMessage saves a message to the database
func SaveConversationMessage(ctx context.Context, conn *data.Conn, conversationID string, userID int, message ChatMessage) (string, error) {
	// Start transaction for atomic operation
	tx, err := conn.DB.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// First verify the user owns this conversation
	if err = VerifyConversationOwnership(conn, conversationID, userID); err != nil {
		return "", err
	}

	// Marshal JSON fields
	contentChunksJSON, err := json.Marshal(message.ContentChunks)
	if err != nil {
		return "", fmt.Errorf("failed to marshal content_chunks: %w", err)
	}
	functionCallsJSON, err := json.Marshal(message.FunctionCalls)
	if err != nil {
		return "", fmt.Errorf("failed to marshal function_calls: %w", err)
	}
	toolResultsJSON, err := json.Marshal(message.ToolResults)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool_results: %w", err)
	}
	contextItemsJSON, err := json.Marshal(message.ContextItems)
	if err != nil {
		return "", fmt.Errorf("failed to marshal context_items: %w", err)
	}
	suggestedQueriesJSON, err := json.Marshal(message.SuggestedQueries)
	if err != nil {
		return "", fmt.Errorf("failed to marshal suggested_queries: %w", err)
	}
	citationsJSON, err := json.Marshal(message.Citations)
	if err != nil {
		return "", fmt.Errorf("failed to marshal citations: %w", err)
	}

	messageID := uuid.New().String()

	// Use atomic operation to get next order and insert message
	// This prevents race conditions when multiple messages are inserted concurrently
	query := `
		INSERT INTO conversation_messages (
			message_id, conversation_id, query, response_text, content_chunks, 
			function_calls, tool_results, context_items, suggested_queries, citations,
			created_at, completed_at, status, token_count, message_order
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			(SELECT COALESCE(MAX(message_order), 0) + 1 FROM conversation_messages WHERE conversation_id = $2)
		)`

	_, err = tx.Exec(ctx, query,
		messageID,
		conversationID,
		message.Query,
		message.ResponseText,
		contentChunksJSON,
		functionCallsJSON,
		toolResultsJSON,
		contextItemsJSON,
		suggestedQueriesJSON,
		citationsJSON,
		message.Timestamp,
		message.CompletedAt,
		message.Status,
		message.TokenCount,
	)

	if err != nil {
		return "", fmt.Errorf("failed to save conversation message: %w", err)
	}

	// Update conversation metadata
	updateQuery := `
		UPDATE conversations 
		SET message_count = message_count + 1,
			total_token_count = total_token_count + $1,
			updated_at = $2
		WHERE conversation_id = $3`

	_, err = tx.Exec(ctx, updateQuery, message.TokenCount, message.Timestamp, conversationID)
	if err != nil {
		return "", fmt.Errorf("failed to update conversation metadata: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return messageID, nil
}

// DeleteConversationInDB deletes a conversation and all its messages
func DeleteConversationInDB(conn *data.Conn, conversationID string, userID int) error {
	// Verify ownership before deletion
	if err := VerifyConversationOwnership(conn, conversationID, userID); err != nil {
		return err
	}

	// Delete the conversation (messages will be deleted via CASCADE)
	query := `DELETE FROM conversations WHERE conversation_id = $1 AND user_id = $2`

	result, err := conn.DB.Exec(context.Background(), query, conversationID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("conversation not found or unauthorized")
	}

	return nil
}

// GetMessageContextItems retrieves context items from a specific message
func GetMessageContextItems(ctx context.Context, conn *data.Conn, conversationID string, messageOrder int) ([]map[string]interface{}, error) {
	querySQL := `SELECT context_items FROM conversation_messages WHERE conversation_id = $1 AND message_order = $2 AND archived = FALSE`

	var contextItemsJSON []byte
	err := conn.DB.QueryRow(ctx, querySQL, conversationID, messageOrder).Scan(&contextItemsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return []map[string]interface{}{}, nil // No context items
		}
		return nil, fmt.Errorf("failed to get context items: %w", err)
	}

	var contextItems []map[string]interface{}
	if len(contextItemsJSON) > 0 {
		if err := json.Unmarshal(contextItemsJSON, &contextItems); err != nil {
			return nil, fmt.Errorf("failed to parse context items: %w", err)
		}
	}

	return contextItems, nil
}

// SavePendingMessageToConversation saves a pending message to a specific conversation ID
func SavePendingMessageToConversation(ctx context.Context, conn *data.Conn, userID int, conversationID string, query string, contextItems []map[string]interface{}) (string, string, error) {
	// Create pending message
	now := time.Now()
	message := ChatMessage{
		Query:        query,
		ContextItems: contextItems,
		Timestamp:    now,
		Status:       "pending",
	}
	// If conversationID is empty, create a new conversation
	if conversationID == "" {
		newConversationID, err := CreateConversationInDB(ctx, conn, userID, "New Chat")
		if err != nil {
			return "", "", fmt.Errorf("failed to create new conversation: %w", err)
		}
		conversationID = newConversationID

		err = SetActiveConversationIDCached(ctx, conn, userID, conversationID)
		if err != nil {
			return "", "", fmt.Errorf("failed to set active conversation ID: %w", err)
		}
		// Generate better title asynchronously and send via websocket
		go generateTitleAsync(conn, userID, query, conversationID)
	}

	// Save to database
	messageID, err := SaveConversationMessage(ctx, conn, conversationID, userID, message)
	if err != nil {
		return "", "", fmt.Errorf("failed to save message to database: %w", err)
	}

	return conversationID, messageID, nil
}

// generateTitleAsync generates a title in the background and sends it via websocket
func generateTitleAsync(conn *data.Conn, userID int, query string, conversationID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if betterTitle, err := GenerateConversationTitle(conn, userID, query); err == nil {
		// Send title update via websocket
		socket.SendTitleUpdate(userID, conversationID, betterTitle)
		// Update the title in the database
		_, updateErr := conn.DB.Exec(ctx,
			"UPDATE conversations SET title = $1 WHERE conversation_id = $2",
			betterTitle, conversationID)
		if updateErr != nil {
			log.Printf("Failed to update conversation title: %v", updateErr)
			return
		}
	} else {
		log.Printf("Failed to generate better title: %v", err)
	}
}

// UpdatePendingMessageToCompleted updates a pending message to completed status
func UpdatePendingMessageToCompleted(ctx context.Context, conn *data.Conn, userID int, query string, contentChunks []ContentChunk, functionCalls []FunctionCall, toolResults []ExecuteResult, suggestedQueries []string, tokenCount int) error {
	activeConversationID, err := GetActiveConversationIDCached(ctx, conn, userID)
	if err != nil || activeConversationID == "" {
		return fmt.Errorf("no active conversation found")
	}

	// Verify user owns the conversation
	if err = VerifyConversationOwnership(conn, activeConversationID, userID); err != nil {
		return err
	}

	// Update the database first
	querySQL := `
		UPDATE conversation_messages 
		SET content_chunks = $1, function_calls = $2, tool_results = $3, 
			suggested_queries = $4, token_count = $5, completed_at = $6, status = $7
		WHERE conversation_id = $8 AND query = $9 AND status = 'pending'`

	// Marshal JSON fields
	contentChunksJSON, _ := json.Marshal(contentChunks)
	functionCallsJSON, _ := json.Marshal(functionCalls)
	toolResultsJSON, _ := json.Marshal(toolResults)
	suggestedQueriesJSON, _ := json.Marshal(suggestedQueries)

	now := time.Now()
	result, err := conn.DB.Exec(ctx, querySQL,
		contentChunksJSON,
		functionCallsJSON,
		toolResultsJSON,
		suggestedQueriesJSON,
		tokenCount,
		now,
		"completed",
		activeConversationID,
		query,
	)

	if err != nil {
		return fmt.Errorf("failed to update pending message: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no pending message found with query: %s", query)
	}

	// Update the cache
	updateFunc := func(msg *ChatMessage) {
		msg.ContentChunks = contentChunks
		msg.FunctionCalls = functionCalls
		msg.ToolResults = toolResults
		msg.SuggestedQueries = suggestedQueries
		msg.TokenCount = tokenCount
		msg.CompletedAt = now
		msg.Status = "completed"
	}

	if err := UpdateMessageInActiveConversationCache(ctx, conn, userID, query, updateFunc); err != nil {
		fmt.Printf("Warning: failed to update message in cache: %v\n", err)
	}

	return nil
}

// UpdatePendingMessageToCompletedInConversation updates a pending message to completed status in a specific conversation
func UpdatePendingMessageToCompletedInConversation(ctx context.Context, conn *data.Conn, userID int, conversationID string, query string, contentChunks []ContentChunk, functionCalls []FunctionCall,
	toolResults []ExecuteResult, suggestedQueries []string, tokenCount TokenCounts) (*MessageCompletionData, error) {
	// Verify user owns the conversation
	if err := VerifyConversationOwnership(conn, conversationID, userID); err != nil {
		return nil, err
	}
	// Marshal JSON fields
	contentChunksJSON, _ := json.Marshal(contentChunks)
	functionCallsJSON, _ := json.Marshal(functionCalls)
	toolResultsJSON, _ := json.Marshal(toolResults)
	suggestedQueriesJSON, _ := json.Marshal(suggestedQueries)

	// Update the database and get the timestamps in a single operation
	now := time.Now()
	querySQL := `
		UPDATE conversation_messages 
		SET content_chunks = $1, function_calls = $2, tool_results = $3, 
			suggested_queries = $4, token_count = $5, completed_at = $6, status = $7
		WHERE conversation_id = $8 AND query = $9 AND status = 'pending'
		RETURNING message_id, created_at, completed_at`

	var messageData MessageCompletionData
	err := conn.DB.QueryRow(ctx, querySQL,
		contentChunksJSON,
		functionCallsJSON,
		toolResultsJSON,
		suggestedQueriesJSON,
		tokenCount.TotalTokenCount,
		now,
		"completed",
		conversationID,
		query,
	).Scan(&messageData.MessageID, &messageData.CreatedAt, &messageData.CompletedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no pending message found with query: %s in conversation: %s", query, conversationID)
		}
		return nil, fmt.Errorf("failed to update pending message: %w", err)
	}

	// Invalidate cache for this conversation since the message was updated
	if err := InvalidateConversationCache(ctx, conn, userID, conversationID); err != nil {
		fmt.Printf("Warning: failed to invalidate conversation cache after completing message: %v\n", err)
	}

	return &messageData, nil
}

// DeletePendingMessageInConversation deletes a pending message when a request is cancelled or fails
func DeletePendingMessageInConversation(ctx context.Context, conn *data.Conn, userID int, conversationID string, query string) error {
	// Delete the pending message from database
	querySQL := `
		DELETE FROM conversation_messages 
		WHERE conversation_id = $1 AND query = $2 AND status = 'pending'`

	result, err := conn.DB.Exec(ctx, querySQL, conversationID, query)
	if err != nil {
		return fmt.Errorf("failed to delete pending message: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// Message might not exist or already completed, which is fine
		return nil
	}

	// Update conversation metadata (decrease message count if needed)
	updateQuery := `
		UPDATE conversations 
		SET message_count = (
			SELECT COUNT(*) FROM conversation_messages 
			WHERE conversation_id = $1 AND archived = FALSE
		),
		updated_at = $2
		WHERE conversation_id = $1 AND user_id = $3`

	_, err = conn.DB.Exec(ctx, updateQuery, conversationID, time.Now(), userID)
	if err != nil {
		// Log error but don't fail - the main goal (deleting pending message) succeeded
		fmt.Printf("Warning: failed to update conversation metadata after deleting pending message: %v\n", err)
	}

	return nil
}

// MarkPendingMessageAsError marks a pending message as error status instead of deleting it
func MarkPendingMessageAsError(ctx context.Context, conn *data.Conn, userID int, conversationID string, messageID string, errorMessage string) error {
	// Update the database to mark as error
	querySQL := `
		UPDATE conversation_messages 
		SET response_text = $1, completed_at = $2, status = $3
		WHERE conversation_id = $4 AND message_id = $5 AND status = 'pending'`

	now := time.Now()
	result, err := conn.DB.Exec(ctx, querySQL,
		fmt.Sprintf("Error: %s", errorMessage),
		now,
		"error",
		conversationID,
		messageID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark pending message as error: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no pending message found with message_id: %s in conversation: %s", messageID, conversationID)
	}

	// Invalidate cache for this conversation since the message was updated
	if err := InvalidateConversationCache(ctx, conn, userID, conversationID); err != nil {
		fmt.Printf("Warning: failed to invalidate conversation cache after marking message as error: %v\n", err)
	}

	return nil
}

// CancelPendingMessage is the endpoint handler for cancelling pending messages
func CancelPendingMessage(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var request struct {
		ConversationID string `json:"conversation_id"`
		Query          string `json:"query"`
	}

	if err := json.Unmarshal(args, &request); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	// Delete the pending message
	if err := DeletePendingMessageInConversation(context.Background(), conn, userID, request.ConversationID, request.Query); err != nil {
		return nil, fmt.Errorf("error cancelling pending message: %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"message": "Pending message cancelled successfully",
	}, nil
}

// FindMessageToEdit finds a message to edit by message ID and returns its order and original query
func FindMessageToEdit(ctx context.Context, conn *data.Conn, conversationID string, messageID string) (int, string, error) {
	if messageID == "" {
		return 0, "", fmt.Errorf("message_id is required")
	}

	var foundOrder int
	var foundQuery string

	querySQL := `SELECT message_order, query FROM conversation_messages WHERE conversation_id = $1 AND message_id = $2 AND archived = FALSE`
	err := conn.DB.QueryRow(ctx, querySQL, conversationID, messageID).Scan(&foundOrder, &foundQuery)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", fmt.Errorf("message not found")
		}
		return 0, "", fmt.Errorf("failed to find message: %w", err)
	}

	return foundOrder, foundQuery, nil
}

// PruneMessagesAfterOrder archives all messages after the specified order
func PruneMessagesAfterOrder(ctx context.Context, conn *data.Conn, conversationID string, messageOrder int) error {
	querySQL := `UPDATE conversation_messages SET archived = TRUE WHERE conversation_id = $1 AND message_order >= $2 AND archived = FALSE`

	_, err := conn.DB.Exec(ctx, querySQL, conversationID, messageOrder)
	if err != nil {
		return fmt.Errorf("failed to archive messages: %w", err)
	}

	return nil
}

// UpdateMessageContentAndStatus updates a message's content and status
func UpdateMessageContentAndStatus(ctx context.Context, conn *data.Conn, conversationID string, messageOrder int, newContent string, status string) error {
	querySQL := `
		UPDATE conversation_messages 
		SET query = $1, status = $2, completed_at = NULL, response_text = '', 
		    content_chunks = '[]', function_calls = '[]', tool_results = '[]', 
		    suggested_queries = '[]', citations = '[]', token_count = 0
		WHERE conversation_id = $3 AND message_order = $4`

	result, err := conn.DB.Exec(ctx, querySQL, newContent, status, conversationID, messageOrder)
	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no message found with order %d in conversation %s", messageOrder, conversationID)
	}

	return nil
}

// UpdateConversationAfterEdit updates conversation metadata after editing
func UpdateConversationAfterEdit(ctx context.Context, conn *data.Conn, conversationID string) error {
	querySQL := `
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

	_, err := conn.DB.Exec(ctx, querySQL, conversationID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update conversation metadata: %w", err)
	}

	return nil
}

// ValidateMessageForEdit ensures the message can be edited
func ValidateMessageForEdit(ctx context.Context, conn *data.Conn, conversationID string, messageOrder int) error {
	// Check if message exists and get its details
	var query, status string
	var completedAt sql.NullTime

	querySQL := `
		SELECT query, status, completed_at 
		FROM conversation_messages 
		WHERE conversation_id = $1 AND message_order = $2 AND archived = FALSE`

	err := conn.DB.QueryRow(ctx, querySQL, conversationID, messageOrder).Scan(&query, &status, &completedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("message not found")
		}
		return fmt.Errorf("failed to validate message: %w", err)
	}

	// Only allow editing if message is not currently pending
	if status == "pending" {
		return fmt.Errorf("cannot edit a message that is currently being processed")
	}

	// Additional validation can be added here
	// For example, checking if it's a user message vs assistant message
	// or if it's too old to edit, etc.

	return nil
}
