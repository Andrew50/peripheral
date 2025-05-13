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
	TokenCount    int32          `json:"token_count"`
}
type Round struct {
	Parallel bool           `json:"parallel"`
	Calls    []FunctionCall `json:"calls"`
}
type Plan struct {
	Stage      Stage   `json:"stage"`
	Rounds     []Round `json:"rounds,omitempty"`
	TokenCount int32   `json:"token_count"`
}

type FinalResponse struct {
	ContentChunks []ContentChunk `json:"content_chunks"`
	TokenCount    int32          `json:"token_count"`
}

const planningModel = "gemini-2.5-flash-preview-04-17"
const finalResponseModel = "gemini-2.5-flash-preview-04-17"

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
	fmt.Println("prompt", prompt)
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
	result, err := client.Models.GenerateContent(ctx, planningModel, genai.Text(prompt), config)
	if err != nil {
		return Plan{}, fmt.Errorf("gemini had an error generating plan : %w", err)
	}
	resultText := ""
	if len(result.Candidates) <= 0 {
		return Plan{}, fmt.Errorf("no candidates found in result")
	}
	candidate := result.Candidates[0]
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				resultText = part.Text
				break
			}
		}
	}
	fmt.Println("\n TOKEN COUNT: ", candidate.TokenCount)
	fmt.Println("groundingMetadata", candidate.GroundingMetadata)
	fmt.Println("citationMetadata", candidate.CitationMetadata)
	fmt.Println("\n\n\n\n\nresultText", resultText)

	// --- Extract JSON block --- START
	jsonBlock := ""
	jsonStartIdx := strings.Index(resultText, "{")
	jsonEndIdx := strings.LastIndex(resultText, "}")
	if jsonStartIdx != -1 && jsonEndIdx != -1 && jsonEndIdx > jsonStartIdx {
		jsonBlock = resultText[jsonStartIdx : jsonEndIdx+1]
	} else {
		// If no JSON block found, we can't parse it as Plan or DirectAnswer
		// Depending on expected behavior, you might return an error or default
		// For now, let the unmarshal attempts fail below, which leads to the final error
		fmt.Println("Warning: No JSON block found in planner response")
	}
	// --- Extract JSON block --- END

	var directAns DirectAnswer
	// Try unmarshalling the extracted block if it's not empty
	if jsonBlock != "" && json.Unmarshal([]byte(jsonBlock), &directAns) == nil && len(directAns.ContentChunks) > 0 {
		directAns.TokenCount = candidate.TokenCount
		return directAns, nil
	}

	var plan Plan
	// Try unmarshalling the extracted block if it's not empty
	if jsonBlock != "" && json.Unmarshal([]byte(jsonBlock), &plan) == nil {
		plan.TokenCount = candidate.TokenCount
		return plan, nil
	}

	// If parsing failed or no JSON block found, return error
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

	resultText := ""
	candidate := result.Candidates[0]
	tokenCount := candidate.TokenCount
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				resultText = part.Text
				break
			}
		}
	}

	// Try to parse as JSON
	var finalResponse FinalResponse

	// First try direct unmarshaling
	if err := json.Unmarshal([]byte(resultText), &finalResponse); err == nil && len(finalResponse.ContentChunks) > 0 {
		finalResponse.TokenCount = tokenCount
		return &finalResponse, nil
	}

	// Try to find JSON block in the text
	jsonStartIdx := strings.Index(resultText, "{")
	jsonEndIdx := strings.LastIndex(resultText, "}")
	if jsonStartIdx != -1 && jsonEndIdx != -1 && jsonEndIdx > jsonStartIdx {
		jsonBlock := resultText[jsonStartIdx : jsonEndIdx+1]
		if err := json.Unmarshal([]byte(jsonBlock), &finalResponse); err == nil && len(finalResponse.ContentChunks) > 0 {
			finalResponse.TokenCount = tokenCount
			return &finalResponse, nil
		}
	}

	// Fallback: Treat the text as a single text chunk
	return &FinalResponse{
		ContentChunks: []ContentChunk{{Type: "text", Content: resultText}},
		TokenCount:    tokenCount,
	}, nil
}

// </planner.go>
