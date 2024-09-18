package tasks


//getWatchlistItems
//getWatchlists
//setWatchlistItem
//deleteWatchlistItem
//deleteWatchlist
//newWatchlist

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type GetWatchlistsResult struct {
    WatchlistId     int         `json:"watchlistId"`    
	WatchlistName    string    `json:"watchlistName"`
}

func GetWatchlists(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), ` SELECT watchlistName s.userId = $1 `, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var watchlists []GetWatchlistsResult
	for rows.Next() {
		var watchlist GetWatchlistsResult
		err := rows.Scan(watchlist.WatchlistId,watchlist.WatchlistName)
		if err != nil {
			return nil, err
		}
		watchlists = append(watchlists, watchlist)
	}
	return watchlists, nil
}

type SaveWatchlistArgs struct {
	Id    int             `json:"id"`
	Entry json.RawMessage `json:"entry"`
}

func SaveWatchlist(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args SaveWatchlistArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("3og9 invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE watchlists Set entry = $1 where watchlistId = $2", args.Entry, args.Id)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("0n8912")
	}
	return nil, err
}

type CompleteWatchlistArgs struct {
	Id        int  `json:"id"`
	Completed bool `json:"completed"`
}

func CompleteWatchlist(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args CompleteWatchlistArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("215d invalid args: %v", err)
	}
	cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE watchlists Set completed = $1 where watchlistId = $2", args.Completed, args.Id)
	if cmdTag.RowsAffected() == 0 {
		return nil, fmt.Errorf("0n8912")
	}
	return nil, err
}

type DeleteWatchlistArgs struct {
	Id int `json:"id"`
}

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

type GetWatchlistEntriesArgs struct {
	WatchlistId int `json:"watchlistId"`
}

type GetWatchlistEntriesResult struct {
    SecurityId int `json:"securityId"`
    Ticker string `json:"ticker"`
}

func GetWatchlistEntries(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetWatchlistEntriesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
    rows, err := conn.DB.Query(context.Background(), 
    `SELECT w.securityId, s.ticker from watchlists as w
    JOIN securities as s 
    where watchlistId = $1", s.securityId = w.securityId`, args.WatchlistId)
	if err != nil {
		return nil, err
	}
    defer rows.Close()
    var entries []GetWatchlistEntriesResult
    for rows.Next(){
        var entry GetWatchlistEntriesResult
        err = rows.Scan(&entry.SecurityId,&entry.Ticker)
        if err != nil {
            return nil, fmt.Errorf("fi0w %v", err)
        }
        entries = append(entries,entry)
    }
    if rows.Err() != nil {
        return nil, fmt.Errorf("m10c %v",err)
    }
    return entries, nil
}

type NewWatchlistArgs struct {
    WatchlistName string `json:"watchlistName"`
}

func NewWatchlist(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewWatchlistArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d %v", err)
	}
	var watchlistId int
	err = conn.DB.QueryRow(context.Background(), "INSERT into watchlists (userId,securityId, timestamp) values ($1,$2,$3) RETURNING watchlistId", userId, args.SecurityId, timestamp).Scan(&watchlistId)
	if err != nil {
		return nil, err
	}
	return watchlistId, err
}
