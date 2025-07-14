package server

import (
	"backend/internal/data"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"backend/internal/services/socket"

	"github.com/dghubble/oauth1"
)

// TwitterWebhookPayload represents only the fields we need from Twitter webhook
type TwitterWebhookPayload struct {
	Tweets    []Tweet `json:"tweets,omitempty"`
	EventType string  `json:"event_type,omitempty"`
}

// Tweet represents only the fields we need from each tweet
type Tweet struct {
	URL       string `json:"url"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
	Author    Author `json:"author"`
}

// Author represents only the username field we need
type Author struct {
	Username string `json:"userName"`
}

// ExtractedTweetData represents the clean data we extract for processing
type ExtractedTweetData struct {
	URL       string `json:"url"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
	Username  string `json:"username"`
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
		// Extract ticker symbols from the tweet text
		tickers := extractTickersFromTweet(tweet.Text)
		SendTweet(conn, "testing tweet: "+tweet.Text)
		socket.SendAlertToAllUsers(socket.AlertMessage{
			AlertID:    1,
			Timestamp:  time.Now().Unix() * 1000,
			SecurityID: 1,
			Message:    tweet.Text,
			Channel:    "alert",
			Tickers:    tickers,
		})
	}
	return nil
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

func SendTweet(conn *data.Conn, tweet string) {
	cfg := oauth1.NewConfig(conn.XAPIKey, conn.XAPISecretKey)
	token := oauth1.NewToken(conn.XAccessToken, conn.XAccessSecret)
	client := cfg.Client(oauth1.NoContext, token)

	payload := map[string]any{"text": tweet}
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

	fmt.Println("Tweet sent successfully")
}

// twitterWebhookHandler is the HTTP handler wrapper
func twitterWebhookHandler(conn *data.Conn) http.HandlerFunc {
	return HandleTwitterWebhook(conn)
}
