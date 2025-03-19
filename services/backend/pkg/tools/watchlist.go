package tools

import (
	"backend/utils"
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
func GetWatchlists(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), ` SELECT watchlistId, watchlistName FROM watchlists where userId = $1 `, userId)
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
func NewWatchlist(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewWatchlistArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("3og9 invalid args: %v", err)
	}
	var watchlistID int
	err = conn.DB.QueryRow(context.Background(), "INSERT INTO watchlists (watchlistName,userId) values ($1,$2) RETURNING watchlistId", args.WatchlistName, userId).Scan(&watchlistID)
	if err != nil {
		return nil, fmt.Errorf("0n8912")
	}

	return watchlistID, err
}

// DeleteWatchlistArgs represents a structure for handling DeleteWatchlistArgs data.
type DeleteWatchlistArgs struct {
	Id int `json:"watchlistId"`
}

// DeleteWatchlist performs operations related to DeleteWatchlist functionality.
func DeleteWatchlist(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteWatchlistArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "DELETE FROM watchlists where watchlistId = $1", args.Id)
	if err != nil {
		return nil, err
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("ssd7g3")
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
func GetWatchlistItems(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetWatchlistEntriesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
	rows, err := conn.DB.Query(context.Background(),
		`SELECT w.securityId, s.ticker, w.watchlistItemId from watchlistItems as w
    JOIN securities as s ON s.securityId = w.securityId
    where w.watchlistId = $1`, args.WatchlistID)
	if err != nil {
		return nil, fmt.Errorf("sovn %v", err)
	}
	defer rows.Close()
	var entries []GetWatchlistEntriesResult
	for rows.Next() {
		var entry GetWatchlistEntriesResult
		err = rows.Scan(&entry.SecurityID, &entry.Ticker, &entry.WatchlistItemID)
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
func DeleteWatchlistItem(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args DeleteWatchlistItemArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "DELETE FROM watchlistItems WHERE watchlistItemId = $1", args.WatchlistItemID)
	if err != nil {
		return nil, fmt.Errorf("niv02 %v", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("mvo2")
	}
	return nil, nil
}

// NewWatchlistItemArgs represents a structure for handling NewWatchlistItemArgs data.
type NewWatchlistItemArgs struct {
	WatchlistID int `json:"watchlistId"`
	SecurityID  int `json:"securityId"`
}

// NewWatchlistItem performs operations related to NewWatchlistItem functionality.
func NewWatchlistItem(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewWatchlistItemArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d %v", err)
	}
	var watchlistID int
	err = conn.DB.QueryRow(context.Background(), "INSERT into watchlistItems (securityId,watchlistId) values ($1,$2) RETURNING watchlistItemId", args.SecurityID, args.WatchlistID).Scan(&watchlistID)
	if err != nil {
		return nil, err
	}
	return watchlistID, err
}
