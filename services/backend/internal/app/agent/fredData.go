package agent

import (
	"backend/internal/data"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type GetFredSeriesArgs struct {
	Keyword string `json:"keyword"`
}

type GetFredSeriesResults struct {
	Series []FredAPISearchSeries `json:"series"`
}

type FredAPISearchSeries struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Units            string `json:"units"`
	LastUpdated      string `json:"last_updated"`
	ObservationStart string `json:"observation_start"`
	ObservationEnd   string `json:"observation_end"`
}
type FredAPISearchResponse struct {
	Series []FredAPISearchSeries `json:"seriess"`
}

func GetFredSeries(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetFredSeriesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}
	baseURL := "https://api.stlouisfed.org/fred/series/search"
	params := url.Values{}
	params.Add("search_text", args.Keyword)
	params.Add("api_key", conn.FredAPIKey)
	params.Add("file_type", "json")
	params.Add("limit", "5")
	params.Add("order_by", "popularity")

	fullURL := baseURL + "?" + params.Encode()

	client := &http.Client{Timeout: 30 * time.Second}
	var resp *http.Response
	var err error
	maxRetries := 3
	retryDelay := 1 * time.Second
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}
		req.Header.Set("User-Agent", "peripheral.io Peripheral Agent")
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error making request: %w", err)
		}
		if resp.StatusCode == 429 {
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("error closing response body: %v", err)
			}
			waitTime := retryDelay * time.Duration(1<<attempt)
			time.Sleep(waitTime)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("FRED API returned status %d and error closing response: %v", resp.StatusCode, err)
			}
			previewLength := 100
			if len(body) < previewLength {
				previewLength = len(body)
			}
			return nil, fmt.Errorf("FRED API returned status %d: %s", resp.StatusCode, string(body[:previewLength]))
		}
		break
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("FRED API rate limit exceeded after %d retries", maxRetries)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	var fredResponse FredAPISearchResponse
	if err := json.Unmarshal(body, &fredResponse); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}
	var results GetFredSeriesResults
	results.Series = fredResponse.Series
	return results, nil
}

type GetFredSeriesDataArgs struct {
	SeriesID  string `json:"series_id"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
	Units     string `json:"units,omitempty"`
	Frequency string `json:"frequency,omitempty"`
}

// FredObservation represents a single observation from the FRED API
type FredObservation struct {
	Date  string `json:"date"`
	Value string `json:"value"`
}

// FredApiResponse represents the complete response from the FRED API
type FredAPIObservationsResponse struct {
	RealtimeStart    string            `json:"realtime_start"`
	RealtimeEnd      string            `json:"realtime_end"`
	ObservationStart string            `json:"observation_start"`
	ObservationEnd   string            `json:"observation_end"`
	Units            string            `json:"units"`
	OrderBy          string            `json:"order_by"`
	SortOrder        string            `json:"sort_order"`
	Count            int               `json:"count"`
	Offset           int               `json:"offset"`
	Observations     []FredObservation `json:"observations"`
}
type GetFredSeriesDataResults struct {
	Dates     []string `json:"dates"`
	Values    []string `json:"values"`
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	Units     string   `json:"units"`
	Count     int      `json:"count"`
}

func GetFredSeriesData(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetFredSeriesDataArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, err
	}

	fredKey := conn.FredAPIKey
	if fredKey == "" {
		return nil, fmt.Errorf("fred api key not found")
	}

	// Build the URL with query parameters
	baseURL := "https://api.stlouisfed.org/fred/series/observations"
	params := url.Values{}
	params.Add("series_id", args.SeriesID)
	params.Add("api_key", fredKey)
	params.Add("file_type", "json")

	// Add optional parameters if provided
	if args.StartDate != "" {
		params.Add("observation_start", args.StartDate)
	}
	if args.EndDate != "" {
		params.Add("observation_end", args.EndDate)
	}
	if args.Units != "" {
		params.Add("units", args.Units)
	}
	if args.Frequency != "" {
		params.Add("frequency", args.Frequency)
	}

	fullURL := baseURL + "?" + params.Encode()

	// Make HTTP request with retry logic for rate limiting
	client := &http.Client{Timeout: 30 * time.Second}
	var resp *http.Response
	var err error
	maxRetries := 3
	retryDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}

		// Set User-Agent header as required by many APIs
		req.Header.Set("User-Agent", "peripheral.io Peripheral Agent")

		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error making request: %w", err)
		}

		// Check for rate limiting (429)
		if resp.StatusCode == 429 {
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("error closing response body: %v", err)
			}

			// Exponential backoff
			waitTime := retryDelay * time.Duration(1<<attempt)
			time.Sleep(waitTime)
			continue
		}

		// Check for other non-success status codes
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("FRED API returned status %d and error closing response: %v", resp.StatusCode, err)
			}
			previewLength := 100
			if len(body) < previewLength {
				previewLength = len(body)
			}
			return nil, fmt.Errorf("FRED API returned status %d: %s", resp.StatusCode, string(body[:previewLength]))
		}

		// Success
		break
	}

	// Check if all retries failed
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("FRED API rate limit exceeded after %d retries", maxRetries)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Parse JSON response
	var fredResponse FredAPIObservationsResponse
	if err := json.Unmarshal(body, &fredResponse); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}

	// Convert observations to separate date and value arrays
	var results GetFredSeriesDataResults
	results.Dates = make([]string, len(fredResponse.Observations))
	results.Values = make([]string, len(fredResponse.Observations))

	for i, obs := range fredResponse.Observations {
		results.Dates[i] = obs.Date
		results.Values[i] = obs.Value
	}

	results.StartDate = fredResponse.ObservationStart
	results.EndDate = fredResponse.ObservationEnd
	results.Units = fredResponse.Units
	results.Count = fredResponse.Count

	// Return the parsed data
	return results, nil
}
