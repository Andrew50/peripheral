package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"google.golang.org/genai"
)

type BacktestArgs struct {
	Query string `json:"query"`
}

func GetBacktestJSONFromGemini(conn *utils.Conn, query string) (string, error) {
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
	result, err := client.Models.GenerateContent(context.Background(), "gemini-2.0-flash-thinking-exp-01-21", genai.Text(query), config)
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

	return responseText, nil
}

// BacktestSpec represents the parsed JSON structure of the backtest specification
type BacktestSpec struct {
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
}

func GetDataForBacktest(conn *utils.Conn, backtestJSON string) (string, error) {
	var spec BacktestSpec

	jsonStartIdx := strings.Index(backtestJSON, "{")
	jsonEndIdx := strings.LastIndex(backtestJSON, "}")

	jsonBlock := backtestJSON[jsonStartIdx : jsonEndIdx+1]
	if err := json.Unmarshal([]byte(jsonBlock), &spec); err != nil {
		return "", fmt.Errorf("error parsing backtest JSON: %v", err)
	}

	// Initialize results
	result := make(map[string]interface{})

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

func executeBacktestQuery(conn *utils.Conn, spec BacktestSpec, timeframe string) ([]map[string]interface{}, error) {
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
		fmt.Println("Row Result: ", row)
		results = append(results, row)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return results, nil
}

func buildBacktestQuery(spec BacktestSpec, timeframe string) (string, []interface{}) {
	var args []interface{}

	// Build CTE for window functions first
	query := "WITH data_with_indicators AS (\n"
	query += "  SELECT s.ticker, d.timestamp, d.open, d.high, d.low, d.close, d.volume"

	// Add basic window expressions for previous values
	query += ",\n    LAG(d.close, 1) OVER (PARTITION BY s.ticker ORDER BY d.timestamp) AS prev_close"
	query += ",\n    LAG(d.open, 1) OVER (PARTITION BY s.ticker ORDER BY d.timestamp) AS prev_open"
	query += ",\n    LAG(d.high, 1) OVER (PARTITION BY s.ticker ORDER BY d.timestamp) AS prev_high"
	query += ",\n    LAG(d.low, 1) OVER (PARTITION BY s.ticker ORDER BY d.timestamp) AS prev_low"

	// Add indicator calculations if needed
	indicatorSelects := getIndicatorSelects(spec.Indicators, timeframe)
	if indicatorSelects != "" {
		query += ",\n    " + indicatorSelects
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
	query += "SELECT ticker, timestamp, open, high, low, close, volume"

	// Include indicators in the output
	for _, indicator := range spec.Indicators {
		if indicator.Timeframe == timeframe {
			query += ", " + indicator.ID
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
		rhs = fmt.Sprintf("%s * $%d", rhs, condition.RHS.Multiplier)
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

func mapOperator(op string) string {
	switch op {
	case "==":
		return "="
	default:
		return op
	}
}

func RunBacktest(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args BacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	fmt.Printf("Running backtest with query: %s\n", args.Query)

	backtestJSON, err := GetBacktestJSONFromGemini(conn, args.Query)
	if err != nil {
		return nil, fmt.Errorf("error getting backtest JSON from gemini: %v", err)
	}
	fmt.Println("Gemini returned backtest JSON: ", backtestJSON)

	// Make sure we have JSON content
	if !strings.Contains(backtestJSON, "{") || !strings.Contains(backtestJSON, "}") {
		return nil, fmt.Errorf("no valid JSON found in Gemini response: %s", backtestJSON)
	}

	data, err := GetDataForBacktest(conn, backtestJSON)
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
	fmt.Println("Formatted results: ", formattedResults)
	return formattedResults, nil
}

// formatResultsForLLM converts raw database results into a clean, LLM-friendly format
func formatResultsForLLM(records []interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Create a clean array of instances
	instances := make([]map[string]interface{}, 0, len(records))

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
		if timestamp, ok := recordMap["timestamp"]; ok {
			instance["timestamp"] = timestamp
		}

		// Process numeric OHLCV fields
		for _, field := range []string{"open", "high", "low", "close", "volume"} {
			if value, ok := recordMap[field]; ok {
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
								instance[field] = actualValue
							} else {
								instance[field] = intVal
							}
						}
					} else {
						// If structure doesn't match expected format, use original value
						instance[field] = value
					}
				} else {
					// Pass through non-numeric values
					instance[field] = value
				}
			}
		}

		// Add any additional fields (indicators, etc.)
		for key, value := range recordMap {
			if key != "ticker" && key != "timestamp" && key != "open" && key != "high" && key != "low" && key != "close" && key != "volume" {
				// Process any other fields the same way as OHLCV
				if numericMap, ok := value.(map[string]interface{}); ok {
					if exp, hasExp := numericMap["Exp"]; hasExp {
						if intVal, hasInt := numericMap["Int"]; hasInt {
							expFloat, expOk := exp.(float64)
							var floatVal float64

							switch v := intVal.(type) {
							case float64:
								floatVal = v
							case string:
								if parsedFloat, err := strconv.ParseFloat(v, 64); err == nil {
									floatVal = parsedFloat
								}
							}

							if expOk && floatVal != 0 {
								actualValue := floatVal * math.Pow(10, expFloat)
								instance[key] = actualValue
							} else {
								instance[key] = intVal
							}
						}
					} else {
						instance[key] = value
					}
				} else {
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

		// Calculate date range
		if len(instances) > 0 {
			if firstDate, ok := instances[0]["timestamp"].(string); ok {
				if lastDate, ok := instances[len(instances)-1]["timestamp"].(string); ok {
					summary["date_range"] = map[string]string{
						"start": firstDate,
						"end":   lastDate,
					}
				}
			}
		}
	}

	// Create the final result structure
	result["instances"] = instances
	result["summary"] = summary

	return result, nil
}
