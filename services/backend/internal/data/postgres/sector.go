package postgres

import (
	"backend/internal/data"
	"context"
)

// GetSectors returns a list of unique sector values for active securities.
func GetSectors(conn *data.Conn) ([]string, error) {
	rows, err := conn.DB.Query(context.Background(), `SELECT DISTINCT sector FROM securities WHERE sector IS NOT NULL AND sector != '' AND maxDate IS NULL ORDER BY sector`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sectors := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		sectors = append(sectors, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sectors, nil
}
