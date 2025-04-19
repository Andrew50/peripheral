package tools

import (
	"backend/utils"
	"context"
	"database/sql"
	"encoding/json"
    "errors"
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


// ---------- ENUMS -----------------------------------------------------------

// Arithmetic, comparison, and aggregate operators.
type ArithOp   string
type CompOp    string
type AggFn     string
type RankFn    string
type LogicOp   string

const (
	ArithAdd ArithOp = "+"
	ArithSub          = "-"
	ArithMul          = "*"
	ArithDiv          = "/"
	ArithOffset       = "offset" // arg[1] = k (const)

	CompEQ CompOp = "=="
	CompNE         = "!="
	CompLT         = "<"
	CompLE         = "<="
	CompGT         = ">"
	CompGE         = ">="

	AggAvg   AggFn = "avg"
	AggStd          = "stdev"
	AggMedian       = "median"
	// (add Sum/Min/Max if needed)

	RankTopPct   RankFn = "top_pct"
	RankBottomPct        = "bottom_pct"
	RankTopN             = "top_n"
	RankBottomN          = "bottom_n"

	LogicAnd LogicOp = "AND"
	LogicOr           = "OR"
	LogicNot          = "NOT"
)

// ---------- VALUE NODES -----------------------------------------------------

// ValueKind tells the decoder which concrete struct to unmarshal into.
type ValueKind string

const (
	ValConst  ValueKind = "const"
	ValCol               = "column"
	ValExpr              = "expr"
	ValAgg               = "agg"
)

// Value is a discriminated union.  Each concrete type embeds it so that the
// "kind" field round‑trips through JSON.
type Value struct {
	Kind ValueKind `json:"kind"`
}

// ----- scalar literals ------------------------------------------------------

type Const struct {
	Value
	Number float64 `json:"number"`         // or String if you need it
}

// ----- raw column -----------------------------------------------------------

type Column struct {
	Value
	Name string `json:"name"`              // e.g. "close"
}

// ----- arithmetic / offset --------------------------------------------------

type Expr struct {
	Value
	Op   ArithOp   `json:"op"`             // "+", "-", "*", "/", "offset"
	Args []NodeID  `json:"args"`           // operands (indices into Spec.Nodes)
}

// ----- aggregation (avg, stdev, …) -----------------------------------------

type Aggregate struct {
	Value
	Fn      AggFn   `json:"fn"`            // "avg", "stdev", "median"
	Of      NodeID  `json:"of"`            // series being reduced
	Scope   string  `json:"scope,omitempty"`  // "self", "sector", "market", peers…
	Period  int     `json:"period,omitempty"` // rolling window in bars; 0 = full
}

// ---------- BOOLEAN NODES ---------------------------------------------------

type Comparison struct {
	Op  CompOp `json:"op"`   // ">", "<=", …
	LHS NodeID `json:"lhs"`  // scalar
	RHS NodeID `json:"rhs"`  // scalar
}

type RankFilter struct {
	Fn    RankFn `json:"fn"`    // "top_pct", "top_n", …
	Expr  NodeID `json:"expr"`  // series to rank
	Param int    `json:"param"` // 10 → top‑10 % or top‑10 rows
}

type Logic struct {
	Op   LogicOp `json:"op"`    // AND / OR / NOT
	Args []NodeID `json:"args"` // boolean children
}

// NodeID is just an int index into Spec.Nodes, so references stay light‑weight
// and graphs are easy to manipulate.
type NodeID int

// ---------- ROOT SPEC -------------------------------------------------------

type StrategySpec struct {
	Name  string        `json:"name"`
	Nodes []any         `json:"nodes"` // slice of *Const, *Column, *Expr, *Aggregate,
	                                  // *Comparison, *RankFilter, *Logic
	Root  NodeID        `json:"root"`  // the final boolean node to evaluate
}



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
		"analysis": analysis,         // Gemini’s narrative
	//	"bars":     json.RawMessage(barsJSON),
	//	"chart":    pngB64,           // base‑64 PNG for client preview
	}, nil
}








type CreateStrategyFromNaturalLanguageArgs struct {
	Query      string `json:"query"`
	StrategyId int    `json:"strategyId,omitempty"`
}

type CreateStrategyFromNaturalLanguageResult struct {
    StrategySpec    StrategySpec    `json:"strategySpec"`
    StrategyId  int     `json"stategyId"`
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
    id, err :=  _newStrategy(conn, userId, name, spec) // bandaid
    return CreateStrategyFromNaturalLanguageResult {
        StrategySpec: spec,
        StrategyId: id,
    }, nil
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


func ValidateSpec(s *StrategySpec) error {
	if len(s.Nodes) == 0 {
		return errors.New("spec has no nodes")
	}

	// --- helpers ------------------------------------------------------------

	checkRef := func(id NodeID, cur, total int) error {
		idx := int(id)
		if idx < 0 || idx >= total {
			return fmt.Errorf("reference to nonexistent node %d", idx)
		}
		if idx >= cur {
			return fmt.Errorf("node %d references future node %d", cur, idx)
		}
		return nil
	}

	// allowed look‑ups
	allowedCols := map[string]struct{}{
		"timestamp": {}, "securityid": {}, "ticker": {}, "open": {}, "high": {},
		"low": {}, "close": {}, "volume": {}, "vwap": {}, "transactions": {},
		"market_cap": {}, "share_class_shares_outstanding": {},
	}

	allowedArith := map[ArithOp]struct{}{
		ArithAdd: {}, ArithSub: {}, ArithMul: {}, ArithDiv: {}, ArithOffset: {},
	}
	allowedComp := map[CompOp]struct{}{
		CompEQ: {}, CompNE: {}, CompLT: {}, CompLE: {}, CompGT: {}, CompGE: {},
	}
	allowedAgg := map[AggFn]struct{}{AggAvg: {}, AggStd: {}, AggMedian: {}}
	allowedRank := map[RankFn]struct{}{
		RankTopPct: {}, RankBottomPct: {}, RankTopN: {}, RankBottomN: {},
	}
	allowedLogic := map[LogicOp]struct{}{LogicAnd: {}, LogicOr: {}, LogicNot: {}}

	// --- root validation ----------------------------------------------------

	if int(s.Root) < 0 || int(s.Root) >= len(s.Nodes) {
		return fmt.Errorf("root index %d out of range", s.Root)
	}
	switch s.Nodes[s.Root].(type) {
	case *Comparison, *RankFilter, *Logic:
		// ok
	default:
		return errors.New("root must reference a boolean‑returning node")
	}

	// --- node‑by‑node checks ------------------------------------------------

	for i, n := range s.Nodes {
		switch node := n.(type) {

		case *Const:
			// nothing extra to validate

		case *Column:
			if _, ok := allowedCols[node.Name]; !ok {
				return fmt.Errorf("node %d: unknown column %q", i, node.Name)
			}

		case *Expr:
			if _, ok := allowedArith[node.Op]; !ok {
				return fmt.Errorf("node %d: invalid arithmetic op %q", i, node.Op)
			}
			if node.Op == ArithOffset && len(node.Args) != 2 {
				return fmt.Errorf("node %d: offset requires exactly 2 args", i)
			}
			if node.Op != ArithOffset && len(node.Args) < 2 {
				return fmt.Errorf("node %d: expr needs ≥2 args", i)
			}
			for _, id := range node.Args {
				if err := checkRef(id, i, len(s.Nodes)); err != nil {
					return err
				}
			}

		case *Aggregate:
			if _, ok := allowedAgg[node.Fn]; !ok {
				return fmt.Errorf("node %d: invalid aggregate fn %q", i, node.Fn)
			}
			if err := checkRef(node.Of, i, len(s.Nodes)); err != nil {
				return err
			}
			if node.Period < 0 {
				return fmt.Errorf("node %d: period cannot be negative", i)
			}

		case *Comparison:
			if _, ok := allowedComp[node.Op]; !ok {
				return fmt.Errorf("node %d: invalid comparison op %q", i, node.Op)
			}
			if err := checkRef(node.LHS, i, len(s.Nodes)); err != nil {
				return err
			}
			if err := checkRef(node.RHS, i, len(s.Nodes)); err != nil {
				return err
			}

		case *RankFilter:
			if _, ok := allowedRank[node.Fn]; !ok {
				return fmt.Errorf("node %d: invalid rank fn %q", i, node.Fn)
			}
			if node.Param <= 0 {
				return fmt.Errorf("node %d: param must be > 0", i)
			}
			if err := checkRef(node.Expr, i, len(s.Nodes)); err != nil {
				return err
			}

		case *Logic:
			if _, ok := allowedLogic[node.Op]; !ok {
				return fmt.Errorf("node %d: invalid logic op %q", i, node.Op)
			}
			if node.Op == LogicNot && len(node.Args) != 1 {
				return fmt.Errorf("node %d: NOT expects exactly 1 arg", i)
			}
			if node.Op != LogicNot && len(node.Args) < 2 {
				return fmt.Errorf("node %d: %s expects ≥2 args", i, node.Op)
			}
			for _, id := range node.Args {
				if err := checkRef(id, i, len(s.Nodes)); err != nil {
					return err
				}
			}

		default:
			return fmt.Errorf("node %d: unknown node type %T", i, n)
		}
	}

	return nil
}

type NewStrategyArgs struct {
	Name     string       `json:"name"`
	Criteria StrategySpec `json:"criteria"`
}

func _newStrategy(conn *utils.Conn, userId int, name string, spec StrategySpec) (int, error) {
	if name == "" {
		return -1, fmt.Errorf("missing required fields")
	}

    err := ValidateSpec(&spec)
    if err != nil {
        return -1, err
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
