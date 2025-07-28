package agent

import (
	"backend/internal/data"
	"context"
	"fmt"

	"google.golang.org/genai"
)

// FunctionResponse represents the response from the LLM with function calls
type FunctionResponse struct {
	FunctionCalls []FunctionCall `json:"function_calls"`
}

type GeminiFunctionResponse struct {
	FunctionCalls []FunctionCall `json:"function_calls"`
	Text          string         `json:"text"`
	Citations     []Citation     `json:"citations,omitempty"`
}

func getGeminiFunctionThinking(ctx context.Context, conn *data.Conn, systemPrompt string, query string, model string) (*GeminiFunctionResponse, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return nil, fmt.Errorf("error getting gemini key: %w", err)
	}

	// Create a new client using the API key
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating gemini client: %w", err)
	}

	// Get the system instruction
	baseSystemInstruction, err := GetSystemInstruction(systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %w", err)
	}

	thinkingBudget := int32(1000)

	// Enhance the system instruction with tool descriptions
	enhancedSystemInstruction := enhanceSystemPromptWithTools(baseSystemInstruction, true)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: enhancedSystemInstruction},
			},
		},
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  &thinkingBudget,
		},
	}

	result, err := client.Models.GenerateContent(
		ctx,
		model,
		genai.Text(query),
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("error generating content with thinking model: %w", err)
	}
	citations := []Citation{}
	// Extract the clean text response for display
	responseText := ""
	if len(result.Candidates) > 0 {
		candidate := result.Candidates[0]
		if candidate != nil && candidate.Content != nil && candidate.Content.Parts != nil {
			for _, part := range candidate.Content.Parts {
				if part != nil && part.Text != "" {
					responseText = part.Text
					break
				}
			}
		}
		seen := make(map[int]bool)
		usedGroundingChunkIndices := []int{} // This will store the unique indices
		if candidate != nil && candidate.GroundingMetadata != nil {
			if len(candidate.GroundingMetadata.GroundingSupports) > 0 {
				for _, groundingSupport := range candidate.GroundingMetadata.GroundingSupports {
					if groundingSupport != nil && groundingSupport.GroundingChunkIndices != nil {
						// Iterate through the indices within this support
						for _, index := range groundingSupport.GroundingChunkIndices {
							// Append each index (casting if necessary, assuming int32 from proto)
							currentIndex := int(index)
							// Check if we've seen this index before adding
							if !seen[currentIndex] {
								seen[currentIndex] = true
								// Correctly assign the result of append back to the slice
								usedGroundingChunkIndices = append(usedGroundingChunkIndices, currentIndex)
							}
						}
					}
				}
			}
		}
		if candidate != nil && candidate.GroundingMetadata != nil && candidate.GroundingMetadata.GroundingChunks != nil {
			for _, index := range usedGroundingChunkIndices {
				if index >= 0 && index < len(candidate.GroundingMetadata.GroundingChunks) {
					groundingChunk := candidate.GroundingMetadata.GroundingChunks[index]
					if groundingChunk != nil {
						if groundingChunk.Web != nil {
							citations = append(citations, Citation{
								Title: groundingChunk.Web.Title,
								URL:   groundingChunk.Web.URI,
							})
						}
						if groundingChunk.RetrievedContext != nil {
							citations = append(citations, Citation{
								Title: groundingChunk.RetrievedContext.Title,
								URL:   groundingChunk.RetrievedContext.URI,
							})
						}
					}
				}
			}
		}
	}
	////fmt.Println("Citations:", citations)
	response := &GeminiFunctionResponse{
		FunctionCalls: []FunctionCall{},
		Text:          responseText,
		Citations:     citations,
	}
	return response, nil
}
