package helpers

import (
	"backend/internal/data"
	"backend/internal/data/polygon"
	"encoding/json"
	"strings"
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
func GetExchanges(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	// Check if rawArgs is empty or not a valid JSON object
	if len(rawArgs) == 0 || !strings.HasPrefix(string(rawArgs), "{") {
		// If no specific exchange type is requested, fetch all distinct exchanges
		exchangeMap, err := polygon.GetExchanges(conn)
		return exchangeMap, err
	}
	return nil, nil
}
