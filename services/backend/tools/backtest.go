package tools


/*
   strategy_sql.go – nested‑tree → SQL compiler (clean build)
   ----------------------------------------------------------
   Compiles the **v2 nested StrategySpec** into a single SQL query that selects
   all rows from `daily_ohlcv` where the boolean `rule` evaluates to TRUE.

   • Supports arithmetic (+ - * / offset), aggregates (avg stdev median),
     comparisons, and logic (AND OR NOT).
   • Rank filters are rejected with a clear error (not implemented yet).
   • Window functions partition by ticker and order by timestamp.
*/

import (
    "backend/utils"
    "context"
    "encoding/json"
    "fmt"
    "strconv"
    "strings"
)
// ---------------------------------------------------------------------------
// SQL compilation – recursive walk
// ---------------------------------------------------------------------------

// builder tracks computed columns that must go in the CTE
type builder struct {
    next   int               // n0, n1, n2 …
    ctes   []string          // `expr AS nX`
}

func buildSQL(spec *StrategySpec) (string, error) {
    // ── parse root ──────────────────────────────────────────────────────────
    var root any
    if err := json.Unmarshal(spec.Rule, &root); err != nil {
        return "", err
    }

    b := &builder{}
    pred, err := b.compileBool(root)
    if err != nil {
        return "", err
    }

    // ── assemble SQL ───────────────────────────────────────────────────────
    cteCols := ""
    if len(b.ctes) > 0 {
        cteCols = ",\n        " + strings.Join(b.ctes, ",\n        ")
    }

    sql := fmt.Sprintf(`
WITH calc AS (
    SELECT
        s.ticker,
        d.*%s
    FROM securities s
    JOIN daily_ohlcv d ON d.securityid = s.securityid
)
SELECT *
FROM calc
WHERE %s
ORDER BY ticker, timestamp;`, cteCols, pred)

    return sql, nil
}

/*──────────────────────── BOOLEAN NODES ──────────────────────────*/

func (b *builder) compileBool(node any) (string, error) {
    m, ok := node.(map[string]any)
    if !ok { return "", fmt.Errorf("bool node not object") }

    switch {
    case m["cmp"]   != nil: return b.compileCmp(m["cmp"])
    case m["logic"] != nil: return b.compileLogic(m["logic"])
    case m["rank"]  != nil: return "", fmt.Errorf("rank filters not implemented yet")
    default:
        return "", fmt.Errorf("unknown boolean node")
    }
}

func (b *builder) compileCmp(raw any) (string, error) {
    c := raw.(map[string]any)
    lhs, err := b.compileVal(c["lhs"]); if err != nil { return "", err }
    rhs, err := b.compileVal(c["rhs"]); if err != nil { return "", err }
    return fmt.Sprintf("(%s %s %s)", lhs, mapComp(c["op"].(string)), rhs), nil
}

func (b *builder) compileLogic(raw any) (string, error) {
    l := raw.(map[string]any)
    op   := l["op"].(string)
    args := l["args"].([]any)

    if op == "NOT" && len(args) != 1      { return "", fmt.Errorf("NOT needs 1 arg") }
    if (op == "AND" || op == "OR") && len(args) < 2 {
        return "", fmt.Errorf("%s needs ≥2 args", op)
    }

    parts := make([]string, len(args))
    for i, a := range args {
        p, err := b.compileBool(a); if err != nil { return "", err }
        parts[i] = p
    }
    if op == "NOT" { return "NOT (" + parts[0] + ")", nil }
    return "(" + strings.Join(parts, " "+op+" ") + ")", nil
}

/*──────────────────────── VALUE NODES ────────────────────────────*/

func (b *builder) compileVal(node any) (string, error) {
    m := node.(map[string]any)

    switch {
    case m["const"]  != nil:
        return strconv.FormatFloat(m["const"].(float64), 'f', -1, 64), nil

    case m["column"] != nil:
        return "d." + pqQuoteIdent(m["column"].(string)), nil

    case m["expr"]   != nil:
        return b.compileExpr(m["expr"])

    case m["agg"]    != nil:
        // aggregates are always window functions ➜ must be aliased
        win, err := b.compileAgg(m["agg"]); if err != nil { return "", err }
        return b.alias(win), nil
    }
    return "", fmt.Errorf("unknown value node")
}

func (b *builder) compileExpr(raw any) (string, error) {
    e   := raw.(map[string]any)
    op  := e["op"].(string)
    args:= e["args"].([]any)

    if len(args) < 2 { return "", fmt.Errorf("expr needs ≥2 args") }

    // compile children first
    parts := make([]string, len(args))
    for i, a := range args {
        p, err := b.compileVal(a); if err != nil { return "", err }
        parts[i] = p
    }

    switch op {
    case "+", "-", "*":
        return "(" + strings.Join(parts, " "+op+" ") + ")", nil

    case "/":
        if len(parts) != 2 { return "", fmt.Errorf("/ needs 2 args") }
        n, d := parts[0], parts[1]
        return fmt.Sprintf("CASE WHEN %s = 0 THEN NULL ELSE %s/%s END", d, n, d), nil

    case "offset":
        // offset is a window function ➜ alias it
        base := parts[0]
        k    := int(args[1].(map[string]any)["const"].(float64))
        fn   := "LAG"; if k < 0 { fn, k = "LEAD", -k }
        win  := fmt.Sprintf("%s(%s,%d) OVER (PARTITION BY s.ticker ORDER BY d.timestamp)", fn, base, k)
        return b.alias(win), nil

    default:
        return "", fmt.Errorf("unknown arith op %q", op)
    }
}

func (b *builder) compileAgg(raw any) (string, error) {
    a  := raw.(map[string]any)
    fn := strings.ToUpper(a["fn"].(string))

    of, err := b.compileVal(a["of"]); if err != nil { return "", err }
    period  := int64(0); if p, ok := a["period"]; ok { period = int64(p.(float64)) }

    win := "ROWS BETWEEN "
    if period == 0 {
        win += "UNBOUNDED PRECEDING"
    } else {
        win += fmt.Sprintf("%d PRECEDING", period-1)
    }
    win += " AND CURRENT ROW"

    return fmt.Sprintf("%s(%s) OVER (PARTITION BY s.ticker ORDER BY d.timestamp %s)", fn, of, win), nil
}

/*──────────────────────── helpers ────────────────────────────────*/

func (b *builder) alias(expr string) string {
    alias := fmt.Sprintf("n%d", b.next)
    b.next++
    b.ctes = append(b.ctes, fmt.Sprintf("%s AS %s", expr, alias))
    return alias
}

func mapComp(op string) string { if op == "==" { return "=" }; return op }
func pqQuoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }


// ---------------------------------------------------------------------------
// Public API – invoked by job runner
// ---------------------------------------------------------------------------
// prettyPrintJSON extracts and formats JSON from a string that may contain text before/after the JSON
func prettyPrintJSON(jsonStr string) (string, error) {
	// Find the JSON block within the string
	jsonStartIdx := strings.Index(jsonStr, "{")
	jsonEndIdx := strings.LastIndex(jsonStr, "}")

	if jsonStartIdx == -1 || jsonEndIdx == -1 || jsonEndIdx < jsonStartIdx {
		return "", fmt.Errorf("no valid JSON block found")
	}

	// Extract the JSON block
	jsonBlock := jsonStr[jsonStartIdx : jsonEndIdx+1]

	// Parse the JSON to validate it
	var parsedJSON interface{}
	if err := json.Unmarshal([]byte(jsonBlock), &parsedJSON); err != nil {
		return "", fmt.Errorf("invalid JSON: %v", err)
	}

	// Pretty print the JSON
	prettyJSON, err := json.MarshalIndent(parsedJSON, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error pretty printing JSON: %v", err)
	}

	return string(prettyJSON), nil
}

type RunBacktestArgs struct { StrategyId int `json:"strategyId"` }

type BacktestResult map[string]any

func RunBacktest(conn *utils.Conn, userId int, raw json.RawMessage) (any, error) {
    var a RunBacktestArgs
    if err := json.Unmarshal(raw, &a); err != nil { return nil, err }

    rawSpec, err := _getStrategySpec(conn, userId, a.StrategyId)
    if err != nil { return nil, err }

    var spec StrategySpec
    if err := json.Unmarshal(rawSpec, &spec); err != nil { return nil, err }

    sql, err := buildSQL(&spec)
    if err != nil { return nil, err }

    fmt.Println("rawSpec: ")
    fmt.Println(rawSpec)
    fmt.Println("compiled sql query: ")
    fmt.Println(sql)

    rows, err := conn.DB.Query(context.Background(), sql)
    if err != nil { return nil, fmt.Errorf("db error: %w (sql=%s)", err, sql) }
    defer rows.Close()

    cols := rows.FieldDescriptions()
    names := make([]string, len(cols))
    for i, fd := range cols { names[i] = string(fd.Name) }

    var recs []map[string]any
    for rows.Next() {
        vals := make([]any, len(names))
        for i := range vals { vals[i] = new(any) }
        if err := rows.Scan(vals...); err != nil { return nil, err }
        rec := map[string]any{}
        for i, n := range names { rec[n] = *(vals[i].(*any)) }
        recs = append(recs, rec)
    }
    if err := rows.Err(); err != nil { return nil, err }

    return BacktestResult{"sql": sql, "rows": recs}, nil
}

// ---------------------------------------------------------------------------
// SQL compilation – recursive walk
// ---------------------------------------------------------------------------


// ---------------- Boolean nodes ----------------

func compileBool(node any) (string, error) {
    m, ok := node.(map[string]any); if !ok { return "", fmt.Errorf("bool node not object") }
    switch {
    case m["cmp"] != nil:
        return compileCmp(m["cmp"])
    case m["logic"] != nil:
        return compileLogic(m["logic"])
    case m["rank"] != nil:
        return "", fmt.Errorf("rank filters not implemented in SQL compiler yet")
    default:
        return "", fmt.Errorf("unknown boolean node")
    }
}

func compileCmp(raw any) (string, error) {
    c := raw.(map[string]any)
    op := mapComp(c["op"].(string))
    lhs, err := compileVal(c["lhs"]); if err != nil { return "", err }
    rhs, err := compileVal(c["rhs"]); if err != nil { return "", err }
    return fmt.Sprintf("(%s %s %s)", lhs, op, rhs), nil
}

func compileLogic(raw any) (string, error) {
    l := raw.(map[string]any)
    op := l["op"].(string)
    args := l["args"].([]any)
    if op == "NOT" && len(args) != 1 { return "", fmt.Errorf("NOT needs 1 arg") }
    if (op == "AND" || op == "OR") && len(args) < 2 { return "", fmt.Errorf("%s needs ≥2 args", op) }

    var parts []string
    for _, a := range args {
        p, err := compileBool(a); if err != nil { return "", err }
        parts = append(parts, p)
    }
    if op == "NOT" { return "NOT (" + parts[0] + ")", nil }
    return "(" + strings.Join(parts, " "+op+" ") + ")", nil
}

// ---------------- Value nodes ------------------

func compileVal(node any) (string, error) {
    m := node.(map[string]any)
    switch {
    case m["const"] != nil:
        num := m["const"].(float64)
        return strconv.FormatFloat(num, 'f', -1, 64), nil
    case m["column"] != nil:
        return "d." + pqQuoteIdent(m["column"].(string)), nil
    case m["expr"] != nil:
        return compileExpr(m["expr"])
    case m["agg"] != nil:
        return compileAgg(m["agg"])
    }
    return "", fmt.Errorf("unknown value node")
}

func compileExpr(raw any) (string, error) {
    e := raw.(map[string]any)
    op := e["op"].(string)
    args := e["args"].([]any)
    if len(args) < 2 { return "", fmt.Errorf("expr needs ≥2 args") }

    parts := make([]string, len(args))
    for i, a := range args { p, err := compileVal(a); if err != nil { return "", err }; parts[i] = p }

    switch op {
    case "+", "-", "*":
        return "(" + strings.Join(parts, " "+op+" ") + ")", nil
    case "/":
        if len(parts) != 2 { return "", fmt.Errorf("/ needs 2 args") }
        n, d := parts[0], parts[1]
        return fmt.Sprintf("CASE WHEN %s = 0 THEN NULL ELSE %s / %s END", d, n, d), nil
    case "offset":
        // validator guarantees args[1] is const
        k := int(args[1].(map[string]any)["const"].(float64))
        fn := "LAG"; if k < 0 { fn, k = "LEAD", -k }
        return fmt.Sprintf("%s(%s,%d) OVER (PARTITION BY s.ticker ORDER BY d.timestamp)", fn, parts[0], k), nil
    default:
        return "", fmt.Errorf("unknown arith op %s", op)
    }
}

func compileAgg(raw any) (string, error) {
    a := raw.(map[string]any)
    fn := strings.ToUpper(a["fn"].(string))
    of, err := compileVal(a["of"]); if err != nil { return "", err }
    period := int64(0); if p, ok := a["period"]; ok { period = int64(p.(float64)) }

    win := "ROWS BETWEEN "; if period == 0 { win += "UNBOUNDED PRECEDING" } else { win += fmt.Sprintf("%d PRECEDING", period-1) }; win += " AND CURRENT ROW"

    return fmt.Sprintf("%s(%s) OVER (PARTITION BY s.ticker ORDER BY d.timestamp %s)", fn, of, win), nil
}

// ---------------- helpers ----------------------

