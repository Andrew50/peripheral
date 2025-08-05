package chart

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
)

// HorizontalLine represents a structure for handling HorizontalLine data.
type HorizontalLine struct {
	ID         int     `json:"id"`
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
func GetHorizontalLines(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetHorizontalLinesArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}
	rows, err := conn.DB.Query(context.Background(), `
		SELECT id, securityId, price, color, line_width
		FROM horizontal_lines
		WHERE securityId = $1
		AND userId = $2`, args.SecurityID, userID)
	if err != nil {
		return nil, fmt.Errorf("error querying horizontal lines: %v", err)
	}
	defer rows.Close()

	var lines []HorizontalLine
	for rows.Next() {
		var line HorizontalLine
		if err := rows.Scan(&line.ID, &line.SecurityID, &line.Price, &line.Color, &line.LineWidth); err != nil {
			return nil, fmt.Errorf("error scanning horizontal line: %v", err)
		}
		lines = append(lines, line)
	}

	return lines, nil
}

// DeleteHorizontalLineArgs represents a structure for handling DeleteHorizontalLineArgs data.
type DeleteHorizontalLineArgs struct {
	ID int `json:"id"`
}

func AgentDeleteHorizontalLine(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := DeleteHorizontalLine(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}
	/*go func() {
		var args DeleteHorizontalLineArgs
		err = json.Unmarshal(rawArgs, &args)
		if err != nil {
			return
		}
		lineData := map[string]interface{}{
			"id": args.ID,
		}
		socket.SendHorizontalLineUpdate(userID, "remove", securityID, lineData)
	}()
	*/
	return res, nil
}

// DeleteHorizontalLine performs operations related to DeleteHorizontalLine functionality.
func DeleteHorizontalLine(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteHorizontalLineArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	// Get security ID before deletion for WebSocket update
	var securityID int
	err := conn.DB.QueryRow(context.Background(),
		`SELECT securityId FROM horizontal_lines WHERE id = $1 AND userId = $2`,
		args.ID, userID).Scan(&securityID)
	if err != nil {
		return nil, fmt.Errorf("horizontal line not found: %v", err)
	}

	cmdTag, err := conn.DB.Exec(context.Background(), `DELETE FROM horizontal_lines WHERE id = $1 AND userId = $2`, args.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("error deleting horizontal line: %v", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("error deleting horizontal line: %v", err)
	}

	return nil, nil
}

func AgentSetHorizontalLine(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := SetHorizontalLine(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}
	/*go func() {
		var args HorizontalLine
		err = json.Unmarshal(rawArgs, &args)
		if err != nil {
			return
		}
		lineData := map[string]interface{}{
			"id":         id,
			"securityId": line.SecurityID,
			"price":      line.Price,
			"color":      line.Color,
			"lineWidth":  line.LineWidth,
		}
		socket.SendHorizontalLineUpdate(userID, "add", line.SecurityID, lineData)
	}()*/
	return res, nil
}

// SetHorizontalLine performs operations related to SetHorizontalLine functionality.
func SetHorizontalLine(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var line HorizontalLine // Reuse the HorizontalLine struct for request body
	if err := json.Unmarshal(rawArgs, &line); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	// Set default values if not provided
	if line.Color == "" {
		line.Color = "#FFFFFF" // Default to white
	}
	if line.LineWidth == 0 {
		line.LineWidth = 1 // Default to 1px
	}

	var id int
	err := conn.DB.QueryRow(context.Background(), `
		INSERT INTO horizontal_lines (securityId, price, userId, color, line_width)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, line.SecurityID, line.Price, userID, line.Color, line.LineWidth).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("error inserting horizontal line: %v", err)
	}

	return id, nil
}

// UpdateHorizontalLineArgs represents a structure for handling UpdateHorizontalLineArgs data.
type UpdateHorizontalLineArgs struct {
	ID         int     `json:"id"`
	SecurityID int     `json:"securityId"`
	Price      float64 `json:"price"`
	Color      string  `json:"color"`
	LineWidth  int     `json:"lineWidth"`
}

func AgentUpdateHorizontalLine(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := UpdateHorizontalLine(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}
	/*go func() {
		lineData := map[string]interface{}{
			"id":         args.ID,
			"securityId": args.SecurityID,
			"price":      args.Price,
			"color":      args.Color,
			"lineWidth":  args.LineWidth,
		}
		socket.SendHorizontalLineUpdate(userID, "update", args.SecurityID, lineData)
	}()*/
	return res, nil
}

// UpdateHorizontalLine performs operations related to UpdateHorizontalLine functionality.
func UpdateHorizontalLine(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args UpdateHorizontalLineArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("error parsing args: %v", err)
	}

	cmdTag, err := conn.DB.Exec(context.Background(), `
		UPDATE horizontal_lines
		SET price = $1, color = $2, line_width = $3
		WHERE id = $4 AND userId = $5 AND securityId = $6`,
		args.Price, args.Color, args.LineWidth, args.ID, userID, args.SecurityID)

	if err != nil {
		return nil, fmt.Errorf("error updating horizontal line: %v", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("no horizontal line found with id %d", args.ID)
	}

	return nil, nil
}
