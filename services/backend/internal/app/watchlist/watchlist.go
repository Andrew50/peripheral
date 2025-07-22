package watchlist

import (
	"backend/internal/app/helpers"
	"backend/internal/data"
	"backend/internal/services/socket"
	"context"
	"encoding/json"
	"fmt"
	"time"
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
	WatchlistName string   `json:"watchlistName"`
	Tickers       []string `json:"tickers,omitempty"`
}

func AgentNewWatchlist(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := NewWatchlist(conn, userID, rawArgs)
	if err != nil {
		return nil, fmt.Errorf("error creating watchlist: %v", err)
	}

	// Handle notification asynchronously
	go func() {
		var args NewWatchlistArgs
		err := json.Unmarshal(rawArgs, &args)
		if err != nil {
			return // Silently ignore notification errors
		}

		value := map[string]interface{}{
			"watchlistName": args.WatchlistName,
			"tickers":       args.Tickers,
			"watchlistId":   res,
		}
		socket.SendAgentStatusUpdate(userID, "newWatchlist", value)
	}()

	return res, nil
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
	if len(args.Tickers) > 0 {
		rawArgs := json.RawMessage(fmt.Sprintf(`{"watchlistId": %d, "tickers": %v}`, watchlistID, args.Tickers))
		_, err = AddTickersToWatchlist(conn, userID, rawArgs)
		if err != nil {
			return nil, fmt.Errorf("error adding tickers to watchlist: %v", err)
		}
	}

	return watchlistID, err
}

// DeleteWatchlistArgs represents a structure for handling DeleteWatchlistArgs data.
type DeleteWatchlistArgs struct {
	ID int `json:"watchlistId"`
}

func AgentDeleteWatchlist(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := DeleteWatchlist(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}

	go func() {
		var args DeleteWatchlistArgs
		err := json.Unmarshal(rawArgs, &args)
		if err != nil {
			return
		}
		socket.SendWatchlistUpdate(userID, "delete", &args.ID, nil, nil, nil)
	}()

	return res, nil
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
	return args.ID, err
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

type AgentGetWatchlistItemsArgs struct {
	WatchlistID int `json:"watchlistId"`
}

func AgentGetWatchlistItems(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args AgentGetWatchlistItemsArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d [agentGetWatchlistItems]: %v", err)
	}
	owns, err := VerifyUserOwnsWatchlist(conn, userID, args.WatchlistID)
	if err != nil {
		return nil, err
	}
	if !owns {
		return nil, fmt.Errorf("watchlist not found or you don't have permission to access it")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := conn.DB.Query(ctx, `
		SELECT s.ticker 
		FROM watchlistItems wi
		JOIN securities s ON wi.securityId = s.securityId
		WHERE wi.watchlistId = $1 AND s.maxDate IS NULL
		ORDER BY s.ticker`,
		args.WatchlistID)
	if err != nil {
		return nil, fmt.Errorf("error querying watchlist items: %v", err)
	}
	defer rows.Close()

	var tickers []string
	for rows.Next() {
		var ticker string
		err = rows.Scan(&ticker)
		if err != nil {
			return nil, fmt.Errorf("error scanning ticker: %v", err)
		}
		tickers = append(tickers, ticker)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating rows: %v", rows.Err())
	}

	// Safely copy needed data before goroutine to prevent race conditions
	watchlistID := args.WatchlistID
	tickersCopy := make([]string, len(tickers))
	copy(tickersCopy, tickers)

	go func() {
		var tickerWtihData []map[string]interface{}
		var watchlistName string
		err := conn.DB.QueryRow(context.Background(), `SELECT watchlistname from watchlists where watchlistId = $1 LIMIT 1`, watchlistID).Scan(&watchlistName)
		if err != nil {
			fmt.Printf("Error getting watchlist name for ID %d: %v\n", watchlistID, err)
			return // Silently ignore notification errors
		}

		// Properly marshal tickers to JSON
		tickersJSON, err := json.Marshal(tickersCopy)
		if err != nil {
			fmt.Printf("Error marshaling tickers for watchlist %d: %v\n", watchlistID, err)
			return // Silently ignore notification errors
		}

		icons, err := helpers.GetIcons(conn, userID, json.RawMessage(fmt.Sprintf(`{"tickers": %s}`, tickersJSON)))
		if err != nil {
			fmt.Printf("Error getting icons for watchlist %d: %v\n", watchlistID, err)
			return // Silently ignore notification errors
		}

		// Convert icons slice to map for efficient lookup with safe type assertion
		iconMap := make(map[string]string)
		if iconResults, ok := icons.([]helpers.GetIconsResults); ok {
			for _, iconResult := range iconResults {
				iconMap[iconResult.Ticker] = iconResult.Icon
			}
		} else {
			fmt.Printf("Warning: Unexpected type for icons result in watchlist %d\n", watchlistID)
		}

		for _, ticker := range tickersCopy {
			res, err := helpers.GetTickerDailySnapshot(conn, userID, json.RawMessage(fmt.Sprintf(`{"ticker": "%s"}`, ticker)))
			if err != nil {
				fmt.Printf("Error getting snapshot for ticker %s in watchlist %d: %v\n", ticker, watchlistID, err)
				continue // Skip this ticker but continue with others
			}

			// Safe type assertion with error checking
			if snapshotResult, ok := res.(helpers.GetTickerDailySnapshotResults); ok {
				tickerWithData := map[string]interface{}{
					"ticker":        ticker,
					"price":         snapshotResult.Close,
					"change":        snapshotResult.TodayChange,
					"changePercent": snapshotResult.TodayChangePercent,
					"icon":          iconMap[ticker], // Use map lookup instead of slice index
				}
				tickerWtihData = append(tickerWtihData, tickerWithData)
			} else {
				fmt.Printf("Warning: Unexpected type for snapshot result for ticker %s in watchlist %d\n", ticker, watchlistID)
				continue // Skip this ticker but continue with others
			}
		}

		value := map[string]interface{}{
			"watchlistId":   watchlistID,
			"tickers":       tickerWtihData,
			"watchlistName": watchlistName,
		}

		socket.SendAgentStatusUpdate(userID, "getWatchlistItems", value)
	}()

	return tickers, nil
}

// DeleteWatchlistItemArgs represents a structure for handling DeleteWatchlistItemArgs data.
type DeleteWatchlistItemArgs struct {
	WatchlistItemID int `json:"watchlistItemId"`
}

func AgentDeleteWatchlistItem(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	res, err := DeleteWatchlistItem(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}
	go func() {
		var args DeleteWatchlistItemArgs
		err := json.Unmarshal(rawArgs, &args)
		if err != nil {
			return
		}
		socket.SendWatchlistUpdate(userID, "remove", &args.WatchlistItemID, nil, nil, nil)
	}()
	return res, nil
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

	return watchlistID, err
}

type AddTickersToWatchlistArgs struct {
	WatchlistID int      `json:"watchlistId"`
	Tickers     []string `json:"tickers"`
}

func AgentAddTickersToWatchlist(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	watchlistItemIDs, err := AddTickersToWatchlist(conn, userID, rawArgs)
	if err != nil {
		return nil, err
	}
	// need to implement something for agent status update later

	/*go func() {
		var args AddTickersToWatchlistArgs
		err = json.Unmarshal(rawArgs, &args)
		if err != nil {
			return
		}
		for _, ticker := range args.Tickers {
			item := map[string]interface{}{
				"watchlistItemId": res,
				"securityId":      args.SecurityID,
				"ticker":          ticker,
			}
			socket.SendWatchlistUpdate(userID, "add", &args.WatchlistID, nil, item, nil)
		}
	}()*/
	return fmt.Sprintf("successfully added %d tickers", len(watchlistItemIDs.([]int))), nil
}

func AddTickersToWatchlist(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args AddTickersToWatchlistArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d [addTickersToWatchlist]: %v", err)
	}

	owns, err := VerifyUserOwnsWatchlist(conn, userID, args.WatchlistID)
	if err != nil {
		return nil, err
	}
	if !owns {
		return nil, fmt.Errorf("watchlist not found or you don't have permission to modify it")
	}

	rows, err := conn.DB.Query(context.Background(),
		`INSERT INTO watchlistItems (securityId, watchlistId)
		SELECT s.securityId, $1
		FROM securities s
		WHERE s.ticker = ANY($2::text[])
		  AND s.maxDate IS NULL
		ON CONFLICT (securityId, watchlistId) 
		DO UPDATE SET securityId = EXCLUDED.securityId
		RETURNING watchlistItemId`,
		args.WatchlistID, args.Tickers)
	if err != nil {
		return nil, fmt.Errorf("error inserting watchlist items: %v", err)
	}
	defer rows.Close()

	var watchlistItemIDs []int

	for rows.Next() {
		var rowWatchlistItemID int
		err = rows.Scan(&rowWatchlistItemID)
		if err != nil {
			return nil, fmt.Errorf("error scanning inserted items: %v", err)
		}
		watchlistItemIDs = append(watchlistItemIDs, rowWatchlistItemID)
	}

	return watchlistItemIDs, nil
}
func VerifyUserOwnsWatchlist(conn *data.Conn, userID int, watchlistID int) (bool, error) {
	var watchlistExists bool
	err := conn.DB.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM watchlists WHERE watchlistId = $1 AND userId = $2)`,
		watchlistID, userID).Scan(&watchlistExists)
	if err != nil {
		return false, fmt.Errorf("error verifying watchlist ownership: %v", err)
	}
	return watchlistExists, nil
}
