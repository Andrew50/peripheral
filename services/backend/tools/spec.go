package tools

import (
    "time"
    "encoding/json"
    "errors"
    "fmt"
    "regexp"
	"strings"
)

// Constants for validation logic
const (
	minWindowSize   = 1
	defaultRhsScale = 1.0
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

*/



// Strategy represents a stock strategy with relevant metadata.
type Strategy struct {
	Name              string    `json:"name"`
	StrategyId        int       `json:"strategyId"`
	UserId            int       `json:"userId"`
	Version           int       `json:"version"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
	Score             int       `json:"score"`
	Complexity        int       `json:"complexity"`  // Estimated compute time / parameter count
	AlertActive       bool      `json:"alertActive"` // Indicates if real-time alerts are enabled
	Spec              Spec      `json:"spec"`        // Strategy specification
    sql                 string //never needs to be stored or go to frontend so no json or capitalization
}


// UniverseFilter lists items to include/exclude in a universe dimension.
type UniverseFilter struct {
	Blacklist []string `json:"blacklist"`
	WhiteList []string `json:"whiteList"`
}

// Universe defines the scope over which features are calculated.
type Universe struct {
	Sectors       UniverseFilter `json:"sectors"`       // e.g. Technology, Finance
	Industries    UniverseFilter `json:"industries"`    // e.g. Major Banks, Property/Casualty Insurance
	Securities    UniverseFilter `json:"securities"`    // by security ID
	Markets       UniverseFilter `json:"markets"`       // e.g. US, Crypto
	Timeframe     string         `json:"timeframe"`     // "1", "1h", "1d", "1w"
	ExtendedHours bool           `json:"extendedHours"` // Only applies to 1-minute data
	StartTime     time.Time      `json:"startTime"`     // Intraday start time for the strategy
	EndTime       time.Time      `json:"endTime"`       // Intraday end time for the strategy
}

// Feature represents a calculated metric used for filtering.
type Feature struct {
	Name       string `json:"name"`
	FeatureId int64  `json:"featureId"`
	Source     string `json:"source"` // "security", "sector", "industry", "related_stocks" (proprietary), "market", specific ticker like "AAPL"
	Output     string `json:"output"` // "raw", "rankn", "rankp"
	Expr       string `json:"expr"`   // Expression using base columns
	Window     int    `json:"window"` // Smoothing window; 1 = none
}

// Filter defines a comparison that eliminates instances from the universe.
type Filter struct {
	Name     string `json:"name"`
	LHS      int    `json:"lhs"`      // Left-hand side feature ID
	Operator string `json:"operator"` // "<", "<=", ">=", ">", "!=", "=="
	RHS      struct {
		FeatureId int     `json:"featureId"` // RHS feature (if any)
		Const     float64     `json:"const"`     // Constant value
		Scale     float64 `json:"scale"`     // Multiplier for RHS feature
	} `json:"rhs"`
}

// Spec bundles the universe, features, filters, and sort definition.
type Spec struct {
	Universe Universe  `json:"universe"` // First stage: scope to operate on
	Features []Feature `json:"features"` // Features created by this strategy
	Filters  []Filter  `json:"filters"`  // Boolean conditions
	SortBy  SortBy `json:"sortBy"`
}

type SortBy struct {
		Feature   int    `json:"feature"`   // Feature ID to sort on
		Direction string `json:"direction"` // "asc" or "desc"
    }



// temp struct for unmarshalling {name, spec} input
type newStrategyInput struct {
	Name string          `json:"name"`
	Spec json.RawMessage `json:"spec"`
}

// UnmarshalAndValidateNewStrategyInput parses and validates input for creating a new strategy.
// It expects a JSON object with "name" (string) and "spec" (object).
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

// temp struct for unmarshalling {strategyId, name, spec} input
type setStrategyInput struct {
	StrategyId int             `json:"strategyId"`
	Name       string          `json:"name"`
	Spec       json.RawMessage `json:"spec"`
}

// UnmarshalAndValidateSetStrategyInput parses and validates input for updating an existing strategy.
// It expects a JSON object with "strategyId" (int), "name" (string), and "spec" (object).
func UnmarshalAndValidateSetStrategyInput(rawInput json.RawMessage) (strategyId int, name string, spec Spec, err error) {
	var input setStrategyInput
	if err = json.Unmarshal(rawInput, &input); err != nil {
		err = fmt.Errorf("failed to unmarshal input JSON: %w", err)
		return
	}

	// Validate StrategyId
	strategyId = input.StrategyId
	if strategyId <= 0 {
		err = errors.New("valid strategyId is required")
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

	return // strategyId, name, spec, nil
}


// ----------------------- validation helpers -----------------------

var (
    allowedTimeframes = map[string]struct{}{
        "1":  {},
        "1h": {},
        "1d": {},
        "1w": {},
    }
    allowedOutputs = map[string]struct{}{
        "raw":   {},
        "rankn": {},
        "rankp": {},
    }
    allowedComparisonOperators = map[string]struct{}{
        "<":  {},
        "<=": {},
        ">":  {},
        ">=": {},
        "==": {},
        "!=": {},
    }
    allowedSources = map[string]struct{}{
        "security":       {},
        "sector":         {},
        "industry":       {},
        "market":         {},
        "related_stocks": {},
    }

    // token‑level whitelist for feature expressions – anything left after
    // removing these tokens is invalid.
    exprToken   = regexp.MustCompile(`(?i)(open|high|low|close|volume|\d+(?:\.\d+)?|[\+\-\*\/\^\(\)\s]|\[|\])`)
    // Allows standard tickers (1-5 letters) and optional suffix like .B for BRK.B
    tickerRegex = regexp.MustCompile(`^[A-Z]{1,5}(\.[A-Z])?$`)
)

func validateSpec(spec *Spec) error {
    var errs []string

	// ---------------- Universe ----------------
	if _, ok := allowedTimeframes[spec.Universe.Timeframe]; !ok {
		// Use constants in error message for clarity, though map keys are still strings
		errs = append(errs, fmt.Sprintf("universe.timeframe must be one of %s, %s, %s, %s", timeframe1Min, timeframe1Hour, timeframe1Day, timeframe1Week))
	}
	if spec.Universe.ExtendedHours && spec.Universe.Timeframe != timeframe1Min {
		errs = append(errs, fmt.Sprintf("universe.extendedHours is only valid when timeframe is '%s'", timeframe1Min))
	}
	if (!spec.Universe.StartTime.IsZero() || !spec.Universe.EndTime.IsZero()) && spec.Universe.Timeframe != timeframe1Min {
		errs = append(errs, fmt.Sprintf("startTime/endTime are only allowed for intraday timeframe '%s'", timeframe1Min))
	}
	if !spec.Universe.StartTime.IsZero() && !spec.Universe.EndTime.IsZero() && spec.Universe.EndTime.Before(spec.Universe.StartTime) {
		errs = append(errs, "startTime must be before endTime")
    }

    // overlap helper
    overlap := func(wl, bl []string, label string) {
        set := make(map[string]struct{}, len(wl))
        for _, v := range wl {
            set[strings.ToUpper(v)] = struct{}{}
        }
        for _, v := range bl {
            if _, ok := set[strings.ToUpper(v)]; ok {
                errs = append(errs, fmt.Sprintf("%s whitelist and blacklist overlap on '%s'", label, v))
            }
        }
    }
    u := &spec.Universe
    overlap(u.Sectors.WhiteList, u.Sectors.Blacklist, "sectors")
    overlap(u.Industries.WhiteList, u.Industries.Blacklist, "industries")
    overlap(u.Securities.WhiteList, u.Securities.Blacklist, "securities") // Check ticker overlap too
    overlap(u.Markets.WhiteList, u.Markets.Blacklist, "markets")

    // Validate ticker format in security lists
    validateTickers := func(tickers []string, listType string) {
        for _, ticker := range tickers {
            if !isTicker(ticker) {
                errs = append(errs, fmt.Sprintf("invalid ticker format '%s' in securities %s", ticker, listType))
            }
        }
    }
    validateTickers(u.Securities.WhiteList, "whitelist")
    validateTickers(u.Securities.Blacklist, "blacklist")


    // ---------------- Features ----------------
    featureIDs := make(map[int]struct{}, len(spec.Features))
    maxFeatureID := -1
    for i, f := range spec.Features {
        featureCtx := fmt.Sprintf("feature[%d]", i)
        if err := checkIdentifierSafety(f.Name, featureCtx); err != nil {
			errs = append(errs, err.Error())
		}

		// Use const for minimum feature ID check
		if f.FeatureId < minFeatureID {
			errs = append(errs, fmt.Sprintf("%s featureId %d cannot be less than %d", featureCtx, f.FeatureId, minFeatureID))
		} else {
			if _, dup := featureIDs[int(f.FeatureId)]; dup {
				errs = append(errs, fmt.Sprintf("%s duplicate featureId %d", featureCtx, f.FeatureId))
            }
            featureIDs[int(f.FeatureId)] = struct{}{}
            if int(f.FeatureId) > maxFeatureID {
                maxFeatureID = int(f.FeatureId)
            }
        }


        if _, ok := allowedOutputs[f.Output]; !ok {
            errs = append(errs, fmt.Sprintf("%s output '%s' invalid", featureCtx, f.Output))
        }
        if _, ok := allowedSources[f.Source]; !ok {
            if !isTicker(f.Source) { // Use updated isTicker check
				errs = append(errs, fmt.Sprintf("%s source '%s' invalid (must be allowed source type or valid ticker)", featureCtx, f.Source))
			}
		}
		// Use const for minimum window size check
		if f.Window < minWindowSize {
			errs = append(errs, fmt.Sprintf("%s window must be >= %d", featureCtx, minWindowSize))
		} else {
			// Check window size against timeframe limits
			if err := checkWindowSize(f.Window, spec.Universe.Timeframe); err != nil {
                 errs = append(errs, fmt.Sprintf("%s %s", featureCtx, err.Error()))
             }
        }

        // Expression validation
        if !exprAllowed(f.Expr) {
            errs = append(errs, fmt.Sprintf("%s expr contains invalid tokens: %s", featureCtx, f.Expr))
        } else {
            // Perform structural checks only if basic tokens are allowed
            if err := balanced(f.Expr, '(', ')'); err != nil {
                 errs = append(errs, fmt.Sprintf("%s %s", featureCtx, err.Error()))
            }
            if err := balanced(f.Expr, '[', ']'); err != nil {
                 errs = append(errs, fmt.Sprintf("%s %s", featureCtx, err.Error()))
            }
            if err := validateBracketContent(f.Expr); err != nil {
                 errs = append(errs, fmt.Sprintf("%s %s", featureCtx, err.Error()))
            }
            if err := checkConsecutiveOperators(f.Expr); err != nil {
                 errs = append(errs, fmt.Sprintf("%s %s", featureCtx, err.Error()))
            }
        }
    }

    // Check feature ID contiguity after collecting all IDs
    if err := contiguousFeatureIDs(featureIDs, len(spec.Features)); err != nil {
        errs = append(errs, err.Error())
    }
    // Define valid range for feature IDs based on count AFTER checking contiguity
    validFeatureIDRange := len(spec.Features)


    // ---------------- Filters ----------------
    for i, flt := range spec.Filters {
        filterCtx := fmt.Sprintf("filter[%d]", i)
        if err := checkIdentifierSafety(flt.Name, filterCtx); err != nil {
            errs = append(errs, err.Error())
        }

		// Validate LHS feature ID
		if _, ok := featureIDs[flt.LHS]; !ok {
			// Check if it's out of range even if contiguity check passed (e.g., negative ID)
			// Use const for minimum feature ID check
			if flt.LHS < minFeatureID || flt.LHS >= validFeatureIDRange {
				errs = append(errs, fmt.Sprintf("%s lhs references out-of-range featureId %d (valid range %d-%d)", filterCtx, flt.LHS, minFeatureID, validFeatureIDRange-1))
			} else {
				// This case implies a non-contiguous set if the ID is within range but not found
				errs = append(errs, fmt.Sprintf("%s lhs references unknown or non-contiguous featureId %d", filterCtx, flt.LHS))
             }
        }

        if _, ok := allowedComparisonOperators[flt.Operator]; !ok {
            errs = append(errs, fmt.Sprintf("%s operator '%s' invalid", filterCtx, flt.Operator))
        }

        // Validate RHS
        hasFeature := flt.RHS.FeatureId != 0
        hasConst := flt.RHS.Const != 0.0 // Be careful with float comparison, but 0.0 is usually exact

        if hasFeature && hasConst {
             errs = append(errs, fmt.Sprintf("%s rhs cannot have both featureId (%d) and const (%f) set", filterCtx, flt.RHS.FeatureId, flt.RHS.Const))
        } else if !hasFeature && !hasConst {
             // Allow if operator is == or != (comparing feature to zero)
             if flt.Operator != "==" && flt.Operator != "!=" {
                 errs = append(errs, fmt.Sprintf("%s rhs must have either featureId or const set when operator is '%s'", filterCtx, flt.Operator))
             }
             // If operator is == or !=, comparing to 0 is valid, so no error here.
		} else if hasFeature {
			// If using featureId, validate it exists and is in range
			if _, ok := featureIDs[flt.RHS.FeatureId]; !ok {
				// Use const for minimum feature ID check
				if flt.RHS.FeatureId < minFeatureID || flt.RHS.FeatureId >= validFeatureIDRange {
					errs = append(errs, fmt.Sprintf("%s rhs.featureId %d is out of range (valid range %d-%d)", filterCtx, flt.RHS.FeatureId, minFeatureID, validFeatureIDRange-1))
				} else {
					errs = append(errs, fmt.Sprintf("%s rhs.featureId %d unknown or non-contiguous", filterCtx, flt.RHS.FeatureId))
				}
             }
        }
        // No specific validation needed for 'const' other than it being a float64

		// Ensure scale defaults to defaultRhsScale if not set and RHS is present
		if (hasFeature || hasConst) && flt.RHS.Scale == 0 {
			spec.Filters[i].RHS.Scale = defaultRhsScale // Default scale if RHS is used
		} else if flt.RHS.Scale == 0 && !(hasFeature || hasConst) {
			// If comparing to implicit zero (e.g., lhs > 0), scale should also be considered 0 or 1, default to 1 for safety? Or ignore? Let's ignore, scale=0 is fine if there's no feature/const.
		}
    }

    // ---------------- SortBy ----------------
    // ➊  Allow feature ID **0** as a valid value to sort on.
    // ➋  Treat the all-zero struct (feature == 0 && direction == "") as
    //     "no sorting requested".
    if spec.SortBy.Feature == 0 && spec.SortBy.Direction == "" {
        // No sort-by clause – nothing to validate.
    } else {
        // Validate direction.
        if spec.SortBy.Direction == "" {
            errs = append(errs, fmt.Sprintf("sortBy.direction ('%s' or '%s') is required when sortBy.feature is set", sortAsc, sortDesc))
        } else if spec.SortBy.Direction != sortAsc && spec.SortBy.Direction != sortDesc {
            errs = append(errs, fmt.Sprintf("sortBy.direction must be '%s' or '%s'", sortAsc, sortDesc))
        }

        // Validate feature ID (0 is now legal).
        if _, ok := featureIDs[spec.SortBy.Feature]; !ok {
            if spec.SortBy.Feature < minFeatureID || spec.SortBy.Feature >= validFeatureIDRange {
                errs = append(errs, fmt.Sprintf("sortBy.feature %d is out of range (valid range %d-%d)",
                    spec.SortBy.Feature, minFeatureID, validFeatureIDRange-1))
            } else {
                errs = append(errs, fmt.Sprintf("sortBy.feature %d unknown or non-contiguous", spec.SortBy.Feature))
            }
        }
    }

    if len(errs) > 0 {
        // Sort errors for consistent output during testing/debugging
        // sort.Strings(errs)
        return errors.New(strings.Join(errs, "; "))
    }
    return nil
}

// exprAllowed returns true if the provided expression only contains
// valid base‑column tokens, numeric literals, arithmetic operators,
// brackets, and whitespace.
func exprAllowed(expr string) bool {
    leftover := exprToken.ReplaceAllString(expr, "")
    return strings.TrimSpace(leftover) == ""
}

func isTicker(s string) bool {
	return tickerRegex.MatchString(s)
}

// ----------------------- Extended Validation Helpers -----------------------

var sqlReservedWords = map[string]struct{}{
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
	// Add more common keywords as needed
}

// Identifiers may now contain spaces between words (but must still start with a
// letter or underscore, and may include letters, digits or underscores
// thereafter).
var identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_ %$#+-./\\&@!?:;,|=<>(){}[\]^*]*$`)
var bracketContentRegex = regexp.MustCompile(`\[\d+\]`)
var consecutiveOperatorRegex = regexp.MustCompile(`[+\-*/^]\s*[+\-*/^]`) // Find two operators potentially separated by whitespace

// checkIdentifierSafety validates names for format and reserved words.
func checkIdentifierSafety(name, context string) error {
	if !identifierRegex.MatchString(name) {
		return fmt.Errorf("%s name '%s' contains invalid characters or starts with a digit", context, name)
	}
	if _, reserved := sqlReservedWords[strings.ToUpper(name)]; reserved {
		return fmt.Errorf("%s name '%s' is a reserved SQL keyword", context, name)
	}
	return nil
}

// contiguousFeatureIDs checks if IDs are 0-based and sequential.
func contiguousFeatureIDs(ids map[int]struct{}, count int) error {
	if len(ids) != count {
		// This condition implies duplicates or missing IDs, duplicate check is already done.
		// So this primarily catches gaps if len(ids) < count.
		return fmt.Errorf("featureIds are not contiguous or contain duplicates (expected %d unique IDs, found %d)", count, len(ids))
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
	// Use const for minimum feature ID check
	if count > 0 && (minID != minFeatureID || maxID != count-1) {
		return fmt.Errorf("featureIds must be contiguous from %d to %d (found min=%d, max=%d)", minFeatureID, count-1, minID, maxID)
	}
	return nil
}

// balanced checks if brackets or parentheses are balanced.
func balanced(expr string, openChar, closeChar rune) error {
	balance := 0
	for _, r := range expr {
		if r == openChar {
			balance++
		} else if r == closeChar {
			balance--
		}
		if balance < 0 {
			return fmt.Errorf("expression '%s' has unbalanced '%c'", expr, closeChar)
		}
	}
	if balance != 0 {
		return fmt.Errorf("expression '%s' has unbalanced '%c'", expr, openChar)
	}
	return nil
}

// validateBracketContent checks if content inside [] is a non-negative integer.
func validateBracketContent(expr string) error {
	// Find all bracket pairs first
	start := -1
	for i, r := range expr {
		if r == '[' {
			if start != -1 {
				return fmt.Errorf("expression '%s' has nested brackets, which is not allowed", expr)
			}
			start = i
		} else if r == ']' {
			if start == -1 {
				return fmt.Errorf("expression '%s' has unbalanced ']'", expr) // Should be caught by balanced(), but good defense
			}
			content := expr[start+1 : i]
			// Use regex to check if the content is just digits
			if !regexp.MustCompile(`^\d+$`).MatchString(content) {
				return fmt.Errorf("expression '%s' has invalid content inside brackets: '[%s]'. Only non-negative integers are allowed", expr, content)
			}
			start = -1 // Reset for next bracket pair
		}
	}
	// Check if we ended inside an open bracket
	if start != -1 {
		return fmt.Errorf("expression '%s' has unbalanced '['", expr) // Should be caught by balanced()
	}
	return nil
}

// checkConsecutiveOperators checks for patterns like ++, --, */ etc.
func checkConsecutiveOperators(expr string) error {
	if consecutiveOperatorRegex.MatchString(expr) {
		return fmt.Errorf("expression '%s' contains consecutive arithmetic operators", expr)
	}
	return nil
}

// checkWindowSize enforces max window based on timeframe.
func checkWindowSize(window int, timeframe string) error {
	// Use constants for max window sizes
	maxWindows := map[string]int{
		timeframe1Min:  maxWindow1Min,
		timeframe1Hour: maxWindow1Hour,
		timeframe1Day:  maxWindow1Day,
		timeframe1Week: maxWindow1Week,
	}
	if _max, ok := maxWindows[timeframe]; ok && window > _max {
		return fmt.Errorf("window size %d exceeds maximum allowed %d for timeframe '%s'", window, _max, timeframe)
	}
	// If timeframe is unknown, it's caught elsewhere, so no error here.
	return nil
}
