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
	"sort"

	"backend/internal/services/socket"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"google.golang.org/genai"
)

const geminiWebSearchModel = "gemini-2.5-flash"
const grokModel = "grok-3-mini-latest"

type WebSearchArgs struct {
	Query string `json:"query"`
}
type WebSearchResult struct {
	ResultText string   `json:"result"`
	Citations  []string `json:"citations,omitempty"`
}

// RunWebSearch performs a web search using the Gemini API
func RunWebSearch(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args WebSearchArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error unmarshalling args: %w", err)
	}
	socket.SendAgentStatusUpdate(userID, "WebSearch", args.Query)
	systemPrompt, err := getSystemInstruction("webSearchPrompt")
	if err != nil {
		return nil, fmt.Errorf("error getting search system instruction: %w", err)
	}
	return _openaiWebSearch(conn, systemPrompt, args.Query)
}

func _openaiWebSearch(conn *data.Conn, systemPrompt string, prompt string) (interface{}, error) {
	apiKey := conn.OpenAIKey
	client := openai.NewClient(option.WithAPIKey(apiKey))
	res, err := client.Responses.New(context.Background(), responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
		Model:        "gpt-4.1",
		Instructions: openai.String(systemPrompt),
		Tools: []responses.ToolUnionParam{
			{
				OfWebSearchPreview: &responses.WebSearchToolParam{
					Type:              "web_search_preview",
					SearchContextSize: "medium",
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error creating response: %w", err)
	}

	return WebSearchResult{
		ResultText: res.OutputText(),
		Citations:  nil,
	}, nil
}

// RunWebSearch performs a web search using the Gemini API
func RunWebSearchGemini(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
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
	if candidate != nil && candidate.Content != nil && candidate.Content.Parts != nil {
		for _, part := range candidate.Content.Parts {
			if part != nil {
				if part.Thought {
					continue
				}
				if part.Text != "" {
					resultText = part.Text
					break
				}
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

// TwitterCache represents the cached data for a Twitter handle
type TwitterCache struct {
	Handle        string               `json:"handle"`
	LastTweetTime time.Time            `json:"last_tweet_time"`
	UpdatedAt     time.Time            `json:"updated_at"`
	Tweets        []LatestTweetsResult `json:"tweets"`
}

// TwitterCacheData represents data to be cached for a handle
type TwitterCacheData struct {
	LatestTime time.Time
	Tweets     []LatestTweetsResult
}

// normalizeHandle removes @ prefix if present
func normalizeHandle(handle string) string {
	if len(handle) > 0 && handle[0] == '@' {
		return handle[1:]
	}
	return handle
}

// getCacheKey returns the Redis cache key for a handle
func getCacheKey(handle string) string {
	return fmt.Sprintf("twitter_cache:%s", normalizeHandle(handle))
}

// getCachedTwitterData retrieves cached data for Twitter handles from Redis
func getCachedTwitterData(conn *data.Conn, handles []string) (map[string]time.Time, []LatestTweetsResult, error) {
	ctx := context.Background()
	cache := make(map[string]time.Time)
	var allCachedTweets []LatestTweetsResult

	for _, handle := range handles {
		normalizedHandle := normalizeHandle(handle)
		cacheKey := getCacheKey(normalizedHandle)

		cachedData, err := conn.Cache.Get(ctx, cacheKey).Result()
		if err != nil {
			// If key doesn't exist, that's fine - we'll fetch from beginning
			continue
		}

		var twitterCache TwitterCache
		if err := json.Unmarshal([]byte(cachedData), &twitterCache); err != nil {
			// If we can't unmarshal, skip this cache entry
			continue
		}

		cache[normalizedHandle] = twitterCache.LastTweetTime
		allCachedTweets = append(allCachedTweets, twitterCache.Tweets...)
	}

	return cache, allCachedTweets, nil
}

// getExistingCacheData retrieves existing cache data for a single handle
func getExistingCacheData(conn *data.Conn, handle string) TwitterCache {
	ctx := context.Background()
	cacheKey := getCacheKey(handle)

	cachedData, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		return TwitterCache{Handle: handle}
	}

	var twitterCache TwitterCache
	if err := json.Unmarshal([]byte(cachedData), &twitterCache); err != nil {
		return TwitterCache{Handle: handle}
	}

	return twitterCache
}

// updateTwitterCache updates the cached data for Twitter handles in Redis
func updateTwitterCache(conn *data.Conn, handleToData map[string]TwitterCacheData) error {
	ctx := context.Background()

	for handle, data := range handleToData {
		// Get existing cache data
		existingCache := getExistingCacheData(conn, handle)

		// Merge new tweets with existing tweets, avoiding duplicates
		mergedTweets := mergeTweets(existingCache.Tweets, data.Tweets)

		// Limit cache size to last 100 tweets per handle to prevent excessive memory usage
		if len(mergedTweets) > 100 {
			mergedTweets = mergedTweets[:100]
		}

		twitterCache := TwitterCache{
			Handle:        handle,
			LastTweetTime: data.LatestTime,
			UpdatedAt:     time.Now(),
			Tweets:        mergedTweets,
		}

		cacheData, err := json.Marshal(twitterCache)
		if err != nil {
			return fmt.Errorf("error marshaling cache data for %s: %w", handle, err)
		}

		// Set cache with 7 days expiration
		cacheKey := getCacheKey(handle)
		if err := conn.Cache.Set(ctx, cacheKey, cacheData, 7*24*time.Hour).Err(); err != nil {
			return fmt.Errorf("error setting cache for %s: %w", handle, err)
		}
	}

	return nil
}

// mergeTweets merges existing and new tweets, avoiding duplicates and sorting by timestamp
func mergeTweets(existing, newTweets []LatestTweetsResult) []LatestTweetsResult {
	// Create a map to track existing tweets by URL to avoid duplicates
	tweetMap := make(map[string]LatestTweetsResult)

	// Add existing tweets to the map
	for _, tweet := range existing {
		tweetMap[tweet.URL] = tweet
	}

	// Add new tweets, overwriting if URL already exists (newer data)
	for _, tweet := range newTweets {
		tweetMap[tweet.URL] = tweet
	}

	// Convert back to slice
	var merged []LatestTweetsResult
	for _, tweet := range tweetMap {
		merged = append(merged, tweet)
	}

	// Sort by created time (descending - newest first)
	sort.Slice(merged, func(i, j int) bool {
		timeI := parseTwitterTimestamp(merged[i].CreatedAt)
		timeJ := parseTwitterTimestamp(merged[j].CreatedAt)
		return timeI.After(timeJ)
	})

	return merged
}

// parseTwitterTimestamp parses the formatted timestamp back to time.Time for sorting
func parseTwitterTimestamp(timestamp string) time.Time {
	formats := []string{
		"01/02/2006 3:04 PM MST",         // Our formatted timestamp
		time.RFC3339,                     // Standard RFC3339
		"2006-01-02T15:04:05.000Z",       // Alternative format
		"Mon Jan 02 15:04:05 +0000 2006", // Twitter's classic format
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, timestamp); err == nil {
			return parsed
		}
	}

	// Return zero time if all parsing fails
	return time.Time{}
}

// formatTweetTimestamp formats a time.Time to EST string format
func formatTweetTimestamp(t time.Time, estLocation *time.Location) string {
	return t.In(estLocation).Format("01/02/2006 3:04 PM MST")
}

// parseTweetTimestamp parses various timestamp formats from Twitter API
func parseTweetTimestamp(timestamp string, estLocation *time.Location) (time.Time, string) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"Mon Jan 02 15:04:05 +0000 2006",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, timestamp); err == nil {
			formatted := formatTweetTimestamp(parsed, estLocation)
			return parsed, formatted
		}
	}

	// Return original timestamp if parsing fails
	return time.Time{}, timestamp
}

// ClearTwitterCache clears cached data for specific Twitter handles
type ClearTwitterCacheArgs struct {
	Handles []string `json:"handles,omitempty"` // If empty, clears all Twitter cache
}

func ClearTwitterCache(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args ClearTwitterCacheArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error unmarshalling args: %w", err)
	}

	ctx := context.Background()
	clearedCount := 0

	if len(args.Handles) == 0 {
		// Clear all Twitter cache entries
		pattern := "twitter_cache:*"
		keys, err := conn.Cache.Keys(ctx, pattern).Result()
		if err != nil {
			return nil, fmt.Errorf("error getting cache keys: %w", err)
		}

		if len(keys) > 0 {
			deletedCount, err := conn.Cache.Del(ctx, keys...).Result()
			if err != nil {
				return nil, fmt.Errorf("error deleting cache keys: %w", err)
			}
			clearedCount = int(deletedCount)
		}
	} else {
		// Clear cache for specific handles
		for _, handle := range args.Handles {
			cacheKey := getCacheKey(handle)
			deletedCount, err := conn.Cache.Del(ctx, cacheKey).Result()
			if err != nil {
				return nil, fmt.Errorf("error deleting cache for %s: %w", handle, err)
			}
			clearedCount += int(deletedCount)
		}
	}

	return map[string]interface{}{
		"cleared_count": clearedCount,
		"message":       fmt.Sprintf("Cleared %d Twitter cache entries", clearedCount),
	}, nil
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

	// Get API key
	apiKey := conn.TwitterAPIioKey
	if apiKey == "" {
		return nil, errors.New("twitter api io key not found")
	}

	// Get cached data for these handles
	cachedData, allCachedTweets, err := getCachedTwitterData(conn, handles)
	if err != nil {
		// Log the error but continue without cache
		fmt.Printf("Warning: Could not retrieve Twitter cache: %v\n", err)
	}

	// Construct the search query: "from:user1 OR from:user2 OR ..."
	query := ""
	var sinceTime time.Time

	for i, handle := range handles {
		normalizedHandle := normalizeHandle(handle)

		if i > 0 {
			query += " OR "
		}
		query += "from:" + normalizedHandle

		// Find the earliest "since" time among all cached handles
		if cachedTime, exists := cachedData[normalizedHandle]; exists {
			if sinceTime.IsZero() || cachedTime.Before(sinceTime) {
				sinceTime = cachedTime
			}
		}
	}

	// Add since parameter if we have cached data
	// Only add since if it's reasonably recent (not older than 30 days)
	if !sinceTime.IsZero() && time.Since(sinceTime) < 30*24*time.Hour {
		// Format the since time for Twitter API (RFC3339 format)
		sinceParam := sinceTime.Format("2006-01-02T15:04:05Z")
		query += fmt.Sprintf(" since:%s", sinceParam)
	}
	fmt.Println("query", query)
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

	// Track the latest tweet time and tweets for each handle for caching
	handleToData := make(map[string]TwitterCacheData)

	// Convert API response to our result format
	var newTweets []LatestTweetsResult
	for _, tweet := range apiResponse.Tweets {
		// Parse and format the timestamp using helper function
		parsedTime, formattedTime := parseTweetTimestamp(tweet.CreatedAt, estLocation)

		// Extract handle from the author username
		handle := normalizeHandle(tweet.Author.UserName)

		// Create tweet result
		tweetResult := LatestTweetsResult{
			URL:       tweet.URL,
			Text:      tweet.Text,
			CreatedAt: formattedTime,
			Name:      tweet.Author.Name,
		}

		newTweets = append(newTweets, tweetResult)

		// Update the latest time and tweets for this handle
		if !parsedTime.IsZero() {
			if existingData, exists := handleToData[handle]; !exists || parsedTime.After(existingData.LatestTime) {
				if !exists {
					handleToData[handle] = TwitterCacheData{
						LatestTime: parsedTime,
						Tweets:     []LatestTweetsResult{tweetResult},
					}
				} else {
					existingData.LatestTime = parsedTime
					existingData.Tweets = append(existingData.Tweets, tweetResult)
					handleToData[handle] = existingData
				}
			} else if exists {
				// Add tweet to existing data even if it's not the latest
				existingData.Tweets = append(existingData.Tweets, tweetResult)
				handleToData[handle] = existingData
			}
		}
	}

	// Merge new tweets with cached tweets
	allTweets := mergeTweets(allCachedTweets, newTweets)

	// Update cache with new data if we have any
	if len(handleToData) > 0 {
		// Preserve existing cache data for handles we didn't get new tweets for
		for handle, cachedTime := range cachedData {
			if _, exists := handleToData[handle]; !exists {
				// Get existing tweets for this handle from cache
				existingCache := getExistingCacheData(conn, handle)
				handleToData[handle] = TwitterCacheData{
					LatestTime: cachedTime,
					Tweets:     existingCache.Tweets,
				}
			}
		}

		if err := updateTwitterCache(conn, handleToData); err != nil {
			// Log the error but don't fail the request
			fmt.Printf("Warning: Could not update Twitter cache: %v\n", err)
		}
	}

	return allTweets, nil
}

// GetCachedTweets retrieves cached tweets for specified handles without making API calls
type GetCachedTweetsArgs struct {
	Handles []string `json:"handles"`
}

func GetCachedTweets(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetCachedTweetsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error unmarshalling args: %w", err)
	}

	if len(args.Handles) == 0 {
		return nil, errors.New("no handles provided")
	}

	// Get cached data for these handles
	_, allCachedTweets, err := getCachedTwitterData(conn, args.Handles)
	if err != nil {
		return nil, fmt.Errorf("error retrieving cached tweets: %w", err)
	}

	// Sort tweets by timestamp (descending - newest first)
	sort.Slice(allCachedTweets, func(i, j int) bool {
		timeI := parseTwitterTimestamp(allCachedTweets[i].CreatedAt)
		timeJ := parseTwitterTimestamp(allCachedTweets[j].CreatedAt)
		return timeI.After(timeJ)
	})

	return allCachedTweets, nil
}
