package tools

import (
	"backend/utils"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed prompts/*
var fs embed.FS // 2️⃣ compiled into the binary

// getSystemInstruction returns the processed prompt named <name>.txt
func getSystemInstruction(name string) (string, error) {
	fmt.Println(fs)
	raw, err := fs.ReadFile("prompts/" + name + ".txt") // 3️⃣ no paths, no os.ReadFile
	if err != nil {
		return "", fmt.Errorf("reading prompt: %w", err)
	}

	now := time.Now()
	s := strings.ReplaceAll(string(raw), "{{CURRENT_TIME}}",
		now.Format(time.RFC3339))
	s = strings.ReplaceAll(s, "{{CURRENT_TIME_MILLISECONDS}}",
		strconv.FormatInt(now.UnixMilli(), 10))
	return s, nil
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
					isReq = " (REQUIRED)"
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
