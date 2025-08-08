// Package screensaver provides functionality for fetching and managing screensaver data.
package screensaver

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GetScreensaversResults represents a structure for handling GetScreensaversResults data.
type GetScreensaversResults struct {
	Ticker     string `json:"ticker"`
	SecurityID int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"`
	Timeframe  string `json:"timeframe"`
}

// PolygonTicker represents a structure for handling PolygonTicker data.
type PolygonTicker struct {
	Ticker string `json:"ticker"`
}

// PolygonSnapshot represents a structure for handling PolygonSnapshot data.
type PolygonSnapshot struct {
	Tickers []PolygonTicker `json:"tickers"`
}

// GetInstancesByTickersArgs represents the arguments for the GetInstancesByTickers function
type GetInstancesByTickersArgs struct {
	Tickers []string `json:"tickers"`
}

// Fetch the snapshot from Polygon.io, attaching the API key
func fetchPolygonSnapshot(endpoint string, apiKey string) ([]string, error) {
	// Safely construct the URL
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %v", err)
	}
	query := parsedURL.Query()
	query.Set("apiKey", apiKey)
	parsedURL.RawQuery = query.Encode()
	fullEndpointStr := parsedURL.String()

	// Validate the URL is for an allowed domain (polygon.io)
	if !strings.HasPrefix(fullEndpointStr, "https://api.polygon.io/") {
		return nil, fmt.Errorf("invalid endpoint domain, only polygon.io is allowed")
	}

	// Make the request to Polygon.io with validated URL using http.Client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", fullEndpointStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Polygon snapshot: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("error closing response body: %v\n", err)
		}
	}()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Unmarshal the JSON into a PolygonSnapshot structure
	var snapshot PolygonSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Extract tickers from the snapshot
	var tickers []string
	for _, ticker := range snapshot.Tickers {
		tickers = append(tickers, ticker.Ticker)
	}

	return tickers, nil
}

// GetInstancesByTickers retrieves screensaver instances by a list of tickers.
func GetInstancesByTickers(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetInstancesByTickersArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error unmarshalling args: %w", err)
	}

	if len(args.Tickers) == 0 {
		return []GetScreensaversResults{}, nil
	}

	// Query the database to get securityId for the provided tickers
	query := `
		SELECT ticker, securityId
		FROM securities
		WHERE ticker = ANY($1) AND maxDate IS NULL`

	rows, err := conn.DB.Query(context.Background(), query, args.Tickers)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	var results []GetScreensaversResults
	for rows.Next() {
		var result GetScreensaversResults
		err := rows.Scan(&result.Ticker, &result.SecurityID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		result.Timestamp = 0 // Set the timestamp to zero
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return results, nil
}

// GetScreensavers retrieves snapshots of gaining and losing tickers.
func GetScreensavers(conn *data.Conn, _ int, _ json.RawMessage) (interface{}, error) {
	// Define Polygon.io endpoints for gainers and losers
	gainersEndpoint := "https://api.polygon.io/v2/snapshot/locale/us/markets/stocks/gainers"
	losersEndpoint := "https://api.polygon.io/v2/snapshot/locale/us/markets/stocks/losers"

	// Fetch gainers and losers snapshots from Polygon.io with the API key
	gainers, err := fetchPolygonSnapshot(gainersEndpoint, conn.PolygonKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gainers: %v", err)
	}
	losers, err := fetchPolygonSnapshot(losersEndpoint, conn.PolygonKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch losers: %v", err)
	}

	// Combine gainers and losers
	tickers := append(gainers, losers...)

	if len(tickers) == 0 {
		return nil, fmt.Errorf("no tickers found in the Polygon snapshots")
	}

	// Query the database to get securityId for the fetched tickers
	query := `
		SELECT ticker, securityId
		FROM securities
		WHERE ticker = ANY($1) AND maxDate IS NULL`

	rowsDB, errDB := conn.DB.Query(context.Background(), query, tickers)
	if errDB != nil {
		return nil, fmt.Errorf("failed to execute query: %v", errDB)
	}
	defer rowsDB.Close()

	var results []GetScreensaversResults
	for rowsDB.Next() {
		var result GetScreensaversResults
		errScan := rowsDB.Scan(&result.Ticker, &result.SecurityID)
		if errScan != nil {
			return nil, fmt.Errorf("failed to scan row: %v", errScan)
		}
		result.Timestamp = 0 // Set the timestamp to zero
		results = append(results, result)
	}

	if errRows := rowsDB.Err(); errRows != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", errRows)
	}

	return results, nil
}
