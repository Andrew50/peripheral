package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type GetScreensaversResults struct {
	Ticker     string `json:"ticker"`
	SecurityId int    `json:"securityId"`
	Timestamp  int64  `json:"timestamp"`
	Timeframe  string `json:"timeframe"`
}

type PolygonTicker struct {
	Ticker string `json:"ticker"`
}

type PolygonSnapshot struct {
	Tickers []PolygonTicker `json:"tickers"`
}

// Fetch the snapshot from Polygon.io, attaching the API key
func fetchPolygonSnapshot(endpoint string, apiKey string) ([]string, error) {
	// Append the API key to the endpoint
	fullEndpoint := fmt.Sprintf("%s?apiKey=%s", endpoint, apiKey)

	// Make the request to Polygon.io
	resp, err := http.Get(fullEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Polygon snapshot: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
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

func GetScreensavers(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
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
    fmt.Println(losers)

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

	rows, err := conn.DB.Query(context.Background(), query, tickers)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	var results []GetScreensaversResults
	for rows.Next() {
		var result GetScreensaversResults
		err := rows.Scan(&result.Ticker, &result.SecurityId)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		result.Timestamp = 0  // Set the timestamp to zero
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return results, nil
}

