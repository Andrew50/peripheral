package tools

import (
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed prompts/*
var fs embed.FS //

// -----------------------------------------------------------------------------
// Helper: quote-each item from the slice and join with ", ".
// -----------------------------------------------------------------------------
func joinQuoteSlice(vals []string) string {
	// Sort a copy to ensure consistent output without modifying the original slice
	sortedVals := make([]string, len(vals))
	copy(sortedVals, vals)
	sort.Strings(sortedVals)

	q := make([]string, len(sortedVals))
	for i, k := range sortedVals {
		q[i] = strconv.Quote(k) // adds surrounding double-quotes & escapes
	}
	return strings.Join(q, ", ")
}

// -----------------------------------------------------------------------------
// Helper: quote-each key from the map[string]struct{} and join with ", ".
// Ensures consistent ordering by sorting keys.
// -----------------------------------------------------------------------------
func joinQuoteSet(vals map[string]struct{}) string {
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Sort keys for consistent output

	q := make([]string, len(keys))
	for i, k := range keys {
		q[i] = strconv.Quote(k) // adds surrounding double-quotes & escapes
	}
	return strings.Join(q, ", ")
}

func joinQuoteMapKeys[K comparable, V any](m map[K]V) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, fmt.Sprintf("%v", k))
	}
	sort.Strings(keys)
	for i, k := range keys {
		keys[i] = strconv.Quote(k)
	}
	return strings.Join(keys, ", ")
}
// -----------------------------------------------------------------------------
// getSystemInstruction reads the prompt file <name>.txt and expands all {{…}}
// placeholders using an internally generated map of standard replacements.
// It returns an error if the prompt file cannot be read or if a placeholder
// in the template does not have a corresponding replacement defined internally.
// -----------------------------------------------------------------------------
func getSystemInstruction(name string) (string, error) {
	// --- 1. Generate Replacements ---
	replacements := make(map[string]string)

	// Time
	now := time.Now()
	replacements["CURRENT_TIME"] = now.Format(time.RFC3339)
	replacements["CURRENT_TIME_MILLISECONDS"] = fmt.Sprintf("%d", now.UnixMilli())

	// Static Validation Lists (from spec.go)
	replacements["ValidSecurityFeatures"] = joinQuoteSet(ValidSecurityFeatures)
	replacements["ValidTimeframes"] = joinQuoteSlice(timeframes)
	replacements["ValidOutputTypes"] = joinQuoteSlice(outputTypes)
	replacements["ValidExprOperators"] = joinQuoteSlice(exprOperators)
	replacements["ValidComparisonOperators"] = joinQuoteSlice(comparisonOperators)
	replacements["ValidDirections"] = joinQuoteSlice(directions)

	// Dynamic Validation Lists (from spec.go, require mutex)
	dynamicSetMutex.RLock() // Acquire read lock

	// Combine OHLCV and Fundamental features for ValidExprColumns
	exprCols := make([]string, 0, len(ohlcvFeatures)+len(validFundamentalFeatures))
	exprCols = append(exprCols, ohlcvFeatures...)
	for k := range validFundamentalFeatures {
		exprCols = append(exprCols, k)
	}
	// Use joinQuoteSlice which sorts for consistency
	replacements["ValidExprColumns"] = joinQuoteSlice(exprCols)

	// Potentially add ValidSectors, ValidIndustries if needed by prompts
    replacements["ValidSectors"]    = joinQuoteMapKeys(validSectors)
	replacements["ValidIndustries"] = joinQuoteMapKeys(validIndustries)
	dynamicSetMutex.RUnlock() // Release read lock

	// --- 2. Read Prompt File ---
	raw, err := fs.ReadFile("prompts/" + name + ".txt")
	if err != nil {
		return "", fmt.Errorf("reading prompt '%s': %w", name, err)
	}

	// --- 3. Perform Replacement ---
	// Regex to capture {{ … }} including inner text.
	rePlaceholder := regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

	var missingPlaceholders []string
	out := rePlaceholder.ReplaceAllStringFunc(string(raw), func(m string) string {
		key := strings.TrimSpace(rePlaceholder.FindStringSubmatch(m)[1])
		if val, ok := replacements[key]; ok {
			return val
		}
		// If the key is not found in the generated replacements map,
		// collect it to return a comprehensive error.
		missingPlaceholders = append(missingPlaceholders, key)
		return m // Keep the original placeholder text for error reporting
	})

	// Check if any placeholders were missing
	if len(missingPlaceholders) > 0 {
		// Sort for consistent error message
		sort.Strings(missingPlaceholders)
		return "", fmt.Errorf("required placeholder(s) [%s] not found in internal replacements for %s.txt", strings.Join(missingPlaceholders, ", "), name)
	}

	return out, nil
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

