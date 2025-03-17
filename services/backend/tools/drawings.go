package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
)
// HorizontalLine represents a structure for handling HorizontalLine data.
type HorizontalLine struct {
	Id         int     `json:"id"`
	SecurityID int     `json:"securityId"`
	Price      float64 `json:"price"`
	Color      string  `json:"color"`
	LineWidth  int     `json:"lineWidth"`
}
// GetHorizontalLinesArgs represents a structure for handling GetHorizontalLinesArgs data.
type GetHorizontalLinesArgs struct {
	SecurityID int `json:"securityId"`
}
// GetHorizontalLines performs operations related to GetHorizontalLines functionality.
func GetHorizontalLines(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetHorizontalLinesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
	rows, err := conn.DB.Query(context.Background(), `
		SELECT id, securityId, price, color, line_width
		FROM horizontal_lines
		WHERE securityId = $1
		AND userId = $2`, args.SecurityID, userId)
	if err != nil {
		return nil, fmt.Errorf("error querying horizontal lines: %v", err)
	}
	defer rows.Close()

	var lines []HorizontalLine
	for rows.Next() {
		var line HorizontalLine
		if err := rows.Scan(&line.Id, &line.SecurityID, &line.Price, &line.Color, &line.LineWidth); err != nil {
			return nil, fmt.Errorf("error scanning horizontal line: %v", err)
		}
		lines = append(lines, line)
	}

	return lines, nil
}
// DeleteHorizontalLineArgs represents a structure for handling DeleteHorizontalLineArgs data.
type DeleteHorizontalLineArgs struct {
	Id int `json:"id"`
}
// DeleteHorizontalLine performs operations related to DeleteHorizontalLine functionality.
func DeleteHorizontalLine(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteHorizontalLineArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), `DELETE FROM horizontal_lines WHERE id = $1 AND userId = $2`, args.Id, userId)
	if err != nil {
		return nil, fmt.Errorf("error deleting horizontal line: %v", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("error deleting horizontal line: %v", err)
	}
	return nil, nil
}
// SetHorizontalLine performs operations related to SetHorizontalLine functionality.
func SetHorizontalLine(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args HorizontalLine // won't have id
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	// Set default values if not provided
	if args.Color == "" {
		args.Color = "#FFFFFF" // Default to white
	}
	if args.LineWidth == 0 {
		args.LineWidth = 1 // Default to 1px
	}

	var id int
	err := conn.DB.QueryRow(context.Background(), `
		INSERT INTO horizontal_lines (securityId, price, userId, color, line_width)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, args.SecurityID, args.Price, userId, args.Color, args.LineWidth).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("error inserting horizontal line: %v", err)
	}
	return id, nil
}
// UpdateHorizontalLineArgs represents a structure for handling UpdateHorizontalLineArgs data.
type UpdateHorizontalLineArgs struct {
	Id         int     `json:"id"`
	SecurityID int     `json:"securityId"`
	Price      float64 `json:"price"`
	Color      string  `json:"color"`
	LineWidth  int     `json:"lineWidth"`
}
// UpdateHorizontalLine performs operations related to UpdateHorizontalLine functionality.
func UpdateHorizontalLine(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args UpdateHorizontalLineArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	cmdTag, err := conn.DB.Exec(context.Background(), `
		UPDATE horizontal_lines
		SET price = $1, color = $2, line_width = $3
		WHERE id = $4 AND userId = $5 AND securityId = $6`,
		args.Price, args.Color, args.LineWidth, args.Id, userId, args.SecurityID)

	if err != nil {
		return nil, fmt.Errorf("error updating horizontal line: %v", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("no horizontal line found with id %d", args.Id)
	}

	return nil, nil
}
