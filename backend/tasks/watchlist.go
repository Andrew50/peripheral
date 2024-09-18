package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
)

type GetWatchlistsResult struct {
    WatchlistId     int         `json:"watchlistId"`    
	WatchlistName    string    `json:"watchlistName"`
}

func GetWatchlists(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	rows, err := conn.DB.Query(context.Background(), ` SELECT watchlistId, watchlistName FROM watchlists where userId = $1 `, userId)
	if err != nil {
		return nil, fmt.Errorf("[pvk %v",err)
	}
	defer rows.Close()
	var watchlists []GetWatchlistsResult
	for rows.Next() {
		var watchlist GetWatchlistsResult
		err := rows.Scan(&watchlist.WatchlistId,&watchlist.WatchlistName)
		if err != nil {
			return nil, fmt.Errorf("1niv %v",err)
		}
		watchlists = append(watchlists, watchlist)
	}
	return watchlists, nil
}

type NewWatchlistArgs struct {
    WatchlistName string `json:"watchlistName"`
}

func NewWatchlist(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewWatchlistArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("3og9 invalid args: %v", err)
	}
	_,err = conn.DB.Exec(context.Background(), "INSERT INTO watchlists (watchlistName,userId) values ($1,$2) RETURNING watchlistId", args.WatchlistName, userId)
	if err != nil {
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
    WatchlistItemId int `json:"watchlistItemId"`
}

func GetWatchlistItems(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetWatchlistEntriesArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("GetCik invalid args: %v", err)
	}
    rows, err := conn.DB.Query(context.Background(), 
    `SELECT w.securityId, s.ticker, w.watchlistItemId from watchlistItems as w
    JOIN securities as s ON s.securityId = w.securityId
    where w.watchlistId = $1`, args.WatchlistId)
	if err != nil {
		return nil, fmt.Errorf("sovn %v",err)
	}
    defer rows.Close()
    var entries []GetWatchlistEntriesResult
    for rows.Next(){
        var entry GetWatchlistEntriesResult
        err = rows.Scan(&entry.SecurityId,&entry.Ticker,&entry.WatchlistItemId)
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

type DeleteWatchlistItemArgs struct {
    WatchlistItemId int `json:"watchlistItemId"`
}

func DeleteWatchlistItem(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{},error) {
	var args DeleteWatchlistItemArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d %v", err)
	}
    cmdTag, err := conn.DB.Exec(context.Background(),"DELETE FROM watchlistItems WHERE watchlistItemId = $1",args.WatchlistItemId)
    if err != nil {
        return nil, fmt.Errorf("niv02 %v",err)
    }
    if cmdTag.RowsAffected() == 0 {
        return nil, fmt.Errorf("mvo2")
    }
    return nil, nil
}



type NewWatchlistItemArgs struct {
    WatchlistId int `json:"watchlistId"`
    SecurityId int `json:"securityId"`
}

func NewWatchlistItem(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args NewWatchlistItemArgs
	err := json.Unmarshal(rawArgs, &args)
	if err != nil {
		return nil, fmt.Errorf("m0ivn0d %v", err)
	}
	var watchlistId int
	err = conn.DB.QueryRow(context.Background(), "INSERT into watchlistItems (securityId,watchlistId) values ($1,$2) RETURNING watchlistId", args.SecurityId, args.WatchlistId).Scan(&watchlistId)
	if err != nil {
		return nil, err
	}
	return watchlistId, err
}
