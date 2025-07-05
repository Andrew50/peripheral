package agent

import (
	"backend/internal/app/strategy"
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type InstanceFilter struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}
type GetBacktestInstancesArgs struct {
	StrategyID int              `json:"strategyId"`
	Filters    []InstanceFilter `json:"filters"`
}

func GetBacktestInstances(ctx context.Context, conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetBacktestInstancesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	backtestResponse, err := GetBacktestData(ctx, conn, userID, args.StrategyID)
	if err != nil {
		return nil, fmt.Errorf("error getting backtest data: %v", err)
	}

	if len(args.Filters) == 0 {
		if len(backtestResponse.Instances) > 20 {
			return backtestResponse.Instances[:20], nil
		}
		return backtestResponse.Instances, nil
	}
	filteredInstances := FilterInstances(backtestResponse.Instances, args.Filters)
	if len(filteredInstances) > 20 {
		return filteredInstances[:20], nil
	}
	return filteredInstances, nil
}

func FilterInstances(instances []strategy.BacktestInstanceRow, filters []InstanceFilter) []strategy.BacktestInstanceRow {

	var filtered []strategy.BacktestInstanceRow
	for _, instance := range instances {
		if matchesAllFilters(instance, filters) {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}

// matchesAllFilters checks if an instance matches all provided filters (AND logic)
func matchesAllFilters(instance strategy.BacktestInstanceRow, filters []InstanceFilter) bool {
	for _, filter := range filters {
		if !matchesFilter(instance, filter) {
			return false
		}
	}
	return true
}

// matchesFilter checks if an instance matches a single filter
func matchesFilter(instance strategy.BacktestInstanceRow, filter InstanceFilter) bool {
	// Extract the value from the instance
	var instanceValue any
	if instance.Instance == nil {
		return false // Field doesn't exist or is nil
	}
	instanceValue = instance.Instance[filter.Column]
	// Apply the operator
	return applyOperator(instanceValue, filter.Operator, filter.Value)
}

// applyOperator applies the comparison operator between instanceValue and filterValue
func applyOperator(instanceValue interface{}, operator string, filterValue interface{}) bool {
	switch operator {
	case "eq":
		return compareEqual(instanceValue, filterValue)
	case "gt", "gte", "lt", "lte":
		return compareNumbers(instanceValue, filterValue, operator)
	case "contains":
		return compareContains(instanceValue, filterValue)
	case "in":
		return compareIn(instanceValue, filterValue)
	default:
		return false
	}
}

// compareEqual checks equality with type conversion
func compareEqual(instanceValue, filterValue interface{}) bool {
	// Handle string comparisons
	if instStr, ok := instanceValue.(string); ok {
		if filtStr, ok := filterValue.(string); ok {
			return instStr == filtStr
		}
	}

	// Handle numeric comparisons using unified function
	if compareNumbers(instanceValue, filterValue, "eq") {
		return true
	}

	// Handle boolean comparisons
	if instBool, ok := instanceValue.(bool); ok {
		if filtBool, ok := filterValue.(bool); ok {
			return instBool == filtBool
		}
	}

	// Fallback to direct comparison
	return instanceValue == filterValue
}

// compareNumbers performs numeric comparison based on operator
func compareNumbers(instanceValue, filterValue interface{}, operator string) bool {
	instNum, instIsNum := convertToFloat64(instanceValue)
	filtNum, filtIsNum := convertToFloat64(filterValue)
	if !instIsNum || !filtIsNum {
		return false
	}

	switch operator {
	case "gt":
		return instNum > filtNum
	case "gte":
		return instNum >= filtNum
	case "lt":
		return instNum < filtNum
	case "lte":
		return instNum <= filtNum
	case "eq":
		return instNum == filtNum
	default:
		return false
	}
}

// compareContains checks if instanceValue contains filterValue (for strings)
func compareContains(instanceValue, filterValue interface{}) bool {
	instStr, instOk := instanceValue.(string)
	filtStr, filtOk := filterValue.(string)
	if instOk && filtOk && len(filtStr) > 0 {
		return strings.Contains(instStr, filtStr)
	}
	return false
}

// compareIn checks if instanceValue is in the filterValue array
func compareIn(instanceValue, filterValue interface{}) bool {
	// filterValue should be an array/slice
	switch filtArray := filterValue.(type) {
	case []interface{}:
		for _, val := range filtArray {
			if compareEqual(instanceValue, val) {
				return true
			}
		}
	case []string:
		instStr, ok := instanceValue.(string)
		if ok {
			for _, val := range filtArray {
				if instStr == val {
					return true
				}
			}
		}
	case []float64:
		instNum, ok := convertToFloat64(instanceValue)
		if ok {
			for _, val := range filtArray {
				if instNum == val {
					return true
				}
			}
		}
	}
	return false
}

// convertToFloat64 attempts to convert various numeric types to float64
func convertToFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}

func GetBacktestData(ctx context.Context, conn *data.Conn, userID int, strategyID int) (*strategy.BacktestResponse, error) {
	response, err := strategy.GetBacktestFromCache(ctx, conn, userID, strategyID)
	if err != nil {
		return nil, fmt.Errorf("error getting backtest from cache: %v", err)
	}
	return response, nil
}
