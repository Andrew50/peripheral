package agent

import (
	"backend/internal/app/chart"
	"backend/internal/app/filings"
	"backend/internal/app/helpers"
	"backend/internal/app/strategy"
	"backend/internal/app/watchlist"
	"backend/internal/data"
	"context"
	"encoding/json"

	"google.golang.org/genai"
)

type Tool struct {
	FunctionDeclaration *genai.FunctionDeclaration
	Function            func(context.Context, *data.Conn, int, json.RawMessage) (interface{}, error)
	StatusMessage       string
}

// Wrapper function to adapt existing functions to context-aware signatures
func wrapWithContext(fn func(*data.Conn, int, json.RawMessage) (interface{}, error)) func(context.Context, *data.Conn, int, json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, conn *data.Conn, userID int, args json.RawMessage) (interface{}, error) {
		// Check if context is cancelled before calling the function
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return fn(conn, userID, args)
	}
}

var (
	Tools = map[string]Tool{
		"getSecurityID": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSecurityID",
				Description: "Get the security ID from a security ticker symbol.",
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
				Description: "Get company name, market, locale, primary exchange, active status, market cap, description, logo, shares outstanding, industry, sector and total shares for a given security.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The security ticker symbol to get details for.",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The securityID of the security to get details for",
						},
					},
					Required: []string{"securityID"},
				},
			},
			Function:      wrapWithContext(helpers.GetTickerMenuDetails),
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
		"deleteWatchlist": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteWatchlist",
				Description: "Delete a watchlist.",
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
				Description: "Backtest a specified strategy, which is based on stock conditions, patterns, and indicators. Can optionally calculate future returns for specified N-day windows.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {
							Type:        genai.TypeInteger,
							Description: "id of the strategy to backtest",
						},
						"returnWindows": {
							Type:        genai.TypeArray,                                                                                                                                                              // Changed from TypeInteger
							Description: "A list of integers representing the specific forward return days (e.g., [1, 5, 20]) to calculate after each backtest result. If omitted, no future returns are calculated.", // Updated description
							Items: &genai.Schema{ // Specify the type of elements in the array
								Type: genai.TypeInteger,
							},
						},
					},
					Required: []string{"strategyId"},
				},
			},
			Function:      strategy.RunBacktest,
			StatusMessage: "Running backtest...",
		},
		"getDailySnapshot": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getDailySnapshot",
				Description: "Get the current (regular or extended hours) price, change, volume, OHLC, previous close for a specified stock.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The security ID of the stock.",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function:      wrapWithContext(helpers.GetTickerDailySnapshot),
			StatusMessage: "Getting market data...",
		},
		"getLastPrice": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getLastPrice",
				Description: "Retrieves the last price (regular or extended hours)for a specified security ticker symbol.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol to get the last price for.",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function:      wrapWithContext(helpers.GetLastPrice),
			StatusMessage: "Getting current price of {ticker}...",
		},
		"getStockPriceAtTime": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStockPriceAtTime",
				Description: "Get the price of a stock at a specific time.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId":    {Type: genai.TypeInteger, Description: "The security ID of the stock to get the price for."},
						"timestamp":     {Type: genai.TypeInteger, Description: "The timestamp in milliseconds."},
						"splitAdjusted": {Type: genai.TypeBoolean, Description: "Optional. Whether the price should be split-adjusted. Default true."},
					},
					Required: []string{"securityId", "timestamp"},
				},
			},
			Function:      wrapWithContext(AgentGetStockPriceAtTime),
			StatusMessage: "Getting price at time...",
		},
		"getStockChange": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStockChange",
				Description: "Get the change in price of a stock between two specific times.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId":    {Type: genai.TypeInteger, Description: "The security ID of the stock to get the change for."},
						"from":          {Type: genai.TypeInteger, Description: "The start of the date range in milliseconds."},
						"to":            {Type: genai.TypeInteger, Description: "The end of the date range in milliseconds."},
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
		/*"getAllTickerSnapshots": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getAllTickerSnapshots",
				Description: "Get a list of the current bid, ask, price, change, percent change, volume, vwap price, and daily open, high, low and close for all securities.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{},
					Required:   []string{},
				},
			},
			Function:      wrapWithContext(helpers.GetAllTickerSnapshots),
			StatusMessage: "Scanning market data...",
		}, */

		"getOHLCVData": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getOHLCVData",
				Description: "Get OHLCV data for a stock",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId":    {Type: genai.TypeInteger, Description: "The security ID to get OHLCV data."},
						"timeframe":     {Type: genai.TypeString, Description: "The timeframe. This is of the form 'n' + 'time_unit'. Minute data has no time unit, hour data is 'h', day data is 'd'. Supports second, minute, hour, day, week, and month."},
						"from":          {Type: genai.TypeInteger, Description: "The start of the date range in milliseconds."},
						"to":            {Type: genai.TypeInteger, Description: "Optional. The end of the date range in milliseconds."},
						"bars":          {Type: genai.TypeInteger, Description: "Required. The number of bars to get. Max is 300."},
						"extended":      {Type: genai.TypeBoolean, Description: "Optional. Whether to include extended hours data. Defaults to false."},
						"splitAdjusted": {Type: genai.TypeBoolean, Description: "Optional. Whether the data should be split-adjusted. Defaults to true."},
						"columns":       {Type: genai.TypeArray, Description: "Optional. The columns to include in the OHLCV data. Use 'o' for open, 'h' for high, 'v' for volume, etc. Defaults to all columns."},
					},
					Required: []string{"securityId", "timeframe", "from", "bars"},
				},
			},
			Function:      wrapWithContext(GetOHLCVData),
			StatusMessage: "Getting Market data...",
		},
		"runIntradayAgent": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "runIntradayAgent",
				Description: "Run an intraday agent to analyze the intraday price action of a specified stock.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId":       {Type: genai.TypeInteger, Description: "The security ID to analyze."},
						"timeframe":        {Type: genai.TypeString, Description: "The timeframe to analyze. This is of the form 'n' + 'time_unit'. Minute data has no time unit e.g 1 minute is '1', hour data is 'h', day data is 'd'. Supports second, minute, hour, day, week."},
						"from":             {Type: genai.TypeInteger, Description: "The start of the date range in milliseconds."},
						"to":               {Type: genai.TypeInteger, Description: "The end of the date range in milliseconds."},
						"extended":         {Type: genai.TypeBoolean, Description: "Optional. Whether to include extended hours data. Defaults to false."},
						"splitAdjusted":    {Type: genai.TypeBoolean, Description: "Optional. Whether the data should be split-adjusted. Defaults to true."},
						"additionalPrompt": {Type: genai.TypeString, Description: "Optional. Additional prompt or context to pass to the intraday agent."},
					},
					Required: []string{"securityId", "timeframe", "from", "to"},
				},
			},
			Function:      wrapWithContext(RunIntradayAgent),
			StatusMessage: "Running intraday agent...",
		},
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
		"getStrategyFromNaturalLanguage": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name: "getStrategyFromNaturalLanguage",
				Description: "IF YOU USE THIS FUNCTION TO CREATE A NEW STRATEGY, USE THE USER'S ORIGINAL QUERY AS IS. Create (or overwrite) a strategy from a natural‑language description. " +
					"Pass strategyId = -1 to create a new strategy.",
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
		"calculateBacktestStatistic": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "calculateBacktestStatistic",
				Description: "Calculates a statistic for a specific column from cached backtest results. Use this instead of requesting raw backtest data for simple calculations.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the strategy whose backtest results should be used.",
						},
						"columnName": {
							Type:        genai.TypeString,
							Description: "The original column name in the backtest results to perform the calculation on (e.g., 'close', 'volume', 'future_1d_return').",
						},
						"calculationType": {
							Type:        genai.TypeString,
							Description: "The type of calculation to perform. Supported values: 'average', 'sum', 'min', 'max', 'count'.",
						},
					},
					Required: []string{"strategyId", "columnName", "calculationType"},
				},
			},
			Function:      wrapWithContext(CalculateBacktestStatistic),
			StatusMessage: "Calculating backtest statistics...",
		},
		// [SEARCH TOOLS]
		"runWebSearch": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "runWebSearch",
				Description: "Run a web search using Google Search.",
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
		"runTwitterSearch": {
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
		},
		"getLatestTweets": {
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
		},
		// [END SEARCH TOOLS]
		// [MODEL HELPERS]
		"dateToMS": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "dateToMS",
				Description: "Convert one or more dates to milliseconds since epoch (Unix timestamp in milliseconds). Pass an array of date objects. Example: {\"dates\": [{\"date\": \"2024-01-01\", \"hour\": 9, \"minute\": 30}, {\"date\": \"2024-01-02\"}]}",
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
			Function:      wrapWithContext(DateToMS),
			StatusMessage: "Converting dates to timestamps...",
		},
	}
)
