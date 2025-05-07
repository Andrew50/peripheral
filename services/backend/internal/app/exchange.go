package tools

import (
	"backend/internal/data"
	"encoding/json"
)

// GetExchanges performs operations related to GetExchanges functionality.
func GetExchanges(conn *data.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
    exchangeMap := marketData.GetExchanges()
	return exchangeMap, nil
}
