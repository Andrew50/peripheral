package agent

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"encoding/json"
)

type uiOpenArgs struct {
	WatchlistID int    `json:"watchlistId,omitempty"`
	AlertID     int    `json:"alertId,omitempty"`
	EventID     int    `json:"eventId,omitempty"`
	StrategyID  int    `json:"strategyId,omitempty"`
	Ticker      string `json:"ticker,omitempty"`
	SecurityID  int    `json:"securityId,omitempty"`
	Timeframe   string `json:"timeframe,omitempty"`
	Timestamp   int64  `json:"timestamp,omitempty"`
}

func uiOpen(conn *data.Conn, userID int, raw json.RawMessage, action string) (interface{}, error) {
	var args uiOpenArgs
	_ = json.Unmarshal(raw, &args)
	params := make(map[string]interface{})
	if args.WatchlistID != 0 {
		params["watchlistId"] = args.WatchlistID
	}
	if args.AlertID != 0 {
		params["alertId"] = args.AlertID
	}
	if args.EventID != 0 {
		params["eventId"] = args.EventID
	}
	if args.StrategyID != 0 {
		params["strategyId"] = args.StrategyID
	}
	if args.Ticker != "" {
		params["ticker"] = args.Ticker
	}
	if args.SecurityID != 0 {
		params["securityId"] = args.SecurityID
	}
	if args.Timeframe != "" {
		params["timeframe"] = args.Timeframe
	}
	if args.Timestamp != 0 {
		params["timestamp"] = args.Timestamp
	}
	socket.SendUIAction(userID, action, params)
	return "ok", nil
}

func OpenWatchlist(conn *data.Conn, userID int, raw json.RawMessage) (interface{}, error) {
	return uiOpen(conn, userID, raw, "open_watchlist")
}

func OpenAlerts(conn *data.Conn, userID int, raw json.RawMessage) (interface{}, error) {
	return uiOpen(conn, userID, raw, "open_alerts")
}

func OpenNews(conn *data.Conn, userID int, raw json.RawMessage) (interface{}, error) {
	return uiOpen(conn, userID, raw, "open_news")
}

func OpenStrategy(conn *data.Conn, userID int, raw json.RawMessage) (interface{}, error) {
	return uiOpen(conn, userID, raw, "open_strategy")
}

func OpenBacktest(conn *data.Conn, userID int, raw json.RawMessage) (interface{}, error) {
	return uiOpen(conn, userID, raw, "open_backtest")
}

func QueryChartUI(conn *data.Conn, userID int, raw json.RawMessage) (interface{}, error) {
	return uiOpen(conn, userID, raw, "query_chart")
}
