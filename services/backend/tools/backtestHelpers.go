package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/go-redis/redis/v8"
)

// SaveBacktestToCache saves the results of a backtest to Redis.
func SaveBacktestToCache(ctx context.Context, conn *utils.Conn, userID int, strategyID int, results interface{}) error {
	if results == nil {
		return fmt.Errorf("cannot save nil backtest results")
	}

	// Construct the cache key
	cacheKey := fmt.Sprintf("user:%d:backtest:%d:results", userID, strategyID)

	// Serialize the results to JSON
	serializedResults, err := json.Marshal(results)
	if err != nil {
		fmt.Printf("Failed to serialize backtest results for strategy %d: %v\n", strategyID, err)
		return fmt.Errorf("failed to serialize backtest results: %w", err)
	}

	// Define an expiration time (e.g., 24 hours)
	expiration := 24 * time.Hour

	// Save to Redis
	fmt.Printf("Saving backtest results for strategy %d to cache key: %s\n", strategyID, cacheKey)
	err = conn.Cache.Set(ctx, cacheKey, serializedResults, expiration).Err()
	if err != nil {
		fmt.Printf("Failed to save backtest results to Redis for strategy %d: %v\n", strategyID, err)
		return fmt.Errorf("failed to save backtest results to cache: %w", err)
	}

	fmt.Printf("Successfully saved backtest results for strategy %d to Redis.\n", strategyID)
	return nil
}

type CalculateBacktestStatisticArgs struct {
	StrategyID      int    `json:"strategyId"`
	ColumnName      string `json:"columnName"`
	CalculationType string `json:"calculationType"` // e.g., "average", "sum", "min", "max", "count"
}

// CalculateBacktestStatistic retrieves cached backtest results and performs a calculation on a specific column.
func CalculateBacktestStatistic(conn *utils.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args CalculateBacktestStatisticArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args for CalculateBacktestStatistic: %v", err)
	}
	fmt.Printf("\n\n\nCalculating statistic for strategy %d, column %s, type %s\n", args.StrategyID, args.ColumnName, args.CalculationType)
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:%d:backtest:%d:results", userID, args.StrategyID)

	// Get the cached data
	cachedValue, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		// Handle cache miss specifically
		if err == redis.Nil { // Use redis.Nil from the redis library
			return nil, fmt.Errorf("no cached backtest results found for strategy %d", args.StrategyID)
		}
		return nil, fmt.Errorf("failed to retrieve cached backtest results for strategy %d: %w", args.StrategyID, err)
	}

	// Deserialize the conversation data
	var backtestResults map[string]interface{}
	if err := json.Unmarshal([]byte(cachedValue), &backtestResults); err != nil {
		return nil, fmt.Errorf("failed to deserialize cached backtest results for strategy %d: %w", args.StrategyID, err)
	}

	// Access the instances
	instancesData, ok := backtestResults["instances"]
	if !ok {
		return nil, fmt.Errorf("cached data for strategy %d does not contain 'instances' key", args.StrategyID)
	}

	instances, ok := instancesData.([]interface{})
	if !ok {
		return nil, fmt.Errorf("cached 'instances' data for strategy %d is not an array", args.StrategyID)
	}

	if len(instances) == 0 {
		return 0.0, nil // Or handle as an error? Return 0 for calculations on empty set.
	}

	var values []float64
	// Iterate through instances and extract values
	for i, instanceInterface := range instances {
		instance, ok := instanceInterface.(map[string]interface{})
		if !ok {
			fmt.Printf("Warning: Instance %d for strategy %d is not a map, skipping.\n", i, args.StrategyID)
			continue
		}

		valueInterface, ok := instance[args.ColumnName]
		if !ok {
			// Column might legitimately not exist in some rows, skip silently or log optionally
			// fmt.Printf("Warning: Column '%s' not found in instance %d for strategy %d.\n", args.ColumnName, i, args.StrategyID)
			continue
		}

		// Attempt to convert value to float64
		valueFloat, ok := valueInterface.(float64)
		if !ok {
			fmt.Printf("Warning: Value for column '%s' in instance %d for strategy %d is not a float64 (%T), skipping.\n", args.ColumnName, i, args.StrategyID, valueInterface)
			continue
		}
		values = append(values, valueFloat)
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("no valid numeric values found for column '%s' in strategy %d", args.ColumnName, args.StrategyID)
	}

	// Perform calculation
	switch args.CalculationType {
	case "average":
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values)), nil
	case "sum":
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum, nil
	case "min":
		minVal := values[0]
		for _, v := range values[1:] {
			if v < minVal {
				minVal = v
			}
		}
		return minVal, nil
	case "max":
		maxVal := values[0]
		for _, v := range values[1:] {
			if v > maxVal {
				maxVal = v
			}
		}
		return maxVal, nil
	case "count":
		return float64(len(values)), nil
	// Add case for "stddev" later if needed
	default:
		return nil, fmt.Errorf("unsupported calculation type: %s. Supported types: average, sum, min, max, count", args.CalculationType)
	}
}

// TableInstructionData holds the parameters for generating a table from cached data
type TableInstructionData struct {
	StrategyID    int               `json:"strategyId"`              // strategyId
	Columns       []string          `json:"columns"`                 // Internal column names to include
	ColumnMapping map[string]string `json:"columnMapping,omitempty"` // Optional: map internal names to display names
	ColumnFormat  map[string]string `json:"columnFormat,omitempty"`  // Optional: map internal names to display formats
}

// GenerateBacktestTableFromInstruction retrieves cached backtest results and formats it
// into a table ContentChunk based on LLM instructions.
func GenerateBacktestTableFromInstruction(ctx context.Context, conn *utils.Conn, userID int, instruction TableInstructionData) (*ContentChunk, error) {

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
	if len(instruction.Columns) == 0 {
		// Even if no specific columns requested, we still need the mandatory instance column
		// return nil, fmt.Errorf("no columns specified in table instruction")
		fmt.Println("Warning: No specific columns requested in backtest_table instruction, defaulting to Instance column only.")
	}

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
					row[i] = fmt.Sprintf("$$$%s-%d$$$", tickerVal, int64(timestampVal))
				}
				continue // Move to next column
			}

			// Handle regular columns
			if value, exists := instance[internalColName]; exists {

				// Handle nil values explicitly first
				if value == nil {
					row[i] = "N/A"
					continue
				}

				// Special handling for explicit timestamp (convert ms to readable string)
				if internalColName == "timestamp" {
					if tsMillis, ok := value.(float64); ok {
						row[i] = time.UnixMilli(int64(tsMillis)).Format(time.RFC3339)
					} else {
						// If timestamp exists but isn't a float, treat as invalid for display
						row[i] = "N/A"
					}
					continue // Skip further processing for timestamp column
				}

				// 2. General formatting/rounding for other columns (value is not nil here)
				formattedValue := value // Start with the non-nil raw value

				// Apply formatting/rounding only if it's a float64
				if floatVal, ok := value.(float64); ok {
					// Check for custom format string
					if formatStr, formatExists := instruction.ColumnFormat[internalColName]; formatExists {
						// Use custom format
						formattedValue = fmt.Sprintf(formatStr, floatVal)
					} else {
						// Apply default rounding (2 decimal places) if no format specified
						formattedValue = math.Round(floatVal*100) / 100
					}
				}
				// else: non-float values remain as `formattedValue = value`

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
