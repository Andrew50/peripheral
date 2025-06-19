// <planner.go>
package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/genai"
)

// Pre-compile regex pattern for ticker formatting cleanup
var tickerFormattingRegex = regexp.MustCompile(`\$\$\$([A-Z0-9]+)-\d+\$\$\$`)

type DirectAnswer struct {
	ContentChunks []ContentChunk `json:"content_chunks"`
	Suggestions   []string       `json:"suggestions,omitempty"`
	TokenCounts   TokenCounts    `json:"token_counts,omitempty"`
}

// ContentChunk represents a piece of content in the response sequence
type ContentChunk struct {
	Type    string      `json:"type"`    // "text", "table", "backtest_table", "plot" (or others later, e.g., "image")
	Content interface{} `json:"content"` // string for "text", TableData for "table", PlotData for "plot"
}

type Round struct {
	Parallel bool           `json:"parallel"`
	Calls    []FunctionCall `json:"calls"`
}
type Plan struct {
	Stage          Stage       `json:"stage"`
	Rounds         []Round     `json:"rounds,omitempty"`
	Thoughts       string      `json:"thoughts,omitempty"`
	DiscardResults []int64     `json:"discard_results,omitempty"`
	TokenCounts    TokenCounts `json:"token_counts,omitempty"`
}

type FinalResponse struct {
	ContentChunks []ContentChunk `json:"content_chunks"`
	Suggestions   []string       `json:"suggestions,omitempty"`
	TokenCounts   TokenCounts    `json:"token_counts,omitempty"`
}

type TokenCounts struct {
	InputTokenCount    int32 `json:"input_token_count,omitempty"`
	OutputTokenCount   int32 `json:"output_token_count,omitempty"`
	ThoughtsTokenCount int32 `json:"thoughts_token_count,omitempty"`
	TotalTokenCount    int32 `json:"total_token_count,omitempty"`
}

func replySchema() *genai.Schema {
	return &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"content_chunks", "suggestions"},
		Properties: map[string]*genai.Schema{
			"content_chunks": {
				Type:  genai.TypeArray,
				Items: contentChunkSchema(),
			},
			"suggestions": {
				Type:  genai.TypeArray,
				Items: &genai.Schema{Type: genai.TypeString},
			},
		},
		Title:       "AtlantisReply",
		Description: "A valid Atlantis agent response",
	}
}

func contentChunkSchema() *genai.Schema {
	// helper: any scalar allowed in a table cell
	scalar := &genai.Schema{
		AnyOf: []*genai.Schema{
			{Type: genai.TypeString},
			{Type: genai.TypeNumber},
			{Type: genai.TypeBoolean},
		},
	}

	// text chunk
	textSchema := &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"type", "content"},
		Properties: map[string]*genai.Schema{
			"type":    {Type: genai.TypeString, Enum: []string{"text"}},
			"content": {Type: genai.TypeString},
		},
	}

	// table chunk
	tableSchema := &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"type", "content"},
		Properties: map[string]*genai.Schema{
			"type": {Type: genai.TypeString, Enum: []string{"table"}},
			"content": {
				Type:     genai.TypeObject,
				Required: []string{"headers", "rows"},
				Properties: map[string]*genai.Schema{
					"caption": {Type: genai.TypeString},
					"headers": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"rows": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type:  genai.TypeArray,
							Items: scalar,
						},
					},
				},
			},
		},
	}

	// backtest_table chunk
	// columnMapping / columnFormat are arrays of {k,v} objects instead of maps
	keyValSchema := &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"k", "v"},
		Properties: map[string]*genai.Schema{
			"k": {Type: genai.TypeString},
			"v": {Type: genai.TypeString},
		},
	}

	backtestSchema := &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"type", "content"},
		Properties: map[string]*genai.Schema{
			"type": {Type: genai.TypeString, Enum: []string{"backtest_table"}},
			"content": {
				Type:     genai.TypeObject,
				Required: []string{"strategyId", "columns"},
				Properties: map[string]*genai.Schema{
					"strategyId": {Type: genai.TypeInteger},
					"columns":    {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"columnMapping": {
						Type:  genai.TypeArray,
						Items: keyValSchema,
					},
					"columnFormat": {
						Type:  genai.TypeArray,
						Items: keyValSchema,
					},
				},
			},
		},
	}

	// plot chunk
	plotSchema := &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"type", "content"},
		Properties: map[string]*genai.Schema{
			"type": {Type: genai.TypeString, Enum: []string{"plot"}},
			"content": {
				Type:     genai.TypeObject,
				Required: []string{"chart_type", "data"},
				Properties: map[string]*genai.Schema{
					"chart_type": {
						Type: genai.TypeString,
						Enum: []string{"line", "bar", "scatter", "histogram", "heatmap"},
					},
					"title": {Type: genai.TypeString},
					"data": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type: genai.TypeObject,
							Properties: map[string]*genai.Schema{
								"x":    {Type: genai.TypeArray, Items: scalar},
								"y":    {Type: genai.TypeArray, Items: scalar},
								"z":    {Type: genai.TypeArray, Items: scalar}, // for heatmaps
								"name": {Type: genai.TypeString},
								"type": {Type: genai.TypeString},
							},
						},
					},
					"layout": {
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"xaxis": {
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"title": {Type: genai.TypeString},
									"type":  {Type: genai.TypeString},
									"range": {Type: genai.TypeArray, Items: scalar},
								},
							},
							"yaxis": {
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"title": {Type: genai.TypeString},
									"type":  {Type: genai.TypeString},
									"range": {Type: genai.TypeArray, Items: scalar},
								},
							},
						},
					},
				},
			},
		},
	}

	// final union
	return &genai.Schema{
		AnyOf: []*genai.Schema{textSchema, tableSchema, backtestSchema, plotSchema},
	}
}

const planningModel = "gemini-2.5-flash"
const finalResponseModel = "gemini-2.5-flash"

func RunPlanner(ctx context.Context, conn *data.Conn, prompt string, initialRound bool) (interface{}, error) {
	var systemPrompt string
	var err error
	if initialRound {
		systemPrompt, err = getSystemInstruction("defaultSystemPrompt")
		if err != nil {
			return nil, fmt.Errorf("error getting system instruction: %w", err)
		}
	} else {
		systemPrompt, err = getSystemInstruction("IntermediateSystemPrompt")
		if err != nil {
			return nil, fmt.Errorf("error getting system instruction: %w", err)
		}
	}
	plan, err := _geminiGeneratePlan(ctx, conn, systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("error generating plan: %w", err)
	}
	return plan, nil
}

func _geminiGeneratePlan(ctx context.Context, conn *data.Conn, systemPrompt string, prompt string) (interface{}, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return Plan{}, fmt.Errorf("error getting gemini key: %w", err)
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("error creating gemini client: %w", err)
	}
	////fmt.Println("prompt", prompt)
	thinkingBudget := int32(10000)
	// Enhance the system instruction with tool descriptions
	enhancedSystemInstruction := enhanceSystemPromptWithTools(systemPrompt)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: enhancedSystemInstruction},
			},
		},
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  &thinkingBudget,
		},
		ResponseMIMEType: "application/json",
	}
	fmt.Println("\n\nprompt", prompt)
	result, err := client.Models.GenerateContent(ctx, planningModel, genai.Text(prompt), config)
	if err != nil {
		return Plan{}, fmt.Errorf("gemini had an error generating plan : %w", err)
	}
	// Concatenate the text from *all* parts to ensure we don't miss the JSON payload
	var sb strings.Builder
	if len(result.Candidates) <= 0 {
		return Plan{}, fmt.Errorf("no candidates found in result")
	}
	candidate := result.Candidates[0]
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Thought {
				fmt.Println("Thought: ", part.Text)
				continue
			}
			if part.Text != "" {
				sb.WriteString(part.Text)
				sb.WriteString("\n")
			}
		}
	}
	resultText := strings.TrimSpace(sb.String())
	fmt.Println("Prompt Token Count", result.UsageMetadata.PromptTokenCount)
	fmt.Println("Candidates Token Count", result.UsageMetadata.CandidatesTokenCount)
	fmt.Println("Thoughts Token Count", result.UsageMetadata.ThoughtsTokenCount)
	fmt.Println("Total Token Count", result.UsageMetadata.TotalTokenCount)
	fmt.Println("groundingMetadata", candidate.GroundingMetadata)
	fmt.Println("citationMetadata", candidate.CitationMetadata)
	fmt.Println("\n\n\n\n\nresultText", resultText)

	// --- Extract JSON block --- START
	jsonBlock := ""

	// First try direct parsing of the entire resultText
	var directAns DirectAnswer
	directParseErr := json.Unmarshal([]byte(resultText), &directAns)
	if directParseErr == nil && len(directAns.ContentChunks) > 0 {
		hasValidContent := false
		for _, chunk := range directAns.ContentChunks {
			if chunk.Content != nil && fmt.Sprintf("%v", chunk.Content) != "" {
				hasValidContent = true
				break
			}
		}
		if hasValidContent {
			directAns.Suggestions = cleanTickerFormattingFromSuggestions(directAns.Suggestions)
			directAns.TokenCounts = TokenCounts{
				InputTokenCount:    result.UsageMetadata.PromptTokenCount,
				OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
				ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
				TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
			}
			return directAns, nil
		}
	}

	var plan Plan
	planParseErr := json.Unmarshal([]byte(resultText), &plan)
	if planParseErr == nil && plan.Stage != "" {
		plan.TokenCounts = TokenCounts{
			InputTokenCount:    result.UsageMetadata.PromptTokenCount,
			OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
			ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
			TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
		}
		return plan, nil
	}

	// If no markdown code block found, try to extract JSON block using { } method
	if jsonBlock == "" {
		jsonStartIdx := strings.Index(resultText, "{")

		if jsonStartIdx != -1 {
			// Try to find the matching closing brace by counting braces
			braceCount := 0
			jsonEndIdx := -1

			for i := jsonStartIdx; i < len(resultText); i++ {
				if resultText[i] == '{' {
					braceCount++
				} else if resultText[i] == '}' {
					braceCount--
					if braceCount == 0 {
						jsonEndIdx = i
						break
					}
				}
			}

			if jsonEndIdx != -1 {
				jsonBlock = resultText[jsonStartIdx : jsonEndIdx+1]
				jsonBlock = strings.TrimSpace(jsonBlock)
			}
		}
	}

	// Try unmarshalling the extracted block if it's not empty
	directAns = DirectAnswer{} // Reset the struct
	if jsonBlock != "" {
		blockDirectParseErr := json.Unmarshal([]byte(jsonBlock), &directAns)
		if blockDirectParseErr == nil && len(directAns.ContentChunks) > 0 {
			hasValidContent := false
			for _, chunk := range directAns.ContentChunks {
				if chunk.Content != nil && fmt.Sprintf("%v", chunk.Content) != "" {
					hasValidContent = true
					break
				}
			}
			if hasValidContent {
				directAns.Suggestions = cleanTickerFormattingFromSuggestions(directAns.Suggestions)
				directAns.TokenCounts = TokenCounts{
					InputTokenCount:    result.UsageMetadata.PromptTokenCount,
					OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
					ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
					TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
				}
				return directAns, nil
			}
		}
	}

	plan = Plan{} // Reset the struct
	// Try unmarshalling the extracted block if it's not empty
	if jsonBlock != "" {
		blockPlanParseErr := json.Unmarshal([]byte(jsonBlock), &plan)
		if blockPlanParseErr == nil && plan.Stage != "" {
			plan.TokenCounts = TokenCounts{
				InputTokenCount:    result.UsageMetadata.PromptTokenCount,
				OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
				ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
				TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
			}
			return plan, nil
		}
	}

	// If parsing failed or no JSON block found, return error

	return nil, fmt.Errorf("no valid plan or direct answer found in response")
}

func GetFinalResponse(ctx context.Context, conn *data.Conn, prompt string) (*FinalResponse, error) {
	systemPrompt, err := getSystemInstruction("finalResponseSystemPrompt")
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %w", err)
	}

	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return nil, fmt.Errorf("error getting gemini key: %w", err)
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating gemini client: %w", err)
	}
	thinkingBudget := int32(10000)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  &thinkingBudget,
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   replySchema(),
	}

	result, err := client.Models.GenerateContent(ctx, finalResponseModel, genai.Text(prompt), config)
	if err != nil {
		return nil, fmt.Errorf("gemini had an error generating final response: %w", err)
	}

	if len(result.Candidates) <= 0 {
		return nil, fmt.Errorf("no candidates found in result")
	}

	// Concatenate the text from *all* parts to ensure we capture the full response
	var frSB strings.Builder
	candidate := result.Candidates[0]
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Thought {
				continue
			}
			if part.Text != "" {
				frSB.WriteString(part.Text)
				frSB.WriteString("\n")
			}
		}
	}
	resultText := strings.TrimSpace(frSB.String())
	fmt.Println("resultText", resultText)
	// Try to parse as JSON
	var finalResponse FinalResponse

	// First try direct unmarshaling
	if err := json.Unmarshal([]byte(resultText), &finalResponse); err == nil && len(finalResponse.ContentChunks) > 0 {
		finalResponse.Suggestions = cleanTickerFormattingFromSuggestions(finalResponse.Suggestions)
		finalResponse.TokenCounts = TokenCounts{
			InputTokenCount:    result.UsageMetadata.PromptTokenCount,
			OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
			ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
			TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
		}
		return &finalResponse, nil
	}

	// Try to extract JSON from markdown code blocks first
	var jsonBlock string

	// Look for ```json ... ``` blocks
	jsonCodeBlockStart := strings.Index(resultText, "```json")
	if jsonCodeBlockStart != -1 {
		jsonCodeBlockStart += len("```json")
		// Skip any whitespace after ```json
		for jsonCodeBlockStart < len(resultText) && (resultText[jsonCodeBlockStart] == '\n' || resultText[jsonCodeBlockStart] == '\r' || resultText[jsonCodeBlockStart] == ' ' || resultText[jsonCodeBlockStart] == '\t') {
			jsonCodeBlockStart++
		}

		jsonCodeBlockEnd := strings.Index(resultText[jsonCodeBlockStart:], "```")
		if jsonCodeBlockEnd != -1 {
			jsonBlock = resultText[jsonCodeBlockStart : jsonCodeBlockStart+jsonCodeBlockEnd]
			jsonBlock = strings.TrimSpace(jsonBlock)
		}
	}

	// If no markdown code block found, try to extract JSON block using { } method
	if jsonBlock == "" {
		jsonStartIdx := strings.Index(resultText, "{")

		if jsonStartIdx != -1 {
			// Try to find the matching closing brace by counting braces
			braceCount := 0
			jsonEndIdx := -1

			for i := jsonStartIdx; i < len(resultText); i++ {
				if resultText[i] == '{' {
					braceCount++
				} else if resultText[i] == '}' {
					braceCount--
					if braceCount == 0 {
						jsonEndIdx = i
						break
					}
				}
			}

			if jsonEndIdx != -1 {
				jsonBlock = resultText[jsonStartIdx : jsonEndIdx+1]
				jsonBlock = strings.TrimSpace(jsonBlock)
			}
		}
	}

	// Try parsing the extracted JSON block
	if jsonBlock != "" {
		if err := json.Unmarshal([]byte(jsonBlock), &finalResponse); err == nil && len(finalResponse.ContentChunks) > 0 {
			finalResponse.Suggestions = cleanTickerFormattingFromSuggestions(finalResponse.Suggestions)
			finalResponse.TokenCounts = TokenCounts{
				InputTokenCount:    result.UsageMetadata.PromptTokenCount,
				OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
				ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
				TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
			}
			return &finalResponse, nil
		}
	}

	// Fallback: Treat the text as a single text chunk
	return &FinalResponse{
		ContentChunks: []ContentChunk{{Type: "text", Content: resultText}},
		TokenCounts:   TokenCounts{},
	}, nil
}

// cleanTickerFormattingFromSuggestions removes the $$$TICKER-TIMESTAMP$$$ formatting from suggestions
// and replaces it with just the ticker symbol
func cleanTickerFormattingFromSuggestions(suggestions []string) []string {
	cleaned := make([]string, len(suggestions))
	for i, suggestion := range suggestions {
		cleaned[i] = tickerFormattingRegex.ReplaceAllString(suggestion, "$1")
	}
	return cleaned
}

const titleModel = "gemini-2.5-flash-lite-preview-06-17"
const titleSystemPrompt = "You are a helpful assistant that generates a title for a conversation based on the first query message given to you. It should be no more than 40 characters and should be 2-4 words, capitalized like a title. Stock symbols should be fully capitalized. Make the title informative and accurately encapsulate the query. Never make it fully capitalized."

func GenerateConversationTitle(conn *data.Conn, _ int, query string) (string, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return "", fmt.Errorf("error getting gemini key: %w", err)
	}
	// Create a new client using the API key
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("error creating gemini client: %w", err)
	}
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: titleSystemPrompt},
			},
		},
	}
	result, err := client.Models.GenerateContent(
		context.Background(),
		titleModel,
		genai.Text(query),
		config,
	)
	if err != nil {
		return "", fmt.Errorf("error generating content with thinking model: %w", err)
	}
	// Extract the clean text response for display
	responseText := ""
	if len(result.Candidates) > 0 {
		candidate := result.Candidates[0]
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					responseText = part.Text
					break
				}
			}
		}
	}
	return responseText, nil

}

// </planner.go>
