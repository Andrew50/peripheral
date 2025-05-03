package tools

import (
    "fmt"
    "strings"
)

// -----------------------------------------------------------------------------
// Public entry‑point
// -----------------------------------------------------------------------------

var (
    TimeframeToTable  = map[string]string{
        "1d": "ohlcv_1d",
        "1": "ohlcv_1",
        "1h": "ohlcv_1h",
        "1w": "ohlcv_1w",

    }
)

// CompileSpecToSQL converts a *validated* Spec into an executable SQL query.
// The returned string contains parameter‑free SQL –  production code should
// switch literals to bind variables before running at scale.
func CompileSpecToSQL(spec Spec) (string, error) {
    if err := validateSpec(&spec); err != nil { // double‑check in debug builds
        return "", fmt.Errorf("spec did not pass validation: %w", err)
    }

    // Get base table name using timeframe
    timeframeStr := string(spec.Universe.Timeframe)
    baseTable, ok := TimeframeToTable[timeframeStr]
    if !ok {
        // This should ideally be caught by validation, but check defensively
        return "", fmt.Errorf("unsupported timeframe %q (not found in TimeframeToTable)", timeframeStr)
    }

    // ------------------------------------------------------------------
    // 1. Universe CTE ----------------------------------------------------
    // ------------------------------------------------------------------


    universeConditions, err := buildUniverseConditions(&spec.Universe)
    if err != nil {
        return "", err
    }
    universeCTE := fmt.Sprintf(`universe AS (
        SELECT  d.timestamp, d.open, d.high, d.low, d.close, d.volume, 
                s.securityid, s.ticker, s.sector, s.industry, s.market
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
        // Get partition key based on the source's field and value
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
            SELECT  features.timestamp,
                    features.securityid,
                    features.ticker,
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

// timeframeToTable is now generated in specdefs

func buildUniverseConditions(u *Universe) ([]string, error) {
    var conds []string
    timeframeStr := string(u.Timeframe)

    // 1. Start/end time – only valid for intraday minute data
    if timeframeStr == timeframe1Min {
        if !u.StartTime.IsZero() {
            conds = append(conds, fmt.Sprintf("EXTRACT(TIME FROM d.timestamp) >= '%s'",
                u.StartTime.Format("15:04:05")))
        }
        if !u.EndTime.IsZero() {
            conds = append(conds, fmt.Sprintf("EXTRACT(TIME FROM d.timestamp) <= '%s'",
                u.EndTime.Format("15:04:05")))
        }
        if !u.ExtendedHours {
            // Updated to use explicit extended_hours column in the new table
            conds = append(conds, "d.extended_hours = false")
        }
    }

    // 2. Process the Filters slice instead of separate whitelist/blacklist fields
    for _, filter := range u.Filters {
        featureStr := string(filter.SecurityFeature)
        // Construct the column name directly by prepending the alias 's.'
        // Assumes featureStr ("ticker", "sector", etc.) matches the column name.
        // Validation should ensure featureStr is valid.
        columnName := "s." + featureStr

        // Add include/exclude conditions
        if len(filter.Include) > 0 {
            conds = append(conds, fmt.Sprintf("%s IN (%s)", columnName, quoteList(filter.Include)))
        }
        if len(filter.Exclude) > 0 {
            conds = append(conds, fmt.Sprintf("%s NOT IN (%s)", columnName, quoteList(filter.Exclude)))
        }
    }

    // Always have at least one condition for syntactic correctness
    if len(conds) == 0 {
        conds = append(conds, "1 = 1")
    }
    return conds, nil
}

// -----------------------------------------------------------------------------
// Internal helpers – features --------------------------------------------------
// -----------------------------------------------------------------------------

func compileFeatureExpr(f Feature, partitionKey string) (string, error) {
	const rowAlias = "u" // the alias we use in the features CTE

	// Handle empty expressions
	if len(f.Expr) == 0 {
		return "", fmt.Errorf("empty expression")
	}

	// Implement RPN evaluation using a stack
	var stack []string

	for _, part := range f.Expr {
		if part.Type == "column" {
			// Push column reference to stack, applying LAG if offset > 0
			colName := strings.ToLower(part.Value)
			colRef := fmt.Sprintf("%s.%s", rowAlias, colName) // Base reference

			if part.Offset < 0 {
				// This should be caught by validation, but handle defensively
				return "", fmt.Errorf("invalid negative offset %d for column %s", part.Offset, colName)
			}

			if part.Offset > 0 {
				// Apply LAG function if offset is specified.
				// LAG operates within the time series of a single security.
				lagPartitionKey := fmt.Sprintf("%s.securityid", rowAlias) // Partition LAG by security
				// Use 0 as the default value for LAG if the lagged row doesn't exist (e.g., at the start of the series)
				// Using COALESCE might be better if NULL is desired instead of 0. For simplicity, using 0 default in LAG.
				colRef = fmt.Sprintf("LAG(%s, %d, 0) OVER (PARTITION BY %s ORDER BY %s.timestamp)",
					colRef, part.Offset, lagPartitionKey, rowAlias)
			}
			stack = append(stack, colRef)

		} else if part.Type == "operator" {
			// Need at least two operands for binary operation
			if len(stack) < 2 {
				return "", fmt.Errorf("not enough operands for operator '%s'", part.Value)
			}

			// Pop the two top operands
			// Note: RPN pops right operand first, then left
			right := stack[len(stack)-1]
			left := stack[len(stack)-2]
			stack = stack[:len(stack)-2] // Remove the top two elements


			// Create the operation expression and push result back to stack
			// Handle potential division by zero - replace 0 with NULLIF or CASE WHEN
			var expr string
			if part.Value == "/" {
                 // Avoid division by zero; NULLIF(divisor, 0) returns NULL if divisor is 0
                 expr = fmt.Sprintf("(%s %s NULLIF(%s, 0))", left, part.Value, right)
            } else if part.Value == "^" {
				// Use POWER function for exponentiation as '^' is not standard SQL for power
				expr = fmt.Sprintf("POWER(%s, %s)", left, right)
			} else {
				expr = fmt.Sprintf("(%s %s %s)", left, part.Value, right)
			}
			stack = append(stack, expr)
		}
	}

	// After processing all parts, we should have exactly one value on the stack
	if len(stack) != 1 {
		return "", fmt.Errorf("invalid RPN expression: does not evaluate to a single result (stack size: %d)", len(stack))
	}

	expr := stack[0]

	// Determine the partitioning column for the outer window functions (AVG, NTILE, PERCENT_RANK)
	// Use the partitionKey provided, which might be securityid, sector, etc.
	windowPartitionCol := partitionKey
	// Ensure the alias 'u.' is used if the key comes from the universe CTE
	if strings.HasPrefix(windowPartitionCol, "s.") {
		windowPartitionCol = fmt.Sprintf("%s.%s", rowAlias, strings.TrimPrefix(windowPartitionCol, "s."))
	} else if !strings.HasPrefix(windowPartitionCol, rowAlias + ".") {
		// If no alias, assume it's a column in the universe CTE and add the alias
		windowPartitionCol = fmt.Sprintf("%s.%s", rowAlias, windowPartitionCol)
	}


	// Wrap smoothing window (simple moving average via AVG)
	if f.Window > 1 {
		expr = fmt.Sprintf(
			"AVG(%s) OVER (PARTITION BY %s ORDER BY %s.timestamp ROWS BETWEEN %d PRECEDING AND CURRENT ROW)",
			expr, windowPartitionCol, rowAlias, f.Window-1,
		)
	}

	// Output post‑processing (Rank/Percentile) - Partitioning uses windowPartitionCol
	switch f.Output {
	case "raw":
		// do nothing
	case "rankn": // ➜ integer rank with gaps
    expr = fmt.Sprintf(
        "RANK() OVER (PARTITION BY %s.timestamp ORDER BY %s ASC)",
        rowAlias, expr,
    )
case "rankp": // ➜ true percentile 0–100 float
    expr = fmt.Sprintf(
        "PERCENT_RANK() OVER (PARTITION BY %s.timestamp ORDER BY %s ASC)",
        rowAlias, expr,
    )
	default:
		return "", fmt.Errorf("unsupported output kind %q", f.Output)
	}

	return expr, nil
}

// partitionKeyForSource determines the SQL column name to use for partitioning window functions
// based on the feature's source definition. It defaults to securityid if the source is
// not relative or the feature field is not recognized for partitioning.
func partitionKeyForSource(src FeatureSource) string {
	// If the source value is specific (not relative), partitioning should happen per security.
	if src.Value != "relative" {
		return "s.securityid" // Partition by the specific security ID
	}

	// If relative, use the specified field for partitioning.
	// Assumes src.Field ("ticker", "sector", etc.) directly corresponds to a column
	// in the 'securities' table (aliased as 's'). Validation ensures this.
	featureStr := string(src.Field)
	return "s." + featureStr

	// Note: The case where src.Value != "relative" is handled before this block.
	// The default case (returning "s.securityid" if the field wasn't recognized)
	// is removed because validation now guarantees src.Field is a valid column name
	// when src.Value is "relative".
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
    dir := strings.ToUpper(string(sb.Direction))
    if dir != "ASC" && dir != "DESC" {
        dir = "DESC" // defensive default
    }
    return fmt.Sprintf("ORDER BY f%d %s, features.securityid, features.timestamp", sb.Feature, dir)
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
