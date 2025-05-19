package postgres

import (
	"backend/internal/data"
	"context"
)

// GetIndustries returns a list of unique industry values for active securities.
func GetIndustries(conn *data.Conn) ([]string, error) {
	rows, err := conn.DB.Query(context.Background(), `SELECT DISTINCT industry FROM securities WHERE industry IS NOT NULL AND industry != '' AND maxDate IS NULL ORDER BY industry`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	industries := []string{}
	for rows.Next() {
		var i string
		if err := rows.Scan(&i); err != nil {
			return nil, err
		}
		industries = append(industries, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return industries, nil
}
