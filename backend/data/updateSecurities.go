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
    _ "github.com/lib/pq"
    "database/sql"
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

	// Trackers to ensure no duplicates
	usedTickers := make(map[string]struct{})

	// Process additions and figi changes
	for ticker, sec := range firstSet {
		if yesterdaySec, found := secondSet[ticker]; !found {
			// Check if already in the additions set
			if _, exists := usedTickers[ticker]; !exists {
				additions = append(additions, sec)
				usedTickers[ticker] = struct{}{}
			}else{
                    fmt.Printf("duplicate %s\n",ticker)
            }
		} else {
			if yesterdaySec.CompositeFIGI != sec.CompositeFIGI {
				// Check if already in the figi changes set
				if _, exists := usedTickers[ticker]; !exists {
					figiChanges = append(figiChanges, sec)
					usedTickers[ticker] = struct{}{}
				}else{
                    fmt.Printf("duplicate %s\n",ticker)
                }
			}
		}
	}

	// Process removals
	for ticker, sec := range secondSet {
		if _, found := firstSet[ticker]; !found {
			// Check if already in the removals set
			if _, exists := usedTickers[ticker]; !exists {
				removals = append(removals, sec)
				usedTickers[ticker] = struct{}{}
			}else{
                    fmt.Printf("duplicate %s\n",ticker)
                }
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
    return iter.Next()
    /*c := 0
    for iter.Next(){
        if (c > 1){
            return true
        }
        c++
    }
    return false*/
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

func contains(slice []string, item string) bool {
	for _, str := range slice {
		if str == item {
			return true
		}
	}
	return false
}

func initTickerDatabase(conn *Conn) error {
    //fmt.Print(dataExists(conn.Polygon,"VBR","2003-09-24","2004-01-29"))
    //return nil
    test := false
    if true{
        query := fmt.Sprintf("TRUNCATE TABLE securities RESTART IDENTITY CASCADE")
        _, err := conn.DB.Exec(context.Background(), query)
        if err != nil {
            return fmt.Errorf("unable to truncate table for test")
        }
    }
    var startDate time.Time
	if test{
        startDate = time.Date(2004, 11, 30, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
    }else{
        startDate = time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
    }
    activeYesterday := make(map[string]models.Ticker) //posibly change to get filtereMap (Alltickers) of startdate.SubDate(0,0,1)
    dateFormat := "2006-01-02"
	for currentDate := startDate; currentDate.Before(time.Now()); currentDate = currentDate.AddDate(0, 0, 1) {
		currentDateString := currentDate.Format(dateFormat)
        yesterdayDateString := currentDate.AddDate(0,0,-1).Format(dateFormat)
        polyTickers, err := AllTickers(conn.Polygon, currentDateString)
        if err != nil {
            return fmt.Errorf("423n %v", err)
        }
        activeToday := toFilteredMap(polyTickers)
        additions, removals, figiChanges := diff(activeToday,activeYesterday)
        if test{
            fmt.Printf("%s: %d additions %d removals\n",currentDateString,len(additions),len(removals))
        }
        for _, sec := range(figiChanges){
            cmdTag, err := conn.DB.Exec(context.Background(),"UPDATE securities set figi = $1 where ticker = $2 and maxDate is NULL",sec.CompositeFIGI,sec.Ticker)
            if err != nil || cmdTag.RowsAffected() == 0 {
                fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                fmt.Printf("22m9i0d %v \n",err)
            }else if test{
                fmt.Printf("figi change: %s\n",sec.Ticker)
            }
        }
        for _,sec := range(additions){
            diagnoses := make([]string,0)
            var maxDate sql.NullTime
            tickerInDB := ""
            if sec.CompositeFIGI != ""{ //if figi exists
                //err := conn.DB.QueryRow(context.Background(),"SELECT ticker, maxDate FROM securities where figi = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1",sec.CompositeFIGI).Scan(&tickerInDB,&maxDate)
                rows, err := conn.DB.Query(context.Background(),"SELECT ticker, maxDate FROM securities where figi = $1 order by COALESCE(maxDate, '2200-01-01') DESC",sec.CompositeFIGI)//.Scan(&tickerInDB,&maxDate)
                if rows.Next(){
                    err = rows.Scan(&tickerInDB,&maxDate)
                    if err != nil {
                        fmt.Printf("v2n92 %v\n",err)
                        continue
                    }
                    if tickerInDB == sec.Ticker {
                        diagnoses = append(diagnoses,"false delist")
                    }else{
                        prevListing := false
                        for rows.Next(){
                            var ticker string
                            var date sql.NullTime
                            err = rows.Scan(&ticker,&date)
                            if err != nil {
                                fmt.Printf("02niv %v\n",err)
                                prevListing = true
                                break
                            }
                            if ticker == sec.Ticker{
                                prevListing = true
                                break
                            }
                        }
                        if !prevListing {
                            diagnoses = append(diagnoses,"ticker change")
                            if (dataExists(conn.Polygon,sec.Ticker,maxDate.Time.Format(dateFormat),yesterdayDateString)){
                                diagnoses = append(diagnoses,"false delist")
                            }
                        }else{
                            fmt.Printf("skipped duplicate listing %s %s\n",sec.Ticker,sec.CompositeFIGI)
                        }

                    }

                }else if err == nil { //figi doesnt exist in db
                    tickerInDB = sec.Ticker
                    err := conn.DB.QueryRow(context.Background(),"SELECT maxDate from securities where ticker = $1",sec.Ticker).Scan(&maxDate)
                    if err == nil {
                        if dataExists(conn.Polygon,sec.Ticker,maxDate.Time.Format(dateFormat),yesterdayDateString){
                            diagnoses = append(diagnoses,"false delist")
                            diagnoses = append(diagnoses,"figi change")
                        }else{
                            diagnoses = append(diagnoses,"listing")
                        }
                    }else if err == pgx.ErrNoRows{
                        diagnoses = append(diagnoses,"listing")
                    }else{
                        fmt.Printf("n9i0v2 %v\n",err)
                        fmt.Println(sec.Ticker)
                    }
                }else{ //valid error
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                    fmt.Printf("32gerf %v \n",err)
                }
                rows.Close()
            }else{ //deal with tickers
                tickerInDB = sec.Ticker
                var figiInDB string
                err := conn.DB.QueryRow(context.Background(),"SELECT figi, maxDate FROM securities where ticker = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1",sec.Ticker).Scan(&figiInDB,&maxDate)
                if (err == nil){ // ticker exists in db and data exists
                    if dataExists(conn.Polygon,sec.Ticker,maxDate.Time.Format(dateFormat),yesterdayDateString){
                        diagnoses = append(diagnoses, "false delist")
                    }else{
                        diagnoses = append(diagnoses, "listing")
                    }
                }else if err == pgx.ErrNoRows{
                    diagnoses = append(diagnoses,"listing")
                }else{
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                    fmt.Printf("321mf %v \n",err)
                }
            }
            if contains(diagnoses, "false delist") {
                cmdTag,err := conn.DB.Exec(context.Background(),"UPDATE securities set maxDate = NULL where ticker = $1 AND (maxDate is null or maxDate = (SELECT max(maxDate) FROM securities WHERE ticker = $1))",tickerInDB)
                if err != nil || cmdTag.RowsAffected() == 0 {
                    fmt.Printf("swe9fo: %v\n",err)
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                }else if test{
                    fmt.Printf("false delist %s\n",sec.Ticker)
                }
            }
            if contains(diagnoses, "ticker change"){
                _,err = conn.DB.Exec(context.Background(),"INSERT INTO securities (securityId, figi, ticker, minDate) SELECT securityID, figi, $1, $2 from securities where figi = $3",sec.Ticker,currentDateString,sec.CompositeFIGI)
                if err != nil {
                    fmt.Printf("mh93: %v\n",err)
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                }else if test{
                    fmt.Printf("ticker change %s\n",sec.Ticker)
                }
            }
            if contains(diagnoses, "figi change"){
                cmdTag, err := conn.DB.Exec(context.Background(),"UPDATE securities set figi = $1 where ticker = $2 and (maxDate is NULL or maxDate = (SELECT max(maxDate) FROM securities where ticker = $2) )",sec.CompositeFIGI,sec.Ticker)
                if err != nil || cmdTag.RowsAffected() == 0 {
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                    fmt.Printf("2mi0d %v \n",err)
                }else if test{
                    fmt.Printf("figi change: %s\n",sec.Ticker)
                }
            }
            if contains(diagnoses,"listing") {
                _,err = conn.DB.Exec(context.Background(),"INSERT INTO securities (figi, ticker, minDate) values ($1,$2,$3)",sec.CompositeFIGI, sec.Ticker, currentDateString)
                if err != nil {
                    fmt.Println("swedfo")
                    fmt.Println(sec.Ticker," ",sec.CompositeFIGI," ",currentDateString)
                }else if test{
                  fmt.Printf("listed %s\n",sec.Ticker)
                }
            }
            
        }
        for _,sec := range(removals){
            cmdTag,err:= conn.DB.Exec(context.Background(),"UPDATE securities SET maxDate = $1 where ticker = $2 and maxDate is NULL",yesterdayDateString,sec.Ticker)
            if err != nil || cmdTag.RowsAffected() == 0 {
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
