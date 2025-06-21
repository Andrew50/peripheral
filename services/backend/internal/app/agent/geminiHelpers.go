package agent

import (
	"embed"

	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed prompts/*
var fs embed.FS // 2️⃣ compiled into the binary

// Cache for enhanced system prompts to avoid recomputing on every request
var (
	cachedEnhancedSystemPrompts = make(map[string]string)
	systemPromptCacheMutex      sync.RWMutex
)

// getSystemInstruction returns the processed prompt named <name>.txt
func getSystemInstruction(name string) (string, error) {
	raw, err := fs.ReadFile("prompts/" + name + ".txt") // 3️⃣ no paths, no os.ReadFile
	if err != nil {
		return "", fmt.Errorf("reading prompt: %w", err)
	}
	constraints, err := fs.ReadFile("prompts/commonConstraints.txt")
	if err != nil {
		return "", fmt.Errorf("reading common constraints: %w", err)
	}
	const rfc3339Seconds = "2006-01-02T15:04:05Z07:00"
	now := time.Now()

	// Get current date in EST timezone
	estLocation, _ := time.LoadLocation("America/New_York")
	estTime := now.In(estLocation)

	s := strings.ReplaceAll(string(raw), "{{COMMON_CONSTRAINTS}}", string(constraints))
	s = strings.ReplaceAll(s, "{{CURRENT_TIME}}",
		estTime.Format(rfc3339Seconds))
	s = strings.ReplaceAll(s, "{{CURRENT_TIME_SECONDS}}",
		strconv.FormatInt(estTime.Unix(), 10))
	s = strings.ReplaceAll(s, "{{CURRENT_YEAR}}",
		strconv.Itoa(estTime.Year()))
	s = strings.ReplaceAll(s, "{{CURRENT_DATE_EST}}",
		estTime.Format("01-02-2006"))
	return s, nil
}

// enhanceSystemPromptWithTools adds a formatted list of available tools to the system prompt
func enhanceSystemPromptWithTools(basePrompt string) string {
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
	toolsDescription.WriteString("\n\nHere are the available tools. '?' indicates an optional parameter.\n")

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
	}

	enhancedPrompt := toolsDescription.String()

	// Cache the result
	systemPromptCacheMutex.Lock()
	cachedEnhancedSystemPrompts[basePrompt] = enhancedPrompt
	systemPromptCacheMutex.Unlock()

	return enhancedPrompt
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
