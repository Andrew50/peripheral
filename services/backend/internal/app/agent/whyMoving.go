package agent

import (
	"backend/internal/app/helpers"
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"
)

type GetWhyMovingArgs struct {
	Tickers []string `json:"tickers"`
}

func GetWhyMoving(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetWhyMovingArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}
	runWhyMovingArgs := WhyMovingArgs{
		Tickers:  args.Tickers,
		Priority: false,
	}
	runWhyMovingArgsBytes, err := json.Marshal(runWhyMovingArgs)
	if err != nil {
		return nil, err
	}
	results, err := RunWhyMoving(conn, json.RawMessage(runWhyMovingArgsBytes))
	if err != nil {
		return nil, err
	}
	return results, nil
}

type WhyMovingArgs struct {
	Tickers  []string `json:"tickers"`
	Priority bool     `json:"priority"`
}

type WhyMovingResult struct {
	Ticker    string    `json:"ticker"`
	IsContent bool      `json:"is_content"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func RunWhyMoving(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args WhyMovingArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}

	if !args.Priority {
		// Get existing reasons and identify missing tickers
		existingResponse, err := getExistingReasons(conn, args.Tickers)
		if err != nil {
			return nil, err
		}

		// If some tickers are missing, generate new reasons for them
		if len(existingResponse.MissingTickers) > 0 {
			newReasons, err := generateWhyMoving(conn, existingResponse.MissingTickers)
			if err != nil {
				return nil, fmt.Errorf("failed to generate new reasons: %w", err)
			}
			for _, reason := range newReasons {
				if reason.IsContent {
					existingResponse.ExistingReasons = append(existingResponse.ExistingReasons, reason)
				}
			}
			return existingResponse.ExistingReasons, nil
		}

		// Return only existing reasons if all tickers were found
		return existingResponse.ExistingReasons, nil
	}
	var results []WhyMovingResult
	movingReasons, err := generateWhyMoving(conn, args.Tickers)
	if err != nil {
		return nil, fmt.Errorf("failed to generate why moving: %w", err)
	}
	for _, reason := range movingReasons {
		if reason.IsContent {
			results = append(results, reason)
		}
	}
	return results, nil
}

func whyIsItMovingSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeArray,
		Items: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"ticker", "content"},
			Properties: map[string]*genai.Schema{
				"ticker": {
					Type:        genai.TypeString,
					Description: "The stock ticker symbol",
				},
				"isContent": {
					Type:        genai.TypeBoolean,
					Description: "Whether there is a reason in the last 24-48 hours why the stock is moving. If there is no reason, set this to false.",
				},
				"content": {
					Type:        genai.TypeString,
					Description: "The explanation of why the stock is moving",
				},
			},
		},
		Title:       "WhyIsItMovingArray",
		Description: "An array of WhyIsItMoving responses for multiple tickers",
	}
}

type LLMWhyMovingResult struct {
	Ticker    string `json:"ticker"`
	IsContent bool   `json:"isContent"`
	Content   string `json:"content"`
}

func generateWhyMoving(conn *data.Conn, tickers []string) ([]WhyMovingResult, error) {
	// Use web search to find reasons for stock movements
	searchTextResult, err := _RunSearchHelper(conn, tickers)
	if err != nil {
		return nil, fmt.Errorf("failed to run web search: %w", err)
	}
	prompt := ""
	for _, ticker := range tickers {
		prompt += fmt.Sprintf("%s\n", ticker)
	}
	prompt += fmt.Sprintf("\n%s", searchTextResult)
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
	systemPrompt, err := getSystemInstruction("whyMovingPrompt")
	if err != nil {
		return nil, fmt.Errorf("error getting why moving system instruction: %w", err)
	}
	thinkingBudget := int32(1000)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: false,
			ThinkingBudget:  &thinkingBudget,
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   whyIsItMovingSchema(),
	}
	result, err := client.Models.GenerateContent(context.Background(), geminiWebSearchModel, genai.Text(prompt), config)
	if err != nil {
		return nil, fmt.Errorf("error generating content: %w", err)
	}
	resultText := ""
	if len(result.Candidates) <= 0 {
		return nil, fmt.Errorf("no candidates found in result")
	}
	candidate := result.Candidates[0]
	if candidate != nil && candidate.Content != nil && candidate.Content.Parts != nil {
		for _, part := range candidate.Content.Parts {
			if part != nil && part.Text != "" {
				resultText = part.Text
				break
			}
		}
	}
	fmt.Println("resultText: ", resultText)
	var llmResults []LLMWhyMovingResult
	err = json.Unmarshal([]byte(resultText), &llmResults)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling llm results: %w", err)
	}

	var results []WhyMovingResult
	now := time.Now()
	for _, result := range llmResults {
		// Only include results where there is actual content
		results = append(results, WhyMovingResult{
			Ticker:    result.Ticker,
			IsContent: result.IsContent,
			Content:   result.Content,
			CreatedAt: now,
		})
	}

	// Insert into database in parallel (non-blocking)
	go func() {
		if err := insertWhyMovingResults(conn, results); err != nil {
			// Log error but don't block the return
			fmt.Printf("Error inserting why moving results to database: %v\n", err)
		}
	}()

	return results, nil
}

func _RunSearchHelper(conn *data.Conn, tickers []string) (string, error) {
	var sb strings.Builder
	sb.WriteString("For each of the following tickers, find out why it is gapping/moving as of recently. Prioritize the most recent news.")
	for _, ticker := range tickers {
		sb.WriteString(fmt.Sprintf("\n%s", ticker))
	}

	// Create channels for parallel execution
	webSearchChan := make(chan string)
	twitterSearchChan := make(chan string)
	webSearchErrChan := make(chan error)
	twitterSearchErrChan := make(chan error)

	// Run web search in parallel
	go func() {
		searchArgs := WebSearchArgs{
			Query: sb.String(),
		}
		argsBytes, err := json.Marshal(searchArgs)
		if err != nil {
			webSearchErrChan <- fmt.Errorf("failed to marshal web search args: %w", err)
			return
		}

		result, err := RunWebSearch(conn, 0, json.RawMessage(argsBytes))
		if err != nil {
			webSearchErrChan <- fmt.Errorf("web search failed: %w", err)
			return
		}

		if searchResult, ok := result.(WebSearchResult); ok {
			webSearchChan <- searchResult.ResultText
		} else {
			webSearchErrChan <- fmt.Errorf("unexpected web search result type")
		}
	}()

	// Run twitter search in parallel
	go func() {
		twitterSearchArgs := TwitterSearchArgs{
			Prompt:   sb.String(),
			FromDate: time.Now().Add(-1 * time.Hour * 24).Format("2006-01-02"),
			ToDate:   time.Now().Format("2006-01-02"),
		}
		argsBytes, err := json.Marshal(twitterSearchArgs)
		if err != nil {
			twitterSearchErrChan <- fmt.Errorf("failed to marshal twitter search args: %w", err)
			return
		}

		result, err := RunTwitterSearch(conn, 0, json.RawMessage(argsBytes))
		if err != nil {
			twitterSearchErrChan <- fmt.Errorf("twitter search failed: %w", err)
			return
		}

		if resultStr, ok := result.(string); ok {
			twitterSearchChan <- resultStr
		} else {
			twitterSearchErrChan <- fmt.Errorf("unexpected twitter search result type")
		}
	}()

	// Collect results from both searches
	var webSearchResult, twitterSearchResult string
	var webSearchErr, twitterSearchErr error

	for i := 0; i < 2; i++ {
		select {
		case webSearchResult = <-webSearchChan:
		case twitterSearchResult = <-twitterSearchChan:
		case webSearchErr = <-webSearchErrChan:
		case twitterSearchErr = <-twitterSearchErrChan:
		}
	}

	// Handle errors
	if webSearchErr != nil && twitterSearchErr != nil {
		return "", fmt.Errorf("both searches failed - web: %w, twitter: %w", webSearchErr, twitterSearchErr)
	}

	// Combine results
	var combinedResult strings.Builder
	if webSearchErr == nil && webSearchResult != "" {
		combinedResult.WriteString("Web Search Results:\n")
		combinedResult.WriteString(webSearchResult)
		combinedResult.WriteString("\n")
	}
	if twitterSearchErr == nil && twitterSearchResult != "" {
		combinedResult.WriteString("Twitter Search Results:\n")
		combinedResult.WriteString(twitterSearchResult)
	}

	return combinedResult.String(), nil
}

type ExistingReasonsResponse struct {
	ExistingReasons []WhyMovingResult `json:"existing_reasons"`
	MissingTickers  []string          `json:"missing_tickers"`
}

func getExistingReasons(conn *data.Conn, tickers []string) (*ExistingReasonsResponse, error) {
	if len(tickers) == 0 {
		return &ExistingReasonsResponse{
			ExistingReasons: []WhyMovingResult{},
			MissingTickers:  []string{},
		}, nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(tickers))
	args := make([]interface{}, len(tickers)+1)

	for i, ticker := range tickers {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = ticker
	}

	// 90 minutes ago
	args[0] = time.Now().Add(-90 * time.Minute)

	query := fmt.Sprintf(`
		SELECT DISTINCT ON (ticker) ticker, is_content, content, created_at
		FROM why_is_it_moving 
		WHERE created_at >= $1 
		AND ticker IN (%s)
		AND is_content = true
		ORDER BY ticker, created_at DESC
	`, strings.Join(placeholders, ","))

	rows, err := conn.DB.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing reasons: %w", err)
	}
	defer rows.Close()

	var existingReasons []WhyMovingResult
	foundTickers := make(map[string]bool)

	for rows.Next() {
		var result WhyMovingResult

		err := rows.Scan(
			&result.Ticker,
			&result.IsContent,
			&result.Content,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		existingReasons = append(existingReasons, result)
		foundTickers[result.Ticker] = true
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	// Identify missing tickers
	var missingTickers []string
	for _, ticker := range tickers {
		if !foundTickers[ticker] {
			missingTickers = append(missingTickers, ticker)
		}
	}

	return &ExistingReasonsResponse{
		ExistingReasons: existingReasons,
		MissingTickers:  missingTickers,
	}, nil
}

// insertWhyMovingResults inserts the results into the database
func insertWhyMovingResults(conn *data.Conn, results []WhyMovingResult) error {
	if len(results) == 0 {
		return nil
	}

	query := `
		INSERT INTO why_is_it_moving (securityid, ticker, is_content, content, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	for _, result := range results {
		// Create the correct args structure for GetCurrentSecurityID
		args := helpers.GetCurrentSecurityIDArgs{
			Ticker: result.Ticker,
		}
		argsBytes, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("failed to marshal security ID args for ticker %s: %w", result.Ticker, err)
		}

		securityID, err := helpers.GetCurrentSecurityID(conn, 0, json.RawMessage(argsBytes))
		if err != nil {
			return fmt.Errorf("failed to get security id for ticker %s: %w", result.Ticker, err)
		}
		_, err = conn.DB.Exec(context.Background(), query, securityID, result.Ticker, result.IsContent, result.Content, result.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert result for ticker %s: %w", result.Ticker, err)
		}
	}

	return nil
}
