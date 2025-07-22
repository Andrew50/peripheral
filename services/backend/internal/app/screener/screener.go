package screener

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ColumnType represents the data type of a column
type ColumnType string

const (
	TypeInteger ColumnType = "integer"
	TypeFloat   ColumnType = "float"
	TypeString  ColumnType = "string"
	TypeBoolean ColumnType = "boolean"
)

// ColumnInfo holds metadata about a screener column
type ColumnInfo struct {
	Name        string
	Type        ColumnType
	AllowedOps  []string
	Description string
}

// Screener column definitions based on the database schema
var screenerColumns = map[string]ColumnInfo{
	// Primary key
	"security_id": {
		Name:        "security_id",
		Type:        TypeInteger,
		AllowedOps:  []string{"IN", "topn", "bottomn"},
		Description: "Security ID",
	},

	// Ticker column
	"ticker": {
		Name:        "ticker",
		Type:        TypeString,
		AllowedOps:  []string{"=", "!=", "LIKE", "IN"},
		Description: "Stock ticker symbol",
	},

	// Price columns
	"open": {
		Name:        "open",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Opening price",
	},
	"high": {
		Name:        "high",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "High price",
	},
	"low": {
		Name:        "low",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Low price",
	},
	"close": {
		Name:        "close",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Closing price (current price)",
	},
	"wk52_low": {
		Name:        "wk52_low",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "52-week low",
	},
	"wk52_high": {
		Name:        "wk52_high",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "52-week high",
	},

	// Pre-market price columns
	"pre_market_open": {
		Name:        "pre_market_open",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market opening price",
	},
	"pre_market_high": {
		Name:        "pre_market_high",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market high price",
	},
	"pre_market_low": {
		Name:        "pre_market_low",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market low price",
	},
	"pre_market_close": {
		Name:        "pre_market_close",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market closing price",
	},

	// Basic information
	"market_cap": {
		Name:        "market_cap",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Market capitalization",
	},
	"sector": {
		Name:        "sector",
		Type:        TypeString,
		AllowedOps:  []string{"=", "!=", "LIKE", "IN"},
		Description: "Sector",
	},
	"industry": {
		Name:        "industry",
		Type:        TypeString,
		AllowedOps:  []string{"=", "!=", "LIKE", "IN"},
		Description: "Industry",
	},

	// Performance columns
	"pre_market_change": {
		Name:        "pre_market_change",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market change",
	},
	"pre_market_change_pct": {
		Name:        "pre_market_change_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market change percentage",
	},
	"extended_hours_change": {
		Name:        "extended_hours_change",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Extended hours change",
	},
	"extended_hours_change_pct": {
		Name:        "extended_hours_change_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Extended hours change percentage",
	},

	// Time-based changes
	"change_1_pct": {
		Name:        "change_1_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-minute change percentage",
	},
	"change_15_pct": {
		Name:        "change_15_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "15-minute change percentage",
	},
	"change_1h_pct": {
		Name:        "change_1h_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-hour change percentage",
	},
	"change_4h_pct": {
		Name:        "change_4h_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "4-hour change percentage",
	},
	"change_1d_pct": {
		Name:        "change_1d_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-day change percentage",
	},
	"change_1w_pct": {
		Name:        "change_1w_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-week change percentage",
	},
	"change_1m_pct": {
		Name:        "change_1m_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-month change percentage",
	},
	"change_3m_pct": {
		Name:        "change_3m_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "3-month change percentage",
	},
	"change_6m_pct": {
		Name:        "change_6m_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "6-month change percentage",
	},
	"change_ytd_1y_pct": {
		Name:        "change_ytd_1y_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Year-to-date change percentage",
	},
	"change_5y_pct": {
		Name:        "change_5y_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "5-year change percentage",
	},
	"change_10y_pct": {
		Name:        "change_10y_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "10-year change percentage",
	},
	"change_all_time_pct": {
		Name:        "change_all_time_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "All-time change percentage",
	},
	"change_from_open": {
		Name:        "change_from_open",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Change from open",
	},
	"change_from_open_pct": {
		Name:        "change_from_open_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Change from open percentage",
	},
	"price_over_52wk_high": {
		Name:        "price_over_52wk_high",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Price over 52-week high percentage",
	},
	"price_over_52wk_low": {
		Name:        "price_over_52wk_low",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Price over 52-week low percentage",
	},

	// Technical indicators
	"rsi": {
		Name:        "rsi",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Relative Strength Index",
	},
	"dma_200": {
		Name:        "dma_200",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "200-day moving average",
	},
	"dma_50": {
		Name:        "dma_50",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "50-day moving average",
	},
	"price_over_50dma": {
		Name:        "price_over_50dma",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Price over 50-day moving average percentage",
	},
	"price_over_200dma": {
		Name:        "price_over_200dma",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Price over 200-day moving average percentage",
	},

	// Beta
	"beta_1y_vs_spy": {
		Name:        "beta_1y_vs_spy",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-year beta vs SPY",
	},
	"beta_1m_vs_spy": {
		Name:        "beta_1m_vs_spy",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-month beta vs SPY",
	},

	// Volume
	"volume": {
		Name:        "volume",
		Type:        TypeInteger,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Volume",
	},
	"avg_volume_1m": {
		Name:        "avg_volume_1m",
		Type:        TypeInteger,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Average volume (1 month)",
	},
	"dollar_volume": {
		Name:        "dollar_volume",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Dollar volume",
	},
	"avg_dollar_volume_1m": {
		Name:        "avg_dollar_volume_1m",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Average dollar volume (1 month)",
	},
	"pre_market_volume": {
		Name:        "pre_market_volume",
		Type:        TypeInteger,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market volume",
	},
	"pre_market_dollar_volume": {
		Name:        "pre_market_dollar_volume",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market dollar volume",
	},
	"relative_volume_14": {
		Name:        "relative_volume_14",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Relative volume (14-day)",
	},
	"pre_market_vol_over_14d_vol": {
		Name:        "pre_market_vol_over_14d_vol",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market volume over 14-day volume",
	},

	// Volatility and ranges
	"range_1m_pct": {
		Name:        "range_1m_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-minute range percentage",
	},
	"range_15m_pct": {
		Name:        "range_15m_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "15-minute range percentage",
	},
	"range_1h_pct": {
		Name:        "range_1h_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-hour range percentage",
	},
	"day_range_pct": {
		Name:        "day_range_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Day range percentage",
	},
	"volatility_1w_pct": {
		Name:        "volatility_1w_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-week volatility percentage",
	},
	"volatility_1m_pct": {
		Name:        "volatility_1m_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "1-month volatility percentage",
	},
	"pre_market_range_pct": {
		Name:        "pre_market_range_pct",
		Type:        TypeFloat,
		AllowedOps:  []string{">", "<", ">=", "<=", "topn", "bottomn", "topn_pct", "bottomn_pct"},
		Description: "Pre-market range percentage",
	},
}

type Filter struct {
	Column   string
	Operator string // =, !=, >, <, >=, <=, LIKE, IN, topn, bottomn, topn_pct, bottomn_pct
	Value    interface{}
}

type ScreenerArgs struct {
	ReturnColumns []string
	OrderBy       string
	SortDirection string
	Limit         int
	Filters       []Filter
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// GetAvailableColumns returns all available columns with their metadata
func GetAvailableColumns() map[string]ColumnInfo {
	return screenerColumns
}

// validateColumn checks if a column exists and is valid
func validateColumn(columnName string) error {
	if _, exists := screenerColumns[columnName]; !exists {
		availableColumns := make([]string, 0, len(screenerColumns))
		for col := range screenerColumns {
			availableColumns = append(availableColumns, col)
		}
		sort.Strings(availableColumns)
		return ValidationError{
			Field:   "column",
			Message: fmt.Sprintf("column '%s' does not exist. Available columns: %s", columnName, strings.Join(availableColumns, ", ")),
		}
	}
	return nil
}

// validateOperator checks if an operator is valid for a given column
func validateOperator(columnName, operator string) error {
	colInfo, exists := screenerColumns[columnName]
	if !exists {
		return ValidationError{
			Field:   "column",
			Message: fmt.Sprintf("column '%s' does not exist", columnName),
		}
	}

	for _, allowedOp := range colInfo.AllowedOps {
		if allowedOp == operator {
			return nil
		}
	}

	return ValidationError{
		Field: "operator",
		Message: fmt.Sprintf("operator '%s' is not allowed for column '%s' (type: %s). Allowed operators: %s",
			operator, columnName, colInfo.Type, strings.Join(colInfo.AllowedOps, ", ")),
	}
}

// validateValue checks if a value is compatible with the column type and operator
func validateValue(columnName, operator string, value interface{}) error {
	colInfo := screenerColumns[columnName]

	// Special handling for ranking operators
	if operator == "topn" || operator == "bottomn" || operator == "topn_pct" || operator == "bottomn_pct" {
		// Value should be a positive integer for topn/bottomn
		switch v := value.(type) {
		case int:
			if v <= 0 {
				return ValidationError{
					Field:   "value",
					Message: fmt.Sprintf("value for operator '%s' must be a positive integer, got %d", operator, v),
				}
			}
		case float64:
			if v <= 0 || v != float64(int(v)) {
				return ValidationError{
					Field:   "value",
					Message: fmt.Sprintf("value for operator '%s' must be a positive integer, got %f", operator, v),
				}
			}
		case string:
			if i, err := strconv.Atoi(v); err != nil || i <= 0 {
				return ValidationError{
					Field:   "value",
					Message: fmt.Sprintf("value for operator '%s' must be a positive integer, got '%s'", operator, v),
				}
			}
		default:
			return ValidationError{
				Field:   "value",
				Message: fmt.Sprintf("value for operator '%s' must be a positive integer, got %T", operator, value),
			}
		}
		return nil
	}

	// Special handling for IN operator
	if operator == "IN" {
		// Value should be a slice or array
		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return ValidationError{
				Field:   "value",
				Message: fmt.Sprintf("value for IN operator must be a slice or array, got %T", value),
			}
		}

		// Validate each element in the slice
		for i := 0; i < rv.Len(); i++ {
			elem := rv.Index(i).Interface()
			if err := validateSingleValue(colInfo.Type, elem); err != nil {
				return ValidationError{
					Field:   "value",
					Message: fmt.Sprintf("invalid value at index %d for IN operator: %s", i, err.Error()),
				}
			}
		}
		return nil
	}

	// Special handling for LIKE operator
	if operator == "LIKE" {
		if colInfo.Type != TypeString {
			return ValidationError{
				Field:   "operator",
				Message: fmt.Sprintf("LIKE operator can only be used with string columns, column '%s' is type %s", columnName, colInfo.Type),
			}
		}
		if _, ok := value.(string); !ok {
			return ValidationError{
				Field:   "value",
				Message: fmt.Sprintf("value for LIKE operator must be a string, got %T", value),
			}
		}
		return nil
	}

	// Standard value validation
	return validateSingleValue(colInfo.Type, value)
}

// validateSingleValue validates a single value against a column type
func validateSingleValue(colType ColumnType, value interface{}) error {
	switch colType {
	case TypeInteger:
		switch v := value.(type) {
		case int, int8, int16, int32, int64:
			return nil
		case float64:
			if v != float64(int64(v)) {
				return ValidationError{
					Field:   "value",
					Message: fmt.Sprintf("value must be an integer, got %f", v),
				}
			}
			return nil
		case string:
			if _, err := strconv.ParseInt(v, 10, 64); err != nil {
				return ValidationError{
					Field:   "value",
					Message: fmt.Sprintf("value must be an integer, got '%s'", v),
				}
			}
			return nil
		default:
			return ValidationError{
				Field:   "value",
				Message: fmt.Sprintf("value must be an integer, got %T", value),
			}
		}

	case TypeFloat:
		switch v := value.(type) {
		case int, int8, int16, int32, int64, float32, float64:
			return nil
		case string:
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return ValidationError{
					Field:   "value",
					Message: fmt.Sprintf("value must be a number, got '%s'", v),
				}
			}
			return nil
		default:
			return ValidationError{
				Field:   "value",
				Message: fmt.Sprintf("value must be a number, got %T", value),
			}
		}

	case TypeString:
		if _, ok := value.(string); !ok {
			return ValidationError{
				Field:   "value",
				Message: fmt.Sprintf("value must be a string, got %T", value),
			}
		}
		return nil

	case TypeBoolean:
		switch v := value.(type) {
		case bool:
			return nil
		case string:
			if v != "true" && v != "false" {
				return ValidationError{
					Field:   "value",
					Message: fmt.Sprintf("value must be 'true' or 'false', got '%s'", v),
				}
			}
			return nil
		default:
			return ValidationError{
				Field:   "value",
				Message: fmt.Sprintf("value must be a boolean, got %T", value),
			}
		}
	}

	return nil
}

// validateArgs validates the entire ScreenerArgs struct
func validateArgs(args ScreenerArgs) error {
	// Validate return columns
	if len(args.ReturnColumns) == 0 {
		return ValidationError{
			Field:   "return_columns",
			Message: "at least one return column must be specified",
		}
	}

	for _, col := range args.ReturnColumns {
		if err := validateColumn(col); err != nil {
			return err
		}
	}

	// Validate order by column
	if args.OrderBy != "" {
		if err := validateColumn(args.OrderBy); err != nil {
			return ValidationError{
				Field:   "order_by",
				Message: fmt.Sprintf("order by column error: %s", err.Error()),
			}
		}
	}

	// Validate sort direction
	if args.SortDirection != "" {
		validDirections := []string{"ASC", "DESC", "asc", "desc"}
		isValid := false
		for _, direction := range validDirections {
			if args.SortDirection == direction {
				isValid = true
				break
			}
		}
		if !isValid {
			return ValidationError{
				Field:   "sort_direction",
				Message: "sort direction must be 'ASC' or 'DESC' (case insensitive)",
			}
		}
	}

	// Validate limit
	if args.Limit <= 0 {
		return ValidationError{
			Field:   "limit",
			Message: "limit must be a positive integer",
		}
	}

	if args.Limit > 10000 {
		return ValidationError{
			Field:   "limit",
			Message: "limit cannot exceed 10000",
		}
	}

	// Validate filters
	for i, filter := range args.Filters {
		if err := validateColumn(filter.Column); err != nil {
			return ValidationError{
				Field:   fmt.Sprintf("filters[%d].column", i),
				Message: err.Error(),
			}
		}

		if err := validateOperator(filter.Column, filter.Operator); err != nil {
			return ValidationError{
				Field:   fmt.Sprintf("filters[%d].operator", i),
				Message: err.Error(),
			}
		}

		if err := validateValue(filter.Column, filter.Operator, filter.Value); err != nil {
			return ValidationError{
				Field:   fmt.Sprintf("filters[%d].value", i),
				Message: err.Error(),
			}
		}
	}

	return nil
}

// buildQuery constructs the SQL query with proper parameterization
func buildQuery(args ScreenerArgs) (string, []interface{}, error) {
	var queryParts []string
	var params []interface{}
	paramIndex := 1

	// Handle ranking filters (topn, bottomn, etc.) separately
	var rankingFilters []Filter
	var standardFilters []Filter

	for _, filter := range args.Filters {
		if filter.Operator == "topn" || filter.Operator == "bottomn" ||
			filter.Operator == "topn_pct" || filter.Operator == "bottomn_pct" {
			rankingFilters = append(rankingFilters, filter)
		} else {
			standardFilters = append(standardFilters, filter)
		}
	}

	// SELECT clause - always include ticker
	var selectColumns []string
	selectColumns = append(selectColumns, "s.ticker")
	for _, col := range args.ReturnColumns {
		selectColumns = append(selectColumns, "s."+col)
	}
	selectClause := "SELECT " + strings.Join(selectColumns, ", ")
	queryParts = append(queryParts, selectClause)

	// FROM clause - ticker is now directly in screener table
	fromClause := "FROM screener s"
	queryParts = append(queryParts, fromClause)

	// WHERE clause for standard filters
	if len(standardFilters) > 0 {
		var whereClauses []string
		for _, filter := range standardFilters {
			clause, filterParams, err := buildFilterClause(filter, paramIndex)
			if err != nil {
				return "", nil, err
			}
			whereClauses = append(whereClauses, clause)
			params = append(params, filterParams...)
			paramIndex += len(filterParams)
		}
		queryParts = append(queryParts, "WHERE "+strings.Join(whereClauses, " AND "))
	}

	// Handle ranking filters by wrapping in subquery
	if len(rankingFilters) > 0 {
		// We need to wrap the current query in a subquery and apply ranking
		baseQuery := strings.Join(queryParts, " ")

		// For ranking filters, we need to determine the ORDER BY and LIMIT
		for _, filter := range rankingFilters {
			orderDirection := "DESC"
			if filter.Operator == "bottomn" || filter.Operator == "bottomn_pct" {
				orderDirection = "ASC"
			}

			var limitValue int
			switch v := filter.Value.(type) {
			case int:
				limitValue = v
			case float64:
				limitValue = int(v)
			case string:
				limitValue, _ = strconv.Atoi(v)
			}

			if filter.Operator == "topn_pct" || filter.Operator == "bottomn_pct" {
				// For percentage-based ranking, we need to calculate the limit based on total count
				countQuery := "SELECT COUNT(*) FROM screener s"
				if len(standardFilters) > 0 {
					var countWhereClauses []string
					countParams := []interface{}{}
					countParamIndex := 1
					for _, stdFilter := range standardFilters {
						clause, filterParams, err := buildFilterClause(stdFilter, countParamIndex)
						if err != nil {
							return "", nil, err
						}
						countWhereClauses = append(countWhereClauses, clause)
						countParams = append(countParams, filterParams...)
						countParamIndex += len(filterParams)
					}
					countQuery += " WHERE " + strings.Join(countWhereClauses, " AND ")
					params = append(params, countParams...)
				}

				// The actual implementation would need to execute the count query first
				// For now, we'll use a simplified approach
				// Always sort NULLs last regardless of direction
				baseQuery = fmt.Sprintf("SELECT * FROM (%s ORDER BY s.%s %s NULLS LAST LIMIT (SELECT CEIL(COUNT(*) * %d / 100.0) FROM screener s)) ranked_results",
					baseQuery, filter.Column, orderDirection, limitValue)
			} else {
				// Always sort NULLs last regardless of direction
				baseQuery = fmt.Sprintf("SELECT * FROM (%s ORDER BY s.%s %s NULLS LAST LIMIT %d) ranked_results",
					baseQuery, filter.Column, orderDirection, limitValue)
			}
		}

		queryParts = []string{baseQuery}
	}

	// ORDER BY clause (if not already handled by ranking)
	if args.OrderBy != "" && len(rankingFilters) == 0 {
		orderClause := "ORDER BY s." + args.OrderBy
		if args.SortDirection != "" {
			orderClause += " " + strings.ToUpper(args.SortDirection)
		}
		// Add NULLS LAST to handle NULL values - always sort NULLs last regardless of direction
		orderClause += " NULLS LAST"
		queryParts = append(queryParts, orderClause)
	}

	// LIMIT clause (if not already handled by ranking)
	if len(rankingFilters) == 0 {
		queryParts = append(queryParts, "LIMIT "+strconv.Itoa(args.Limit))
	}

	query := strings.Join(queryParts, " ")
	return query, params, nil
}

// buildFilterClause builds a WHERE clause for a single filter
func buildFilterClause(filter Filter, startParamIndex int) (string, []interface{}, error) {
	var clause string
	var params []interface{}

	// Add table alias to column name
	columnWithAlias := "s." + filter.Column

	switch filter.Operator {
	case "=", "!=", ">", "<", ">=", "<=":
		clause = fmt.Sprintf("%s %s $%d", columnWithAlias, filter.Operator, startParamIndex)
		params = append(params, filter.Value)

	case "LIKE":
		clause = fmt.Sprintf("%s LIKE $%d", columnWithAlias, startParamIndex)
		params = append(params, filter.Value)

	case "IN":
		rv := reflect.ValueOf(filter.Value)
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return "", nil, fmt.Errorf("IN operator requires slice or array value")
		}

		var placeholders []string
		for i := 0; i < rv.Len(); i++ {
			placeholders = append(placeholders, fmt.Sprintf("$%d", startParamIndex+i))
			params = append(params, rv.Index(i).Interface())
		}
		clause = fmt.Sprintf("%s IN (%s)", columnWithAlias, strings.Join(placeholders, ", "))

	default:
		return "", nil, fmt.Errorf("unsupported operator: %s", filter.Operator)
	}

	return clause, params, nil
}

// GetScreenerData retrieves screener data based on the provided arguments
func GetScreenerData(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args ScreenerArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal screener arguments: %w", err)
	}
	if err := validateArgs(args); err != nil {
		return nil, err
	}

	// Build query
	query, params, err := buildQuery(args)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	// Execute query
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column descriptions
	fieldDescriptions := rows.FieldDescriptions()
	columnNames := make([]string, len(fieldDescriptions))
	for i, desc := range fieldDescriptions {
		columnNames[i] = string(desc.Name)
	}

	// Scan results
	var results []map[string]interface{}
	for rows.Next() {
		// Create a slice to hold the values
		values := make([]interface{}, len(columnNames))
		valuePtrs := make([]interface{}, len(columnNames))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Create result map
		result := make(map[string]interface{})
		for i, colName := range columnNames {
			result[colName] = values[i]
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Wrap results in a map structure for consistent handling in planner
	response := map[string]interface{}{
		"results": results,
		"count":   len(results),
		"columns": columnNames,
	}

	return response, nil
}
