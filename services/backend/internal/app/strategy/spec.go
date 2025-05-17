package strategy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"backend/internal/data"
)

/*cant do old
- moving averages - new window feature
- earnings - earnings base columns
- volitility injdicators - vix, custom indicators, stdev (manual claculation as expression with window smootthing)
- short interest - add columnscj
- borrow costs
- earnings yeild
- funeamental ratios - use base eearnings columns along with other base columns
- price action patterns - use features
    - doji, engulfing, hammer, etc
- order flow, level 2 - add columns
- weighting of different features such as value + momentumn > threshold - complex features that use multiple base columns
- event based (news): copreate actions, geopolictal events - stored as base columns
- social media sintements, news, macro indicators - stored as based columns
- multi asset -

- orh: needs a way to get first minute of day

*/

//things to add
/* new base columns:
- earnings: eps, revenuve, etc
- order flow/level 2: bid, ask, bid_size, ask_size
- corperate action: dividend (0 or 1)
- sentiments: socail_sentiment, fear_greed
- short interest, borrow_costs
-

other
- tirggerrs per day limit?


*/

/* examples test queries
create a strategy for all times gold gapped up over 3%
- get all the isntance when a stock whose sector was up more than 100% on the year gapped up more than 5%
- get the leading (>90 percentile price change) stocks on the year
- get all the times AVGO was up more than NVDA on the day but then closed down more than NVDA.
- get all the isntances when a stock was up more than its adr * 3 + its macd value
- "Show the top‑decile stocks (10 %) whose sector is **Technology** and whose 20‑day change > sector average + 5 %."
- create a streategy for stocks that gap up more than 1.5x their adr on the daily, and from those results filter on the 1 minute non extended hours when they trade above the opening range (1st minute) high.
- get all time NVDA outperformed AVGO over last three days.
- create a strategy for when the EPS is more than 3 times greater than the eps 2 years ago.
- get every time NVAX closed up more than 4% 4 days in a row.



ohlcv_<timeframe>: (row per timestep, timeframe, and security) only one timeframe per setup
- timestamp
- open, high, low, close, volume,

time series sporadic high freq: row per change //no implelentation yet

- bid, ask, bid_size, ask_size

fundamental: time series 2 (row per timestep and security), interval is as fast as possible
- eps, revenue
- dividend
- social_sentiment, fear_greed
- short_interest, borrow_fee
- market_cap, shares_outstanding

securities: scalar (row per security), never changes:
- sector, industry, market, related_securities
- security_id




questions:
- what resolution on non ohlcv time series



*/

/*
table layout:

ohlcv tables: ohlcv_1, ohlcv_1h, ohlcv_1d, ohlcv_1w
- timestamp: timestamp
- open: decimal
- high: decimal
- low: decimal
- close: decimal
- volume: float


*/

type FeatureID int
type OHLCVFeature string       //open, high, low, close
type FundamentalFeature string //market_cap, total_shares, active //eventually eps, revenue, news stuff
type SecurityFeature string    //securityId, ticker, locale, market, primary_exchange, active, sector, industry //doesnt change over time
type OutputType string         //raw, rankn, rankp
type ComparisonOperator string // >, >=, <, <=
type ExprOperator string       //+, -, *, /, ^
type Direction string          // asc, desc
type Timeframe string          // 1, 1h, 1d, 1w

/*

locales: us
markets: otc, stocks
primary_exchange: XNAS, XASE, BATS, XNYS, ARCX
*/

// --- Option Lists -------------------------------------------------------
// These slices define the valid values for each field of a strategy spec.
// They are exported so other packages (and prompts) can reference them.
var (
	SecurityFeatures    = []string{"SecurityId", "Ticker", "Locale", "Market", "PrimaryExchange", "Active", "Sector", "Industry"}
	OHLCVFeatures       = []string{"open", "high", "low", "close", "volume"}
	FundamentalFeatures = []string{"market_cap", "shares_outstanding", "eps", "revenue", "dividend", "social_sentiment", "fear_greed", "short_interest", "borrow_fee"}
	Timeframes          = []string{"1", "1h", "1d", "1w"}
	OutputTypes         = []string{"raw", "rankn", "rankp"}
	ComparisonOperators = []string{"<", "<=", ">", ">=", "==", "!="}
	ExprOperators       = []string{"+", "-", "*", "/", "^"}
	Directions          = []string{"asc", "desc"}
)

// SpecPromptVars holds pre-formatted strings for injecting option lists into
// the strategy system prompt. Keys correspond to the placeholder names used in
// prompts (e.g. {{ SECURITY_FEATURES }}).
var SpecPromptVars map[string]string

func init() {
	SpecPromptVars = map[string]string{
		"SECURITY_FEATURES":    quoteJoin(SecurityFeatures),
		"OHLCV_FEATURES":       quoteJoin(OHLCVFeatures),
		"FUNDAMENTAL_FEATURES": quoteJoin(FundamentalFeatures),
		"TIMEFRAMES":           quoteJoin(Timeframes),
		"OUTPUT_TYPES":         quoteJoin(OutputTypes),
		"COMPARISON_OPERATORS": quoteJoin(ComparisonOperators),
		"EXPR_OPERATORS":       quoteJoin(ExprOperators),
		"DIRECTIONS":           quoteJoin(Directions),
	}
}

// Strategy represents a stock strategy with relevant metadata.
type Strategy struct {
	Name              string    `json:"name"`
	StrategyID        int       `json:"strategyId"`
	UserID            int       `json:"userId"`
	Version           int       `json:"version"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
	Score             int       `json:"score"`
	Complexity        int       `json:"complexity"`  // Estimated compute time / parameter count
	AlertActive       bool      `json:"alertActive"` // Indicates if real-time alerts are enabled
	Spec              Spec      `json:"spec"`        // Strategy specification
	// sql               string    // never needs to be stored or go to frontend so no json or capitalization
}

// UniverseFilter lists items to include/exclude in a universe dimension.
type UniverseFilter struct { //applied using FundamentalFeatures
	SecurityFeature SecurityFeature `json:"securityFeature"`
	Include         []string        `json:"include"`
	Exclude         []string        `json:"exclude"`
}

// Universe defines the scope over which features are calculated.
type Universe struct {
	Filters       []UniverseFilter `json:"filters"`
	Timeframe     Timeframe        `json:"timeframe"`     // "1", "1h", "1d", "1w"
	ExtendedHours bool             `json:"extendedHours"` // Only applies to 1-minute data
	StartTime     time.Time        `json:"startTime"`     // Intraday start time for the strategy
	EndTime       time.Time        `json:"endTime"`       // Intraday end time for the strategy
}

type FeatureSource struct {
	Field SecurityFeature `json:"field"` //
	Value string          `json:"value"` // either "relative" meaning get the value from the security out of the universe, or a specific string value.
}

type ExprPart struct {
	Type   string `json:"type"`   // "column" | "operator"
	Value  string `json:"value"`  // Feature name (OHLCVFeature, FundamentalFeature) or ExprOperator
	Offset int    `json:"offset"` // Default 0, >= 0, time step offset for 'column' type
}

// Feature represents a calculated metric used for filtering.
type Feature struct {
	Name      string        `json:"name"`
	FeatureID FeatureID     `json:"featureId"`
	Source    FeatureSource `json:"source"` // "security", "sector", "industry", "related_stocks" (proprietary), "market", specific ticker like "AAPL" // NEW
	Output    OutputType    `json:"output"` // "raw", "rankn", "rankp"
	Expr      []ExprPart    `json:"expr"`   // Expression using +, -, /, *, ^, and fundamentalfeatures and ohlcvfeatures.
	Window    int           `json:"window"` // Smoothing window; 1 = none
}

// Filter defines a comparison that eliminates instances from the universe.
type Filter struct {
	Name     string             `json:"name"`
	LHS      FeatureID          `json:"lhs"`      // Left-hand side feature ID
	Operator ComparisonOperator `json:"operator"` // "<", "<=", ">=", ">", "!=", "=="
	RHS      struct {
		FeatureID FeatureID `json:"featureId"` // RHS feature (if any)
		Const     float64   `json:"const"`     // Constant value
		Scale     float64   `json:"scale"`     // Multiplier for RHS feature
	} `json:"rhs"`
}

// Spec bundles the universe, features, filters, and sort definition.
type Spec struct {
	Universe Universe  `json:"universe"` // First stage: scope to operate on
	Features []Feature `json:"features"` // Features created by this strategy
	Filters  []Filter  `json:"filters"`  // Boolean conditions
	SortBy   SortBy    `json:"sortBy"`
}

type SortBy struct {
	Feature FeatureID `json:"feature"` // feature to sort by

	Direction Direction `json:"direction"`
}

// Constants for validation logic
const (
	minWindowSize   = 1
	defaultRHSScale = 1.0
	sortAsc         = "asc"
	sortDesc        = "desc"
	minFeatureID    = 0

	// Timeframe identifiers
	timeframe1Min  = "1"
	timeframe1Hour = "1h"
	timeframe1Day  = "1d"
	timeframe1Week = "1w"

	// Maximum window sizes per timeframe
	maxWindow1Min  = 20000
	maxWindow1Hour = 2000

	maxWindow1Day  = 1000
	maxWindow1Week = 200

	StudyTypeBacktest = "backtest"
)

// Define valid values for enum types
var (
	validTimeframes          = toSet(Timeframe(""), Timeframes)
	validOutputTypes         = toSet(OutputType(""), OutputTypes)
	validComparisonOperators = toSet(ComparisonOperator(""), ComparisonOperators)
	validExprOperators       = toSet(ExprOperator(""), ExprOperators)
	validDirections          = toSet(Direction(""), Directions)
	validSecurityFeatures    = toSet(SecurityFeature(""), SecurityFeatures)
	validOHLCVFeatures       = toStringSet(OHLCVFeatures)
	validFundamentalFeatures = toStringSet(FundamentalFeatures)

	// Ticker validation regex
	tickerRegex = regexp.MustCompile(`^[A-Z]{1,5}(\.[A-Z])?$`)

	// Identifier validation regex
	identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_ %$#+-./\\&@!?:;,|=<>(){}[\]^*]*$`)

	// SQL reserved words
	sqlReservedWords = map[string]struct{}{
		"SELECT": {}, "FROM": {}, "WHERE": {}, "GROUP": {}, "ORDER": {}, "BY": {},
		"INSERT": {}, "UPDATE": {}, "DELETE": {}, "CREATE": {}, "ALTER": {}, "DROP": {},
		"TABLE": {}, "VIEW": {}, "INDEX": {}, "DATABASE": {}, "SCHEMA": {}, "CONSTRAINT": {},
		"PRIMARY": {}, "FOREIGN": {}, "KEY": {}, "REFERENCES": {}, "UNIQUE": {}, "CHECK": {},
		"NULL": {}, "NOT": {}, "DEFAULT": {}, "AUTO_INCREMENT": {}, "IDENTITY": {},
		"AND": {}, "OR": {}, "BETWEEN": {}, "LIKE": {}, "IN": {}, "IS": {}, "EXISTS": {},
		"ALL": {}, "ANY": {}, "SOME": {}, "CASE": {}, "WHEN": {}, "THEN": {}, "ELSE": {}, "END": {},
		"JOIN": {}, "INNER": {}, "LEFT": {}, "RIGHT": {}, "FULL": {}, "OUTER": {}, "ON": {}, "USING": {},
		"UNION": {}, "INTERSECT": {}, "EXCEPT": {}, "HAVING": {}, "LIMIT": {}, "OFFSET": {},
		"AS": {}, "ASC": {}, "DESC": {}, "DISTINCT": {}, "CAST": {}, "CONVERT": {},
	}

	// Dynamic sets loaded from the database
	ValidSectors     map[string]int
	ValidIndustries  map[string]int
	ValidSectorIds   map[int]string
	ValidIndustryIds map[int]string
)

// Main validation function
func validateSpec(spec *Spec) error {
	var errs []string

	// ---------------- Universe ----------------
	if err := validateUniverse(&spec.Universe); err != nil {
		errs = append(errs, err.Error())
	}

	// ---------------- Features ----------------
	featureIDs := make(map[int]struct{}, len(spec.Features))
	maxFeatureID := -1
	for i, f := range spec.Features {
		featureCtx := fmt.Sprintf("feature[%d]", i)
		if err := validateFeature(&f, featureCtx, &featureIDs, &maxFeatureID, spec.Universe.Timeframe); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// Check feature ID contiguity after collecting all IDs
	if err := contiguousFeatureIDs(featureIDs, len(spec.Features)); err != nil {
		errs = append(errs, err.Error())
	}
	validFeatureIDRange := len(spec.Features)

	// ---------------- Filters ----------------
	for i, flt := range spec.Filters {
		filterCtx := fmt.Sprintf("filter[%d]", i)
		if err := validateFilter(&flt, filterCtx, featureIDs, validFeatureIDRange); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// ---------------- SortBy ----------------
	if err := validateSortBy(&spec.SortBy, featureIDs, validFeatureIDRange); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// Helper validation functions

func validateUniverse(u *Universe) error {
	var errs []string

	// Validate timeframe by comparing string values
	timeframeStr := string(u.Timeframe)
	validTimeframeFound := false
	for tf := range validTimeframes {
		if timeframeStr == string(tf) {
			validTimeframeFound = true
			break
		}
	}
	if !validTimeframeFound {
		errs = append(errs, fmt.Sprintf("universe.timeframe must be one of %s, %s, %s, %s",
			timeframe1Min, timeframe1Hour, timeframe1Day, timeframe1Week))
	}

	// Validate extended hours
	if u.ExtendedHours && timeframeStr != timeframe1Min {
		errs = append(errs, fmt.Sprintf("universe.extendedHours is only valid when timeframe is '%s'", timeframe1Min))
	}

	// Validate start/end times
	if (!u.StartTime.IsZero() || !u.EndTime.IsZero()) && timeframeStr != timeframe1Min {
		errs = append(errs, fmt.Sprintf("startTime/endTime are only allowed for intraday timeframe '%s'", timeframe1Min))
	}
	if !u.StartTime.IsZero() && !u.EndTime.IsZero() && u.EndTime.Before(u.StartTime) {
		errs = append(errs, "startTime must be before endTime")
	}

	// Validate universe filters
	for i, filter := range u.Filters {
		filterCtx := fmt.Sprintf("universe.filter[%d]", i)

		// Validate SecurityFeature by converting to string
		featureStr := string(filter.SecurityFeature)
		validFeature := false
		for sf := range validSecurityFeatures {
			if featureStr == string(sf) {
				validFeature = true
				break
			}
		}

		if !validFeature {
			errs = append(errs, fmt.Sprintf("%s: invalid SecurityFeature '%s'", filterCtx, filter.SecurityFeature))
		}

		// Check for overlap between include and exclude
		set := make(map[string]struct{}, len(filter.Include))
		for _, v := range filter.Include {
			set[strings.ToUpper(v)] = struct{}{}
		}
		for _, v := range filter.Exclude {
			if _, ok := set[strings.ToUpper(v)]; ok {
				errs = append(errs, fmt.Sprintf("%s: include and exclude lists overlap on '%s'", filterCtx, v))
			}
		}

		// If the filter is for securities, validate ticker format
		if featureStr == "SecurityId" || featureStr == "Ticker" {
			for _, ticker := range filter.Include {
				if !isTicker(ticker) {
					errs = append(errs, fmt.Sprintf("%s: invalid ticker format '%s' in include list", filterCtx, ticker))
				}
			}
			for _, ticker := range filter.Exclude {
				if !isTicker(ticker) {
					errs = append(errs, fmt.Sprintf("%s: invalid ticker format '%s' in exclude list", filterCtx, ticker))
				}
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validateFeature(f *Feature, context string, featureIDs *map[int]struct{}, maxFeatureID *int, timeframe Timeframe) error {
	var errs []string

	// Validate name
	if err := checkIdentifierSafety(f.Name, context); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate featureId
	if int(f.FeatureID) < minFeatureID {
		errs = append(errs, fmt.Sprintf("%s featureId %d cannot be less than %d",
			context, f.FeatureID, minFeatureID))
	} else {
		if _, dup := (*featureIDs)[int(f.FeatureID)]; dup {
			errs = append(errs, fmt.Sprintf("%s duplicate featureId %d", context, f.FeatureID))
		}
		(*featureIDs)[int(f.FeatureID)] = struct{}{}
		if int(f.FeatureID) > *maxFeatureID {
			*maxFeatureID = int(f.FeatureID)
		}
	}

	// Validate output type
	if _, ok := validOutputTypes[f.Output]; !ok {
		errs = append(errs, fmt.Sprintf("%s output '%s' invalid", context, f.Output))
	}

	// Validate source
	if err := validateFeatureSource(&f.Source, context); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate window
	if f.Window < minWindowSize {
		errs = append(errs, fmt.Sprintf("%s window must be >= %d", context, minWindowSize))
	} else {
		if err := checkWindowSize(f.Window, string(timeframe)); err != nil {
			errs = append(errs, fmt.Sprintf("%s %s", context, err.Error()))
		}
	}

	// Validate expression
	if err := validateExpr(f.Expr, context); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validateFeatureSource(fs *FeatureSource, context string) error {
	// Validate Field by converting to string
	fieldStr := string(fs.Field)
	validField := false
	for field := range validSecurityFeatures {
		if fieldStr == string(field) {
			validField = true
			break
		}
	}

	if !validField {
		return fmt.Errorf("%s source.field '%s' invalid", context, fs.Field)
	}

	// Value can be "relative" or a specific string value
	if fs.Value != "relative" && !isTicker(fs.Value) {
		// This is simplified validation - you may need more complex validation based on the field
		switch fieldStr {
		case "Sector", "Industry", "Market":
			// For these fields, we assume any non-empty string is valid
			if fs.Value == "" {
				return fmt.Errorf("%s source.value cannot be empty for field '%s'", context, fs.Field)
			}
		default:
			// For other fields, we might want to implement specific validation
			// For now, just accept non-empty values
			if fs.Value == "" {
				return fmt.Errorf("%s source.value cannot be empty", context)
			}
		}
	}

	return nil
}

// Validator for expressions in RPN format
func validateExpr(exprParts []ExprPart, context string) error {
	var errs []string

	if len(exprParts) == 0 {
		errs = append(errs, fmt.Sprintf("%s expr cannot be empty", context))
		return errors.New(strings.Join(errs, "; "))
	}

	// Track operand/operator stack depth for RPN validation
	stackDepth := 0

	// Validate each expression part
	for i, part := range exprParts {
		partCtx := fmt.Sprintf("%s.expr[%d]", context, i)

		// Validate part type
		if part.Type != "column" && part.Type != "operator" {
			errs = append(errs, fmt.Sprintf("%s type must be 'column' or 'operator', got '%s'",
				partCtx, part.Type))
		}

		// Process based on part type
		if part.Type == "column" {
			// Validate column name
			if part.Value == "" {
				errs = append(errs, fmt.Sprintf("%s value cannot be empty for type 'column'", partCtx))
			} else {
				// Check if column is a valid OHLCV feature or fundamental feature
				lowerValue := strings.ToLower(part.Value)
				_, isValidOHLCV := validOHLCVFeatures[lowerValue]
				_, isValidFundamental := validFundamentalFeatures[lowerValue]

				if !isValidOHLCV && !isValidFundamental {
					errs = append(errs, fmt.Sprintf("%s value '%s' is not a valid OHLCV or fundamental feature",
						partCtx, part.Value))
				}
			}

			// Validate offset (must be non-negative for columns)
			if part.Offset < 0 {
				errs = append(errs, fmt.Sprintf("%s offset %d must be non-negative", partCtx, part.Offset))
			}

			// Each column adds one value to the stack
			stackDepth++

		} else if part.Type == "operator" {
			// Validate operator
			validOp := false
			for op := range validExprOperators {
				if part.Value == string(op) {
					validOp = true
					break
				}
			}
			if !validOp {
				errs = append(errs, fmt.Sprintf("%s value '%s' is not a valid operator", partCtx, part.Value))
			}

			// Offset is not applicable to operators, should be 0
			if part.Offset != 0 {
				errs = append(errs, fmt.Sprintf("%s offset must be 0 for type 'operator', got %d", partCtx, part.Offset))
			}

			// Each operator removes two operands and adds one result
			stackDepth -= 2
			stackDepth++

			// Check if we have enough operands
			if stackDepth < 0 {
				errs = append(errs, fmt.Sprintf("%s not enough operands for operator '%s'", partCtx, part.Value))
				// Reset stack depth to avoid cascading errors
				stackDepth = 0
			}
		}
	}

	// After processing all parts, we should have exactly one value on the stack
	if stackDepth != 1 {
		errs = append(errs, fmt.Sprintf("%s expression does not evaluate to a single result, %d leftover", context, stackDepth-1))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validateFilter(f *Filter, context string, featureIDs map[int]struct{}, validFeatureIDRange int) error {
	var errs []string

	// Validate name
	if err := checkIdentifierSafety(f.Name, context); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate LHS feature ID
	if _, ok := featureIDs[int(f.LHS)]; !ok {
		if int(f.LHS) < minFeatureID || int(f.LHS) >= validFeatureIDRange {
			errs = append(errs, fmt.Sprintf("%s lhs references out-of-range featureId %d (valid range %d-%d)",
				context, f.LHS, minFeatureID, validFeatureIDRange-1))
		} else {
			errs = append(errs, fmt.Sprintf("%s lhs references unknown or non-contiguous featureId %d",
				context, f.LHS))
		}
	}

	// Validate operator
	if _, ok := validComparisonOperators[f.Operator]; !ok {
		errs = append(errs, fmt.Sprintf("%s operator '%s' invalid", context, f.Operator))
	}

	// Validate RHS
	hasFeature := f.RHS.FeatureID != 0
	hasConst := f.RHS.Const != 0.0 // Be careful with float comparison, but 0.0 is usually exact

	if hasFeature && hasConst {
		errs = append(errs, fmt.Sprintf("%s rhs cannot have both featureId (%d) and const (%f) set",
			context, f.RHS.FeatureID, f.RHS.Const))
	} else if !hasFeature && !hasConst {
		// Allow if operator is == or != (comparing feature to zero)
		if f.Operator != "==" && f.Operator != "!=" {
			errs = append(errs, fmt.Sprintf("%s rhs must have either featureId or const set when operator is '%s'",
				context, f.Operator))
		}
	} else if hasFeature {
		// If using featureId, validate it exists and is in range
		if _, ok := featureIDs[int(f.RHS.FeatureID)]; !ok {
			if int(f.RHS.FeatureID) < minFeatureID || int(f.RHS.FeatureID) >= validFeatureIDRange {
				errs = append(errs, fmt.Sprintf("%s rhs.featureId %d is out of range (valid range %d-%d)",
					context, f.RHS.FeatureID, minFeatureID, validFeatureIDRange-1))
			} else {
				errs = append(errs, fmt.Sprintf("%s rhs.featureId %d unknown or non-contiguous",
					context, f.RHS.FeatureID))
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validateSortBy(sb *SortBy, featureIDs map[int]struct{}, validFeatureIDRange int) error {
	var errs []string

	// Allow feature ID 0 as a valid value, or treat all-zero struct as "no sorting requested"
	if int(sb.Feature) == 0 && sb.Direction == "" {
		// No sort-by clause – nothing to validate.
		return nil
	}

	// Validate direction
	if sb.Direction == "" {
		errs = append(errs, fmt.Sprintf("sortBy.direction ('%s' or '%s') is required when sortBy.feature is set",
			sortAsc, sortDesc))
	} else {
		// Convert Direction to string for validation
		directionStr := string(sb.Direction)
		validDir := false
		for dir := range validDirections {
			if directionStr == string(dir) {
				validDir = true
				break
			}
		}
		if !validDir {
			errs = append(errs, fmt.Sprintf("sortBy.direction must be '%s' or '%s'", sortAsc, sortDesc))
		}
	}

	// Validate feature ID (0 is legal)
	if _, ok := featureIDs[int(sb.Feature)]; !ok && int(sb.Feature) != 0 {
		if int(sb.Feature) < minFeatureID || int(sb.Feature) >= validFeatureIDRange {
			errs = append(errs, fmt.Sprintf("sortBy.feature %d is out of range (valid range %d-%d)",
				sb.Feature, minFeatureID, validFeatureIDRange-1))
		} else {
			errs = append(errs, fmt.Sprintf("sortBy.feature %d unknown or non-contiguous", sb.Feature))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// Helper functions

func isTicker(s string) bool {
	return tickerRegex.MatchString(s)
}

func checkIdentifierSafety(name, context string) error {
	if !identifierRegex.MatchString(name) {
		return fmt.Errorf("%s name '%s' contains invalid characters or starts with a digit", context, name)
	}
	if _, reserved := sqlReservedWords[strings.ToUpper(name)]; reserved {
		return fmt.Errorf("%s name '%s' is a reserved SQL keyword", context, name)
	}
	return nil
}

func contiguousFeatureIDs(ids map[int]struct{}, count int) error {
	if len(ids) != count {
		return fmt.Errorf("featureIds are not contiguous or contain duplicates (expected %d unique IDs, found %d)",
			count, len(ids))
	}
	if count == 0 {
		return nil // No features, nothing to check
	}

	minID, maxID := -1, -1
	for id := range ids {
		if minID == -1 || id < minID {
			minID = id
		}
		if maxID == -1 || id > maxID {
			maxID = id
		}
	}

	if minID != minFeatureID || maxID != count-1 {
		return fmt.Errorf("featureIds must be contiguous from %d to %d (found min=%d, max=%d)",
			minFeatureID, count-1, minID, maxID)
	}
	return nil
}

// quoteJoin joins items with quotes for prompt injection.
func quoteJoin(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = fmt.Sprintf("%q", v)
	}
	return strings.Join(out, ", ")
}

// toSet converts a slice of strings to a set. The dummy generic parameter allows
// callers to specify the target type when casting.
func toSet[T comparable](dummy T, vals []string) map[T]struct{} {
	out := make(map[T]struct{}, len(vals))
	for _, v := range vals {
		out[T(v)] = struct{}{}
	}
	return out
}

// toStringSet converts a slice of strings to a map set.
func toStringSet(vals []string) map[string]struct{} {
	out := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		out[v] = struct{}{}
	}
	return out
}

func checkWindowSize(window int, timeframe string) error {
	maxWindows := map[string]int{
		timeframe1Min:  maxWindow1Min,
		timeframe1Hour: maxWindow1Hour,
		timeframe1Day:  maxWindow1Day,
		timeframe1Week: maxWindow1Week,
	}
	if _max, ok := maxWindows[timeframe]; ok && window > _max {
		return fmt.Errorf("window size %d exceeds maximum allowed %d for timeframe '%s'", window, _max, timeframe)
	}
	return nil
}

// Unmarshal and validation functions from JSON input (these stay mostly the same)

// temp struct for unmarshalling {name, spec} input
type newStrategyInput struct {
	Name string          `json:"name"`
	Spec json.RawMessage `json:"spec"`
}

// UnmarshalAndValidateNewStrategyInput parses and validates input for creating a new strategy.
func UnmarshalAndValidateNewStrategyInput(rawInput json.RawMessage) (name string, spec Spec, err error) {
	var input newStrategyInput
	if err = json.Unmarshal(rawInput, &input); err != nil {
		err = fmt.Errorf("failed to unmarshal input JSON: %w", err)
		return
	}

	// Validate name
	name = strings.TrimSpace(input.Name)
	if name == "" {
		err = errors.New("strategy name is required and cannot be empty")
		return
	}
	// Basic identifier safety check for name
	if err = checkIdentifierSafety(name, "strategy name"); err != nil {
		return
	}

	// Unmarshal and validate spec
	if err = json.Unmarshal(input.Spec, &spec); err != nil {
		err = fmt.Errorf("failed to unmarshal 'spec' field: %w", err)
		return
	}
	if err = validateSpec(&spec); err != nil {
		// Error from validateSpec is already descriptive
		return
	}

	return // name, spec, nil
}

// Used for unmarshalling SetStrategy and NewStrategy calls
type setStrategyInput struct {
	StrategyID int             `json:"strategyId"` // Optional for NewStrategy
	Name       string          `json:"name"`
	Spec       json.RawMessage `json:"spec"` // Keep as RawMessage for initial parse
}

// UnmarshalAndValidateSetStrategyInput unmarshals and validates the input for SetStrategy.
// It returns the strategyId, name, validated Spec, and any error encountered.
func UnmarshalAndValidateSetStrategyInput(rawInput json.RawMessage) (strategyID int, name string, spec Spec, err error) {
	var input setStrategyInput
	if err = json.Unmarshal(rawInput, &input); err != nil {
		return 0, "", Spec{}, fmt.Errorf("error unmarshalling basic input: %w", err)
	}

	strategyID = input.StrategyID // Keep the original strategyId from input
	name = input.Name

	// Now unmarshal the spec part specifically
	if err = json.Unmarshal(input.Spec, &spec); err != nil {
		return 0, "", Spec{}, fmt.Errorf("error unmarshalling spec: %w. Raw spec: %s", err, string(input.Spec))
	}

	// Validate the unmarshalled Spec
	if err = validateSpec(&spec); err != nil {
		return 0, "", Spec{}, fmt.Errorf("spec validation failed: %w", err)
	}

	return strategyID, name, spec, nil
}

// RefreshSectorIndustryMaps loads valid sector and industry values from the
// database and updates the in-memory validation sets. It also updates the
// corresponding prompt variables used for strategy generation.
func RefreshSectorIndustryMaps(conn *data.Conn) error {
	const sectorQuery = `SELECT DISTINCT sector FROM securities WHERE sector IS NOT NULL AND sector != '' AND maxDate IS NULL ORDER BY sector`
	const industryQuery = `SELECT DISTINCT industry FROM securities WHERE industry IS NOT NULL AND industry != '' AND maxDate IS NULL ORDER BY industry`

	ctx := context.Background()

	sectorRows, err := conn.DB.Query(ctx, sectorQuery)
	if err != nil {
		return err
	}
	defer sectorRows.Close()

	sectors := []string{}
	for sectorRows.Next() {
		var s string
		if err := sectorRows.Scan(&s); err != nil {
			return err
		}
		sectors = append(sectors, s)
	}
	if err := sectorRows.Err(); err != nil {
		return err
	}

	industryRows, err := conn.DB.Query(ctx, industryQuery)
	if err != nil {
		return err
	}
	defer industryRows.Close()

	industries := []string{}
	for industryRows.Next() {
		var i string
		if err := industryRows.Scan(&i); err != nil {
			return err
		}
		industries = append(industries, i)
	}
	if err := industryRows.Err(); err != nil {
		return err
	}

	ValidSectors = make(map[string]int, len(sectors))
	ValidSectorIds = make(map[int]string, len(sectors))
	for idx, s := range sectors {
		id := idx + 1
		ValidSectors[strings.ToLower(s)] = id
		ValidSectorIds[id] = s
	}

	ValidIndustries = make(map[string]int, len(industries))
	ValidIndustryIds = make(map[int]string, len(industries))
	for idx, i := range industries {
		id := idx + 1
		ValidIndustries[strings.ToLower(i)] = id
		ValidIndustryIds[id] = i
	}

	SpecPromptVars["SECTORS"] = quoteJoin(sectors)
	SpecPromptVars["INDUSTRIES"] = quoteJoin(industries)

	return nil
}
