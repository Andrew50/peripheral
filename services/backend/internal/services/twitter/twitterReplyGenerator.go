// Package twitter integrates with X/Twitter workflows for generating replies
// and media for tweets using internal agent and rendering services.
package twitter

import (
	"backend/internal/app/agent"
	"backend/internal/app/helpers"
	"backend/internal/data"
	"backend/internal/services/plotly"
	"backend/internal/services/telegram"
	"context"
	"log"
	"time"
)

type AgentReplyTweet struct {
	Text string      `json:"text" jsonschema:"required"`
	Plot interface{} `json:"plot" jsonschema:"required"`
}

type FormattedReplyTweet struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Text     string `json:"text"`
	Image    string `json:"image"`
	ID       string `json:"id"`
}
type ExtractedTweetData struct {
	URL       string `json:"url"`
	Username  string `json:"username"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
}

func HandleTweetForReply(conn *data.Conn, tweet ExtractedTweetData) error {
	prompt := "TWEET: " + tweet.Text

	agentResult, err := agent.RunGeneralAgent[AgentReplyTweet](conn, 0, "TweetBreakingHeadlineSystemPrompt", "TweetReplyFinalResponse", prompt, "o4-mini", "medium")
	if err != nil {
		log.Printf("Error running agent: %v", err)
		return err
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
					}

					//saveImageToContainer(base64PNG)

				}
			}
		}
	}
	formattedReply := FormattedReplyTweet{
		URL:      tweet.URL,
		Username: tweet.Username,
		Text:     agentResult.Text,
		Image:    base64PNG,
		ID:       tweet.ID,
	}
	return SendTweetToTelegram(conn, formattedReply)
}
func SendTweetToTelegram(conn *data.Conn, tweet FormattedReplyTweet) error {

	return telegram.SendTelegramBenTweetsMessage(tweet.URL, tweet.ID, tweet.Text, tweet.Image)

}
