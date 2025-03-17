package server

import (
	"backend/tasks"
	"backend/utils"
	"encoding/json"

	"github.com/google/generative-ai-go/genai"
)

type Tool struct {
	genai.FunctionDeclaration
	Function func(*utils.Conn, int, json.RawMessage) (interface{}, error)
}

var privateFunc = map[string]Tool{
	"verifyAuth": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "verifyAuth",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: verifyAuth,
	},
	//securities
	"getSimilarInstances": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getSimilarInstances",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetSimilarInstances,
	},
	"getSecuritiesFromTicker": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getSecuritiesFromTicker",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetSecuritiesFromTicker,
	},
	"getCurrentTicker": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getCurrentTicker",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetCurrentTicker,
	},
	"getTickerMenuDetails": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getTickerMenuDetails",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetTickerMenuDetails,
	},
	"getIcons": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getIcons",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetIcons,
	},

	//chart
	"getChartData": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getChartData",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetChartData,
	},
	//study
	"getStudies": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getStudies",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetStudies,
	},

	"newStudy": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "newStudy",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.NewStudy,
	},
	"saveStudy": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "saveStudy",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.SaveStudy,
	},
	"deleteStudy": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "deleteStudy",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.DeleteStudy,
	},
	"getStudyEntry": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getStudyEntry",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetStudyEntry,
	},
	"completeStudy": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "completeStudy",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.CompleteStudy,
	},
	"setStudySetup": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "setStudySetup",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.SetStudySetup,
	},
	//journal
	"getJournals": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getJournals",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetJournals,
	},
	"saveJournal": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "saveJournal",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.SaveJournal,
	},
	"deleteJournal": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "deleteJournal",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.DeleteJournal,
	},
	"getJournalEntry": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getJournalEntry",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetJournalEntry,
	},
	"completeJournal": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "completeJournal",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.CompleteJournal,
	},
	//screensaver
	"getScreensavers": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getScreensavers",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetScreensavers,
	},
	//watchlist
	"getWatchlists": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getWatchlists",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetWatchlists,
	},
	"deleteWatchlist": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "deleteWatchlist",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.DeleteWatchlist,
	},
	"newWatchlist": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "newWatchlist",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.NewWatchlist,
	},
	"getWatchlistItems": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getWatchlistItems",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetWatchlistItems,
	},
	"deleteWatchlistItem": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "deleteWatchlistItem",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.DeleteWatchlistItem,
	},
	"newWatchlistItem": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "newWatchlistItem",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.NewWatchlistItem,
	},
	//singles
	"getPrevClose": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getPrevClose",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetPrevClose,
	},
	//"getMarketCap": tasks.GetMarketCap,
	//settings
	"getSettings": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getSettings",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetSettings,
	},
	"setSettings": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "setSettings",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.SetSettings,
	},
	//profile
	"updateProfilePicture": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "updateProfilePicture",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: UpdateProfilePicture,
	},
	//exchanges
	"getExchanges": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getExchanges",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetExchanges,
	},
	//setups
	"getSetups": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getSetups",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetSetups,
	},
	"newSetup": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "newSetup",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.NewSetup,
	},
	"setSetup": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "setSetup",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.SetSetup,
	},
	"deleteSetup": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "deleteSetup",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.DeleteSetup,
	},
	//algos
	//"getAlgos": tasks.GetAlgos,
	//samples
	"labelTrainingQueueInstance": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "labelTrainingQueueInstance",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.LabelTrainingQueueInstance,
	},
	"getTrainingQueue": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getTrainingQueue",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetTrainingQueue,
	},
	"setSample": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "setSample",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.SetSample,
	},
	//telegram
	//	"sendMessage": telegram.SendMessage,
	//alerts
	"getAlerts": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getAlerts",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetAlerts,
	},
	"getAlertLogs": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getAlertLogs",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetAlertLogs,
	},
	"newAlert": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "newAlert",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.NewAlert,
	},
	"deleteAlert": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "deleteAlert",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.DeleteAlert,
	},

	// deprecated
	// "getTradeData":            tasks.GetTradeData,
	//
	//	"getLastTrade":            tasks.GetLastTrade,
	//
	// "getQuoteData":            tasks.GetQuoteData,
	// "getSecurityDateBounds":   tasks.GetSecurityDateBounds,
	"setHorizontalLine": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "setHorizontalLine",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.SetHorizontalLine,
	},
	"getHorizontalLines": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getHorizontalLines",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetHorizontalLines,
	},
	"deleteHorizontalLine": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "deleteHorizontalLine",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.DeleteHorizontalLine,
	},
	"updateHorizontalLine": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "updateHorizontalLine",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.UpdateHorizontalLine,
	},
	//active
	"getActive": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getActive",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetActive,
	},
	//sector, industry
	"getSecurityClassifications": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getSecurityClassifications",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetSecurityClassifications,
	},
	"getLatestEdgarFilings": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getLatestEdgarFilings",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetLatestEdgarFilings,
	},
	"getChartEvents": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "getChartEvents",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetChartEvents,
	},

	// Add the new trade-related functions
	"grab_user_trades": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "grab_user_trades",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GrabUserTrades,
	},
	"get_trade_statistics": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "get_trade_statistics",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetTradeStatistics,
	},
	"get_ticker_performance": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "get_ticker_performance",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.GetTickerPerformance,
	},
	"delete_all_user_trades": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "delete_all_user_trades",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.DeleteAllUserTrades,
	},
	"handle_trade_upload": {
		FunctionDeclaration: genai.FunctionDeclaration{
			Name:        "handle_trade_upload",
			Description: "",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
				Required:   []string{},
			},
		},
		Function: tasks.HandleTradeUpload,
	},
}
