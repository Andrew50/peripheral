package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const perplexityURL = "https://api.perplexity.ai/chat/completions"

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PerplexityRequest represents the request structure for Perplexity API
type PerplexityRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	MaxTokens        int       `json:"max_tokens"`
	Temperature      float64   `json:"temperature"`
	TopP             float64   `json:"top_p"`
	TopK             int       `json:"top_k"`
	Stream           bool      `json:"stream"`
	PresencePenalty  float64   `json:"presence_penalty"`
	FrequencyPenalty float64   `json:"frequency_penalty"`
}

// PerplexityResponse represents the response structure from Perplexity API
type PerplexityResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func queryPerplexity(query string) (string, error) {
	// Create the request payload
	reqData := PerplexityRequest{ //testing sonar
		Model: "sonar",
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a financial analyst expert. Provide concise, well-structured summaries of stock movements. Focus on the most likely causes including news, events, earnings, analyst ratings, broader market trends, or sector-specific factors. Use bullet points for clarity when appropriate. Limit your response to 290 tokens.",
			},
			{
				Role:    "user",
				Content: query,
			},
		},
		MaxTokens:        300,
		Temperature:      0.2,
		TopP:             0.9,
		TopK:             0,
		Stream:           false,
		PresencePenalty:  0,
		FrequencyPenalty: 1,
	}

	// Marshal the request data to JSON
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("error marshaling JSON: %w", err)
	}

	// Print the JSON payload for debugging
	fmt.Printf("\n\n\nRequest payload: %s\n\n\n", string(jsonData))

	// Create the HTTP request
	req, err := http.NewRequest("POST", perplexityURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	// Add headers
	// In a real application, this should be stored in environment variables or a secure config
	req.Header.Add("Authorization", "Bearer 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")
	req.Header.Add("Content-Type", "application/json")

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	// Parse the response
	var perplexityResponse PerplexityResponse
	if err := json.Unmarshal(body, &perplexityResponse); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}
	fmt.Printf("Perplexity response: %+v\n \n\n\n\n", perplexityResponse)
	// Check if we have a valid response
	if len(perplexityResponse.Choices) == 0 {
		return "", fmt.Errorf("no response choices received from Perplexity API")
	}

	// Return the content from the first choice
	return perplexityResponse.Choices[0].Message.Content, nil
}

// Updated function with timestamp parameter
func QueryPerplexityWithDate(ticker string, timestamp int64, price float64) (string, error) {
	// Convert timestamp to a readable date format
	date := time.Unix(timestamp/1000, 0).Format("January 2, 2006")

	// Format the query with the provided information including the formatted date
	query := fmt.Sprintf("Provide a comprehensive analysis of %s stock performance on %s with price $%.2f. Include likely causes for price movements, relevant news, market trends, and technical analysis.", ticker, date, price)

	// Create the request payload
	reqData := PerplexityRequest{
		Model: "sonar",
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a financial analyst expert. Provide comprehensive, well-structured analysis of stock movements. Your analysis should include the following sections: 1) Summary of Price Movement, 2) Key News and Events, 3) Market Context, 4) Technical Analysis, and 5) Outlook. Focus on the specific date provided and explain why the stock moved on that day. Use bullet points where appropriate for clarity. Limit your response to 500 tokens.",
			},
			{
				Role:    "user",
				Content: query,
			},
		},
		MaxTokens:        500,
		Temperature:      0.2,
		TopP:             0.9,
		TopK:             0,
		Stream:           false,
		PresencePenalty:  0,
		FrequencyPenalty: 1,
	}

	// Marshal the request data to JSON
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("error marshaling JSON: %w", err)
	}

	// Print the JSON payload for debugging
	fmt.Printf("\n\n\nRequest payload: %s\n\n\n", string(jsonData))

	// Create the HTTP request
	req, err := http.NewRequest("POST", perplexityURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	// Add headers
	// In a real application, this should be stored in environment variables or a secure config
	req.Header.Add("Authorization", "Bearer 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")
	req.Header.Add("Content-Type", "application/json")

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	// Parse the response
	var perplexityResponse PerplexityResponse
	if err := json.Unmarshal(body, &perplexityResponse); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}
	fmt.Printf("Perplexity response: %+v\n \n\n\n\n", perplexityResponse)
	// Check if we have a valid response
	if len(perplexityResponse.Choices) == 0 {
		return "", fmt.Errorf("no response choices received from Perplexity API")
	}

	// Return the content from the first choice
	return perplexityResponse.Choices[0].Message.Content, nil
}
