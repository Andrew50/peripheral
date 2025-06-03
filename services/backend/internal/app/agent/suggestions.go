package agent

import (
	"backend/internal/app/chart"
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"
)

var thinkingModel = "gemini-2.5-flash-preview-05-20"

// buildContextPrompt formats incoming chart/filing context for the model
func buildContextPrompt(contextItems []map[string]interface{}) string {
	var sb strings.Builder
	for _, item := range contextItems {
		// Treat filing contexts first
		if _, ok := item["link"]; ok {
			ticker, _ := item["ticker"].(string)
			fType, _ := item["filingType"].(string)
			link, _ := item["link"].(string)
			sb.WriteString(fmt.Sprintf("Filing - Ticker: %s, Type: %s, Link: %s\n", ticker, fType, link))
		} else if _, ok := item["timestamp"]; ok {
			// Then treat instance contexts
			ticker, _ := item["ticker"].(string)
			secID := fmt.Sprint(item["securityId"])
			tsStr := fmt.Sprint(item["timestamp"])
			sb.WriteString(fmt.Sprintf("Instance - Ticker: %s, SecurityId: %s, TimestampMs: %s\n", ticker, secID, tsStr))
		}
	}
	return sb.String()
}

type GetSuggestedQueriesResponse struct {
	Suggestions []string `json:"suggestions"`
}

func GetSuggestedQueries(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {

	// Use the standardized Redis connectivity test
	ctx := context.Background()
	success, message := conn.TestRedisConnectivity(ctx, userID)
	if !success {
		return nil, fmt.Errorf("%s", message)
		////fmt.Printf("WARNING: %s\n", message)
	}
	//else {
	////fmt.Println(message)
	//}
	conversationData, err := GetConversationFromCache(ctx, conn, userID)
	if err != nil || conversationData == nil {
		return GetSuggestedQueriesResponse{}, nil
	}
	var conversationHistory string
	if len(conversationData.Messages) > 0 {
		conversationHistory = _buildConversationContext(conversationData.Messages)
	}

	geminiRes, err := getGeminiFunctionThinking(ctx, conn, "suggestedQueriesPrompt", conversationHistory, thinkingModel)
	if err != nil {
		return nil, fmt.Errorf("error getting suggested queries from Gemini: %w", err)
	}
	jsonStartIdx := strings.Index(geminiRes.Text, "{")
	jsonEndIdx := strings.LastIndex(geminiRes.Text, "}")
	if jsonStartIdx == -1 || jsonEndIdx == -1 {
		return GetSuggestedQueriesResponse{}, nil
	}
	jsonBlock := geminiRes.Text[jsonStartIdx : jsonEndIdx+1]
	var response GetSuggestedQueriesResponse
	if err := json.Unmarshal([]byte(jsonBlock), &response); err != nil {
		return GetSuggestedQueriesResponse{}, fmt.Errorf("error unmarshalling suggested queries: %w", err)
	}
	return response, nil

}

type GetInitialQuerySuggestionsArgs struct {
	ActiveChartInstance map[string]interface{} `json:"activeChartInstance"`
}
type GetInitialQuerySuggestionsResponse struct {
	Suggestions []string `json:"suggestions"`
}

func GetInitialQuerySuggestions(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	ctx := context.Background()

	var args GetInitialQuerySuggestionsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error unmarshalling initial query suggestions args: %w", err)
	}

	if args.ActiveChartInstance == nil {
		return GetInitialQuerySuggestionsResponse{Suggestions: []string{}}, nil
	}

	// --- Data Fetching ---
	securityIDFloat, secIDOk := args.ActiveChartInstance["securityId"].(float64)
	//timestampFloat, tsOk := args.ActiveChartInstance["timestamp"].(float64)
	//ticker, tickerOk := args.ActiveChartInstance["ticker"].(string)
	barsToFetch := 10 // Fetch a decent number for context/plotting

	if !secIDOk {
		return nil, fmt.Errorf("invalid activeChartInstance format: missing or wrong type for securityId")
	}

	securityID := int(securityIDFloat)
	//timestamp := int64(timestampFloat)

	// Fetch recent price bars
	/*jjjpriceBars, err := postgres.GetLatestBarsForSecurity(conn, securityID, barsToFetch, timestamp)
	if err != nil {
		////fmt.Printf("Warning: Could not fetch price bars for %s: %v\n", ticker, err)
		// Don't fail, just won't have price context
	}*/

	// Fetch recent news
	chartReq := chart.GetChartDataArgs{
		SecurityID:    securityID,
		Timeframe:     "1d",
		Timestamp:     0,
		Direction:     "backward",
		Bars:          barsToFetch,
		ExtendedHours: false,
		IsReplay:      false,
	}
	reqBytes, _ := json.Marshal(chartReq)

	rawResp, chartErr := chart.GetChartData(conn, userID, reqBytes)
	if chartErr != nil {
		////fmt.Printf("Warning: error fetching chart data for suggestions: %v\n", chartErr)
		return GetInitialQuerySuggestionsResponse{Suggestions: []string{}}, nil
	}
	resp, ok := rawResp.(chart.GetChartDataResponse)
	if !ok || len(resp.Bars) == 0 {
		////fmt.Println("Warning: no bars returned or unexpected type from GetChartData.")
		return GetInitialQuerySuggestionsResponse{Suggestions: []string{}}, nil
	}
	// --- End Data Fetching ---

	// Add DateString to each bar
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, err
		// Handle error, perhaps log and continue without date strings
		////fmt.Printf("Warning: could not load America/New_York timezone: %v\n", err)
	}

	processedBars := make([]map[string]interface{}, len(resp.Bars))
	for i, bar := range resp.Bars {
		// Create a map from the struct fields for JSON marshalling
		barMap := map[string]interface{}{
			"open":   bar.Open,
			"high":   bar.High,
			"low":    bar.Low,
			"close":  bar.Close,
			"volume": bar.Volume,
		}

		if easternLocation != nil {
			// bar.Timestamp is float64, assume it's Unix timestamp in seconds or milliseconds.
			// Convert to int64 for time.Unix
			ts := bar.Timestamp
			var sec int64
			var nsec int64
			// Check if float64 has a fractional part or is large (milliseconds)
			if ts > 1e12 { // Heuristic: if it's a large number, assume milliseconds
				sec = int64(ts) / 1000
				nsec = (int64(ts) % 1000) * 1e6
			} else if ts == float64(int64(ts)) { // Whole number, likely seconds
				sec = int64(ts)
			} else { // Has fractional part, treat as seconds with nanoseconds
				sec = int64(ts)
				nsec = int64((ts - float64(sec)) * 1e9)
			}
			timestamp := time.Unix(sec, nsec).In(easternLocation)
			barMap["DateS"] = timestamp.Format("2006-01-02")
		} else {
			barMap["Date"] = bar.Timestamp
		}
		processedBars[i] = barMap
	}

	barsJSON, _ := json.MarshalIndent(processedBars, "", "  ") // Use processedBars

	sysPrompt, err := getSystemInstruction("initialQueriesPrompt")
	if err != nil {
		////fmt.Printf("Error getting system instruction: %v\n", err)
		return GetInitialQuerySuggestionsResponse{Suggestions: []string{}}, fmt.Errorf("error fetching system prompt: %w", err)
	}

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: sysPrompt}},
		},
	}
	// Build user content parts
	userParts := []*genai.Part{
		{Text: "<ChartInstanceContext>\n" + buildContextPrompt([]map[string]interface{}{args.ActiveChartInstance}) + "</ChartInstanceContext>\n"},
		{Text: "<RecentOHLCVData>\n```json\n" + string(barsJSON) + "\n```\n</RecentOHLCVData>\n"},
	}
	userContent := &genai.Content{Parts: userParts}
	// --- End Prompt Preparation ---

	// --- Call LLM ---
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return nil, fmt.Errorf("error getting Gemini key: %w", err)
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Gemini client: %w", err)
	}

	// Use GenerateContent with []*genai.Content input
	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash-preview-05-20",
		[]*genai.Content{userContent},
		cfg,
	)
	if err != nil {
		////fmt.Printf("Error getting initial suggestions from Gemini: %v\n", err)
		return GetInitialQuerySuggestionsResponse{Suggestions: []string{}}, nil
	}
	// --- End Call LLM ---

	// --- Parse Response ---
	llmResponseText := ""
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		for _, p := range result.Candidates[0].Content.Parts {
			if p.Thought {
				continue
			}
			if p.Text != "" {
				llmResponseText = p.Text
				break
			}
		}
	}

	jsonStartIdx := strings.Index(llmResponseText, "{")
	jsonEndIdx := strings.LastIndex(llmResponseText, "}")

	if jsonStartIdx == -1 || jsonEndIdx == -1 || jsonEndIdx < jsonStartIdx {
		////fmt.Printf("No valid JSON block found in initial suggestions response: %s\n", llmResponseText)
		return GetInitialQuerySuggestionsResponse{Suggestions: []string{}}, nil
	}

	jsonBlock := llmResponseText[jsonStartIdx : jsonEndIdx+1]
	var response GetInitialQuerySuggestionsResponse
	if err := json.Unmarshal([]byte(jsonBlock), &response); err != nil {
		////fmt.Printf("Error unmarshalling initial suggestions JSON: %v. JSON block: %s\n", err, jsonBlock)
		return GetInitialQuerySuggestionsResponse{Suggestions: []string{}}, nil
	}
	// --- End Parse Response ---

	return response, nil
}
