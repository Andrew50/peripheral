package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"backend/internal/data"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

// GetSecurityID performs operations related to GetSecurityID functionality.
func GetSecurityID(conn *data.Conn, ticker string, timestamp time.Time) (int, error) {
	var securityID int
	err := conn.DB.QueryRow(context.Background(), "SELECT securityId from securities where ticker = $1 and minDate <= $2 and (maxDate >= $2 or maxDate is NULL)", ticker, timestamp).Scan(&securityID)
	if err != nil {
		return 0, fmt.Errorf("43333ngb %v %v %v", err, ticker, timestamp)
	}
	return securityID, nil
}

// GetTicker performs operations related to GetTicker functionality.
func GetTicker(conn *data.Conn, securityID int, timestamp time.Time) (string, error) {
	var ticker string
	err := conn.DB.QueryRow(context.Background(), "SELECT ticker from securities where securityId = $1 and minDate <= $2 and (maxDate >= $2 or maxDate is NULL)", securityID, timestamp).Scan(&ticker)
	if err != nil {
		return "", fmt.Errorf("igw0ngb %v", err)
	}
	return ticker, nil
}

// GetCIKFromTicker performs operations related to GetCIKFromTicker functionality.
func GetCIKFromTicker(conn *data.Conn, ticker string, timestamp time.Time) (int64, error) {
	var cik int64
	err := conn.DB.QueryRow(context.Background(), "SELECT cik from securities where ticker = $1 and minDate <= $2 and (maxDate >= $2 or maxDate is NULL)", ticker, timestamp).Scan(&cik)
	if err != nil {
		return 0, fmt.Errorf("3333w0ngb %v", err)
	}
	return cik, nil
}

// GetTickerNews performs operations related to GetTickerNews functionality.
func GetTickerNews(client *polygon.Client, ticker string, millisTime models.Millis, ord string, limit int, compareType models.Comparator) *iter.Iter[models.TickerNews] {
	sortOrder := models.Asc
	if ord == "desc" {
		sortOrder = models.Desc
	}
	params := models.ListTickerNewsParams{}.
		WithTicker(models.EQ, ticker).
		WithSort(models.PublishedUTC).
		WithOrder(sortOrder).
		WithLimit(limit).
		WithPublishedUTC(compareType, millisTime)
	iter := client.ListTickerNews(context.Background(), params)
	return iter
}

// GetLatestTickerNews performs operations related to GetLatestTickerNews functionality.
func GetLatestTickerNews(client *polygon.Client, ticker string, numResults int) *iter.Iter[models.TickerNews] {
	return GetTickerNews(client, ticker, models.Millis(time.Now()), "asc", numResults, models.LTE)
}

// GetPolygonRelatedTickers performs operations related to GetPolygonRelatedTickers functionality.
func GetPolygonRelatedTickers(client *polygon.Client, ticker string) ([]string, error) {
	params := &models.GetTickerRelatedCompaniesParams{
		Ticker: ticker,
	}
	res, err := client.GetTickerRelatedCompanies(context.Background(), params)
	if err != nil {
		return nil, err
	}
	relatedTickers := []string{}
	for _, relatedTicker := range res.Results {
		relatedTickers = append(relatedTickers, relatedTicker.Ticker)
	}
	return relatedTickers, nil
}

// Custom types to match the actual API response structure
type TickerEvent struct {
	Type         string                 `json:"type"`
	Date         string                 `json:"date"`
	TickerChange map[string]interface{} `json:"ticker_change,omitempty"`
}

type TickerEventResult struct {
	Name   string        `json:"name"`
	Events []TickerEvent `json:"events"`
}

type TickerEventsAPIResponse struct {
	Results struct {
		Name          string        `json:"name"`
		CompositeFigi string        `json:"composite_figi"`
		CIK           string        `json:"cik"`
		Events        []TickerEvent `json:"events"`
	} `json:"results"`
	Status    string `json:"status"`
	RequestID string `json:"request_id"`
}

// GetTickerEventsCustom bypasses the SDK to call the API directly
func GetTickerEventsCustom(_ *polygon.Client, id string, apiKey string) (TickerEventResult, error) {
	// Validate ticker ID to prevent URL manipulation
	if id == "" || len(id) > 10 || strings.ContainsAny(id, "/:?#[]@!$&'()*+,;=") {
		return TickerEventResult{}, fmt.Errorf("invalid ticker ID: %s", id)
	}

	polygonURL := fmt.Sprintf("https://api.polygon.io/vX/reference/tickers/%s/events?apiKey=%s", id, apiKey)

	// #nosec G107 - URL is constructed with validated ticker ID for Polygon API only
	resp, err := http.Get(polygonURL)
	if err != nil {
		return TickerEventResult{}, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Ticker not found, return empty results
		return TickerEventResult{}, nil
	}

	if resp.StatusCode != 200 {
		return TickerEventResult{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TickerEventResult{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResponse TickerEventsAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return TickerEventResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert to in-house format
	var results TickerEventResult
	if len(apiResponse.Results.Events) > 0 {
		// Create a single TickerEventResult with all events
		for _, event := range apiResponse.Results.Events {
			results.Events = append(results.Events, TickerEvent{
				Type:         event.Type,
				Date:         event.Date,
				TickerChange: event.TickerChange,
			})
		}
		results.Name = apiResponse.Results.Name

	}

	return results, nil
}

func GetStockSplits(client *polygon.Client, ticker string) ([]models.Split, error) {
	// Set up parameters for the splits API call
	params := models.ListSplitsParams{
		TickerEQ: &ticker,
	}.WithOrder(models.Order("desc")).WithLimit(10)

	// Execute the API call and get an iterator
	iter := client.ListSplits(context.Background(), params)

	// Collect all splits
	var splits []models.Split
	for iter.Next() {
		split := iter.Item()
		splits = append(splits, split)
	}
	// Check for errors during iteration
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("error fetching splits for %s: %w", ticker, err)
	}

	return splits, nil
}
func GetTickerFromCIK(client *polygon.Client, cik int, dateString string) (string, error) {
	params := models.ListTickersParams{}.WithCIK(cik)
	if dateString != "" {
		dt, err := time.Parse(time.DateOnly, dateString)
		if err != nil {
			return "", err
		}
		dateObj := models.Date(dt)
		params = params.WithDate(dateObj)
	}
	iter := client.ListTickers(context.Background(), params)
	for iter.Next() {
		return iter.Item().Ticker, nil
	}
	return "", fmt.Errorf("no ticker found for cik %d", cik)
}

// GetTickerEvents performs operations related to GetTickerEvents functionality.
func GetTickerEvents(client *polygon.Client, id string) ([]models.TickerEventResult, error) {
	params := &models.GetTickerEventsParams{
		ID: id,
	}
	res, err := client.VX.GetTickerEvents(context.Background(), params)
	fmt.Printf("🟢 Ticker events for %s - error: %v\n", id, err)
	fmt.Printf("🟢 Ticker events for %s - response: %+v\n", id, res)
	if err != nil {
		errStr := fmt.Sprintf("%v", err)
		// If this is a JSON unmarshaling error, it might mean the ticker doesn't have events
		// or the API response format is different. Return an empty slice instead of failing.
		if strings.Contains(errStr, "cannot unmarshal object into Go struct field GetTickerEventsResponse.results") {
			return []models.TickerEventResult{}, nil
		}
		// If this is a 404 NOT_FOUND error, the ticker doesn't exist or has no events
		if strings.Contains(errStr, "404") && strings.Contains(errStr, "NOT_FOUND") {
			return []models.TickerEventResult{}, nil
		}
		return nil, fmt.Errorf("failed to get ticker events for %s: %w", id, err)
	}
	fmt.Printf("🟢 Ticker events: %v\n", res)
	return res.Results, nil
}
