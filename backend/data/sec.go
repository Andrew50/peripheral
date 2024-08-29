
package data
import (
    "fmt"
    "strings"
    "time"
    "context"
    "unicode"
)

type ActiveSecurity struct {
    securityId    int
	ticker        string
	cik           string
	figi          string
	tickerActivationDate time.Time
}
type god struct {
    ticker string
    cik string
    typ string
}
func writeSecurity(conn *Conn, sec *ActiveSecurity, date *time.Time) error {
    var maxDate interface{}
    if date == nil {
        maxDate = nil
    }else{
        maxDate = *date
    }
    _, err := conn.DB.Exec(context.Background(), "INSERT INTO securities (securityId, ticker, figi, minDate, maxDate) VALUES ($1, $2, $3, $4, $5)", sec.securityId, sec.ticker, sec.figi, sec.tickerActivationDate, maxDate)
    if err != nil {
        fmt.Print(sec.securityId," ", sec.ticker," ", sec.figi," ", sec.tickerActivationDate," ", date, "\n")
    }
    return err
}

func findTickerFromFigi(activeSecurities map[string]ActiveSecurity, figi string) string {
    for ticker, security := range activeSecurities {
        if (security.figi == figi){
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
	currentDate := startDate
	activeSecuritiesRecord := make(map[string]ActiveSecurity)    // indexed by ticker
    nextSecurityId := 1
	for currentDate.Before(time.Now()) {
		currentDateString := currentDate.Format("2006-01-02")
		polygonActiveSecurities := AllTickers(conn.Polygon, currentDateString)
        polygonActiveTickers := make(map[string]interface{}) //doesnt actually store a value besides the key so just use empty interface
        listings := 0
        delistings := 0
        tickerChanges := 0
        missed := 0
		for _, polySec := range polygonActiveSecurities {
            if (strings.Contains(polySec.Ticker,".") ||
        containsLowercase(polySec.Ticker)){
                missed ++
                continue
            }
			if _, exists := activeSecuritiesRecord[polySec.Ticker]; !exists {
                var tickerChange = false
                var prevTicker string
                if (polySec.CompositeFIGI != ""){
                    if prevTicker = findTickerFromFigi(activeSecuritiesRecord, polySec.CompositeFIGI); prevTicker != "" {
                        tickerChange = true
                    }
                }
                if tickerChange { //change
                    prevTickerRecord := activeSecuritiesRecord[prevTicker]
                    err := writeSecurity(conn, &prevTickerRecord, &currentDate)
                    if err != nil {
                        //return err
                    }
     //               fmt.Printf("changed %s -> %s\n", prevTickerRecord.ticker, polySec.Ticker)
                    delete(activeSecuritiesRecord, prevTickerRecord.ticker)
                    prevTickerRecord.ticker = polySec.Ticker
                    prevTickerRecord.tickerActivationDate = currentDate
                    activeSecuritiesRecord[polySec.Ticker] = prevTickerRecord
                    tickerChanges ++
                }else{ //listing
                    activeSecuritiesRecord[polySec.Ticker] = ActiveSecurity{
                        securityId: nextSecurityId,
                        ticker:polySec.Ticker,
                        cik: polySec.CIK,
                        figi: polySec.CompositeFIGI,
                        tickerActivationDate: currentDate,
                    }
                    nextSecurityId ++
                    listings ++
    //                fmt.Printf("listed %s\n",polySec.Ticker)
                }
            }
            polygonActiveTickers[polySec.Ticker] = struct{}{} //empty anonymous struct cause just use key to check existence not retrieve valu
		}
    
        for ticker, security := range activeSecuritiesRecord {
            if _, exists := polygonActiveTickers[ticker]; !exists { //delisted becuase already handled ticker chantges as best as possibel
                delete(activeSecuritiesRecord, ticker)
                err := writeSecurity(conn,&security, &currentDate)
                if err != nil {
                    //return err
                }
      //          fmt.Printf("delisted: %s\n", security.ticker)
                delistings ++
            }
        }
		currentDate = currentDate.AddDate(0, 0, 1)
        fmt.Printf("%d active securities, %d listings, %d delistings, %d ticker changes, %d missed, on %s ------------------------ \n",len(activeSecuritiesRecord),listings,delistings,tickerChanges,missed,currentDateString )
      //  var god string
	}
    for _, security := range activeSecuritiesRecord {
        writeSecurity(conn,&security,nil)
    }
    return nil
}
