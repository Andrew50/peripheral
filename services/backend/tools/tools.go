package tools

import (
	"backend/utils"
	"encoding/json"
	"sync"

	"google.golang.org/genai"
)

type Tool struct {
	genai.FunctionDeclaration
	Function func(*utils.Conn, int, json.RawMessage) (interface{}, error)
}

// Tools is a map of function names to Tool objects
var Tools = map[string]Tool{}
var toolsInitialized bool
var toolsMutex sync.Mutex

// GetTools returns a copy of the Tools map to avoid import cycles
func GetTools() map[string]Tool {
	toolsMutex.Lock()
	defer toolsMutex.Unlock()

	if !toolsInitialized {
		initTools()
		toolsInitialized = true
	}

	return Tools
}

// Initialize all tools
func initTools() {
	// Initialize the Tools map
	Tools = map[string]Tool{
		"getQuery": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "getQuery",
				Description: "n/a",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query": {
							Type:        genai.TypeString,
							Description: "n/a",
						},
					},
					Required: []string{"query"},
				},
			},
			Function: GetQuery,
		},
		"getSimilarInstances": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getCurrentSecurityID": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getSecuritiesFromTicker": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getCurrentTicker": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "getCurrentTicker",
				Description: "Gets the current ticker fo a securityID",
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
		},
		"getTickerMenuDetails": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getIcons": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},

		//chart
		"getChartData": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		//study
		"getStudies": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},

		"newStudy": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"saveStudy": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"deleteStudy": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getStudyEntry": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"completeStudy": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"setStudySetup": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "setStudySetup",
				Description: "Associates a setup configuration with a study entry",
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
			Function: SetStudySetup,
		},
		//journal
		"getJournals": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "getJournals",
				Description: "Retrieves all journal entries for the current user",
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
			Function: GetJournals,
		},
		"saveJournal": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "saveJournal",
				Description: "Saves content for an existing journal entry",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id": {
							Type:        genai.TypeInteger,
							Description: "The ID of the journal entry to update",
						},
						"entry": {
							Type:        genai.TypeObject,
							Description: "The content of the journal entry in JSON format",
							Properties: map[string]*genai.Schema{
								"title": {
									Type:        genai.TypeString,
									Description: "The title of the journal entry",
								},
								"content": {
									Type:        genai.TypeString,
									Description: "The content of the journal entry",
								},
								"date": {
									Type:        genai.TypeString,
									Description: "The date of the journal entry",
								},
							},
						},
					},
					Required: []string{"id", "entry"},
				},
			},
			Function: SaveJournal,
		},
		"deleteJournal": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "deleteJournal",
				Description: "Deletes a journal entry for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id": {
							Type:        genai.TypeInteger,
							Description: "The ID of the journal entry to delete",
						},
					},
					Required: []string{"id"},
				},
			},
			Function: DeleteJournal,
		},
		"getJournalEntry": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "getJournalEntry",
				Description: "Retrieves the content of a specific journal entry",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"journalId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the journal entry to retrieve",
						},
					},
					Required: []string{"journalId"},
				},
			},
			Function: GetJournalEntry,
		},
		"completeJournal": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "completeJournal",
				Description: "Marks a journal entry as completed or not completed",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id": {
							Type:        genai.TypeInteger,
							Description: "The ID of the journal entry to update",
						},
						"completed": {
							Type:        genai.TypeBoolean,
							Description: "Whether the journal entry is completed",
						},
					},
					Required: []string{"id", "completed"},
				},
			},
			Function: CompleteJournal,
		},
		//screensaver
		"getScreensavers": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		//watchlist
		"getWatchlists": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"deleteWatchlist": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"newWatchlist": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getWatchlistItems": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"deleteWatchlistItem": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"newWatchlistItem": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		//singles
		"getPrevClose": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		//"getMarketCap": GetMarketCap,
		//settings
		"getSettings": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"setSettings": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		//profile
		"updateProfilePicture": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		//exchanges
		"getExchanges": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		//setups
		"getSetups": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "getSetups",
				Description: "Retrieves all setup configurations for the current user",
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
			Function: GetSetups,
		},
		"newSetup": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "newSetup",
				Description: "Creates a new setup configuration for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"name": {
							Type:        genai.TypeString,
							Description: "The name of the setup",
						},
						"timeframe": {
							Type:        genai.TypeString,
							Description: "The timeframe for the setup (e.g., '1d', '1h')",
						},
						"bars": {
							Type:        genai.TypeInteger,
							Description: "The number of bars to consider for the setup",
						},
						"threshold": {
							Type:        genai.TypeInteger,
							Description: "The threshold value for the setup",
						},
						"dolvol": {
							Type:        genai.TypeNumber,
							Description: "The dollar volume filter for the setup",
						},
						"adr": {
							Type:        genai.TypeNumber,
							Description: "The Average Daily Range filter for the setup",
						},
						"mcap": {
							Type:        genai.TypeNumber,
							Description: "The market capitalization filter for the setup",
						},
					},
					Required: []string{"name", "timeframe"},
				},
			},
			Function: NewSetup,
		},
		"setSetup": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "setSetup",
				Description: "Updates an existing setup configuration for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"setupId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the setup to update",
						},
						"name": {
							Type:        genai.TypeString,
							Description: "The name of the setup",
						},
						"timeframe": {
							Type:        genai.TypeString,
							Description: "The timeframe for the setup (e.g., '1d', '1h')",
						},
						"bars": {
							Type:        genai.TypeInteger,
							Description: "The number of bars to consider for the setup",
						},
						"threshold": {
							Type:        genai.TypeInteger,
							Description: "The threshold value for the setup",
						},
						"dolvol": {
							Type:        genai.TypeNumber,
							Description: "The dollar volume filter for the setup",
						},
						"adr": {
							Type:        genai.TypeNumber,
							Description: "The Average Daily Range filter for the setup",
						},
						"mcap": {
							Type:        genai.TypeNumber,
							Description: "The market capitalization filter for the setup",
						},
					},
					Required: []string{"setupId", "name", "timeframe"},
				},
			},
			Function: SetSetup,
		},
		"deleteSetup": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "deleteSetup",
				Description: "Deletes a setup configuration for the current user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"setupId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the setup to delete",
						},
					},
					Required: []string{"setupId"},
				},
			},
			Function: DeleteSetup,
		},
		//algos
		//"getAlgos": GetAlgos,
		//samples
		"labelTrainingQueueInstance": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "labelTrainingQueueInstance",
				Description: "Labels a training instance in the queue",
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
			Function: LabelTrainingQueueInstance,
		},
		"getTrainingQueue": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "getTrainingQueue",
				Description: "Retrieves the current training queue instances",
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
			Function: GetTrainingQueue,
		},
		"setSample": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "setSample",
				Description: "Sets a sample for training purposes",
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
			Function: SetSample,
		},
		"getAlerts": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getAlertLogs": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"newAlert": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"deleteAlert": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"setHorizontalLine": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getHorizontalLines": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"deleteHorizontalLine": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"updateHorizontalLine": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getActive": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "getActive",
				Description: "Retrieves a list of active securities",
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
			Function: GetActive,
		},
		"getSecurityClassifications": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		// chart events / SEC filings
		"getLatestEdgarFilings": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getStockEdgarFilings": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getChartEvents": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getEarningsText": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getFilingText": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		// Account / User Trades
		"grab_user_trades": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"get_trade_statistics": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"get_ticker_performance": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"delete_all_user_trades": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"handle_trade_upload": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		// Notes
		"get_notes": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "get_notes",
				Description: "Retrieves notes for the current user with optional filtering",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"category": {
							Type:        genai.TypeString,
							Description: "Optional category to filter notes by",
						},
						"tags": {
							Type:        genai.TypeArray,
							Description: "Optional array of tags to filter notes by",
							Items: &genai.Schema{
								Type: genai.TypeString,
							},
						},
						"isPinned": {
							Type:        genai.TypeBoolean,
							Description: "Optional filter for pinned notes",
						},
						"isArchived": {
							Type:        genai.TypeBoolean,
							Description: "Optional filter for archived notes",
						},
						"searchQuery": {
							Type:        genai.TypeString,
							Description: "Optional text search query to filter notes by content",
						},
					},
					Required: []string{},
				},
			},
			Function: GetNotes,
		},
		"search_notes": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "search_notes",
				Description: "Performs a full-text search on notes with highlighted results",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"query": {
							Type:        genai.TypeString,
							Description: "The search query to find relevant notes",
						},
						"isArchived": {
							Type:        genai.TypeBoolean,
							Description: "Optional filter for archived notes",
						},
					},
					Required: []string{"query"},
				},
			},
			Function: SearchNotes,
		},
		"get_note": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "get_note",
				Description: "Retrieves a single note by ID",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"noteId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the note to retrieve",
						},
					},
					Required: []string{"noteId"},
				},
			},
			Function: GetNote,
		},
		"create_note": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "create_note",
				Description: "Creates a new note for the user",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"title": {
							Type:        genai.TypeString,
							Description: "The title of the note",
						},
						"content": {
							Type:        genai.TypeString,
							Description: "The content of the note",
						},
						"category": {
							Type:        genai.TypeString,
							Description: "Optional category for the note",
						},
						"tags": {
							Type:        genai.TypeArray,
							Description: "Optional array of tags for the note",
							Items: &genai.Schema{
								Type: genai.TypeString,
							},
						},
						"isPinned": {
							Type:        genai.TypeBoolean,
							Description: "Whether the note is pinned",
						},
						"isArchived": {
							Type:        genai.TypeBoolean,
							Description: "Whether the note is archived",
						},
					},
					Required: []string{"title"},
				},
			},
			Function: CreateNote,
		},
		"update_note": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "update_note",
				Description: "Updates an existing note",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"noteId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the note to update",
						},
						"title": {
							Type:        genai.TypeString,
							Description: "The updated title of the note",
						},
						"content": {
							Type:        genai.TypeString,
							Description: "The updated content of the note",
						},
						"category": {
							Type:        genai.TypeString,
							Description: "The updated category for the note",
						},
						"tags": {
							Type:        genai.TypeArray,
							Description: "The updated array of tags for the note",
							Items: &genai.Schema{
								Type: genai.TypeString,
							},
						},
						"isPinned": {
							Type:        genai.TypeBoolean,
							Description: "Whether the note is pinned",
						},
						"isArchived": {
							Type:        genai.TypeBoolean,
							Description: "Whether the note is archived",
						},
					},
					Required: []string{"noteId", "title"},
				},
			},
			Function: UpdateNote,
		},
		"delete_note": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "delete_note",
				Description: "Deletes a note",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"noteId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the note to delete",
						},
					},
					Required: []string{"noteId"},
				},
			},
			Function: DeleteNote,
		},
		"toggle_note_pin": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "toggle_note_pin",
				Description: "Toggles the pinned status of a note",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"noteId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the note to toggle pin status",
						},
						"isPinned": {
							Type:        genai.TypeBoolean,
							Description: "The new pinned status",
						},
					},
					Required: []string{"noteId", "isPinned"},
				},
			},
			Function: ToggleNotePin,
		},
		"toggle_note_archive": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "toggle_note_archive",
				Description: "Toggles the archived status of a note",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"noteId": {
							Type:        genai.TypeInteger,
							Description: "The ID of the note to toggle archive status",
						},
						"isArchived": {
							Type:        genai.TypeBoolean,
							Description: "The new archived status",
						},
					},
					Required: []string{"noteId", "isArchived"},
				},
			},
			Function: ToggleNoteArchive,
		},
		"verifyAuth": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
				},
			},
			Function: func(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) { return nil, nil },
		},
		"getUserConversation": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"clearConversationHistory": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getTickerDailySnapshot": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getAllTickerSnapshots": {
			FunctionDeclaration: genai.FunctionDeclaration{
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
		},
		"getSuggestedQueries": {
			FunctionDeclaration: genai.FunctionDeclaration{
				Name:        "getSuggestedQueries",
				Description: "DO NOT use this function. Internal function only.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"a": {
							Type:        genai.TypeString,
							Description: "Dummy parameter to satisfy requirements",
						},
					},
					Required: []string{},
				},
			},
			Function: GetSuggestedQueries,
		},
	}
}
