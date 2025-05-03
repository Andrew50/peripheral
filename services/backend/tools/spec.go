package tools

import (
    "time"
    "encoding/json"
    "log"
    "sync"
    "errors"
    "fmt"
    "regexp"
	"strings"
    "context"
	"github.com/jackc/pgx/v4"
    "backend/utils"
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

var (
	securityFeatures    = []string{"ticker", "sector", "industry", "market", "locale", "primaryExchange", "active"}
	ohlcvFeatures       = []string{"open", "high", "low", "close", "volume"}
	//fundamentalFeatures = []string{"marketCap", "sharesOutstanding", "eps", "revenue", "dividend", "socialSentiment", "fearGreed", "shortInterest", "borrowFee"}
	timeframes          = []string{ "1d"} //add 1, 1h, 1w
	outputTypes         = []string{"raw", "rankn", "rankp"}
	comparisonOperators = []string{"<", "<=", ">", ">="}
	exprOperators       = []string{"+", "-", "*", "/", "^"}
	directions          = []string{"asc", "desc"}
)

// --- Derived Sets for Validation ---
// These maps (sets) are created for efficient 'contains' checks.
var (
	ValidSecurityFeatures    = toSet(securityFeatures)
	ValidOhlcvFeatures       = toSet(ohlcvFeatures)
	ValidTimeframes          = toSet(timeframes)
	ValidOutputTypes         = toSet(outputTypes)
	ValidComparisonOperators = toSet(comparisonOperators)
	ValidExprOperators       = toSet(exprOperators)
	ValidDirections          = toSet(directions)
	// Dynamic sets loaded from DB
	ValidSectors             map[string]int // name -> id
	ValidIndustries          map[string]int // name -> id
	ValidSectorIds           map[int]string // id -> name
	ValidIndustryIds         map[int]string // id -> name
	ValidFundamentalFeatures map[string]struct{}
)

func toSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}

// Globals for dynamic validation sets loaded from the database
var (
	validSectors             = make(map[string]int) // name -> id
	validIndustries          = make(map[string]int) // name -> id
	validSectorIds           = make(map[int]string) // id -> name
	validIndustryIds         = make(map[int]string) // id -> name
	validFundamentalFeatures = make(map[string]struct{})
	dynamicSetMutex          sync.RWMutex
)

// UpdateDynamicSet refreshes exactly one dynamic-validation map.
// key must be "sectors", "industries", or "fundamentalFeatures".
func UpdateDynamicSet(ctx context.Context, conn *utils.Conn, key string) error {
	var (
		rows pgx.Rows
		err  error
	)

	switch key {
	case "sectors":
		// Select both id and name for mapping
		const q = `SELECT sectorId, LOWER(sector)
				   FROM sectors
				   WHERE sector IS NOT NULL
					 AND sector <> ''
					 AND LOWER(sector) <> 'unknown'`
		if rows, err = conn.DB.Query(ctx, q); err != nil {
			return fmt.Errorf("querying sectors: %w", err)
		}

	case "industries":
		// Select both id and name for mapping
		const q = `SELECT industryId, LOWER(industry)
				   FROM industries
				   WHERE industry IS NOT NULL
					 AND industry <> ''
					 AND LOWER(industry) <> 'unknown'`
		if rows, err = conn.DB.Query(ctx, q); err != nil {
			return fmt.Errorf("querying industries: %w", err)
		}

	case "fundamentalFeatures":
		const q = `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name   = 'fundamentals'
			  AND column_name NOT IN ('security_id', 'timestamp')`
		if rows, err = conn.DB.Query(ctx, q); err != nil {
			return fmt.Errorf("querying fundamental columns: %w", err)
		}

	default:
		return fmt.Errorf("unknown dynamic set key: %s", key)
	}

	// Collect values and build maps
	newSectors := make(map[string]int)
	newIndustries := make(map[string]int)
	newSectorIds := make(map[int]string)
	newIndustryIds := make(map[int]string)
	newFundamentalFeatures := make(map[string]struct{})

	for rows.Next() {
		var id int
		var name string
		var featureName string // For fundamental features

		switch key {
		case "sectors":
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return fmt.Errorf("scanning sector: %w", err)
			}
			name = strings.ToLower(name)
			newSectors[name] = id
			newSectorIds[id] = name
		case "industries":
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return fmt.Errorf("scanning industry: %w", err)
			}
			name = strings.ToLower(name)
			newIndustries[name] = id
			newIndustryIds[id] = name
		case "fundamentalFeatures":
			if err := rows.Scan(&featureName); err != nil {
				rows.Close()
				return fmt.Errorf("scanning fundamental feature: %w", err)
			}
			newFundamentalFeatures[strings.ToLower(featureName)] = struct{}{}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows for %s: %w", key, err)
	}

	// Swap the global maps safely
	dynamicSetMutex.Lock()
	defer dynamicSetMutex.Unlock()

	count := 0
	switch key {
	case "sectors":
		validSectors = newSectors
		validSectorIds = newSectorIds
		count = len(validSectors)
	case "industries":
		validIndustries = newIndustries
		validIndustryIds = newIndustryIds
		count = len(validIndustries)
	case "fundamentalFeatures":
		validFundamentalFeatures = newFundamentalFeatures
		count = len(validFundamentalFeatures)
	}
	fmt.Printf("Updated %s: %d entries\n", key, count)
	return nil
}

// InitializeDynamicValidationSets populates the dynamic validation maps from the database.
// This should be called once during application startup after the database connection is established.
func InitializeDynamicValidationSets(ctx context.Context, conn *utils.Conn) error {
	var errs []string
	fmt.Println("Initializing dynamic validation sets...")

	if err := UpdateDynamicSet(ctx, conn, "sectors"); err != nil {
		errs = append(errs, fmt.Sprintf("failed to initialize sectors: %v", err))
	}
	if err := UpdateDynamicSet(ctx, conn, "industries"); err != nil {
		errs = append(errs, fmt.Sprintf("failed to initialize industries: %v", err))
	}
	if err := UpdateDynamicSet(ctx, conn, "fundamentalFeatures"); err != nil {
		errs = append(errs, fmt.Sprintf("failed to initialize fundamental features: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during dynamic set initialization: %s", strings.Join(errs, "; "))
	}

	log.Println("Dynamic validation sets initialized successfully.")
	return nil
}


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
//	sql               string    // never needs to be stored or go to frontend so no json or capitalization
}

// UniverseFilter lists items to include/exclude in a universe dimension.
type UniverseFilter struct {
	SecurityFeature string `json:"securityFeature"`
	// Fields used for API input/output (names)
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	// Fields used for database storage (IDs) - omitempty helps keep JSON clean if unused
	IncludeIds []int `json:"includeIds,omitempty"`
	ExcludeIds []int `json:"excludeIds,omitempty"`
}

// Universe defines the scope over which features are calculated.
type Universe struct {
	Filters        []UniverseFilter `json:"filters"`
	Timeframe      string        `json:"timeframe"`     // "1", "1h", "1d", "1w"
	ExtendedHours  bool             `json:"extendedHours"` // Only applies to 1-minute data
	StartTime      time.Time        `json:"startTime"`     // Intraday start time for the strategy
	EndTime        time.Time        `json:"endTime"`       // Intraday end time for the strategy
}

type FeatureSource struct {
	Field   string `json:"field"`   // 
	Value   string          `json:"value"`   // either "relative" meaning get the value from the security out of the universe, or a specific string value.
}

type ExprPart struct {
	Type   string `json:"type"`   // "column" | "operator"
	Value  string `json:"value"`  // Feature name (OHLCVFeature, FundamentalFeature) or ExprOperator
	Offset int    `json:"offset"` // Default 0, >= 0, time step offset for 'column' type
}

// Feature represents a calculated metric used for filtering.
type Feature struct {
	Name      string        `json:"name"`
	FeatureId int           `json:"featureId"`
	Source    FeatureSource `json:"source"` // "security", "sector", "industry", "related_stocks" (proprietary), "market", specific ticker like "AAPL" // NEW 
	Output    string    `json:"output"` // "raw", "rankn", "rankp"
	Expr      []ExprPart    `json:"expr"`   // Expression using +, -, /, *, ^, and fundamentalfeatures and ohlcvfeatures.
	Window    int           `json:"window"` // Smoothing window; 1 = none
}

// Filter defines a comparison that eliminates instances from the universe.
type Filter struct {
	Name     string             `json:"name"`
	LHS      int          `json:"lhs"`      // Left-hand side feature ID
	Operator string `json:"operator"` // "<", "<=", ">=", ">"
	RHS      struct {
		FeatureId int `json:"featureId"` // RHS feature (if any)
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
	Feature   int `json:"feature"`   // feature to sort by
	Direction string  `json:"direction"`
}

var (
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

    // Validate timeframe
    timeframeStr := string(u.Timeframe)
    if _, ok := ValidTimeframes[timeframeStr]; !ok {
        errs = append(errs, fmt.Sprintf("universe.timeframe '%s' invalid; must be one of %v",
            u.Timeframe, timeframes)) // Use the original slice for the error message
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

        // Validate SecurityFeature
        featureStr := string(filter.SecurityFeature)
        if _, ok := ValidSecurityFeatures[featureStr]; !ok {
            errs = append(errs, fmt.Sprintf("%s: invalid SecurityFeature '%s'; must be one of %v",
                filterCtx, filter.SecurityFeature, securityFeatures)) // Use the original slice for the error message
            continue // Skip further validation for this filter if feature is invalid
        }

        // Validate include/exclude values based on SecurityFeature
        dynamicSetMutex.RLock() // Lock for reading dynamic sets
        switch featureStr {
        case "sector":
            for _, name := range filter.Include {
                lowerName := strings.ToLower(name)
                if _, ok := validSectors[lowerName]; !ok {
                    errs = append(errs, fmt.Sprintf("%s: invalid sector '%s' in include list", filterCtx, name))
                }
            }
            for _, name := range filter.Exclude {
                lowerName := strings.ToLower(name)
                if _, ok := validSectors[lowerName]; !ok {
                    errs = append(errs, fmt.Sprintf("%s: invalid sector '%s' in exclude list", filterCtx, name))
                }
            }
        case "industry":
            for _, name := range filter.Include {
                lowerName := strings.ToLower(name)
                if _, ok := validIndustries[lowerName]; !ok {
                    errs = append(errs, fmt.Sprintf("%s: invalid industry '%s' in include list", filterCtx, name))
                }
            }
            for _, name := range filter.Exclude {
                lowerName := strings.ToLower(name)
                if _, ok := validIndustries[lowerName]; !ok {
                    errs = append(errs, fmt.Sprintf("%s: invalid industry '%s' in exclude list", filterCtx, name))
                }
            }
        case "ticker": // Assuming "ticker" is the feature name for security filtering by ticker symbol
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
        // Add cases for other SecurityFeatures if they need specific validation
        }
        dynamicSetMutex.RUnlock() // Unlock after reading

        // Check for overlap between include and exclude (case-insensitive)
        includeSet := make(map[string]struct{}, len(filter.Include))
        for _, v := range filter.Include {
            includeSet[strings.ToLower(v)] = struct{}{}
        }
        for _, v := range filter.Exclude {
            if _, ok := includeSet[strings.ToLower(v)]; ok {
                errs = append(errs, fmt.Sprintf("%s: include and exclude lists overlap on '%s'", filterCtx, v))
            }
        }

        // Note: Ticker format validation moved inside the switch statement
        /*
        if featureStr == "SecurityId" || featureStr == "Ticker" { // Assuming Ticker is used
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
        */
    }

    if len(errs) > 0 {
        return errors.New(strings.Join(errs, "; "))
    }
    return nil
}

func validateFeature(f *Feature, context string, featureIDs *map[int]struct{}, maxFeatureID *int, timeframe string) error {
    var errs []string
    
    // Validate name
    if err := checkIdentifierSafety(f.Name, context); err != nil {
        errs = append(errs, err.Error())
    }
    
    // Validate featureId
    if int(f.FeatureId) < minFeatureID {
        errs = append(errs, fmt.Sprintf("%s featureId %d cannot be less than %d", 
            context, f.FeatureId, minFeatureID))
    } else {
        if _, dup := (*featureIDs)[int(f.FeatureId)]; dup {
            errs = append(errs, fmt.Sprintf("%s duplicate featureId %d", context, f.FeatureId))
        }
        (*featureIDs)[int(f.FeatureId)] = struct{}{}
        if int(f.FeatureId) > *maxFeatureID {
            *maxFeatureID = int(f.FeatureId)
        }
    }

    // Validate output type
    outputStr := string(f.Output)
    if _, ok := ValidOutputTypes[outputStr]; !ok {
        errs = append(errs, fmt.Sprintf("%s output '%s' invalid; must be one of %v",
            context, f.Output, outputTypes)) // Use the original slice for the error message
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

    // Validate Field
    fieldStr := string(fs.Field)
    if _, ok := ValidSecurityFeatures[fieldStr]; !ok {
        // Get keys from the map for the error message
        validFields := make([]string, 0, len(ValidSecurityFeatures))
        for k := range ValidSecurityFeatures {
            validFields = append(validFields, k)
        }
        return fmt.Errorf("%s source.field '%s' invalid; must be one of %v",
            context, fs.Field, validFields)
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
				_, isValidOHLCV := ValidOhlcvFeatures[lowerValue]
				dynamicSetMutex.RLock() // Need read lock for dynamic set
				_, isValidFundamental := ValidFundamentalFeatures[lowerValue]
				dynamicSetMutex.RUnlock()

				if !isValidOHLCV && !isValidFundamental {
					// Collect valid keys from both maps for the error message
					validCols := make([]string, 0, len(ValidOhlcvFeatures)+len(ValidFundamentalFeatures))
					for k := range ValidOhlcvFeatures {
						validCols = append(validCols, k)
					}
					for k := range ValidFundamentalFeatures {
						validCols = append(validCols, k)
					}
					errs = append(errs, fmt.Sprintf("%s value '%s' is not a valid OHLCV or fundamental feature; must be one of %v",
						partCtx, part.Value, validCols))
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
			if _, ok := ValidExprOperators[part.Value]; !ok {
				errs = append(errs, fmt.Sprintf("%s value '%s' is not a valid operator; must be one of %v",
					partCtx, part.Value, exprOperators)) // Use the original slice for the error message
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
    opStr := string(f.Operator)
    if _, ok := ValidComparisonOperators[opStr]; !ok {
        errs = append(errs, fmt.Sprintf("%s operator '%s' invalid; must be one of %v",
            context, f.Operator, comparisonOperators)) // Use the original slice for the error message
    }

    // Validate RHS
    hasFeature := f.RHS.FeatureId != 0
    hasConst := f.RHS.Const != 0.0 // Be careful with float comparison, but 0.0 is usually exact
    
    if hasFeature && hasConst {
        errs = append(errs, fmt.Sprintf("%s rhs cannot have both featureId (%d) and const (%f) set", 
            context, f.RHS.FeatureId, f.RHS.Const))
    } else if !hasFeature && !hasConst {
        // Allow if operator is == or != (comparing feature to zero)
        if f.Operator != "==" && f.Operator != "!=" {
            errs = append(errs, fmt.Sprintf("%s rhs must have either featureId or const set when operator is '%s'", 
                context, f.Operator))
        }
    } else if hasFeature {
        // If using featureId, validate it exists and is in range
        if _, ok := featureIDs[int(f.RHS.FeatureId)]; !ok {
            if int(f.RHS.FeatureId) < minFeatureID || int(f.RHS.FeatureId) >= validFeatureIDRange {
                errs = append(errs, fmt.Sprintf("%s rhs.featureId %d is out of range (valid range %d-%d)", 
                    context, f.RHS.FeatureId, minFeatureID, validFeatureIDRange-1))
            } else {
                errs = append(errs, fmt.Sprintf("%s rhs.featureId %d unknown or non-contiguous", 
                    context, f.RHS.FeatureId))
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

        // Validate direction
        directionStr := string(sb.Direction)
        if _, ok := ValidDirections[directionStr]; !ok {
            errs = append(errs, fmt.Sprintf("sortBy.direction '%s' invalid; must be one of %v",
                sb.Direction, directions)) // Use the original slice for the error message
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


// convertSpecNamesToIds modifies the spec in-place, converting sector/industry names
// in universe filters to their corresponding IDs using the globally loaded maps.
// It clears the name fields (Include/Exclude) before returning.
// This should be called BEFORE marshalling the spec to JSON for database storage.
func convertSpecNamesToIds(spec *Spec) error {
	dynamicSetMutex.RLock() // Read lock needed for accessing validation maps
	defer dynamicSetMutex.RUnlock()

	for i := range spec.Universe.Filters {
		filter := &spec.Universe.Filters[i] // Operate on pointer to modify original
		var nameMap map[string]int
		featureName := ""

		switch filter.SecurityFeature {
		case "sector":
			nameMap = validSectors
			featureName = "sector"
		case "industry":
			nameMap = validIndustries
			featureName = "industry"
		default:
			continue // Skip filters that are not sector or industry
		}

		// Convert Include names to IDs
		filter.IncludeIds = make([]int, 0, len(filter.Include))
		for _, name := range filter.Include {
			lowerName := strings.ToLower(name)
			if id, ok := nameMap[lowerName]; ok {
				filter.IncludeIds = append(filter.IncludeIds, id)
			} else {
				// This should ideally not happen if validation passed, but handle defensively
				return fmt.Errorf("universe filter %d: unknown %s name '%s' found during ID conversion", i, featureName, name)
			}
		}

		// Convert Exclude names to IDs
		filter.ExcludeIds = make([]int, 0, len(filter.Exclude))
		for _, name := range filter.Exclude {
			lowerName := strings.ToLower(name)
			if id, ok := nameMap[lowerName]; ok {
				filter.ExcludeIds = append(filter.ExcludeIds, id)
			} else {
				return fmt.Errorf("universe filter %d: unknown %s name '%s' found during ID conversion", i, featureName, name)
			}
		}

		// Clear the name fields as they are redundant for storage
		filter.Include = nil
		filter.Exclude = nil
	}
	return nil
}

// convertSpecIdsToNames modifies the spec in-place, converting sector/industry IDs
// in universe filters back to their names using the globally loaded maps.
// It clears the ID fields (IncludeIds/ExcludeIds) before returning.
// This should be called AFTER unmarshalling the spec from database JSON.
func convertSpecIdsToNames(spec *Spec) error {
	dynamicSetMutex.RLock() // Read lock needed for accessing validation maps
	defer dynamicSetMutex.RUnlock()

	for i := range spec.Universe.Filters {
		filter := &spec.Universe.Filters[i] // Operate on pointer to modify original
		var idMap map[int]string
		featureName := ""

		switch filter.SecurityFeature {
		case "sector":
			idMap = validSectorIds
			featureName = "sector"
		case "industry":
			idMap = validIndustryIds
			featureName = "industry"
		default:
			continue // Skip filters that are not sector or industry
		}

		// Convert Include IDs to names
		filter.Include = make([]string, 0, len(filter.IncludeIds))
		for _, id := range filter.IncludeIds {
			if name, ok := idMap[id]; ok {
				filter.Include = append(filter.Include, name)
			} else {
				// Data inconsistency? Log or return error
				log.Printf("Warning: universe filter %d: unknown %s ID '%d' found during name conversion", i, featureName, id)
				// Optionally return an error:
				// return fmt.Errorf("universe filter %d: unknown %s ID '%d' found during name conversion", i, featureName, id)
			}
		}

		// Convert Exclude IDs to names
		filter.Exclude = make([]string, 0, len(filter.ExcludeIds))
		for _, id := range filter.ExcludeIds {
			if name, ok := idMap[id]; ok {
				filter.Exclude = append(filter.Exclude, name)
			} else {
				log.Printf("Warning: universe filter %d: unknown %s ID '%d' found during name conversion", i, featureName, id)
				// Optionally return an error
			}
		}

		// Clear the ID fields as they are redundant for API response
		filter.IncludeIds = nil
		filter.ExcludeIds = nil
	}
	return nil
}


// Unmarshal and validation functions from JSON input

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

// temp struct for unmarshalling {strategyId, name, spec} input
type setStrategyInput struct {
    StrategyId int             `json:"strategyId"`
    Name       string          `json:"name"`
    Spec       json.RawMessage `json:"spec"`
}

// UnmarshalAndValidateSetStrategyInput parses and validates input for updating an existing strategy.
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
