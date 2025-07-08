package socket

import (
	"backend/internal/data"
	"context"
	"encoding/json"
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
	// Retrieve the userID directly from the client instance
	userID := c.userID

	if userID == 0 {
		// No user associated with this connection (unexpected). Ask the client to reconnect.
		c.SendChatResponse(requestID, false, nil, "Session expired, please reconnect")
		return
	}

	if chatHandler == nil {
		// Send error response
		c.SendChatResponse(requestID, false, nil, "Chat handler not initialized")
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
		// Create a context for the request
		ctx := context.Background()

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
