package agent

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed prompts/* chatUXPrompts/*
var fs embed.FS // 2️⃣ compiled into the binary

// Cache for enhanced system prompts to avoid recomputing on every request
var (
	cachedEnhancedSystemPrompts = make(map[string]string)
	systemPromptCacheMutex      sync.RWMutex
)

// GetSystemInstruction returns the processed prompt named <name>.txt
func GetSystemInstruction(name string) (string, error) {
	raw, err := fs.ReadFile("prompts/" + name + ".txt") // 3️⃣ no paths, no os.ReadFile
	if err != nil {
		return "", fmt.Errorf("reading prompt: %w", err)
	}
	const rfc3339Seconds = "2006-01-02T15:04:05Z07:00"
	now := time.Now()
	// Get current date in EST timezone
	estLocation, _ := time.LoadLocation("America/New_York")
	estTime := now.In(estLocation)
	s := string(raw)
	s = strings.ReplaceAll(s, "{{CURRENT_TIME}}",
		estTime.Format(rfc3339Seconds))
	s = strings.ReplaceAll(s, "{{CURRENT_TIME_SECONDS}}",
		strconv.FormatInt(estTime.Unix(), 10))
	s = strings.ReplaceAll(s, "{{CURRENT_YEAR}}",
		strconv.Itoa(estTime.Year()))
	s = strings.ReplaceAll(s, "{{CURRENT_DATE_EST}}",
		estTime.Format("01-02-2006"))
	// Fast check if we need to process any constraints
	if strings.Contains(s, "{{COMMON_CONSTRAINTS}}") {
		constraints, err := fs.ReadFile("prompts/commonConstraints.txt")
		if err != nil {
			return "", fmt.Errorf("reading common constraints: %w", err)
		}
		s = strings.ReplaceAll(s, "{{COMMON_CONSTRAINTS}}", string(constraints))
	}
	if strings.Contains(s, "{{EXECUTION_CONSTRAINTS}}") {
		executionConstraints, err := fs.ReadFile("prompts/executionConstraints.txt")
		if err != nil {
			return "", fmt.Errorf("reading execution constraints: %w", err)
		}
		s = strings.ReplaceAll(s, "{{EXECUTION_CONSTRAINTS}}", string(executionConstraints))
	}
	return s, nil
}

func getCleanThinkingTracePrompt() string {
	raw, err := fs.ReadFile("chatUXPrompts/cleanThinkingTrace.txt")
	if err != nil {
		return ""
	}
	return string(raw)
}

// enhanceSystemPromptWithTools adds a formatted list of available tools to the system prompt
func enhanceSystemPromptWithTools(basePrompt string, userSpecificTools bool) string {
	// Check cache first
	systemPromptCacheMutex.RLock()
	if cached, exists := cachedEnhancedSystemPrompts[basePrompt]; exists {
		systemPromptCacheMutex.RUnlock()
		return cached
	}
	systemPromptCacheMutex.RUnlock()

	// Compute enhanced prompt
	var toolsDescription strings.Builder

	// Start with the base prompt
	toolsDescription.WriteString(basePrompt)
	toolsDescription.WriteString("\n## Function Tools")
	toolsDescription.WriteString("\n### Functions Available in JSON Format")

	toolsAsJSON, err := GetToolsAsJSON(userSpecificTools)
	if err != nil {
		fmt.Println("Error getting tools as JSON:", err)
	}
	toolsDescription.WriteString(toolsAsJSON)

	/* // Sort tool names for consistent output
	var toolNames []string
	for name := range Tools {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)

	// Add each tool's description and parameters
	for _, name := range toolNames {
		tool := Tools[name]
		if !userSpecificTools && tool.UserSpecificTool {
			continue // skip user specific tools if not user specific
		}
		// Add function name and description
		toolsDescription.WriteString(fmt.Sprintf("- %s: %s\n", name, tool.FunctionDeclaration.Description))

		// Add parameters if they exist
		if tool.FunctionDeclaration.Parameters != nil && len(tool.FunctionDeclaration.Parameters.Properties) > 0 {
			toolsDescription.WriteString("  Params:\n")

			// Get required parameters
			required := make(map[string]bool)
			for _, req := range tool.FunctionDeclaration.Parameters.Required {
				required[req] = true
			}

			// Add each parameter with its description
			for paramName, paramSchema := range tool.FunctionDeclaration.Parameters.Properties {
				isReq := ""
				if !required[paramName] {
					isReq = "?"
				}
				toolsDescription.WriteString(fmt.Sprintf("  - %s%s: %s\n", paramName, isReq, paramSchema.Description))
			}
		}

		// Add spacing between functions
		toolsDescription.WriteString("\n")
	}*/
	enhancedPrompt := toolsDescription.String()

	// Cache the result
	systemPromptCacheMutex.Lock()
	cachedEnhancedSystemPrompts[basePrompt] = enhancedPrompt
	systemPromptCacheMutex.Unlock()

	return enhancedPrompt
}

// GetToolsAsJSON attempts to directly marshal the existing tools to JSON
func GetToolsAsJSON(userSpecificTools bool) (string, error) {
	var toolLines []string

	// Sort tool names for consistent output
	var toolNames []string
	for name := range Tools {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)

	for _, name := range toolNames {
		tool := Tools[name]
		if !userSpecificTools && tool.UserSpecificTool {
			continue // skip user specific tools if not user specific
		}

		// Marshal each tool individually
		jsonBytes, err := json.Marshal(tool.FunctionDeclaration)
		if err != nil {
			return "", fmt.Errorf("error marshaling tool %s to JSON: %w", name, err)
		}
		toolLines = append(toolLines, string(jsonBytes))
	}

	result := strings.Join(toolLines, "\n")
	return result, nil
}

// ClearSystemPromptCache clears the cached enhanced system prompts
// This should be called during hot reloads when tools or prompts change
func ClearSystemPromptCache() {
	systemPromptCacheMutex.Lock()
	defer systemPromptCacheMutex.Unlock()

	// Clear the cache map
	for k := range cachedEnhancedSystemPrompts {
		delete(cachedEnhancedSystemPrompts, k)
	}
}
