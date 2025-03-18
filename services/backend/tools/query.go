package tools

import (
	"backend/utils"
	"encoding/json"
	"fmt"
)

type Query struct {
	Query string `json:"query"`
}

// ParsedQuery represents the structured JSON output from the LLM
type ParsedQuery struct {
	Timeframes []string    `json:"timeframes"`
	Stocks     Stocks      `json:"stocks"`
	Indicators []Indicator `json:"indicators"`
	Conditions []Condition `json:"conditions"`
	Sequence   Sequence    `json:"sequence"`
	DateRange  DateRange   `json:"date_range"`
}

type Stocks struct {
	Universe string            `json:"universe"`
	Include  []string          `json:"include"`
	Exclude  []string          `json:"exclude"`
	Filters  map[string]Filter `json:"filters"`
}

type Filter struct {
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
}

type Indicator struct {
	Type      string `json:"type"`
	Period    int    `json:"period"`
	Timeframe string `json:"timeframe"`
}

type Condition struct {
	LHS       FieldWithOffset `json:"lhs"`
	Operation string          `json:"operation"`
	RHS       RHS             `json:"rhs"`
	Timeframe string          `json:"timeframe"`
}

type FieldWithOffset struct {
	Field  string `json:"field"`
	Offset int    `json:"offset"`
}

type RHS struct {
	Field     string  `json:"field,omitempty"`
	Offset    int     `json:"offset,omitempty"`
	Indicator string  `json:"indicator,omitempty"`
	Period    int     `json:"period,omitempty"`
	Value     float64 `json:"value,omitempty"`
}

type Sequence struct {
	Condition string `json:"condition"`
	Timeframe string `json:"timeframe"`
	Window    int    `json:"window"`
}

type DateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
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
	geminiFuncResponse, err := getGeminiFunctionResponse(conn, query.Query)
	if err != nil {
		return nil, fmt.Errorf("error getting function calls: %w", err)
	}
	
	functionCalls := geminiFuncResponse.FunctionCalls
	responseText := geminiFuncResponse.Text

	// If no function calls were returned, fall back to the text response
	if len(functionCalls) == 0 {
		// If we already have a text response, use it
		if responseText != "" {
			return map[string]interface{}{
				"type": "text",
				"text": responseText,
			}, nil
		}
		
		// Otherwise, get a direct text response
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
		tool, exists := Tools[fc.Name]
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

	// Return both the function call results and the text response
	return map[string]interface{}{
		"type":    "function_calls",
		"results": results,
		"text":    responseText,
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
