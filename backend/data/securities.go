
package data

import (
    "fmt"
    "time"
)

/*type Security struct {
	internalSecurityId           int
	internalCompanyId            int
	ticker                       string
	cik                          string
	figi                         string
	activationDate               string
	deactivationDate             string
	precedingInternalSecurityId  int
	succeedingInternalSecurityId int
}*/

type ActiveSecurity struct {
    securityId    int
	ticker        string
	cik           string
	figi          string
    //preceedingSecurityId  int
	activationDate time.Time
}

type god struct {
    ticker string
    cik string
    typ string
}


/*func writeSecurity(ActiveSecurity *ActiveSecurity, date time.Time, succeedingId *int) error {
    return nil
}*/
func writeSecurity(ActiveSecurity *ActiveSecurity, date time.Time) error {
    return nil
}

func initTickerDatabase(conn *Conn) {
	startDate := time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
	currentDate := startDate
	//securityDatabase := make(map[string]Security)// indexed
	activeSecuritiesRecord := make(map[string]ActiveSecurity)    // indexed by CIK or FIGI
    nextSecurityId := 1
	for currentDate.Before(time.Now()) {
		currentDateString := currentDate.Format("2006-01-02")
		polygonActiveSecurities := AllTickers(conn.Polygon, currentDateString)
        polygonActiveSecuritiesById := make(map[string]interface{}) //doesnt actually store a value besides the key so just use empty interface
        listings := 0
        delistings := 0
        tickerChanges := 0
        missed := 0
        resusedSIKS := make(map[string][]god)
		for _, polySec := range polygonActiveSecurities {
            var securityId string
            if (polySec.CompositeFIGI != ""){
                securityId = polySec.CompositeFIGI
            }else if (polySec.Type != "ETF" && polySec.Type != "" && polySec.CIK != ""){ //shit ass ident god
                fmt.Println("gid")
                //continue
                //if _, exists := resusedSIKS[polySec.CIK]; exists {
                    list := resusedSIKS[polySec.CIK]
                    list = append(list,god{ticker:polySec.Ticker,cik:polySec.CIK, typ: polySec.Type})
                    resusedSIKS[polySec.CIK] = list
                //}

                continue
                
            }else{
                if (polySec.Type == "") {
                    fmt.Printf("missed etf %s \n", polySec.Ticker)
                }
                missed ++
                //fmt.Printf("ticker %s id missing\n",ticker)
                continue
            }
            //fmt.Printf("ticker: %s security id: %s\n", ticker, securityId)
			if security, exists := activeSecuritiesRecord[securityId]; exists {
				if security.ticker != polySec.Ticker { //ticker change
                   /* fmt.Print("old ticker bytes: ")
                    for _, b := range []byte(security.ticker) {
                        fmt.Printf("0x%x ", b)
                    }
                    fmt.Println()
                    fmt.Print("new ticker bytes: ")
                    for _, b := range []byte(ticker) {
                        fmt.Printf("0x%x ", b)
                    }*/
                    fmt.Println()
                    oldTicker := security.ticker
                    security.ticker = polySec.Ticker
                    //security.preceedingSecurityId = security.securityId
                    //succeedingId :=  security.securityId
                    //writeSecurity(&security, currentDate,  &succeedingId)
                    writeSecurity(&security,currentDate)
                    activeSecuritiesRecord[securityId] = security //security is a copy so have to update the map 
                    fmt.Printf("ticker change: %s -> %s ID: %s\n", oldTicker, polySec.Ticker, securityId)
                    tickerChanges ++
                }
			} else { //listing
                activeSecuritiesRecord[securityId] = ActiveSecurity{
                    securityId: nextSecurityId,
                    ticker:polySec.Ticker,
                    cik: polySec.CIK,
                    figi: polySec.CompositeFIGI,
                    activationDate: currentDate,
                }
                nextSecurityId ++
                //fmt.Printf("listing: %s\n", polySec.Ticker)
                listings ++
			}
            polygonActiveSecuritiesById[securityId] = struct{}{} //empty anonymous struct cause just use key to check existence not retrieve value
		}
    
        for securityId, security := range activeSecuritiesRecord {
            if _, exists := polygonActiveSecuritiesById[securityId]; !exists { //delisted
                delete(activeSecuritiesRecord, securityId)
                //writeSecurity(&security, currentDate, nil)
                writeSecurity(&security, currentDate)
                fmt.Printf("delisting: %s\n", security.ticker)
                delistings ++
            }
        }
		currentDate = currentDate.AddDate(0, 0, 1)
        fmt.Printf("%d active securities, %d listings, %d delistings, %d ticker changes, %d missed, on %s ------------------------ \n",len(activeSecuritiesRecord),listings,delistings,tickerChanges,missed,currentDateString )
        for _, lis := range resusedSIKS {
            if len(lis) > 1 {
                for _, gosh := range lis {
                    fmt.Println(gosh)
                }
            }
        }

        var god string
        fmt.Scanf("%s", &god)
	}
}
