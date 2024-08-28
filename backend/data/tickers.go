package data

import (
	"context"
	"fmt"
	"time"
)

// needs fixing
func updateTickerDatabase(conn *Conn, cik string) string {
	if cik != "" {
		tickerString, tickerRequestError := GetTickerFromCIK(conn.Polygon, cik)
		if tickerRequestError != nil {
			return fmt.Sprintf("error with updateTickerDatabase(); invalid CIK: %s", cik)
		}
		row := conn.DB.QueryRow(context.Background(), "SELECT ticker FROM securities WHERE cik = $1", cik)
		if err := row.Scan(); err != nil {
			conn.DB.Exec(context.Background(), "insert into securities (cik, ticker) values ($1, $2)", cik, tickerString)
			return "success"
		} else {
			conn.DB.Exec(context.Background(), "UPDATE securities SET ticker = $1 WHERE cik = $2", tickerString, cik)
		}
	}
	tickers := AllTickers(conn.Polygon, "")
	for _, ticker := range tickers {
		if ticker.CIK == "" && ticker.CompositeFIGI == "" {
			fmt.Printf("Processing Ticker: %s, CIK: {%s}, FIGI: {%s}\n", ticker.Ticker, ticker.CIK, ticker.CompositeFIGI)
		}

		var returnedTicker string
		err := conn.DB.QueryRow(context.Background(), "SELECT ticker FROM securities WHERE cik = $1", ticker.CIK).Scan(&returnedTicker)
		//fmt.Println(err)
		if err != nil {
			_, err := conn.DB.Exec(context.Background(), "insert into securities (cik, ticker) values ($1, $2)", ticker.CIK, ticker.Ticker)
			if err != nil {
				fmt.Printf("NewInstance execution failed: %s", ticker.Ticker)
				return ""
			}
		} else {
			if returnedTicker != ticker.Ticker {
				conn.DB.Exec(context.Background(), "UPDATE securities SET ticker = $1 WHERE cik = $2", ticker.Ticker, ticker.CIK)
			}
		}
	}
	return "success"
}
type TickerDatabaseObject struct {
	Ticker string 
	CIK string 
	figi string 
	tickerStartDate string 
	tickerEndDate string 

}
func initTickerDatabase(conn *Conn) {
	currentDate := time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC)
	var currentList []TickerDatabaseObject; 
	for {
		if currentDate.Format(time.DateOnly) == time.Now().Format(time.DateOnly) {
			break;
		}
		currentDateString := currentDate.Format(time.DateOnly)
		tickers := AllTickers(conn.Polygon, currentDateString) 
		for _, ticker := range tickers {
			currentList.
		}

		currentDate = currentDate.AddDate(0, 0, 1)
	}

}
