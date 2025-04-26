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
    ReturnResults bool `json:"returnResults"`
}

// RunBacktest executes a backtest for the given strategy
// RunBacktest executes a backtest for the given strategy
func RunBacktest(conn *utils.Conn, userId int, rawArgs json.RawMessage) (any, error) {
	var args RunBacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	fmt.Println("backtesting")
	fmt.Println(args.StrategyId)

	backtestJSON, err := _getStrategySpec(conn,  args.StrategyId,userId) // get spec from db using helper
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
	fmt.Println("Generated SQL:", sql)

	// Execute the query
	ctx := context.Background()
	rows, err := conn.DB.Query(ctx, sql)
	if err != nil {
		// Handle potential query errors (syntax errors, connection issues, etc.)
		return nil, fmt.Errorf("error executing backtest query: %v", err)
	}
	defer rows.Close() // Ensure rows are closed even if ScanRows errors

	// Process the results
	// ScanRows will return an empty slice if there are no rows, and a nil error.
	records, err := ScanRows(rows)
	if err != nil {
		// Handle errors during row scanning (data type mismatches, etc.)
		return nil, fmt.Errorf("error scanning backtest results: %v", err)
	}

	// ******************************************************************
	// ** CHANGE: Removed the check for len(records) == 0            **
	// ** The code now proceeds even if records is empty.            **
	// ******************************************************************
	// // Check if we have any records
	// // if len(records) == 0 {
	// // 	 return nil, fmt.Errorf("no data found for backtest") // <--- REMOVED THIS BLOCK
	// // }

	// Convert to interface slice for formatBacktestResults
	// This correctly handles an empty records slice, resulting in an empty recordsInterface slice.
	recordsInterface := make([]any, len(records))
	for i, record := range records {
		recordsInterface[i] = record
	}

	// Format the results for LLM readability
	// formatBacktestResults handles an empty input slice correctly,
	// returning a map with empty "instances" and "summary".
	formattedResults, err := formatBacktestResults(recordsInterface, &spec)
	if err != nil {
		// This error path should ideally not be reached if formatBacktestResults
		// handles empty input, but kept for robustness.
		return nil, fmt.Errorf("error formatting results for LLM: %v", err)
	}

	// Save the full formatted results (including instances) to cache
	// This will save the empty structure if no results were found.
	go func() { // Run in a goroutine to avoid blocking the main response
		bgCtx := context.Background() // Use a background context for the goroutine
		if err := SaveBacktestToCache(bgCtx, conn, userId, args.StrategyId, formattedResults); err != nil {
			fmt.Printf("Warning: Failed to save backtest results to cache for strategy %d: %v\n", args.StrategyId, err)
			// We log the error but don't fail the main operation
		}
	}()

	// Extract only the summary to return to the LLM (Now handled inside formatBacktestResults)
	// The summary will be an empty map if there were no records.
	summary, ok := formattedResults["summary"].(map[string]any)
	if !ok {
		// This should ideally not happen if formatBacktestResults worked correctly
		// even for empty input.
		return nil, fmt.Errorf("failed to extract summary from formatted backtest results (internal error)")
	}

	// --- Save Summary to Persistent Context ---
	// This will save the empty summary map if no results were found.
	go func() {
		ctx := context.Background()
		contextKey := fmt.Sprintf("backtest_summary_strategy_%d", args.StrategyId)
		// Use 0 for itemExpiration to rely on the default context expiration (7 days)
		if err := AddOrUpdatePersistentContextItem(ctx, conn, userId, contextKey, summary, 0); err != nil {
			fmt.Printf("Warning: Failed to save backtest summary (strategy %d) to persistent context: %v\n", args.StrategyId, err)
		}
	}()
	// --- End Save Summary ---

    if (args.ReturnResults){
        return formattedResults, nil
	// Return the formatted results (which will contain empty instances/summary if no hits) and nil error
    }else{
        return summary, nil
    }
}

// runCompiledBacktest executes a backtest for a given spec using the new compiler

// ScanRows converts pgx.Rows to a slice of maps
func ScanRows(rows pgx.Rows) ([]map[string]any, error) {
	// Get field descriptions
	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columns[i] = string(fd.Name)
	}

	// Process results
	var results []map[string]any

	// Scan each row
	for rows.Next() {
		// Create a slice of any to hold the values
		values := make([]any, len(columns))
		for i := range values {
			values[i] = new(any)

		}

		// Scan the row into the slice
		if err := rows.Scan(values...); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Create a map for this row
		row := make(map[string]any)
		for i, col := range columns {
			row[col] = *(values[i].(*any))
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
// with feature values using feature names from the spec if available
func formatBacktestResults(records []any, spec *Spec) (map[string]any, error) {
    result := make(map[string]any)

    // Create a clean array of instances
    instances := make([]map[string]any, 0, len(records))

    // Map feature IDs to feature names if spec is provided
    featureMap := make(map[string]string) // Maps "f0", "f1" to actual feature names
    if spec != nil {
        for _, feature := range spec.Features {
            featureKey := fmt.Sprintf("f%d", feature.FeatureId)
            featureMap[featureKey] = feature.Name
        }
    }

    // Get all unique column names from the first record
    var columnNames []string
    if len(records) > 0 {
        if recordMap, ok := records[0].(map[string]any); ok {
            for key := range recordMap {
                columnNames = append(columnNames, key)
            }
        }
    }

    for _, record := range records {
        recordMap, ok := record.(map[string]any)
        if !ok {
            continue
        }

        // Create a clean instance
        instance := make(map[string]any)

        // Process ticker, securityId, and timestamp directly
        if ticker, ok := recordMap["ticker"]; ok {
            instance["ticker"] = ticker
        }
        if securityId, ok := recordMap["securityid"]; ok {
            instance["securityId"] = securityId
        }
        if timestampValue, ok := recordMap["timestamp"]; ok {
            // Convert timestamp logic (keeping existing implementation)
            var timestampMs int64
            switch t := timestampValue.(type) {
            case time.Time:
                timestampMs = t.UnixMilli()
            case string:
                parsedTime, err := time.Parse(time.RFC3339, t)
                if err == nil {
                    timestampMs = parsedTime.UnixMilli()
                } else {
                    fmt.Printf("Warning: Could not parse timestamp string '%s': %v\n", t, err)
                    instance["timestamp"] = timestampValue
                    continue
                }
            default:
                fmt.Printf("Warning: Unhandled timestamp type: %T\n", timestampValue)
                instance["timestamp"] = timestampValue
                continue
            }
            instance["timestamp"] = timestampMs
        } else {
            instance["timestamp"] = nil
        }

// Process all numeric fields including OHLCV and indicators
        for _, key := range columnNames {
            // Skip already handled fields
            if key == "ticker" || key == "securityid" || key == "timestamp" {
                continue
            }

            // Check if this is a feature column (like "f0", "f1", etc.)
            columnName := key
            if spec != nil && strings.HasPrefix(key, "f") {
                if mappedName, ok := featureMap[key]; ok {
                    columnName = mappedName // Use the proper feature name from spec
                }
            }

            if value, ok := recordMap[key]; ok {
                // Process the value (numeric handling logic from original implementation)
                processedValue := processNumericValue(value)
                
                // Double-check if the processed value is still a map with numeric type fields
                // This handles nested numeric objects that might not be caught by the first pass
                if valueMap, isMap := processedValue.(map[string]any); isMap {
                    if _, hasExp := valueMap["Exp"]; hasExp {
                        if _, hasInt := valueMap["Int"]; hasInt {
                            // This is still a numeric object, process it again
                            processedValue = processNumericValue(processedValue)
                        }
                    }
                }
                
                instance[columnName] = processedValue
            }
        }

        instances = append(instances, instance)
    }

    // Create a summary
    summary := make(map[string]any)

    // Add the count of instances to the summary
    summary["count"] = len(instances)

    if len(instances) > 0 {
    // Find the earliest and latest timestamps
    var minTimeMs int64 = math.MaxInt64
    var maxTimeMs int64 = 0
    var startTimeStr, endTimeStr string

    // Scan all instances to find min and max timestamps
    for _, instance := range instances {
        if tsMs, ok := instance["timestamp"].(int64); ok {
            if tsMs < minTimeMs {
                minTimeMs = tsMs
                startTimeStr = time.UnixMilli(minTimeMs).Format(time.RFC3339)
            }
            if tsMs > maxTimeMs {
                maxTimeMs = tsMs
                endTimeStr = time.UnixMilli(maxTimeMs).Format(time.RFC3339)
            }
        }
    }

    if minTimeMs != math.MaxInt64 && maxTimeMs != 0 {
        summary["date_range"] = map[string]any{
            "start_ms": minTimeMs,
            "end_ms":   maxTimeMs,
            "start":    startTimeStr,
            "end":      endTimeStr,
        }
    }
}

    // Create the final result structure
    result["instances"] = instances
    result["summary"] = summary

    return result, nil
}

// Helper function to process numeric values from the database
func processNumericValue(value any) any {
    // Handle PostgreSQL numeric type
    if numericMap, ok := value.(map[string]any); ok {
        // Check if this is a PostgreSQL numeric type with Exp and Int fields
        exp, hasExp := numericMap["Exp"]
        intVal, hasInt := numericMap["Int"]
        
        if hasExp && hasInt {
            // Convert exponent to float64
            var expFloat float64
            switch e := exp.(type) {
            case float64:
                expFloat = e
            case int:
                expFloat = float64(e)
            case int64:
                expFloat = float64(e)
            case string:
                if parsed, err := strconv.ParseFloat(e, 64); err == nil {
                    expFloat = parsed
                }
            }
            
            // Convert integer value to float64
            var floatVal float64
            switch v := intVal.(type) {
            case float64:
                floatVal = v
            case int:
                floatVal = float64(v)
            case int64:
                floatVal = float64(v)
            case string:
                if parsed, err := strconv.ParseFloat(v, 64); err == nil {
                    floatVal = parsed
                }
            }
            
            // Calculate actual decimal value
            return floatVal * math.Pow(10, expFloat)
        }
        
        // If it's not a standard numeric type but has a direct numeric value
        if val, ok := numericMap["Float64"]; ok {
            return val
        }
        
        // Return the raw value if we can't process it
        return value
    }
    
    // Handle other numeric types that might be strings
    if strVal, ok := value.(string); ok {
        if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
            return floatVal
        }
    }
    
    // Pass through non-numeric values
    return value
}
