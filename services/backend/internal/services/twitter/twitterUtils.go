package twitter

import (
	"backend/internal/data"
	"backend/internal/services/telegram"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/dghubble/oauth1"
)

type FormattedPeripheralTweet struct {
	Text  string `json:"text"`
	Image string `json:"image"`
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

func SendTweetReplyToPeripheralTwitterAccount(conn *data.Conn, tweet FormattedPeripheralTweet, replyToTweetID string) { // TODO: Implement plot rendering and image upload
	cfg := oauth1.NewConfig(conn.XAPIKey, conn.XAPISecretKey)
	token := oauth1.NewToken(conn.XAccessToken, conn.XAccessSecret)
	client := cfg.Client(oauth1.NoContext, token)
	payload := map[string]any{"text": tweet.Text, "reply": map[string]any{"in_reply_to_tweet_id": replyToTweetID}}

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
		telegram.SendTelegramAskPeripheralTweets(replyToTweetID, tweet.Text, tweet.Image)
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
