package postgres

import (
	"backend/internal/data"
	"context"
)

// GetLocales returns unique locale values for active securities.
func GetLocales(conn *data.Conn) ([]string, error) {
	rows, err := conn.DB.Query(context.Background(), `SELECT DISTINCT locale FROM securities WHERE locale IS NOT NULL AND locale != '' AND maxDate IS NULL ORDER BY locale`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	locales := []string{}
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		locales = append(locales, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return locales, nil
}
