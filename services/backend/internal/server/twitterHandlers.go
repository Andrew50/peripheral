package server

import (
	"backend/internal/data"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// TwitterWebhookPayload represents only the fields we need from Twitter webhook
type TwitterWebhookPayload struct {
	Tweets []Tweet `json:"tweets"`
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

		// Queue the extracted data for background processing
		err = queueTwitterWebhookEvent(conn, extractedTweets)
		if err != nil {
			log.Printf("Error queueing Twitter webhook event: %v", err)
			http.Error(w, "Error processing webhook", http.StatusInternalServerError)
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf("Processed %d tweets", len(extractedTweets)),
		})
	}
}

// queueTwitterWebhookEvent queues the extracted tweet data for background processing
func queueTwitterWebhookEvent(conn *data.Conn, tweets []ExtractedTweetData) error {
	fmt.Println("queueTwitterWebhookEvent extractedTweets", tweets)
	return nil
}

// twitterWebhookHandler is the HTTP handler wrapper
func twitterWebhookHandler(conn *data.Conn) http.HandlerFunc {
	return HandleTwitterWebhook(conn)
}
