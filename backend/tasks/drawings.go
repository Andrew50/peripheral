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
		SELECT id, securityId, price
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
		if err := rows.Scan(&line.Id, &line.SecurityId, &line.Price); err != nil {
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
	var id int
	err := conn.DB.QueryRow(context.Background(), `
		INSERT INTO horizontal_lines (securityId, price, userId)
		VALUES ($1, $2, $3)
		RETURNING id`, args.SecurityId, args.Price, userId).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("error inserting horizontal line: %v", err)
	}
	return id, nil
}
