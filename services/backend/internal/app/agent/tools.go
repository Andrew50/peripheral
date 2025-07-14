package agent

import (
	"backend/internal/app/alerts"
	"backend/internal/app/chart"
	"backend/internal/app/filings"
	"backend/internal/app/helpers"
	"backend/internal/app/screener"
	"backend/internal/app/strategy"
	"backend/internal/app/watchlist"
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

type Tool struct {
	FunctionDeclaration *genai.FunctionDeclaration
	Function            func(context.Context, *data.Conn, int, json.RawMessage) (interface{}, error)
	StatusMessage       string
}

// Wrapper function to adapt existing functions to context-aware signatures
func wrapWithContext(fn func(*data.Conn, int, json.RawMessage) (interface{}, error)) func(context.Context, *data.Conn, int, json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, conn *data.Conn, userID int, args json.RawMessage) (result interface{}, err error) {
		// Add panic recovery to prevent function panics from crashing the backend
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Panic recovered in function execution: %v\n", r)
				err = fmt.Errorf("function panic: %v", r)
				result = nil
			}
		}()

		// Check if context is cancelled before calling the function
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Set LLM execution flag for this function call
		conn.IsLLMExecution = true
		defer func() {
			conn.IsLLMExecution = false
		}()

		return fn(conn, userID, args)
	}
}

var (
	Tools = map[string]Tool{
		"getSecurityID": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSecurityID",
				Description: "Get the security ID for a given ticker. This is not necessary unless other tools require securityID as an argument.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol, e.g. NVDA, AAPL, etc",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function:      wrapWithContext(helpers.GetCurrentSecurityID),
			StatusMessage: "Looking up {ticker}...",
		},
		"getStockDetails": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStockDetails",
				Description: "Get company name, market, locale, primary exchange, market cap, shares outstanding, industry, sector and total shares for a given security.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityID": {
							Type:        genai.TypeInteger,
							Description: "The securityID of the security to get details for",
						},
					},
					Required: []string{"securityID"},
				},
			},
			Function:      wrapWithContext(helpers.GetAgentTickerMenuDetails),
			StatusMessage: "Getting {ticker} details...",
		},
		//watchlist
		"getWatchlists": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getWatchlists",
				Description: "Get all watchlist names and IDs.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{}, // Empty map indicates no properties/arguments
					Required:   []string{},
				},
			},
			Function:      wrapWithContext(watchlist.GetWatchlists),
			StatusMessage: "Fetching watchlists...",
		},
		"newWatchlist": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "newWatchlist",
				Description: "Create a new empty watchlist",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"watchlistName": {
							Type:        genai.TypeString,
							Description: "The name of the watchlist to create",
						},
					},
					Required: []string{"watchlistName"},
				},
			},
			Function:      wrapWithContext(watchlist.NewWatchlist),
			StatusMessage: "Creating new watchlist...",
		},
		"getWatchlistItems": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getWatchlistItems",
				Description: "Retrieves the security ID's of the securities in a specified watchlist.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"watchlistId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the watchlist to get the list of security IDs for.",
						},
					},
					Required: []string{"watchlistId"},
				},
			},
			Function:      wrapWithContext(watchlist.GetWatchlistItems),
			StatusMessage: "Getting watchlist items...",
		},
		"deleteWatchlistItem": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteWatchlistItem",
				Description: "Removes a security from a watchlist using a given watchlist item ID.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"watchlistItemId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the watchlist item to delete",
						},
					},
					Required: []string{"watchlistItemId"},
				},
			},
			Function:      wrapWithContext(watchlist.DeleteWatchlistItem),
			StatusMessage: "Removing item from watchlist...",
		},
		"newWatchlistItem": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "newWatchlistItem",
				Description: "Add a security to a watchlist.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"watchlistId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the watchlist to add the security to.",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to add to the watchlist.",
						},
					},
					Required: []string{"watchlistId", "securityId"},
				},
			},
			Function:      wrapWithContext(watchlist.NewWatchlistItem),
			StatusMessage: "Adding item to watchlist...",
		},
		"deleteWatchlist": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteWatchlist",
				Description: "Delete a watchlist and all its items.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"watchlistId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the watchlist to delete.",
						},
					},
					Required: []string{"watchlistId"},
				},
			},
			Function:      wrapWithContext(watchlist.DeleteWatchlist),
			StatusMessage: "Deleting watchlist...",
		},
		//singles

		"setHorizontalLine": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "setHorizontalLine",
				Description: "Create a new horizontal line on the chart of a specified security ID at a specificed price.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to add the horizontal line to.",
						},
						"price": {
							Type:        genai.TypeNumber,
							Description: "The price level for the horizontal line.",
						},
						"color": {
							Type:        genai.TypeString,
							Description: "The color of the horizontal line (hex format, defaults to #FFFFFF).",
						},
						"lineWidth": {
							Type:        genai.TypeInteger,
							Description: "The width of the horizontal line in pixels (defaults to 1).",
						},
					},
					Required: []string{"securityId", "price"},
				},
			},
			Function:      wrapWithContext(chart.SetHorizontalLine),
			StatusMessage: "Adding horizontal line...",
		},
		"getHorizontalLines": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getHorizontalLines",
				Description: "Retrieves all horizontal lines for a specific security",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to get horizontal lines for",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function:      wrapWithContext(chart.GetHorizontalLines),
			StatusMessage: "Fetching horizontal lines...",
		},
		"deleteHorizontalLine": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteHorizontalLine",
				Description: "Delete a horizontal line on the chart of a specified security ID.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id": {
							Type:        genai.TypeInteger,
							Description: "The ID of the horizontal line to delete.",
						},
					},
					Required: []string{"id"},
				},
			},
			Function:      wrapWithContext(chart.DeleteHorizontalLine),
			StatusMessage: "Deleting horizontal line...",
		},
		"updateHorizontalLine": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "updateHorizontalLine",
				Description: "Update an existing horizontal line on the chart of a specified security ID.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id": {
							Type:        genai.TypeInteger,
							Description: "The ID of the horizontal line to update.",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security the horizontal line belongs to.",
						},
						"price": {
							Type:        genai.TypeNumber,
							Description: "The new price level for the horizontal line.",
						},
						"color": {
							Type:        genai.TypeString,
							Description: "The new color of the horizontal line (hex format).",
						},
						"lineWidth": {
							Type:        genai.TypeInteger,
							Description: "The new width of the horizontal line in pixels.",
						},
					},
					Required: []string{"id", "securityId", "price"},
				},
			},
			Function:      wrapWithContext(chart.UpdateHorizontalLine),
			StatusMessage: "Updating horizontal line...",
		},
		"getStockEvents": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStockEvents",
				Description: "Retrieves splits, dividends and possibly SEC filings for a specified security ID within a date range",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to get events for.",
						},
						"from": {
							Type:        genai.TypeInteger,
							Description: "The start of the date range in milliseconds.",
						},
						"to": {
							Type:        genai.TypeInteger,
							Description: "The end of the date range in milliseconds.",
						},
						"includeSECFilings": {
							Type:        genai.TypeBoolean,
							Description: "Whether to include SEC filings in the result.",
						},
					},
					Required: []string{"securityId", "from", "to"},
				},
			},
			Function:      wrapWithContext(chart.GetChartEvents),
			StatusMessage: "Fetching chart events...",
		},
		// SEC Filing Tools
		"getStockEdgarFilings": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStockEdgarFilings",
				Description: "Retrieve a list of urls and filing types for all SEC filings for a specified security within a specified time range.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"start": {
							Type:        genai.TypeInteger,
							Description: "The start of the date range in milliseconds.",
						},
						"end": {
							Type:        genai.TypeInteger,
							Description: "The end of the date range in milliseconds.",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to get filings for.",
						},
					},
					Required: []string{"start", "end", "securityId"},
				},
			},
			Function:      wrapWithContext(filings.GetStockEdgarFilings),
			StatusMessage: "Searching SEC filings...",
		},
		"getEarningsText": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getEarningsText",
				Description: "Get the plain text content of the earnings SEC filing for a specified quarter, year, and security.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The security ID to get the filing for.",
						},
						"quarter": {
							Type:        genai.TypeString,
							Description: "The specific quarter (Q1, Q2, Q3, Q4) to retrieve the filing for, returns the latest filing if not specified.",
						},
						"year": {
							Type:        genai.TypeInteger,
							Description: "The specific year to retrieve the filing from.",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function:      wrapWithContext(filings.GetEarningsText),
			StatusMessage: "Getting earnings transcript...",
		},
		"getFilingText": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getFilingText",
				Description: "Retrieves the text content of a SEC filing from a specified url.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"url": {
							Type:        genai.TypeString,
							Description: "The URL of the SEC filing to retrieve.",
						},
					},
					Required: []string{"url"},
				},
			},
			Function:      wrapWithContext(filings.GetFilingText),
			StatusMessage: "Reading filing...",
		},
		"getExhibitList": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getExhibitList",
				Description: "Retrieves the list of exhibits for a specified SEC filing URL.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"url": {Type: genai.TypeString, Description: "The URL of the SEC filing to retrieve exhibits for."},
					},
					Required: []string{"url"},
				},
			},
			Function:      wrapWithContext(filings.GetExhibitList),
			StatusMessage: "Reading Exhibits in SEC Filing...",
		},
		"getExhibitContent": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getExhibitContent",
				Description: "Retrieves the content of a specific exhibit from a SEC filing. Use this after getExhibitList to get the content of an exhibit.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"url": {Type: genai.TypeString, Description: "The URL of the SEC exhibit to retrieve content for."},
					},
					Required: []string{"url"},
				},
			},
			Function:      wrapWithContext(filings.GetExhibitContent),
			StatusMessage: "Reading Exhibit Content...",
		},
		// <End SEC Filing Tools>
		// <Backtest Tools>
		"run_backtest": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "run_backtest",
				Description: "Execute a comprehensive historical backtest of a Python trading strategy to find all instances where the strategy conditions were met in historical data. Use this after creating a strategy to discover all historical occurrences of patterns, conditions, or comparative analysis. Strategies have access to rich market data including OHLCV data, 20+ technical indicators (SMA, EMA, RSI, MACD, Bollinger Bands), fundamental data (P/E, market cap, sector), and derived metrics. Returns all historical instances where the strategy criteria matched, along with timestamps, tickers, and relevant data. Execution typically takes 30-90 seconds for full market analysis. Use for finding historical patterns, identifying all occurrences of conditions, comparative analysis over time, and generating detailed historical results with optional forward return calculations.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {
							Type:        genai.TypeInteger,
							Description: "id of the strategy to backtest",
						},
						"startDate": {
							Type:        genai.TypeString,
							Description: "The start date of the backtest in YYYY-MM-DD format.",
						},
						"endDate": {
							Type:        genai.TypeString,
							Description: "The end date of the backtest in YYYY-MM-DD format.",
						},
					},
					Required: []string{"strategyId", "startDate", "endDate"},
				},
			},
			Function:      strategy.RunBacktest,
			StatusMessage: "Running backtest...",
		},
		"getBacktestInstances": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getBacktestInstances",
				Description: "Get the instances of a backtest for a specified strategy. Use this to look at the instances of a backtest for a specified strategy if the user refers to a backtest instance. Filters are required to get the specific instances the user might be looking for or that you want to analyze. Limited to 20 instances. ",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyID": {Type: genai.TypeInteger, Description: "The ID of the strategy to get instances for."},
						"filters": {
							Type: genai.TypeArray,
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"column":   {Type: genai.TypeString, Description: "The name of the column to filter on. e.g. 'ticker', 'timestamp', 'classification', 'volume', 'classification', 'score'."},
									"operator": {Type: genai.TypeString, Description: "The operator to use for the filter. e.g. 'eq', 'gt', 'gte', 'lt', 'lte', 'contains', 'in'."},
									"value":    {Type: genai.TypeUnspecified, Description: "The value to compare to. The value within the column will be compared using the operator to this value."},
								},
							},
							Required: []string{"column", "operator", "value"},
						},
					},
					Required: []string{"strategyID", "filters"},
				},
			},
			Function:      GetBacktestInstances,
			StatusMessage: "Scanning backtest instances...",
		},
		"run_screener": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "run_screener",
				Description: "Screen current market opportunities using a Python trading strategy. Processes live market data to identify and rank securities matching strategy criteria. Strategies access real-time OHLCV data, technical indicators, fundamental metrics, and market conditions. Execution takes 15-45 seconds for full market screening. Use for finding current trading opportunities, generating ranked watchlists, and identifying securities matching specific criteria right now.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {
							Type:        genai.TypeInteger,
							Description: "id of the strategy to use for screening",
						},
						"universe": {
							Type:        genai.TypeArray,
							Description: "Optional. List of ticker symbols to screen. If omitted, screens entire market universe.",
							Items: &genai.Schema{
								Type: genai.TypeString,
							},
						},
						"limit": {
							Type:        genai.TypeInteger,
							Description: "Optional. Maximum number of results to return (default: 100)",
						},
					},
					Required: []string{"strategyId"},
				},
			},
			Function:      wrapWithContext(strategy.RunScreening),
			StatusMessage: "Running screener...",
		},
		"runPythonAgent": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "runPythonAgent",
				Description: "[DO NOT RUN SEVERAL OF THESE IN PARALLEL.] Run a Python agent to analyze historical market data, do comparative analysis, create plot visualizations, or do other analysis. This is good for ad hoc data querying/analysis. For event driven analysis, use this. For more persistent backtesting, use run_backtest. This agent already has access to market data. DO NOT ASK FOR SPECIFIC RETURN TYPES OR INFORMATION IN THE QUERY.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"prompt": {Type: genai.TypeString, Description: "The NL query to pass to the Python agent."},
					},
					Required: []string{"prompt"},
				},
			},
			Function:      RunPythonAgent,
			StatusMessage: "Running Python agent...",
		},
		"getDailySnapshot": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getDailySnapshot",
				Description: "Get the current price (regular or extended hours), change, volume, OHLC, previous close for a specified stock.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol of the stock.",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function:      wrapWithContext(helpers.GetTickerDailySnapshot),
			StatusMessage: "Getting market data...",
		},
		"getLastPrice": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getLastPrice",
				Description: "Retrieves the last price (regular or extended hours) for a specified ticker symbol.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol to get the last price for, e.g. 'AAPL'.",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function:      wrapWithContext(helpers.GetLastPrice),
			StatusMessage: "Getting current price of {ticker}...",
		},
		/*"getPriceAtTime": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getPriceAtTime",
				Description: "Get the price of a stock at a specific time.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId":    {Type: genai.TypeInteger, Description: "The security ID of the stock to get the price for."},
						"timestamp":     {Type: genai.TypeInteger, Description: "The timestamp in seconds."},
						"splitAdjusted": {Type: genai.TypeBoolean, Description: "Optional. Whether the price should be split-adjusted. Default true."},
					},
					Required: []string{"securityId", "timestamp"},
				},
			},
			Function:      wrapWithContext(GetStockPriceAtTime),
			StatusMessage: "Getting price at time...",
		},*/
		/*"getStockChange": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStockChange",
				Description: "Returns the change and percent change in the price of a stock between two specific times.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId":    {Type: genai.TypeInteger, Description: "The security ID of the stock to get the change for."},
						"from":          {Type: genai.TypeInteger, Description: "The start of the date range in seconds."},
						"to":            {Type: genai.TypeInteger, Description: "The end of the date range in seconds."},
						"fromPoint":     {Type: genai.TypeString, Description: "Optional. The point of the day of the from timestamp to get the price from. 'open', 'high', 'low', 'close'. If omitted, the most recent price to the timestamp is used."},
						"toPoint":       {Type: genai.TypeString, Description: "Optional. The point of the day of the to timestamp to get the price to. 'open', 'high', 'low', 'close'. If omitted, the most recent price to the timestamp is used."},
						"splitAdjusted": {Type: genai.TypeBoolean, Description: "Optional. Whether the price should be split-adjusted. Default true."},
					},
					Required: []string{"securityId", "from", "to"},
				},
			},
			Function:      wrapWithContext(GetStockChange),
			StatusMessage: "Getting stock change...",
		},

		"getOHLCData": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getOHLCData",
				Description: "Get OHLCV data for a stock. Only use this function if other market data tools are not sufficient.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId":    {Type: genai.TypeInteger, Description: "The security ID to get OHLCV data."},
						"timeframe":     {Type: genai.TypeString, Description: "The timeframe. This is of the form 'n' + 'time_unit'. Minute data has no time unit, hour data is 'h', day data is 'd'. Supports second, minute, hour, day, week, and month."},
						"from":          {Type: genai.TypeInteger, Description: "The start of the date range in seconds."},
						"to":            {Type: genai.TypeInteger, Description: "Optional. The end of the date range in seconds."},
						"bars":          {Type: genai.TypeInteger, Description: "Required. The number of bars to get. MAX is 10."},
						"extended":      {Type: genai.TypeBoolean, Description: "Optional. Whether to include extended hours data. Defaults to false."},
						"splitAdjusted": {Type: genai.TypeBoolean, Description: "Optional. Whether the data should be split-adjusted. Defaults to true."},
						"columns":       {Type: genai.TypeArray, Description: "Optional. The columns to include in the OHLCV data. Use 'o' for open, 'h' for high, 'v' for volume, etc. Defaults to all columns."},
					},
					Required: []string{"securityId", "timeframe", "from", "bars"},
				},
			},
			Function:      wrapWithContext(GetOHLCVData),
			StatusMessage: "Getting Market data...",
		},*/
		/*"runIntradayAgent": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "runIntradayAgent",
				Description: "Run an intraday agent to analyze the intraday price action of a specified stock.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId":       {Type: genai.TypeInteger, Description: "The security ID to analyze."},
						"timeframe":        {Type: genai.TypeString, Description: "The timeframe to analyze. This is of the form 'n' + 'time_unit'. Minute data has no time unit e.g 1 minute is '1', hour data is 'h', day data is 'd'. Supports second, minute, hour, day, week."},
						"from":             {Type: genai.TypeInteger, Description: "The start of the date range in seconds."},
						"to":               {Type: genai.TypeInteger, Description: "The end of the date range in seconds."},
						"extended":         {Type: genai.TypeBoolean, Description: "Optional. Whether to include extended hours data. Defaults to false."},
						"splitAdjusted":    {Type: genai.TypeBoolean, Description: "Optional. Whether the data should be split-adjusted. Defaults to true."},
						"additionalPrompt": {Type: genai.TypeString, Description: "Optional. Additional prompt or context to pass to the intraday agent."},
					},
					Required: []string{"securityId", "timeframe", "from", "to"},
				},
			},
			Function:      wrapWithContext(RunIntradayAgent),
			StatusMessage: "Running intraday agent...",
		},*/
		// ────────────────────────────────────────────────────────────────────
		"getStrategies": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStrategies",
				Description: "Retrieves all strategy names and ids for the current user. Use this to fetch unknown strategy ids.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{},
					Required:   []string{},
				},
			},
			Function:      wrapWithContext(strategy.GetStrategies),
			StatusMessage: "Fetching strategies...",
		},
		"deleteStrategy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteStrategy",
				Description: "Deletes a strategy configuration",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {Type: genai.TypeInteger, Description: "Strategy ID"},
					},
					Required: []string{"strategyId"},
				},
			},
			Function:      wrapWithContext(strategy.DeleteStrategy),
			StatusMessage: "Deleting strategy...",
		},
		"runStrategyAgent": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "runStrategyAgent",
				Description: "Create or edit a Python strategy from natural language description for pattern detection, historical analysis, and comparative studies. Use this tool for requests that involve finding patterns in historical data, comparing stocks over time, or identifying specific market conditions. Examples: 'find all times X happened', 'get instances when Y condition was met', 'compare A vs B performance', 'identify patterns in historical data'. Strategies are automatically generated as secure Python functions with access to comprehensive market data (OHLCV, technical indicators, fundamentals). Generated strategies can be used for backtesting historical patterns, screening current opportunities, and real-time monitoring. Creation process includes security validation and takes 15-30 seconds with priority queue processing. IF YOU USE THIS FUNCTION TO CREATE A NEW STRATEGY, USE THE USER'S ORIGINAL QUERY AS IS. This agent does not have access to websearch. First websearch any required information and then pass it in with the query.Pass strategyId = -1 to create a new strategy.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query":      {Type: genai.TypeString, Description: "Original NL query"},
						"strategyId": {Type: genai.TypeInteger, Description: "-1 for new strategy, else overwrite"},
					},
					Required: []string{"query", "strategyId"},
				},
			},
			Function:      wrapWithContext(strategy.CreateStrategyFromPrompt),
			StatusMessage: "Building strategy...",
		},
		// [SEARCH TOOLS]
		"runWebSearch": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "runWebSearch",
				Description: "Run a web search using Google Search. Never use web search to look up historical performance or stock analysis.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query": {Type: genai.TypeString, Description: "The query to search. Be highly specific and detailed, asking for the specific information you need. "},
					},
					Required: []string{"query"},
				},
			},
			Function:      wrapWithContext(RunWebSearch),
			StatusMessage: "Searching the web...",
		},
		/*"runTwitterSearch": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "runTwitterSearch",
				Description: "Run a search on X using Grok.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"prompt":   {Type: genai.TypeString, Description: "The query to search for. Be HIGHLY specific and detailed, asking for the specific information you need."},
						"handles":  {Type: genai.TypeArray, Description: "A list of Twitter handles. If omitted, the search will search the entirety of Twitter."},
						"fromDate": {Type: genai.TypeString, Description: "The date YYYY-MM-DD to start searching from."},
						"toDate":   {Type: genai.TypeString, Description: "The date YYYY-MM-DD to stop searching at."},
					},
					Required: []string{"prompt"},
				},
			},
			Function:      wrapWithContext(RunTwitterSearch),
			StatusMessage: "Searching Twitter...",
		},*/ //COMMENTED out for now since we ran out of credits
		/*"getLatestTweets": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getLatestTweets",
				Description: "Get the latest tweets from a list of Twitter handles.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"handles": {Type: genai.TypeArray, Description: "A list of Twitter handles to search for."},
					},
					Required: []string{"handles"},
				},
			},
			Function:      wrapWithContext(GetLatestTweets),
			StatusMessage: "Searching Twitter...",
		},*/
		// [END SEARCH TOOLS]
		// [ALERT TOOLS]
		"createPriceAlert": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "createPriceAlert",
				Description: "Create a new price alert for a specific security. The alert will trigger when the price reaches the specified level.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"price": {
							Type:        genai.TypeNumber,
							Description: "The price level at which the alert should trigger.",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The security ID of the stock to create the alert for.",
						},
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol of the stock (e.g., 'AAPL', 'NVDA').",
						},
					},
					Required: []string{"price", "securityId", "ticker"},
				},
			},
			Function:      wrapWithContext(alerts.NewAlert),
			StatusMessage: "Creating price alert...",
		},
		"getAlerts": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getAlerts",
				Description: "Get all current price alerts for the user, including active and triggered alerts.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{}, // No parameters needed
					Required:   []string{},
				},
			},
			Function:      wrapWithContext(alerts.GetAlerts),
			StatusMessage: "Fetching alerts...",
		},
		"getAlertLogs": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getAlertLogs",
				Description: "Get all triggered/fired price alerts for the user. These are alerts that have been activated.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{}, // No parameters needed
					Required:   []string{},
				},
			},
			Function:      wrapWithContext(alerts.GetAlertLogs),
			StatusMessage: "Fetching alert history...",
		},
		"deleteAlert": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteAlert",
				Description: "Delete a specific price alert by its alert ID.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"alertId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the alert to delete.",
						},
					},
					Required: []string{"alertId"},
				},
			},
			Function:      wrapWithContext(alerts.DeleteAlert),
			StatusMessage: "Deleting alert...",
		},
		// [END ALERT TOOLS]
		// [SCREENER TOOLS]
		"runScreener": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "runScreener",
				Description: "Screen stocks based on financial metrics, technical indicators, and market data. Filter securities using price, volume, performance, sector, technical indicators (RSI, moving averages), and more. Supports 47+ data columns including OHLCV, market cap, sector/industry, pre-market data, volatility, beta, and performance metrics across multiple timeframes. Use comparison operators (>, <, =, !=, >=, <=), pattern matching (LIKE), set operations (IN), and ranking filters (topn, bottomn, topn_pct, bottomn_pct). Results can be ordered with custom sort direction (ASC/DESC) and limited. Perfect for finding stocks matching specific criteria, generating watchlists, or analyzing market segments.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"returnColumns": {
							Type:        genai.TypeArray,
							Description: "Array of column names to return in results. Available columns: security_id, open, high, low, close, wk52_low, wk52_high, pre_market_open, pre_market_high, pre_market_low, pre_market_close, market_cap, sector, industry, pre_market_change, pre_market_change_pct, extended_hours_change, extended_hours_change_pct, change_1_pct, change_15_pct, change_1h_pct, change_4h_pct, change_1d_pct, change_1w_pct, change_1m_pct, change_3m_pct, change_6m_pct, change_ytd_1y_pct, change_5y_pct, change_10y_pct, change_all_time_pct, change_from_open, change_from_open_pct, price_over_52wk_high, price_over_52wk_low, rsi, dma_200, dma_50, price_over_50dma, price_over_200dma, beta_1y_vs_spy, beta_1m_vs_spy, volume, avg_volume_1m, dollar_volume, avg_dollar_volume_1m, pre_market_volume, pre_market_dollar_volume, relative_volume_14, pre_market_vol_over_14d_vol, range_1m_pct, range_15m_pct, range_1h_pct, day_range_pct, volatility_1w, volatility_1m, pre_market_range_pct. At least one column is required.",
							Items: &genai.Schema{
								Type: genai.TypeString,
							},
						},
						"orderBy": {
							Type:        genai.TypeString,
							Description: "Optional. Column name to order results by. Must be one of the available columns.",
						},
						"sortDirection": {
							Type:        genai.TypeString,
							Description: "Optional. Sort direction for ordering results. Must be 'ASC' or 'DESC' (case insensitive). Defaults to 'ASC' if not specified.",
						},
						"limit": {
							Type:        genai.TypeInteger,
							Description: "Maximum number of results to return. Must be between 1 and 10,000. Defaults to 100 if not specified.",
						},
						"filters": {
							Type:        genai.TypeArray,
							Description: "Array of filter objects to apply to the screener query. Each filter specifies a column, operator, and value.",
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"column": {
										Type:        genai.TypeString,
										Description: "Column name to filter on. Must be one of the available screener columns.",
									},
									"operator": {
										Type:        genai.TypeString,
										Description: "Comparison operator. Options: '=', '!=', '>', '<', '>=', '<=', 'LIKE' (for strings), 'IN' (for arrays), 'topn', 'bottomn', 'topn_pct', 'bottomn_pct' (for ranking).",
									},
									"value": {
										Type:        genai.TypeUnspecified,
										Description: "Value to compare against. Type must match column type. For IN operator, use array. For ranking operators (topn/bottomn), use positive integer.",
									},
								},
								Required: []string{"column", "operator", "value"},
							},
						},
					},
					Required: []string{"returnColumns", "limit"},
				},
			},
			Function:      wrapWithContext(screener.GetScreenerData),
			StatusMessage: "Screening stocks...",
		},
		// [END SCREENER TOOLS]
		// [MODEL HELPERS]
		"dateToSeconds": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "dateToSeconds",
				Description: "Convert one or more dates to seconds since epoch (Unix timestamp in seconds). Pass an array of date objects. Example: {\"dates\": [{\"date\": \"2024-01-01\", \"hour\": 9, \"minute\": 30}, {\"date\": \"2024-01-02\"}]}",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dates": {
							Type:        genai.TypeArray,
							Description: "Array of date objects. Each object must have a 'date' field in YYYY-MM-DD format. Hour, minute, and second are optional and default to 0.",
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"date":   {Type: genai.TypeString, Description: "Date in YYYY-MM-DD format (required)"},
									"hour":   {Type: genai.TypeInteger, Description: "Hour in 24-hour format (0-23), defaults to 0"},
									"minute": {Type: genai.TypeInteger, Description: "Minute (0-59), defaults to 0"},
									"second": {Type: genai.TypeInteger, Description: "Second (0-59), defaults to 0"},
								},
								Required: []string{"date"},
							},
						},
					},
					Required: []string{"dates"},
				},
			},
			Function:      wrapWithContext(DateToSeconds),
			StatusMessage: "Converting dates to timestamps...",
		},
	}
)
