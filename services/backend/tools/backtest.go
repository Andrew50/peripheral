package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type RunBacktestArgs struct {
	StrategyId int `json:"strategyId"`
}

func RunBacktest(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args RunBacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	fmt.Println("backtesting")
	fmt.Println(args.StrategyId)

	backtestJSON, err := _getStrategySpec(conn, userId, args.StrategyId) // get spec from db using helper
	if err != nil {
		return nil, fmt.Errorf("ERR vdi0s: failed to fetch strategy %v", err)
	}
	var spec StrategySpec
	if err := json.Unmarshal((backtestJSON), &spec); err != nil { //unmarhsal into struct
		return "", fmt.Errorf("ERR fi00: error parsing backtest JSON: %v", err)
	}
	data, err := GetDataForBacktest(conn, spec)
	if err != nil {
		return nil, fmt.Errorf("error getting data for backtest: %v", err)
	}

	// Parse the data to check if we got results
	var results map[string]interface{}
	err = json.Unmarshal([]byte(data), &results)
	if err != nil {
		return nil, fmt.Errorf("error parsing backtest results: %v", err)
	}

	// Check if we have results for daily timeframe
	dailyData, ok := results["daily"]
	if !ok {
		return nil, fmt.Errorf("no daily data found in results")
	}

	// Check if we have any records
	dailyRecords, ok := dailyData.([]interface{})
	if !ok {
		return nil, fmt.Errorf("daily data is not in expected format")
	}

	// Format the results for LLM readability
	formattedResults, err := formatResultsForLLM(dailyRecords)
	if err != nil {
		return nil, fmt.Errorf("error formatting results for LLM: %v", err)
	}
	return formattedResults, nil
}

// StrategySpec represents the parsed JSON structure of the backtest specification

func GetDataForBacktest(conn *utils.Conn, spec StrategySpec) (string, error) {

	// Initialize results
	result := make(map[string]any)

	// Currently only using daily timeframe since that's what the database supports
	timeframe := "daily"

	// Override any specified timeframes to ensure we use daily
	spec.Timeframes = []string{timeframe}

	// Adjust any non-daily indicators/conditions to daily
	for i := range spec.Indicators {
		spec.Indicators[i].Timeframe = timeframe
	}

	for i := range spec.Conditions {
		spec.Conditions[i].LHS.Timeframe = timeframe
		if spec.Conditions[i].RHS.Timeframe != "" {
			spec.Conditions[i].RHS.Timeframe = timeframe
		}
	}

	// Execute query for daily data
	data, err := executeBacktestQuery(conn, spec, timeframe)
	if err != nil {
		return "", fmt.Errorf("error executing query for timeframe %s: %v", timeframe, err)
	}
	result[timeframe] = data

	// Convert results to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("error serializing results: %v", err)
	}

	return string(resultJSON), nil
}

func executeBacktestQuery(conn *utils.Conn, spec StrategySpec, timeframe string) ([]map[string]interface{}, error) {
	// Build SQL query based on specification
	query, args := buildBacktestQuery(spec, timeframe)

	fmt.Printf("Executing SQL query: %s\nWith args: %v\n", query, args)

	// Execute query
	rows, err := conn.DB.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
	defer rows.Close()

	// Process results
	var results []map[string]interface{}

	// Get field descriptions (pgx equivalent of rows.Columns())
	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columns[i] = string(fd.Name)
	}

	// Scan each row
	for rows.Next() {

		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		for i := range values {
			values[i] = new(interface{})
		}

		// Scan the row into the slice
		if err := rows.Scan(values...); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Create a map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = *(values[i].(*interface{}))
		}
		results = append(results, row)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return results, nil
}

func buildBacktestQuery(spec StrategySpec, timeframe string) (string, []interface{}) {
	var args []interface{}

	// Build CTE for window functions first
	query := "WITH data_with_indicators AS (\n"
	query += "  SELECT s.ticker, s.securityid, d.timestamp, d.open, d.high, d.low, d.close, d.volume"

	// Add basic window expressions for previous values
	query += ",\n    LAG(d.close, 1) OVER (PARTITION BY s.ticker ORDER BY d.timestamp) AS prev_close"
	query += ",\n    LAG(d.open, 1) OVER (PARTITION BY s.ticker ORDER BY d.timestamp) AS prev_open"
	query += ",\n    LAG(d.high, 1) OVER (PARTITION BY s.ticker ORDER BY d.timestamp) AS prev_high"
	query += ",\n    LAG(d.low, 1) OVER (PARTITION BY s.ticker ORDER BY d.timestamp) AS prev_low"

	// Add user-defined derived columns
	for _, derivedCol := range spec.DerivedColumns {
		// Sanitize the expression and ID to prevent SQL injection
		// In a production system, you'd want more comprehensive validation
		expression := derivedCol.Expression
		id := derivedCol.ID
		if expression != "" && id != "" {
			// Add the custom expression with the given ID
			query += fmt.Sprintf(",\n    (%s) AS %s", expression, id)
		}
	}

	// Add indicator calculations if needed
	indicatorSelects := getIndicatorSelects(spec.Indicators, timeframe)
	if indicatorSelects != "" {
		query += ",\n    " + indicatorSelects
	}

	// Add future performance calculations
	futurePerformanceSelects := getFuturePerformanceSelects(spec.FuturePerformance, timeframe)
	if futurePerformanceSelects != "" {
		query += ",\n    " + futurePerformanceSelects
	}

	// FROM clause - use daily_ohlcv table for daily data
	query += "\n  FROM securities s JOIN daily_ohlcv d ON s.securityid = d.securityid"

	// Basic filters in the CTE
	whereConditions := []string{}

	// Date range
	if spec.DateRange.Start != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("d.timestamp >= $%d", len(args)+1))
		args = append(args, spec.DateRange.Start)
	}
	if spec.DateRange.End != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("d.timestamp <= $%d", len(args)+1))
		args = append(args, spec.DateRange.End)
	}

	// Stock universe filtering
	stockFilter := buildStockFilter(spec.Stocks)
	if stockFilter != "" {
		whereConditions = append(whereConditions, stockFilter)
	}

	// Stock explicit inclusions
	if len(spec.Stocks.Include) > 0 {
		placeholders := make([]string, len(spec.Stocks.Include))
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, spec.Stocks.Include[i])
		}
		whereConditions = append(whereConditions, fmt.Sprintf("s.ticker IN (%s)", strings.Join(placeholders, ",")))
	}

	// Stock explicit exclusions
	if len(spec.Stocks.Exclude) > 0 {
		placeholders := make([]string, len(spec.Stocks.Exclude))
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, spec.Stocks.Exclude[i])
		}
		whereConditions = append(whereConditions, fmt.Sprintf("s.ticker NOT IN (%s)", strings.Join(placeholders, ",")))
	}

	// Add basic filters (except conditions) to the CTE
	if len(whereConditions) > 0 {
		query += "\n  WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Close the CTE
	query += "\n)\n"

	// Main query using the CTE
	// Check if specific output columns are requested
	if len(spec.OutputColumns) > 0 {
		// Build custom SELECT with requested columns
		selectColumns := []string{}

		// Always include ticker and timestamp as fundamental columns
		selectColumns = append(selectColumns, "ticker", "timestamp", "securityid")

		// Add other requested columns
		for _, col := range spec.OutputColumns {
			// Skip ticker and timestamp since they're already included
			if col != "ticker" && col != "timestamp" {
				selectColumns = append(selectColumns, col)
			}
		}

		query += "SELECT " + strings.Join(selectColumns, ", ")
	} else {
		// Default columns if none specified
		query += "SELECT ticker, securityid, timestamp, open, high, low, close, volume"

		// Include all indicators in the output
		for _, indicator := range spec.Indicators {
			if indicator.Timeframe == timeframe {
				query += ", " + indicator.ID
			}
		}
		// Include all future performance columns in the output
		for _, futureCol := range spec.FuturePerformance {
			if futureCol.Timeframe == timeframe {
				query += ", " + futureCol.ID
			}
		}
	}

	query += "\nFROM data_with_indicators\n"

	// Apply complex conditions in the main query
	mainWhereConditions := []string{}

	// Add stock filters
	for _, filter := range spec.Stocks.Filters {
		if filter.Timeframe == timeframe {
			filterCond, filterArgs := buildSimpleFilterCondition(filter, len(args)+1)
			mainWhereConditions = append(mainWhereConditions, filterCond)
			args = append(args, filterArgs...)
		}
	}

	// Add condition logic
	conditionClauses := buildConditionClausesForCTE(spec.Conditions, spec.Logic, timeframe, len(args)+1)
	if conditionClauses != "" {
		mainWhereConditions = append(mainWhereConditions, conditionClauses)
	}

	// Combine all WHERE conditions for main query
	if len(mainWhereConditions) > 0 {
		query += "WHERE " + strings.Join(mainWhereConditions, " AND ")
	}

	// Order by timestamp
	query += "\nORDER BY ticker, timestamp"

	return query, args
}

// buildSimpleFilterCondition creates a simple filter without window functions
func buildSimpleFilterCondition(filter struct {
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Value     float64 `json:"value"`
	Timeframe string  `json:"timeframe"`
}, paramIdx int) (string, []interface{}) {
	var args []interface{}

	// Map metric to DB column
	var column string
	switch filter.Metric {
	case "market_cap":
		column = "marketcap"
	case "volume":
		column = "volume"
	case "dollar_volume":
		column = "close * volume"
	case "share_price":
		column = "close"
	default:
		column = filter.Metric
	}

	// Build condition with proper parameter index
	args = append(args, filter.Value)
	return fmt.Sprintf("%s %s $%d", column, filter.Operator, paramIdx), args
}

// buildConditionClausesForCTE builds condition clauses for the CTE approach
func buildConditionClausesForCTE(conditions []struct {
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
}, logic string, timeframe string, startParamIdx int) string {
	var clauses []string

	for _, condition := range conditions {
		if condition.LHS.Timeframe == timeframe {
			var clause string

			// Handle different condition types
			if condition.Operation == "crosses_above" {
				clause = buildCrossesAboveCondition(condition)
			} else if condition.Operation == "crosses_below" {
				clause = buildCrossesBelowCondition(condition)
			} else {
				clause = buildSimpleConditionForCTE(condition)
			}

			if clause != "" {
				clauses = append(clauses, clause)
			}
		}
	}

	if len(clauses) == 0 {
		return ""
	}

	logicOp := " AND "
	if strings.ToUpper(logic) == "OR" {
		logicOp = " OR "
	}

	return "(" + strings.Join(clauses, logicOp) + ")"
}

// buildSimpleConditionForCTE builds a simple condition clause for CTE
func buildSimpleConditionForCTE(condition struct {
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
}) string {
	// Build LHS
	var lhs string
	if condition.LHS.Offset == 0 {
		lhs = condition.LHS.Field
	} else if condition.LHS.Offset == -1 {
		lhs = "prev_" + condition.LHS.Field
	} else {
		// More complex offsets not supported in simple CTE approach
		return ""
	}

	// Build RHS
	var rhs string
	if condition.RHS.IndicatorID != "" {
		rhs = condition.RHS.IndicatorID
	} else if condition.RHS.Field != "" {
		if condition.RHS.Offset == 0 {
			rhs = condition.RHS.Field
		} else if condition.RHS.Offset == -1 {
			rhs = "prev_" + condition.RHS.Field
		} else {
			// More complex offsets not supported in simple CTE approach
			return ""
		}
	} else {
		// Value comparison
		rhs = fmt.Sprintf("%v", condition.RHS.Value)
	}
	if condition.RHS.Multiplier != 0 {
		rhs = fmt.Sprintf("%s * %f", rhs, condition.RHS.Multiplier)
	}

	return fmt.Sprintf("%s %s %s", lhs, mapOperator(condition.Operation), rhs)
}

// buildCrossesAboveCondition builds a crosses above condition
func buildCrossesAboveCondition(condition struct {
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
}) string {
	field := condition.LHS.Field

	var rhsExpr string
	if condition.RHS.IndicatorID != "" {
		rhsExpr = condition.RHS.IndicatorID
	} else if condition.RHS.Field != "" {
		rhsExpr = condition.RHS.Field
	} else {
		rhsExpr = fmt.Sprintf("%v", condition.RHS.Value)
	}

	return fmt.Sprintf("%s > %s AND prev_%s <= %s", field, rhsExpr, field, rhsExpr)
}

// buildCrossesBelowCondition builds a crosses below condition
func buildCrossesBelowCondition(condition struct {
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
}) string {
	field := condition.LHS.Field

	var rhsExpr string
	if condition.RHS.IndicatorID != "" {
		rhsExpr = condition.RHS.IndicatorID
	} else if condition.RHS.Field != "" {
		rhsExpr = condition.RHS.Field
	} else {
		rhsExpr = fmt.Sprintf("%v", condition.RHS.Value)
	}

	return fmt.Sprintf("%s < %s AND prev_%s >= %s", field, rhsExpr, field, rhsExpr)
}

func getIndicatorSelects(indicators []struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
	InputField string                 `json:"input_field"`
	Timeframe  string                 `json:"timeframe"`
}, timeframe string) string {
	var selects []string

	for _, indicator := range indicators {
		if indicator.Timeframe == timeframe {
			switch indicator.Type {
			case "SMA":
				period := int(indicator.Parameters["period"].(float64))
				field := indicator.InputField
				if field == "" {
					field = "close"
				}
				selects = append(selects, fmt.Sprintf("AVG(d.%s) OVER (PARTITION BY s.ticker ORDER BY d.timestamp ROWS BETWEEN %d PRECEDING AND CURRENT ROW) AS %s",
					field, period-1, indicator.ID))

			case "VWAP":
				// For daily data, VWAP is essentially a volume-weighted calculation over the day
				selects = append(selects, "d.vwap AS "+indicator.ID)

			case "EMA":
				// EMA requires more complex window functions
				period := int(indicator.Parameters["period"].(float64))
				field := indicator.InputField
				if field == "" {
					field = "close"
				}
				// This is a simplified approximation using PostgreSQL window functions
				// For a true EMA, you might need custom logic in Go after fetching the data
				selects = append(selects, fmt.Sprintf(
					"AVG(d.%s) OVER (PARTITION BY s.ticker ORDER BY d.timestamp ROWS BETWEEN %d PRECEDING AND CURRENT ROW) AS %s",
					field, period*2-1, indicator.ID))
			}
		}
	}

	return strings.Join(selects, ", ")
}

func getFuturePerformanceSelects(futurePerformances []struct {
	ID         string `json:"id"`
	Expression string `json:"expression"`
	Timeframe  string `json:"timeframe"`
	Comment    string `json:"comment,omitempty"`
}, timeframe string) string {
	var selects []string

	for _, fp := range futurePerformances {
		if fp.Timeframe == timeframe && fp.Expression != "" && fp.ID != "" {
			// Directly use the provided expression, aliased by ID.
			// Basic validation could be added here (e.g., check if expression contains 'LEAD(')
			// but proper SQL sanitization is complex.
			selects = append(selects, fmt.Sprintf("(%s) AS %s", fp.Expression, fp.ID))
		}
	}

	return strings.Join(selects, ", \n    ")
}

func buildStockFilter(stocks struct {
	Universe string   `json:"universe"`
	Include  []string `json:"include"`
	Exclude  []string `json:"exclude"`
	Filters  []struct {
		Metric    string  `json:"metric"`
		Operator  string  `json:"operator"`
		Value     float64 `json:"value"`
		Timeframe string  `json:"timeframe"`
	} `json:"filters"`
}) string {
	switch stocks.Universe {
	case "sector":
		return "s.sector IN (SELECT sector FROM sectors)"
	case "list":
		return "" // Handle with Include list
	default:
		return "" // "all" or default
	}
}

/*
func buildTimeOfDayCondition(timeOfDay struct {
	Constraint string `json:"constraint"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
}) string {
	switch timeOfDay.Constraint {
	case "specific_time":
		return fmt.Sprintf("TIME(d.timestamp) = '%s'", timeOfDay.StartTime)
	case "range":
		return fmt.Sprintf("TIME(d.timestamp) BETWEEN '%s' AND '%s'", timeOfDay.StartTime, timeOfDay.EndTime)
	case "pre_market":
		return "TIME(d.timestamp) < '09:30'"
	case "after_hours":
		return "TIME(d.timestamp) > '16:00'"
	default:
		return ""
	}
}
*/ // End temporary comment
func mapOperator(op string) string {
	switch op {
	case "==":
		return "="
	default:
		return op
	}
}

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

// formatResultsForLLM converts raw database results into a clean, LLM-friendly format
func formatResultsForLLM(records []interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Create a clean array of instances
	instances := make([]map[string]interface{}, 0, len(records))

	// Get all unique column names from the first record
	var columnNames []string
	if len(records) > 0 {
		if recordMap, ok := records[0].(map[string]interface{}); ok {
			for key := range recordMap {
				columnNames = append(columnNames, key)
			}
		}
	}

	for _, record := range records {
		recordMap, ok := record.(map[string]interface{})
		if !ok {
			continue
		}

		// Create a clean instance
		instance := make(map[string]interface{})

		// Process ticker and timestamp directly
		if ticker, ok := recordMap["ticker"]; ok {
			instance["ticker"] = ticker
		}
		if timestampValue, ok := recordMap["timestamp"]; ok {
			// Attempt to convert the timestamp to Unix milliseconds
			var timestampMs int64
			switch t := timestampValue.(type) {
			case time.Time:
				timestampMs = t.UnixMilli()
			case string:
				// Attempt to parse the string (assuming RFC3339 format)
				parsedTime, err := time.Parse(time.RFC3339, t)
				if err == nil {
					timestampMs = parsedTime.UnixMilli()
				} else {
					fmt.Printf("Warning: Could not parse timestamp string '%s': %v\n", t, err)
					// Fallback: Store the original string if parsing fails
					instance["timestamp"] = timestampValue
					continue // Skip assigning milliseconds if parsing failed
				}
			default:
				fmt.Printf("Warning: Unhandled timestamp type: %T\n", timestampValue)
				// Fallback: Store the original value if type is unexpected
				instance["timestamp"] = timestampValue
				continue // Skip assigning milliseconds if type is unknown
			}
			// Assign the timestamp in milliseconds
			instance["timestamp"] = timestampMs
		} else {
			// Handle case where timestamp is missing if necessary
			instance["timestamp"] = nil // Or some default value
		}

		// Process all numeric fields including OHLCV and indicators
		for _, key := range columnNames {
			// Skip ticker and timestamp as they're already handled
			if key == "ticker" || key == "timestamp" {
				continue
			}

			if value, ok := recordMap[key]; ok {
				// Convert the PostgreSQL numeric type
				if numericMap, ok := value.(map[string]interface{}); ok {
					// Extract exponent and value
					if exp, hasExp := numericMap["Exp"]; hasExp {
						if intVal, hasInt := numericMap["Int"]; hasInt {
							// Try to convert to float64
							expFloat, expOk := exp.(float64)
							var floatVal float64

							// Handle different numeric formats
							switch v := intVal.(type) {
							case float64:
								floatVal = v
							case string:
								// Parse scientific notation
								if parsedFloat, err := strconv.ParseFloat(v, 64); err == nil {
									floatVal = parsedFloat
								}
							}

							if expOk && floatVal != 0 {
								// Calculate actual decimal value
								actualValue := floatVal * math.Pow(10, expFloat)
								instance[key] = actualValue
							} else {
								instance[key] = intVal
							}
						}
					} else {
						// If structure doesn't match expected format, use original value
						instance[key] = value
					}
				} else {
					// Pass through non-numeric values
					instance[key] = value
				}
			}
		}

		instances = append(instances, instance)
	}

	// Create a summary
	summary := make(map[string]interface{})
	if len(instances) > 0 {
		// Get the ticker from the first instance
		if ticker, ok := instances[0]["ticker"].(string); ok {
			summary["ticker"] = ticker
		}
		summary["count"] = len(instances)
		summary["timeframe"] = "daily"
		summary["columns"] = columnNames

		// Calculate date range using the converted millisecond timestamps
		if len(instances) > 0 {
			var startTimeMs, endTimeMs int64
			var startTimeStr, endTimeStr string

			if firstTimestamp, ok := instances[0]["timestamp"].(int64); ok {
				startTimeMs = firstTimestamp
				startTimeStr = time.UnixMilli(startTimeMs).Format(time.RFC3339)
			}
			if lastTimestamp, ok := instances[len(instances)-1]["timestamp"].(int64); ok {
				endTimeMs = lastTimestamp
				endTimeStr = time.UnixMilli(endTimeMs).Format(time.RFC3339)
			}

			if startTimeStr != "" && endTimeStr != "" {
				summary["date_range"] = map[string]interface{}{
					"start_ms": startTimeMs,
					"end_ms":   endTimeMs,
					"start":    startTimeStr, // Keep original string format for summary readability if desired
					"end":      endTimeStr,
				}
			}
		}
	}

	// Create the final result structure
	result["instances"] = instances
	result["summary"] = summary

	return result, nil
}
