package data

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
    "github.com/jackc/pgx/v4"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
)

type ActiveSecurity struct {
	securityId           int
	ticker               string
	cik                  string
	figi                 string
	tickerActivationDate time.Time
    falseDelist bool
}

func validateTickerString(ticker string) bool {
    if strings.Contains(ticker, "."){
        return false
    }
	for _, char := range ticker {
		if unicode.IsLower(char) {
			return false
		}
	}
	return true
}

func diff(firstSet, secondSet map[string]models.Ticker) ([]models.Ticker, []models.Ticker, []models.Ticker) {
	additions := []models.Ticker{}
	removals := []models.Ticker{}
    figiChanges := []models.Ticker{}
	for ticker, sec := range firstSet {
		if yesterdaySec, found := secondSet[ticker]; !found {
			additions = append(additions, sec)
		}else{
            if yesterdaySec.CompositeFIGI != sec.CompositeFIGI{
                figiChanges = append(figiChanges,sec)
            }
        }
	}
	for ticker, sec := range secondSet {
		if _, found := firstSet[ticker]; !found {
			removals = append(removals, sec)
        }
	}

	return additions, removals, figiChanges
}
func dataExists(client *polygon.Client, ticker string, fromDate string, toDate string) bool {
	timespan := models.Timespan("day")
    fromMillis, err := MillisFromDatetimeString(fromDate)
    if err != nil {
        fmt.Println(fromDate)
    }
    toMillis, err := MillisFromDatetimeString(toDate)
    if err != nil {
        fmt.Println(toDate)
    }
	params := models.ListAggsParams{
		Ticker:     ticker,
		Multiplier: 1,
		Timespan:   timespan,
		From:       fromMillis,
		To:         toMillis,
	}
	iter := client.ListAggs(context.Background(), &params)
    for iter.Next(){
        return true
    }
    return false
}

func toFilteredMap(tickers []models.Ticker) map[string]models.Ticker {
	tickerMap := make(map[string]models.Ticker)
    for _,sec := range(tickers){
        if validateTickerString(sec.Ticker){
            tickerMap[sec.Ticker] = sec
        }
    }
	return tickerMap
}

func initTickerDatabase(conn *Conn) error {
    test := false
    if true{
        query := fmt.Sprintf("TRUNCATE TABLE securities RESTART IDENTITY CASCADE")
        _, err := conn.DB.Exec(context.Background(), query)
        if err != nil {
            panic(err)
        }
    }
    var startDate time.Time
	if test{
        startDate = time.Date(2004, 11, 30, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
    }else{
        startDate = time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
    }
    activeYesterday := make(map[string]models.Ticker) //posibly change to get filtereMap (Alltickers) of startdate.SubDate(0,0,1)
	for currentDate := startDate; currentDate.Before(time.Now()); currentDate = currentDate.AddDate(0, 0, 1) {
		currentDateString := currentDate.Format("2006-01-02")
        yesterdayDateString := currentDate.AddDate(0,0,-1).Format("2006-01-02")
        polyTickers := AllTickers(conn.Polygon, currentDateString)
        activeToday := toFilteredMap(polyTickers)
        additions, removals, figiChanges := diff(activeToday,activeYesterday)
        if test{
            fmt.Printf("%s: %d additions %d removals\n",currentDateString,len(additions),len(removals))
        }
        for _, sec := range(figiChanges){
            cmdTag, err := conn.DB.Exec(context.Background(),"UPDATE securities set figi = $1 where ticker = $2 and maxDate is NULL",sec.CompositeFIGI,sec.Ticker)
            if err != nil || cmdTag.RowsAffected() == 0 {
                fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                fmt.Printf("2mi0d %v \n",err)
            }else if test{
                fmt.Printf("figi change: %s\n",sec.Ticker)
            }
        }
        for _,sec := range(additions){
            var diagnoses string
            var maxDate time.Time
            tickerMatchFromFigiLookup := ""
            if sec.CompositeFIGI != ""{
                err := conn.DB.QueryRow(context.Background(),"SELECT ticker FROM securities where figi = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1",sec.CompositeFIGI).Scan(&tickerMatchFromFigiLookup)
                if err != nil && err != pgx.ErrNoRows {
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                    fmt.Printf("32gerf %v \n",err)
                }
            }
            err := conn.DB.QueryRow(context.Background(),"SELECT COALESCE(maxDate, '2200-12-31') FROM securities where ticker = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1",sec.Ticker).Scan(&maxDate)
            var tickerInDb bool
            if err == pgx.ErrNoRows {
                tickerInDb = false
            }else{
                tickerInDb = true
            }
            maxDateString := maxDate.Format("2006-01-01")

            if err != nil && err != pgx.ErrNoRows {
                fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                fmt.Printf("vm2d: %v\n",err)
            }
            if tickerMatchFromFigiLookup != ""{ //means there is a figi and it exists in db at some point
                if tickerMatchFromFigiLookup == sec.Ticker{ //figi and ticker matches
                    diagnoses = "false delist"
                }else{ //ticker is different but figi found means trackable ticker change
                    diagnoses = "ticker change"
                }
            }else if (tickerInDb && dataExists(conn.Polygon,sec.Ticker,maxDateString,yesterdayDateString)){ //no figi but ticker exists and there is data since supposed delist
                diagnoses = "false delist" 
            }else{
                diagnoses = "listing"
            }
            if diagnoses == "listing" {
                _,err = conn.DB.Exec(context.Background(),"INSERT INTO securities (figi, ticker, minDate) values ($1,$2,$3)",sec.CompositeFIGI, sec.Ticker, currentDateString)
                if err != nil {
                    fmt.Println("swedfo")
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString," ",maxDateString)
                }else if test{
                  fmt.Printf("listed %s\n",sec.Ticker)
                }
            }else if diagnoses == "ticker change"{
                err = conn.DB.QueryRow(context.Background(),"SELECT 1 FROM securities where ticker = $1 and figi = $2",sec.Ticker,sec.CompositeFIGI).Scan(new(interface{}))
                if err == pgx.ErrNoRows {
                    fmt.Printf("Inserting: Ticker: %s, FIGI: %s, MinDate: %s\n", sec.Ticker, sec.CompositeFIGI, currentDateString)
                    _,err = conn.DB.Exec(context.Background(),"INSERT INTO securities (securityId, figi, ticker, minDate) SELECT securityID, figi, $1, $2 from securities where figi = $3",sec.Ticker,yesterdayDateString,sec.CompositeFIGI)
                    if err != nil {
                        fmt.Printf("mh93: %v\n",err)
                        fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString," ",maxDateString)
                        fmt.Println(yesterdayDateString)
                    }else if test{
                        fmt.Printf("ticker change %s\n",sec.Ticker)
                    }
                }else if err != nil{
                    fmt.Printf("29jgk: %v\n",err)
                        fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString," ",maxDateString)
                }else{
                    fmt.Printf("skipped supposed ticker change %s %s",sec.Ticker,sec.CompositeFIGI)
                }
            }else if diagnoses == "false delist"{
                _,err = conn.DB.Exec(context.Background(),"UPDATE securities set maxDate = NULL where ticker = $1 AND maxDate = (SELECT max(maxDate) FROM securities WHERE ticker = $1)",sec.Ticker)
                if err != nil {
                    fmt.Printf("swe9fo: %v\n",err)
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString," ",maxDateString)
                }else if test{
                    fmt.Printf("false delist %s\n",sec.Ticker)
                }
            }
            
        }
        for _,sec := range(removals){
            _,err:= conn.DB.Exec(context.Background(),"UPDATE securities SET maxDate = $1 where ticker = $2 and maxDate is NULL",yesterdayDateString,sec.Ticker)
            if err != nil {
                fmt.Println("91md")
                fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
            }else if test{
               fmt.Printf("delisted %s\n",sec.Ticker)
            }
        }
        //TODO check for figi cahnges
        yesterdayDateString = currentDateString
        activeYesterday = activeToday
    }
	return nil
}
