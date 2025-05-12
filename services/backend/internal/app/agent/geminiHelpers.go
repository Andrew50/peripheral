package agent

import (
	"backend/internal/data"
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
	raw, err := fs.ReadFile("prompts/" + name + ".txt") // 3️⃣ no paths, no os.ReadFile
	if err != nil {
		return "", fmt.Errorf("reading prompt: %w", err)
	}
	const rfc3339Seconds = "2006-01-02T15:04:05Z07:00"
	now := time.Now()
	s := strings.ReplaceAll(string(raw), "{{CURRENT_TIME}}",
		now.Format(rfc3339Seconds))
	s = strings.ReplaceAll(s, "{{CURRENT_TIME_MILLISECONDS}}",
		strconv.FormatInt(now.UnixMilli(), 10))
	s = strings.ReplaceAll(s, "{{CURRENT_YEAR}}",
		strconv.Itoa(now.Year()))
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
	for name := range Tools {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)

	// Add each tool's description and parameters
	for _, name := range toolNames {
		tool := Tools[name]

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
func ClearConversationHistory(conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
	ctx := context.Background()
	conversationKey := fmt.Sprintf("user:%d:conversation", userID)
	////fmt.Printf("Attempting to delete conversation for key: %s\n", conversationKey)

	// Delete the conversation history key from Redis
	err := conn.Cache.Del(ctx, conversationKey).Err()
	if err != nil {
        return nil, err
		////fmt.Printf("Failed to delete conversation from Redis: %v\n", err)
		// Don't return immediately, still try to delete persistent context
	} 
    //else {
		////fmt.Printf("Successfully deleted conversation for key: %s\n", conversationKey)
	//}

	// Also delete the persistent context key
	persistentContextKey := fmt.Sprintf(persistentContextKeyFormat, userID) // Use constant from persistentContext.go
	pErr := conn.Cache.Del(ctx, persistentContextKey).Err()
	if pErr != nil {
		////fmt.Printf("Failed to delete persistent context from Redis: %v\n", pErr)
		// If conversation deletion succeeded but this failed, maybe return a specific error?
		// For now, just log it and return the original error if it exists, or this one if not.
		if err == nil { // If conversation delete was ok, return this error
			return nil, fmt.Errorf("failed to clear persistent context: %w", pErr)
		}
	}
    //else {
		////fmt.Printf("Successfully deleted persistent context for key: %s\n", persistentContextKey)
	//}

	// If the conversation deletion failed initially, return that error now
	if err != nil {
		return nil, fmt.Errorf("failed to clear conversation history: %w", err)
	}

	////fmt.Printf("Successfully deleted conversation for key: %s\n", conversationKey)
	return map[string]string{"message": "Conversation history cleared successfully"}, nil
}
