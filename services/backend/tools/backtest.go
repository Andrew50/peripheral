package tools

import (
	"backend/utils"
	"context"
	"database/sql" // <-- Added for sql.NullFloat64
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v4"
	"math"
	"strconv"
	"strings"
	"time"
)

type RunBacktestArgs struct {
	StrategyId    int  `json:"strategyId"`
	ReturnResults bool `json:"returnResults"`
	ReturnWindow  int  `json:"returnWindow"` // Added return window
}

// RunBacktest executes a backtest for the given strategy and calculates future returns
func RunBacktest(conn *utils.Conn, userId int, rawArgs json.RawMessage) (any, error) {
	var args RunBacktestArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Validate ReturnWindow
	if args.ReturnWindow <= 0 {
		// Provide a default or return an error if 0 or negative is invalid
		// For now, we'll proceed but the calculation won't make sense.
		// Consider adding validation: return nil, fmt.Errorf("returnWindow must be positive")
		fmt.Printf("Warning: returnWindow is %d, return calculation might not be meaningful.\n", args.ReturnWindow)
	}

	fmt.Println("backtesting strategyId:", args.StrategyId)

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

	// --- BEGIN: Calculate N-Day Returns ---
	if args.ReturnWindow > 0 && len(records) > 0 {
		fmt.Printf("Calculating %d-Day Returns for %d results...\n", args.ReturnWindow, len(records))
		returnColumnName := fmt.Sprintf("%d Day Return %%", args.ReturnWindow)

		// Prepare the query once
		// Fetches the close price at the start timestamp and the first close price
		// on or after the timestamp + returnWindow days.
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
		// Use CROSS JOIN because future_price is guaranteed to have 0 or 1 row.
		// If we need to handle cases where start_data might be missing, a different join/structure might be needed.
		// A simpler alternative if start_data is guaranteed by the backtest:
		/*
			WITH target_date AS (
				SELECT ($2::timestamp + $3 * interval '1 day')::date as date -- Calculate target date
			), future_price AS (
				SELECT close
				FROM ohlcv_1d
				WHERE securityid = $1
				AND timestamp::date >= (SELECT date FROM target_date)
				ORDER BY timestamp ASC
				LIMIT 1
			)
			SELECT
				t1.close AS start_close,
				fp.close AS end_close
			FROM ohlcv_1d t1
			LEFT JOIN future_price fp ON true -- Left join guarantees the start row is returned
			WHERE t1.securityid = $1
			AND t1.timestamp = $2
			LIMIT 1;
		*/


		for _, record := range records {
			// Extract necessary info, handling potential type issues
			secIdAny, okSecId := record["securityid"]
			tsAny, okTs := record["timestamp"]

			if !okSecId || !okTs {
				fmt.Println("Warning: Skipping return calculation for a record due to missing securityid or timestamp.")
				record[returnColumnName] = nil // Set return to nil if key info is missing
				continue
			}

			// Convert securityId to int (adjust type if necessary, e.g., int64)
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
				fmt.Printf("Warning: Skipping return calculation for a record due to unexpected securityid type: %T\n", secIdAny)
				record[returnColumnName] = nil
				continue
			}


			// Convert timestamp to time.Time
			var startTime time.Time
			switch t := tsAny.(type) {
			case time.Time:
				startTime = t
			// Add handling for other potential timestamp representations if needed (e.g., string, int64)
			default:
				fmt.Printf("Warning: Skipping return calculation for record with securityid %d due to unexpected timestamp type: %T\n", securityId, tsAny)
				record[returnColumnName] = nil
				continue
			}

			// --- Execute query to get start and end prices ---
			var startClose, endClose sql.NullFloat64 // Use NullFloat64 for safety

			err := conn.DB.QueryRow(ctx, returnQuery, securityId, startTime, args.ReturnWindow).Scan(&startClose, &endClose)

			if err != nil {
				if err == pgx.ErrNoRows {
					// This case means the *starting* price wasn't found in ohlcv_1d, which is odd if the backtest found it.
					fmt.Printf("Warning: No starting price found in ohlcv_1d for securityId %d at %v. Setting return to nil.\n", securityId, startTime)
					record[returnColumnName] = nil
				} else {
					// Other potential errors (DB connection, query syntax)
					fmt.Printf("Error fetching return data for securityId %d at %v: %v. Setting return to nil.\n", securityId, startTime, err)
					record[returnColumnName] = nil
				}
				continue // Move to the next record
			}

			// --- Calculate percentage change ---
			if startClose.Valid && endClose.Valid && startClose.Float64 != 0 {
				percentChange := ((endClose.Float64 - startClose.Float64) / startClose.Float64) * 100
				// Round to reasonable precision, e.g., 2 decimal places
				record[returnColumnName] = math.Round(percentChange*100) / 100
			} else {
				// Handle cases: start price missing, end price missing, or start price is 0
				if !startClose.Valid {
					fmt.Printf("Info: Start price missing for securityId %d at %v. Return set to nil.\n", securityId, startTime)
				} else if !endClose.Valid {
					fmt.Printf("Info: End price missing (%d days later) for securityId %d at %v. Return set to nil.\n", args.ReturnWindow, securityId, startTime)
				} else if startClose.Float64 == 0 {
					fmt.Printf("Info: Start price is 0 for securityId %d at %v. Cannot calculate return, setting to nil.\n", securityId, startTime)
				}
				record[returnColumnName] = nil // Assign nil if calculation cannot be done
			}
		}
		fmt.Println("Finished calculating returns.")
	}
	// --- END: Calculate N-Day Returns ---

	// Convert to interface slice for formatBacktestResults
	// This correctly handles an empty records slice, resulting in an empty recordsInterface slice.
	recordsInterface := make([]any, len(records))
	for i, record := range records {
		recordsInterface[i] = record // record now potentially includes the return column
	}

	// Format the results for LLM readability
	// formatBacktestResults handles an empty input slice correctly,
	// returning a map with empty "instances" and "summary".
	// It should now also include the new return column if it was added.
	formattedResults, err := formatBacktestResults(recordsInterface, &spec)
	if err != nil {
		// This error path should ideally not be reached if formatBacktestResults
		// handles empty input, but kept for robustness.
		return nil, fmt.Errorf("error formatting results for LLM: %v", err)
	}

	// Save the full formatted results (including instances) to cache
	// This will save the structure potentially including the return column.
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
		return formattedResults, nil // Return full results (potentially with return column)
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

	var columnNames []string
	if len(records) > 0 {
		if recordMap, ok := records[0].(map[string]any); ok {
			for key := range recordMap {
				columnNames = append(columnNames, key) // Collect all keys, including the new return column
			}
		}
	}

	for _, record := range records {
		recordMap, ok := record.(map[string]any)
		if !ok {
			fmt.Println("Warning: Skipping record during formatting as it's not a map[string]any")
			continue
		}

		instance := make(map[string]any)
		processedTimestamp := false // Flag to ensure timestamp is handled only once

		// Explicitly handle core fields first if they exist
		if ticker, exists := recordMap["ticker"]; exists {
			instance["ticker"] = ticker
		}
		if securityId, exists := recordMap["securityid"]; exists {
			instance["securityId"] = securityId // Use consistent casing
		}
		if timestampValue, exists := recordMap["timestamp"]; exists {
			// Convert timestamp logic (keeping existing implementation)
			var timestampMs int64 = -1 // Default to -1 or some indicator of failure
			switch t := timestampValue.(type) {
			case time.Time:
				// Ensure time is not zero before converting
				if !t.IsZero() {
					timestampMs = t.UnixMilli()
				} else {
					fmt.Printf("Warning: Encountered zero timestamp value for securityId %v\n", instance["securityId"])
				}
			case string:
				parsedTime, err := time.Parse(time.RFC3339Nano, t) // Try with Nano first
				if err != nil {
				    parsedTime, err = time.Parse(time.RFC3339, t) // Fallback to RFC3339
				}
				if err == nil && !parsedTime.IsZero() {
					timestampMs = parsedTime.UnixMilli()
				} else {
					fmt.Printf("Warning: Could not parse timestamp string '%v': %v\n", t, err)
					// Keep original value if parsing fails
					instance["timestamp"] = timestampValue
				}
			case nil:
				// Handle nil timestamp if necessary
				fmt.Printf("Warning: Encountered nil timestamp for securityId %v\n", instance["securityId"])
			default:
				fmt.Printf("Warning: Unhandled timestamp type: %T for value %v\n", timestampValue, timestampValue)
				// Keep original value if type is unhandled
				instance["timestamp"] = timestampValue
			}

			// Only assign timestampMs if it was successfully processed
			if timestampMs != -1 {
				instance["timestamp"] = timestampMs
			} else if _, exists := instance["timestamp"]; !exists {
			    // Ensure the key exists even if processing failed, potentially setting it to nil
			    instance["timestamp"] = nil
			}
			processedTimestamp = true
		} else {
			instance["timestamp"] = nil // Ensure key exists if missing in source
		}


		// Process all other fields dynamically
		for key, value := range recordMap {
			// Skip already handled fields
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
			// The new return column (float64 or nil) will be handled correctly by processNumericValue
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
			if tsMs, ok := instance["timestamp"].(int64); ok {
			    foundValidTime = true // Found at least one valid timestamp
				if tsMs < minTimeMs {
					minTimeMs = tsMs
				}
				if tsMs > maxTimeMs {
					maxTimeMs = tsMs
				}
			} else {
			    // Log if timestamp is not int64 as expected after processing
			    // fmt.Printf("Warning: Instance timestamp is not int64: %T, value: %v\n", instance["timestamp"], instance["timestamp"])
			}
		}

		if foundValidTime { // Only add date range if valid timestamps were found
			startTimeStr = time.UnixMilli(minTimeMs).UTC().Format(time.RFC3339) // Use UTC for consistency
			endTimeStr = time.UnixMilli(maxTimeMs).UTC().Format(time.RFC3339)   // Use UTC for consistency
			summary["date_range"] = map[string]any{
				"start_ms": minTimeMs,
				"end_ms":   maxTimeMs,
				"start":    startTimeStr,
				"end":      endTimeStr,
			}
		} else {
		     fmt.Println("Warning: No valid timestamps found in instances to calculate date range.")
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

	// Handle PostgreSQL numeric type (represented as map[string]any by pgx)
	if numericMap, ok := value.(map[string]any); ok {
		// Check standard pgx numeric representation (Status, Int, Exp, Neg)
		// Note: pgx v4/v5 might represent numeric differently. Check your pgx version's behavior.
		// This example assumes a common map structure often seen.
		statusAny, hasStatus := numericMap["Status"] // pgtype.Present, pgtype.Null etc.
		intStrAny, hasInt := numericMap["Int"]       // Usually a string representation of the integer part
		expAny, hasExp := numericMap["Exp"]          // Exponent

		// Check if it looks like the pgtype.Numeric structure
		if hasStatus && hasInt && hasExp {
			// Check if the status indicates a present (non-NULL) value
			if status, ok := statusAny.(byte); ok && status == 2 { // 2 usually means Present
				intStr, okIntStr := intStrAny.(string)
				expInt, okExp := expAny.(int32) // Exponent is often int32

				if okIntStr && okExp {
					// Attempt to construct the float value
					// This is a simplified conversion; a robust one would use big.Float
					f, _, err := new(big.Float).Parse(intStr+"e"+strconv.Itoa(int(expInt)), 10)
					if err == nil {
						floatVal, _ := f.Float64() // Get float64 representation
						return floatVal
					} else {
						fmt.Printf("Warning: Failed to parse pgtype.Numeric representation: %v\n", err)
						return value // Return original map if parsing fails
					}
				}
			} else {
				// Handle NULL status if needed, though outer nil check should catch it
				return nil
			}
		}
		// Fallback for other map structures potentially representing numbers
		// Example: Simple {"Float64": 123.45} map (less common from DB drivers)
		if floatVal, ok := numericMap["Float64"]; ok {
			return floatVal
		}
		// Could add more checks for other numeric representations if necessary
	}

	// Direct type assertions for common numeric types
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
    case []uint8: // Handle byte slices which might represent numbers
        strVal := string(v)
		if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
			return floatVal
		}
        fmt.Printf("Warning: Could not parse []uint8 as float64: %s\n", strVal)
        return strVal // Return as string if not parsable
	}

	// If none of the above, return the value as is
	fmt.Printf("Warning: Unhandled type in processNumericValue: %T, returning original value.\n", value)
	return value
}
