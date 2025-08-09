package polygon

import (
	"backend/internal/data"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Exchange represents a structure for handling Exchange data.
type Exchange struct {
	ID  int    `json:"id"`
	MIC string `json:"mic"`
}

// Response represents a structure for handling Response data.
type Response struct {
	Results []Exchange `json:"results"`
}

// GetExchanges queries Polygon's reference endpoint for equity exchanges and
// returns a map of exchange ID to MIC code using the provided connection's API key.
func GetExchanges(conn *data.Conn) (map[int]string, error) {
	baseURL := "https://api.polygon.io/v3/reference/exchanges"

	// Create URL with query parameters using url.Parse and url.Values
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("w0ig01 invalid URL: %v", err)
	}

	params := url.Values{}
	params.Add("asset_class", "stocks")
	params.Add("apiKey", conn.PolygonKey)
	parsedURL.RawQuery = params.Encode()

	// Make the request with the safely constructed URL
	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return nil, fmt.Errorf("w0ig00 %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("k0if200i %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("i10i0 %v", err)
	}
	var apiResponse Response
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("dkn0q0 %v", err)
	}
	exchangeMap := make(map[int]string)
	for _, exchange := range apiResponse.Results {
		exchangeMap[exchange.ID] = exchange.MIC
	}
	return exchangeMap, nil
}
