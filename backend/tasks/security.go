package tasks

import (
	"backend/utils"
	"context"
	"database/sql"
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
	SecurityId int    `json:"securityId"`
	Ticker     string `json:"ticker"`
	Timestamp  int64  `json:"timestamp"`
	Icon       string `json:"icon"`
	Name       string `json:"name"`
}

func GetSecuritiesFromTicker(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetSecurityFromTickerArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
	}

	// Clean and prepare the search query
	query := strings.ToUpper(strings.TrimSpace(args.Ticker))

	// Modified query to properly handle name and icon and prioritize active securities
	sqlQuery := `
	WITH ranked_results AS (
		SELECT DISTINCT ON (s.ticker) 
			securityId, 
			ticker,
			NULLIF(name, '') as name,
			NULLIF(icon, '') as icon, 
			maxDate,
			CASE 
				WHEN UPPER(ticker) = UPPER($1) THEN 1
				WHEN UPPER(ticker) LIKE UPPER($1) || '%' THEN 2
				WHEN UPPER(ticker) LIKE '%' || UPPER($1) || '%' THEN 3
				ELSE 4
			END as match_type,
			similarity(UPPER(ticker), UPPER($1)) as sim_score,
			CASE 
				WHEN maxDate IS NULL THEN 0
				ELSE 1
			END as is_delisted
		FROM securities s
		WHERE (
			UPPER(ticker) = UPPER($1) OR
			UPPER(ticker) LIKE UPPER($1) || '%' OR 
			UPPER(ticker) LIKE '%' || UPPER($1) || '%' OR
			similarity(UPPER(ticker), UPPER($1)) > 0.3
		)
		ORDER BY ticker, maxDate DESC NULLS FIRST
	)
	SELECT securityId, ticker, name, icon, maxDate
	FROM ranked_results
	ORDER BY match_type, is_delisted, sim_score DESC
	LIMIT 10
	`

	rows, err := conn.DB.Query(context.Background(), sqlQuery, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var securities []GetSecurityFromTickerResults
	for rows.Next() {
		var security GetSecurityFromTickerResults
		var timestamp sql.NullTime
		var name, icon sql.NullString
		if err := rows.Scan(&security.SecurityId, &security.Ticker, &name, &icon, &timestamp); err != nil {
			return nil, err
		}
		if timestamp.Valid {
			security.Timestamp = timestamp.Time.UnixMilli()
		} else {
			security.Timestamp = 0
		}
		// Properly assign name and icon from NullString
		security.Name = name.String
		security.Icon = icon.String
		securities = append(securities, security)
	}
	return securities, nil
}

type GetTickerDetailsArgs struct {
	SecurityId int    `json:"securityId"`
	Ticker     string `json:"ticker,omitempty"`
	Timestamp  int64  `json:"timestamp,omitempty"`
}

type GetTickerMenuDetailsResults struct {
	Ticker                      string          `json:"ticker"`
	Name                        sql.NullString  `json:"name"`
	Market                      sql.NullString  `json:"market"`
	Locale                      sql.NullString  `json:"locale"`
	PrimaryExchange             sql.NullString  `json:"primary_exchange"`
	Active                      string          `json:"active"`
	MarketCap                   sql.NullFloat64 `json:"market_cap"`
	Description                 sql.NullString  `json:"description"`
	Logo                        sql.NullString  `json:"logo"`
	Icon                        sql.NullString  `json:"icon"`
	ShareClassSharesOutstanding sql.NullInt64   `json:"share_class_shares_outstanding"`
	Industry                    sql.NullString  `json:"industry"`
	Sector                      sql.NullString  `json:"sector"`
	TotalShares                 sql.NullInt64   `json:"totalShares"`
}

func GetTickerMenuDetails(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTickerDetailsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Modified query to handle NULL market_cap
	query := `
		SELECT 
			ticker,
			NULLIF(name, '') as name,
			NULLIF(market, '') as market,
			NULLIF(locale, '') as locale,
			NULLIF(primary_exchange, '') as primary_exchange,
			CASE 
				WHEN maxDate IS NULL THEN 'Now'
				ELSE to_char(maxDate, 'YYYY-MM-DD')
			END as active,
			NULLIF(market_cap, 0),  -- This will convert 0 to NULL
			NULLIF(description, '') as description,
			NULLIF(logo, '') as logo,
			NULLIF(icon, '') as icon,
			share_class_shares_outstanding,
			NULLIF(industry, '') as industry,
			NULLIF(sector, '') as sector,
			total_shares
		FROM securities 
		WHERE securityId = $1 AND (maxDate IS NULL OR maxDate = (
			SELECT MAX(maxDate) 
			FROM securities 
			WHERE securityId = $1
		))`

	var results GetTickerMenuDetailsResults
	err := conn.DB.QueryRow(context.Background(), query, args.SecurityId).Scan(
		&results.Ticker,
		&results.Name,
		&results.Market,
		&results.Locale,
		&results.PrimaryExchange,
		&results.Active,
		&results.MarketCap,
		&results.Description,
		&results.Logo,
		&results.Icon,
		&results.ShareClassSharesOutstanding,
		&results.Industry,
		&results.Sector,
		&results.TotalShares,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker details: %v", err)
	}

	// Create a map to store the results and handle NULL values
	response := map[string]interface{}{
		"ticker":                         results.Ticker,
		"name":                           results.Name.String,
		"market":                         results.Market.String,
		"locale":                         results.Locale.String,
		"primary_exchange":               results.PrimaryExchange.String,
		"active":                         results.Active,
		"market_cap":                     nil,
		"description":                    results.Description.String,
		"logo":                           results.Logo.String,
		"icon":                           results.Icon.String,
		"share_class_shares_outstanding": nil,
		"industry":                       results.Industry.String,
		"sector":                         results.Sector.String,
		"totalShares":                    nil,
	}

	// Only include market_cap if it's valid
	if results.MarketCap.Valid {
		response["market_cap"] = results.MarketCap.Float64
	}

	// Only include totalShares if it's valid
	if results.TotalShares.Valid {
		response["totalShares"] = results.TotalShares.Int64
	}

	// Only include share_class_shares_outstanding if it's valid
	if results.ShareClassSharesOutstanding.Valid {
		response["share_class_shares_outstanding"] = results.ShareClassSharesOutstanding.Int64
	}

	return response, nil
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
	Icon                        string  `json:"icon"`
	ShareClassSharesOutstanding int64   `json:"share_class_shares_outstanding"`
	Industry                    string  `json:"industry"`
	Sector                      string  `json:"sector"`
}

func GetTickerDetails(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTickerDetailsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	tim := time.UnixMilli(args.Timestamp)

	var ticker string
	var err error
	if args.Ticker == "" {
		ticker, err = utils.GetTicker(conn, args.SecurityId, tim)
		if err != nil {
			return nil, fmt.Errorf("failed to get ticker: %s: %v", args.Ticker, err)
		}
	} else {
		ticker = args.Ticker
	}
	details, err := utils.GetTickerDetails(conn.Polygon, ticker, "now")
	if err != nil {
		fmt.Println("failed to get ticker details: %v", err)
		return nil, nil
		//return nil, fmt.Errorf("failed to get ticker details: %v", err)
	}

	var sector, industry string
	err = conn.DB.QueryRow(context.Background(), `SELECT sector, industry from securities 
    where securityId = $1 and maxDate is NULL`, args.SecurityId).Scan(&sector, &industry)
	if err != nil {
		return nil, fmt.Errorf("01if0d %v", err)
	}

	// Helper function to fetch and encode image data
	fetchImage := func(url string) (string, error) {
		if url == "" {
			return "", nil
		}

		client := &http.Client{}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Add("Authorization", "Bearer "+conn.PolygonKey)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to fetch image: %v", err)
		}
		defer resp.Body.Close()

		imageData, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read image data: %v", err)
		}

		return base64.StdEncoding.EncodeToString(imageData), nil
	}

	// Fetch both logo and icon

	logoBase64, err := fetchImage(details.Branding.LogoURL)
	iconBase64, err := fetchImage(details.Branding.IconURL)
	/*
		if err != nil {
			return nil, fmt.Errorf("failed to fetch logo: %v", err)	const defaultIcons = {
			stock: '<svg>...</svg>', // Add your default SVG content
			fund: '<svg>...</svg>',
			futures: '<svg>...</svg>',
			forex: '<svg>...</svg>',
			indices: '<svg>...</svg>'
		} as const;*/

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
		Icon:                        iconBase64,
		Sector:                      sector,
		Industry:                    industry,
	}

	return response, nil
}

type GetSecurityClassificationsResults struct {
	Sectors    []string `json:"sectors"`
	Industries []string `json:"industries"`
}

func GetSecurityClassifications(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	// Query to get unique sectors, excluding NULL values and empty strings
	sectorQuery := `
		SELECT DISTINCT sector 
		FROM securities 
		WHERE sector IS NOT NULL 
		AND sector != '' 
		AND maxDate IS NULL 
		ORDER BY sector
	`

	industryQuery := `
		SELECT DISTINCT industry 
		FROM securities 
		WHERE industry IS NOT NULL 
		AND industry != '' 
		AND maxDate IS NULL 
		ORDER BY industry
	`

	sectorRows, err := conn.DB.Query(context.Background(), sectorQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query sectors: %v", err)
	}
	defer sectorRows.Close()

	var sectors []string
	for sectorRows.Next() {
		var sector string
		if err := sectorRows.Scan(&sector); err != nil {
			return nil, fmt.Errorf("failed to scan sector: %v", err)
		}
		sectors = append(sectors, sector)
	}

	if err := sectorRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over sectors: %v", err)
	}

	industryRows, err := conn.DB.Query(context.Background(), industryQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query industries: %v", err)
	}
	defer industryRows.Close()

	var industries []string
	for industryRows.Next() {
		var industry string
		if err := industryRows.Scan(&industry); err != nil {
			return nil, fmt.Errorf("failed to scan industry: %v", err)
		}
		industries = append(industries, industry)
	}

	if err := industryRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over industries: %v", err)
	}

	return GetSecurityClassificationsResults{
		Sectors:    sectors,
		Industries: industries,
	}, nil
}

type GetEdgarFilingsArgs struct {
	SecurityId int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"`
	From       *int64 `json:"from,omitempty"`
	To         *int64 `json:"to,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

func GetEdgarFilings(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetEdgarFilingsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Set default timestamp to now if not provided
	if args.Timestamp == 0 {
		args.Timestamp = time.Now().UnixMilli()
	}

	// Create EdgarFilingOptions from the args
	var opts *utils.EdgarFilingOptions
	if args.From != nil || args.To != nil || args.Limit > 0 {
		opts = &utils.EdgarFilingOptions{
			Limit: args.Limit,
		}

		// Convert From timestamp if provided
		if args.From != nil {
			fromTime := time.UnixMilli(*args.From)
			opts.From = &fromTime
		}

		// Convert To timestamp if provided
		if args.To != nil {
			if *args.To == 0 {
				now := time.Now()
				opts.To = &now
			} else {
				toTime := time.UnixMilli(*args.To)
				opts.To = &toTime
			}
		}
	}

	filings, err := utils.GetRecentEdgarFilings(conn, args.SecurityId, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get EDGAR filings: %v", err)
	}

	return filings, nil
}

type GetIconsArgs struct {
	Tickers []string `json:"tickers"`
}

type GetIconsResults struct {
	Ticker string `json:"ticker"`
	Icon   string `json:"icon"`
}

func GetIcons(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetIconsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Prepare a query to fetch icons for the given tickers
	query := `
		SELECT ticker, icon
		FROM securities
		WHERE ticker = ANY($1)
	`

	rows, err := conn.DB.Query(context.Background(), query, args.Tickers)
	if err != nil {
		return nil, fmt.Errorf("failed to query icons: %v", err)
	}
	defer rows.Close()

	var results []GetIconsResults
	for rows.Next() {
		var result GetIconsResults
		if err := rows.Scan(&result.Ticker, &result.Icon); err != nil {
			return nil, fmt.Errorf("failed to scan icon: %v", err)
		}
		results = append(results, result)
	}

	return results, nil
}
