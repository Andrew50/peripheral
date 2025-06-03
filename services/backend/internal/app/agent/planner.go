// <planner.go>
package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type DirectAnswer struct {
	ContentChunks []ContentChunk `json:"content_chunks"`
	Suggestions   []string       `json:"suggestions,omitempty"`
	TokenCounts   TokenCounts    `json:"token_counts,omitempty"`
}
type Round struct {
	Parallel bool           `json:"parallel"`
	Calls    []FunctionCall `json:"calls"`
}
type Plan struct {
	Stage       Stage       `json:"stage"`
	Rounds      []Round     `json:"rounds,omitempty"`
	TokenCounts TokenCounts `json:"token_counts,omitempty"`
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

const planningModel = "gemini-2.5-flash-preview-05-20"
const finalResponseModel = "gemini-2.5-flash-preview-05-20"

func RunPlanner(ctx context.Context, conn *data.Conn, prompt string) (interface{}, error) {
	systemPrompt, err := getSystemInstruction("defaultSystemPrompt")
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %w", err)
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
	thinkingBudget := int32(1000)
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
	fmt.Printf("DEBUG: Direct DirectAnswer parse - error: %v, contentChunks length: %d\n", directParseErr, len(directAns.ContentChunks))
	if directParseErr == nil && len(directAns.ContentChunks) > 0 {
		// Additional check: make sure at least one content chunk has actual content
		hasValidContent := false
		for _, chunk := range directAns.ContentChunks {
			if chunk.Content != nil && fmt.Sprintf("%v", chunk.Content) != "" {
				hasValidContent = true
				break
			}
		}
		fmt.Printf("DEBUG: DirectAnswer has valid content: %t\n", hasValidContent)
		if hasValidContent {
			fmt.Printf("DEBUG: DirectAnswer parsing SUCCESS, returning DirectAnswer\n")
			directAns.TokenCounts = TokenCounts{
				InputTokenCount:    result.UsageMetadata.PromptTokenCount,
				OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
				ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
				TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
			}
			return directAns, nil
		} else {
			fmt.Printf("DEBUG: DirectAnswer has empty content chunks, skipping\n")
		}
	}

	var plan Plan
	planParseErr := json.Unmarshal([]byte(resultText), &plan)
	fmt.Printf("DEBUG: Direct Plan parse - error: %v, stage: %s\n", planParseErr, plan.Stage)
	if planParseErr == nil {
		fmt.Printf("DEBUG: Plan parsing SUCCESS, returning Plan\n")
		plan.TokenCounts = TokenCounts{
			InputTokenCount:    result.UsageMetadata.PromptTokenCount,
			OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
			ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
			TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
		}
		return plan, nil
	}

	// If direct parsing fails, try to extract JSON block
	jsonStartIdx := strings.Index(resultText, "{")
	jsonEndIdx := strings.LastIndex(resultText, "}")
	fmt.Printf("DEBUG: JSON extraction - start: %d, end: %d\n", jsonStartIdx, jsonEndIdx)
	if jsonStartIdx != -1 && jsonEndIdx != -1 && jsonEndIdx > jsonStartIdx {
		jsonBlock = resultText[jsonStartIdx : jsonEndIdx+1]
		fmt.Printf("DEBUG: Extracted JSON block length: %d\n", len(jsonBlock))
		fmt.Printf("DEBUG: First 200 chars of extracted JSON: %.200s\n", jsonBlock)
	}

	// Try unmarshalling the extracted block if it's not empty
	directAns = DirectAnswer{} // Reset the struct
	if jsonBlock != "" {
		blockDirectParseErr := json.Unmarshal([]byte(jsonBlock), &directAns)
		fmt.Printf("DEBUG: Block DirectAnswer parse - error: %v, contentChunks length: %d\n", blockDirectParseErr, len(directAns.ContentChunks))
		if blockDirectParseErr == nil && len(directAns.ContentChunks) > 0 {
			// Additional check: make sure at least one content chunk has actual content
			hasValidContent := false
			for _, chunk := range directAns.ContentChunks {
				if chunk.Content != nil && fmt.Sprintf("%v", chunk.Content) != "" {
					hasValidContent = true
					break
				}
			}
			fmt.Printf("DEBUG: Block DirectAnswer has valid content: %t\n", hasValidContent)
			if hasValidContent {
				fmt.Printf("DEBUG: Block DirectAnswer parsing SUCCESS, returning DirectAnswer\n")
				directAns.TokenCounts = TokenCounts{
					InputTokenCount:    result.UsageMetadata.PromptTokenCount,
					OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
					ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
					TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
				}
				return directAns, nil
			} else {
				fmt.Printf("DEBUG: Block DirectAnswer has empty content chunks, skipping\n")
			}
		}
	}

	plan = Plan{} // Reset the struct
	// Try unmarshalling the extracted block if it's not empty
	if jsonBlock != "" {
		blockPlanParseErr := json.Unmarshal([]byte(jsonBlock), &plan)
		fmt.Printf("DEBUG: Block Plan parse - error: %v, stage: %s\n", blockPlanParseErr, plan.Stage)
		if blockPlanParseErr == nil {
			fmt.Printf("DEBUG: Block Plan parsing SUCCESS, returning Plan\n")
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
	fmt.Printf("DEBUG: All parsing attempts failed - resultText length: %d\n", len(resultText))
	fmt.Printf("DEBUG: resultText (truncated to 500 chars): %.500s\n", resultText)
	return nil, fmt.Errorf("no valid plan or direct answer found in response: %s", resultText)
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
	thinkingBudget := int32(2000)
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

	// Try to parse as JSON
	var finalResponse FinalResponse

	// First try direct unmarshaling
	if err := json.Unmarshal([]byte(resultText), &finalResponse); err == nil && len(finalResponse.ContentChunks) > 0 {
		finalResponse.TokenCounts = TokenCounts{
			InputTokenCount:    result.UsageMetadata.PromptTokenCount,
			OutputTokenCount:   result.UsageMetadata.CandidatesTokenCount,
			ThoughtsTokenCount: result.UsageMetadata.ThoughtsTokenCount,
			TotalTokenCount:    result.UsageMetadata.TotalTokenCount,
		}
		return &finalResponse, nil
	}

	// Try to find JSON block in the text
	jsonStartIdx := strings.Index(resultText, "{")
	jsonEndIdx := strings.LastIndex(resultText, "}")
	if jsonStartIdx != -1 && jsonEndIdx != -1 && jsonEndIdx > jsonStartIdx {
		jsonBlock := resultText[jsonStartIdx : jsonEndIdx+1]
		if err := json.Unmarshal([]byte(jsonBlock), &finalResponse); err == nil && len(finalResponse.ContentChunks) > 0 {
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

// </planner.go>
