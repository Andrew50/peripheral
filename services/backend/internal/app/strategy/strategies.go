package strategy

import (
	"backend/internal/app/chart"
	"backend/internal/data"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bytes"
	"encoding/base64"

	"github.com/pplcc/plotext/custplotter"
	"gonum.org/v1/plot"
	"google.golang.org/genai"
)

//go:embed prompts/*
var fs embed.FS // 2️⃣ compiled into the binary

// AnalyzeInstanceFeaturesArgs contains parameters for analyzing features of a specific security instance
type AnalyzeInstanceFeaturesArgs struct {
	SecurityID int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"` // Unix ms of reference bar (0 ⇒ "now")
	Timeframe  string `json:"timeframe"` // e.g. "15m", "h", "d"
	Bars       int    `json:"bars"`      // # of candles to pull **backward** from timestamp
}

// AnalyzeInstanceFeatures analyzes chart data for a specific security and returns Gemini's analysis
func AnalyzeInstanceFeatures(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {

	/* 1. Parse args */
	var args AnalyzeInstanceFeaturesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	if args.Bars <= 0 {
		args.Bars = 50 // sensible default
	}

	/* 2. Pull chart data (uses existing GetChartData) */
	chartReq := chart.GetChartDataArgs{
		SecurityID:    args.SecurityID,
		Timeframe:     args.Timeframe,
		Timestamp:     args.Timestamp,
		Direction:     "backward",
		Bars:          args.Bars,
		ExtendedHours: false,
		IsReplay:      false,
	}
	reqBytes, _ := json.Marshal(chartReq)

	rawResp, err := chart.GetChartData(conn, userID, reqBytes)
	if err != nil {
		return nil, fmt.Errorf("error fetching chart data: %v", err)
	}
	resp, ok := rawResp.(chart.GetChartDataResponse)
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
	StrategyID int    `json:"strategyId,omitempty"`
}

func CreateStrategyFromNaturalLanguage(conn *data.Conn, userID int, rawArgs json.RawMessage) (any, error) {
	var args CreateStrategyFromNaturalLanguageArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	////fmt.Printf("INFO: Starting CreateStrategyFromNaturalLanguage for user %d with query: %q\n", userID, args.Query)

	apikey, err := conn.GetGeminiKey()
	if err != nil {
		////fmt.Printf("ERROR: Error getting Gemini key for user %d: %v\n", userID, err)
		return nil, fmt.Errorf("error getting gemini key: %v", err)
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apikey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating gemini client: %v", err)
	}

	systemInstruction, err := getSystemInstruction("spec")
	if err != nil {
		return nil, fmt.Errorf("error getting system instruction: %v", err)
	}
	thinkingBudget := int32(2000)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemInstruction},
			},
		},
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  &thinkingBudget,
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
		////fmt.Printf("Attempt %d/%d to generate and validate strategy spec...\n", attempt+1, maxRetries)

		// Generate content using the current conversation history
		result, err := client.Models.GenerateContent(context.Background(), "gemini-2.5-flash-preview-05-20", conversationHistory, config)
		if err != nil {
			lastErr = fmt.Errorf("error generating content (attempt %d): %w", attempt+1, err)
			////fmt.Printf("WARN: Attempt %d/%d for user %d: Gemini content generation failed: %v\n", attempt+1, maxRetries, userID, err)
			// Append error as user message for context? Maybe too noisy. Let's just retry.
			continue // Retry the API call
		}

		responseText := ""
		if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
			// Append assistant's response to history for the next turn
			conversationHistory = append(conversationHistory, result.Candidates[0].Content)
			for _, part := range result.Candidates[0].Content.Parts {
				if part.Thought {
					continue
				}
				if part.Text != "" {
					responseText = part.Text
					break
				}
			}
		} else {
			lastErr = fmt.Errorf("gemini returned no response candidates (attempt %d)", attempt+1)
			////fmt.Printf("WARN: Attempt %d/%d for user %d: Gemini returned no response candidates.\n", attempt+1, maxRetries, userID)
			// Append error message as user turn
			errorMsg := fmt.Sprintf("Attempt %d failed: Gemini returned no response. Please try again.", attempt+1)
			// Parts expects []*genai.Part, and we need to create a Part directly
			conversationHistory = append(conversationHistory, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: errorMsg}}})
			continue
		}

		////fmt.Printf("DEBUG: Attempt %d/%d for user %d: Raw Gemini response: %s\n", attempt+1, maxRetries, userID, responseText)

		// Extract JSON block
		jsonBlock := ""
		// Try extracting from ```json ... ``` block first
		jsonCodeBlockStart := strings.Index(responseText, "```json")
		if jsonCodeBlockStart != -1 {
			jsonCodeBlockStart += len("```json") // Move past the marker
			jsonCodeBlockEnd := strings.Index(responseText[jsonCodeBlockStart:], "```")
			if jsonCodeBlockEnd != -1 {
				// Found the closing ```
				jsonBlock = responseText[jsonCodeBlockStart : jsonCodeBlockStart+jsonCodeBlockEnd]
				////fmt.Printf("DEBUG: Attempt %d/%d for user %d: Extracted JSON from code block.\n", attempt+1, maxRetries, userID)
			}
		}

		// If not found in code block, fall back to searching for first { and last }
		if jsonBlock == "" {
			////fmt.Printf("DEBUG: Attempt %d/%d for user %d: JSON code block not found, falling back to {} search.\n", attempt+1, maxRetries, userID)
			jsonStartIdx := strings.Index(responseText, "{")
			jsonEndIdx := strings.LastIndex(responseText, "}")
			if jsonStartIdx != -1 && jsonEndIdx != -1 && jsonEndIdx > jsonStartIdx {
				jsonBlock = responseText[jsonStartIdx : jsonEndIdx+1]
			}
		}

		if jsonBlock == "" {
			lastErr = fmt.Errorf("no valid JSON block found in Gemini response (attempt %d): %s", attempt+1, responseText)
			////fmt.Printf("WARN: Attempt %d/%d for user %d: No valid JSON block found in Gemini response.\n", attempt+1, maxRetries, userID)
			// Append error message as user turn
			errorMsg := fmt.Sprintf("Attempt %d failed: Could not find a JSON block (neither in ```json ... ``` nor between '{' and '}') in the response. Please ensure the response contains a single, valid JSON object. The response received was:\n%s", attempt+1, responseText)
			// Parts expects []*genai.Part, and we need to create a Part directly
			conversationHistory = append(conversationHistory, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: errorMsg}}})
			continue
		}

		// Log the raw JSON block returned by Gemini
		////fmt.Printf("DEBUG: Attempt %d/%d for user %d: Extracted JSON block: \n%s\n", attempt+1, maxRetries, userID, jsonBlock)

		jsonBlock = strings.TrimSpace(jsonBlock)
		// Use the new helper function to unmarshal and validate the entire JSON block
		name, spec, err := UnmarshalAndValidateNewStrategyInput([]byte(jsonBlock))
		if err != nil {
			lastErr = fmt.Errorf("failed to unmarshal or validate Gemini JSON response (attempt %d): %w", attempt+1, err)
			////fmt.Printf("WARN: Attempt %d/%d for user %d: Failed to unmarshal/validate JSON: %v\n", attempt+1, maxRetries, userID, err)
			// Format error message for the next prompt
			errorMsg := fmt.Sprintf("Attempt %d failed: Could not parse or validate the JSON structure. Ensure the response is a single JSON object with 'name' (string) and 'spec' (object) fields, and that the spec is valid. Error: %v\nReceived JSON:\n%s\nPlease fix the JSON based on the error and the original query.", attempt+1, err, jsonBlock)
			// Parts expects []*genai.Part, and we need to create a Part directly
			conversationHistory = append(conversationHistory, &genai.Content{Role: "user", Parts: []*genai.Part{{Text: errorMsg}}})
			continue
		}

		////fmt.Printf("INFO: Attempt %d/%d for user %d: Successfully validated name '%s' and spec.\n", attempt+1, maxRetries, userID, name)

		// --- Compile the validated spec to SQL ---
		_, compileErr := CompileSpecToSQL(spec)
		if compileErr != nil {
			// Log the error but proceed with saving the strategy for now.
			// The user might want to fix the spec manually later.
			////fmt.Printf("ERROR: User %d: Failed to compile validated spec for strategy '%s' to SQL: %v\n", userID, name, compileErr)
			// Optionally, you could add this error info back into the conversation history
			// if you wanted Gemini to potentially fix the spec based on compilation failure.
			return nil, compileErr
		}
		// --- End SQL Compilation ---

		// If validation succeeds, create the strategy using the validated name and spec
		strategyID, err := _newStrategy(conn, userID, name, spec)
		if err != nil {
			// This is an internal error, not Gemini's fault, so return directly
			////fmt.Printf("ie20hifi0: %v\n", err)
			return nil, fmt.Errorf("error saving validated strategy: %w", err)
		}

		// Return the successful result
		return map[string]interface{}{
			"strategyId": strategyID,
			"name":       name,
			//"spec":       spec, // Return the validated spec object
		}, nil
	}

	// If loop finishes without success
	////fmt.Printf("ERROR: User %d: Failed to create valid strategy from query %q after %d attempts. Last error: %v\n", userID, args.Query, maxRetries, lastErr)
	return nil, fmt.Errorf("failed to create valid strategy after %d attempts: %w", maxRetries, lastErr)
}

type GetStrategySpecArgs struct {
	StrategyID int `json:"strategyId"`
}

func GetStrategySpec(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStrategySpecArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	return _getStrategySpec(conn, args.StrategyID, userID)
}

func _getStrategySpec(conn *data.Conn, strategyID int, userID int) (json.RawMessage, error) {
	var strategyCriteria json.RawMessage
	////fmt.Println(userID)
	err := conn.DB.QueryRow(context.Background(), `
    SELECT spec
    FROM strategies WHERE strategyId = $1 and userId = $2`, strategyID, userID).Scan(&strategyCriteria)
	if err != nil {
		return nil, err
	}

	return strategyCriteria, nil
}

// GetStrategies performs operations related to GetStrategies functionality.
func GetStrategies(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
    SELECT strategyId, name
    FROM strategies WHERE userId = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []Strategy
	for rows.Next() {
		var strategy Strategy

		if err := rows.Scan(&strategy.StrategyID, &strategy.Name); err != nil {
			return nil, fmt.Errorf("error scanning strategy: %v", err)
		}

		// Get the score from the studies table (if available)
		var score sql.NullInt32
		err := conn.DB.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM studies 
			WHERE userId = $1 AND strategyId = $2 AND completed = true`,
			userID, strategy.StrategyID).Scan(&score)

		if err == nil && score.Valid {
			strategy.Score = int(score.Int32)
		}

		strategies = append(strategies, strategy)
	}

	return strategies, nil
}

// No longer needed:
// type NewStrategyArgs struct { ... }

// _newStrategy saves a validated strategy spec to the database.
// It now takes userID, name, and the validated Spec object directly.
func _newStrategy(conn *data.Conn, userID int, name string, spec Spec) (int, error) {
	if name == "" {
		return -1, fmt.Errorf("strategy name cannot be empty")
	}
	// userID is assumed to be validated by the caller function's context

	// Convert the validated spec object back to JSON for database storage
	specJSON, err := json.Marshal(spec)
	if err != nil {
		// This should ideally not happen if the spec was correctly validated/constructed
		return -1, fmt.Errorf("internal error marshaling validated spec: %w", err)
	}

	var strategyID int
	// Ensure the userID from the function argument is used
	err = conn.DB.QueryRow(context.Background(), `
		INSERT INTO strategies (name, spec, userId)
		VALUES ($1, $2, $3) RETURNING strategyId`,
		name, specJSON, userID, // Use the passed userID
	).Scan(&strategyID)

	if err != nil {
		// Consider checking for specific DB errors (e.g., unique constraint violation) if needed
		return -1, fmt.Errorf("error inserting strategy into database: %w", err)
	}
	////fmt.Printf("Successfully created strategy with ID: %d for user %d\n", strategyID, userID)
	return strategyID, nil
}

// NewStrategy performs operations related to NewStrategy functionality.
// It expects a JSON object with "name" and "spec" fields.
func NewStrategy(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	// Use the new helper function to unmarshal and validate the input
	name, spec, err := UnmarshalAndValidateNewStrategyInput(rawArgs)
	if err != nil {
		// Error message from helper is already descriptive
		return nil, fmt.Errorf("invalid new strategy input: %w", err)
	}

	// Call _newStrategy with validated data
	strategyID, err := _newStrategy(conn, userID, name, spec)
	if err != nil {
		return nil, err // _newStrategy already formats the error
	}

	// Return the created strategy details using the main Strategy struct
	return Strategy{
		StrategyID: strategyID,
		UserID:     userID, // Reflect the correct user ID
		Name:       name,
		Spec:       spec, // Return the validated spec
		Score:      0,    // New strategy has no score yet
		// Other fields like CreationTimestamp, AlertActive etc., would be set by DB defaults or other logic
	}, nil
}

// DeleteStrategyArgs represents a structure for handling DeleteStrategyArgs data.
type DeleteStrategyArgs struct {
	StrategyID int `json:"strategyId"`
}

// DeleteStrategy performs operations related to DeleteStrategy functionality.
func DeleteStrategy(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}

	result, err := conn.DB.Exec(context.Background(), `
		DELETE FROM strategies 
		WHERE strategyId = $1 AND userId = $2`, args.StrategyID, userID)

	if err != nil {
		return nil, fmt.Errorf("error deleting strategy: %v", err)
	}

	// Check if any rows were affected
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("strategy not found or you don't have permission to delete it")
	}

	return nil, nil
}

// SetStrategyArgs represents a structure for handling SetStrategyArgs data.
// Note: We'll parse into the main Strategy struct for consistency.
// type SetStrategyArgs struct {
// 	StrategyID int          `json:"strategyId"`
// 	Name       string       `json:"name"`
// 	Spec   Spec `json:"spec"`
// }

// _setStrategy updates an existing strategy in the database after validation.
func _setStrategy(conn *data.Conn, userID int, strategyID int, name string, spec Spec) error {
	if name == "" {
		return fmt.Errorf("strategy name cannot be empty")
	}
	if strategyID <= 0 {
		return fmt.Errorf("invalid strategy ID")
	}

	// Convert the validated spec object back to JSON for database storage
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("internal error marshaling validated spec: %w", err)
	}

	// Update the strategy, ensuring the userID matches for authorization
	cmdTag, err := conn.DB.Exec(context.Background(), `
		UPDATE strategies
		SET name = $1, spec = $2
		WHERE strategyId = $3 AND userId = $4`,
		name, specJSON, strategyID, userID) // Use userID from context

	if err != nil {
		return fmt.Errorf("error updating strategy in database: %w", err)
	}
	if cmdTag.RowsAffected() != 1 {
		// This means either the strategyID didn't exist or it didn't belong to the user
		return fmt.Errorf("strategy not found or permission denied")
	}
	////fmt.Printf("Successfully updated strategy ID: %d for user %d\n", strategyID, userID)
	return nil
}

// SetStrategy performs operations related to SetStrategy functionality.
// It expects a JSON object containing the strategyID, new Name, and new Spec.
func SetStrategy(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	// Use the new helper function to unmarshal and validate the input
	strategyID, name, spec, err := UnmarshalAndValidateSetStrategyInput(rawArgs)
	if err != nil {
		// Error message from helper is already descriptive
		return nil, fmt.Errorf("invalid set strategy input: %w", err)
	}

	// Call _setStrategy with validated data and userID from context
	err = _setStrategy(conn, userID, strategyID, name, spec)
	if err != nil {
		return nil, err // _setStrategy already formats the error
	}

	// Return the updated strategy details using the main Strategy struct
	return Strategy{
		StrategyID: strategyID,
		UserID:     userID, // Reflect the correct user ID
		Name:       name,
		Spec:       spec, // Return the validated spec
		// Score is not updated here, would need separate logic/query if needed
	}, nil
}

// getSystemInstruction returns the processed prompt named <name>.txt
func getSystemInstruction(name string) (string, error) {
	raw, err := fs.ReadFile("prompts/" + name + ".txt")
	if err != nil {
		return "", fmt.Errorf("reading prompt: %w", err)
	}
	now := time.Now()
	s := strings.ReplaceAll(string(raw), "{{CURRENT_TIME}}",
		now.Format(time.RFC3339))
	s = strings.ReplaceAll(s, "{{CURRENT_TIME_MILLISECONDS}}",
		strconv.FormatInt(now.UnixMilli(), 10))
	return s, nil
}
