package tools

import(
	"strings"

	"bytes"
	"encoding/base64"

	"github.com/pplcc/plotext/custplotter"
	"gonum.org/v1/plot"
	"google.golang.org/genai"
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	// Removed "strings" import as quoteSlice is moved
)

// AnalyzeInstanceFeaturesArgs contains parameters for analyzing features of a specific security instance
type AnalyzeInstanceFeaturesArgs struct {
	SecurityID int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"` // Unix ms of reference bar (0 ⇒ “now”)
	Timeframe  string `json:"timeframe"` // e.g. "15m", "h", "d"
	Bars       int    `json:"bars"`      // # of candles to pull **backward** from timestamp
}

// AnalyzeInstanceFeatures analyzes chart data for a specific security and returns Gemini's analysis
func AnalyzeInstanceFeatures(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {

	/* 1. Parse args */
	var args AnalyzeInstanceFeaturesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	if args.Bars <= 0 {
		args.Bars = 50 // sensible default
	}

	/* 2. Pull chart data (uses existing GetChartData) */
	chartReq := GetChartDataArgs{
		SecurityID:    args.SecurityID,
		Timeframe:     args.Timeframe,
		Timestamp:     args.Timestamp,
		Direction:     "backward",
		Bars:          args.Bars,
		ExtendedHours: false,
		IsReplay:      false,
	}
	reqBytes, _ := json.Marshal(chartReq)

	rawResp, err := GetChartData(conn, userId, reqBytes)
	if err != nil {
		return nil, fmt.Errorf("error fetching chart data: %v", err)
	}
	resp, ok := rawResp.(GetChartDataResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected GetChartData response type")
	}
	if len(resp.Bars) == 0 {
		return nil, fmt.Errorf("no bars returned for that window")
	}

	/* 3. Render a quick candlestick PNG (go‑chart v2 expects parallel slices) */
	// ─── Step 3: build and render the chart ─────────────────────────────────────
	var bars custplotter.TOHLCVs
	for _, b := range resp.Bars {
		// the candlestick plotter expects Unix seconds for the X value
		bars = append(bars, struct {
			T, O, H, L, C, V float64
		}{
			T: float64(b.Timestamp) / 1e3, // resp.Bars is milliseconds
			O: b.Open,
			H: b.High,
			L: b.Low,
			C: b.Close,
			V: b.Volume,
		})
	}

	// create the plot
	p := plot.New()
	//if err != nil { return nil, fmt.Errorf("plot init: %w", err) }

	p.HideY() // optional cosmetics
	p.X.Tick.Marker = plot.TimeTicks{Format: "01‑02\n15:04"}

	// add candlesticks
	candles, err := custplotter.NewCandlesticks(bars)
	if err != nil {
		return nil, fmt.Errorf("candles: %w", err)
	}
	p.Add(candles)

	// render to an in‑memory PNG
	var png bytes.Buffer
	wt, err := p.WriterTo(600, 300, "png") // width, height, format
	if err != nil {
		return nil, fmt.Errorf("writer: %w", err)
	}
	if _, err = wt.WriteTo(&png); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	pngB64 := base64.StdEncoding.EncodeToString(png.Bytes())

	barsJSON, _ := json.Marshal(resp.Bars)

	sysPrompt, err := getSystemInstruction("analyzeInstance")
	if err != nil {
		return nil, fmt.Errorf("error fetching system prompt: %v", err)
	}

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: sysPrompt}},
		},
	}

	// User‑side content parts
	userContent := &genai.Content{
		Parts: []*genai.Part{
			{Text: "BARS_JSON:\n" + string(barsJSON)},
			{Text: "CHART_PNG_BASE64:\n" + pngB64},
		},
	}

	apiKey, err := conn.GetGeminiKey()

	if err != nil {
		return nil, fmt.Errorf("error getting Gemini key: %v", err)
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})

	if err != nil {
		return nil, fmt.Errorf("error creating Gemini client: %v", err)
	}

	result, err := client.Models.GenerateContent(
		context.Background(),
		"gemini-2.0-flash-thinking-exp-01-21",
		[]*genai.Content{userContent}, // expects []*genai.Content
		cfg,
	)
	if err != nil {
		return nil, fmt.Errorf("gemini call failed: %v", err)
	}

	analysis := ""
	if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
		for _, p := range result.Candidates[0].Content.Parts {
			if p.Text != "" {
				analysis = p.Text
				break
			}
		}
	}

	return map[string]interface{}{
		"analysis": analysis, // Gemini’s narrative
		//	"bars":     json.RawMessage(barsJSON),
		//	"chart":    pngB64,           // base‑64 PNG for client preview
	}, nil
}

type CreateStrategyFromNaturalLanguageArgs struct {
	Query      string `json:"query"`
	StrategyId int    `json:"strategyId,omitempty"`
}

func CreateStrategyFromNaturalLanguage(conn *utils.Conn, userId int, rawArgs json.RawMessage) (any, error) {
	var args CreateStrategyFromNaturalLanguageArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	fmt.Printf("INFO: Starting CreateStrategyFromNaturalLanguage for user %d with query: %q\n", userId, args.Query)

	apikey, err := conn.GetGeminiKey()
	if err != nil {
		fmt.Printf("ERROR: Error getting Gemini key for user %d: %v\n", userId, err)
		return nil, fmt.Errorf("error getting gemini key: %v", err)
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apikey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating gemini client: %v", err)
	}



	// getSystemInstruction now fetches all required data internally
	systemInstruction, err := getSystemInstruction("spec")
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %v", err)
	}
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
	}

	maxRetries := 3
	var lastErr error
	// genai.Text returns []*genai.Content, we need the first element for the initial message
	initialContent := genai.Text(args.Query) // genai.Text returns one value: []*genai.Content
	if len(initialContent) == 0 || initialContent[0] == nil {
		// Handle the case where genai.Text might return an empty slice or nil content, though unlikely for a simple query.
		return nil, fmt.Errorf("failed to create initial content from query")
	}
	conversationHistory := []*genai.Content{initialContent[0]} // Start with the user's query

	for attempt := 0; attempt < maxRetries; attempt++ {
		fmt.Printf("Attempt %d/%d to generate and validate strategy spec...\n", attempt+1, maxRetries)

		// Generate content using the current conversation history
		result, err := client.Models.GenerateContent(context.Background(), "gemini-2.0-flash-thinking-exp-01-21", conversationHistory, config)
		if err != nil {
			lastErr = fmt.Errorf("error generating content (attempt %d): %w", attempt+1, err)
			fmt.Printf("WARN: Attempt %d/%d for user %d: Gemini content generation failed: %v\n", attempt+1, maxRetries, userId, err)
			// Append error as user message for context? Maybe too noisy. Let's just retry.
			continue // Retry the API call
		}

		responseText := ""
		if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
			// Append assistant's response to history for the next turn
			conversationHistory = append(conversationHistory, result.Candidates[0].Content)
			for _, part := range result.Candidates[0].Content.Parts {
				if part.Text != "" {
					responseText = part.Text
					break
				}
			}
		} else {
			lastErr = fmt.Errorf("gemini returned no response candidates (attempt %d)", attempt+1)
			fmt.Printf("WARN: Attempt %d/%d for user %d: Gemini returned no response candidates.\n", attempt+1, maxRetries, userId)
			// Append error message as user turn
			errorMsg := fmt.Sprintf("Attempt %d failed: Gemini returned no response. Please try again.", attempt+1)
			// Parts expects []*genai.Part, and we need to create a Part directly
			conversationHistory = append(conversationHistory, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: errorMsg}}})
			continue
		}

		fmt.Printf("DEBUG: Attempt %d/%d for user %d: Raw Gemini response: %s\n", attempt+1, maxRetries, userId, responseText)

		// Extract JSON block
		jsonStartIdx := strings.Index(responseText, "{")
		jsonEndIdx := strings.LastIndex(responseText, "}")

		if jsonStartIdx == -1 || jsonEndIdx == -1 || jsonEndIdx <= jsonStartIdx {
			lastErr = fmt.Errorf("no valid JSON block found in Gemini response (attempt %d): %s", attempt+1, responseText)
			fmt.Printf("WARN: Attempt %d/%d for user %d: No valid JSON block found in Gemini response.\n", attempt+1, maxRetries, userId)
			// Append error message as user turn
			errorMsg := fmt.Sprintf("Attempt %d failed: Could not find a valid JSON block in the response. Please ensure the response contains a single, valid JSON object starting with '{' and ending with '}'. The response received was:\n%s", attempt+1, responseText)
			// Parts expects []*genai.Part, and we need to create a Part directly
			conversationHistory = append(conversationHistory, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: errorMsg}}})

			continue
		}

		jsonBlock := responseText[jsonStartIdx : jsonEndIdx+1]

		// Log the raw JSON block returned by Gemini
		fmt.Printf("DEBUG: Attempt %d/%d for user %d: Extracted JSON block: \n%s\n", attempt+1, maxRetries, userId, jsonBlock)

		// Use the new helper function to unmarshal and validate the entire JSON block
		name, spec, err := UnmarshalAndValidateNewStrategyInput([]byte(jsonBlock))
		if err != nil {
			lastErr = fmt.Errorf("failed to unmarshal or validate Gemini JSON response (attempt %d): %w", attempt+1, err)
			fmt.Printf("WARN: Attempt %d/%d for user %d: Failed to unmarshal/validate JSON: %v\n", attempt+1, maxRetries, userId, err)
			// Format error message for the next prompt
			errorMsg := fmt.Sprintf("Attempt %d failed: Could not parse or validate the JSON structure. Ensure the response is a single JSON object with 'name' (string) and 'spec' (object) fields, and that the spec is valid. Error: %v\nReceived JSON:\n%s\nPlease fix the JSON based on the error and the original query.", attempt+1, err, jsonBlock)
			// Parts expects []*genai.Part, and we need to create a Part directly
			conversationHistory = append(conversationHistory, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: errorMsg}}})
			continue
		}

		fmt.Printf("INFO: Attempt %d/%d for user %d: Successfully validated name '%s' and spec.\n", attempt+1, maxRetries, userId, name)

		// --- Compile the validated spec to SQL ---
		sqlQuery, compileErr := CompileSpecToSQL(spec)
		if compileErr != nil {
			// Log the error but proceed with saving the strategy for now.
			// The user might want to fix the spec manually later.
			fmt.Printf("ERROR: User %d: Failed to compile validated spec for strategy '%s' to SQL: %v\n", userId, name, compileErr)
			// Optionally, you could add this error info back into the conversation history
			// if you wanted Gemini to potentially fix the spec based on compilation failure.
		} else {
			fmt.Printf("INFO: User %d: Successfully compiled spec for strategy '%s' to SQL:\n%s\n", userId, name, sqlQuery)
			// You could potentially store this compiled SQL alongside the strategy spec if needed.
		}
		// --- End SQL Compilation ---

		// If validation succeeds, create the strategy using the validated name and spec
		strategyId, err := NewStrategyUtil(conn, userId, name, spec)
		if err != nil {
			// This is an internal error, not Gemini's fault, so return directly
			fmt.Printf("ie20hifi0: %v\n", err)
			return nil, fmt.Errorf("error saving validated strategy: %w", err)
		}

		// Return the successful result
		return map[string]interface{}{
			"strategyId": strategyId,
			"name":       name,
			//"spec":       spec, // Return the validated spec object
		}, nil
	}

	// If loop finishes without success
	fmt.Printf("ERROR: User %d: Failed to create valid strategy from query %q after %d attempts. Last error: %v\n", userId, args.Query, maxRetries, lastErr)
	return nil, fmt.Errorf("failed to create valid strategy after %d attempts: %w", maxRetries, lastErr)
}

// Removed quoteSlice helper function - moved to prompt.go as joinQuoteSlice
