package server

import (
	"backend/internal/data"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"time"

	"backend/internal/app/agent"
	"backend/internal/app/helpers"
	"backend/internal/services/plotly"
	"backend/internal/services/socket"

	"github.com/dghubble/oauth1"
	"github.com/go-redis/redis/v8"
	"google.golang.org/genai"
)

// TwitterWebhookPayload represents only the fields we need from Twitter webhook
type TwitterWebhookPayload struct {
	Tweets    []Tweet `json:"tweets,omitempty"`
	EventType string  `json:"event_type,omitempty"`
}

// Tweet represents only the fields we need from each tweet
type Tweet struct {
	URL       string `json:"url,omitempty"`
	Text      string `json:"text,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	Author    Author `json:"author,omitempty"`
}

// Author represents only the username field we need
type Author struct {
	Username string `json:"userName,omitempty"`
}

// ExtractedTweetData represents the clean data we extract for processing
type ExtractedTweetData struct {
	URL       string `json:"url,omitempty"`
	Text      string `json:"text,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	Username  string `json:"username,omitempty"`
}

// TwitterAPIUpdateRuleRequest represents the request body for updating a Twitter API rule
type TwitterAPIUpdateWebhookRequest struct {
	RuleID          string `json:"rule_id"`
	Tag             string `json:"tag"`
	Value           string `json:"value"`
	IntervalSeconds int    `json:"interval_seconds"`
	IsEffect        int    `json:"is_effect"`
}

var twitterWebhookRuleset = "from:tradfi_noticias OR from:tier10k OR from:TreeNewsFeed within_time:10m -filter:replies"

func turnOffTwitterNewsWebhook(conn *data.Conn) error {
	err := updateTwitterAPIRule(conn, TwitterAPIUpdateWebhookRequest{
		RuleID:          "6d13a825822c4fe1990857f154b1cd6b",
		Tag:             "Main Twitter",
		Value:           twitterWebhookRuleset,
		IntervalSeconds: 30,
		IsEffect:        0,
	})
	if err != nil {
		log.Printf("Error turning off Twitter webhook: %v", err)
		return err
	}
	return nil
}
func turnOnTwitterNewsWebhook(conn *data.Conn) error {
	err := updateTwitterAPIRule(conn, TwitterAPIUpdateWebhookRequest{
		RuleID:          "6d13a825822c4fe1990857f154b1cd6b",
		Tag:             "Main Twitter",
		Value:           twitterWebhookRuleset,
		IntervalSeconds: 30,
		IsEffect:        1,
	})
	if err != nil {
		log.Printf("Error turning on Twitter webhook: %v", err)
		return err
	}
	return nil
}
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

	resp, err := client.Do(req)
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "success",
				"message": "Test webhook received",
			})
			return
		}

		// Process each tweet
		var extractedTweets []ExtractedTweetData
		for _, tweet := range payload.Tweets {
			extracted := ExtractedTweetData{
				URL:       tweet.URL,
				Text:      tweet.Text,
				CreatedAt: tweet.CreatedAt,
				Username:  tweet.Author.Username,
			}
			extractedTweets = append(extractedTweets, extracted)

		}
		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "success",
		})
		// Queue the extracted data for background processing
		err = processTwitterWebhookEvent(conn, extractedTweets)
		if err != nil {
			log.Printf("Error queueing Twitter webhook event: %v", err)
			http.Error(w, "Error processing webhook", http.StatusInternalServerError)
			return
		}
	}
}

// processTwitterWebhookEvent processes the extracted tweet data
func processTwitterWebhookEvent(conn *data.Conn, tweets []ExtractedTweetData) error {
	fmt.Println("queueTwitterWebhookEvent extractedTweets", tweets)
	for _, tweet := range tweets {
		processTweet(conn, tweet)
	}
	return nil
}

func processTweet(conn *data.Conn, tweet ExtractedTweetData) {

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
		Tickers:    tickers,
	})
	storeTweet(conn, tweet)

	peripheralContentToTweet, err := CreatePeripheralTweetFromNews(conn, tweet)
	if err != nil {
		log.Printf("Error creating peripheral tweet: %v", err)
		return
	}
	fmt.Println("Peripheral tweet", peripheralContentToTweet)
	SendTweetToPeripheralTwitterAccount(conn, peripheralContentToTweet)

}

type AgentPeripheralTweet struct {
	Text string      `json:"text" jsonschema:"required"`
	Plot interface{} `json:"plot" jsonschema:"required"`
}

type FormattedPeripheralTweet struct {
	Text  string `json:"text"`
	Image string `json:"image"`
}

func CreatePeripheralTweetFromNews(conn *data.Conn, tweet ExtractedTweetData) (FormattedPeripheralTweet, error) { // to implement don't forget

	prompt := tweet.Text
	fmt.Println("Starting Creating a Periphearl tweet from prompt", prompt)

	agentResult, err := agent.RunGeneralAgent[AgentPeripheralTweet](conn, "TweetCraftAdditionalSystemPrompt", "TweetCraftFinalSystemPrompt", prompt, "o4-mini", "medium")
	if err != nil {
		return FormattedPeripheralTweet{}, fmt.Errorf("error running general agent for tweet generation: %w", err)
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
	// Test Plotly rendering if plot data exists
	if agentResult.Plot != nil {
		if plotMap, ok := agentResult.Plot.(map[string]interface{}); ok {
			if titleTicker, exists := plotMap["titleTicker"].(string); exists && titleTicker != "" {
				titleIcon, _ := helpers.GetIcon(conn, titleTicker)
				plotMap["titleIcon"] = titleIcon
			}
			if _, hasData := plotMap["data"]; hasData {

				// Create renderer
				renderer, err := plotly.New()
				if err != nil {
					log.Printf("Failed to create Plotly renderer: %v", err)
				} else {
					defer renderer.Close()

					// Render the plot
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()

					base64PNG, err = renderer.RenderPlot(ctx, agentResult.Plot, nil)
					if err != nil {
						log.Printf("Failed to render plot: %v", err)
					}

					//saveImageToContainer(base64PNG)

				}
			}
		}
	}
	formattedPeripheralTweet := FormattedPeripheralTweet{
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
func determineIfAlreadySeenTweet(conn *data.Conn, tweet ExtractedTweetData) bool {
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
func storeTweet(conn *data.Conn, tweet ExtractedTweetData) {
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

func SendTweetToPeripheralTwitterAccount(conn *data.Conn, tweet FormattedPeripheralTweet) { // TODO: Implement plot rendering and image upload
	cfg := oauth1.NewConfig(conn.XAPIKey, conn.XAPISecretKey)
	token := oauth1.NewToken(conn.XAccessToken, conn.XAccessSecret)
	client := cfg.Client(oauth1.NoContext, token)
	payload := map[string]any{"text": tweet.Text}

	if tweet.Image != "" {
		imageID, err := UploadImageToTwitter(conn, tweet.Image)
		if err != nil {
			log.Printf("Error uploading image: %v", err)
			return
		}
		payload["media"] = map[string]any{"media_ids": []string{imageID}}
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.x.com/2/tweets", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending tweet: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated { // 201 on success
		log.Printf("X API returned %d — check rate limit or perms", resp.StatusCode)
		return
	}
	fmt.Println("Tweet sent successfully")
}

func UploadImageToTwitter(conn *data.Conn, image string) (string, error) {
	cfg := oauth1.NewConfig(conn.XAPIKey, conn.XAPISecretKey)
	token := oauth1.NewToken(conn.XAccessToken, conn.XAccessSecret)
	client := cfg.Client(oauth1.NoContext, token)

	// Create JSON payload with base64 image data
	payload := map[string]any{
		"media":          image, // base64 string as-is
		"media_category": "tweet_image",
		"media_type":     "image/png",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshaling payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.x.com/2/media/upload", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return "", fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request: %v", err)
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK { // 200 on success for v1.1 API
		log.Printf("X API returned %d — check rate limit or perms. Response: %s", resp.StatusCode, string(responseBody))
		return "", fmt.Errorf("x api returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse the JSON response (v1.1 API format)
	var uploadResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
		Errors []struct {
			Detail string `json:"detail,omitempty"`
			Status int    `json:"status,omitempty"`
			Title  string `json:"title,omitempty"`
			Type   string `json:"type,omitempty"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(responseBody, &uploadResponse); err != nil {
		log.Printf("Error parsing response JSON: %v", err)
		return "", fmt.Errorf("error parsing response JSON: %w", err)
	}

	// Check for errors in the response
	if len(uploadResponse.Errors) > 0 {
		log.Printf("X API returned errors: %+v", uploadResponse.Errors)
		return "", fmt.Errorf("x api error: %s", uploadResponse.Errors[0].Detail)
	}

	// Check if we got a media ID
	if uploadResponse.Data.ID == "" {
		log.Printf("No media ID in response: %s", string(responseBody))
		return "", fmt.Errorf("no media ID returned in response")
	}

	fmt.Printf("Image uploaded successfully with ID: %s\n", uploadResponse.Data.ID)
	return uploadResponse.Data.ID, nil
}

// saveImageToContainer saves base64 image data to container filesystem for debugging
func saveImageToContainer(base64Data string) {
	if base64Data == "" {
		return
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		log.Printf("Failed to decode base64 image: %v", err)
		return
	}

	// Use fixed filename
	filename := "/tmp/peripheral_plot.png"

	// Write to file
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Printf("Failed to save image to %s: %v", filename, err)
		return
	}

	log.Printf("✅ Plot image saved to container at: %s", filename)
	fmt.Printf("🚀 One-liner: docker cp $(docker ps --format 'table {{.Names}}' | grep backend | head -n1):/tmp/peripheral_plot.png ~/Desktop/ && open ~/Desktop/peripheral_plot.png\n")
}
