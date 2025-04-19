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

/*
   Nested‑tree Strategy Spec (v2)
   ------------------------------
   A strategy is a JSON object of the form:

     {
       "name": "Label",
       "rule": <BoolNode>
     }

   Where <BoolNode> is one of:
     • Comparison  {"cmp":{"op":"<","lhs":<Value>,"rhs":<Value>}}
     • Rank        {"rank":{"fn":"top_pct","expr":<Value>,"param":10}}
     • Logic       {"logic":{"op":"AND","args":[<BoolNode>,…]}}

   <Value> is one of:
     • Const   {"const":1.23}
     • Column  {"column":"close"}
     • Expr    {"expr":{"op":"*","args":[<Value>,<Value>]}}
     • Agg     {"agg":{"fn":"avg","of":<Value>,"period":50}}

   Validation guarantees:
     1. `rule` is a boolean node (cmp / rank / logic).
     2. All enums, column names, numeric constraints are valid.
     3. Numeric literals appear only in `const` nodes.
*/


// ───────────────────── Enumerations ────────────────────────

type ArithOp string
const (
    ArithAdd    ArithOp = "+"
    ArithSub            = "-"
    ArithMul            = "*"
    ArithDiv            = "/"
    ArithOffset         = "offset"
)

type AggFn string
const (
    AggAvg   AggFn = "avg"
    AggStd          = "stdev"
    AggMedian       = "median"
)

type CompOp string
const (
    CompEQ CompOp = "=="
    CompNE         = "!="
    CompLT         = "<"
    CompLE         = "<="
    CompGT         = ">"
    CompGE         = ">="
)

type RankFn string
const (
    RankTopPct    RankFn = "top_pct"
    RankBottomPct        = "bottom_pct"
    RankTopN             = "top_n"
    RankBottomN          = "bottom_n"
)

type LogicOp string
const (
    LogicAnd LogicOp = "AND"
    LogicOr          = "OR"
    LogicNot         = "NOT"
)

// ─────────────────── Top‑level Spec struct ──────────────────

type StrategySpec struct {
    Name string          `json:"name"`
    Rule json.RawMessage `json:"rule"`
}

// ───────────────────── Validation API ───────────────────────

var (
    allowedCols = map[string]struct{}{
        "timestamp":{}, "securityid":{}, "ticker":{}, "open":{}, "high":{},
        "low":{}, "close":{}, "volume":{}, "vwap":{}, "transactions":{},
        "market_cap":{}, "share_class_shares_outstanding":{},
    }
    allowedArith = map[string]struct{}{string(ArithAdd):{},string(ArithSub):{},string(ArithMul):{},string(ArithDiv):{},string(ArithOffset):{}}
    allowedAgg   = map[string]struct{}{string(AggAvg):{},string(AggStd):{},string(AggMedian):{}}
    allowedComp  = map[string]struct{}{string(CompEQ):{},string(CompNE):{},string(CompLT):{},string(CompLE):{},string(CompGT):{},string(CompGE):{}}
    allowedRank  = map[string]struct{}{string(RankTopPct):{},string(RankBottomPct):{},string(RankTopN):{},string(RankBottomN):{}}
    allowedLogic = map[string]struct{}{string(LogicAnd):{},string(LogicOr):{},string(LogicNot):{}}
)

// ValidateSpec walks a StrategySpec and returns an error if it violates
// any format rule or hard constraint.
func ValidateSpec(s *StrategySpec) error {
    if s.Rule == nil {
        return errors.New("missing rule root")
    }
    var root interface{}
    if err := json.Unmarshal(s.Rule, &root); err != nil {
        return fmt.Errorf("rule is not valid JSON: %v", err)
    }
    if err := validateBoolNode(root); err != nil {
        return fmt.Errorf("rule: %w", err)
    }
    return nil
}

// ───────────────────── helpers ──────────────────────────────

// validateBoolNode checks cmp / rank / logic structures
func validateBoolNode(node interface{}) error {
    m, ok := node.(map[string]interface{})
    if !ok {
        return errors.New("boolean node must be an object")
    }
    switch {
    case m["cmp"] != nil:
        cmp, ok := m["cmp"].(map[string]interface{})
        if !ok { return errors.New("cmp must be object") }
        op, ok := cmp["op"].(string)
        if !ok || !hasKey(allowedComp, op) {
            return fmt.Errorf("cmp: invalid op %v", cmp["op"])
        }
        if err := validateValueNode(cmp["lhs"]); err != nil { return fmt.Errorf("cmp.lhs: %w", err) }
        if err := validateValueNode(cmp["rhs"]); err != nil { return fmt.Errorf("cmp.rhs: %w", err) }
        return nil

    case m["rank"] != nil:
        r, ok := m["rank"].(map[string]interface{})
        if !ok { return errors.New("rank must be object") }
        fn, ok := r["fn"].(string)
        if !ok || !hasKey(allowedRank, fn) {
            return fmt.Errorf("rank: invalid fn %v", r["fn"])
        }
        if param, ok := r["param"].(float64); !ok || param <= 0 || param != float64(int(param)) {
            return errors.New("rank.param must be positive int")
        }
        if err := validateValueNode(r["expr"]); err != nil { return fmt.Errorf("rank.expr: %w", err) }
        return nil

    case m["logic"] != nil:
        l, ok := m["logic"].(map[string]interface{})
        if !ok { return errors.New("logic must be object") }
        op, ok := l["op"].(string)
        if !ok || !hasKey(allowedLogic, op) {
            return fmt.Errorf("logic: invalid op %v", l["op"])
        }
        args, ok := l["args"].([]interface{})
        if !ok || len(args) == 0 {
            return errors.New("logic.args must be non‑empty array")
        }
        if op == string(LogicNot) && len(args) != 1 {
            return errors.New("logic NOT requires exactly one arg")
        }
        if op != string(LogicNot) && len(args) < 2 {
            return errors.New("logic AND/OR require ≥2 args")
        }
        for i, a := range args {
            if err := validateBoolNode(a); err != nil {
                return fmt.Errorf("logic arg %d: %w", i, err)
            }
        }
        return nil
    default:
        return errors.New("boolean node must contain cmp, rank, or logic key")
    }
}

// validateValueNode checks const / column / expr / agg
func validateValueNode(node interface{}) error {
    m, okObj := node.(map[string]interface{})
    if !okObj {
        return errors.New("value node must be object")
    }

    switch {
    case m["const"] != nil:
        _, ok := m["const"].(float64)
        if !ok {
            return errors.New("const must be number")
        }
        return nil

    case m["column"] != nil:
        col, ok := m["column"].(string)
        if !ok || !hasKey(allowedCols, col) {
            return fmt.Errorf("invalid column %v", m["column"])
        }
        return nil

    case m["expr"] != nil:
        e, ok := m["expr"].(map[string]interface{})
        if !ok { return errors.New("expr must be object") }
        op, ok := e["op"].(string)
        if !ok || !hasKey(allowedArith, op) {
            return fmt.Errorf("expr: invalid op %v", e["op"])
        }
        args, ok := e["args"].([]interface{})
        if !ok || len(args) < 2 {
            return errors.New("expr.args must be array with ≥2 items")
        }
        if op == string(ArithOffset) && len(args) != 2 {
            return errors.New("offset requires exactly 2 args")
        }
        for i, a := range args {
            if err := validateValueNode(a); err != nil {
                return fmt.Errorf("expr arg %d: %w", i, err)
            }
        }
        return nil

    case m["agg"] != nil:
        a, ok := m["agg"].(map[string]interface{})
        if !ok { return errors.New("agg must be object") }
        fn, ok := a["fn"].(string)
        if !ok || !hasKey(allowedAgg, fn) {
            return fmt.Errorf("agg: invalid fn %v", a["fn"])
        }
        if err := validateValueNode(a["of"]); err != nil {
            return fmt.Errorf("agg.of: %w", err)
        }
        if p, ok := a["period"]; ok {
            if v, ok2 := p.(float64); !ok2 || v < 0 || v != float64(int(v)) {
                return errors.New("agg.period must be non‑negative int")
            }
        }
        return nil
    default:
        return errors.New("value node must contain const, column, expr, or agg key")
    }
}

func hasKey(m map[string]struct{}, k string) bool { _, ok := m[k]; return ok }



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

func isLogicOp(op string) bool {
	switch LogicOp(strings.ToUpper(op)) {
	case LogicAnd, LogicOr, LogicNot:
		return true
	default:
		return false
	}
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
    if err := json.Unmarshal([]byte(jsonBlock), &spec); err != nil {
		return "", fmt.Errorf("ERR 01v: error parsing backtest JSON: %v", err)
    }

	name, ok := extractName(responseText, jsonEndIdx)
	if !ok || name == "" {
		name = "UntitledStrategy" // fallback or return error, your choice
	}

	//if args.StrategyId < 0 { // if it wants new then it passes strat id of -1
    id, err :=  _newStrategy(conn, userId, name, spec) // bandaid
    if err != nil {
        return nil, err
    }
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

// helper at top of file
func isCompOp(op string) bool {
    switch CompOp(op) {
    case CompEQ, CompNE, CompLT, CompLE, CompGT, CompGE:
        return true
    default:
        return false
    }
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
        fmt.Printf("SPEC VLAIDATION FAILED ---------------- %v",err)
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
