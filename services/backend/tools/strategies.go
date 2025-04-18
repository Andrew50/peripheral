package tools

import (
	"backend/utils"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/genai"
	"bytes"
	"encoding/base64"
     "github.com/pplcc/plotext/custplotter"
    "gonum.org/v1/plot"
    //"gonum.org/v1/plot/vg"

	//"github.com/wcharczuk/go-chart/v2"
)

type StrategySpec struct {
	Timeframes []string `json:"timeframes"`
	Stocks     struct {
		Universe string   `json:"universe"`
		Include  []string `json:"include"`
		Exclude  []string `json:"exclude"`
		Filters  []struct {
			Metric    string  `json:"metric"`
			Operator  string  `json:"operator"`
			Value     float64 `json:"value"`
			Timeframe string  `json:"timeframe"`
		} `json:"filters"`
	} `json:"stocks"`
	Indicators []struct {
		ID         string                 `json:"id"`
		Type       string                 `json:"type"`
		Parameters map[string]interface{} `json:"parameters"`
		InputField string                 `json:"input_field"`
		Timeframe  string                 `json:"timeframe"`
	} `json:"indicators"`
	DerivedColumns []struct {
		ID         string `json:"id"`
		Expression string `json:"expression"`
		Comment    string `json:"comment,omitempty"`
	} `json:"derived_columns,omitempty"`
	FuturePerformance []struct {
		ID         string `json:"id"`
		Expression string `json:"expression"`
		Timeframe  string `json:"timeframe"`
		Comment    string `json:"comment,omitempty"`
	} `json:"future_performance,omitempty"`
	Conditions []struct {
		ID  string `json:"id"`
		LHS struct {
			Field     string `json:"field"`
			Offset    int    `json:"offset"`
			Timeframe string `json:"timeframe"`
		} `json:"lhs"`
		Operation string `json:"operation"`
		RHS       struct {
			Field       string  `json:"field,omitempty"`
			Offset      int     `json:"offset,omitempty"`
			Timeframe   string  `json:"timeframe,omitempty"`
			IndicatorID string  `json:"indicator_id,omitempty"`
			Value       float64 `json:"value,omitempty"`
			Multiplier  float64 `json:"multiplier,omitempty"`
		} `json:"rhs"`
	} `json:"conditions"`
	Logic     string `json:"logic"`
	DateRange struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"date_range"`
	TimeOfDay struct {
		Constraint string `json:"constraint"`
		StartTime  string `json:"start_time"`
		EndTime    string `json:"end_time"`
	} `json:"time_of_day"`
	OutputColumns []string `json:"output_columns"`
}



/*─────────────── Call‑side JSON args ───────────────*/
// AnalyzeInstanceFeaturesArgs contains parameters for analyzing features of a specific security instance
type AnalyzeInstanceFeaturesArgs struct {
	SecurityID int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"` // Unix ms of reference bar (0 ⇒ “now”)
	Timeframe  string `json:"timeframe"` // e.g. "15m", "h", "d"
	Bars       int    `json:"bars"`      // # of candles to pull **backward** from timestamp
}

/*────────── Main entrypoint exposed to Gemini ─────────*/
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

p.HideY()                       // optional cosmetics
p.X.Tick.Marker = plot.TimeTicks{Format: "01‑02\n15:04"}

// add candlesticks
candles, err := custplotter.NewCandlesticks(bars)
if err != nil { return nil, fmt.Errorf("candles: %w", err) }
p.Add(candles)

// render to an in‑memory PNG
var png bytes.Buffer
wt, err := p.WriterTo(600, 300, "png") // width, height, format
if err != nil { return nil, fmt.Errorf("writer: %w", err) }
if _, err = wt.WriteTo(&png); err != nil {
    return nil, fmt.Errorf("render: %w", err)
}
pngB64 := base64.StdEncoding.EncodeToString(png.Bytes())

	/* 4. Build Gemini prompt */
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

	/* 5. Call Gemini */
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
		return nil, fmt.Errorf("Gemini call failed: %v", err)
	}

	/* 6. Extract Gemini’s textual answer */
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
		"analysis": analysis,         // Gemini’s narrative
	//	"bars":     json.RawMessage(barsJSON),
	//	"chart":    pngB64,           // base‑64 PNG for client preview
	}, nil
}








type CreateStrategyFromNaturalLanguageArgs struct {
	Query      string `json:"query"`
	StrategyId int    `json:"strategyId,omitempty"`
}

func extractName(resp string, jsonEnd int) (string, bool) {
	// Slice the response starting *after* the last `}`
	if jsonEnd < 0 || jsonEnd+1 >= len(resp) {
		return "", false
	}
	afterJSON := resp[jsonEnd+1:]

	// Regular expression: beginning of line, optional back‑ticks or code‑block fences,
	// then "NAME:", then capture anything until EOL.
	re := regexp.MustCompile(`(?m)^\s*NAME:\s*(.+?)\s*$`)
	if m := re.FindStringSubmatch(afterJSON); len(m) == 2 {
		return strings.TrimSpace(m[1]), true
	}
	return "", false
}

func CreateStrategyFromNaturalLanguage(conn *utils.Conn, userId int, rawArgs json.RawMessage) (any, error) {
	var args CreateStrategyFromNaturalLanguageArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	fmt.Printf("Running backtest with query: %s\n", args.Query)

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
	result, err := client.Models.GenerateContent(context.Background(), "gemini-2.0-flash-thinking-exp-01-21", genai.Text(args.Query), config)
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
	jsonStartIdx := strings.Index(responseText, "{")
	jsonEndIdx := strings.LastIndex(responseText, "}")

	jsonBlock := responseText[jsonStartIdx : jsonEndIdx+1]

	if !strings.Contains(jsonBlock, "{") || !strings.Contains(jsonBlock, "}") {
		return nil, fmt.Errorf("no valid JSON found in Gemini response: %s", jsonBlock)
	}

	//TODO return to gemini on faillure to verify and fix the format in a loop here???

	// Pretty print the JSON spec for better readability
	prettyJSON, err := prettyPrintJSON(jsonBlock)
	if err != nil {
		fmt.Printf("Warning: Could not pretty print JSON (using raw): %v\n", err)
		fmt.Println("Gemini returned backtest JSON: ", jsonBlock)
	} else {
		fmt.Println("Gemini returned backtest JSON: \n", prettyJSON)
	}

	var spec StrategySpec
	if err := json.Unmarshal(([]byte(jsonBlock)), &spec); err != nil { //unmarhsal into struct
		return "", fmt.Errorf("ERR 01v: error parsing backtest JSON: %v", err)
	}

	name, ok := extractName(responseText, jsonEndIdx)
	if !ok || name == "" {
		name = "UntitledStrategy" // fallback or return error, your choice
	}

	//if args.StrategyId < 0 { // if it wants new then it passes strat id of -1
	return _newStrategy(conn, userId, name, spec) // bandaid
	//}else {
	//return args.StrategyId, _setStrategy(conn,userId,args.StrategyId,name,spec)
	//}
}

// StrategyResult represents a strategy configuration with its evaluation score.
type StrategyResult struct {
	StrategyID int          `json:"strategyId"`
	Name       string       `json:"name"`
	Criteria   StrategySpec `json:"criteria"`
	Score      int          `json:"score"`
}

type GetStrategySpecArgs struct {
	StrategyId int `json:"strategyId"`
}

func GetStrategySpec(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetStrategySpecArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	return _getStrategySpec(conn, args.StrategyId, userId)
}

func _getStrategySpec(conn *utils.Conn, userId int, strategyId int) (json.RawMessage, error) {
	var strategyCriteria json.RawMessage
	fmt.Println(userId)
	err := conn.DB.QueryRow(context.Background(), `
    SELECT criteria
    FROM strategies WHERE strategyId = $1`, strategyId).Scan(&strategyCriteria)
	//TODO add user id check back
	if err != nil {
		return nil, err
	}

	return strategyCriteria, nil
}

// GetStrategies performs operations related to GetStrategies functionality.
func GetStrategies(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), `
    SELECT strategyId, name, criteria
    FROM strategies WHERE userId = $1`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []StrategyResult
	for rows.Next() {
		var strategy StrategyResult
		var criteriaJSON json.RawMessage

		if err := rows.Scan(&strategy.StrategyID, &strategy.Name, &criteriaJSON); err != nil {
			return nil, fmt.Errorf("error scanning strategy: %v", err)
		}

		// Parse the criteria JSON
		if err := json.Unmarshal(criteriaJSON, &strategy.Criteria); err != nil {
			return nil, fmt.Errorf("error parsing criteria JSON: %v", err)
		}

		// Get the score from the studies table (if available)
		var score sql.NullInt32
		err := conn.DB.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM studies 
			WHERE userId = $1 AND strategyId = $2 AND completed = true`,
			userId, strategy.StrategyID).Scan(&score)

		if err == nil && score.Valid {
			strategy.Score = int(score.Int32)
		}

		strategies = append(strategies, strategy)
	}

	return strategies, nil
}

// NewStrategyArgs represents a structure for handling NewStrategyArgs data.
type NewStrategyArgs struct {
	Name     string       `json:"name"`
	Criteria StrategySpec `json:"criteria"`
}

func _newStrategy(conn *utils.Conn, userId int, name string, spec StrategySpec) (int, error) {
	if name == "" {
		return -1, fmt.Errorf("missing required fields")
	}

	// Convert criteria to JSON
	criteriaJSON, err := json.Marshal(spec)
	if err != nil {
		return -1, fmt.Errorf("error marshaling criteria: %v", err)
	}

	var strategyID int
	err = conn.DB.QueryRow(context.Background(), `
		INSERT INTO strategies (name, criteria, userId) 
		VALUES ($1, $2, $3) RETURNING strategyId`,
		name, criteriaJSON, userId,
	).Scan(&strategyID)

	if err != nil {
		return -1, fmt.Errorf("error creating strategy: %v", err)
	}
	return strategyID, nil

}

// NewStrategy performs operations related to NewStrategy functionality.
func NewStrategy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}

	strategyId, err := _newStrategy(conn, userId, args.Name, args.Criteria)
	if err != nil {
		return nil, err
	}

	return StrategyResult{
		StrategyID: strategyId,
		Name:       args.Name,
		Criteria:   args.Criteria,
		Score:      0, // New strategy has no score yet
	}, nil
}

// DeleteStrategyArgs represents a structure for handling DeleteStrategyArgs data.
type DeleteStrategyArgs struct {
	StrategyID int `json:"strategyId"`
}

// DeleteStrategy performs operations related to DeleteStrategy functionality.
func DeleteStrategy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}

	result, err := conn.DB.Exec(context.Background(), `
		DELETE FROM strategies 
		WHERE strategyId = $1 AND userId = $2`, args.StrategyID, userId)

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
type SetStrategyArgs struct {
	StrategyID int          `json:"strategyId"`
	Name       string       `json:"name"`
	Criteria   StrategySpec `json:"criteria"`
}

func _setStrategy(conn *utils.Conn, userId int, strategyId int, name string, spec StrategySpec) error {
	if name == "" {
		return fmt.Errorf("missing required field name")
	}

	// Convert criteria to JSON
	criteriaJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("error marshaling criteria: %v", err)
	}

	cmdTag, err := conn.DB.Exec(context.Background(), `
		UPDATE strategies 
		SET name = $1, criteria = $2
		WHERE strategyId = $3 AND userId = $4`,
		name, criteriaJSON, strategyId, userId)

	if err != nil {
		return fmt.Errorf("error updating strategy: %v", err)
	} else if cmdTag.RowsAffected() != 1 {
		return fmt.Errorf("strategy not found or you don't have permission to update it")
	}
	return nil
}

// SetStrategy performs operations related to SetStrategy functionality.
func SetStrategy(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SetStrategyArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
	err := _setStrategy(conn, userId, args.StrategyID, args.Name, args.Criteria)
	if err != nil {
		return nil, err
	}
	return StrategyResult{
		StrategyID: args.StrategyID,
		Name:       args.Name,
		Criteria:   args.Criteria,
		Score:      0, // We don't have the score here, it would need to be queried separately
	}, nil
}
