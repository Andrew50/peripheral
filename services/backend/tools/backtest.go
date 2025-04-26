package tools

import (
	"backend/utils"
	"context"
    "github.com/jackc/pgx/v4"
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

// RunBacktest executes a backtest for the given strategy
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
	
	var spec Spec
	if err := json.Unmarshal((backtestJSON), &spec); err != nil { //unmarshal into struct
		return "", fmt.Errorf("ERR fi00: error parsing backtest JSON: %v", err)
	}
	
	// *** New approach using CompileSpecToSQL ***
	// Generate SQL from the spec
	sql, err := CompileSpecToSQL(spec)
	if err != nil {
		return nil, fmt.Errorf("error compiling SQL for backtest: %v", err)
	}
    fmt.Println(sql)
	
	// Execute the query
	ctx := context.Background()
	rows, err := conn.DB.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("error executing backtest query: %v", err)
	}
	defer rows.Close()
	
	// Process the results
	records, err := ScanRows(rows)
	if err != nil {
		return nil, fmt.Errorf("error scanning backtest results: %v", err)
	}
	
	// Check if we have any records
	if len(records) == 0 {
		return nil, fmt.Errorf("no data found for backtest")
	}
	
	// Convert to interface slice for formatBacktestResults
	recordsInterface := make([]interface{}, len(records))
	for i, record := range records {
		recordsInterface[i] = record
	}
	
	// Format the results for LLM readability
	formattedResults, err := formatBacktestResults(recordsInterface)
	if err != nil {
		return nil, fmt.Errorf("error formatting results for LLM: %v", err)
	}

	// Save the full formatted results (including instances) to cache
	go func() { // Run in a goroutine to avoid blocking the main response
		bgCtx := context.Background() // Use a background context for the goroutine
		if err := SaveBacktestToCache(bgCtx, conn, userId, args.StrategyId, formattedResults); err != nil {
			fmt.Printf("Warning: Failed to save backtest results to cache for strategy %d: %v\n", args.StrategyId, err)
			// We log the error but don't fail the main operation
		}
	}()

	// Extract only the summary to return to the LLM
	summary, ok := formattedResults["summary"].(map[string]interface{})
	if !ok {
		// This should ideally not happen if formatResultsForLLM worked
		return nil, fmt.Errorf("failed to extract summary from formatted backtest results")
	}

	// --- Save Summary to Persistent Context ---
	go func() {
		ctx := context.Background()
		contextKey := fmt.Sprintf("backtest_summary_strategy_%d", args.StrategyId)
		// Use 0 for itemExpiration to rely on the default context expiration (7 days)
		if err := AddOrUpdatePersistentContextItem(ctx, conn, userId, contextKey, summary, 0); err != nil {
			fmt.Printf("Warning: Failed to save backtest summary (strategy %d) to persistent context: %v\n", args.StrategyId, err)
		}
	}()
	// --- End Save Summary ---

	return summary, nil // Return only the summary map
}

// runCompiledBacktest executes a backtest for a given spec using the new compiler
func runCompiledBacktest(conn *utils.Conn, userId int, spec Spec) (map[string]interface{}, error) {
	// Generate SQL from the spec
	sql, err := CompileSpecToSQL(spec)
	if err != nil {
		return nil, fmt.Errorf("error compiling SQL for backtest: %v", err)
	}
	
	// Execute the query
	ctx := context.Background()
	rows, err := conn.DB.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("error executing backtest query: %v", err)
	}
	defer rows.Close()
	
	// Process the results
	records, err := ScanRows(rows)
	if err != nil {
		return nil, fmt.Errorf("error scanning backtest results: %v", err)
	}
	
	// Check if we have any records
	if len(records) == 0 {
		return nil, fmt.Errorf("no data found for backtest")
	}
	
	// Convert to interface slice for formatBacktestResults
	recordsInterface := make([]interface{}, len(records))
	for i, record := range records {
		recordsInterface[i] = record
	}
	
	// Format the results for LLM readability
	formattedResults, err := formatBacktestResults(recordsInterface)
	if err != nil {
		return nil, fmt.Errorf("error formatting results for LLM: %v", err)
	}

	// Extract only the summary to return
	summary, ok := formattedResults["summary"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to extract summary from formatted backtest results")
	}
	
	return summary, nil
}

// ScanRows converts pgx.Rows to a slice of maps
func ScanRows(rows pgx.Rows) ([]map[string]interface{}, error) {
	// Get field descriptions
	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columns[i] = string(fd.Name)
	}

	// Process results
	var results []map[string]interface{}

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

// formatBacktestResults converts raw database results into a clean, LLM-friendly format
func formatBacktestResults(records []interface{}) (map[string]interface{}, error) {
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
