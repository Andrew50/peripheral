package server

import (
	"backend/internal/data"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"time"

	"backend/internal/app/agent"
	"backend/internal/services/plotly"
	"backend/internal/services/socket"
	"backend/internal/services/twitter"

	"github.com/go-redis/redis/v8"
	"google.golang.org/genai"
)

// TwitterWebhookPayload represents only the fields we need from Twitter webhook
type TwitterWebhookPayload struct {
	Tweets    []Tweet `json:"tweets,omitempty"`
	EventType string  `json:"event_type,omitempty"`
	RuleTag   string  `json:"rule_tag,omitempty"`
}

// Tweet represents only the fields we need from each tweet
type Tweet struct {
	URL       string `json:"url,omitempty"`
	Text      string `json:"text,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	Author    Author `json:"author,omitempty"`
	ID        string `json:"id,omitempty"`
}

// Author represents only the username field we need
type Author struct {
	Username string `json:"userName,omitempty"`
}

// TwitterAPIUpdateRuleRequest represents the request body for updating a Twitter API rule
type TwitterAPIUpdateWebhookRequest struct {
	RuleID          string `json:"rule_id"`
	Tag             string `json:"tag"`
	Value           string `json:"value"`
	IntervalSeconds int    `json:"interval_seconds"`
	IsEffect        int    `json:"is_effect"`
}

var twitterWebhookRuleset = "from:trad_fin OR from:tier10k OR from:TreeNewsFeed within_time:10m -filter:replies"

//var replyWebhookRuleset = "within_time:20m -filter:replies -from:TheShortBear from:amitisinvesting OR from:StockMKTNewz OR from:EliteOptions2 OR from:fundstrat OR from:TrendSpider OR from:GURGAVIN OR from:unusual_whales"

func updateTwitterNewsWebhookPollingFrequency(conn *data.Conn, intervalSeconds int, webhookStatus bool) error {
	isEffect := 0
	if webhookStatus {
		isEffect = 1
	}
	err := updateTwitterAPIRule(conn, TwitterAPIUpdateWebhookRequest{
		RuleID:          "6d13a825822c4fe1990857f154b1cd6b",
		Tag:             "Main Twitter",
		Value:           twitterWebhookRuleset,
		IntervalSeconds: intervalSeconds,
		IsEffect:        isEffect,
	})
	if err != nil {
		log.Printf("Error updating Twitter webhook polling frequency: %v", err)
		return err
	}
	return nil
}
func verifyTwitterWebhookConfiguration(conn *data.Conn) error {
	// Load New York timezone
	nyTimezone, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("Error loading New York timezone: %v", err)
		return fmt.Errorf("failed to load timezone: %w", err)
	}
	dayOfTheWeek := time.Now().In(nyTimezone).Weekday()
	if dayOfTheWeek == time.Saturday || dayOfTheWeek == time.Sunday {
		return updateTwitterNewsWebhookPollingFrequency(conn, 300, true) // on weekends poll every 5 mins
	}
	// Get current time in New York
	currentTime := time.Now().In(nyTimezone)
	currentHour := currentTime.Hour()

	// Check if it's between 6 AM (6) and 9 PM (21) - market hours
	if currentHour >= 6 && currentHour < 21 {

		return updateTwitterNewsWebhookPollingFrequency(conn, 30, true)
	} else {
		return updateTwitterNewsWebhookPollingFrequency(conn, 30, false)
	}
}
func updateTwitterAPIRule(conn *data.Conn, request TwitterAPIUpdateWebhookRequest) error {
	// Marshal the request body
	jsonData, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshaling Twitter API request: %v", err)
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.twitterapi.io/oapi/tweet_filter/update_rule", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating Twitter API request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", conn.TwitterAPIioKey)

	// Make the request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := data.DoWithRetry(client, req)
	if err != nil {
		log.Printf("Error making Twitter API request: %v", err)
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading Twitter API response: %v", err)
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("Twitter API returned non-200 status: %d, body: %s", resp.StatusCode, string(responseBody))
		return fmt.Errorf("twitter api returned status %d: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// twitterWebhookHandler is the HTTP handler wrapper
func twitterWebhookHandler(conn *data.Conn) http.HandlerFunc {
	return HandleTwitterWebhook(conn)
}

// HandleTwitterWebhook processes incoming Twitter webhook events
func HandleTwitterWebhook(conn *data.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Verify the X-API-Key header for request authenticity
		/*twitterAPIKey := r.Header.Get("X-API-Key")
		if twitterAPIKey == "" {
			log.Printf("Twitter webhook request missing X-API-Key header")
			http.Error(w, "Missing API key", http.StatusUnauthorized)
			return
		}

		if twitterAPIKey != conn.TwitterAPIioKey {
			log.Printf("Twitter webhook request with invalid API key: %s", twitterAPIKey)
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}*/

		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading Twitter webhook body: %v", err)
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Parse the JSON payload
		var payload TwitterWebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("Error parsing Twitter webhook JSON: %v", err)
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		// Handle test webhook
		if payload.EventType == "test_webhook_url" {
			log.Printf("Received test webhook event")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "success",
				"message": "Test webhook received",
			}); err != nil {
				log.Printf("Warning: failed to encode JSON response: %v", err)
			}
			return
		}

		// Process each tweet
		var extractedTweets []twitter.ExtractedTweetData
		for _, tweet := range payload.Tweets {
			extracted := twitter.ExtractedTweetData{
				URL:       tweet.URL,
				Text:      tweet.Text,
				CreatedAt: tweet.CreatedAt,
				Username:  tweet.Author.Username,
				ID:        tweet.ID,
			}
			extractedTweets = append(extractedTweets, extracted)

		}
		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "success",
		}); err != nil {
			log.Printf("Warning: failed to encode JSON response: %v", err)
		}
		// Queue the extracted data for background processing
		err = processTwitterWebhookEvent(conn, payload.RuleTag, extractedTweets)
		if err != nil {
			log.Printf("Error queueing Twitter webhook event: %v", err)
			http.Error(w, "Error processing webhook", http.StatusInternalServerError)
			return
		}
	}
}

// processTwitterWebhookEvent processes the extracted tweet data
func processTwitterWebhookEvent(conn *data.Conn, ruleTag string, tweets []twitter.ExtractedTweetData) error {
	fmt.Println("queueTwitterWebhookEvent ruletag", ruleTag, "extractedTweets", tweets)
	fmt.Println("tweets", tweets)
	fmt.Println("ruleTag", ruleTag)
	for _, tweet := range tweets {
		if ruleTag == "Main Twitter" {
			processTweet(conn, tweet)
		} else if ruleTag == "Reply Webhook" {
			if err := twitter.HandleTweetForReply(conn, tweet); err != nil {
				log.Printf("Warning: failed to handle tweet for reply: %v", err)
			}
		} else if ruleTag == "Ask Peripheral" {
			fmt.Println("Processing @Ask Peripheral tweet", tweet)
			twitter.GenerateAskPeripheralTweet(conn, tweet)
		}
	}
	return nil
}

func processTweet(conn *data.Conn, tweet twitter.ExtractedTweetData) {

	seen := determineIfAlreadySeenTweet(conn, tweet)
	//seen = false
	fmt.Println("seen", seen)
	if seen {
		storeTweet(conn, tweet)
		return
	}
	// Extract ticker symbols from the tweet text
	tickers := extractTickersFromTweet(tweet.Text)

	socket.SendAlertToAllUsers(socket.AlertMessage{
		AlertID:    1,
		Timestamp:  time.Now().Unix() * 1000,
		SecurityID: 1,
		Message:    tweet.Text,
		Channel:    "alert",
		Type:       "news",
		Tickers:    tickers,
	})
	storeTweet(conn, tweet)

	peripheralContentToTweet, err := CreatePeripheralTweetFromNews(conn, tweet)
	if err != nil {
		log.Printf("Error creating peripheral tweet: %v", err)
		return
	}
	fmt.Println("Peripheral tweet", peripheralContentToTweet)
	twitter.SendTweetToPeripheralTwitterAccount(conn, peripheralContentToTweet)

}

type AgentPeripheralTweet struct {
	Text string      `json:"text" jsonschema:"required"`
	Plot interface{} `json:"plot" jsonschema:"required"`
}

func CreatePeripheralTweetFromNews(conn *data.Conn, tweet twitter.ExtractedTweetData) (twitter.FormattedPeripheralTweet, error) { // to implement don't forget

	prompt := tweet.Text
	fmt.Println("Starting Creating a Periphearl tweet from prompt", prompt)

	agentResult, err := agent.RunGeneralAgent[AgentPeripheralTweet](conn, 0, "TweetBreakingHeadlineSystemPrompt", "TweetCraftFinalSystemPrompt", prompt, "o4-mini", "medium")
	if err != nil {
		return twitter.FormattedPeripheralTweet{}, fmt.Errorf("error running general agent for tweet generation: %w", err)
	}
	/*agentResult := AgentPeripheralTweet{
		Text: prompt,
		Plot: nil,
	}

	// Override with sample plot data for testing
	samplePlot := map[string]interface{}{
		"chart_type": "bar",
		"data": []map[string]interface{}{
			{
				"x":    []string{"SPY", "QQQ", "DIA"},
				"y":    []float64{-0.2, -0.24, -0.66},
				"name": "% Change vs Prev Close",
				"type": "bar",
			},
		},
		"title":       "Intraday Performance",
		"titleTicker": "COIN",
		"layout": map[string]interface{}{
			"xaxis": map[string]interface{}{
				"title": "Ticker",
			},
			"yaxis": map[string]interface{}{
				"title": "% Change vs Prev Close",
			},
		},
	}
	agentResult.Plot = samplePlot*/

	var base64PNG string
	base64PNG, err = plotly.RenderTwitterPlotToBase64(conn, agentResult.Plot, false)
	if err != nil {
		log.Printf("🚨 ERROR rendering Twitter plot: %v", err)
	}
	formattedPeripheralTweet := twitter.FormattedPeripheralTweet{
		Text:  agentResult.Text,
		Image: base64PNG,
	}
	return formattedPeripheralTweet, nil
}

type alreadySeenTweetGeminiResponseStruct struct {
	Duplicate bool `json:"duplicate"`
}

func duplicateDetectionSchema() *genai.Schema {
	return &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"duplicate"},
		Properties: map[string]*genai.Schema{
			"duplicate": {
				Type:        genai.TypeBoolean,
				Description: "Whether the tweet is a duplicate of any recent tweets",
			},
		},
		Title:       "DuplicateDetectionResponse",
		Description: "Response indicating if a tweet is a duplicate",
	}
}
func determineIfAlreadySeenTweet(conn *data.Conn, tweet twitter.ExtractedTweetData) bool {
	// TODO: Implement LLM-based duplicate detection
	// For now, return false to allow all tweets through
	recentTweets := getStoredTweets(conn)
	recentTweetsString := strings.Join(recentTweets, "\n")
	geminiClient := conn.GeminiClient
	if geminiClient == nil {
		log.Printf("\nGemini client not initialized!!!")
		return false
	}

	prompt := fmt.Sprintf("Already Seen Tweets: %s\nNew Tweet: %s", recentTweetsString, tweet.Text)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: "You are an assistant tasked to determine if a tweet's content has already been seen. You will be given a list of tweets and a new tweet. Determine whether the content of the tweet or anything related to the tweet has already been seen."},
			},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   duplicateDetectionSchema(),
	}
	model := "gemini-2.5-flash-lite-preview-06-17"
	geminiResponse, err := geminiClient.Models.GenerateContent(context.Background(), model, genai.Text(prompt), config)
	if err != nil {
		log.Printf("Error generating content: %v", err)
		return false
	}
	candidate := geminiResponse.Candidates[0]
	var sb strings.Builder
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
	var response alreadySeenTweetGeminiResponseStruct
	if err := json.Unmarshal([]byte(resultText), &response); err != nil {
		log.Printf("Error unmarshalling Gemini response: %v", err)
		return false
	}
	return response.Duplicate
}

func extractTickersFromTweet(tweet string) []string {
	// Regex pattern to match $ followed by 1-6 alphanumeric characters, stopping at space or end
	pattern := `\$([A-Za-z0-9]{1,6})(?:\s|$)`
	re := regexp.MustCompile(pattern)

	matches := re.FindAllStringSubmatch(tweet, -1)
	var tickers []string

	for _, match := range matches {
		if len(match) > 1 {
			// Convert to uppercase as ticker symbols are typically uppercase
			ticker := strings.ToUpper(match[1])
			tickers = append(tickers, ticker)
		}
	}

	return tickers
}

func getStoredTweets(conn *data.Conn) []string {
	cutoff := time.Now().Add(-18 * time.Hour).Unix()

	// Get tweets from the last 18 hours using ZRangeByScore
	results, err := conn.Cache.ZRangeByScore(context.Background(), "twitterTweets", &redis.ZRangeBy{
		Min: strconv.FormatInt(cutoff, 10),
		Max: "+inf",
	}).Result()

	if err != nil {
		log.Printf("Error retrieving stored tweets: %v", err)
		return []string{}
	}

	return results
}
func storeTweet(conn *data.Conn, tweet twitter.ExtractedTweetData) {
	timestamp := time.Now().Unix()
	conn.Cache.ZAdd(context.Background(), "twitterTweets", &redis.Z{
		Score:  float64(timestamp),
		Member: tweet.Text,
	})
	// Cleanup old tweets (optional, can be done periodically)
	cutoff := time.Now().Add(-18 * time.Hour).Unix()
	conn.Cache.ZRemRangeByScore(context.Background(), "twitterTweets", "-inf", strconv.FormatInt(cutoff, 10))

	/* query := `INSERT INTO news_tweets (tweet_text, created_at, url, username) VALUES ($1, $2, $3, $4)`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := conn.DB.Exec(ctx, query, tweet.Text, tweet.CreatedAt, tweet.URL, tweet.Username)
	if err != nil {
		log.Printf("Error storing tweet: %v", err)
	}*/
}
