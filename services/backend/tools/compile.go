// spec_sql_compiler.go
//
// Author: ChatGPT (April 2025)
//
// A very first‑pass SQL compiler that translates a validated *Spec* (defined in
// tools/spec.go) into a runnable ANSI‑SQL query.  The implementation follows
// the guidelines in spec.txt & attached notes.  It deliberately keeps to the
// simplest subset that covers security‑level features and the four official
// timeframes (1 min, 1 h, 1 d, 1 w).  More sophisticated partitioning (sector‑
// level, cross‑ticker joins, etc.) can be layered on top in future PRs.
//
// ⚠️ This file **assumes** the caller has already executed the heavy‑duty
// validation logic in spec.go.  Compile*() therefore trusts that every token
// in feature.Expr is drawn from the allowed whitelist and will panic early if
// that contract is violated.
//
// Usage example:
//      sql, err := tools.CompileSpecToSQL(mySpec)
//      if err != nil { … }
//      rows, err := db.Query(ctx, sql)
//
package tools

import (
    "fmt"
    "regexp"
    "strings"
)

// -----------------------------------------------------------------------------
// Public entry‑point
// -----------------------------------------------------------------------------

// CompileSpecToSQL converts a *validated* Spec into an executable SQL query.
// The returned string contains parameter‑free SQL –  production code should
// switch literals to bind variables before running at scale.
func CompileSpecToSQL(spec Spec) (string, error) {
    if err := validateSpec(&spec); err != nil { // double‑check in debug builds
        return "", fmt.Errorf("spec did not pass validation: %w", err)
    }

    baseTable, ok := timeframeToTable[spec.Universe.Timeframe]
    if !ok {
        return "", fmt.Errorf("unsupported timeframe %q", spec.Universe.Timeframe)
    }

    // ------------------------------------------------------------------
    // 1. Universe CTE ----------------------------------------------------
    // ------------------------------------------------------------------

    universeConditions, err := buildUniverseConditions(&spec.Universe)
    if err != nil {
        return "", err
    }
    universeCTE := fmt.Sprintf(`universe AS (
        SELECT  d.*,  s.securityid, s.ticker, s.sector, s.industry, s.market
        FROM    %s               AS d
        JOIN    securities       AS s  ON s.securityid = d.securityid
        WHERE   %s
    )`, baseTable, strings.Join(universeConditions, " AND "))

    // ------------------------------------------------------------------
    // 2. Feature CTE -----------------------------------------------------
    // ------------------------------------------------------------------

    featureCols := make([]string, len(spec.Features)) // f0, f1 …
    featureExprs := make([]string, len(spec.Features))

    for i, f := range spec.Features {
        pKey := partitionKeyForSource(f.Source)
        compiledExpr, err := compileFeatureExpr(f, pKey)
        if err != nil {
            return "", fmt.Errorf("feature[%d] (%s): %w", i, f.Name, err)
        }
        featureAlias := fmt.Sprintf("f%d", f.FeatureId)
        featureCols[i] = fmt.Sprintf("%s AS %s", compiledExpr, featureAlias)
        featureExprs[i] = featureAlias // for later filter expansion
    }

    featureCTE := fmt.Sprintf(`features AS (
        SELECT  u.*,
                %s
        FROM    universe AS u
    )`, strings.Join(featureCols, ",\n                "))

    // ------------------------------------------------------------------
    // 3. Final SELECT with filters & sorting ----------------------------
    // ------------------------------------------------------------------

    whereClauses := buildFilterClauses(spec.Filters)
    orderByClause := buildOrderBy(spec.SortBy)

    finalSQL := fmt.Sprintf(`WITH
%s,

%s

SELECT  timestamp,
        securityid,
        ticker,
        %s
FROM    features
%s
%s;`,
        indent(universeCTE, 1),
        indent(featureCTE, 1),
        strings.Join(featureExprs, ", "),
        optional("WHERE", strings.Join(whereClauses, " AND ")),
        orderByClause,
    )

    return finalSQL, nil
}

// -----------------------------------------------------------------------------
// Internal helpers – universe --------------------------------------------------
// -----------------------------------------------------------------------------

var timeframeToTable = map[string]string{
    "1":  "minute_ohlcv",
    "1h": "hourly_ohlcv",
    "1d": "daily_ohlcv",
    "1w": "weekly_ohlcv",
}

func buildUniverseConditions(u *Universe) ([]string, error) {
    var conds []string

    // 1. Start/end time – only valid for intraday minute data
    if u.Timeframe == timeframe1Min {
        if !u.StartTime.IsZero() {
            conds = append(conds, fmt.Sprintf("EXTRACT(TIME FROM d.timestamp) >= '%s'",
                u.StartTime.Format("15:04:05")))
        }
        if !u.EndTime.IsZero() {
            conds = append(conds, fmt.Sprintf("EXTRACT(TIME FROM d.timestamp) <= '%s'",
                u.EndTime.Format("15:04:05")))
        }
        if !u.ExtendedHours {
            // Assumes a boolean column minute_ohlcv.is_extended_hours (legacy code)
            conds = append(conds, "d.is_extended_hours = false")
        }
    }

    // 2. Whitelists / blacklists
    appendInNotIn := func(column string, whitelist, blacklist []string) {
        if len(whitelist) > 0 {
            conds = append(conds, fmt.Sprintf("%s IN (%s)", column, quoteList(whitelist)))
        }
        if len(blacklist) > 0 {
            conds = append(conds, fmt.Sprintf("%s NOT IN (%s)", column, quoteList(blacklist)))
        }
    }

    appendInNotIn("s.sector", u.Sectors.WhiteList, u.Sectors.Blacklist)
    appendInNotIn("s.industry", u.Industries.WhiteList, u.Industries.Blacklist)
    appendInNotIn("s.ticker", u.Securities.WhiteList, u.Securities.Blacklist)
    appendInNotIn("s.market", u.Markets.WhiteList, u.Markets.Blacklist)

    // Always have at least one condition for syntactic correctness
    if len(conds) == 0 {
        conds = append(conds, "1 = 1")
    }
    return conds, nil
}

// -----------------------------------------------------------------------------
// Internal helpers – features --------------------------------------------------
// -----------------------------------------------------------------------------

// Accept base‑column tokens plus numeric literals/operators.  Validation layer
// guarantees safety; we still use a strict regexp to convert [lag] tokens.
var (
    lagPattern        = regexp.MustCompile(`\b(open|high|low|close|volume)\[(\d+)\]`)
    // Simple pattern to find base keywords, context check done separately.
    basePatternSimple = regexp.MustCompile(`\b(open|high|low|close|volume)\b`)
)

func compileFeatureExpr(f Feature, partitionKey string) (string, error) {
    expr := f.Expr

    // Replace lag tokens first – keep them idempotent to avoid double‑replacement.
    expr = lagPattern.ReplaceAllStringFunc(expr, func(tok string) string {
        parts := lagPattern.FindStringSubmatch(tok)
        col, offset := parts[1], parts[2]
        return fmt.Sprintf(
            "LAG(d.%s, %s) OVER (PARTITION BY %s ORDER BY d.timestamp)",
            col, offset, partitionKey,
        )
    })

    // Replace base columns (open, high, low, close, volume) with qualified names (d.open, etc.)
    // ONLY if they are NOT followed by '[' (which indicates a lag handled above).
    // We use FindAllStringIndex and check context manually because Go's regexp (RE2)
    // doesn't support negative lookahead (?!\[).
    matches := basePatternSimple.FindAllStringIndex(expr, -1)
    var replacements []struct{ start, end int; replacement string }

    for _, matchIndices := range matches {
        start := matchIndices[0]
        end := matchIndices[1]
        word := expr[start:end]

        // Check if the character immediately after the match is '['
        isLag := false
        if end < len(expr) && expr[end] == '[' {
            isLag = true
        }

        if !isLag {
            // Schedule replacement if it's not a lag token
            replacements = append(replacements, struct{ start, end int; replacement string }{
                start:       start,
                end:         end,
                replacement: "d." + word,
            })
        }
    }

    // Apply replacements in reverse order to avoid messing up indices
    // as the string length changes.
    for i := len(replacements) - 1; i >= 0; i-- {
        rep := replacements[i]
        expr = expr[:rep.start] + rep.replacement + expr[rep.end:]
    }


    // Wrap smoothing window (simple moving average via AVG)
    if f.Window > 1 {
        expr = fmt.Sprintf(
            "AVG((%s)) OVER (PARTITION BY %s ORDER BY d.timestamp ROWS BETWEEN %d PRECEDING AND CURRENT ROW)",
            expr, partitionKey, f.Window-1,
        )
    }

    // Output post‑processing
    switch f.Output {
    case "raw":
        // do nothing
    case "rankn": // 0‑1 normalised
        expr = fmt.Sprintf(
            "PERCENT_RANK() OVER (PARTITION BY d.timestamp ORDER BY %s)",
            expr,
        )
    case "rankp": // 1‑100 percentiles using NTILE
        expr = fmt.Sprintf(
            "NTILE(100) OVER (PARTITION BY d.timestamp ORDER BY %s)",
            expr,
        )
    default:
        return "", fmt.Errorf("unsupported output kind %q", f.Output)
    }

    return expr, nil
}

func partitionKeyForSource(src string) string {
    switch strings.ToLower(src) {
    case "security", "":
        return "s.securityid"
    case "sector":
        return "s.sector"
    case "industry":
        return "s.industry"
    case "market":
        return "s.market"
    default:
        // A specific ticker or proprietary source – fall back to securityid.
        return "s.securityid"
    }
}

// -----------------------------------------------------------------------------
// Internal helpers – filters & sorting ----------------------------------------
// -----------------------------------------------------------------------------

func buildFilterClauses(filters []Filter) []string {
    clauses := make([]string, 0, len(filters))
    for _, f := range filters {
        lhs := fmt.Sprintf("f%d", f.LHS)
        var rhs string
        switch {
        case f.RHS.FeatureId != 0:
            rhs = fmt.Sprintf("f%d", f.RHS.FeatureId)
        default:
            // constants – format with full precision, but trim trailing zeros
            rhs = trimFloat(f.RHS.Const)
        }
        if f.RHS.Scale != 0 && f.RHS.Scale != 1 {
            rhs = fmt.Sprintf("(%s * %s)", rhs, trimFloat(f.RHS.Scale))
        }
        clauses = append(clauses, fmt.Sprintf("%s %s %s", lhs, f.Operator, rhs))
    }
    return clauses
}

func buildOrderBy(sb SortBy) string {
    if sb.Feature == 0 && sb.Direction == "" {
        return "" // No sorting specified.
    }
    dir := strings.ToUpper(sb.Direction)
    if dir != "ASC" && dir != "DESC" {
        dir = "DESC" // defensive default
    }
    return fmt.Sprintf("ORDER BY f%d %s, securityid, timestamp", sb.Feature, dir)
}

// -----------------------------------------------------------------------------
// Misc. string helpers ---------------------------------------------------------
// -----------------------------------------------------------------------------

func quoteList(xs []string) string {
    quoted := make([]string, len(xs))
    for i, v := range xs {
        quoted[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
    }
    return strings.Join(quoted, ", ")
}

func trimFloat(f float64) string {
    s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", f), "0"), ".")
    if s == "" || s == "-" {
        return "0"
    }
    return s
}

func optional(keyword, expr string) string {
    expr = strings.TrimSpace(expr)
    if expr == "" {
        return ""
    }
    return fmt.Sprintf("%s %s", keyword, expr)
}

func indent(s string, levels int) string {
    pad := strings.Repeat("    ", levels)
    lines := strings.Split(s, "\n")
    for i, ln := range lines {
        lines[i] = pad + ln
    }
    return strings.Join(lines, "\n")
}

// -----------------------------------------------------------------------------
// Basic smoke‑test -------------------------------------------------------------
// -----------------------------------------------------------------------------

// The tiny test lives in this file for convenience.  Real projects should move
// it to *_test.go.

// func example() {
//     spec := Spec{ /* … fill or unmarshal … */ }
//     sql, err := CompileSpecToSQL(spec)
//     if err != nil { panic(err) }
//     fmt.Println(sql)
// }

// -----------------------------------------------------------------------------
// End of file ------------------------------------------------------------------

