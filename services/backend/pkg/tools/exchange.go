package tools

import (
	"backend/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// GetExchanges performs operations related to GetExchanges functionality.
func GetExchanges(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	url := fmt.Sprintf("https://api.polygon.io/v3/reference/exchanges?asset_class=stocks&apiKey=%s", conn.PolygonKey)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("w0ig00 %v", err)
	}
	defer resp.Body.Close()
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
