package tools

import (
	"backend/utils"
	"encoding/json"
	"sync"

	"github.com/tmc/langchaingo/llms"
)

/*
   ──────────────────────────────────────────────────────────────────────────
   Type definition
   ──────────────────────────────────────────────────────────────────────────
*/

// LangChain-ready wrapper around a business-logic handler.
type Tool struct {
	Definition *llms.FunctionDefinition                                     // nil == not LLM-callable
	Handler    func(*utils.Conn, int, json.RawMessage) (interface{}, error) // required
	IsQuery    bool                                                         // exposed in chat mode
	IsAPI      bool                                                         // exposed on HTTP API
}

/*
   ──────────────────────────────────────────────────────────────────────────
   Global container
   ──────────────────────────────────────────────────────────────────────────
*/

var (
	toolsMu       sync.Mutex
	toolsInitOnce bool
	registry      = map[string]Tool{}
)

// Public accessor (returns copy).
func GetTools(api bool) map[string]Tool {
	toolsMu.Lock()
	defer toolsMu.Unlock()

	if !toolsInitOnce {
		initTools()
		toolsInitOnce = true
	}
	out := make(map[string]Tool)
	for k, v := range registry {
		if api && v.IsAPI || !api && v.IsQuery {
			out[k] = v
		}
	}
	return out
}

/*
   ──────────────────────────────────────────────────────────────────────────
   Declarative spec → registry
   ──────────────────────────────────────────────────────────────────────────
*/

// helper trims repetitive JSON quoting noise
func js(src string) json.RawMessage { return json.RawMessage(src) }

// NB: keep everything here; the compiler drops what you comment out.
func initTools() {
	specs := []struct {
		Name, Desc string
		Params     string // raw JSON schema
		Handler    func(*utils.Conn, int, json.RawMessage) (interface{}, error)
		Query, API bool
	}{
		// ════════════ market intelligence ════════════
		{
			"getSimilarInstances",
			"Get a list of securities related to a specified security at the current time.",
			`{"type":"object","properties":{"securityId":{"type":"integer","description":"The security ID of the security"}},"required":["securityId"]}`,
			GetSimilarInstances, true, true,
		},
		{
			"getCurrentSecurityID",
			"Get the current security ID from a security ticker symbol.",
			`{"type":"object","properties":{"ticker":{"type":"string","description":"The security ticker symbol, e.g. NVDA, AAPL, etc"}},"required":["ticker"]}`,
			GetCurrentSecurityID, true, true,
		},
		{
			"getSecuritiesFromTicker",
			"Get a list of the closest 10 securitie ticker symbols to an input string.",
			`{"type":"object","properties":{"ticker":{"type":"string","description":"string input to retrieve the list based on."}},"required":["ticker"]}`,
			GetSecuritiesFromTicker, true, true,
		},
		{
			"getCurrentTicker",
			"Get the current security ticker symbol for a security ID.",
			`{"type":"object","properties":{"securityId":{"type":"integer","description":"The security ID of the security to get the current ticker for."}},"required":["securityId"]}`,
			GetCurrentTicker, true, true,
		},
		{
			"getTickerMenuDetails",
			"Get company name, market, locale, primary exchange, active status, market cap, description, logo, shares outstanding, industry, sector and total shares for a given security.",
			`{"type":"object","properties":{"ticker":{"type":"string","description":"The security ticker symbol to get details for."}},"required":["ticker"]}`,
			GetTickerMenuDetails, true, true,
		},
		{
			"getInstancesByTickers",
			"Get security IDs for a list of security ticker symbols.",
			`{"type":"object","properties":{"tickers":{"type":"array","description":"List of security ticker symbols.","items":{"type":"string"}}},"required":["tickers"]}`,
			GetInstancesByTickers, true, true,
		},
		{
			"getPrevClose",
			"Retrieves the previous closing price for a specified security ticker symbol.",
			`{"type":"object","properties":{"ticker":{"type":"string","description":"The ticker symbol to get the previous close for."}},"required":["ticker"]}`,
			GetPrevClose, true, true,
		},
		{
			"getTickerDailySnapshot",
			"Get the current bid, ask, price, change, percent change, volume, vwap price, and daily open, high, low and close for a specified security.",
			`{"type":"object","properties":{"securityId":{"type":"integer","description":"The security ID to get the information for."}},"required":["securityId"]}`,
			GetTickerDailySnapshot, true, false, // API=false as per original
		},
		{
			"getAllTickerSnapshots",
			"Get a list of the current bid, ask, price, change, percent change, volume, vwap price, and daily open, high, low and close for all securities.",
			`{"type":"object","properties":{},"required":[]}`,
			GetAllTickerSnapshots, true, false, // API=false as per original
		},
		{
			"getSecurityClassifications",
			"Retrieves sector and industry classifications for securities",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetSecurityClassifications, false, true,
		},
		{
			"getActive",
			"Retrieves a list of active securities",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetActive, false, true,
		},
		{
			"getExchanges",
			"Retrieves a list of stock exchanges and their MIC codes from Polygon.io",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetExchanges, false, true,
		},

		// ════════════ chart data ════════════
		{
			"getChartData",
			"Retrieves price chart data for a security",
			`{"type":"object","properties":{"securityId":{"type":"integer","description":"The ID of the security to get chart data for"},"timeframe":{"type":"string","description":"The timeframe for the chart data (e.g., '1d', '1h')"},"from":{"type":"integer","description":"The start timestamp in milliseconds"},"to":{"type":"integer","description":"The end timestamp in milliseconds"}},"required":["securityId","timeframe","from","to"]}`,
			GetChartData, true, true,
		},
		{
			"setHorizontalLine",
			"Create a new horizontal line on the chart of a specified security ID at a specificed price.",
			`{"type":"object","properties":{"securityId":{"type":"integer","description":"The ID of the security to add the horizontal line to."},"price":{"type":"number","description":"The price level for the horizontal line."},"color":{"type":"string","description":"The color of the horizontal line (hex format, defaults to #FFFFFF)."},"lineWidth":{"type":"integer","description":"The width of the horizontal line in pixels (defaults to 1)."}},"required":["securityId","price"]}`,
			SetHorizontalLine, true, true,
		},
		{
			"getHorizontalLines",
			"Retrieves all horizontal lines for a specific security",
			`{"type":"object","properties":{"securityId":{"type":"integer","description":"The ID of the security to get horizontal lines for"}},"required":["securityId"]}`,
			GetHorizontalLines, true, true,
		},
		{
			"deleteHorizontalLine",
			"Delete a horizontal line on the chart of a specified security ID.",
			`{"type":"object","properties":{"id":{"type":"integer","description":"The ID of the horizontal line to delete."}},"required":["id"]}`,
			DeleteHorizontalLine, true, true,
		},
		{
			"updateHorizontalLine",
			"Update an existing horizontal line on the chart of a specified security ID.",
			`{"type":"object","properties":{"id":{"type":"integer","description":"The ID of the horizontal line to update."},"securityId":{"type":"integer","description":"The ID of the security the horizontal line belongs to."},"price":{"type":"number","description":"The new price level for the horizontal line."},"color":{"type":"string","description":"The new color of the horizontal line (hex format)."},"lineWidth":{"type":"integer","description":"The new width of the horizontal line in pixels."}},"required":["id","securityId","price"]}`,
			UpdateHorizontalLine, true, true,
		},

		// ════════════ chart events / SEC filings ════════════
		{
			"getLatestEdgarFilings",
			"Retrieves the latest SEC EDGAR filings across all securities",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetLatestEdgarFilings, false, true,
		},
		{
			"getStockEdgarFilings",
			"Retrieve a list of urls and filing types for all SEC filings for a specified security within a specified time range.",
			`{"type":"object","properties":{"start":{"type":"integer","description":"The start of the date range in milliseconds."},"end":{"type":"integer","description":"The end of the date range in milliseconds."},"securityId":{"type":"integer","description":"The ID of the security to get filings for."}},"required":["start","end","securityId"]}`,
			GetStockEdgarFilings, true, true,
		},
		{
			"getChartEvents",
			"Retrieves splits, dividends and possibly SEC filings for a specified security ID within a date range",
			`{"type":"object","properties":{"securityId":{"type":"integer","description":"The ID of the security to get events for."},"from":{"type":"integer","description":"The start of the date range in milliseconds."},"to":{"type":"integer","description":"The end of the date range in milliseconds."},"includeSECFilings":{"type":"boolean","description":"Whether to include SEC filings in the result."}},"required":["securityId","from","to"]}`,
			GetChartEvents, true, true,
		},
		{
			"getEarningsText",
			"Get the plain text content of the earnings SEC filing for a specified quarter, year, and security.",
			`{"type":"object","properties":{"securityId":{"type":"integer","description":"The security ID to get the filing for."},"quarter":{"type":"string","description":"The specific quarter (Q1, Q2, Q3, Q4) to retrieve the filing for, returns the latest filing if not specified."},"year":{"type":"integer","description":"The specific year to retrieve the filing from."}},"required":["securityId"]}`,
			GetEarningsText, true, true,
		},
		{
			"getFilingText",
			"Retrieves the text content of a SEC filing from a specified url.",
			`{"type":"object","properties":{"url":{"type":"string","description":"The URL of the SEC filing to retrieve."}},"required":["url"]}`,
			GetFilingText, true, true,
		},

		// ════════════ studies (back-office) ════════════
		{"getStudies", "", `{}`, GetStudies, false, true},    // No FunctionDeclaration originally
		{"newStudy", "", `{}`, NewStudy, false, true},      // No FunctionDeclaration originally
		{"saveStudy", "", `{}`, SaveStudy, false, true},     // No FunctionDeclaration originally
		{"deleteStudy", "", `{}`, DeleteStudy, false, true},   // No FunctionDeclaration originally
		{"getStudyEntry", "", `{}`, GetStudyEntry, false, true}, // No FunctionDeclaration originally
		{
			"completeStudy",
			"Marks a study entry as completed or not completed",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			CompleteStudy, false, true,
		},
		{
			"setStudySetup",
			"Associates a setup configuration with a study entry",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			SetStudySetup, false, true,
		},

		// ════════════ journal ════════════
		{
			"getJournals",
			"Retrieves all journal entries for the current user",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetJournals, false, true,
		},
		{
			"saveJournal",
			"Saves content for an existing journal entry",
			`{"type":"object","properties":{"id":{"type":"integer","description":"The ID of the journal entry to update"},"entry":{"type":"object","description":"The content of the journal entry in JSON format","properties":{"title":{"type":"string","description":"The title of the journal entry"},"content":{"type":"string","description":"The content of the journal entry"},"date":{"type":"string","description":"The date of the journal entry"}}}},"required":["id","entry"]}`,
			SaveJournal, false, true,
		},
		{
			"deleteJournal",
			"Deletes a journal entry for the current user",
			`{"type":"object","properties":{"id":{"type":"integer","description":"The ID of the journal entry to delete"}},"required":["id"]}`,
			DeleteJournal, true, true,
		},
		{
			"getJournalEntry",
			"Retrieves the content of a specific journal entry",
			`{"type":"object","properties":{"journalId":{"type":"integer","description":"The ID of the journal entry to retrieve"}},"required":["journalId"]}`,
			GetJournalEntry, false, true,
		},
		{
			"completeJournal",
			"Marks a journal entry as completed or not completed",
			`{"type":"object","properties":{"id":{"type":"integer","description":"The ID of the journal entry to update"},"completed":{"type":"boolean","description":"Whether the journal entry is completed"}},"required":["id","completed"]}`,
			CompleteJournal, true, true,
		},

		// ════════════ watchlist ════════════
		{
			"getWatchlists",
			"Get all watchlist names and IDs.",
			`{"type":"object","properties":{},"required":[]}`, // Already updated to have no params
			GetWatchlists, true, true,
		},
		{
			"deleteWatchlist",
			"Delete a watchlist.",
			`{"type":"object","properties":{"watchlistId":{"type":"integer","description":"The ID of the watchlist to delete."}},"required":["watchlistId"]}`,
			DeleteWatchlist, true, true,
		},
		{
			"newWatchlist",
			"Create a new empty watchlist",
			`{"type":"object","properties":{"watchlistName":{"type":"string","description":"The name of the watchlist to create"}},"required":["watchlistName"]}`,
			NewWatchlist, true, true,
		},
		{
			"getWatchlistItems",
			"Retrieves the security ID's of the securities in a specified watchlist.",
			`{"type":"object","properties":{"watchlistId":{"type":"integer","description":"The ID of the watchlist to get the list of security IDs for."}},"required":["watchlistId"]}`,
			GetWatchlistItems, true, true,
		},
		{
			"deleteWatchlistItem",
			"Removes a security from a watchlist using a given watchlist item ID.",
			`{"type":"object","properties":{"watchlistItemId":{"type":"integer","description":"The ID of the watchlist item to delete"}},"required":["watchlistItemId"]}`,
			DeleteWatchlistItem, true, true,
		},
		{
			"newWatchlistItem",
			"Add a security to a watchlist.",
			`{"type":"object","properties":{"watchlistId":{"type":"integer","description":"The ID of the watchlist to add the security to."},"securityId":{"type":"integer","description":"The ID of the security to add to the watchlist."}},"required":["watchlistId","securityId"]}`,
			NewWatchlistItem, true, true,
		},

		// ════════════ settings ════════════
		{
			"getSettings",
			"Retrieves the user settings for the current user",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetSettings, false, true,
		},
		{
			"setSettings",
			"Updates the user settings for the current user",
			`{"type":"object","properties":{"settings":{"type":"object","description":"The settings data in JSON format","properties":{"theme":{"type":"string","description":"Theme preference (e.g., 'light', 'dark')"},"notifications":{"type":"boolean","description":"Whether notifications are enabled"}}}},"required":["settings"]}`,
			SetSettings, false, true,
		},

		// ════════════ profile ════════════
		{
			"updateProfilePicture",
			"Updates the profile picture for the current user",
			`{"type":"object","properties":{"imageData":{"type":"string","description":"The base64-encoded image data for the profile picture"}},"required":["imageData"]}`,
			UpdateProfilePicture, false, true,
		},

		// ════════════ setups ════════════
		{
			"getSetups",
			"Retrieves all setup configurations for the current user",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetSetups, false, true,
		},
		{
			"newSetup",
			"Creates a new setup configuration for the current user",
			`{"type":"object","properties":{"name":{"type":"string","description":"The name of the setup"},"timeframe":{"type":"string","description":"The timeframe for the setup (e.g., '1d', '1h')"},"bars":{"type":"integer","description":"The number of bars to consider for the setup"},"threshold":{"type":"integer","description":"The threshold value for the setup"},"dolvol":{"type":"number","description":"The dollar volume filter for the setup"},"adr":{"type":"number","description":"The Average Daily Range filter for the setup"},"mcap":{"type":"number","description":"The market capitalization filter for the setup"}},"required":["name","timeframe"]}`,
			NewSetup, false, true,
		},
		{
			"setSetup",
			"Updates an existing setup configuration for the current user",
			`{"type":"object","properties":{"setupId":{"type":"integer","description":"The ID of the setup to update"},"name":{"type":"string","description":"The name of the setup"},"timeframe":{"type":"string","description":"The timeframe for the setup (e.g., '1d', '1h')"},"bars":{"type":"integer","description":"The number of bars to consider for the setup"},"threshold":{"type":"integer","description":"The threshold value for the setup"},"dolvol":{"type":"number","description":"The dollar volume filter for the setup"},"adr":{"type":"number","description":"The Average Daily Range filter for the setup"},"mcap":{"type":"number","description":"The market capitalization filter for the setup"}},"required":["setupId","name","timeframe"]}`,
			SetSetup, false, true,
		},
		{
			"deleteSetup",
			"Deletes a setup configuration for the current user",
			`{"type":"object","properties":{"setupId":{"type":"integer","description":"The ID of the setup to delete"}},"required":["setupId"]}`,
			DeleteSetup, false, true,
		},

		// ════════════ samples / training ════════════
		{
			"labelTrainingQueueInstance",
			"Labels a training instance in the queue",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			LabelTrainingQueueInstance, false, true,
		},
		{
			"getTrainingQueue",
			"Retrieves the current training queue instances",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetTrainingQueue, false, true,
		},
		{
			"setSample",
			"Sets a sample for training purposes",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			SetSample, false, true,
		},

		// ════════════ alerts ════════════
		{
			"getAlerts",
			"Retrieves all alerts for the current user",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetAlerts, false, true,
		},
		{
			"getAlertLogs",
			"Retrieves the history of triggered alerts for the current user",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetAlertLogs, false, true,
		},
		{
			"newAlert",
			"Creates a new alert for the current user",
			`{"type":"object","properties":{"alertType":{"type":"string","description":"The type of alert (price, setup, or algo)"},"price":{"type":"number","description":"The price threshold for a price alert (required for price alerts)"},"securityId":{"type":"integer","description":"The ID of the security for a price alert (required for price alerts)"},"setupId":{"type":"integer","description":"The ID of the setup for a setup alert (required for setup alerts)"},"ticker":{"type":"string","description":"The ticker symbol for a price alert"},"algoId":{"type":"integer","description":"The ID of the algorithm for an algo alert (required for algo alerts)"}},"required":["alertType"]}`,
			NewAlert, false, true,
		},
		{
			"deleteAlert",
			"Deletes an alert for the current user",
			`{"type":"object","properties":{"alertId":{"type":"integer","description":"The ID of the alert to delete"}},"required":["alertId"]}`,
			DeleteAlert, false, true,
		},

		// ════════════ Account / User Trades ════════════
		{
			"grab_user_trades",
			"Get user trades with optional filtering.",
			`{"type":"object","properties":{"ticker":{"type":"string","description":"Security ticker symbol to filter trades by."},"startDate":{"type":"string","description":"Date range start to filter trades by (format: YYYY-MM-DD)."},"endDate":{"type":"string","description":"Date range end to filter trades by (format: YYYY-MM-DD)."}},"required":[]}`,
			GrabUserTrades, true, true,
		},
		{
			"get_trade_statistics",
			"Get user trading performance statistics.",
			`{"type":"object","properties":{"ticker":{"type":"string","description":"Security ticker symbol to filter trades by."},"startDate":{"type":"string","description":"Date range start to filter trades by (format: YYYY-MM-DD)."},"endDate":{"type":"string","description":"Date range end to filter trades by (format: YYYY-MM-DD)."}},"required":[]}`,
			GetTradeStatistics, true, true,
		},
		{
			"get_ticker_performance",
			"Get user trade performance statistics for a specific security.",
			`{"type":"object","properties":{"ticker":{"type":"string","description":"The security ticker symbol to get performance statistics for."},"securityId":{"type":"integer","description":"The security ID to get performance statistics for."}},"required":["ticker","securityId"]}`,
			GetTickerPerformance, true, true,
		},
		{
			"delete_all_user_trades",
			"Deletes all trade records for the current user",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			DeleteAllUserTrades, false, true,
		},
		{
			"handle_trade_upload",
			"Processes and imports trade data from a CSV file",
			`{"type":"object","properties":{"csvData":{"type":"string","description":"The CSV trade data to process and import"},"broker":{"type":"string","description":"The broker name for the trade data (e.g., 'interactive_brokers', 'td_ameritrade')"}},"required":["csvData","broker"]}`,
			HandleTradeUpload, false, true,
		},
		{
			"get_daily_trade_stats",
			"Retrieves user trading statistics for a specified year and month.",
			`{"type":"object","properties":{"year":{"type":"integer","description":"The year part of the date to get statistics for."},"month":{"type":"integer","description":"The month part of the date to get statistics for."}},"required":["year","month"]}`,
			GetDailyTradeStats, true, true,
		},

		// ════════════ backtesting ════════════
		{
			"run_backtest",
			"Runs a backtest based on a natural language query about stock conditions, patterns, and indicators IF YOU CALL THIS TOOL, USE THE USER'S ORIGINAL QUERY. DO NOT GENERATE A NEW QUERY.",
			`{"type":"object","properties":{"query":{"type":"string","description":"Natural language query describing the backtest criteria  IF YOU CALL THIS TOOL, USE THE USER'S ORIGINAL QUERY. DO NOT GENERATE A NEW QUERY."}},"required":["query"]}`,
			RunBacktest, true, true,
		},

		// ════════════ Notes ════════════
		{
			"get_notes",
			"Retrieves notes for the current user with optional filtering",
			`{"type":"object","properties":{"category":{"type":"string","description":"Optional category to filter notes by"},"tags":{"type":"array","description":"Optional array of tags to filter notes by","items":{"type":"string"}},"isPinned":{"type":"boolean","description":"Optional filter for pinned notes"},"isArchived":{"type":"boolean","description":"Optional filter for archived notes"},"searchQuery":{"type":"string","description":"Optional text search query to filter notes by content"}},"required":[]}`,
			GetNotes, false, true,
		},
		{
			"search_notes",
			"Performs a full-text search on notes with highlighted results",
			`{"type":"object","properties":{"query":{"type":"string","description":"The search query to find relevant notes"},"isArchived":{"type":"boolean","description":"Optional filter for archived notes"}},"required":["query"]}`,
			SearchNotes, false, true,
		},
		{
			"get_note",
			"Retrieves a single note by ID",
			`{"type":"object","properties":{"noteId":{"type":"integer","description":"The ID of the note to retrieve"}},"required":["noteId"]}`,
			GetNote, false, true,
		},
		{
			"create_note",
			"Creates a new note for the user",
			`{"type":"object","properties":{"title":{"type":"string","description":"The title of the note"},"content":{"type":"string","description":"The content of the note"},"category":{"type":"string","description":"Optional category for the note"},"tags":{"type":"array","description":"Optional array of tags for the note","items":{"type":"string"}},"isPinned":{"type":"boolean","description":"Whether the note is pinned"},"isArchived":{"type":"boolean","description":"Whether the note is archived"}},"required":["title"]}`,
			CreateNote, false, true,
		},
		{
			"update_note",
			"Updates an existing note",
			`{"type":"object","properties":{"noteId":{"type":"integer","description":"The ID of the note to update"},"title":{"type":"string","description":"The updated title of the note"},"content":{"type":"string","description":"The updated content of the note"},"category":{"type":"string","description":"The updated category for the note"},"tags":{"type":"array","description":"The updated array of tags for the note","items":{"type":"string"}},"isPinned":{"type":"boolean","description":"Whether the note is pinned"},"isArchived":{"type":"boolean","description":"Whether the note is archived"}},"required":["noteId","title"]}`,
			UpdateNote, false, true,
		},
		{
			"delete_note",
			"Deletes a note",
			`{"type":"object","properties":{"noteId":{"type":"integer","description":"The ID of the note to delete"}},"required":["noteId"]}`,
			DeleteNote, false, true,
		},
		{
			"toggle_note_pin",
			"Toggles the pinned status of a note",
			`{"type":"object","properties":{"noteId":{"type":"integer","description":"The ID of the note to toggle pin status"},"isPinned":{"type":"boolean","description":"The new pinned status"}},"required":["noteId","isPinned"]}`,
			ToggleNotePin, false, true,
		},
		{
			"toggle_note_archive",
			"Toggles the archived status of a note",
			`{"type":"object","properties":{"noteId":{"type":"integer","description":"The ID of the note to toggle archive status"},"isArchived":{"type":"boolean","description":"The new archived status"}},"required":["noteId","isArchived"]}`,
			ToggleNoteArchive, false, true,
		},

		// ════════════ conversation management ════════════
		{
			"getUserConversation",
			"Retrieves the conversation history for the current user",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetUserConversation, false, true,
		},
		{
			"clearConversationHistory",
			"Deletes the entire conversation history for the current user.",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			ClearConversationHistory, false, true,
		},

		// ════════════ utilities (non-LLM) ════════════
		{"getIcons", "", `{}`, GetIcons, false, true}, // No FunctionDeclaration originally
		{"getQuery", "", `{}`, GetQuery, false, true}, // internal MRKL entry point, No FunctionDeclaration originally
		{
			"verifyAuth",
			"Verifies the authentication status of the user",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			func(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) { return nil, nil }, false, true,
		},
		{"getSuggestedQueries", "", `{}`, GetSuggestedQueries, false, true}, // No FunctionDeclaration originally
		{"getScreensavers", // screensaver moved to utilities as it seems less core
			"Retrieves a list of trending securities for the screensaver display",
			`{"type":"object","properties":{},"required":[]}`, // Dummy param removed
			GetScreensavers, false, true,
		},
	}

	// ---- build registry ----
	for _, s := range specs {
		var def *llms.FunctionDefinition
		if s.Desc != "" { // make LLM-callable only when we have a schema
			def = &llms.FunctionDefinition{
				Name:        s.Name,
				Description: s.Desc,
				Parameters:  js(s.Params),
			}
		}
		registry[s.Name] = Tool{
			Definition: def,
			Handler:    s.Handler,
			IsQuery:    s.Query,
			IsAPI:      s.API,
		}
	}
}

