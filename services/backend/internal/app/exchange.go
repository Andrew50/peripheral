package tools

import (
	"backend/internal/data"
	"encoding/json"
    "backend/internal/data/polygon"
)

// GetExchanges performs operations related to GetExchanges functionality.
func GetExchanges(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    exchangeMap, err := polygon.GetExchanges(conn)
	return exchangeMap, err
}
