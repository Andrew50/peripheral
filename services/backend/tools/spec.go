package tools

type FeatureSource string

const (
	ColumnSrc      FeatureSource = "raw"        // raw DB column
	TASrc          FeatureSource = "derived"    // technical equation
	FundamentalSrc FeatureSource = "fundamental" // fundamentals table
	RelativeSrc    FeatureSource = "relative"   // sector / peer aggregates
	NewsSrc        FeatureSource = "news"       // news sentiment, etc.
)

type Operator string

const (
	OpEQ Operator = "=="
	OpNE Operator = "!="
	OpGT Operator = ">"
	OpLT Operator = "<"
	OpGE Operator = ">="
	OpLE Operator = "<="
)



// -----------  Comparison ------------------
type Operand struct { // this will do mult*feature[shift] + const , therefore if feature undefined then just a const
	FeatureId   int      `json:"feature_id,omitempty"` // ID of the feature to reference
	ConstString *string  `json:"const_string,omitempty"` // string constant value
	ConstFloat  *float64 `json:"const_float,omitempty"`  // float constant value
	Multiplier  *int     `json:"multiplier,omitempty"`   // multiplier for feature value
}

type Feature struct {
	Id        int           `json:"id"`
	Column    string        `json:"column"`      // time, open, high, low, close, sentiment, news importance, sector, industry, market cap, shares outstanding, fundamental data, ticker, whatever is stored in the database
	Source    FeatureSource `json:"source"`      // source of the feature data
	Timeframe string        `json:"timeframe,omitempty"` // only used for technical features
	Modifier  string        `json:"modifier,omitempty"`  // stddev, mean, median, rsi, macd, raw
	Period    int           `json:"period,omitempty"`    // period for technical indicators
	Shift     int           `json:"shift,omitempty"`     // bars back, put this here instead of operand because you usually display feature values after backtest in query
    //ScopeId    *ScopeId       `json:"scope_id,omitempty"`  // ← one small pointer
}

type ComparisonId int

type Comparison struct { // evaluates to true or false
	ID   ComparisonId `json:"id"`              // unique identifier for this comparison
	LHS  Operand      `json:"lhs"`             // left-hand side operand
	Op   string       `json:"op"`              // >, <, ==, !=, <=, >=
	RHS  Operand      `json:"rhs"`             // right-hand side operand
}

// -----------  Rules (recursive) -----------
type RuleNode struct {
	All          []RuleNode   `json:"all,omitempty"`        // AND condition (all must be true)
	Any          []RuleNode   `json:"any,omitempty"`        // OR condition (any can be true)
	None         []RuleNode   `json:"none,omitempty"`       // NOT condition (none can be true)
	ComparisonId ComparisonId `json:"comparison_id,omitempty"` // reference to a comparison
}

type Stats struct {
	Score int `json:"score"`
}

type Spec struct {
    Spec    *Spec   `json:"spec"`
	Comparisons []Comparison `json:"comparisons"`
	Features    []Feature    `json:"features"`
	Logic       []RuleNode   `json:"logic"`
}

type Strategy struct {
	Id          int     `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Version     int     `json:"version"`
	Spec        Spec    `json:"spec"`
	Stats       Stats   `json:"stats,omitempty"`
	Deployed    bool    `json:"deployed"`
}
