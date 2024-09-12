package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

/*type ValidateDatetimeArgs struct {
    Securityid

func ValidateDatetime(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    var args ValidateDatetimeArgs
    if err := json.Unmarshal(rawArgs, &args); err != nil {
        return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
    }*/

type GetSecurityFromTickerArgs struct {
	Ticker string `json:"ticker"`
}

type GetSecurityFromTickerResults struct {
	SecurityId int        `json:"securityId"`
	Ticker     string     `json:"ticker"`
	MaxDate    *time.Time `json:"maxDate"`
}

func GetSecuritiesFromTicker(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetSecurityFromTickerArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
	}
	rows, err := conn.DB.Query(context.Background(), `
    SELECT securityId, ticker, maxDate 
    from securities where ticker = $1
    ORDER BY maxDate IS  NULL DESC,
    maxDate DESC
    `, args.Ticker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var securities []GetSecurityFromTickerResults
	for rows.Next() {
		var security GetSecurityFromTickerResults
		if err := rows.Scan(&security.SecurityId, &security.Ticker, &security.MaxDate); err != nil {
			return nil, err
		}
		securities = append(securities, security)
	}
	return securities, nil
}
