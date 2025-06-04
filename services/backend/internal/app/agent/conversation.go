package agent

import (
	"time"
)

// ConversationData represents the data structure for storing conversation context
type ConversationData struct {
	ConversationID string        `json:"conversation_id,omitempty"` // Active conversation ID
	Title          string        `json:"title,omitempty"`           // Conversation title
	Messages       []ChatMessage `json:"messages"`
	TokenCount     int           `json:"token_count"`
	Timestamp      time.Time     `json:"timestamp"`
}

// ChatMessage represents a single message in the conversation
type ChatMessage struct {
	MessageID        string                   `json:"message_id,omitempty"` // Database message ID for editing
	Query            string                   `json:"query"`
	ContentChunks    []ContentChunk           `json:"content_chunks,omitempty"`
	ResponseText     string                   `json:"response_text,omitempty"`
	FunctionCalls    []FunctionCall           `json:"function_calls"`
	ToolResults      []ExecuteResult          `json:"tool_results"`
	ContextItems     []map[string]interface{} `json:"context_items,omitempty"`     // Store context sent with user message
	SuggestedQueries []string                 `json:"suggested_queries,omitempty"` // Store suggested queries from LLM
	Timestamp        time.Time                `json:"timestamp"`
	Citations        []Citation               `json:"citations,omitempty"`
	TokenCount       int                      `json:"token_count"`
	CompletedAt      time.Time                `json:"completed_at,omitempty"` // When the response was completed
	Status           string                   `json:"status,omitempty"`       // "pending", "completed", "error"
}
