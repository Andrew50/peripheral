package strategy

import (
	"time"
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

// Type definitions for the old complex system (kept for backward compatibility of existing code)
type FeatureID int
type OHLCVFeature string       //open, high, low, close
type FundamentalFeature string //market_cap, total_shares, active //eventually eps, revenue, news stuff
type SecurityFeature string    //securityId, ticker, locale, market, primary_exchange, active, sector, industry //doesnt change over time
type OutputType string         //raw, rankn, rankp
type ComparisonOperator string // >, >=, <, <=
type ExprOperator string       //+, -, *, /, ^
type Direction string          // asc, desc
type Timeframe string          // 1, 1h, 1d, 1w

// Legacy structs for backward compatibility - these are no longer used in the new prompt-based system
type LegacyStrategy struct {
	Name              string    `json:"name"`
	StrategyID        int       `json:"strategyId"`
	UserID            int       `json:"userId"`
	Version           int       `json:"version"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
	Score             int       `json:"score"`
	Complexity        int       `json:"complexity"`  // Estimated compute time / parameter count
	AlertActive       bool      `json:"alertActive"` // Indicates if real-time alerts are enabled
	Spec              Spec      `json:"spec"`        // Strategy specification
}

// Minimal legacy structs needed for compilation (deprecated)
type UniverseFilter struct {
	SecurityFeature SecurityFeature `json:"securityFeature"`
	Include         []string        `json:"include"`
	Exclude         []string        `json:"exclude"`
}

type Universe struct {
	Filters       []UniverseFilter `json:"filters"`
	Timeframe     Timeframe        `json:"timeframe"`
	ExtendedHours bool             `json:"extendedHours"`
	StartTime     time.Time        `json:"startTime"`
	EndTime       time.Time        `json:"endTime"`
}

type FeatureSource struct {
	Field SecurityFeature `json:"field"`
	Value string          `json:"value"`
}

type ExprPart struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Offset int    `json:"offset"`
}

type Feature struct {
	Name      string        `json:"name"`
	FeatureID FeatureID     `json:"featureId"`
	Source    FeatureSource `json:"source"`
	Output    OutputType    `json:"output"`
	Expr      []ExprPart    `json:"expr"`
	Window    int           `json:"window"`
}

type Filter struct {
	Name     string             `json:"name"`
	LHS      FeatureID          `json:"lhs"`
	Operator ComparisonOperator `json:"operator"`
	RHS      struct {
		FeatureID FeatureID `json:"featureId"`
		Const     float64   `json:"const"`
		Scale     float64   `json:"scale"`
	} `json:"rhs"`
}

type SortBy struct {
	Feature   FeatureID `json:"feature"`
	Direction Direction `json:"direction"`
}

type Spec struct {
	Universe Universe  `json:"universe"`
	Features []Feature `json:"features"`
	Filters  []Filter  `json:"filters"`
	SortBy   SortBy    `json:"sortBy"`
}

// ... rest of legacy structs remain for backward compatibility but are deprecated ...
