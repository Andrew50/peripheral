package watchlist

import (
	"backend/internal/data"
	"backend/internal/services/socket"
	"context"
	"encoding/json"
	"fmt"
)

// GetWatchlistsResult represents a structure for handling GetWatchlistsResult data.
type GetWatchlistsResult struct {
	WatchlistID   int    `json:"watchlistId"`
	WatchlistName string `json:"watchlistName"`
}

// GetWatchlists performs operations related to GetWatchlists functionality.
func GetWatchlists(conn *data.Conn, userID int, _ json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(),
		`SELECT watchlistId, watchlistName
		FROM watchlists
		WHERE userId = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("[pvk %v", err)
	}
	defer rows.Close()
	var watchlists []GetWatchlistsResult
	for rows.Next() {
		var watchlist GetWatchlistsResult
		err := rows.Scan(&watchlist.WatchlistID, &watchlist.WatchlistName)
		if err != nil {
			return nil, fmt.Errorf("1niv %v", err)
		}
		watchlists = append(watchlists, watchlist)
	}
	return watchlists, nil
}

// NewWatchlistArgs represents a structure for handling NewWatchlistArgs data.
type NewWatchlistArgs struct {
	WatchlistName string `json:"watchlistName"`
}

// NewWatchlist performs operations related to NewWatchlist functionality.
func NewWatchlist(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewWatchlistArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("3og9 invalid args: %v", err)
	}
	var watchlistID int
	err = conn.DB.QueryRow(context.Background(), "INSERT INTO watchlists (watchlistName,userId) values ($1,$2) RETURNING watchlistId", args.WatchlistName, userID).Scan(&watchlistID)
	if err != nil {
		return nil, fmt.Errorf("0n8912: %v", err)
	}

	// NEW: Send WebSocket update after successful creation
	// Only send WebSocket update if called by LLM (frontend handles its own updates)
	if conn.IsLLMExecution {
		socket.SendWatchlistUpdate(userID, "create", &watchlistID, &args.WatchlistName, nil, nil)
	}

	return watchlistID, err
}

// DeleteWatchlistArgs represents a structure for handling DeleteWatchlistArgs data.
type DeleteWatchlistArgs struct {
	ID int `json:"watchlistId"`
}

// DeleteWatchlist performs operations related to DeleteWatchlist functionality.
func DeleteWatchlist(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteWatchlistArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "DELETE FROM watchlists WHERE watchlistId = $1 AND userId = $2", args.ID, userID)
	if err != nil {
		return nil, err
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("watchlist not found or you don't have permission to delete it")
	}

	// NEW: Send WebSocket update after successful deletion
	// Only send WebSocket update if called by LLM (frontend handles its own updates)
	if conn.IsLLMExecution {
		socket.SendWatchlistUpdate(userID, "delete", &args.ID, nil, nil, nil)
	}

	return nil, err
}

// GetWatchlistEntriesArgs represents a structure for handling GetWatchlistEntriesArgs data.
type GetWatchlistEntriesArgs struct {
	WatchlistID int `json:"watchlistId"`
}

// GetWatchlistEntriesResult represents a structure for handling GetWatchlistEntriesResult data.
type GetWatchlistEntriesResult struct {
	SecurityID      int    `json:"securityId"`
	Ticker          string `json:"ticker"`
	WatchlistItemID int    `json:"watchlistItemId"`
}

// GetWatchlistItems performs operations related to GetWatchlistItems functionality.
func GetWatchlistItems(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetWatchlistEntriesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}

	// First verify that the watchlist belongs to the user
	var watchlistExists bool
	err = conn.DB.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM watchlists WHERE watchlistId = $1 AND userId = $2)`,
		args.WatchlistID, userID).Scan(&watchlistExists)
	if err != nil {
		return nil, fmt.Errorf("error verifying watchlist ownership: %v", err)
	}
	if !watchlistExists {
		return nil, fmt.Errorf("watchlist not found or you don't have permission to access it")
	}

	rows, err := conn.DB.Query(context.Background(),
		`SELECT securityId, ticker, watchlistItemId, maxDate
		FROM (
			SELECT w.securityId, s.ticker, w.watchlistItemId, s.maxDate,
				   ROW_NUMBER() OVER (PARTITION BY w.securityId ORDER BY COALESCE(s.maxDate, CURRENT_TIMESTAMP) DESC, w.watchlistItemId DESC) as rn
			FROM watchlistItems as w
			JOIN securities as s ON s.securityId = w.securityId
			WHERE w.watchlistId = $1
		) ranked
		WHERE rn = 1
		ORDER BY COALESCE(maxDate, CURRENT_TIMESTAMP) DESC, watchlistItemId ASC`, args.WatchlistID)
	if err != nil {
		return nil, fmt.Errorf("sovn %v", err)
	}
	defer rows.Close()
	var entries []GetWatchlistEntriesResult
	for rows.Next() {
		var entry GetWatchlistEntriesResult
		var maxDate interface{} // temporary variable to scan maxDate (for ordering only)
		err = rows.Scan(&entry.SecurityID, &entry.Ticker, &entry.WatchlistItemID, &maxDate)
		if err != nil {
			return nil, fmt.Errorf("fi0w %v", err)
		}
		entries = append(entries, entry)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("m10c %v", err)
	}
	return entries, nil
}

// DeleteWatchlistItemArgs represents a structure for handling DeleteWatchlistItemArgs data.
type DeleteWatchlistItemArgs struct {
	WatchlistItemID int `json:"watchlistItemId"`
}

// DeleteWatchlistItem performs operations related to DeleteWatchlistItem functionality.
func DeleteWatchlistItem(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteWatchlistItemArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d %v", err)
	}

	// Get watchlist ID before deletion for WebSocket update
	var watchlistID int
	err = conn.DB.QueryRow(context.Background(),
		`SELECT watchlistId FROM watchlistItems WHERE watchlistItemId = $1`,
		args.WatchlistItemID).Scan(&watchlistID)
	if err != nil {
		return nil, fmt.Errorf("watchlist item not found: %v", err)
	}

	cmdTag, err := conn.DB.Exec(context.Background(), `
		DELETE FROM watchlistItems 
		WHERE watchlistItemId = $1 
		AND watchlistId IN (SELECT watchlistId FROM watchlists WHERE userId = $2)`,
		args.WatchlistItemID, userID)
	if err != nil {
		return nil, fmt.Errorf("niv02 %v", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("watchlist item not found or you don't have permission to delete it")
	}

	// NEW: Send WebSocket update after successful deletion
	// Only send WebSocket update if called by LLM (frontend handles its own updates)
	if conn.IsLLMExecution {
		socket.SendWatchlistUpdate(userID, "remove", &watchlistID, nil, nil, &args.WatchlistItemID)
	}

	return nil, nil
}

// NewWatchlistItemArgs represents a structure for handling NewWatchlistItemArgs data.
type NewWatchlistItemArgs struct {
	WatchlistID int `json:"watchlistId"`
	SecurityID  int `json:"securityId"`
}

// NewWatchlistItem performs operations related to NewWatchlistItem functionality.
func NewWatchlistItem(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewWatchlistItemArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d %v", err)
	}

	// Verify that the watchlist belongs to the user
	var watchlistExists bool
	err = conn.DB.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM watchlists WHERE watchlistId = $1 AND userId = $2)`,
		args.WatchlistID, userID).Scan(&watchlistExists)
	if err != nil {
		return nil, fmt.Errorf("error verifying watchlist ownership: %v", err)
	}
	if !watchlistExists {
		return nil, fmt.Errorf("watchlist not found or you don't have permission to modify it")
	}

	var watchlistID int
	err = conn.DB.QueryRow(context.Background(),
		"INSERT into watchlistItems (securityId,watchlistId) values ($1,$2) RETURNING watchlistItemId",
		args.SecurityID, args.WatchlistID).Scan(&watchlistID)
	if err != nil {
		return nil, err
	}

	// NEW: Send WebSocket update after successful insertion
	// Only send WebSocket update if called by LLM (frontend handles its own updates)
	if conn.IsLLMExecution {
		// Get ticker for the security
		var ticker string
		err = conn.DB.QueryRow(context.Background(),
			`SELECT ticker FROM securities WHERE securityId = $1 AND maxDate IS NULL LIMIT 1`,
			args.SecurityID).Scan(&ticker)
		if err == nil {
			// Create item data for WebSocket update
			item := map[string]interface{}{
				"watchlistItemId": watchlistID,
				"securityId":      args.SecurityID,
				"ticker":          ticker,
			}
			socket.SendWatchlistUpdate(userID, "add", &args.WatchlistID, nil, item, nil)
		}
	}

	return watchlistID, err
}
