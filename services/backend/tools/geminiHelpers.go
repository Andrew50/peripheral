package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// getSystemInstruction reads the content of query.txt to be used as system instruction
func getSystemInstruction(systemPrompt string) (string, error) {
	// Get the directory of the current file (gemini.go)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("error getting current file path")
	}
	currentDir := filepath.Dir(filename)

	systemPrompt = systemPrompt + ".txt"
	// Construct path to query.txt
	queryFilePath := filepath.Join(currentDir, systemPrompt)

	// Read the content of query.txt
	content, err := os.ReadFile(queryFilePath)
	if err != nil {
		return "", fmt.Errorf("error reading query.txt: %w", err)
	}

	// Replace the {{CURRENT_TIME}} placeholder with the actual current time
	currentTime := time.Now().Format(time.RFC3339)
	currentTimeMilliseconds := time.Now().UnixMilli()
	instruction := strings.Replace(string(content), "{{CURRENT_TIME}}", currentTime, -1)
	instruction = strings.Replace(instruction, "{{CURRENT_TIME_MILLISECONDS}}", fmt.Sprintf("%d", currentTimeMilliseconds), -1)

	return instruction, nil
}

// enhanceSystemPromptWithTools adds a formatted list of available tools to the system prompt
func enhanceSystemPromptWithTools(basePrompt string) string {
	var toolsDescription strings.Builder

	// Start with the base prompt
	toolsDescription.WriteString(basePrompt)
	toolsDescription.WriteString("\n\nHere are the functions you can use:\n\n")

	// Sort tool names for consistent output
	var toolNames []string
	for name := range GetTools(false) {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)

	// Add each tool's description and parameters
	for _, name := range toolNames {
		tool := GetTools(false)[name]

		// Add function name and description
		toolsDescription.WriteString(fmt.Sprintf("- %s: %s\n", name, tool.FunctionDeclaration.Description))

		// Add parameters if they exist
		if tool.FunctionDeclaration.Parameters != nil && len(tool.FunctionDeclaration.Parameters.Properties) > 0 {
			toolsDescription.WriteString("  Parameters:\n")

			// Get required parameters
			required := make(map[string]bool)
			for _, req := range tool.FunctionDeclaration.Parameters.Required {
				required[req] = true
			}

			// Add each parameter with its description
			for paramName, paramSchema := range tool.FunctionDeclaration.Parameters.Properties {
				isReq := ""
				if required[paramName] {
					isReq = " (required)"
				}
				toolsDescription.WriteString(fmt.Sprintf("  - %s: %s%s\n", paramName, paramSchema.Description, isReq))
			}
		}

		// Add spacing between functions
		toolsDescription.WriteString("\n")
	}

	return toolsDescription.String()
}

// ClearConversationHistory deletes the conversation for a user
func ClearConversationHistory(conn *utils.Conn, userID int, args json.RawMessage) (interface{}, error) {
	ctx := context.Background()
	conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	fmt.Printf("Attempting to delete conversation for key: %s\n", conversationKey)

	// Delete the key from Redis
	err := conn.Cache.Del(ctx, conversationKey).Err()
	if err != nil {
		fmt.Printf("Failed to delete conversation from Redis: %v\n", err)
		return nil, fmt.Errorf("failed to clear conversation history: %w", err)
	}

	fmt.Printf("Successfully deleted conversation for key: %s\n", conversationKey)
	return map[string]string{"message": "Conversation history cleared successfully"}, nil
}
