package tools
/*
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
*/
