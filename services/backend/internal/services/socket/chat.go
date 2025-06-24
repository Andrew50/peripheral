package socket

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ChatHandler defines the interface for handling chat requests
type ChatHandler func(ctx context.Context, conn *data.Conn, userID int, args json.RawMessage) (interface{}, error)

// Global chat handler function - will be set during initialization
var chatHandler ChatHandler

// SetChatHandler sets the chat request handler to avoid circular imports
func SetChatHandler(handler ChatHandler) {
	chatHandler = handler
}

// HandleChatQuery handles chat queries received via WebSocket
func (c *Client) HandleChatQuery(requestID, query string, contextItems []map[string]interface{}, activeChartContext map[string]interface{}, conversationID string) {
	// Add nil pointer checks
	if c == nil {
		return
	}

	// Find the userID for this client
	var userID int
	UserToClientMutex.RLock()
	for uid, client := range UserToClient {
		if client == c {
			userID = uid
			break
		}
	}
	UserToClientMutex.RUnlock()

	if userID == 0 {
		// Send error response
		c.SendChatResponse(requestID, false, nil, "User not found")
		return
	}

	if chatHandler == nil {
		// Send error response
		c.SendChatResponse(requestID, false, nil, "Chat handler not initialized")
		return
	}

	// Add nil pointer check for connection
	if c.conn == nil {
		c.SendChatResponse(requestID, false, nil, "Database connection not available")
		return
	}

	// Create the chat request
	chatRequest := map[string]interface{}{
		"query":              query,
		"context":            contextItems,
		"activeChartContext": activeChartContext,
		"conversation_id":    conversationID,
	}

	// Marshal the request
	requestBytes, err := json.Marshal(chatRequest)
	if err != nil {
		c.SendChatResponse(requestID, false, nil, "Failed to marshal request")
		return
	}

	// Call the chat request function in a goroutine to avoid blocking
	go func() {
		// Create a context for the request with timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Add recovery to prevent panics from crashing the WebSocket connection
		defer func() {
			if r := recover(); r != nil {
				c.SendChatResponse(requestID, false, nil, fmt.Sprintf("Internal error: %v", r))
			}
		}()

		// Call the chat handler function
		result, err := chatHandler(ctx, c.conn, userID, requestBytes)
		if err != nil {
			c.SendChatResponse(requestID, false, nil, err.Error())
			return
		}

		c.SendChatResponse(requestID, true, result, "")
	}()
}

// SendChatResponse sends a chat response back to the client via WebSocket
func (c *Client) SendChatResponse(requestID string, success bool, data interface{}, errorMsg string) {
	response := map[string]interface{}{
		"type":       "chat_response",
		"request_id": requestID,
		"success":    success,
	}

	if success {
		response["data"] = data
	} else {
		response["error"] = errorMsg
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return // Can't marshal response, nothing we can do
	}

	// Send non-blocking
	select {
	case c.send <- jsonData:
		// Success
	default:
		// Channel full or closed, drop message
	}
}
