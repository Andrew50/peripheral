package query

import (
	"backend/utils"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"
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
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("error creating gemini client: %w", err)
	}
	systemInstruction, err := getSystemInstruction()
	if err != nil {
		return "", fmt.Errorf("error getting system instruction: %w", err)
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{&genai.Part{Text: systemInstruction}}},
	}
	resp, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-001", genai.Text(query), config)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}
	return resp.Text()

}
