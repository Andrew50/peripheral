package query

import (
	"backend/tasks"
	"backend/utils"
	"encoding/json"
	"fmt"
)

// GetQuery processes a natural language query and returns the result
// This function has the same signature as tasks.GetQuery
func GetQuery(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var query tasks.Query
	if err := json.Unmarshal(args, &query); err != nil {
		return nil, fmt.Errorf("error parsing request: %w", err)
	}

	// If the query is empty, return an error
	if query.Query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Get function calls from the LLM
	functionCalls, err := getGeminiFunctionResponse(conn, query.Query)
	if err != nil {
		return nil, fmt.Errorf("error getting function calls: %w", err)
	}

	// If no function calls were returned, fall back to the text response
	if len(functionCalls) == 0 {
		textResponse, err := getGeminiResponse(conn, query.Query)
		if err != nil {
			return nil, fmt.Errorf("error getting text response: %w", err)
		}
		return map[string]interface{}{
			"type": "text",
			"text": textResponse,
		}, nil
	}

	// We'll need to use a different approach to execute functions without importing server
	// For now, return a temporary implementation
	return map[string]interface{}{
		"type": "text",
		"text": "Function calling will be implemented after resolving import cycles.",
	}, nil
}

// GetLLMParsedQuery is kept for backward compatibility
func GetLLMParsedQuery(conn *utils.Conn, query tasks.Query) (string, error) {
	llmResponse, err := getGeminiResponse(conn, query.Query)
	if err != nil {
		return "", fmt.Errorf("error getting gemini response: %w", err)
	}

	return llmResponse, nil
}
