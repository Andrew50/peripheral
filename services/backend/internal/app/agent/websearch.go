package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

const geminiWebSearchModel = "gemini-2.5-flash-preview-04-17"

type WebSearchArgs struct {
	Query string `json:"query"`
}
type WebSearchResult struct {
	ResultText string   `json:"result_text"`
	Citations  []string `json:"citations,omitempty"`
}

// RunWebSearch performs a web search using the Tavily API.
func RunWebSearch(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args WebSearchArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error unmarshalling args: %w", err)
	}
	systemPrompt, err := getSystemInstruction("webSearchPrompt")
	if err != nil {
		return nil, fmt.Errorf("error getting search system instruction: %w", err)
	}
	return _geminiWebSearch(conn, systemPrompt, args.Query)
}

func _geminiWebSearch(conn *data.Conn, systemPrompt string, prompt string) (interface{}, error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return nil, fmt.Errorf("error getting gemini key: %w", err)
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating gemini client: %w", err)
	}
	thinkingBudget := int32(1000)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
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
	result, err := client.Models.GenerateContent(context.Background(), geminiWebSearchModel, genai.Text(prompt), config)
	if err != nil {
		return WebSearchResult{}, fmt.Errorf("error generating web search: %w", err)
	}
	resultText := ""
	if len(result.Candidates) <= 0 {
		return WebSearchResult{}, fmt.Errorf("no candidates found in result")
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
	//if candidate.GroundingMetadata != nil {
	////fmt.Println("groundingMetadata", candidate.GroundingMetadata)
	//}
	return WebSearchResult{
		ResultText: resultText,
	}, nil
}
