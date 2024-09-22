package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
    "io"
    "net/http"
	"github.com/jackc/pgx/v4"
)

/*type ValidateDatetimeArgs struct {
    Securityid

func ValidateDatetime(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    var args ValidateDatetimeArgs
    if err := json.Unmarshal(rawArgs, &args); err != nil {
        return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
    }*/

type GetCurrentTickerArgs struct {
    SecurityId int `json:"securityId"`
}

func GetCurrentTicker(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetCurrentTickerArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("di1n0fni0: %v", err)
	}
	var ticker string
	err := conn.DB.QueryRow(context.Background(),  "SELECT ticker FROM securities WHERE securityid=$1 AND maxDate is NULL",args.SecurityId).Scan(&ticker)
    if err == pgx.ErrNoRows {
        return "delisted" , nil
    } else if err != nil {
		return nil, fmt.Errorf("k01n0v0e: %v", err)
	}
    return ticker, nil
}

type GetPrevCloseArgs struct {
	SecurityId int `json:"securityId"`
	Timestamp  int `json:"timestamp"`
}

type PolygonBar struct {
	Close float64 `json:"close"`
}

func GetPrevClose(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetPrevCloseArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("getPrevClose invalid args: %v", err)
	}

	// Start at the given timestamp and subtract a day until a valid close is found
	currentDay := time.Unix(int64(args.Timestamp/1000), 0).UTC()
    currentDay = currentDay.AddDate(0, 0, -1)

	var bar PolygonBar
	var ticker string
    maxDaysToCheck := 10
    daysChecked := 0
	for daysChecked < maxDaysToCheck{
		// Check if it's a weekend (Saturday or Sunday)
		if currentDay.Weekday() == time.Saturday || currentDay.Weekday() == time.Sunday {
			// If it's a weekend, subtract another day
			currentDay = currentDay.AddDate(0, 0, -1)
			continue
		}

		// Format the current day as yyyy-mm-dd
		date := currentDay.Format("2006-01-02")

		// Query the ticker for the given securityId and date range
		query := `SELECT ticker FROM securities WHERE securityid=$1 AND (minDate <= $2 AND (maxDate IS NULL or maxDate >= $2)) ORDER BY minDate ASC`
		err := conn.DB.QueryRow(context.Background(), query, args.SecurityId, date).Scan(&ticker)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve ticker: %v", err)
		}

		// Make a request to Polygon's API for that date and ticker
		endpoint := fmt.Sprintf("https://api.polygon.io/v1/open-close/%s/%s?adjusted=true&apiKey=%s", ticker, date, conn.PolygonKey)
		resp, err := http.Get(endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Polygon snapshot: %v", err)
		}
		defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %v", err)
		}

		// Unmarshal the response into a PolygonBar struct
		if err := json.Unmarshal(body, &bar); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
		}

		// If the close price is found, return it
		if bar.Close != 0 {
            fmt.Println(currentDay)
			return bar.Close, nil
		}

		// If not a valid market day (e.g., holiday or no trading), go back one day
		currentDay = currentDay.AddDate(0, 0, -1)
        daysChecked ++
	}
    return nil, fmt.Errorf("dn10vn20")

}
/*type GetPrevCloseArgs struct {
    Ticker string `json:"ticker"`
}

type PolygonBar struct {
    Results []struct {
        Close float64 `json:"c"`  // Close price
    } `json:"results"`
}

func GetPrevClose(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetPrevCloseArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
	}
	endpoint := fmt.Sprintf("https://api.polygon.io/v2/aggs/ticker/%s/prev?adjusted=true&apiKey=%s",args.Ticker,conn.PolygonKey)
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Polygon snapshot: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	var bar PolygonBar
	if err := json.Unmarshal(body, &bar); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}
    if len(bar.Results) > 0 {
        return bar.Results[0].Close, nil
    }
    return nil, fmt.Errorf("lkmk2")

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


type GetSimilarInstancesArgs struct {
	Ticker     string `json:"ticker"`
	SecurityId int    `json:"securityId"`
	Timestamp   int64 `json:"timestamp"`
	Timeframe  string `json:"timeframe"`
}
type GetSimilarInstancesResults struct {
	Ticker     string `json:"ticker"`
	SecurityId int    `json:"securityId"`
	Timestamp   int64 `json:"timestamp"`
	Timeframe  string `json:"timeframe"`
}

func GetSimilarInstances(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetSimilarInstancesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("9hsdf invalid args: %v", err)
	}
	var queryTicker string
	conn.DB.QueryRow(context.Background(), `SELECT ticker from securities where securityId = $1
         ORDER BY maxDate IS NULL DESC, maxDate DESC`, args.SecurityId).Scan(&queryTicker)
	tickers, err := utils.GetPolygonRelatedTickers(conn.Polygon, queryTicker)
	if err != nil {
		return nil, fmt.Errorf("failed to get related tickers: %v", err)
	}
	if len(tickers) == 0 {
		return nil, fmt.Errorf("49sb no related tickers")
	}
	query := `
		SELECT ticker, securityId
		FROM securities
		WHERE ticker = ANY($1) AND (maxDate IS NULL OR maxDate >= $2) AND minDate <= $2
	`
    timestamp := time.Unix(args.Timestamp,0)
	rows, err := conn.DB.Query(context.Background(), query, tickers,timestamp)
	if err != nil {
		return nil, fmt.Errorf("1imvd: %v", err)
	}
	defer rows.Close()
	var results []GetSimilarInstancesResults
	for rows.Next() {
		var result GetSimilarInstancesResults
		err := rows.Scan(&result.Ticker, &result.SecurityId)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		fmt.Print(result.Ticker)
		result.Timestamp = args.Timestamp
		result.Timeframe = args.Timeframe
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}
	return results, nil
}

