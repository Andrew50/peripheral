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

			// Log the extracted data
			log.Printf("Extracted Tweet Data: URL=%s, Text=%s, CreatedAt=%s, Username=%s",
				extracted.URL, extracted.Text, extracted.CreatedAt, extracted.Username)
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
	fmt.Println("seen", seen)
	/*if seen {
		storeTweet(conn, tweet)
		return
	}*/
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
	//SendTweetToPeripheralTwitterAccount(conn, peripheralContentToTweet)

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
	agentResult, err := agent.RunGeneralAgent[AgentPeripheralTweet](conn, "TweetCraftAdditionalSystemPrompt", "TweetCraftFinalSystemPrompt", prompt)
	if err != nil {
		return FormattedPeripheralTweet{}, fmt.Errorf("error running general agent for tweet generation: %w", err)
	}
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
					} else {

						// Save PNG file for verification
						pngData, err := base64.StdEncoding.DecodeString(base64PNG)
						if err == nil {
							// Use a fixed filename for easy copying
							filename := "/tmp/latest_plot.png"
							if err := os.WriteFile(filename, pngData, 0644); err == nil {
								fmt.Printf("Plot saved to container: %s\n", filename)
								fmt.Printf("To copy to your local machine, run:\n")
								fmt.Printf("docker cp dev-backend-1:%s ./plot.png && open ./plot.png\n", filename)
							}
						}
					}
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
	body, _ := json.Marshal(payload)
	fmt.Println("body", string(body))

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

// twitterWebhookHandler is the HTTP handler wrapper
func twitterWebhookHandler(conn *data.Conn) http.HandlerFunc {
	return HandleTwitterWebhook(conn)
}
