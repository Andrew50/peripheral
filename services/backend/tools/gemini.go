package tools

import (
	"backend/server"
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var ctx = context.Background()

// getSystemInstruction reads the content of query.txt to be used as system instruction
func getSystemInstruction() (string, error) {
	// Get the current file's directory
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("error getting current directory: %w", err)
	}

	// Construct path to query.txt
	queryFilePath := filepath.Join(currentDir, "query", "query.txt")

	// Read the content of query.txt
	content, err := os.ReadFile(queryFilePath)
	if err != nil {
		return "", fmt.Errorf("error reading query.txt: %w", err)
	}

	// Replace the {{CURRENT_TIME}} placeholder with the actual current time
	currentTime := time.Now().Format(time.RFC3339)
	instruction := strings.Replace(string(content), "{{CURRENT_TIME}}", currentTime, -1)

	return instruction, nil
}

func getGeminiResponse(conn *utils.Conn, query string) (string, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return "", fmt.Errorf("error getting gemini key: %w", err)
	}

	// Create a new client using the API key
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("error creating gemini client: %w", err)
	}
	defer client.Close()

	// Get the system instruction
	systemInstruction, err := getSystemInstruction()
	if err != nil {
		return "", fmt.Errorf("error getting system instruction: %w", err)
	}

	// Create a model instance
	model := client.GenerativeModel("gemini-2.0-flash-001")

	// Set the system instruction
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(systemInstruction),
		},
	}

	// Generate content
	resp, err := model.GenerateContent(ctx, genai.Text(query))
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}

	// Extract the response text
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	// Get the text from the response
	text := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	return text, nil
}

// FunctionCall represents a function to be called with its arguments
type FunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// FunctionResponse represents the response from the LLM with function calls
type FunctionResponse struct {
	FunctionCalls []FunctionCall `json:"function_calls"`
}

// getGeminiFunctionResponse uses the Google Function API to return an ordered list of functions to execute
func getGeminiFunctionResponse(conn *utils.Conn, query string) ([]FunctionCall, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return nil, fmt.Errorf("error getting gemini key: %w", err)
	}

	// Create a new client using the API key
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("error creating gemini client: %w", err)
	}
	defer client.Close()

	// Get the system instruction
	systemInstruction, err := getSystemInstruction()
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %w", err)
	}

	// Create a model instance
	model := client.GenerativeModel("gemini-2.0-flash-001")

	// Set the system instruction
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(systemInstruction),
		},
	}

	// Get tools from server through the GetTools function
	tools := server.GetTools()

	// Create Gemini tools from function declarations
	var geminiTools []*genai.Tool
	for _, tool := range tools {
		// Convert the FunctionDeclaration to a Tool
		geminiTools = append(geminiTools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        tool.FunctionDeclaration.Name,
					Description: tool.FunctionDeclaration.Description,
					Parameters:  tool.FunctionDeclaration.Parameters,
				},
			},
		})
	}

	// Set the tools for the model
	model.Tools = geminiTools

	// Generate content with function calling
	resp, err := model.GenerateContent(ctx, genai.Text(query))
	if err != nil {
		return nil, fmt.Errorf("error generating content with function calling: %w", err)
	}

	// Extract function calls from response
	var functionCalls []FunctionCall

	// Process the response to extract function calls
	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			// Check if the part is a FunctionCall
			if fc, ok := part.(genai.FunctionCall); ok {
				// Convert arguments to JSON
				args, err := json.Marshal(fc.Args)
				if err != nil {
					return nil, fmt.Errorf("error marshaling function args: %w", err)
				}

				functionCalls = append(functionCalls, FunctionCall{
					Name: fc.Name,
					Args: args,
				})
			}
		}
	}

	return functionCalls, nil
}

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
