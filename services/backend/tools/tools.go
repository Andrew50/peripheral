package tools

import (
	"backend/utils"
	"encoding/json"
	"sync"
	"google.golang.org/genai"
)

type Tool struct {
	FunctionDeclaration *genai.FunctionDeclaration
	Function            func(*utils.Conn, int, json.RawMessage) (interface{}, error)
	Query               bool
	Api                 bool
}

// Tools is a map of function names to Tool objects
var Tools = map[string]Tool{}
var toolsInitialized bool
var toolsMutex sync.Mutex

// GetTools returns a copy of the Tools map to avoid import cycles
func GetTools(api bool) map[string]Tool {
	toolsMutex.Lock()
	defer toolsMutex.Unlock()

	if !toolsInitialized {
		initTools()
		toolsInitialized = true
	}

	filteredTools := make(map[string]Tool)
	for name, tool := range Tools {
		if api && tool.Api {
			filteredTools[name] = tool
		} else if !api && tool.Query {
			filteredTools[name] = tool
		}
	}

	return filteredTools
}

// Initialize all tools
func initTools() {
	// Initialize the Tools map
	Tools = map[string]Tool{
		"getQuery": {
			FunctionDeclaration: nil,
            Function: GetQuery,
            Query: false,
            Api: true,
		},
		"getSimilarInstances": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSimilarInstances",
				Description: "Retrieves similar securities based on sector, industry, and market cap for a given security",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol of the security",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to find similar instances for",
						},
						"timestamp": {
							Type:        genai.TypeInteger,
							Description: "The timestamp (in milliseconds) to use as reference point",
						},
						"timeframe": {
							Type:        genai.TypeString,
							Description: "The timeframe to use for the chart data (e.g., '1d', '1h')",
						},
					},
					Required: []string{"ticker", "securityId", "timestamp", "timeframe"},
				},
			},
			Function: GetSimilarInstances,
            Query: true,
            Api: false,
		},
		"getCurrentSecurityID": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getCurrentSecurityID",
				Description: "Retrieves the current security ID of a ticker symbol.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol to search for, e.g. NVDA, AAPL, etc",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function: GetCurrentSecurityID,
            Query: true,
            Api: true,
		},
		"getSecuritiesFromTicker": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSecuritiesFromTicker",
				Description: "Retrieves securities information based on a ticker symbol.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol to search for",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function: GetSecuritiesFromTicker,
            Query: true,
            Api: true,
		},
		"getCurrentTicker": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getCurrentTicker",
				Description: "Gets the current ticker for a securityID",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The securityID of the security to get the current ticker for",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function: GetCurrentTicker,
            Query: true,
            Api: true,
		},
		"getTickerMenuDetails": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getTickerMenuDetails",
				Description: "Retrieves ticker menu information for a security; ticker, name, market, primary exchange, etc",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol to get details for",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function: GetTickerMenuDetails,
            Query: true,
            Api: true,
		},
		"getIcons": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getIcons",
				Description: "Retrieves icon URLs for securities",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"tickers": {
							Type:        genai.TypeArray,
							Description: "List of ticker symbols to get icons for",
							Items: &genai.Schema{
								Type: genai.TypeString,
							},
						},
					},
					Required: []string{"tickers"},
				},
			},
			Function: GetIcons,
            Query: true,
            Api: true,
		},

		//chart
		"getChartData": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getChartData",
				Description: "Retrieves price chart data for a security",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to get chart data for",
						},
						"timeframe": {
							Type:        genai.TypeString,
							Description: "The timeframe for the chart data (e.g., '1d', '1h')",
						},
						"from": {
							Type:        genai.TypeInteger,
							Description: "The start timestamp in milliseconds",
						},
						"to": {
							Type:        genai.TypeInteger,
							Description: "The end timestamp in milliseconds",
						},
					},
					Required: []string{"securityId", "timeframe", "from", "to"},
				},
			},
			Function: GetChartData,
            Query: true,
            Api: true,
		},
		//study
		"getStudies": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStudies",
				Description: "Retrieves all study entries for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetStudies,
            Query: false,
            Api: true,
		},

		"newStudy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "newStudy",
				Description: "Creates a new study entry for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: NewStudy,
            Query: false,
            Api: true,
		},
		"saveStudy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "saveStudy",
				Description: "Saves content for an existing study entry",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: SaveStudy,
            Query: false,
            Api: true,
		},
		"deleteStudy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteStudy",
				Description: "Deletes a study entry for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: DeleteStudy,
            Query: false,
            Api: true,
		},
		"getStudyEntry": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStudyEntry",
				Description: "Retrieves the content of a specific study entry",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetStudyEntry,
            Query: false,
            Api: true,
		},
		"completeStudy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "completeStudy",
				Description: "Marks a study entry as completed or not completed",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: CompleteStudy,
            Query: false,
            Api: true,
		},
		"setStudyStrategy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "setStudyStrategy",
				Description: "Associates a strategy configuration with a study entry",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: SetStudyStrategy,
            Query: false,
            Api: true,
		},
			//screensaver
		"getScreensavers": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getScreensavers",
				Description: "Retrieves a list of trending securities for the screensaver display",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetScreensavers,
            Query: false,
            Api: true,
		},
		"getInstancesByTickers": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getInstancesByTickers",
				Description: "Retrieves security instances for a list of ticker symbols",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"tickers": {
							Type:        genai.TypeArray,
							Description: "Array of ticker symbols to retrieve",
							Items: &genai.Schema{
								Type: genai.TypeString,
							},
						},
					},
					Required: []string{"tickers"},
				},
			},
			Function: GetInstancesByTickers,
            Query: true,
            Api: true,
		},
		//watchlist
		"getWatchlists": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getWatchlists",
				Description: "Retrieves all watchlists for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetWatchlists,
            Query: false,
            Api: true,
		},
		"deleteWatchlist": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteWatchlist",
				Description: "Deletes a watchlist for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"watchlistId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the watchlist to delete",
						},
					},
					Required: []string{"watchlistId"},
				},
			},
			Function: DeleteWatchlist,
            Query: true,
            Api: true,
		},
		"newWatchlist": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "newWatchlist",
				Description: "Creates a new watchlist for the current user",
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
			Function: NewWatchlist,
            Query: true,
            Api: true,
		},
		"getWatchlistItems": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getWatchlistItems",
				Description: "Retrieves the securityID's of the securities in a specific watchlist",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"watchlistId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the watchlist to get items for",
						},
					},
					Required: []string{"watchlistId"},
				},
			},
			Function: GetWatchlistItems,
            Query: true,
            Api: true,
		},
		"deleteWatchlistItem": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteWatchlistItem",
				Description: "Removes a security from a watchlist",
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
			Function: DeleteWatchlistItem,
            Query: true,
            Api: true,
		},
		"newWatchlistItem": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "newWatchlistItem",
				Description: "Adds a security to a watchlist",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"watchlistId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the watchlist to add the security to",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to add to the watchlist",
						},
					},
					Required: []string{"watchlistId", "securityId"},
				},
			},
			Function: NewWatchlistItem,
            Query: true,
            Api: true,
		},
		//singles
		"getPrevClose": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getPrevClose",
				Description: "Retrieves the previous closing price for a security",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol to get the previous close for",
						},
					},
					Required: []string{"ticker"},
				},
			},
			Function: GetPrevClose,
            Query: true,
            Api: true,
		},
		//"getMarketCap": GetMarketCap,
		//settings
		"getSettings": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSettings",
				Description: "Retrieves the user settings for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetSettings,
            Query: false,
            Api: true,
		},
		"setSettings": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "setSettings",
				Description: "Updates the user settings for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"settings": {
							Type:        genai.TypeObject,
							Description: "The settings data in JSON format",
							Properties: map[string]*genai.Schema{
								"theme": {
									Type:        genai.TypeString,
									Description: "Theme preference (e.g., 'light', 'dark')",
								},
								"notifications": {
									Type:        genai.TypeBoolean,
									Description: "Whether notifications are enabled",
								},
							},
						},
					},
					Required: []string{"settings"},
				},
			},
			Function: SetSettings,
            Query: true,
            Api: true,
		},
		//profile
		"updateProfilePicture": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "updateProfilePicture",
				Description: "Updates the profile picture for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"imageData": {
							Type:        genai.TypeString,
							Description: "The base64-encoded image data for the profile picture",
						},
					},
					Required: []string{"imageData"},
				},
			},
			Function: UpdateProfilePicture,
            Query: true,
            Api: true,
		},
		//exchanges
		"getExchanges": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getExchanges",
				Description: "Retrieves a list of stock exchanges and their MIC codes from Polygon.io",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetExchanges,
            Query: false,
            Api: true,
		},
		//stratagies
		"getStrategies": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStrategies",
				Description: "Retrieves all strategy configurations for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetStrategies,
            Query: false,
            Api: true,
		},
		"newStrategy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "newStrategy",
				Description: "Creates a new strategy configuration for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"name": {
							Type:        genai.TypeString,
							Description: "The name of the strategy",
						},
						"timeframe": {
							Type:        genai.TypeString,
							Description: "The timeframe for the strategy (e.g., '1d', '1h')",
						},
						"bars": {
							Type:        genai.TypeInteger,
							Description: "The number of bars to consider for the strategy",
						},
						"threshold": {
							Type:        genai.TypeInteger,
							Description: "The threshold value for the strategy",
						},
						"dolvol": {
							Type:        genai.TypeNumber,
							Description: "The dollar volume filter for the strategy",
						},
						"adr": {
							Type:        genai.TypeNumber,
							Description: "The Average Daily Range filter for the strategy",
						},
						"mcap": {
							Type:        genai.TypeNumber,
							Description: "The market capitalization filter for the strategy",
						},
					},
					Required: []string{"name", "timeframe"},
				},
			},
			Function: NewStrategy,
            Query: false,
            Api: true,
		},
		"setStrategy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "setStrategy",
				Description: "Updates an existing strategy configuration for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the strategy to update",
						},
						"name": {
							Type:        genai.TypeString,
							Description: "The name of the strategy",
						},
						"timeframe": {
							Type:        genai.TypeString,
							Description: "The timeframe for the strategy (e.g., '1d', '1h')",
						},
						"bars": {
							Type:        genai.TypeInteger,
							Description: "The number of bars to consider for the strategy",
						},
						"threshold": {
							Type:        genai.TypeInteger,
							Description: "The threshold value for the strategy",
						},
						"dolvol": {
							Type:        genai.TypeNumber,
							Description: "The dollar volume filter for the strategy",
						},
						"adr": {
							Type:        genai.TypeNumber,
							Description: "The Average Daily Range filter for the strategy",
						},
						"mcap": {
							Type:        genai.TypeNumber,
							Description: "The market capitalization filter for the strategy",
						},
					},
					Required: []string{"strategyId", "name", "timeframe"},
				},
			},
			Function: SetStrategy,
            Query: true,
            Api: true,
		},
		"deleteStrategy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteStrategy",
				Description: "Deletes a strategy configuration for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the strategy to delete",
						},
					},
					Required: []string{"strategyId"},
				},
			},
			Function: DeleteStrategy,
            Query: true,
            Api: true,
		},
		//algos
		//"getAlgos": GetAlgos,
		//samples
		"getAlerts": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getAlerts",
				Description: "Retrieves all alerts for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetAlerts,
            Query: false,
            Api: true,
		},
		"getAlertLogs": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getAlertLogs",
				Description: "Retrieves the history of triggered alerts for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetAlertLogs,
            Query: false,
            Api: true,
		},
		"newAlert": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "newAlert",
				Description: "Creates a new alert for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"alertType": {
							Type:        genai.TypeString,
							Description: "The type of alert (price, strategy, or algo)",
						},
						"price": {
							Type:        genai.TypeNumber,
							Description: "The price threshold for a price alert (required for price alerts)",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security for a price alert (required for price alerts)",
						},
						"strategyId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the strategy for a strategy alert (required for strategy alerts)",
						},
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol for a price alert",
						},
						"algoId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the algorithm for an algo alert (required for algo alerts)",
						},
					},
					Required: []string{"alertType"},
				},
			},
			Function: NewAlert,
            Query: true,
            Api: true,
		},
		"deleteAlert": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteAlert",
				Description: "Deletes an alert for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"alertId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the alert to delete",
						},
					},
					Required: []string{"alertId"},
				},
			},
			Function: DeleteAlert,
            Query: true,
            Api: true,
		},
		"setHorizontalLine": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "setHorizontalLine",
				Description: "Creates a new horizontal line on a chart",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to add the horizontal line to",
						},
						"price": {
							Type:        genai.TypeNumber,
							Description: "The price level for the horizontal line",
						},
						"color": {
							Type:        genai.TypeString,
							Description: "The color of the horizontal line (hex format, defaults to #FFFFFF)",
						},
						"lineWidth": {
							Type:        genai.TypeInteger,
							Description: "The width of the horizontal line in pixels (defaults to 1)",
						},
					},
					Required: []string{"securityId", "price"},
				},
			},
			Function: SetHorizontalLine,
            Query: true,
            Api: true,
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
			Function: GetHorizontalLines,
            Query: true,
            Api: true,
		},
		"deleteHorizontalLine": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "deleteHorizontalLine",
				Description: "Deletes a horizontal line from a chart",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id": {
							Type:        genai.TypeInteger,
							Description: "The ID of the horizontal line to delete",
						},
					},
					Required: []string{"id"},
				},
			},
			Function: DeleteHorizontalLine,
            Query: true,
            Api: true,
		},
		"updateHorizontalLine": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "updateHorizontalLine",
				Description: "Updates an existing horizontal line on a chart",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id": {
							Type:        genai.TypeInteger,
							Description: "The ID of the horizontal line to update",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security the horizontal line belongs to",
						},
						"price": {
							Type:        genai.TypeNumber,
							Description: "The new price level for the horizontal line",
						},
						"color": {
							Type:        genai.TypeString,
							Description: "The new color of the horizontal line (hex format)",
						},
						"lineWidth": {
							Type:        genai.TypeInteger,
							Description: "The new width of the horizontal line in pixels",
						},
					},
					Required: []string{"id", "securityId", "price"},
				},
			},
			Function: UpdateHorizontalLine,
            Query: true,
            Api: true,
		},
		"verifyAuth": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "verifyAuth",
				Description: "Verifies the authentication status of the user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
                    Required: []string{}, // Added Required field
				},
			},
			Function: func(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) { return nil, nil },
            Query: false,
            Api: true,
		},
		"getSecurityClassifications": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSecurityClassifications",
				Description: "Retrieves sector and industry classifications for securities",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetSecurityClassifications,
            Query: false,
            Api: true,
		},
		// chart events / SEC filings
		"getLatestEdgarFilings": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getLatestEdgarFilings",
				Description: "Retrieves the latest SEC EDGAR filings across all securities",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter; no parameter needed.",
						},
					},
					Required: []string{},
				},
			},
			Function: GetLatestEdgarFilings,
            Query: false,
            Api: true,
		},
		"getStockEdgarFilings": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStockEdgarFilings",
				Description: "Retrieves all SEC filings for a security within a time range. Returns a list of the filing type and URLs to the filings.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"start": {
							Type:        genai.TypeInteger,
							Description: "The start timestamp in milliseconds",
						},
						"end": {
							Type:        genai.TypeInteger,
							Description: "The end timestamp in milliseconds",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to get filings for",
						},
					},
					Required: []string{"start", "end", "securityId"},
				},
			},
			Function: GetStockEdgarFilings,
            Query: true,
            Api: true,
		},
		"getChartEvents": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getChartEvents",
				Description: "Retrieves events (splits, dividends, SEC filings) for a security within a time range",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to get events for",
						},
						"from": {
							Type:        genai.TypeInteger,
							Description: "The start timestamp in milliseconds",
						},
						"to": {
							Type:        genai.TypeInteger,
							Description: "The end timestamp in milliseconds",
						},
						"includeSECFilings": {
							Type:        genai.TypeBoolean,
							Description: "Whether to include SEC filings in the result",
						},
					},
					Required: []string{"securityId", "from", "to"},
				},
			},
			Function: GetChartEvents,
            Query: true,
            Api: true,
		},
		"getEarningsText": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getEarningsText",
				Description: "Retrieves the text content of the latest 10-K or 10-Q SEC filing for a security",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to get the filing for",
						},
						"quarter": {
							Type:        genai.TypeString,
							Description: "Optional: The specific quarter to retrieve (Q1, Q2, Q3, Q4). If not specified, returns the latest filing.",
						},
						"year": {
							Type:        genai.TypeInteger,
							Description: "Optional: The specific year to retrieve the filing from. Used in conjunction with quarter parameter.",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function: GetEarningsText,
            Query: true,
            Api: true,
		},
		"getFilingText": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getFilingText",
				Description: "Retrieves the text content of a SEC filing",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"url": {
							Type:        genai.TypeString,
							Description: "The URL of the SEC filing to retrieve",
						},
					},
					Required: []string{"url"},
				},
			},
			Function: GetFilingText,
            Query: true,
            Api: true,
		},
		// Account / User Trades
		"grab_user_trades": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "grab_user_trades",
				Description: "Retrieves all trades for the current user with optional filtering",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "Optional ticker symbol to filter trades by",
						},
						"startDate": {
							Type:        genai.TypeString,
							Description: "Optional start date to filter trades (format: YYYY-MM-DD)",
						},
						"endDate": {
							Type:        genai.TypeString,
							Description: "Optional end date to filter trades (format: YYYY-MM-DD)",
						},
					},
					Required: []string{},
				},
			},
			Function: GrabUserTrades,
            Query: true,
            Api: true,
		},
		"get_trade_statistics": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "get_trade_statistics",
				Description: "Retrieves trading performance statistics for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"startDate": {
							Type:        genai.TypeString,
							Description: "Optional start date to filter statistics (format: YYYY-MM-DD)",
						},
						"endDate": {
							Type:        genai.TypeString,
							Description: "Optional end date to filter statistics (format: YYYY-MM-DD)",
						},
						"ticker": {
							Type:        genai.TypeString,
							Description: "Optional ticker symbol to filter statistics by",
						},
					},
					Required: []string{},
				},
			},
			Function: GetTradeStatistics,
            Query: true,
            Api: true,
		},
		"get_ticker_performance": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "get_ticker_performance",
				Description: "Retrieves detailed performance statistics for a specific ticker",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type:        genai.TypeString,
							Description: "The ticker symbol to get performance statistics for",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security to get performance statistics for",
						},
					},
					Required: []string{"ticker", "securityId"},
				},
			},
			Function: GetTickerPerformance,
            Query: true,
            Api: true,
		},
		"delete_all_user_trades": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "delete_all_user_trades",
				Description: "Deletes all trade records for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: DeleteAllUserTrades,
            Query: false,
            Api: true,
		},
		"handle_trade_upload": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "handle_trade_upload",
				Description: "Processes and imports trade data from a CSV file",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"csvData": {
							Type:        genai.TypeString,
							Description: "The CSV trade data to process and import",
						},
						"broker": {
							Type:        genai.TypeString,
							Description: "The broker name for the trade data (e.g., 'interactive_brokers', 'td_ameritrade')",
						},
					},
					Required: []string{"csvData", "broker"},
				},
			},
			Function: HandleTradeUpload,
            Query: true,
            Api: true,
		},
		"get_daily_trade_stats": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "get_daily_trade_stats",
				Description: "Retrieves daily trading statistics for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"year": {
							Type:        genai.TypeInteger,
							Description: "The year to get daily stats for",
						},
						"month": {
							Type:        genai.TypeInteger,
							Description: "The month to get daily stats for",
						},
					},
					Required: []string{"year", "month"},
				},
			},
			Function: GetDailyTradeStats,
            Query: true,
            Api: true,
		},
		"run_backtest": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "run_backtest",
				Description: "Backtest a specified strategy, which is based on stock conditions, patterns, and indicators.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"strategyId": {
							Type:        genai.TypeInteger,
							Description: "id of the strategy to backtest",
						},
					},
					Required: []string{"strategyId"},
				},
			},
			Function: RunBacktest,
            Query: true,
            Api: true,
		},
		
        "getStrategyFromNaturalLanguage" : {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getStrategyFromNaturalLanguage",
				Description: "Create a strategy and save it to the specified id (-1 to create new and the funciton will return the new id) based on a natural language query about stock conditions, patterns, and indicators primarily used for running a backtest on. This function does not run the backtest itself, that is the run_backtest function. IF YOU CALL THIS TOOL, USE THE USER'S ORIGINAL QUERY. DO NOT GENERATE A NEW QUERY.",

				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query": {
							Type:        genai.TypeString,
							Description: "Natural language query describing the strategy criteria  IF YOU CALL THIS TOOL, USE THE USER'S ORIGINAL QUERY. DO NOT GENERATE A NEW QUERY.",
						},
						"strategyId": {
							Type:        genai.TypeInteger,
							Description: "id of the strategy to overwrite, -1 means create new",
						},
					},
					Required: []string{"query","strategyId"},
				},
			},
			Function: CreateStrategyFromNaturalLanguage,
            Query: true,
            Api: true,
		},
		"getUserConversation": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getUserConversation",
				Description: "Retrieves the conversation history for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy Gemini API requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetUserConversation,
            Query: false,
            Api: true,
		},
		"clearConversationHistory": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "clearConversationHistory",
				Description: "Deletes the entire conversation history for the current user.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"dummy": {
							Type:        genai.TypeString,
							Description: "Dummy parameter, no input needed.",
						},
					},
					Required: []string{},
				},
			},
			Function: ClearConversationHistory,
            Query: false,
            Api: true,
		},
		"getTickerDailySnapshot": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getTickerDailySnapshot",
				Description: "Retrieves the most recent daily data for a ticker symbol, today's change (absolute and percentage), volume, vwap, OHLC, last price, last bid/ask, etc.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The securityID of the ticker to get daily snapshot data for",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function: GetTickerDailySnapshot,
            Query: true,
            Api: false,
		},
		"getAllTickerSnapshots": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getAllTickerSnapshots",
				Description: "Retrieves the most recent daily data for all stocks, today's change (absolute and percentage), volume, vwap, OHLC, last price, last bid/ask, etc",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"na": {
							Type:        genai.TypeString,
							Description: "No params needed",
						},
					},
					Required: []string{},
				},
			},
			Function: GetAllTickerSnapshots,
            Query: true,
            Api: false,
		},
		"getSuggestedQueries": {
			FunctionDeclaration: nil,
            Function: GetSuggestedQueries,
            Query: false,
            Api: true,
		},
	}
}
