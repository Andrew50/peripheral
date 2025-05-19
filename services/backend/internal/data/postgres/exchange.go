package postgres

import (
	"backend/internal/data"
	"context"
)

// GetPrimaryExchanges returns unique primary_exchange values for active securities.
func GetPrimaryExchanges(conn *data.Conn) ([]string, error) {
	rows, err := conn.DB.Query(context.Background(), `SELECT DISTINCT primary_exchange FROM securities WHERE primary_exchange IS NOT NULL AND primary_exchange != '' AND maxDate IS NULL ORDER BY primary_exchange`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	exchanges := []string{}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, err
		}
		exchanges = append(exchanges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return exchanges, nil
}
