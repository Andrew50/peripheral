package agent

// GenerateBacktestTableFromInstruction retrieves cached backtest results and formats it
// into a table ContentChunk based on LLM instructions.
/* func GenerateBacktestTableFromInstruction(ctx context.Context, conn *data.Conn, userID int, instruction BacktestTableChunkData) (*ContentChunk, error) {

	cacheKey := fmt.Sprintf("user:%d:backtest:%d:results", userID, instruction.StrategyID)

	// --- Retrieve and Unmarshal Cached Data ---
	cachedValue, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("no cached backtest results found for strategy %d", instruction.StrategyID)
		}
		return nil, fmt.Errorf("failed to retrieve cached backtest results for strategy %d: %w", instruction.StrategyID, err)
	}

	var fullResults map[string]interface{}
	if err := json.Unmarshal([]byte(cachedValue), &fullResults); err != nil {
		return nil, fmt.Errorf("failed to deserialize cached backtest results for strategy %d: %w", instruction.StrategyID, err)
	}

	instancesData, ok := fullResults["instances"]
	if !ok {
		return nil, fmt.Errorf("cached backtest results for strategy %d does not contain 'instances' key", instruction.StrategyID)
	}
	instances, ok := instancesData.([]interface{})
	if !ok {
		return nil, fmt.Errorf("cached backtest 'instances' data for strategy %d is not an array", instruction.StrategyID)
	}

	// --- Process Instructions ---
	//	if len(instruction.Columns) == 0 {
	// Even if no specific columns requested, we still need the mandatory instance column
	// return nil, fmt.Errorf("no columns specified in table instruction")
	////fmt.Println("Warning: No specific columns requested in backtest_table instruction, defaulting to Instance column only.")
	//	}

	var tableHeaders []string
	var finalRows [][]interface{}
	var finalColumns []string // Store the actual columns we will process for rows

	// 1. Add mandatory "instance" column first
	finalColumns = append(finalColumns, "instance")
	instanceHeader := "Instance" // Default header
	if mappedName, exists := instruction.ColumnMapping["instance"]; exists {
		instanceHeader = mappedName
	}
	tableHeaders = append(tableHeaders, instanceHeader)

	// 2. Add other requested columns, filtering out ticker/timestamp
	requestedColumns := make(map[string]bool)
	for _, colName := range instruction.Columns {
		requestedColumns[colName] = true
	}

	for _, colName := range instruction.Columns { // Iterate again to maintain requested order somewhat
		if colName == "instance" || colName == "ticker" || colName == "timestamp" {
			continue // Skip these as "instance" covers them or they are excluded
		}

		// Add the column to be processed for rows
		finalColumns = append(finalColumns, colName)

		// Determine the display header
		displayName := colName
		if mappedName, exists := instruction.ColumnMapping[colName]; exists {
			displayName = mappedName
		}
		tableHeaders = append(tableHeaders, displayName)
	}

	numRows := 0
	for _, instanceInterface := range instances {

		instance, ok := instanceInterface.(map[string]interface{})
		if !ok {
			continue // Skip malformed instances
		}

		row := make([]interface{}, len(finalColumns))
		includeRow := true
		for i, internalColName := range finalColumns {
			// Handle the 'instance' pseudo-column
			if internalColName == "instance" {
				tickerVal, tickerOk := instance["ticker"].(string)
				timestampVal, tsOk := instance["timestamp"].(float64) // JSON numbers often float64

				if tickerOk && tsOk {
					row[i] = fmt.Sprintf("$$%s-%d$$", tickerVal, int64(timestampVal))
				}
				continue // Move to next column
			}

			// Handle regular columns
			if valueInterface, exists := instance[internalColName]; exists {

				// First, attempt to process the value in case it's a raw numeric map from cache
				processedValue := processNumericValue(valueInterface)

				// Handle nil values explicitly first (after potential conversion)
				if processedValue == nil {
					row[i] = "N/A"
					continue
				}

				// Special handling for explicit timestamp (convert ms to readable string)
				if internalColName == "timestamp" {
					if tsMillis, ok := processedValue.(float64); ok {
						row[i] = time.UnixMilli(int64(tsMillis)).Format(time.RFC3339)
					} else {
						row[i] = "N/A"
					}
					continue
				}

				// 2. General formatting/rounding for other columns
				formattedValue := processedValue // Start with the potentially converted value

				// Apply formatting/rounding only if it's a float64
				if floatVal, ok := processedValue.(float64); ok {
					// Check for custom format string
					if formatStr, formatExists := instruction.ColumnFormat[internalColName]; formatExists {
						// Check if the format string is intended for percentage display
						if strings.Contains(formatStr, "%%") {
							// Multiply by 100 for percentage formatting
							formattedValue = fmt.Sprintf(formatStr, floatVal*100)
						} else {
							// Use custom format directly for non-percentage cases
							formattedValue = fmt.Sprintf(formatStr, floatVal)
						}
					} else {
						// Apply default rounding (2 decimal places) if no format specified
						formattedValue = math.Round(floatVal*100) / 100
					}
				}
				// else: non-float values (like strings) remain as is

				row[i] = formattedValue

			} else {
				// Key doesn't exist for this instance
				row[i] = "N/A"
			}
		}

		if includeRow {
			finalRows = append(finalRows, row)
			numRows++
		}
	}

	// --- Construct Final Table Chunk ---
	tableContent := map[string]interface{}{
		"headers":             tableHeaders,
		"rows":                finalRows,
		"strategyId":          instruction.StrategyID, // Metadata for LLM context
		"internalColumnNames": finalColumns,           // Metadata for LLM context
	}

	finalChunk := &ContentChunk{
		Type:    "table",
		Content: tableContent,
	}

	return finalChunk, nil
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
*/
