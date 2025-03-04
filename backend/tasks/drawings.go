package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
)

type HorizontalLine struct {
	Id         int     `json:"id"`
	SecurityId int     `json:"securityId"`
	Price      float64 `json:"price"`
	Color      string  `json:"color"`
	LineWidth  int     `json:"lineWidth"`
}

type GetHorizontalLinesArgs struct {
	SecurityId int `json:"securityId"`
}

func GetHorizontalLines(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetHorizontalLinesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
	rows, err := conn.DB.Query(context.Background(), `
		SELECT id, securityId, price, color, line_width
		FROM horizontal_lines
		WHERE securityId = $1
		AND userId = $2`, args.SecurityId, userId)
	if err != nil {
		return nil, fmt.Errorf("error querying horizontal lines: %v", err)
	}
	defer rows.Close()

	var lines []HorizontalLine
	for rows.Next() {
		var line HorizontalLine
		if err := rows.Scan(&line.Id, &line.SecurityId, &line.Price, &line.Color, &line.LineWidth); err != nil {
			return nil, fmt.Errorf("error scanning horizontal line: %v", err)
		}
		lines = append(lines, line)
	}

	return lines, nil
}

type DeleteHorizontalLineArgs struct {
	Id int `json:"id"`
}

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
		RETURNING id`, args.SecurityId, args.Price, userId, args.Color, args.LineWidth).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("error inserting horizontal line: %v", err)
	}
	return id, nil
}

type UpdateHorizontalLineArgs struct {
	Id         int     `json:"id"`
	SecurityId int     `json:"securityId"`
	Price      float64 `json:"price"`
	Color      string  `json:"color"`
	LineWidth  int     `json:"lineWidth"`
}

func UpdateHorizontalLine(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args UpdateHorizontalLineArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	cmdTag, err := conn.DB.Exec(context.Background(), `
		UPDATE horizontal_lines
		SET price = $1, color = $2, line_width = $3
		WHERE id = $4 AND userId = $5 AND securityId = $6`,
		args.Price, args.Color, args.LineWidth, args.Id, userId, args.SecurityId)

	if err != nil {
		return nil, fmt.Errorf("error updating horizontal line: %v", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("no horizontal line found with id %d", args.Id)
	}

	return nil, nil
}
