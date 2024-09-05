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
	params := models.ListAggsParams{
		Ticker:     ticker,
		Multiplier: 1,
		Timespan:   timespan,
		From:       MillisFromDatetimeString(fromDate),
		To:         MillisFromDatetimeString(toDate),
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
    if test{
        query := fmt.Sprintf("TRUNCATE TABLE securities RESTART IDENTITY CASCADE")
        _, err := conn.DB.Exec(context.Background(), query)
        if err != nil {
            panic(err)
        }
    }
    var startDate time.Time
	if test{
        startDate = time.Date(2016, 5, 16, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
    }else{
        startDate = time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
    }
    yesterdayDateString := ""
    activeYesterday := make(map[string]models.Ticker) //posibly change to get filtereMap (Alltickers) of startdate.SubDate(0,0,1)
	for currentDate := startDate; currentDate.Before(time.Now()); currentDate = currentDate.AddDate(0, 0, 1) {
		currentDateString := currentDate.Format("2006-01-02")
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
            tickerMatch := ""
            if sec.CompositeFIGI != ""{
                err := conn.DB.QueryRow(context.Background(),"SELECT ticker FROM securities where figi = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1",sec.CompositeFIGI).Scan(&tickerMatch)
                if err != nil && err != pgx.ErrNoRows {
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                    fmt.Printf("32gerf %v \n",err)
                    continue
                }

            }
            err := conn.DB.QueryRow(context.Background(),"SELECT COALESCE(maxDate, CURRENT_TIMESTAMP) FROM securities where ticker = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1",sec.Ticker).Scan(&maxDate)
            if err != nil && err != pgx.ErrNoRows {
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                fmt.Printf("vm2d: %v\n",err)
            }
            maxDateString := maxDate.Format("2008-01-01")
            if tickerMatch != ""{
                if tickerMatch == sec.Ticker{
                    diagnoses = "false delist"
                }else{
                    diagnoses = "ticker change"
                }
            }else if (!maxDate.IsZero() && dataExists(conn.Polygon,sec.Ticker,maxDateString,yesterdayDateString)){
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
                _,err = conn.DB.Exec(context.Background(),"INSERT INTO securities (securityId, figi, ticker, minDate) SELECT securityID, figi, $2, $3 from securities where figi = $1",sec.CompositeFIGI,sec.Ticker,yesterdayDateString)
                if err != nil {
                    fmt.Printf("mh93: %v\n",err)
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString," ",maxDateString)
                }else if test{
                    fmt.Printf("ticker change %s\n",sec.Ticker)
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
            _,err:= conn.DB.Exec(context.Background(),"UPDATE securities SET maxDate = $1 where ticker = $2 and maxDate = NULL",yesterdayDateString,sec.Ticker)
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
