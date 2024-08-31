package data

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v4"
)

type ActiveSecurity struct {
	securityId           int
	ticker               string
	cik                  string
	figi                 string
	tickerActivationDate time.Time
}
type god struct {
	ticker string
	cik    string
	typ    string
}

func writeSecurity(conn *Conn, sec *ActiveSecurity, date *time.Time) error {
	var maxDate interface{}
	if date == nil {
		maxDate = nil
	} else {
		maxDate = *date
	}
	_, err := conn.DB.Exec(context.Background(), "INSERT INTO securities (securityid, ticker, figi, minDate, maxDate) VALUES ($1, $2, $3, $4, $5)", sec.securityId, sec.ticker, sec.figi, sec.tickerActivationDate, maxDate)
	if err != nil {
		//fmt.Printf("Error at 2kfpe, %v \n", err)
		fmt.Print("ERROR: ", err, " , ", sec.securityId, " , ", sec.ticker, " , ", sec.figi, " , ", sec.tickerActivationDate, " , ", date, "\n")
	}
	return err
}

func findTickerFromFigi(activeSecurities map[string]ActiveSecurity, figi string) string {
	for ticker, security := range activeSecurities {
		if security.figi == figi {
			return ticker
		}
	}
	return ""
}
func containsLowercase(s string) bool {
	for _, char := range s {
		if unicode.IsLower(char) {
			return true
		}
	}
	return false
}
func initTickerDatabase(conn *Conn) error {
	startDate := time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
	//startDate := time.Date(2024, 8, 20, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
	//currentDate := startDate
	activeSecuritiesRecord := make(map[string]ActiveSecurity) // indexed by ticker
	nextSecurityId := 1
    prevActiveSecurities := 0 //used to catch polygon supposed mass delisting
    for currentDate := startDate; currentDate.Before(time.Now()); currentDate = currentDate.AddDate(0, 0, 1) {
		currentDateString := currentDate.Format("2006-01-02")
		polygonActiveSecurities := AllTickers(conn.Polygon, currentDateString)
        supposedDelistings := prevActiveSecurities - len(polygonActiveSecurities)
        if (supposedDelistings > 50){ // check if polygon data is bad for that day (tons of tickers not active / missing)
            fmt.Printf("skipped %d delistings on %s\n",supposedDelistings,currentDateString)
            continue
        }
        prevActiveSecurities = len(polygonActiveSecurities)
		polygonActiveTickers := make(map[string]interface{}) //doesnt actually store a value besides the key so just use empty interface
		listings := 0
		delistings := 0
		tickerChanges := 0
		missed := 0
		figiChanges := 0
		// Loop through the active stocks for the given day
		for _, polySec := range polygonActiveSecurities {
			polygonActiveTickers[polySec.Ticker] = struct{}{} //empty anonymous struct cause just use key to check existence not retrieve valu
			if strings.Contains(polySec.Ticker, ".") ||
				containsLowercase(polySec.Ticker) {
				missed++
				continue
			}
			if sec, exists := activeSecuritiesRecord[polySec.Ticker]; !exists {
				var tickerChange = false
				var prevTicker string
				if polySec.CompositeFIGI != "" {
					if prevTicker = findTickerFromFigi(activeSecuritiesRecord, polySec.CompositeFIGI); prevTicker != "" {
						// This fixes the case where FB is counted as active and it tries to delist the new ticker, META everyday
						err := conn.DB.QueryRow(context.Background(), "SELECT * from securities WHERE ticker = $1 AND figi = $2", polySec.Ticker, polySec.CompositeFIGI).Scan()
						if err != pgx.ErrNoRows {
							continue
						} else {
							tickerChange = true
						}
					}
				}
				if tickerChange { //change
					prevTickerRecord := activeSecuritiesRecord[prevTicker]
					err := writeSecurity(conn, &prevTickerRecord, &currentDate)
					if err != nil {
						//fmt.Printf("ticker change error: %v", err)
						//return err
					}
					//               fmt.Printf("changed %s -> %s\n", prevTickerRecord.ticker, polySec.Ticker)
					delete(activeSecuritiesRecord, prevTickerRecord.ticker)
					prevTickerRecord.ticker = polySec.Ticker
					prevTickerRecord.tickerActivationDate = currentDate
					activeSecuritiesRecord[polySec.Ticker] = prevTickerRecord
					tickerChanges++
				} else { //listing

					activeSecuritiesRecord[polySec.Ticker] = ActiveSecurity{
						securityId:           nextSecurityId,
						ticker:               polySec.Ticker,
						cik:                  polySec.CIK,
						figi:                 polySec.CompositeFIGI,
						tickerActivationDate: currentDate,
					}
					nextSecurityId++
					listings++
					//                fmt.Printf("listed %s\n",polySec.Ticker)
				}
			} else if polySec.CompositeFIGI != "" && sec.figi != polySec.CompositeFIGI { //figi change
				err := conn.DB.QueryRow(context.Background(), "SELECT * from securities WHERE figi = $1 AND securityId = $2", polySec.CompositeFIGI, sec.securityId).Scan()
				if err == pgx.ErrNoRows {
					//fmt.Printf("ticker %s figi change: %s -> %s\n",sec.ticker, sec.figi, polySec.CompositeFIGI)
					if err := writeSecurity(conn, &sec, &currentDate); err != nil {
						//fmt.Printf("figi change error: %v", err)
					}
					sec.figi = polySec.CompositeFIGI
					sec.tickerActivationDate = currentDate
					activeSecuritiesRecord[polySec.Ticker] = sec
					figiChanges++
				}
			}
		}
		for ticker, security := range activeSecuritiesRecord {
			if _, exists := polygonActiveTickers[ticker]; !exists { //delisted becuase already handled ticker chantges as best as possibel
				delete(activeSecuritiesRecord, ticker)
				err := writeSecurity(conn, &security, &currentDate)
				if err != nil {
					//return err
				}
				//          fmt.Printf("delisted: %s\n", security.ticker)
				delistings++
			}
		}
		//fmt.Printf("%d active securities, %d listings, %d delistings, %d ticker changes, %d figi changes, %d missed, on %s ------------------------ \n", len(activeSecuritiesRecord), listings, delistings, tickerChanges, figiChanges, missed, currentDateString)

	}
	for _, security := range activeSecuritiesRecord {
		writeSecurity(conn, &security, nil)
	}
	return nil
}
