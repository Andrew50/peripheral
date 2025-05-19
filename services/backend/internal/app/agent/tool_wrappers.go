package agent

import (
	"backend/internal/app/helpers"
	"backend/internal/data"
	"encoding/json"
)

// GetSecuritiesFromTickerLight wraps helpers.GetSecuritiesFromTicker and
// returns only the securityId, ticker and name fields.
func GetSecuritiesFromTickerLight(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := helpers.GetSecuritiesFromTicker(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}

	list, ok := res.([]helpers.GetSecurityFromTickerResults)
	if !ok {
		return res, nil
	}

	slim := make([]map[string]interface{}, 0, len(list))
	for _, s := range list {
		slim = append(slim, map[string]interface{}{
			"securityId": s.SecurityID,
			"ticker":     s.Ticker,
			"name":       s.Name,
		})
	}
	return slim, nil
}

// GetTickerMenuSummary wraps helpers.GetTickerMenuDetails and strips out
// large image fields that are unnecessary for agent responses.
func GetTickerMenuSummary(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := helpers.GetTickerMenuDetails(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}

	details, ok := res.(map[string]interface{})
	if !ok {
		return res, nil
	}

	summary := map[string]interface{}{
		"ticker":           details["ticker"],
		"name":             details["name"],
		"market":           details["market"],
		"locale":           details["locale"],
		"primary_exchange": details["primary_exchange"],
		"active":           details["active"],
		"market_cap":       details["market_cap"],
		"description":      details["description"],
		"industry":         details["industry"],
		"sector":           details["sector"],
	}
	if val, ok := details["totalShares"]; ok {
		summary["totalShares"] = val
	}
	if val, ok := details["share_class_shares_outstanding"]; ok {
		summary["share_class_shares_outstanding"] = val
	}

	return summary, nil
}
