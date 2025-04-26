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

/*
///////

prompt guidlines

#use following terminology:#

security ID - security id
security ticker symbol - ticker


///////
*/

// Initialize all tools
func initTools() {
	// Initialize the Tools map
	Tools = map[string]Tool{
		"getQuery": {
			FunctionDeclaration: nil,
			Function:            GetQuery,
			Query:               false,
			Api:                 true,
		},
		"getSimilarInstances": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSimilarInstances",
				Description: "Get a list of securities related to a specified security at the current time.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The security ID of the security",
						},
					},
					Required: []string{"securityId"},
				},
			},
			Function: GetSimilarInstances,
			Query:    true,
			Api:      true,
		},
		"getCurrentSecurityID": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getCurrentSecurityID",
				Description: "Get the current security ID from a security ticker symbol.",
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
			Function: GetCurrentSecurityID,
			Query:    true,
			Api:      true,
		},
		"getSecurityIDFromTickerTimestamp": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSecurityIDFromTickerTimestamp",
				Description: "Get the security ID for a given ticker and timestamp.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"ticker": {
							Type: genai.TypeString,
						},
						"timestamp": {
							Type: genai.TypeInteger,
						},
					},
					Required: []string{"ticker", "timestamp"},
				},
			},
			Function: GetSecurityIDFromTickerTimestamp,
			Query:    false,
			Api:      true,
		},

		//TODO remove icon for query
		"getSecuritiesFromTicker": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getSecuritiesFromTicker",
				Description: "Get a list of the closest 10 securitie ticker symbols to an input string.",
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
			Function: GetSecuritiesFromTicker,
			Query:    true,
			Api:      true,
		},
		"getCurrentTicker": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getCurrentTicker",
				Description: "Get the current security ticker symbol for a security ID.",
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
			Function: GetCurrentTicker,
			Query:    true,
			Api:      true,
		},
		//TODO remove logo
		"getTickerMenuDetails": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getTickerMenuDetails",
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
					Required: []string{"ticker", "securityId"},
				},
			},
			Function: GetTickerMenuDetails,
			Query:    true,
			Api:      true,
		},
		"getIcons": {
			FunctionDeclaration: nil,
			Function:            GetIcons,
			Query:               false,
			Api:                 true,
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
			Query:    false,
			Api:      true,
		},
		//study
		//
		"getStudies": {
			FunctionDeclaration: nil, /* &genai.FunctionDeclaration{
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
			},*/
			Function: GetStudies,
			Query:    false,
			Api:      true,
		},

		"newStudy": {
			FunctionDeclaration: nil, /*&genai.FunctionDeclaration{
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
			},*/
			Function: NewStudy,
			Query:    false,
			Api:      true,
		},
		"saveStudy": {
			FunctionDeclaration: nil, /*&genai.FunctionDeclaration{
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
			},*/
			Function: SaveStudy,
			Query:    false,
			Api:      true,
		},
		"deleteStudy": {
			FunctionDeclaration: nil, /*&genai.FunctionDeclaration{
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
			},*/
			Function: DeleteStudy,
			Query:    false,
			Api:      true,
		},
		"getStudyEntry": {
			FunctionDeclaration: nil, /* &genai.FunctionDeclaration{
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
			},*/
			Function: GetStudyEntry,
			Query:    false,
			Api:      true,
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
			Query:    false,
			Api:      true,
		},
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
			Query:    false,
			Api:      true,
		},
		"getInstancesByTickers": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getInstancesByTickers",
				Description: "Get security IDs for a list of security ticker symbols.",
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
			Function: GetInstancesByTickers,
			Query:    true,
			Api:      true,
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
			Function: GetWatchlists,
			Query:    true,
			Api:      true,
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
			Function: DeleteWatchlist,
			Query:    true,
			Api:      true,
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
			Function: NewWatchlist,
			Query:    true,
			Api:      true,
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
			Function: GetWatchlistItems,
			Query:    true,
			Api:      true,
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
			Function: DeleteWatchlistItem,
			Query:    true,
			Api:      true,
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
			Function: NewWatchlistItem,
			Query:    true,
			Api:      true,
		},
		//singles
		"getPrevClose": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getPrevClose",
				Description: "Retrieves the previous closing price for a specified security ticker symbol. This also gets the most recent price if the market is closed or in after hours.",
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
			Function: GetPrevClose,
			Query:    true,
			Api:      true,
		},
		"getLastPrice": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getLastPrice",
				Description: "Retrieves the last price for a specified security ticker symbol.",
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
			Function: GetLastPrice,
			Query:    true,
			Api:      false,
		},
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
			Query:    false,
			Api:      true,
		},
		//TODO?
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
			Query:    false,
			Api:      true,
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
			Query:    false,
			Api:      true,
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
			Query:    false,
			Api:      true,
		},
		//setups
		//TODO
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
			Query:    false,
			Api:      true,
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
			Query:    false,
			Api:      true,
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
							Description: "The type of alert (price, setup, or algo)",
						},
						"price": {
							Type:        genai.TypeNumber,
							Description: "The price threshold for a price alert (required for price alerts)",
						},
						"securityId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the security for a price alert (required for price alerts)",
						},
						"setupId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the setup for a setup alert (required for setup alerts)",
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
			Query:    false,
			Api:      true,
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
			Query:    false,
			Api:      true,
		},
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
			Function: SetHorizontalLine,
			Query:    true,
			Api:      true,
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
			Query:    true,
			Api:      true,
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
			Function: DeleteHorizontalLine,
			Query:    true,
			Api:      true,
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
			Function: UpdateHorizontalLine,
			Query:    true,
			Api:      true,
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
			Query:    false,
			Api:      true,
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
			Query:    false,
			Api:      true,
		},
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
			Function: GetStockEdgarFilings,
			Query:    true,
			Api:      true,
		},
		"getChartEvents": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getChartEvents",
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
			Function: GetChartEvents,
			Query:    true,
			Api:      true,
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
			Function: GetEarningsText,
			Query:    true,
			Api:      true,
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
			Function: GetFilingText,
			Query:    true,
			Api:      true,
		},
		// Account / User Trades
		"grab_user_trades": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "grab_user_trades",
				Description: "Get user trades with optional filtering.",
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
			Function: GrabUserTrades,
			Query:    true,
			Api:      true,
		},
		"get_trade_statistics": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "get_trade_statistics",
				Description: "Get user trading performance statistics.",
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
			Function: GetTradeStatistics,
			Query:    true,
			Api:      true,
		},
		"get_ticker_performance": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "get_ticker_performance",
				Description: "Retrieves the user's trade performance statistics for a specific ticker (p/l, win rate, average gain/loss, etc)",
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
			Function: GetTickerPerformance,
			Query:    true,
			Api:      true,
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
			Query:    false,
			Api:      true,
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
			Query:    false,
			Api:      true,
		},
		"get_daily_trade_stats": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "get_daily_trade_stats",
				Description: "Retrieves user trading statistics for a specified year and month.",
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
			Function: GetDailyTradeStats,
			Query:    true,
			Api:      true,
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
			Query:    true,
			Api:      true,
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
			Query:    false,
			Api:      true,
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
			Query:    false,
			Api:      true,
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
			Query:    false,
			Api:      true,
		},
		"getTickerDailySnapshot": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getTickerDailySnapshot",
				Description: "Get the current price, change, percent change, volume, vwap price, and open, high, low and close for a specified security.",
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
			Function: GetTickerDailySnapshot,
			Query:    true,
			Api:      false,
		},
		"getAllTickerSnapshots": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "getAllTickerSnapshots",
				Description: "Get a list of the current bid, ask, price, change, percent change, volume, vwap price, and daily open, high, low and close for all securities.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{},
					Required:   []string{},
				},
			},
			Function: GetAllTickerSnapshots,
			Query:    true,
			Api:      false,
		},
		"getSuggestedQueries": {
			FunctionDeclaration: nil,
			Function:            GetSuggestedQueries,
			Query:               false,
			Api:                 true,
		},
		"setStudyStrategy": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name:        "setStudyStrategy",
				Description: "Associates a strategy configuration with a study entry",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{},
					Required:   []string{},
				},
			},
			Function: SetStudyStrategy,
			Query:    false,
			Api:      true,
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
			Function: GetStrategies,
			Query:    true,
			Api:      true,
		},
		"newStrategy": {
			FunctionDeclaration:nil,
            Function: NewStrategy,
			Query:    false,
			Api:      true,
		},
		"setStrategy": {
			FunctionDeclaration: nil,
			Function: SetStrategy,
			Query:    false,
			Api:      true,
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
			Function: DeleteStrategy,
			Query:    true,
			Api:      true,
		},
		"getStrategyFromNaturalLanguage": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name: "getStrategyFromNaturalLanguage",
				Description: "Create (or overwrite) a strategy from a natural‑language description. " +
					"IF YOU USE THIS FUNCTION, USE THE USER'S ORIGINAL QUERY AS IS. Pass strategyId = -1 to create a new strategy. This function will return the strategyId of the created strategy.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query":      {Type: genai.TypeString, Description: "Original NL query"},
						"strategyId": {Type: genai.TypeInteger, Description: "-1 for new strategy, else overwrite"},
					},
					Required: []string{"query", "strategyId"},
				},
			},
			Function: CreateStrategyFromNaturalLanguage,
			Query:    true,
			Api:      true,
		},
		"analyzeInstanceFeatures": {
			FunctionDeclaration: &genai.FunctionDeclaration{
				Name: "analyzeInstanceFeatures",
				Description: `Analyze recent price‑action around a specific market *instance* ` +
					`and return technical context (trend, volatility, indicators, S/R, …). ` +
					`Call this **before** getStrategyFromNaturalLanguage whenever the user ` +
					`has attached an explicit instance, especially if they provided little or no NL description.

					An "instance" is uniquely identified by:
					• securityId - numeric DB identifier  
					• timestamp - Unix epoch *seconds* marking the "current bar"  
					• timeframe - candle resolution (e.g. "15m", "2h", "1d")  
					• bars - number of historical candles to analyse (looking **backwards**)`,
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"securityId": {Type: genai.TypeInteger, Description: "DB security ID"},
						"timestamp":  {Type: genai.TypeInteger, Description: "Reference Unix timestamp (s)"},
						"timeframe":  {Type: genai.TypeString, Description: "Candle interval (e.g. \"1d\")"},
						"bars":       {Type: genai.TypeInteger, Description: "Candles to analyse"},
					},
					Required: []string{"securityId", "timestamp", "timeframe", "bars"},
				},
			},
			Function: AnalyzeInstanceFeatures,
			Query:    true,
			Api:      false,
		},
        "getStrategySpec": {
            FunctionDeclaration: nil,
            Function: GetStrategySpec,
            Query: false, //prolly should allow this
            Api: true,
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
			Function: CalculateBacktestStatistic,
			Query:    true,
			Api:      false,
		},
	}

}
