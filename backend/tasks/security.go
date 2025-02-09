package tasks

import (
	"backend/utils"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

type GetCurrentTickerArgs struct {
	SecurityId int `json:"securityId"`
}

func GetCurrentTicker(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetCurrentTickerArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("di1n0fni0: %v", err)
	}
	var ticker string
	err := conn.DB.QueryRow(context.Background(), "SELECT ticker FROM securities WHERE securityid=$1 AND maxDate is NULL", args.SecurityId).Scan(&ticker)
	if err == pgx.ErrNoRows {
		return "delisted", nil
	} else if err != nil {
		return nil, fmt.Errorf("k01n0v0e: %v", err)
	}
	return ticker, nil
}

type GetMarketCapArgs struct {
	Ticker string `json:"ticker"`
}

type GetMarketCapResults struct {
	MarketCap int `json:"marketCap"`
}

func GetMarketCap(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetMarketCapArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("di1n0fni0: %v", err)
	}

	details, err := utils.GetTickerDetails(conn.Polygon, args.Ticker, "now")
	if err != nil {
		return nil, fmt.Errorf("k01n0v0e: %v", err)
	}

	if details.MarketCap == 0 {
		return GetMarketCapResults{MarketCap: 0}, nil
	}

	return GetMarketCapResults{MarketCap: int(details.MarketCap)}, nil
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
	for daysChecked < maxDaysToCheck {
		// Check if it's a weekend (Saturday or Sunday)
		if currentDay.Weekday() == time.Saturday || currentDay.Weekday() == time.Sunday {
			// If it's a weekend, subtract another day
			currentDay = currentDay.AddDate(0, 0, -1)
			continue
		}

		// Format the current day as yyyy-mm-dd
		date := currentDay.Format("2006-01-02")

		// Query the ticker for the given securityId and date range
		query := `SELECT ticker FROM securities WHERE securityid=$1 AND (minDate <= $2 AND (maxDate IS NULL or maxDate >= $2))`
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
		daysChecked++
	}
	return nil, fmt.Errorf("dn10vn20")

}

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

	// Clean and prepare the search query
	query := strings.ToUpper(strings.TrimSpace(args.Ticker))
	
	// Modified query to include fuzzy matching, removed COALESCE(name, '') since name column doesn't exist
	sqlQuery := `
	SELECT DISTINCT ON (s.ticker) securityId, ticker, maxDate
	FROM securities s
	WHERE (
		ticker LIKE $1 || '%' OR 
		ticker LIKE '%' || $1 || '%' OR
		similarity(ticker, $1) > 0.3
	)
	ORDER BY ticker, maxDate DESC NULLS FIRST
	LIMIT 20
	`

	rows, err := conn.DB.Query(context.Background(), sqlQuery, query)
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
	Timestamp  int64  `json:"timestamp"`
	Timeframe  string `json:"timeframe"`
}
type GetSimilarInstancesResults struct {
	Ticker     string `json:"ticker"`
	SecurityId int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"`
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
	timestamp := time.Unix(args.Timestamp, 0)
	rows, err := conn.DB.Query(context.Background(), query, tickers, timestamp)
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

type GetTickerDetailsArgs struct {
	SecurityId int `json:"securityId"`
}

type TickerDetailsResponse struct {
	Ticker                      string  `json:"ticker"`
	Name                        string  `json:"name"`
	Market                      string  `json:"market"`
	Locale                      string  `json:"locale"`
	PrimaryExchange             string  `json:"primary_exchange"`
	Active                      bool    `json:"active"`
	MarketCap                   float64 `json:"market_cap"`
	Description                 string  `json:"description"`
	Logo                        string  `json:"logo"`
	ShareClassSharesOutstanding int64   `json:"share_class_shares_outstanding"`
	Industry                    string  `json:"industry"`
	Sector                      string  `json:"sector"`
}

func GetTickerDetails(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTickerDetailsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	ticker, err := utils.GetTicker(conn, args.SecurityId, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker: %v", err)
	}
	details, err := utils.GetTickerDetails(conn.Polygon, ticker, "now")
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker details: %v", err)
	}

	var sector, industry string
	err = conn.DB.QueryRow(context.Background(), `SELECT sector, industry from securities 
    where securityId = $1 and maxDate is NULL`, args.SecurityId).Scan(&sector, &industry)
	if err != nil {
		return nil, fmt.Errorf("01if0d %v", err)
	}

	// Fetch the logo image if URL exists
	var logoBase64 string
	if details.Branding.LogoURL != "" {
		// Create HTTP client
		client := &http.Client{}

		// Create request with API key
		req, err := http.NewRequest("GET", details.Branding.LogoURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request for logo: %v", err)
		}
		req.Header.Add("Authorization", "Bearer "+conn.PolygonKey)

		// Make the request
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch logo: %v", err)
		}
		defer resp.Body.Close()

		// Read the image data
		imageData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read logo data: %v", err)
		}

		// Convert to base64
		logoBase64 = base64.StdEncoding.EncodeToString(imageData)
	}

	response := TickerDetailsResponse{
		Ticker:                      details.Ticker,
		Name:                        details.Name,
		Market:                      string(details.Market),
		Locale:                      string(details.Locale),
		PrimaryExchange:             details.PrimaryExchange,
		Active:                      details.Active,
		MarketCap:                   details.MarketCap,
		Description:                 details.Description,
		ShareClassSharesOutstanding: details.ShareClassSharesOutstanding,
		Logo:                        logoBase64,
		Sector:                      sector,
		Industry:                    industry,
	}

	return response, nil
}
