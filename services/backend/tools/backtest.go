package tools

import (
	"backend/utils"
	"context"
	"database/sql" // <-- Added for sql.NullFloat64
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v4"
	"math"
	"math/big" // <-- Added for big.Float in processNumericValue
	"strconv"
	"strings"
	"time"
	// "sort" // <-- Might be needed if sorting columnNames in formatBacktestResults is desired
)

type RunBacktestArgs struct {
	StrategyId    int   `json:"strategyId"`
	ReturnResults bool  `json:"returnResults"`
	ReturnWindows []int `json:"returnWindows"` // Changed to slice of ints
}

// RunBacktest executes a backtest for the given strategy and calculates future returns for multiple windows
func RunBacktest(conn *utils.Conn, userId int, rawArgs json.RawMessage) (any, error) {
	var args RunBacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Optional: Validate ReturnWindows - Ensure they are positive? Remove duplicates?
	validWindows := []int{}
	if args.ReturnWindows != nil {
		for _, w := range args.ReturnWindows {
			if w > 0 {
				validWindows = append(validWindows, w) // Keep only positive windows
			} else {
				fmt.Printf("Warning: Skipping non-positive return window: %d\n", w)
			}
		}
		// Remove duplicates (optional)
		// uniqueWindows := make(map[int]bool)
		// tempWindows := []int{}
		// for _, w := range validWindows {
		//  if !uniqueWindows[w] {
		//      uniqueWindows[w] = true
		//      tempWindows = append(tempWindows, w)
		//  }
		// }
		// validWindows = tempWindows
		args.ReturnWindows = validWindows // Use the filtered list
	}


	fmt.Println("backtesting strategyId:", args.StrategyId)
	if len(args.ReturnWindows) > 0 {
		fmt.Printf("Will calculate future returns for windows (days): %v\n", args.ReturnWindows)
	}

	backtestJSON, err := _getStrategySpec(conn, args.StrategyId, userId) // get spec from db using helper
	if err != nil {
		return nil, fmt.Errorf("ERR vdi0s: failed to fetch strategy: %v", err)
	}

	var spec Spec
	if err := json.Unmarshal((backtestJSON), &spec); err != nil { //unmarshal into struct
		return "", fmt.Errorf("ERR fi00: error parsing backtest JSON: %v", err)
	}

	// *** New approach using CompileSpecToSQL ***
	// Generate SQL from the spec
	sqlQuery, err := CompileSpecToSQL(spec) // Renamed 'sql' to 'sqlQuery' to avoid conflict
	if err != nil {
		return nil, fmt.Errorf("error compiling SQL for backtest: %v", err)
	}
	fmt.Println("Generated SQL:", sqlQuery)

	// Execute the query
	ctx := context.Background()
	rows, err := conn.DB.Query(ctx, sqlQuery)
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

	// --- BEGIN: Calculate N-Day Returns for Multiple Windows ---
	if len(args.ReturnWindows) > 0 && len(records) > 0 {
		fmt.Printf("Calculating %d-Day Returns for %d results across %d windows...\n", len(args.ReturnWindows), len(records), len(args.ReturnWindows))

		// Prepare the query once (remains the same structure)
		returnQuery := `
            WITH start_data AS (
                SELECT timestamp, close
                FROM ohlcv_1d
                WHERE securityid = $1 AND timestamp = $2
                LIMIT 1
            ), target_date AS (
                -- Calculate the date ReturnWindow days after the start timestamp
                SELECT ($2::timestamp + $3 * interval '1 day')::date AS date
            ), future_price AS (
                -- Find the first closing price on or after the target date
                SELECT close
                FROM ohlcv_1d
                WHERE securityid = $1
                  AND timestamp::date >= (SELECT date FROM target_date)
                ORDER BY timestamp ASC
                LIMIT 1
            )
            SELECT
                sd.close AS start_close,
                fp.close AS end_close
            FROM start_data sd
            CROSS JOIN future_price fp; -- Use CROSS JOIN as future_price returns at most one row
        `

		// Loop through each result from the initial backtest

		for _, record := range records {
			// Extract necessary info once per record
			secIdAny, okSecId := record["securityid"]
			tsAny, okTs := record["timestamp"]

			if !okSecId || !okTs {
				fmt.Println("Warning: Skipping return calculation for a record due to missing securityid or timestamp.")
				// Set all potential return columns to nil for this record
				for _, window := range args.ReturnWindows {
					returnColumnName := fmt.Sprintf("%d Day Return %%", window)
					record[returnColumnName] = nil
				}
				continue
			}

			// Convert securityId once per record
			var securityId int
			switch v := secIdAny.(type) {
			case int:
				securityId = v
			case int32:
				securityId = int(v)
			case int64:
				securityId = int(v) // Potential overflow if original is large int64
			case float64: // Handle if ID comes as float
				securityId = int(v)
			default:
				fmt.Printf("Warning: Skipping return calculation for a record due to unexpected securityid type: %T. Value: %v\n", secIdAny, secIdAny)
				for _, window := range args.ReturnWindows {
					returnColumnName := fmt.Sprintf("%d Day Return %%", window)
					record[returnColumnName] = nil
				}
				continue
			}

			// Convert timestamp once per record
			var startTime time.Time
			switch t := tsAny.(type) {
			case time.Time:
				startTime = t
			case string: // Handle potential string timestamp from initial query
				parsedTime, err := time.Parse(time.RFC3339Nano, t)
				if err != nil {
					parsedTime, err = time.Parse(time.RFC3339, t)
				}
				if err == nil {
					startTime = parsedTime
				} else {
					fmt.Printf("Warning: Could not parse timestamp string '%s' for secId %d: %v\n", t, securityId, err)
					for _, window := range args.ReturnWindows {
						returnColumnName := fmt.Sprintf("%d Day Return %%", window)
						record[returnColumnName] = nil
					}
					continue
				}
			default:
				fmt.Printf("Warning: Skipping return calculation for record with securityid %d due to unexpected timestamp type: %T. Value: %v\n", securityId, tsAny, tsAny)
				for _, window := range args.ReturnWindows {
					returnColumnName := fmt.Sprintf("%d Day Return %%", window)
					record[returnColumnName] = nil
				}
				continue
			}

			// Check for zero time
			if startTime.IsZero() {
				fmt.Printf("Warning: Skipping return calculation for record with securityid %d due to zero timestamp.\n", securityId)
				for _, window := range args.ReturnWindows {
					returnColumnName := fmt.Sprintf("%d Day Return %%", window)
					record[returnColumnName] = nil
				}
				continue
			}


			// Now, loop through each requested return window for the current record
			for _, window := range args.ReturnWindows {
				returnColumnName := fmt.Sprintf("%d Day Return %%", window)

				// --- Execute query to get start and end prices for this specific window ---
				var startClose, endClose sql.NullFloat64 // Use NullFloat64 for safety

				err := conn.DB.QueryRow(ctx, returnQuery, securityId, startTime, window).Scan(&startClose, &endClose)

				if err != nil {
					if err == pgx.ErrNoRows {
						// This usually means start_data or future_price CTE returned no rows.
						// Check if start price exists separately for better debugging if needed.
						fmt.Printf("Warning: No price data found (start or %d days later) for securityId %d at %v. Setting '%s' to nil.\n", window, securityId, startTime, returnColumnName)
						record[returnColumnName] = nil
					} else {
						// Other potential errors (DB connection, query syntax)
						fmt.Printf("Error fetching %d-day return data for securityId %d at %v: %v. Setting '%s' to nil.\n", window, securityId, startTime, err, returnColumnName)
						record[returnColumnName] = nil
					}
					continue // Continue to the next window for this record
				}

				// --- Calculate percentage change for this window ---
				if startClose.Valid && endClose.Valid && startClose.Float64 != 0 {
					percentChange := ((endClose.Float64 - startClose.Float64) / startClose.Float64) * 100
					// Round to reasonable precision, e.g., 2 decimal places
					record[returnColumnName] = math.Round(percentChange*100) / 100
				} else {
					// Handle cases: start price missing, end price missing, or start price is 0
					// Log specific reason for nil result for this window
					if !startClose.Valid {
						// This is less likely if the cross join query succeeded without pgx.ErrNoRows, but check anyway.
						fmt.Printf("Info: Start price missing for securityId %d at %v. Return '%s' set to nil.\n", securityId, startTime, returnColumnName)
					} else if !endClose.Valid {
						fmt.Printf("Info: End price missing (%d days later) for securityId %d at %v. Return '%s' set to nil.\n", window, securityId, startTime, returnColumnName)
					} else if startClose.Float64 == 0 {
						fmt.Printf("Info: Start price is 0 for securityId %d at %v. Cannot calculate %d-day return, setting '%s' to nil.\n", securityId, startTime, window, returnColumnName)
					}
					record[returnColumnName] = nil // Assign nil if calculation cannot be done
				}
			} // End loop over windows
		} // End loop over records
		fmt.Println("Finished calculating returns for all windows.")
	}
	// --- END: Calculate N-Day Returns ---

	// Convert to interface slice for formatBacktestResults
	// This correctly handles an empty records slice, resulting in an empty recordsInterface slice.
	recordsInterface := make([]any, len(records))
	for i, record := range records {
		recordsInterface[i] = record // record now potentially includes multiple return columns
	}

	// Format the results for LLM readability
	// formatBacktestResults handles an empty input slice correctly,
	// returning a map with empty "instances" and "summary".
	// It should now also include the new return columns if they were added.
	formattedResults, err := formatBacktestResults(recordsInterface, &spec)
	if err != nil {
		// This error path should ideally not be reached if formatBacktestResults
		// handles empty input, but kept for robustness.
		return nil, fmt.Errorf("error formatting results for LLM: %v", err)
	}

	// Save the full formatted results (including instances and return columns) to cache
	go func() { // Run in a goroutine to avoid blocking the main response
		bgCtx := context.Background() // Use a background context for the goroutine
		if err := SaveBacktestToCache(bgCtx, conn, userId, args.StrategyId, formattedResults); err != nil {
			fmt.Printf("Warning: Failed to save backtest results to cache for strategy %d: %v\n", args.StrategyId, err)
			// We log the error but don't fail the main operation
		}
	}()

	// Extract only the summary to return to the LLM
	summary, ok := formattedResults["summary"].(map[string]any)
	if !ok {
		// This should ideally not happen if formatBacktestResults worked correctly
		return nil, fmt.Errorf("failed to extract summary from formatted backtest results (internal error)")
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

	if args.ReturnResults {
		return formattedResults, nil // Return full results (potentially with multiple return columns)
	} else {
		return summary, nil // Return only the summary
	}
}

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
		// Use pointers to handle potential NULL values correctly
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Scan the row into the slice of pointers
		if err := rows.Scan(valuePtrs...); err != nil {
			// Check if the error is about incompatible types, often useful for debugging
             if pgxErr, ok := err.(*pgx.ScanArgError); ok {
                 fmt.Printf("Scan error: Column '%s', Index %d. Expected Go type compatible with DB type OID %d. Received type: %T\n",
                     "idk", pgxErr.ColumnIndex, rows.FieldDescriptions()[pgxErr.ColumnIndex].DataTypeOID, values[pgxErr.ColumnIndex])
             }
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Create a map for this row
		row := make(map[string]any)
		for i, col := range columns {
			// Dereference the pointer to get the actual value.
			// If the DB value was NULL, the corresponding `values[i]` will be `nil`.
			row[col] = values[i]
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
// This function does NOT need changes, as it dynamically handles all keys present in the records.
func formatBacktestResults(records []any, spec *Spec) (map[string]any, error) {
	result := make(map[string]any)
	instances := make([]map[string]any, 0, len(records))

	featureMap := make(map[string]string) // Maps "f0", "f1" to actual feature names
	if spec != nil {
		for _, feature := range spec.Features {
			featureKey := fmt.Sprintf("f%d", feature.FeatureId)
			featureMap[featureKey] = feature.Name
		}
	}

	// Dynamically get all column names from the first record (if available)
	// This ensures all columns, including dynamically added return columns, are processed.
	// Note: This assumes all records have the same set of columns, which should be true here.
	// var columnNames []string
	// if len(records) > 0 {
	// 	if recordMap, ok := records[0].(map[string]any); ok {
	// 		for key := range recordMap {
	// 			columnNames = append(columnNames, key)
	// 		}
	// 		// Optional: Sort column names for consistent output order
	// 		// sort.Strings(columnNames)
	// 	}
	// }

	for _, record := range records {
		recordMap, ok := record.(map[string]any)
		if !ok {
			fmt.Println("Warning: Skipping record during formatting as it's not a map[string]any")
			continue
		}

		instance := make(map[string]any)
		processedTimestamp := false // Flag to ensure timestamp is handled only once

		// Explicitly handle core fields first if they exist, for potential ordering preference
		if ticker, exists := recordMap["ticker"]; exists {
			instance["ticker"] = ticker
		}
		if securityId, exists := recordMap["securityid"]; exists {
			instance["securityId"] = securityId // Use consistent casing
		}
        if timestampValue, exists := recordMap["timestamp"]; exists {
			// Convert timestamp logic
			var timestampMs int64 = -1
			switch t := timestampValue.(type) {
			case time.Time:
				if !t.IsZero() {
					timestampMs = t.UnixMilli()
				} else {
					fmt.Printf("Warning format: Encountered zero timestamp value for securityId %v\n", instance["securityId"])
				}
			case string:
				parsedTime, err := time.Parse(time.RFC3339Nano, t)
				if err != nil {
				    parsedTime, err = time.Parse(time.RFC3339, t)
				}
				if err == nil && !parsedTime.IsZero() {
					timestampMs = parsedTime.UnixMilli()
				} else {
					fmt.Printf("Warning format: Could not parse timestamp string '%v': %v\n", t, err)
					instance["timestamp"] = timestampValue // Keep original if parsing fails
				}
			case int64: // Handle if timestamp is already processed to ms
				timestampMs = t
			case nil:
				fmt.Printf("Warning format: Encountered nil timestamp for securityId %v\n", instance["securityId"])
			default:
				fmt.Printf("Warning format: Unhandled timestamp type: %T for value %v\n", timestampValue, timestampValue)
				instance["timestamp"] = timestampValue // Keep original if unhandled
			}

			if timestampMs != -1 {
				instance["timestamp"] = timestampMs
			} else if _, exists := instance["timestamp"]; !exists {
			    instance["timestamp"] = nil // Ensure key exists even if processing failed
			}
			processedTimestamp = true
		} else {
			instance["timestamp"] = nil // Ensure key exists if missing in source
		}


		// Process all *other* fields dynamically using the keys from the recordMap
		for key, value := range recordMap {
			// Skip already handled standard fields
			if key == "ticker" || key == "securityid" || (key == "timestamp" && processedTimestamp) {
				continue
			}

			// Determine the final column name (use feature name if mapped)
			columnName := key
			if spec != nil && strings.HasPrefix(key, "f") {
				if mappedName, ok := featureMap[key]; ok {
					columnName = mappedName // Use the proper feature name from spec
				}
			}

			// Process the value (handle numeric types, pass others through)
			// This will correctly handle the new return columns (float64 or nil)
			instance[columnName] = processNumericValue(value)
		}

		instances = append(instances, instance)
	}

	// Create a summary
	summary := make(map[string]any)
	summary["count"] = len(instances)

	if len(instances) > 0 {
		var minTimeMs int64 = math.MaxInt64
		var maxTimeMs int64 = 0
		var startTimeStr, endTimeStr string
		foundValidTime := false

		for _, instance := range instances {
			// Use type assertion with check
			if tsMs, ok := instance["timestamp"].(int64); ok && tsMs > 0 { // Ensure timestamp is positive int64
			    foundValidTime = true
				if tsMs < minTimeMs {
					minTimeMs = tsMs
				}
				if tsMs > maxTimeMs {
					maxTimeMs = tsMs
				}
			} else if tsMs == 0 {
                // Optionally log zero timestamps if they are unexpected
                // fmt.Printf("Debug format: Instance has zero timestamp (ms) for secId %v\n", instance["securityId"])
			} else {
			    // Log if timestamp is not int64 as expected after processing
			    // fmt.Printf("Warning format: Instance timestamp is not int64 or is invalid: Type %T, value: %v\n", instance["timestamp"], instance["timestamp"])
			}
		}

		if foundValidTime { // Only add date range if valid, non-zero timestamps were found
			startTimeStr = time.UnixMilli(minTimeMs).UTC().Format(time.RFC3339) // Use UTC for consistency
			endTimeStr = time.UnixMilli(maxTimeMs).UTC().Format(time.RFC3339)   // Use UTC for consistency
			summary["date_range"] = map[string]any{
				"start_ms": minTimeMs,
				"end_ms":   maxTimeMs,
				"start":    startTimeStr,
				"end":      endTimeStr,
			}
		} else {
		     fmt.Println("Warning format: No valid, positive timestamps found in instances to calculate date range.")
		}
	}

	result["instances"] = instances
	result["summary"] = summary

	return result, nil
}


// Helper function to process numeric values from the database
func processNumericValue(value any) any {
    // Handle nil directly
    if value == nil {
        return nil
    }

	// Handle PostgreSQL numeric type (represented as map[string]any by pgx v4)
	// Note: pgx v5 uses pgtype.Numeric directly. This code assumes v4 behavior.
	if numericMap, ok := value.(map[string]any); ok {
		statusAny, hasStatus := numericMap["Status"]
		intStrAny, hasInt := numericMap["Int"]
		expAny, hasExp := numericMap["Exp"]

		// Check if it strongly resembles the pgtype.Numeric structure from pgx v4
		if hasStatus && hasInt && hasExp {
			if status, ok := statusAny.(byte); ok && (status == 2 || status == 1) { // 2=Present, 1=Present in pgx v4? Check docs. Assume 2 is main one.
				intStr, okIntStr := intStrAny.(string)
				expInt, okExp := expAny.(int32)

				if okIntStr && okExp {
					// Use big.Float for accurate conversion from scientific notation parts
					f := new(big.Float)
					_, _, err := f.Parse(intStr+"e"+strconv.Itoa(int(expInt)), 10) // Use base 10

					if err == nil {
						// Check for negative sign if the map has it separately (common pgx v4 pattern)
						if neg, hasNeg := numericMap["Negative"]; hasNeg {
							if isNeg, okIsNeg := neg.(bool); okIsNeg && isNeg {
								f.Neg(f) // Make the big.Float negative
							}
						}

						floatVal, _ := f.Float64() // Convert big.Float to float64
						// Check for infinity resulting from overflow
						if math.IsInf(floatVal, 0) {
							fmt.Printf("Warning: pgx numeric conversion resulted in infinity for %v\n", numericMap)
							// Decide how to handle infinity: return nil, max/min float, or keep original map?
							return nil // Return nil for simplicity
						}
						return floatVal
					} else {
						fmt.Printf("Warning: Failed to parse big.Float from pgtype.Numeric map parts: %v, Map: %v\n", err, numericMap)
						return value // Return original map if parsing fails
					}
				}
			} else if status, ok := statusAny.(byte); ok && status == 0 { // Status 0 usually means NULL
                return nil
            }
		}
		// Fallback check for other map structures (less likely from pgx but possible)
		if floatVal, ok := numericMap["Float64"]; ok {
			if fv, okf := floatVal.(float64); okf { return fv }
		}
	}

	// Direct type assertions for Go standard numeric types
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		// Try parsing string as float
		if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
			return floatVal
		}
		// If not parsable as float, return original string
		return v
    case []uint8: // Handle byte slices which might represent numbers (e.g., from certain DB types)
        strVal := string(v)
		if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
			return floatVal
		}
        // If not parsable, return as string
        // fmt.Printf("Warning: Could not parse []uint8 as float64: %s\n", strVal)
        return strVal
	}

	// If none of the above conversions worked, return the value as is
	// Consider logging this case if it's unexpected.
	// fmt.Printf("Debug: Unhandled type in processNumericValue: %T, passing through value: %v\n", value, value)
	return value
}

