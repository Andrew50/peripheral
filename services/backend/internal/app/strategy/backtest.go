package strategy

import (
	"backend/internal/data"
	"context"
	"database/sql" // <-- Added for sql.NullFloat64
	"encoding/json"
	"fmt"
	"math" // <-- Added for big.Float in processNumericValue
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"

	"github.com/jackc/pgtype"
)

type BacktestArgs struct {
	StrategyID    int   `json:"strategyId"`
	Securities    []int `json:"securities"`
	Start         int64 `json:"start"`
	ReturnWindows []int `json:"returnWindows"` // Changed to slice of ints
	FullResults   bool  `json:"fullResults"`   // New field to control output type
}

// RunBacktest executes a backtest for the given strategy and calculates future returns for multiple windows
func RunBacktest(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (any, error) {
	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var args BacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Optional: Validate ReturnWindows - Ensure they are positive? Remove duplicates?
	validWindows := []int{}
	if args.ReturnWindows != nil {
		for _, w := range args.ReturnWindows {
			if w > 0 {
				validWindows = append(validWindows, w) // Keep only positive windows
			} //else {
			////fmt.Printf("Warning: Skipping non-positive return window: %d\n", w)
			//}
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

	////fmt.Println("backtesting strategyId:", args.StrategyID)
	//if len(args.ReturnWindows) > 0 {
	////fmt.Printf("Will calculate future returns for windows (days): %v\n", args.ReturnWindows)
	//}

	backtestJSON, err := _getStrategySpec(conn, args.StrategyID, userID) // get spec from db using helper
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
	////fmt.Println("Generated SQL:", sqlQuery)

	// Check if context is cancelled before executing query
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Execute the query with context
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

	// Check if context is cancelled before processing return windows
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// --- BEGIN: Calculate N-Day Returns for Multiple Windows ---
	if len(args.ReturnWindows) > 0 && len(records) > 0 {
		////fmt.Printf("Calculating returns for %d results across %d windows...\n", len(records), len(args.ReturnWindows))

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
			// Check if context is cancelled during processing
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			// Extract necessary info once per record
			secIDAny, okSecID := record["securityid"]
			tsAny, okTs := record["timestamp"]

			if !okSecID || !okTs {
				////fmt.Println("Warning: Skipping return calculation for a record due to missing securityid or timestamp.")
				// Set all potential return columns to nil for this record
				for _, window := range args.ReturnWindows {
					returnColumnName := fmt.Sprintf("%d Day Return %%", window)
					record[returnColumnName] = nil
				}
				continue
			}

			var securityID int
			switch v := secIDAny.(type) {
			case int:
				securityID = v
			case int32:
				securityID = int(v)
			case int64:
				securityID = int(v) // Potential overflow if original is large int64
			case float64: // Handle if ID comes as float
				securityID = int(v)
			default:
				////fmt.Printf("Warning: Skipping return calculation for a record due to unexpected securityid type: %T. Value: %v\n", secIDAny, secIDAny)
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
				if err == nil && !parsedTime.IsZero() {
					startTime = parsedTime
				} else if err != nil {
					////fmt.Printf("Warning: Could not parse timestamp string '%s' for secID %d: %v\n", t, securityID, err)
					for _, window := range args.ReturnWindows {
						returnColumnName := fmt.Sprintf("%d Day Return %%", window)
						record[returnColumnName] = nil
					}
					continue
				} else { // Handle zero time after parsing
					////fmt.Printf("Warning: Parsed timestamp is zero for secID %d. Original string: %s\n", securityID, t)
					for _, window := range args.ReturnWindows {
						returnColumnName := fmt.Sprintf("%d Day Return %%", window)
						record[returnColumnName] = nil
					}
					continue
				}
			case nil:
				////fmt.Printf("Warning: Skipping return calculation for record with securityID %d due to nil timestamp.\n", securityID)
				for _, window := range args.ReturnWindows {
					returnColumnName := fmt.Sprintf("%d Day Return %%", window)
					record[returnColumnName] = nil
				}
				continue
			default:
				////fmt.Printf("Warning: Skipping return calculation for record with securityID %d due to unexpected timestamp type: %T. Value: %v\n", securityID, tsAny, tsAny)
				for _, window := range args.ReturnWindows {
					returnColumnName := fmt.Sprintf("%d Day Return %%", window)
					record[returnColumnName] = nil
				}
				continue
			}

			// Final check for zero time (should be redundant if parsing logic is correct, but safe)
			if startTime.IsZero() {
				////fmt.Printf("Warning: Skipping return calculation for record with securityID %d due to zero timestamp after processing.\n", securityID)
				for _, window := range args.ReturnWindows {
					returnColumnName := fmt.Sprintf("%d Day Return %%", window)
					record[returnColumnName] = nil
				}
				continue
			}

			// Now, loop through each requested return window for the current record
			for _, window := range args.ReturnWindows {
				// Check if context is cancelled during return calculations
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}

				returnColumnName := fmt.Sprintf("%d Day Return %%", window)

				// --- Execute query to get start and end prices for this specific window ---
				var startClose, endClose sql.NullFloat64 // Use NullFloat64 for safety

				err := conn.DB.QueryRow(ctx, returnQuery, securityID, startTime, window).Scan(&startClose, &endClose)

				if err != nil {
					if err == pgx.ErrNoRows {
						// This usually means start_data or future_price CTE returned no rows.
						// Check if start price exists separately for better debugging if needed.
						////fmt.Printf("Warning: No price data found (start or %d days later) for securityID %d at %v. Setting '%s' to nil.\n", window, securityID, startTime, returnColumnName)
						record[returnColumnName] = nil
					} else {
						// Other potential errors (DB connection, query syntax)
						////fmt.Printf("Error fetching %d-day return data for securityID %d at %v: %v. Setting '%s' to nil.\n", window, securityID, startTime, err, returnColumnName)
						record[returnColumnName] = nil
					}
					continue // Continue to the next window for this record
				}

				// --- Calculate percentage change for this window ---
				if startClose.Valid && endClose.Valid && startClose.Float64 != 0 {
					// Calculate the change as a decimal
					decimalChange := (endClose.Float64 - startClose.Float64) / startClose.Float64
					// Store as decimal, rounded to 4 places for reasonable precision
					record[returnColumnName] = math.Round(decimalChange*10000) / 10000
				} else {
					// Handle cases: start price missing, end price missing, or start price is 0
					// Log specific reason for nil result for this window
					//if !startClose.Valid {
					// This is less likely if the cross join query succeeded without pgx.ErrNoRows, but check anyway.
					////fmt.Printf("Info: Start price missing for securityID %d at %v. Return '%s' set to nil.\n", securityID, startTime, returnColumnName)
					//}
					/*else if !endClose.Valid {
						////fmt.Printf("Info: End price missing (%d days later) for securityID %d at %v. Return '%s' set to nil.\n", window, securityID, startTime, returnColumnName)
					} else if startClose.Float64 == 0 {
						////fmt.Printf("Info: Start price is 0 for securityID %d at %v. Cannot calculate %d-day return, setting '%s' to nil.\n", securityID, startTime, window, returnColumnName)
					}*/
					record[returnColumnName] = nil // Assign nil if calculation cannot be done
				}
			} // End loop over windows
		} // End loop over records
		////fmt.Println("Finished calculating returns for all windows.")
	}
	// --- END: Calculate N-Day Returns ---

	// Check if context is cancelled before final processing
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

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
	//////fmt.Println("\n\n FORMATTED RESULTS: ", formattedResults)
	if err != nil {
		// This error path should ideally not be reached if formatBacktestResults
		// handles empty input, but kept for robustness.
		return nil, fmt.Errorf("error formatting results for LLM: %v", err)
	}

	// Save the full formatted results (including instances and return columns) to cache
	//go func() { // Run in a goroutine to avoid blocking the main response
	if err := SaveBacktestToCache(ctx, conn, userID, args.StrategyID, formattedResults); err != nil {
		////fmt.Printf("Warning: Failed to save backtest results to cache for strategy %d: %v\n", args.StrategyID, err)
		// We log the error but don't fail the main operation
		return nil, err
	}
	//}()

	// Extract only the summary to return to the LLM
	summary, ok := formattedResults["summary"].(map[string]any)
	if !ok {
		// This should ideally not happen if formatBacktestResults worked correctly
		return nil, fmt.Errorf("failed to extract summary from formatted backtest results (internal error)")
	}
	////fmt.Println("\n\n SUMMARY: ", summary)

	if args.FullResults {
		return formattedResults, nil // Return full results (potentially with multiple return columns)
	}
	// else, return only the summary
	return summary, nil // Return only the summary
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
			/*if pgxErr, ok := err.(*pgx.ScanArgError); ok {
				////fmt.Printf("Scan error: Column '%s', Index %d. Expected Go type compatible with DB type OID %d. Received type: %T\n", "idk", pgxErr.ColumnIndex, rows.FieldDescriptions()[pgxErr.ColumnIndex].DataTypeOID, values[pgxErr.ColumnIndex])
			}*/
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

	// Create a clean array of instances
	instances := make([]map[string]any, 0, len(records))

	// --- Sample Collection Initialization ---
	columnSamples := make(map[string][]any)
	sampleCounts := make(map[string]int)
	const maxSamples = 3 // Number of samples to collect per column
	// --- End Sample Collection Initialization ---

	featureMap := make(map[string]string) // Maps "f0", "f1" to actual feature names
	if spec != nil {
		for _, feature := range spec.Features {
			featureKey := fmt.Sprintf("f%d", feature.FeatureID)
			featureMap[featureKey] = feature.Name
		}
	}

	for _, record := range records {
		recordMap, ok := record.(map[string]any)
		if !ok {
			////fmt.Println("Warning: Skipping record during formatting as it's not a map[string]any")
			continue
		}

		instance := make(map[string]any)
		processedTimestamp := false // Flag to ensure timestamp is handled only once

		// Explicitly handle core fields first if they exist, for potential ordering preference
		if ticker, exists := recordMap["ticker"]; exists {
			instance["ticker"] = ticker
		}
		if securityID, exists := recordMap["securityid"]; exists {
			instance["securityID"] = securityID // Use consistent casing
		}
		if timestampValue, exists := recordMap["timestamp"]; exists {
			// Convert timestamp logic
			var timestampMs int64 = -1
			switch t := timestampValue.(type) {
			case time.Time:
				if !t.IsZero() {
					timestampMs = t.UnixMilli()
				}
				//else {
				////fmt.Printf("Warning format: Encountered zero timestamp value for securityID %v\n", instance["securityID"])
				//}
			case string:
				parsedTime, err := time.Parse(time.RFC3339Nano, t)
				if err != nil {
					parsedTime, err = time.Parse(time.RFC3339, t)
				}
				if err == nil && !parsedTime.IsZero() {
					timestampMs = parsedTime.UnixMilli()
				} else {
					////fmt.Printf("Warning format: Could not parse timestamp string '%v': %v\n", t, err)
					instance["timestamp"] = timestampValue // Keep original if parsing fails
				}
			case int64: // Handle if timestamp is already processed to ms
				timestampMs = t
			case nil:
				////fmt.Printf("Warning format: Encountered nil timestamp for securityID %v\n", instance["securityID"])
			default:
				////fmt.Printf("Warning format: Unhandled timestamp type: %T for value %v\n", timestampValue, timestampValue)
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
		for key := range recordMap {
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

		// --- Collect Samples ---
		for colName, value := range instance {
			if sampleCounts[colName] < maxSamples && value != nil {
				// Ensure the sample itself is processed, just in case it wasn't fully converted earlier
				processedSample := processNumericValue(value)
				columnSamples[colName] = append(columnSamples[colName], processedSample)
				sampleCounts[colName]++
			}
		}
		// --- End Collect Samples ---
	}

	// Create a summary
	summary := make(map[string]any)
	summary["count"] = len(instances)
	summary["columnSamples"] = columnSamples
	if len(instances) > 0 {
		var minTimeMs int64 = math.MaxInt64
		var maxTimeMs int64
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
		}
		//else {
		////fmt.Println("Warning format: No valid, positive timestamps found in instances to calculate date range.")
		//}
	}

	result["instances"] = instances
	result["summary"] = summary

	return result, nil
}

// Helper function to process numeric values from the database
func processNumericValue(value any) any {
	// --- ADDED: Handle pgtype.Numeric directly ---
	if pgNum, ok := value.(pgtype.Numeric); ok {
		var f float64
		err := pgNum.AssignTo(&f) // Try to assign the numeric value to a float64
		if err == nil {
			return f // Return the float64 if successful
		}
		// If AssignTo fails, fall through to map handling or return original
		////fmt.Printf("Warning: Failed to assign pgtype.Numeric to float64: %v\n", err)
	}
	// --- END ADDED SECTION ---

	// Handle PostgreSQL numeric type represented as map[string]any (fallback or other cases)
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
