package data

/*import (
	"context"
	"fmt"
	"time"
)*/

// needs fixing
/*func updateTickerDatabase(conn *Conn, cik string) string {
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
}*/
/*type Security struct {
    interalSecurityId int
    internalCompanyId int
	ticker string 
	cik string 
	figi string 
	activationDate string 
    deactivationDate string
    precedingInternalSecurityId int
    succeedingInternalSecurityId int
}
type ActiveSecurity struct { //indexed by ticker
    securityId int
    cik string
    figi string
    activationDate time.Time
}
//track ticker changes for given company id
//track delisting of company
//track listing of company
/*func contains (list []string, target string) bool {
    for _. item := range list {
        if item == target {
            return true
        }
    } 
    return false
}

func initTickerDatabase(conn *Conn) {
    startDate := time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC) //will need to be set to most recent secuties database update
	currentDate := startDate
    allSecurities := make(map[int]Security)
    activeSecurityIds := make([]string)
    activeSecurities := make(map[string]ActiveTicker)
	for (currentDate.Before(time.Now())){
		//currentDateString := currentDate.Format(time.DateOnly)
		currentDateString := currentDate.Format("2006-01-02")
		activeSecuritiesAtDate := AllTickers(conn.Polygon, currentDateString) 
        //newActiveTickers := make(map[int]Security)
        for _, sec := range activeSecuritiesAtDate {
            sik := sec.SIK
            figi := sec.FIGI
            ticker := sec.ticker
            /*var securityId int
            if (sik != nil) {
                securityId = sik
            } else if (figi != nil){
                securityId = figi
            } else {
                fmt.Printf("%s has no figi or sik", sec.ticker)
                continue
            }*/
/*            //addition
            if _, exists := activeTickers[ticker]; !exists {
                if contains(activeSecurityIds)
                        

                newSecurity = ActiveTickers{ cik: cik, figi: figi, activationDate: currentDateString}
                activeTickers = append(activeTickers, newSecurity)

        //deletions
        for securityId, security := range activeTickers { 
            if _, exists := newActiveTickers[companyId]; !exists {
                deactivatedSecurity = Security{
                    internalSecurityId: nextInternalSecurityId ++,
                    ticker: security.ticker,
                    cik: secruity.cik,
                    figi: security.figi,
                    activationDate: security.activationDate,
                    deactivationDate: currentDateString,
                    precedingInternalSecurityId: 
                    succeddingInternalSecurityId: nil,
                }

		currentDate = currentDate.AddDate(0, 0, 1)
	}

    for securityId, security := range activeTickers {
    }

}


*/
/*
package main

import (
	"fmt"
	"time"
)

type Security struct {
	internalSecurityId           int
	internalCompanyId            int
	ticker                       string
	cik                          string
	figi                         string
	activationDate               string
	deactivationDate             string
	precedingInternalSecurityId  int
	succeedingInternalSecurityId int
}

type ActiveSecurity struct {
    securityId    string
	ticker        string
	cik           string
	figi          string
	activationDate time.Time
}

// Helper function to create a new Security from an ActiveTicker
func newSecurityFromActive(active ActiveTicker, securityId, companyId int) Security {
	return Security{
		internalSecurityId:           securityId,
		internalCompanyId:            companyId,
		ticker:                       active.ticker,
		cik:                          active.cik,
		figi:                         active.figi,
		activationDate:               active.activationDate.Format("2006-01-02"),
		deactivationDate:             "",
		precedingInternalSecurityId:  0,
		succeedingInternalSecurityId:  0,
	}
}

// Initialize ticker database
func initTickerDatabase(conn *Conn) {
	startDate := time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC)
	currentDate := startDate

	// Maps for active tickers
	tickerDatabase := make(map[int]Security)
	activeTickersById := make(map[string]ActiveTicker)    // Map indexed by CIK or FIGI
	activeTickersByTicker := make(map[string]ActiveTicker) // Map indexed by ticker

	for currentDate.Before(time.Now()) {
		currentDateString := currentDate.Format("2006-01-02")
		activeSecuritiesAtDate := AllTickers(conn.Polygon, currentDateString)
		newActiveTickersById := make(map[string]ActiveTicker)
		newActiveTickersByTicker := make(map[string]ActiveTicker)
		for _, sec := range activeSecuritiesAtDate {
            ticker := sec.Ticker
            cik := sec.CIK
            figi := sec.ComposeiteFIGI
            var securityId string
            if (cik != nil){
                securityId = cik
            }else if (figi != nil){
                securityId = figi
            }else{
                fmt.Printf("ticker %s has no usable id",ticker)
                continue
            }
			if activeTicker, exists := activeTickersById[securityId]; exists {
				// If the ID exists, check if the ticker has changed (ticker change)
				if activeTicker.ticker != ticker {
					fmt.Printf("Ticker change detected: %s -> %s for ID: %s\n", activeTicker.ticker, ticker, securityId)

					// Update ticker in active tickers
					activeTickersById[securityId] = ActiveTicker{
						ticker:        ticker,
						cik:           sec.cik,
						figi:          sec.figi,
						activationDate: currentDate,
					}
				}
			} else {
				// If the ID does not exist, it's a new listing
				fmt.Printf("New listing detected for ticker: %s\n", ticker)

				// Add the new ticker to active tickers
				activeTickersById[securityId] = ActiveTicker{
					ticker:        ticker,
					cik:           sec.cik,
					figi:          sec.figi,
					activationDate: currentDate,
				}

				// Create a new Security from ActiveTicker and add it to the database
				tickerDatabase[securityId] = newSecurityFromActive(activeTickersById[securityId], securityId, companyId)
			}

			// Add the ticker to the new active tickers for this day
			newActiveTickersById[securityId] = activeTickersById[securityId]
			newActiveTickersByTicker[ticker] = activeTickersById[securityId]
		}

		// Deletions: Deactivate old tickers or detect delistings
		for securityId, activeTicker := range activeTickersById {
			// If an ID no longer exists in the new active tickers list, it has been delisted
			if _, exists := newActiveTickersById[securityId]; !exists {
				// Update the deactivation date
				deactivatedSecurity := tickerDatabase[securityId]
				deactivatedSecurity.deactivationDate = currentDateString

				fmt.Printf("Delisting detected for ticker: %s\n", activeTicker.ticker)

				// Update the database
				tickerDatabase[securityId] = deactivatedSecurity

				// Remove from active tickers
				delete(activeTickersById, securityId)
				delete(activeTickersByTicker, activeTicker.ticker)
			}
		}

		// Move to the next day
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	// Print the final ticker database for verification
	for _, sec := range tickerDatabase {
		fmt.Printf("Ticker: %s, CIK: %s, Activation: %s, Deactivation: %s\n",
			sec.ticker, sec.cik, sec.activationDate, sec.deactivationDate)
	}
}

// Helper function to determine the unique Security ID (based on cik or figi)
func getSecurityId(sec Security) string {
	// This function should prioritize CIK or FIGI for creating a unique identifier
	// Adjust as needed
	if sec.cik != "" {
		return sec.cik
	} else if sec.figi != "" {
		return sec.figi
	}
	return sec.ticker // Fallback to ticker
}

// Placeholder for AllTickers function that fetches tickers for a date
func AllTickers(polygonClient interface{}, date string) []Security {
	// Simulate fetching data from an external API
	return []Security{
		{internalSecurityId: 1, internalCompanyId: 1, ticker: "AAPL", cik: "0000320193", figi: "", activationDate: "", deactivationDate: "", precedingInternalSecurityId: 0, succeedingInternalSecurityId: 0},
		{internalSecurityId: 2, internalCompanyId: 2, ticker: "MSFT", cik: "0000789019", figi: "", activationDate: "", deactivationDate: "", precedingInternalSecurityId: 0, succeedingInternalSecurityId: 0},
	}
}

// Placeholder for the Conn struct and other external dependencies
type Conn struct {
	Polygon interface{}
}

func main() {
	conn := &Conn{}
	initTickerDatabase(conn)
}

*/
