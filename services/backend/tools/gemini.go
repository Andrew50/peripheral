package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

/*
func getTextResponseFromGemini(ctx context.Context, conn *utils.Conn, model string, systemPrompt string, query string) (string, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return "", fmt.Errorf("error getting gemini key: %w", err)
	}

	// Create a new client using the API key
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("error creating gemini client: %w", err)
	}

	systemInstruction, err := getSystemInstruction(systemPrompt)
	if err != nil {
		return "", fmt.Errorf("error getting system instruction: %w", err)
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
	}
	result, err := client.Models.GenerateContent(ctx, model, genai.Text(query), config)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}

	// Extract the response text
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	// Get the text from the response
	text := fmt.Sprintf("%v", result.Candidates[0].Content.Parts[0].Text)
	return text, nil
}*/

// FunctionCall represents a function to be called with its arguments
type FunctionCall struct {
	Name   string          `json:"name"`
	CallID string          `json:"call_id,omitempty"`
	Args   json.RawMessage `json:"args,omitempty"`
}

// FunctionResponse represents the response from the LLM with function calls
type FunctionResponse struct {
	FunctionCalls []FunctionCall `json:"function_calls"`
}

type Citation struct {
	Title string `json:"title"`
	Url   string `json:"url"`
}

type GeminiFunctionResponse struct {
	FunctionCalls []FunctionCall `json:"function_calls"`
	Text          string         `json:"text"`
	Citations     []Citation     `json:"citations,omitempty"`
}

func getGeminiFunctionThinking(ctx context.Context, conn *utils.Conn, systemPrompt string, query string, model string) (*GeminiFunctionResponse, error) {
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
	baseSystemInstruction, err := getSystemInstruction(systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %w", err)
	}

	thinkingBudget := int32(2000)

	// Enhance the system instruction with tool descriptions
	enhancedSystemInstruction := enhanceSystemPromptWithTools(baseSystemInstruction)
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
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					responseText = part.Text
					break
				}
			}
		}
		fmt.Println("Grounding:", candidate.GroundingMetadata)
		// Collect all indices first
		// More efficient deduplication: collect and check uniqueness in one pass
		seen := make(map[int]bool)
		usedGroundingChunkIndices := []int{} // This will store the unique indices
		if candidate.GroundingMetadata != nil {
			if len(candidate.GroundingMetadata.GroundingSupports) > 0 {
				for _, groundingSupport := range candidate.GroundingMetadata.GroundingSupports {
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
		for _, index := range usedGroundingChunkIndices {
			groundingChunk := candidate.GroundingMetadata.GroundingChunks[index]
			if groundingChunk.Web != nil {
				citations = append(citations, Citation{
					Title: groundingChunk.Web.Title,
					Url:   groundingChunk.Web.URI,
				})
			}
			if groundingChunk.RetrievedContext != nil {
				citations = append(citations, Citation{
					Title: groundingChunk.RetrievedContext.Title,
					Url:   groundingChunk.RetrievedContext.URI,
				})
			}
		}
	}
	fmt.Println("Citations:", citations)
	response := &GeminiFunctionResponse{
		FunctionCalls: []FunctionCall{},
		Text:          responseText,
		Citations:     citations,
	}
	return response, nil
}

// getGeminiFunctionResponse uses the Google Function API to return an ordered list of functions to execute
func getGeminiFunctionResponse(ctx context.Context, conn *utils.Conn, query string) (*GeminiFunctionResponse, error) {
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

	systemInstruction := "You are a helpful assistant that can run functions"
	var geminiTools []*genai.Tool
	for _, tool := range GetTools(false) {
		// Convert the FunctionDeclaration to a Tool
		geminiTools = append(geminiTools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        tool.FunctionDeclaration.Name,
					Description: tool.FunctionDeclaration.Description,
					Parameters:  tool.FunctionDeclaration.Parameters,
				},
			},
		})
	}

	config := &genai.GenerateContentConfig{
		Tools: geminiTools,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
	}
	// Check if query has conversation history format ("User: ... Assistant: ...")
	// If it does, use that directly as the prompt
	if strings.Contains(query, "User:") && strings.Contains(query, "Assistant:") {
		// Use the formatted query directly since it already contains the conversation history
		result, err := client.Models.GenerateContent(
			ctx,
			"gemini-2.0-flash",
			genai.Text(query),
			config,
		)
		if err != nil {
			return nil, fmt.Errorf("error generating content with history: %w", err)
		}

		// Process the result similarly to the single-query case
		responseText := ""
		if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
			for _, part := range result.Candidates[0].Content.Parts {
				if part.Text != "" {
					responseText = part.Text
					break
				}
			}
		}

		// Extract function calls
		var functionCalls []FunctionCall
		for _, candidate := range result.Candidates {
			if candidate.Content == nil {
				continue
			}

			for _, part := range candidate.Content.Parts {
				// Check if the part is a FunctionCall
				if fc := part.FunctionCall; fc != nil {
					// Convert arguments to JSON
					args, err := json.Marshal(fc.Args)
					if err != nil {
						return nil, fmt.Errorf("error marshaling function args: %w", err)
					}

					functionCalls = append(functionCalls, FunctionCall{
						Name: fc.Name,
						Args: args,
					})
				}
			}
		}

		return &GeminiFunctionResponse{
			FunctionCalls: functionCalls,
			Text:          responseText,
		}, nil
	}

	// Fall back to the original implementation for simple queries
	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-001", genai.Text(query), config)
	if err != nil {
		return nil, fmt.Errorf("error generating content: %w", err)
	}

	// Extract the clean text response for display
	responseText := ""
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		for _, part := range result.Candidates[0].Content.Parts {
			if part.Text != "" {
				responseText = part.Text
				break
			}
		}
	}

	// Print the response for debugging
	fmt.Println("Gemini response:", responseText)

	// Extract function calls from response
	var functionCalls []FunctionCall

	// Process the response to extract function calls
	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			// Check if the part is a FunctionCall
			if fc := part.FunctionCall; fc != nil {
				// Convert arguments to JSON
				args, err := json.Marshal(fc.Args)
				if err != nil {
					return nil, fmt.Errorf("error marshaling function args: %w", err)
				}

				functionCalls = append(functionCalls, FunctionCall{
					Name: fc.Name,
					Args: args,
				})
			}
		}
	}

	return &GeminiFunctionResponse{
		FunctionCalls: functionCalls,
		Text:          responseText,
	}, nil
}
