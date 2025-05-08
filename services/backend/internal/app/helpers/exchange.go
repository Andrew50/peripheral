package helpers

import (
	"backend/internal/data"
	"encoding/json"
    "backend/internal/data/polygon"
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
func GetExchanges(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    exchangeMap, err := polygon.GetExchanges(conn)
	return exchangeMap, err
}
