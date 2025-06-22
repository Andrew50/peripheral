package helpers

import (
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/postgres"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

// GetCurrentTickerArgs represents a structure for handling GetCurrentTickerArgs data.
type GetCurrentTickerArgs struct {
	SecurityID int `json:"securityId"`
}

// GetCurrentTicker performs operations related to GetCurrentTicker functionality.
func GetCurrentTicker(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetCurrentTickerArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("di1n0fni0: %v", err)
	}
	var ticker string
	err := conn.DB.QueryRow(context.Background(), "SELECT ticker FROM securities WHERE securityid=$1 AND maxDate is NULL", args.SecurityID).Scan(&ticker)
	if err == pgx.ErrNoRows {
		return "delisted", nil
	} else if err != nil {
		return nil, fmt.Errorf("k01n0v0e: %v", err)
	}
	return ticker, nil
}

// GetMarketCapArgs represents a structure for handling GetMarketCapArgs data.
type GetMarketCapArgs struct {
	Ticker string `json:"ticker"`
}

// GetMarketCapResults represents a structure for handling GetMarketCapResults data.
type GetMarketCapResults struct {
	MarketCap int64 `json:"marketCap"`
}

// GetMarketCap performs operations related to GetMarketCap functionality.
func GetMarketCap(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetMarketCapArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("di1n0fni0: %v", err)
	}

	details, err := polygon.GetTickerDetails(conn.Polygon, args.Ticker, "now")
	if err != nil {
		return nil, fmt.Errorf("k01n0v0e: %v", err)
	}

	if details.MarketCap == 0 {
		return GetMarketCapResults{MarketCap: 0}, nil
	}

	return GetMarketCapResults{MarketCap: int64(details.MarketCap)}, nil
}

// GetPrevCloseArgs represents a structure for handling GetPrevCloseArgs data.
type GetPrevCloseArgs struct {
	SecurityID int `json:"securityId"`
}

// PolygonBar represents a structure for handling PolygonBar data.
type PolygonBar struct {
	Close float64 `json:"close"`
}

// GetPrevClose performs operations related to GetPrevClose functionality.
func GetPrevClose(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetPrevCloseArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("getPrevClose invalid args: %v", err)
	}
	currentDay := time.Now()
	// Start at the given timestamp and subtract a day until a valid close is found
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
		err := conn.DB.QueryRow(context.Background(), query, args.SecurityID, date).Scan(&ticker)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve ticker: %v", err)
		}

		// Make a request to Polygon's API for that date and ticker
		baseURL := "https://api.polygon.io/v1/open-close"

		// Create URL with query parameters using url.Parse and url.Values
		parsedURL, err := url.Parse(baseURL + "/" + ticker + "/" + date)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %v", err)
		}

		params := url.Values{}
		params.Add("adjusted", "true")
		params.Add("apiKey", conn.PolygonKey)
		parsedURL.RawQuery = params.Encode()

		// Validate the URL is for the correct domain
		finalURL := parsedURL.String()
		if !strings.HasPrefix(finalURL, "https://api.polygon.io/") {
			return nil, fmt.Errorf("invalid API URL domain")
		}

		// Make the request with the safely constructed URL using http.Client
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		req, err := http.NewRequest("GET", finalURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		resp, err := client.Do(req)
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
			////fmt.Println(currentDay)
			return bar.Close, nil
		}

		// If not a valid market day (e.g., holiday or no trading), go back one day
		currentDay = currentDay.AddDate(0, 0, -1)
		daysChecked++
	}
	return nil, fmt.Errorf("dn10vn20")

}

type GetPopularTickersResults struct {
	SecurityID int    `json:"securityId"`
	Ticker     string `json:"ticker"`
	Timestamp  int64  `json:"timestamp"`
	Icon       string `json:"icon"`
	Name       string `json:"name"`
}

func GetPopularTickers(conn *data.Conn, _ json.RawMessage) (interface{}, error) {
	// Get the 5 most popular tickers based on chart queries in the last 24 hours
	query := `
	WITH popular_securities AS (
		SELECT 
			securityid,
			COUNT(*) as query_count
		FROM chart_queries 
		WHERE timestamp = 0 
			AND created_at >= NOW() - INTERVAL '24 hours'
		GROUP BY securityid
		ORDER BY query_count DESC
		LIMIT 5
	)
	SELECT 
		s.securityid,
		s.ticker,
		COALESCE(s.icon, '') as icon,
		COALESCE(s.name, '') as name
	FROM popular_securities ps
	JOIN securities s ON ps.securityid = s.securityid
	WHERE s.maxDate IS NULL  -- only active securities
	ORDER BY ps.query_count DESC`

	rows, err := conn.DB.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query popular tickers: %v", err)
	}
	defer rows.Close()

	var results []GetPopularTickersResults
	for rows.Next() {
		var result GetPopularTickersResults
		if err := rows.Scan(&result.SecurityID, &result.Ticker, &result.Icon, &result.Name); err != nil {
			return nil, fmt.Errorf("failed to scan popular ticker: %v", err)
		}
		// Set timestamp to 0 since we're filtering by timestamp = 0
		result.Timestamp = 0
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over popular tickers: %v", err)
	}

	return results, nil
}

// GetSecurityFromTickerArgs represents a structure for handling GetSecurityFromTickerArgs data.
type GetSecurityFromTickerArgs struct {
	Ticker string `json:"ticker"`
}

// GetSecurityFromTickerResults represents a structure for handling GetSecurityFromTickerResults data.
type GetSecurityFromTickerResults struct {
	SecurityID int    `json:"securityId"`
	Ticker     string `json:"ticker"`
	Timestamp  int64  `json:"timestamp"`
	Icon       string `json:"icon"`
	Name       string `json:"name"`
}

// GetSecuritiesFromTicker runs fuzzy finding to a user's input and returns similar tickers
func GetSecuritiesFromTicker(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args GetSecurityFromTickerArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("getAnnotations invalid args: %v", err)
	}

	// Clean and prepare the search query
	query := strings.ToUpper(strings.TrimSpace(args.Ticker))

	// Modified query to properly handle name and icon and prioritize active securities
	sqlQuery := `
	WITH ranked_results AS (
		WITH normalized AS (
			SELECT 
				securityId, 
				ticker,
				NULLIF(name, '') as name,
				NULLIF(icon, '') as icon, 
				maxDate,
				UPPER(ticker) as ticker_upper,
				UPPER(COALESCE(name, '')) as name_upper,
				REPLACE(UPPER(ticker), '.', '') as ticker_norm,
				REPLACE(UPPER(COALESCE(name, '')), '.', '') as name_norm
			FROM securities s
			WHERE maxDate IS NULL
		)
		SELECT DISTINCT ON (ticker) 
			securityId, 
			ticker,
			name,
			icon, 
			maxDate,
			CASE 
				WHEN ticker_upper = UPPER($1) OR ticker_norm = REPLACE(UPPER($1), '.', '') THEN 1
				WHEN name_upper = UPPER($1) OR name_norm = REPLACE(UPPER($1), '.', '') THEN 2
				WHEN ticker_upper LIKE UPPER($1) || '%' OR ticker_norm LIKE REPLACE(UPPER($1), '.', '') || '%' THEN 3
				WHEN name_upper LIKE UPPER($1) || '%' OR name_norm LIKE REPLACE(UPPER($1), '.', '') || '%' THEN 4
				WHEN ticker_upper LIKE '%' || UPPER($1) || '%' OR ticker_norm LIKE '%' || REPLACE(UPPER($1), '.', '') || '%' THEN 5
				WHEN name_upper LIKE '%' || UPPER($1) || '%' OR name_norm LIKE '%' || REPLACE(UPPER($1), '.', '') || '%' THEN 6
				ELSE 7
			END as match_type,
			GREATEST(
				similarity(ticker_upper, UPPER($1)),
				similarity(ticker_norm, REPLACE(UPPER($1), '.', '')),
				COALESCE(similarity(name_upper, UPPER($1)), 0),
				COALESCE(similarity(name_norm, REPLACE(UPPER($1), '.', '')), 0)
			) as sim_score
		FROM normalized
		WHERE (
			ticker_upper = UPPER($1) OR
			ticker_norm = REPLACE(UPPER($1), '.', '') OR
			ticker_upper LIKE UPPER($1) || '%' OR 
			ticker_norm LIKE REPLACE(UPPER($1), '.', '') || '%' OR
			ticker_upper LIKE '%' || UPPER($1) || '%' OR
			ticker_norm LIKE '%' || REPLACE(UPPER($1), '.', '') || '%' OR
			similarity(ticker_upper, UPPER($1)) > 0.3 OR
			similarity(ticker_norm, REPLACE(UPPER($1), '.', '')) > 0.3 OR
			name_upper = UPPER($1) OR
			name_norm = REPLACE(UPPER($1), '.', '') OR
			name_upper LIKE UPPER($1) || '%' OR 
			name_norm LIKE REPLACE(UPPER($1), '.', '') || '%' OR
			name_upper LIKE '%' || UPPER($1) || '%' OR
			name_norm LIKE '%' || REPLACE(UPPER($1), '.', '') || '%' OR
			similarity(name_upper, UPPER($1)) > 0.3 OR
			similarity(name_norm, REPLACE(UPPER($1), '.', '')) > 0.3
		)
		ORDER BY ticker, maxDate DESC NULLS FIRST
	)
	SELECT securityId, ticker, name, icon, maxDate
	FROM ranked_results
	ORDER BY match_type, sim_score DESC
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
		if err := rows.Scan(&security.SecurityID, &security.Ticker, &name, &icon, &timestamp); err != nil {
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
func GetAgentTickerMenuDetails(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTickerDetailsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	details, err := GetTickerMenuDetails(conn, rawArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker details: %v", err)
	}
	detailsMap := details.(map[string]interface{})

	// Helper function to safely convert string to sql.NullString
	toNullString := func(val interface{}) sql.NullString {
		if val == nil {
			return sql.NullString{Valid: false}
		}
		if str, ok := val.(string); ok {
			return sql.NullString{String: str, Valid: str != ""}
		}
		if nullStr, ok := val.(sql.NullString); ok {
			return nullStr
		}
		return sql.NullString{Valid: false}
	}

	// Helper function to safely convert to sql.NullFloat64
	toNullFloat64 := func(val interface{}) sql.NullFloat64 {
		if val == nil {
			return sql.NullFloat64{Valid: false}
		}
		if f, ok := val.(float64); ok {
			return sql.NullFloat64{Float64: f, Valid: true}
		}
		if nullFloat, ok := val.(sql.NullFloat64); ok {
			return nullFloat
		}
		return sql.NullFloat64{Valid: false}
	}

	// Helper function to safely convert to sql.NullInt64
	toNullInt64 := func(val interface{}) sql.NullInt64 {
		if val == nil {
			return sql.NullInt64{Valid: false}
		}
		if i, ok := val.(int64); ok {
			return sql.NullInt64{Int64: i, Valid: true}
		}
		if nullInt, ok := val.(sql.NullInt64); ok {
			return nullInt
		}
		return sql.NullInt64{Valid: false}
	}

	return GetTickerMenuDetailsResults{
		Ticker:                      detailsMap["ticker"].(string),
		Name:                        toNullString(detailsMap["name"]),
		PrimaryExchange:             toNullString(detailsMap["primary_exchange"]),
		MarketCap:                   toNullFloat64(detailsMap["market_cap"]),
		ShareClassSharesOutstanding: toNullInt64(detailsMap["share_class_shares_outstanding"]),
		Industry:                    toNullString(detailsMap["industry"]),
		Sector:                      toNullString(detailsMap["sector"]),
	}, nil
}

// GetTickerDetailsArgs represents a structure for handling GetTickerDetailsArgs data.
type GetTickerDetailsArgs struct {
	SecurityID int    `json:"securityId"`
	Ticker     string `json:"ticker,omitempty"`
	Timestamp  int64  `json:"timestamp,omitempty"`
}

// GetTickerMenuDetailsResults represents a structure for handling GetTickerMenuDetailsResults data.
type GetTickerMenuDetailsResults struct {
	Ticker                      string          `json:"ticker"`
	Name                        sql.NullString  `json:"name"`
	Market                      sql.NullString  `json:"market,omitempty"`
	Locale                      sql.NullString  `json:"locale,omitempty"`
	PrimaryExchange             sql.NullString  `json:"primary_exchange"`
	Active                      string          `json:"active,omitempty"`
	MarketCap                   sql.NullFloat64 `json:"market_cap"`
	Description                 sql.NullString  `json:"description,omitempty"`
	Logo                        sql.NullString  `json:"logo,omitempty"`
	Icon                        sql.NullString  `json:"icon,omitempty"`
	ShareClassSharesOutstanding sql.NullInt64   `json:"share_class_shares_outstanding"`
	Industry                    sql.NullString  `json:"industry"`
	Sector                      sql.NullString  `json:"sector"`
	TotalShares                 sql.NullInt64   `json:"totalShares"`
}

// GetTickerMenuDetails performs operations related to GetTickerMenuDetails functionality.
func GetTickerMenuDetails(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTickerDetailsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Modified query to handle NULL market_cap and missing columns
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
			CASE 
				WHEN EXISTS (
					SELECT 1 FROM information_schema.columns 
					WHERE table_name = 'securities' AND column_name = 'total_shares'
				) 
				THEN (SELECT total_shares FROM securities WHERE securityId = $1 LIMIT 1)
				ELSE 0
			END as total_shares
		FROM securities 
		WHERE securityId = $1 AND (maxDate IS NULL OR maxDate = (
			SELECT MAX(maxDate) 
			FROM securities 
			WHERE securityId = $1
		))
		ORDER BY maxDate IS NULL DESC, maxDate DESC NULLS FIRST
		LIMIT 1`

	var results GetTickerMenuDetailsResults
	err := conn.DB.QueryRow(context.Background(), query, args.SecurityID).Scan(
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

// TickerDetailsResponse represents a structure for handling TickerDetailsResponse data.
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

// GetTickerDetails performs operations related to GetTickerDetails functionality.
func GetTickerDetails(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTickerDetailsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	tim := time.UnixMilli(args.Timestamp)

	var ticker string
	var err error
	if args.Ticker == "" {
		ticker, err = postgres.GetTicker(conn, args.SecurityID, tim)
		if err != nil {
			return nil, fmt.Errorf("failed to get ticker: %s: %v", args.Ticker, err)
		}
	} else {
		ticker = args.Ticker
	}
	details, err := polygon.GetTickerDetails(conn.Polygon, ticker, "now")
	if err != nil {
		////fmt.Printf("failed to get ticker details: %v\n", err)
		return nil, nil
		//return nil, fmt.Errorf("failed to get ticker details: %v", err)
	}

	var sector, industry string
	err = conn.DB.QueryRow(context.Background(), `SELECT sector, industry from securities 
    where securityId = $1 and maxDate is NULL`, args.SecurityID).Scan(&sector, &industry)
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

		// Check if the response status is OK
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to fetch image, status code: %d", resp.StatusCode)
		}

		imageData, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read image data: %v", err)
		}

		// If no image data was returned, return empty string
		if len(imageData) == 0 {
			return "", fmt.Errorf("empty image data received")
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			// Try to detect content type from image data
			contentType = http.DetectContentType(imageData)

			// If still empty, default to a safe type based on URL extension
			if contentType == "" || contentType == "application/octet-stream" {
				if strings.HasSuffix(strings.ToLower(url), ".svg") {
					contentType = "image/svg+xml"
				} else if strings.HasSuffix(strings.ToLower(url), ".png") {
					contentType = "image/png"
				} else {
					contentType = "image/jpeg"
				}
			}
		}

		// Ensure the content type doesn't already contain a data URL prefix
		if strings.HasPrefix(contentType, "data:") {
			return "", fmt.Errorf("invalid content type: %s", contentType)
		}

		base64Data := base64.StdEncoding.EncodeToString(imageData)

		// Check if base64Data already contains a data URL prefix to prevent duplication
		if strings.HasPrefix(base64Data, "data:") {
			return base64Data, nil
		}

		return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
	}

	// Fetch both logo and icon with proper error handling
	logoBase64, logoErr := fetchImage(details.Branding.LogoURL)
	if logoErr != nil {
		////fmt.Printf("Warning: Failed to fetch logo: %v\n", logoErr)
		logoBase64 = "" // Set to empty string on error
	}

	iconBase64, iconErr := fetchImage(details.Branding.IconURL)
	if iconErr != nil {
		////fmt.Printf("Warning: Failed to fetch icon: %v\n", iconErr)
		iconBase64 = "" // Set to empty string on error
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
		Icon:                        iconBase64,
		Sector:                      sector,
		Industry:                    industry,
	}

	return response, nil
}

// GetSecurityClassificationsResults represents a structure for handling GetSecurityClassificationsResults data.
type GetSecurityClassificationsResults struct {
	Sectors    []string `json:"sectors"`
	Industries []string `json:"industries"`
}

// GetSecurityClassifications performs operations related to GetSecurityClassifications functionality.
func GetSecurityClassifications(conn *data.Conn, _ json.RawMessage) (interface{}, error) {
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

// GetIconsArgs represents a structure for handling GetIconsArgs data.
type GetIconsArgs struct {
	Tickers []string `json:"tickers"`
}

// GetIconsResults represents a structure for handling GetIconsResults data.
type GetIconsResults struct {
	Ticker string `json:"ticker"`
	Icon   string `json:"icon"`
}

// GetIcons performs operations related to GetIcons functionality.
func GetIcons(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetIconsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Filter out any nil or empty ticker values
	var validTickers []string
	for _, ticker := range args.Tickers {
		if ticker != "" {
			validTickers = append(validTickers, ticker)
		}
	}

	// If no valid tickers, return empty result
	if len(validTickers) == 0 {
		return []GetIconsResults{}, nil
	}

	// Prepare a query to fetch icons for the given tickers
	query := `
		WITH latest_securities AS (
			SELECT DISTINCT ON (ticker) ticker, icon
			FROM securities
			WHERE ticker = ANY($1) AND ticker IS NOT NULL AND ticker != ''
			ORDER BY ticker, maxDate DESC NULLS FIRST
		)
		SELECT ticker, CASE 
			WHEN icon IS NULL OR icon = '' THEN ''
			ELSE icon 
		END as icon
		FROM latest_securities
	`

	rows, err := conn.DB.Query(context.Background(), query, validTickers)
	if err != nil {
		return nil, fmt.Errorf("failed to query icons: %v", err)
	}
	defer rows.Close()

	// Create a map to store found icons by ticker
	foundIcons := make(map[string]string)

	// Scan results into the map
	for rows.Next() {
		var nullableTicker, nullableIcon sql.NullString
		if err := rows.Scan(&nullableTicker, &nullableIcon); err != nil {
			return nil, fmt.Errorf("failed to scan icon: %v", err)
		}

		// Skip invalid tickers
		if !nullableTicker.Valid || nullableTicker.String == "" {
			continue
		}

		// Add to the found icons map
		foundIcons[nullableTicker.String] = nullableIcon.String
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over icons: %v", err)
	}

	// Create results for all requested tickers, even if not found in DB
	var results []GetIconsResults
	for _, ticker := range validTickers {
		result := GetIconsResults{
			Ticker: ticker,
			Icon:   "", // Default to empty icon
		}

		// If we found an icon for this ticker, use it
		if icon, found := foundIcons[ticker]; found {
			result.Icon = icon
		}

		results = append(results, result)
	}

	return results, nil
}

type GetCurrentSecurityIDArgs struct {
	Ticker string `json:"ticker"`
}

// GetCurrentSecurityID performs operations related to GetSecurityID functionality.
func GetCurrentSecurityID(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetCurrentSecurityIDArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	var securityID int
	err := conn.DB.QueryRow(context.Background(), "SELECT securityId from securities where ticker = $1 and maxDate is NULL", args.Ticker).Scan(&securityID)
	if err != nil {
		return 0, fmt.Errorf("43333ngb %v", err)
	}
	return securityID, nil
}

type GetSecurityIDFromTickerTimestampArgs struct {
	Ticker    string `json:"ticker"`
	Timestamp int64  `json:"timestamp"`
}

type GetSecurityIDFromTickerTimestampResults struct {
	SecurityID int `json:"securityId"`
}

func GetSecurityIDFromTickerTimestamp(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	var args GetSecurityIDFromTickerTimestampArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	var timestamp time.Time
	if args.Timestamp == 0 {
		timestamp = time.Now()
	} else {
		timestamp = time.Unix(args.Timestamp/1000, (args.Timestamp%1000)*1e6)
	}
	securityID, err := postgres.GetSecurityID(conn, args.Ticker, timestamp)
	if err != nil {
		return nil, fmt.Errorf("error getting security ID: %v", err)
	}
	return GetSecurityIDFromTickerTimestampResults{SecurityID: securityID}, nil
}

type GetTickerDailySnapshotArgs struct {
	SecurityID int `json:"securityId"`
}
type GetTickerDailySnapshotResults struct {
	Ticker             string  `json:"ticker"`
	LastBid            float64 `json:"lastBid,omitempty"`
	LastAsk            float64 `json:"lastAsk,omitempty"`
	LastTradePrice     float64 `json:"lastTradePrice"`
	TodayChange        float64 `json:"todayChange"`
	TodayChangePercent float64 `json:"todayChangePercent"`
	Timestamp          int64   `json:"timestamp"`
	Volume             float64 `json:"volume"`
	Vwap               float64 `json:"vwap"`
	TodayOpen          float64 `json:"todayOpen"`
	TodayHigh          float64 `json:"todayHigh"`
	TodayLow           float64 `json:"todayLow"`
	TodayClose         float64 `json:"todayClose"`
	PreviousClose      float64 `json:"previousClose"`
}

func GetTickerDailySnapshot(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetTickerDailySnapshotArgs

	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	ticker, err := postgres.GetTicker(conn, args.SecurityID, time.Now())

	if err != nil {
		return nil, fmt.Errorf("error getting ticker: %v", err)
	}

	res, err := polygon.GetPolygonTickerSnapshot(context.Background(), conn.Polygon, ticker)
	if err != nil {
		return nil, fmt.Errorf("error getting ticker snapshot: %v", err)
	}
	snapshot := res.Snapshot
	var results GetTickerDailySnapshotResults
	results.LastTradePrice = snapshot.LastTrade.Price
	currPrice := snapshot.Day.Close
	lastClose := snapshot.PrevDay.Close
	results.TodayChange = math.Round((currPrice-lastClose)*100) / 100
	results.TodayChangePercent = math.Round(((currPrice-lastClose)/lastClose)*100*100) / 100
	results.Volume = snapshot.Day.Volume
	results.TodayOpen = snapshot.Day.Open
	results.TodayHigh = snapshot.Day.High
	results.TodayLow = snapshot.Day.Low
	results.TodayClose = snapshot.Day.Close
	results.PreviousClose = lastClose
	results.Ticker = ticker
	////fmt.Println(results)
	return results, nil
}

type GetAllTickerSnapshotResults struct {
	Tickers []GetTickerDailySnapshotResults `json:"tickers"`
}

func GetAllTickerSnapshots(conn *data.Conn, _ int, _ json.RawMessage) (interface{}, error) {
	res, err := polygon.GetPolygonAllTickerSnapshots(context.Background(), conn.Polygon)
	if err != nil {
		return nil, fmt.Errorf("error getting all ticker snapshots: %v", err)
	}
	response := res.Tickers
	var results GetAllTickerSnapshotResults
	for _, snapshot := range response {
		var ticker GetTickerDailySnapshotResults
		ticker.LastTradePrice = snapshot.LastTrade.Price
		ticker.TodayChange = snapshot.Day.Close - snapshot.PrevDay.Close
		ticker.TodayChangePercent = ((snapshot.Day.Close - snapshot.PrevDay.Close) / snapshot.PrevDay.Close) * 100
		ticker.Timestamp = int64(time.Time(snapshot.Updated).Unix())
		ticker.Volume = snapshot.Day.Volume
		ticker.Vwap = snapshot.Day.VolumeWeightedAverage
		ticker.TodayOpen = snapshot.Day.Open
		ticker.TodayHigh = snapshot.Day.High
		ticker.TodayLow = snapshot.Day.Low
		ticker.TodayClose = snapshot.Day.Close
		results.Tickers = append(results.Tickers, ticker)
	}
	return results, nil
}

type GetFilteredTickerSnapshotArgs struct {
	SecurityID int `json:"securityId"`
	Start      int `json:"start"`
	End        int `json:"end"`
}
type GetFilteredTickerSnapshotResults struct {
	Snapshots []GetTickerDailySnapshotResults `json:"snapshots"`
}

type GetLastPriceArgs struct {
	Ticker string `json:"ticker"`
}

func GetLastPrice(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetLastPriceArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	trade, err := polygon.GetLastTrade(conn.Polygon, args.Ticker, true)
	if err != nil {
		return nil, fmt.Errorf("error getting last trade: %v", err)
	}
	roundedPrice := math.Round(trade.Price*100) / 100
	return roundedPrice, nil
}
