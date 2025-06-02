package agent

import (
	"backend/internal/data"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"google.golang.org/genai"
)

const geminiWebSearchModel = "gemini-2.5-flash-preview-05-20"
const grokModel = "grok-3-mini-latest"

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
			if part.Thought {
				continue
			}
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

type GrokMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type GrokSources struct {
	Type     string   `json:"type"`
	XHandles []string `json:"x_handles,omitempty"`
}
type GrokSearchParameters struct {
	Mode            string        `json:"mode"`
	ReturnCitations bool          `json:"return_citations,omitempty"`
	FromDate        string        `json:"from_date,omitempty"`
	ToDate          string        `json:"to_date,omitempty"`
	Sources         []GrokSources `json:"sources,omitempty"`
}
type GrokChatCompletionsRequest struct {
	Messages         []GrokMessage        `json:"messages"`
	Model            string               `json:"model"`
	SearchParameters GrokSearchParameters `json:"search_parameters,omitempty"`
}
type GrokChatCompletionsResponse struct {
	Choices []struct {
		Message GrokMessage `json:"message"`
	} `json:"choices"`
}
type TwitterSearchArgs struct {
	Prompt   string   `json:"prompt"`
	Handles  []string `json:"handles,omitempty"`
	FromDate string   `json:"from_date,omitempty"`
	ToDate   string   `json:"to_date,omitempty"`
}

type TwitterSearchResult struct {
	ResultText string   `json:"result_text"`
	Citations  []string `json:"citations,omitempty"`
}

func RunTwitterSearch(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args TwitterSearchArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error unmarshalling args: %w", err)
	}
	systemPrompt, err := _getTwitterSearchSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("error getting twitter search system prompt: %w", err)
	}
	model := grokModel
	searchParameters := GrokSearchParameters{
		Mode:            "on",
		ReturnCitations: true,
	}

	// Only add sources if handles are specifically provided
	// This might be causing the 422 error if empty
	if len(args.Handles) > 0 {
		searchParameters.Sources = []GrokSources{
			{
				Type:     "x",
				XHandles: args.Handles,
			},
		}
	}

	// Add date filters if provided
	if args.FromDate != "" {
		searchParameters.FromDate = args.FromDate
	}
	if args.ToDate != "" {
		searchParameters.ToDate = args.ToDate
	}

	grokRequestBody := GrokChatCompletionsRequest{
		Messages: []GrokMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: args.Prompt,
			},
		},
		Model:            model,
		SearchParameters: searchParameters,
	}
	bodyBytes, _ := json.Marshal(grokRequestBody)
	httpReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://api.x.ai/v1/chat/completions",
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+conn.XAPIKey)
	cli := &http.Client{Timeout: 60 * time.Second}
	resp, err := cli.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Read the response body to get error details
		errorBodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			errorBodyBytes = []byte("failed to read error response")
		}
		requestBodyBytes, _ := json.Marshal(grokRequestBody)
		fmt.Printf("Request body: %s\n", string(requestBodyBytes))
		fmt.Printf("Response status: %s\n", resp.Status)
		fmt.Printf("Response body: %s\n", string(errorBodyBytes))
		return nil, fmt.Errorf("grok: non-200 response: %s", resp.Status)
	}
	var output GrokChatCompletionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	if len(output.Choices) == 0 {
		return nil, errors.New("grok: no choices in response")
	}
	return output.Choices[0].Message.Content, nil

}

func _getTwitterSearchSystemPrompt() (string, error) {
	prompt, err := getSystemInstruction("twitterSearchPrompt")
	if err != nil {
		return "", fmt.Errorf("error getting twitter search system prompt: %w", err)
	}
	return prompt, nil

}

type TwitterAPITweet struct {
	URL       string `json:"url"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
	Author    struct {
		UserName string `json:"userName"`
		Name     string `json:"name"`
	} `json:"author"`
}

type TwitterAPIResponse struct {
	Tweets      []TwitterAPITweet `json:"tweets"`
	HasNextPage bool              `json:"has_next_page"`
	NextCursor  string            `json:"next_cursor"`
}

type LatestTweetsResult struct {
	URL       string `json:"url"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
	Name      string `json:"name"`
}

func GetLatestTweets(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args TwitterSearchArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error unmarshalling args: %w", err)
	}
	handles := args.Handles
	if len(handles) == 0 {
		return nil, errors.New("no handles provided")
	}

	// Get API key - assuming you have a method to get Twitter API key
	apiKey := conn.TwitterAPIioKey // You may need to add this field to your Conn struct
	if apiKey == "" {
		return nil, errors.New("twitter api io key not found")
	}

	// Construct the search query: "from:user1 OR from:user2 OR ..."
	query := ""
	for i, handle := range handles {
		// Remove @ if present
		if len(handle) > 0 && handle[0] == '@' {
			handle = handle[1:]
		}

		if i > 0 {
			query += " OR "
		}
		query += "from:" + handle
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://api.twitterapi.io/twitter/tweet/advanced_search",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set query parameters
	q := req.URL.Query()
	q.Add("query", query)
	q.Add("queryType", "Latest")
	q.Add("cursor", "") // First page
	req.URL.RawQuery = q.Encode()

	// Set headers
	req.Header.Set("X-API-Key", apiKey)

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var apiResponse TwitterAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	// Load EST timezone
	estLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("error loading EST timezone: %w", err)
	}

	// Convert to our result format
	results := make([]LatestTweetsResult, 0, len(apiResponse.Tweets))
	for _, tweet := range apiResponse.Tweets {
		// Parse and format the timestamp
		formattedTime := tweet.CreatedAt // Default fallback

		// Try to parse the timestamp and convert to EST
		if parsedTime, parseErr := time.Parse(time.RFC3339, tweet.CreatedAt); parseErr == nil {
			// Convert to EST and format nicely
			estTime := parsedTime.In(estLocation)
			formattedTime = estTime.Format("01/02/2006 3:04 PM MST")
		} else if parsedTime, parseErr := time.Parse("2006-01-02T15:04:05.000Z", tweet.CreatedAt); parseErr == nil {
			// Try alternative format
			estTime := parsedTime.In(estLocation)
			formattedTime = estTime.Format("01/02/2006 3:04 PM MST")
		} else if parsedTime, parseErr := time.Parse("Mon Jan 02 15:04:05 +0000 2006", tweet.CreatedAt); parseErr == nil {
			// Try Twitter's classic format
			estTime := parsedTime.In(estLocation)
			formattedTime = estTime.Format("01/02/2006 3:04 PM MST")
		}

		results = append(results, LatestTweetsResult{
			URL:       tweet.URL,
			Text:      tweet.Text,
			CreatedAt: formattedTime,
			Name:      tweet.Author.Name,
		})
	}

	return results, nil
}
