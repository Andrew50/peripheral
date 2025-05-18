package postgres

import (
	"backend/internal/data"
	"context"
)

// GetMarkets returns unique market values for active securities.
func GetMarkets(conn *data.Conn) ([]string, error) {
	rows, err := conn.DB.Query(context.Background(), `SELECT DISTINCT market FROM securities WHERE market IS NOT NULL AND market != '' AND maxDate IS NULL ORDER BY market`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	markets := []string{}
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		markets = append(markets, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return markets, nil
}
