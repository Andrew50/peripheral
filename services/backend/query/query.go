package query

import (
	"backend/server"
	"backend/utils"
	"encoding/json"
	"fmt"
)

type Query struct {
	Query string `json:"query"`
}

// ExecuteResult represents the result of executing a function
type ExecuteResult struct {
	FunctionName string      `json:"function_name"`
	Result       interface{} `json:"result"`
	Error        string      `json:"error,omitempty"`
}

// GetQuery processes a natural language query and returns the result
func GetQuery(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	var query Query
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

	// Execute the functions in order and collect results
	var results []ExecuteResult
	for _, fc := range functionCalls {
		// Check if the function exists in Tools map
		tool, exists := server.Tools[fc.Name]
		if !exists {
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Error:        fmt.Sprintf("function '%s' not found", fc.Name),
			})
			continue
		}

		// Execute the function
		result, err := tool.Function(conn, userID, fc.Args)
		if err != nil {
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Error:        err.Error(),
			})
		} else {
			results = append(results, ExecuteResult{
				FunctionName: fc.Name,
				Result:       result,
			})
		}
	}

	return map[string]interface{}{
		"type":    "function_calls",
		"results": results,
	}, nil
}

// GetLLMParsedQuery is kept for backward compatibility
func GetLLMParsedQuery(conn *utils.Conn, query Query) (string, error) {
	llmResponse, err := getGeminiResponse(conn, query.Query)
	if err != nil {
		return "", fmt.Errorf("error getting gemini response: %w", err)
	}

	return llmResponse, nil
}
