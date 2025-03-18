package query

import (
	"backend/tasks"
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

// FunctionResponse represents the response from the LLM with function calls
type FunctionResponse struct {
	FunctionCalls []tasks.FunctionCall `json:"function_calls"`
}

// getGeminiFunctionResponse uses the Google Function API to return an ordered list of functions to execute
func getGeminiFunctionResponse(conn *utils.Conn, query string) ([]tasks.FunctionCall, error) {
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

	// We need to get tool definitions from somewhere else to avoid circular dependency
	// For now, we'll use a temporary empty list
	var geminiTools []*genai.Tool

	// TODO: Create a way to register tools without circular dependencies
	// This is a temporary implementation until we find a better solution

	// Set the tools for the model
	model.Tools = geminiTools

	// Generate content with function calling
	resp, err := model.GenerateContent(ctx, genai.Text(query))
	if err != nil {
		return nil, fmt.Errorf("error generating content with function calling: %w", err)
	}

	// Extract function calls from response
	var functionCalls []tasks.FunctionCall

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

				functionCalls = append(functionCalls, tasks.FunctionCall{
					Name: fc.Name,
					Args: args,
				})
			}
		}
	}

	return functionCalls, nil
}
