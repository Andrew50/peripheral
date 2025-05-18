package agent

import (
	"backend/internal/app/account"
	"backend/internal/app/chart"
	"backend/internal/app/filings"
	"backend/internal/app/helpers"
	"backend/internal/app/screensaver"
	"backend/internal/app/strategy"
	"backend/internal/app/watchlist"
	"backend/internal/data"
	"encoding/json"

	"google.golang.org/genai"
)

type Tool struct {
	FunctionDeclaration *genai.FunctionDeclaration
	Function            func(*data.Conn, int, json.RawMessage) (interface{}, error)
	StatusMessage       string
}

var (
	Tools = map[string]Tool{
		"getCurrentSecurityID": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getCurrentSecurityID",
				Description: "Return the integer securityId for a ticker symbol.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The security ticker symbol, e.g. NVDA, AAPL, etc",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function:      helpers.GetCurrentSecurityID,
			StatusMessage: "Looking up {ticker}...",
		},
		"getSecuritiesFromTicker": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSecuritiesFromTicker",
				Description: "Search a partial ticker string and return up to 10 matching tickers with securityId, name and icon.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "string input to retrieve the list based on.",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function:      helpers.GetSecuritiesFromTicker,
			StatusMessage: "Searching for matching tickers...",
		},
		"getCurrentTicker": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getCurrentTicker",
				Description: "Return the current ticker symbol for a given securityId.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The security ID of the security to get the current ticker for.",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function:      helpers.GetCurrentTicker,
			StatusMessage: "Looking up {ticker}...",
		},
		"getTickerMenuDetails": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getTickerMenuDetails",
				Description: "Return company information such as name, market, market cap, industry and more for a ticker or securityId.",
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
					Required: []string{"ticker", "securityId"},
				},
			},
			Function:      helpers.GetTickerMenuDetails,
			StatusMessage: "Getting {ticker} details...",
		},
		"getInstancesByTickers": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getInstancesByTickers",
				Description: "Return securityIds for a list of ticker symbols.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"tickers": {
							Type:        genai.TypeArray,
							Description: "List of security ticker symbols.",
							Items: &genai.Schema{
								Type: genai.TypeString,
							},
						},
					},
					Required: []string{"tickers"},
				},
			},
			Function:      screensaver.GetInstancesByTickers,
			StatusMessage: "Looking up tickers...",
		},
		//watchlist
		"getWatchlists": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getWatchlists",
				Description: "Return all watchlist names and their IDs.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{}, // Empty map indicates no properties/arguments
					Required:   []string{},
				},
			},
			Function:      watchlist.GetWatchlists,
			StatusMessage: "Fetching watchlists...",
		},
		"deleteWatchlist": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteWatchlist",
				Description: "Delete a watchlist by ID. Returns null on success.",
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
			Function:      watchlist.DeleteWatchlist,
			StatusMessage: "Deleting watchlist...",
		},
		"newWatchlist": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "newWatchlist",
				Description: "Create a new watchlist and return the new watchlistId",
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
			Function:      watchlist.NewWatchlist,
			StatusMessage: "Creating new watchlist...",
		},
		"getWatchlistItems": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getWatchlistItems",
				Description: "Return the securityIds contained in a watchlist.",
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
			Function:      watchlist.GetWatchlistItems,
			StatusMessage: "Getting watchlist items...",
		},
		"deleteWatchlistItem": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteWatchlistItem",
				Description: "Delete a watchlist item by id. Returns null on success.",
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
			Function:      watchlist.DeleteWatchlistItem,
			StatusMessage: "Removing item from watchlist...",
		},
		"newWatchlistItem": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "newWatchlistItem",
				Description: "Add a security to a watchlist and return the new watchlistItemId.",
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
			Function:      watchlist.NewWatchlistItem,
			StatusMessage: "Adding item to watchlist...",
		},
		//singles
		"getPrevClose": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getPrevClose",
				Description: "Return the previous closing price for a securityId. Uses the most recent price if the market is closed.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The security ID of the stock to get the previous close for.",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function:      helpers.GetPrevClose,
			StatusMessage: "Getting previous closing price...",
		},
		"getLastPrice": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getLastPrice",
				Description: "Return the last trade price for a ticker symbol.",
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
			Function:      helpers.GetLastPrice,
			StatusMessage: "Getting current price of {ticker}...",
		},
		"setHorizontalLine": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "setHorizontalLine",
				Description: "Create a horizontal line on a chart and return the new lineId.",
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
			Function:      chart.SetHorizontalLine,
			StatusMessage: "Adding horizontal line...",
		},
		"getHorizontalLines": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getHorizontalLines",
				Description: "Return all horizontal lines for a securityId.",
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
			Function:      chart.GetHorizontalLines,
			StatusMessage: "Fetching horizontal lines...",
		},
		"deleteHorizontalLine": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteHorizontalLine",
				Description: "Delete a horizontal line by id and return null on success.",
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
			Function:      chart.DeleteHorizontalLine,
			StatusMessage: "Deleting horizontal line...",
		},
		"updateHorizontalLine": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "updateHorizontalLine",
				Description: "Update a horizontal line's parameters and return null on success.",
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
			Function:      chart.UpdateHorizontalLine,
			StatusMessage: "Updating horizontal line...",
		},
		"getStockEdgarFilings": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStockEdgarFilings",
				Description: "Return URLs and filing types for a securityId within a date range.",
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
			Function:      filings.GetStockEdgarFilings,
			StatusMessage: "Searching SEC filings...",
		},
		"getChartEvents": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getChartEvents",
				Description: "Return splits, dividends and optional filing events for a securityId within a date range.",
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
			Function:      chart.GetChartEvents,
			StatusMessage: "Fetching chart events...",
		},
		"getEarningsText": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getEarningsText",
				Description: "Return the plain text of the earnings filing for a securityId, optionally for a specific quarter and year.",
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
			Function:      filings.GetEarningsText,
			StatusMessage: "Getting earnings transcript...",
		},
		"getFilingText": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getFilingText",
				Description: "Return the text content of an SEC filing from its URL.",
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
			Function:      filings.GetFilingText,
			StatusMessage: "Reading filing...",
		},
		"getExhibitList": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getExhibitList",
				Description: "Return the list of exhibits for a filing URL.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"url": {Type: genai.TypeString, Description: "The URL of the SEC filing to retrieve exhibits for."},
					},
					Required: []string{"url"},
				},
			},
			Function:      filings.GetExhibitList,
			StatusMessage: "Reading Exhibits in SEC Filing...",
		},
		"getExhibitContent": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getExhibitContent",
				Description: "Return the text content for a specific exhibit URL. Call after getExhibitList.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"url": {Type: genai.TypeString, Description: "The URL of the SEC exhibit to retrieve content for."},
					},
					Required: []string{"url"},
				},
			},
			Function:      filings.GetExhibitContent,
			StatusMessage: "Reading Exhibit Content...",
		},
		// Account / User Trades
		"grab_user_trades": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "grab_user_trades",
				Description: "Return the user's trades, optionally filtered by ticker and date range.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "Ticker symbol to grab trades for.",
						},
						"startDate": {
							Type:        genai.TypeString,
							Description: "Date range start to filter trades by (format: YYYY-MM-DD).",
						},
						"endDate": {
							Type:        genai.TypeString,
							Description: "Date range end to filter trades by (format: YYYY-MM-DD).",
						},
					},
					Required: []string{},
				},
			},
			Function:      account.GrabUserTrades,
			StatusMessage: "Fetching trades...",
		},
		"get_trade_statistics": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "get_trade_statistics",
				Description: "Return overall trading statistics, optionally filtered by ticker and date range.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "Security ticker symbol to filter trades by.",
						},
						"startDate": {
							Type:        genai.TypeString,
							Description: "Date range start to filter trades by (format: YYYY-MM-DD).",
						},
						"endDate": {
							Type:        genai.TypeString,
							Description: "Date range end to filter trades by (format: YYYY-MM-DD).",
						},
					},
					Required: []string{},
				},
			},
			Function:      account.GetTradeStatistics,
			StatusMessage: "Calculating trade statistics...",
		},
		"get_ticker_performance": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "get_ticker_performance",
				Description: "Return trade performance metrics for a ticker or securityId.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The security ticker symbol to get performance statistics for.",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The security ID to get performance statistics for.",
						},
					},
					Required: []string{"ticker", "securityId"},
				},
			},
			Function:      account.GetTickerPerformance,
			StatusMessage: "Analyzing ticker performance for {ticker}...",
		},
		"get_daily_trade_stats": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "get_daily_trade_stats",
				Description: "Return daily trading statistics for a given year and month.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"year": {
							Type:        genai.TypeInteger,
							Description: "The year part of the date to get statistics for.",
						},
						"month": {
							Type:        genai.TypeInteger,
							Description: "The month part of the date to get statistics for.",
						},
					},
					Required: []string{"year", "month"},
				},
			},
			Function:      account.GetDailyTradeStats,
			StatusMessage: "Getting daily trade stats...",
		},
		"run_backtest": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "run_backtest",
				Description: "Run a strategy backtest and return a summary object. Optionally calculate forward returns for specific windows.",
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
		"getTickerDailySnapshot": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getTickerDailySnapshot",
				Description: "Return the latest daily snapshot values for a securityId.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The security ID to get the information for.",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function:      helpers.GetTickerDailySnapshot,
			StatusMessage: "Getting daily market data...",
		},
		"getAllTickerSnapshots": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getAllTickerSnapshots",
				Description: "Return daily snapshot information for all securities.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{},
					Required:   []string{},
				},
			},
			Function:      helpers.GetAllTickerSnapshots,
			StatusMessage: "Scanning market data...",
		},
		// ────────────────────────────────────────────────────────────────────
		"getStrategies": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStrategies",
				Description: "Return all strategy names and their ids for the current user.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{},
					Required:   []string{},
				},
			},
			Function:      strategy.GetStrategies,
			StatusMessage: "Fetching strategies...",
		},
		"deleteStrategy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteStrategy",
				Description: "Delete a strategy by id. Returns null on success.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {Type: genai.TypeInteger, Description: "Strategy ID"},
					},
					Required: []string{"strategyId"},
				},
			},
			Function:      strategy.DeleteStrategy,
			StatusMessage: "Deleting strategy...",
		},
		"getStrategyFromNaturalLanguage": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStrategyFromNaturalLanguage",
				Description: "Create or overwrite a strategy from a natural language query. Pass strategyId=-1 to create a new strategy.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query":      {Type: genai.TypeString, Description: "Original NL query"},
						"strategyId": {Type: genai.TypeInteger, Description: "-1 for new strategy, else overwrite"},
					},
					Required: []string{"query", "strategyId"},
				},
			},
			Function:      strategy.CreateStrategyFromNaturalLanguage,
			StatusMessage: "Building strategy...",
		},
		"calculateBacktestStatistic": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "calculateBacktestStatistic",
				Description: "Calculate a statistic from cached backtest results and return the numeric value.",
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
			Function:      CalculateBacktestStatistic,
			StatusMessage: "Calculating backtest statistics...",
		},
		"runWebSearch": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "runWebSearch",
				Description: "Run a Google search and return a search result object.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query": {Type: genai.TypeString, Description: "The query to search for."},
					},
					Required: []string{"query"},
				},
			},
			Function:      RunWebSearch,
			StatusMessage: "Searching the web...",
		},
		"ui_open_watchlist": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "ui_open_watchlist",
				Description: "Open the watchlist sidebar to the given watchlistId. Returns 'ok'.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"watchlistId": {Type: genai.TypeInteger, Description: "The watchlist id to open."},
					},
					Required: []string{"watchlistId"},
				},
			},
			Function: OpenWatchlist,
		},
		"ui_open_alerts": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "ui_open_alerts",
				Description: "Open the alerts sidebar. Returns 'ok'.",
				Parameters:  &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{}, Required: []string{}},
			},
			Function: OpenAlerts,
		},
		"ui_open_news": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "ui_open_news",
				Description: "Open the news sidebar to an optional eventId. Returns 'ok'.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"eventId": {Type: genai.TypeInteger, Description: "Optional news event id."},
					},
				},
			},
			Function: OpenNews,
		},
		"ui_open_strategy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "ui_open_strategy",
				Description: "Open the strategy editor to a strategyId. Returns 'ok'.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {Type: genai.TypeInteger, Description: "The strategy id to open."},
					},
					Required: []string{"strategyId"},
				},
			},
			Function: OpenStrategy,
		},
		"ui_open_backtest": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "ui_open_backtest",
				Description: "Open the backtest window for a strategyId and run the test. Returns 'ok'.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {Type: genai.TypeInteger, Description: "The strategy id to backtest."},
					},
					Required: []string{"strategyId"},
				},
			},
			Function: OpenBacktest,
		},
		"ui_query_chart": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "ui_query_chart",
				Description: "Change the active chart using the provided parameters. Returns 'ok'.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker":     {Type: genai.TypeString, Description: "Ticker symbol to load."},
						"securityId": {Type: genai.TypeInteger, Description: "Security ID to load."},
						"timeframe":  {Type: genai.TypeString, Description: "Chart timeframe, e.g. '1d', '1h'."},
						"timestamp":  {Type: genai.TypeInteger, Description: "Starting timestamp in ms."},
					},
				},
			},
			Function: QueryChartUI,
        },
		"dateToMS": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "dateToMS",
				Description: "Convert a date to milliseconds since epoch.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"date":   {Type: genai.TypeString, Description: "The date in 2006-01-02 format to convert to milliseconds since epoch."},
						"hour":   {Type: genai.TypeInteger, Description: "The hour (24 hour format)on date."},
						"minute": {Type: genai.TypeInteger, Description: "The minute on date."},
						"second": {Type: genai.TypeInteger, Description: "The second on date."},
					},
					Required: []string{"date", "hour", "minute", "second"},
				},
			},
			Function:      DateToMS,
			StatusMessage: "Figuring out date range...",
		},
	}
)

// ToolReturnDesc provides a brief description of what each tool returns.
var ToolReturnDesc = map[string]string{
	"getCurrentSecurityID":           "integer securityId",
	"getSecuritiesFromTicker":        "list of ticker matches with securityId",
	"getCurrentTicker":               "string ticker symbol",
	"getTickerMenuDetails":           "object with company info",
	"getInstancesByTickers":          "list of securityIds",
	"getWatchlists":                  "list of watchlists",
	"deleteWatchlist":                "null on success",
	"newWatchlist":                   "new watchlistId",
	"getWatchlistItems":              "list of watchlist entries",
	"deleteWatchlistItem":            "null on success",
	"newWatchlistItem":               "new watchlistItemId",
	"getPrevClose":                   "float price",
	"getLastPrice":                   "float price",
	"setHorizontalLine":              "new lineId",
	"getHorizontalLines":             "list of horizontal lines",
	"deleteHorizontalLine":           "null on success",
	"updateHorizontalLine":           "null on success",
	"getStockEdgarFilings":           "list of filing summaries",
	"getChartEvents":                 "list of events",
	"getEarningsText":                "string text",
	"getFilingText":                  "string text",
	"getExhibitList":                 "list of exhibits",
	"getExhibitContent":              "string text",
	"grab_user_trades":               "list of trades",
	"get_trade_statistics":           "object with statistics",
	"get_ticker_performance":         "object with performance stats",
	"get_daily_trade_stats":          "object with daily stats",
	"run_backtest":                   "backtest summary object",
	"getTickerDailySnapshot":         "snapshot object",
	"getAllTickerSnapshots":          "list of snapshots",
	"getStrategies":                  "list of strategies",
	"deleteStrategy":                 "null on success",
	"getStrategyFromNaturalLanguage": "strategy info",
	"calculateBacktestStatistic":     "numeric result",
	"runWebSearch":                   "search result object",
	"ui_open_watchlist":              "ok",
	"ui_open_alerts":                 "ok",
	"ui_open_news":                   "ok",
	"ui_open_strategy":               "ok",
	"ui_open_backtest":               "ok",
	"ui_query_chart":                 "ok",
}
