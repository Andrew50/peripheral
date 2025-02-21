package jobs

import (
	"backend/utils"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jackc/pgx/v4"

	"database/sql"

	"encoding/base64"

	_ "github.com/lib/pq"
	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
)

type ActiveSecurity struct {
	securityId           int
	ticker               string
	cik                  string
	figi                 string
	tickerActivationDate time.Time
	falseDelist          bool
}

func logAction(test bool, loop int, ticker string, targetTicker string, figi string, currentDate string, action string, err error) {
	if test {
		if err != nil {
			fmt.Printf("loop %-5d | time %s | ticker %-10s | targetTicker %-12s | figi %-20s | date %-10s | action %-20s | error %v\n",
				loop,                                     // Loop number (5 characters)
				time.Now().Format("2006-01-02 15:04:05"), // Time
				ticker,                                   // Ticker (10 characters)
				targetTicker,                             // Target Ticker (12 characters)
				figi,                                     // FIGI (20 characters)
				currentDate,                              // Date (10 characters)
				action,                                   // Action (20 characters)
				err)                                      // Error message
		}
		log.Printf("loop %-5d | ticker %-10s | targetTicker %-12s | figi %-20s | date %-10s | action %-35s | error %v\n",
			loop,         // Loop number (5 characters)
			ticker,       // Ticker (10 characters)
			targetTicker, // Target Ticker (12 characters)
			figi,         // FIGI (20 characters)
			currentDate,  // Date (10 characters)
			action,       // Action (20 characters)
			err)          // Error message
	}
}

func validateTickerString(ticker string) bool {
	if strings.Contains(ticker, ".") {
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
			} else {
				fmt.Printf("duplicate %s\n", ticker)
			}
		} else {
			if yesterdaySec.CompositeFIGI != sec.CompositeFIGI {
				// Check if already in the figi changes set
				if _, exists := usedTickers[ticker]; !exists {
					figiChanges = append(figiChanges, sec)
					usedTickers[ticker] = struct{}{}
				} else {
					fmt.Printf("duplicate %s\n", ticker)
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
			} else {
				fmt.Printf("duplicate %s\n", ticker)
			}
		}
	}

	return additions, removals, figiChanges
}
func dataExists(client *polygon.Client, ticker string, fromDate string, toDate string) bool {
	timespan := models.Timespan("day")
	fromMillis, err := utils.MillisFromDatetimeString(fromDate)
	if err != nil {
		fmt.Println(fromDate)
	}
	toMillis, err := utils.MillisFromDatetimeString(toDate)
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
	for _, sec := range tickers {
		if validateTickerString(sec.Ticker) {
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

func updateSecurities(conn *utils.Conn, test bool) error {
	var startDate time.Time
	//fmt.Print(dataExists(conn.Polygon,"VBR","2003-09-24","2004-01-29"))
	//return nil
	if test {
		shouldClearLog := true // Set this based on your requirements
		flags := os.O_CREATE | os.O_WRONLY
		if shouldClearLog {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_APPEND
		}

		file, err := os.OpenFile("app.log", flags, 0666)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer file.Close()

		log.SetOutput(file)
		query := fmt.Sprintf("TRUNCATE TABLE securities RESTART IDENTITY CASCADE")
		_, err = conn.DB.Exec(context.Background(), query)
		if err != nil {
			return fmt.Errorf("unable to truncate table for test")
		}
		startDate = time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
		//startDate = time.Date(2005, 1, 3, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
		//startDate = time.Date(2004, 11, 1, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
	} else {
		var startDateNull sql.NullTime
		err := conn.DB.QueryRow(context.Background(), "SELECT MAX(minDate) from securities").Scan(&startDateNull)
		if err != nil {
			return err
		}
		if startDateNull.Valid {
			startDate = startDateNull.Time
		} else {
			// Default to a specific date if no valid date is found
			startDate = time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC)
		}
		//startDate = time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC) //need to pull from a record of last update, prolly in db
	}
	dateFormat := "2006-01-02"
	yesterdayPolyTickers, err := utils.AllTickers(conn.Polygon, startDate.AddDate(0, 0, -1).Format(dateFormat))
	if err != nil {
		return fmt.Errorf("1j9v %v", err)
	}
	activeYesterday := toFilteredMap(yesterdayPolyTickers)
	for currentDate := startDate; currentDate.Before(time.Now()); currentDate = currentDate.AddDate(0, 0, 1) {
		currentDateString := currentDate.Format(dateFormat)
		yesterdayDateString := currentDate.AddDate(0, 0, -1).Format(dateFormat)
		polyTickers, err := utils.AllTickers(conn.Polygon, currentDateString)
		if err != nil {
			return fmt.Errorf("423n %v", err)
		}
		activeToday := toFilteredMap(polyTickers)
		additions, removals, figiChanges := diff(activeToday, activeYesterday)
		if test {
			//            fmt.Printf("%s: %d additions %d removals\n",currentDateString,len(additions),len(removals))
		}
		for i, sec := range figiChanges {
			cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE securities set figi = $1 where ticker = $2 and maxDate is NULL", sec.CompositeFIGI, sec.Ticker)
			if err != nil {
				logAction(test, i, sec.Ticker, "", sec.CompositeFIGI, currentDateString, "figi change 1", err)
			} else if cmdTag.RowsAffected() == 0 {
				logAction(test, i, sec.Ticker, "", sec.CompositeFIGI, currentDateString, "figi change 1", fmt.Errorf("no rows affected"))
			} else if test {
				logAction(test, i, sec.Ticker, "", sec.CompositeFIGI, currentDateString, "figi change 1", nil)
			}
		}
		for i, sec := range additions {
			diagnoses := make([]string, 0)
			var maxDate sql.NullTime
			targetTicker := ""
			if sec.CompositeFIGI != "" { //if figi exists
				//err := conn.DB.QueryRow(context.Background(),"SELECT ticker, maxDate FROM securities where figi = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1",sec.CompositeFIGI).Scan(&tickerInDB,&maxDate)
				rows, err := conn.DB.Query(context.Background(), "SELECT ticker, maxDate FROM securities where figi = $1 order by COALESCE(maxDate, '2200-01-01') DESC", sec.CompositeFIGI) //.Scan(&tickerInDB,&maxDate)
				if rows.Next() {
					err = rows.Scan(&targetTicker, &maxDate)
					if err != nil {
						logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "db error 1", err)
						fmt.Printf("v2n92 %v\n", err)
						continue
					}
					if targetTicker == sec.Ticker {
						logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "false delist 1", nil)
						diagnoses = append(diagnoses, "false delist")
					} else {
						prevListing := false
						for rows.Next() {
							var targetTicker string
							var date sql.NullTime
							err = rows.Scan(&targetTicker, &date)
							if err != nil {
								logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "db error 2", err)
								prevListing = true //simply to avoid doing more actions with error case
								break
							}
							if targetTicker == sec.Ticker {
								logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "prev listing hit", nil)
								prevListing = true
								break
							}
						}
						if !prevListing {
							logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "ticker change 1", nil)
							diagnoses = append(diagnoses, "ticker change")
							if dataExists(conn.Polygon, sec.Ticker, maxDate.Time.Format(dateFormat), yesterdayDateString) {
								logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "false delist and ticker change", nil)
								diagnoses = append(diagnoses, "false delist")
							}
						} else {
							logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "skipped dupe listing", nil)
						}
					}
				} else if err == nil { //figi doesnt exist in db
					targetTicker = sec.Ticker
					err := conn.DB.QueryRow(context.Background(), "SELECT maxDate from securities where ticker = $1", sec.Ticker).Scan(&maxDate)
					if err == nil {
						if dataExists(conn.Polygon, sec.Ticker, maxDate.Time.Format(dateFormat), yesterdayDateString) {
							diagnoses = append(diagnoses, "false delist")
							diagnoses = append(diagnoses, "figi change")
							logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "false delist and figi change", nil)
						} else {
							diagnoses = append(diagnoses, "listing")
							logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "listing 1", nil)
						}
					} else if err == pgx.ErrNoRows {
						diagnoses = append(diagnoses, "listing")
						logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "listing 2", nil)
					} else {
						fmt.Printf("n9i0v2 %v\n", err)
						fmt.Println(sec.Ticker)
						logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "db err 3", err)
					}
				} else { //valid error
					fmt.Println(sec.Ticker, " ", sec.CompositeFIGI, " ", currentDateString)
					fmt.Printf("32gerf %v \n", err)
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "db err 4", err)
				}
				rows.Close()
			} else { //deal with tickers
				targetTicker = sec.Ticker
				var figiInDB string
				err := conn.DB.QueryRow(context.Background(), "SELECT figi, maxDate FROM securities where ticker = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1", sec.Ticker).Scan(&figiInDB, &maxDate)
				if err == nil { // ticker exists in db and data exists
					if dataExists(conn.Polygon, sec.Ticker, maxDate.Time.Format(dateFormat), yesterdayDateString) {
						diagnoses = append(diagnoses, "false delist")
						logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "false delist 2", nil)
					} else {
						diagnoses = append(diagnoses, "listing")
						logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "listing 3", nil)
					}
				} else if err == pgx.ErrNoRows {
					diagnoses = append(diagnoses, "listing")
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "listing 4", nil)
				} else {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "db err 5", nil)
				}
			}
			if contains(diagnoses, "false delist") {
				cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE securities set maxDate = NULL where ticker = $1 AND (maxDate is null or maxDate = (SELECT max(maxDate) FROM securities WHERE ticker = $1))", targetTicker)
				if err != nil {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "false delist exec", err)
				} else if cmdTag.RowsAffected() == 0 {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "false delist exec", fmt.Errorf("no rows affected"))
				} else {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "false delist exec", err)
				}
				continue
			}
			if contains(diagnoses, "ticker change") {
				cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE securities SET maxDate = $1 where figi = $2 and maxDate is NULL", currentDateString, sec.CompositeFIGI)
				if err != nil {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "remove prev exec", err)
				} else if cmdTag.RowsAffected() != 1 {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "remove prev exec", fmt.Errorf("%d rows affected", cmdTag.RowsAffected()))
					rows, _ := conn.DB.Query(context.Background(), "SELECT securityId, ticker, figi, mindate, maxdate from securities where figi = $1 or ticker = $2", sec.CompositeFIGI, sec.Ticker)
					for rows.Next() {
						var ticker string
						var secId int
						var figi string
						var minDate sql.NullTime
						var maxDate sql.NullTime
						rows.Scan(&secId, &ticker, &figi, &minDate, &maxDate)
						var minDtStr string
						var maxDtStr string
						if minDate.Valid {
							minDtStr = minDate.Time.Format(dateFormat)
						} else {
							minDtStr = "NULL"
						}
						if maxDate.Valid {
							maxDtStr = maxDate.Time.Format(dateFormat)
						} else {

							maxDtStr = "NULL"
						}
						fmt.Printf("%s %d %s %s %s\n", ticker, secId, figi, minDtStr, maxDtStr)
					}
					rows.Close()
				} else {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "remove prev exec", err)
				}
				_, err = conn.DB.Exec(context.Background(), "INSERT INTO securities (securityId, figi, ticker, minDate) SELECT securityID, figi, $1, $2 from securities where figi = $3", sec.Ticker, currentDateString, sec.CompositeFIGI)
				logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "ticker change exec", err)
			}
			if contains(diagnoses, "figi change") {
				cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE securities set figi = $1 where ticker = $2 and (maxDate is NULL or maxDate = (SELECT max(maxDate) FROM securities where ticker = $2) )", sec.CompositeFIGI, sec.Ticker)
				if err != nil {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "figi change exec", err)
				} else if cmdTag.RowsAffected() == 0 {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "figi change exec", fmt.Errorf("no rows affected"))
				} else {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "figi change exec", err)
				}
			}
			if contains(diagnoses, "listing") {
				_, err = conn.DB.Exec(context.Background(), "INSERT INTO securities (figi, ticker, minDate) values ($1,$2,$3)", sec.CompositeFIGI, sec.Ticker, currentDateString)

				logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "listing exec", err)
			}
		}
		for i, sec := range removals {

			cmdTag, err := conn.DB.Exec(context.Background(), "UPDATE securities SET maxDate = $1 where ticker = $2 and maxDate is NULL", yesterdayDateString, sec.Ticker)
			targetTicker := ""
			if err != nil {
				fmt.Println("91md")
				fmt.Println(sec.Ticker, " ", sec.CompositeFIGI, " ", currentDateString)
				logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "remove 1", err)
			} else if cmdTag.RowsAffected() == 0 { //this whole thing is just for error checking but if rows affected is zero then it should be a removal of a overdue removal after a ticker change
				ok := false
				if sec.CompositeFIGI != "" { //if figi exists
					rows, err := conn.DB.Query(context.Background(), "SELECT ticker, maxDate FROM securities where figi = $1 order by COALESCE(maxDate, '2200-01-01') DESC", sec.CompositeFIGI) //.Scan(&tickerInDB,&maxDate)
					var targetTicker string
					var maxDate sql.NullTime
					if rows.Next() {
						err = rows.Scan(&targetTicker, &maxDate)
						if err != nil {
							fmt.Printf("21j1m %v\n", err)
						} else {
							if targetTicker == sec.Ticker {
								fmt.Printf("23kniv %s %s %s\n", sec.Ticker, sec.CompositeFIGI, currentDateString)
							} else {
								for rows.Next() {
									var ticker string
									var date sql.NullTime
									err = rows.Scan(&ticker, &date)
									if err != nil {
										fmt.Printf("02200iv %v\n", err)
										break
									}
									if ticker == sec.Ticker {
										if date.Valid {
											ok = true
											break
										} else {
											fmt.Printf("23kn1n9div %s %s %s\n", sec.Ticker, sec.CompositeFIGI, currentDateString)
										}
									}
								}
							}
						}
					}
					rows.Close()
				}
				if !ok {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "remove valid skip", err)
				} else {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "remove invalid skip", err)
				}
			}
		}
		yesterdayDateString = currentDateString
		activeYesterday = activeToday
	}

	return nil
}

func updateSecurityDetails(conn *utils.Conn, test bool) error {
	// Query active securities (where maxDate is null)
	fmt.Println("Updating security details")
	rows, err := conn.DB.Query(context.Background(),
		`SELECT securityid, ticker 
		 FROM securities 
		 WHERE maxDate IS NULL AND (logo IS NULL OR icon IS NULL)`)
	if err != nil {
		return fmt.Errorf("failed to query active securities: %v", err)
	}
	defer rows.Close()

	// Create a rate limiter for 10 requests per second
	rateLimiter := time.NewTicker(100 * time.Millisecond) // 10 requests per second
	defer rateLimiter.Stop()

	// Create a worker pool with semaphore to limit concurrent requests
	maxWorkers := 15

	sem := make(chan struct{}, maxWorkers)
	errChan := make(chan error, maxWorkers)
	var wg sync.WaitGroup

	// Helper function to fetch and encode image data
	fetchImage := func(url string, polygonKey string) (string, error) {
		if url == "" {
			return "", nil
		}

		client := &http.Client{}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Add("Authorization", "Bearer "+polygonKey)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to fetch image: %v", err)
		}
		defer resp.Body.Close()

		imageData, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read image data: %v", err)
		}

		return base64.StdEncoding.EncodeToString(imageData), nil
	}

	// Worker function to process each security
	processSecurity := func(securityId int, ticker string) {
		defer wg.Done()
		defer func() { <-sem }() // Release semaphore slot

		<-rateLimiter.C // Wait for rate limiter

		details, err := utils.GetTickerDetails(conn.Polygon, ticker, "now")
		if err != nil {
			fmt.Printf("Failed to get details for %s: %v", ticker, err)
			return
		}

		// Fetch both logo and icon
		logoBase64, _ := fetchImage(details.Branding.LogoURL, conn.PolygonKey)
		iconBase64, _ := fetchImage(details.Branding.IconURL, conn.PolygonKey)
		currentPrice, err := utils.GetMostRecentRegularClose(conn.Polygon, ticker, time.Now())
		if err != nil {
			fmt.Printf("Failed to get current price for %s: %v", ticker, err)
			//return
		}

		// Update the security record with all details
		_, err = conn.DB.Exec(context.Background(),
			`UPDATE securities 
			 SET name = $1,
				 market = $2,
				 locale = $3,
				 primary_exchange = $4,
				 active = $5,
				 market_cap = $6,
				 description = $7,
				 logo = $8,
				 icon = $9,
				 share_class_shares_outstanding = $10,
				 total_shares = CASE 
					 WHEN $6::numeric > 0 AND $12::numeric > 0 THEN CAST(($6::numeric / $12::numeric) AS BIGINT)
					 ELSE NULL 
				 END
			 WHERE securityid = $11`,
			details.Name,
			string(details.Market),
			string(details.Locale),

			details.PrimaryExchange,
			details.Active,
			details.MarketCap,
			details.Description,
			logoBase64,
			iconBase64,
			details.ShareClassSharesOutstanding,
			securityId,
			currentPrice)

		if err != nil {
			if test {
				log.Printf("Failed to update details for %s: %v", ticker, err)
			}
			errChan <- fmt.Errorf("failed to update %s: %v", ticker, err)
			return
		}

		if test {
			log.Printf("Successfully updated details for %s", ticker)
		}
	}

	// Process all securities
	for rows.Next() {
		var securityId int
		var ticker string
		if err := rows.Scan(&securityId, &ticker); err != nil {
			return fmt.Errorf("failed to scan security row: %v", err)
		}

		sem <- struct{}{} // Acquire semaphore slot
		wg.Add(1)
		go processSecurity(securityId, ticker)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors during update: %v", len(errors), errors)
	}
	fmt.Println("Security details updated successfully")

	return nil
}
