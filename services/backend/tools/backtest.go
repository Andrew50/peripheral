package tools

/*
import (
	"backend/utils"
	"context"
	"fmt"

	"google.golang.org/genai"
)

type BacktestArgs struct {
	Query string `json:"query"`
}

func GetBacktestJSONFromGemini(conn *utils.Conn, query string) (string, error) {
	apikey, err := conn.GetGeminiKey()
	if err != nil {
		return "", fmt.Errorf("error getting gemini key: %v", err)
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apikey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("error creating gemini client: %v", err)
	}

	systemInstruction, err := getSystemInstruction("backtestSystemPrompt")
	if err != nil {
		return "", fmt.Errorf("error getting system instruction: %v", err)
	}
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
	}
	result, err := client.Models.GenerateContent(context.Background(), "gemini-2.0-flash-thinking-exp-01-21", genai.Text(query), config)
	if err != nil {
		return "", fmt.Errorf("error generating content: %v", err)
	}

	responseText := ""
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		for _, part := range result.Candidates[0].Content.Parts {
			if part.Text != "" {
				responseText = part.Text
				break
			}
		}
	}

	return responseText, nil
}
func GetDataForBacktest(conn *utils.Conn, backtestJSON string) (string, error) {
	// TODO: Implement this
	return "", nil
}

/*
func RunBacktest(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {

	var args BacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	backtestJSON, err := GetBacktestJSONFromGemini(conn, args.Query)
	if err != nil {
		return nil, fmt.Errorf("Error getting backtest JSON from Gemini: %v", err)
	}
	data, err := GetDataForBacktest(conn, backtestJSON)
	if err != nil {
		return nil, fmt.Errorf("Error getting data for backtest: %v", err)
	}

}*/
