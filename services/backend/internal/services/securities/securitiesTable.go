//go:build !test
// +build !test

package securities

import (
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/utils"

	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"

	"database/sql"

	"encoding/base64"

	_ "github.com/lib/pq" // Register postgres driver
	_polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
)

// Security represents a structure for handling Security data.

// logAction logs security-related actions for debugging and auditing purposes.
// Currently unused but kept for debugging and future use.
// nolint:unused
//
//lint:ignore U1000 kept for debugging and future use
func logAction(test bool, loop int, ticker string, targetTicker string, figi string, currentDate string, action string, err error) {
	if test {
		/*if err != nil {
			fmt.Printf("loop %-5d | time %s | ticker %-10s | targetTicker %-12s | figi %-20s | date %-10s | action %-20s | error %v\n", loop, time.Now().Format("2006-01-02 15:04:05"), ticker, targetTicker, figi, currentDate, action, err)
		}*/
		log.Printf("loop %-5d | ticker %-10s | targetTicker %-12s | figi %-20s | date %-10s | action %-35s | error %v\n", loop, ticker, targetTicker, figi, currentDate, action, err)
	}
}

// dataExists checks if market data exists for a ticker in a given date range.
// Currently unused but kept for future data validation features.
// nolint:unused
//
//lint:ignore U1000 kept for future use
func dataExists(client *_polygon.Client, ticker string, fromDate string, toDate string) bool {
	timespan := models.Timespan("day")
	fromMillis, err := utils.MillisFromDatetimeString(fromDate)
	if err != nil {
		return false
	}
	toMillis, err := utils.MillisFromDatetimeString(toDate)
	if err != nil {
		return false
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
}

// contains checks if a string slice contains a specific item.
// Currently unused but kept for future utility use.
// nolint:unused
//
//lint:ignore U1000 kept for future use
func contains(slice []string, item string) bool {
	for _, str := range slice {
		if str == item {
			return true
		}
	}
	return false
}

// UpdateSecurities updates the securities table with new data.
// Currently unused but kept for future automated updates.
// nolint:unused
//
//lint:ignore U1000 kept for future use
func UpdateSecurities(conn *data.Conn, test bool) error {
	var startDate time.Time
	//return nil
	if test {
		shouldClearLog := true // Set this based on your requirements
		flags := os.O_CREATE | os.O_WRONLY
		if shouldClearLog {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_APPEND
		}

		logDir := os.Getenv("LOG_DIR")
		if logDir == "" {
			logDir = "logs"
		}
		// Ensure log directory exists
		if err := os.MkdirAll(logDir, 0750); err != nil {
			log.Fatalf("Failed to create log directory: %v", err)
		}

		// Get absolute path for the log directory
		absLogDir, err := filepath.Abs(logDir)
		if err != nil {
			log.Fatalf("Failed to resolve log directory: %v", err)
		}

		// Hard-code the filename and construct the full path
		const fixedFilename = "securities.log"
		safeLogPath := filepath.Join(absLogDir, fixedFilename)

		// Add a gosec directive to ignore this specific line since we've validated the path
		// #nosec G304 - safeLogPath is constructed from validated parts and cannot be manipulated
		file, err := os.OpenFile(safeLogPath, flags, 0600)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer file.Close()

		log.SetOutput(file)
		query := "TRUNCATE TABLE securities RESTART IDENTITY CASCADE"
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
	yesterdayPolyTickers, err := polygon.AllTickers(conn.Polygon, startDate.AddDate(0, 0, -1).Format(dateFormat))
	if err != nil {
		return fmt.Errorf("1j9v %v", err)
	}
	activeYesterday := toFilteredMap(yesterdayPolyTickers)
	for currentDate := startDate; currentDate.Before(time.Now()); currentDate = currentDate.AddDate(0, 0, 1) {
		currentDateString := currentDate.Format(dateFormat)
		polyTickers, err := polygon.AllTickers(conn.Polygon, currentDateString)
		if err != nil {
			return fmt.Errorf("423n %v", err)
		}
		activeToday := toFilteredMap(polyTickers)
		additions, removals, figiChanges := diff(activeToday, activeYesterday)
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
							if dataExists(conn.Polygon, sec.Ticker, maxDate.Time.Format(dateFormat), currentDateString) {
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
						if dataExists(conn.Polygon, sec.Ticker, maxDate.Time.Format(dateFormat), currentDateString) {
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
						logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "db err 3", err)
					}
				} else { //valid error
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "db err 4", err)
				}
				rows.Close()
			} else { //deal with tickers
				targetTicker = sec.Ticker
				var figiInDB string
				err := conn.DB.QueryRow(context.Background(), "SELECT figi, maxDate FROM securities where ticker = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1", sec.Ticker).Scan(&figiInDB, &maxDate)
				if err == nil { // ticker exists in db and data exists
					if dataExists(conn.Polygon, sec.Ticker, maxDate.Time.Format(dateFormat), currentDateString) {
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
						var secID int
						var figi string
						var minDate sql.NullTime
						var maxDate sql.NullTime
						if err := rows.Scan(&secID, &ticker, &figi, &minDate, &maxDate); err != nil {
							log.Printf("Error scanning row: %v", err)
							continue
						}
						/*						var minDtStr string
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
												}*/
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

			cmdTag, _ := conn.DB.Exec(context.Background(), "UPDATE securities SET maxDate = $1 where ticker = $2 and maxDate is NULL", currentDateString, sec.Ticker)
			// Log the number of rows affected if needed
			if test && cmdTag.RowsAffected() > 0 {
				log.Printf("Updated %d rows for ticker %s", cmdTag.RowsAffected(), sec.Ticker)
			}
			targetTicker := ""
			if cmdTag.RowsAffected() == 0 { //this whole thing is just for error checking but if rows affected is zero then it should be a removal of a overdue removal after a ticker change
				ok := false
				if sec.CompositeFIGI != "" { //if figi exists
					rows, err := conn.DB.Query(context.Background(), "SELECT ticker, maxDate FROM securities where figi = $1 order by COALESCE(maxDate, '2200-01-01') DESC", sec.CompositeFIGI) //.Scan(&tickerInDB,&maxDate)
					if err != nil {
						logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "query error", err)
						continue
					}
					var targetTicker string
					var maxDate sql.NullTime
					if rows.Next() {
						err = rows.Scan(&targetTicker, &maxDate)
						if err != nil {
							logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "query error", err)
							continue
						}
						if targetTicker != sec.Ticker {
							for rows.Next() {
								var ticker string
								var date sql.NullTime
								err = rows.Scan(&ticker, &date)
								if err != nil {
									break
								}
								if ticker == sec.Ticker {
									if date.Valid {
										ok = true
										break
									}
								}
							}
						}
					}
					rows.Close()
				}
				if !ok {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "remove valid skip", nil)
				} else {
					logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "remove invalid skip", nil)
				}
			}
		}
		activeYesterday = activeToday
	}

	return nil
}

func UpdateSecurityDetails(conn *data.Conn, test bool) error {
	// Query active securities (where maxDate is null)

	// First, count how many securities need updating
	var count int
	err := conn.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) 
		 FROM securities 
		 WHERE maxDate IS NULL AND (logo IS NULL OR icon IS NULL)`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count securities needing updates: %v", err)
	}

	// If no securities need updating, return success
	if count == 0 {
		return nil
	}

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
	maxWorkers := 5

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

		// Check if the response status is OK
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to fetch image, status code: %d", resp.StatusCode)
		}

		imageData, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read image data: %v", err)
		}

		// If no image data was returned, return empty string
		if len(imageData) == 0 {
			return "", fmt.Errorf("empty image data received")
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			// Try to detect content type from image data
			contentType = http.DetectContentType(imageData)

			// If still empty, default to a safe type based on URL extension
			if contentType == "" || contentType == "application/octet-stream" {
				if strings.HasSuffix(strings.ToLower(url), ".svg") {
					contentType = "image/svg+xml"
				} else if strings.HasSuffix(strings.ToLower(url), ".png") {
					contentType = "image/png"
				} else {
					contentType = "image/jpeg"
				}
			}
		}

		// Ensure the content type doesn't already contain a data URL prefix
		if strings.HasPrefix(contentType, "data:") {
			return "", fmt.Errorf("invalid content type: %s", contentType)
		}

		base64Data := base64.StdEncoding.EncodeToString(imageData)

		// Check if base64Data already contains a data URL prefix to prevent duplication
		if strings.HasPrefix(base64Data, "data:") {
			return base64Data, nil
		}

		return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
	}

	// Worker function to process each security
	processSecurity := func(securityID int, ticker string) {
		defer wg.Done()
		defer func() { <-sem }() // Release semaphore slot

		<-rateLimiter.C // Wait for rate limiter

		details, err := polygon.GetTickerDetails(conn.Polygon, ticker, "now")
		if err != nil {
			//log.Printf("Failed to get details for %s: %v", ticker, err)
			return
		}

		// Fetch both logo and icon
		logoBase64, err := fetchImage(details.Branding.LogoURL, conn.PolygonKey)
		if err != nil {
			log.Printf("Failed to fetch logo for %s: %v", ticker, err)
		}
		iconBase64, err := fetchImage(details.Branding.IconURL, conn.PolygonKey)
		if err != nil {
			log.Printf("Failed to fetch icon for %s: %v", ticker, err)
		}
		currentPrice, err := polygon.GetMostRecentRegularClose(conn.Polygon, ticker, time.Now())
		if err != nil {
			//log.Printf("Failed to get current price for %s: %v", ticker, err)
			return
		}

		// Update the security record with all details
		_, err = conn.DB.Exec(context.Background(),
			`UPDATE securities 
			 SET name = NULLIF($1, ''),
				 market = NULLIF($2, ''),
				 locale = NULLIF($3, ''),
				 primary_exchange = NULLIF($4, ''),
				 active = $5,
				 market_cap = NULLIF($6::BIGINT, 0),
				 description = NULLIF($7, ''),
				 logo = NULLIF($8, ''),
				 icon = NULLIF($9, ''),
				 share_class_shares_outstanding = NULLIF($10::BIGINT, 0),
				 total_shares = CASE 
					 WHEN NULLIF($6::BIGINT, 0) > 0 AND NULLIF($12, 0) > 0 
					 THEN CAST(($6::BIGINT / $12) AS BIGINT)
					 ELSE NULL 
				 END
			 WHERE securityid = $11`,
			utils.NullString(truncateString(details.Name, 500)),
			utils.NullString(truncateString(string(details.Market), 50)),
			utils.NullString(truncateString(string(details.Locale), 50)),
			utils.NullString(truncateString(details.PrimaryExchange, 50)),
			details.Active,
			utils.NullInt64(int64(details.MarketCap)),
			utils.NullString(details.Description),
			utils.NullString(logoBase64),
			utils.NullString(iconBase64),
			utils.NullInt64(details.ShareClassSharesOutstanding),
			securityID,
			currentPrice)

		if err != nil {
			if test {
				log.Printf("Failed to update details for %s: Column error - market_cap=%v, share_class_shares_outstanding=%v - Error: %v",
					ticker,
					details.MarketCap,
					details.ShareClassSharesOutstanding,
					err)
			}
			errChan <- fmt.Errorf("failed to update %s: Column error - market_cap=%v, share_class_shares_outstanding=%v - Error: %v",
				ticker,
				details.MarketCap,
				details.ShareClassSharesOutstanding,
				err)
			return
		}

		// Successfully updated details - no action needed in non-test mode
		// Uncomment the log line below if you want to enable logging in test mode
		// if test {
		//     log.Printf("Successfully updated details for %s", ticker)
		// }
	}

	// Process all securities
	for rows.Next() {
		var securityID int
		var ticker string
		if err := rows.Scan(&securityID, &ticker); err != nil {
			return fmt.Errorf("failed to scan security row: %v", err)
		}

		sem <- struct{}{} // Acquire semaphore slot
		wg.Add(1)
		go processSecurity(securityID, ticker)
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

	return nil
}

type CompositeFIGIUpdate struct {
	Ticker           string
	NewCompositeFIGI string
}
type ShareClassFIGIUpdate struct {
	Ticker            string
	NewShareClassFIGI string
}

/*
func UpdateSecuritiesV2(conn *data.Conn, test bool) error {
	var startDate time.Time

	var startDateNull sql.NullTime
	err := conn.DB.QueryRow(context.Background(), "SELECT MAX(maxDate) FROM securities").Scan(&startDateNull)
	if err != nil {
		return fmt.Errorf("failed to get max date: %v", err)
	}
	if startDateNull.Valid {
		startDate = startDateNull.Time
	} else {
		startDate = time.Date(2003, 9, 10, 0, 0, 0, 0, time.UTC)
	}
	dateFormat := "2006-01-02"
	yesterdayPolygonTickers, err := polygon.AllTickers(conn.Polygon, startDate.AddDate(0, 0, -1).Format(dateFormat))
	if err != nil {
		return fmt.Errorf("[UpdateSecuritiesV2] failed to get yesterday's tickers: %v", err)
	}
	activeYesterday := toFilteredMap(yesterdayPolygonTickers)
	for currentDate := startDate; currentDate.Before(time.Now()); currentDate = currentDate.AddDate(0, 0, 1) {
		if currentDate.Weekday() == time.Saturday || currentDate.Weekday() == time.Sunday {
			continue
		}
		currentDateString := currentDate.Format(dateFormat)
		currentDatePolygonTickers, err := polygon.AllTickers(conn.Polygon, currentDateString)
		if err != nil {
			return fmt.Errorf("[UpdateSecuritiesV2] failed to get %s tickers: %v", currentDateString, err)
		}
		activeToday := toFilteredMap(currentDatePolygonTickers)
		additions, removals, compositeFigiChanges, shareClassFigiChanges := diff(activeToday, activeYesterday)
		compositeFigiUpdates := []CompositeFIGIUpdate{}
		for _, sec := range compositeFigiChanges {
			compositeFigiUpdates = append(compositeFigiUpdates, CompositeFIGIUpdate{
				Ticker:           sec.Ticker,
				NewCompositeFIGI: sec.CompositeFIGI,
			})
		}
		shareClassFigiUpdates := []ShareClassFIGIUpdate{}
		for _, sec := range shareClassFigiChanges {
			shareClassFigiUpdates = append(shareClassFigiUpdates, ShareClassFIGIUpdate{
				Ticker:            sec.Ticker,
				NewShareClassFIGI: sec.ShareClassFIGI,
			})
		}
		for _, sec := range additions {
			diagnoses := make([]string, 0)
			var maxDate sql.NullTime
			targetTicker := ""
			if sec.CompositeFIGI != "" { // If the Polygon Security has a composite FIGI, check if it exists in the Database
				rows, err := conn.DB.Query(context.Background(), "SELECT ticer, maxDate FROM securities where composite_figi=$1 order by COALESCE(maxDate, '2200-01-01') DESC", sec.CompositeFIGI)
				if rows.Next() {
					err = rows.Scan(&targetTicker, &maxDate)
					if err != nil {
						fmt.Println("[UpdateSecuritiesV2][ttk3kr] Error scanning rows:", err)
						continue
					}
					if targetTicker == sec.Ticker {
						diagnoses = append(diagnoses, "false delist")
					} else {
						prevListing := false
						for rows.Next() {
							var targetTicker string
							var date sql.NullTime
							err = rows.Scan(&targetTicker, &date)
							if err != nil {
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
							diagnoses = append(diagnoses, "ticker change")
							if dataExists(conn.Polygon, sec.Ticker, maxDate.Time.Format(dateFormat), currentDateString) {
								diagnoses = append(diagnoses, "false delist")
							}
						} // Otherwise, skip duplicate listing
					}
				} else if err == nil { // Composite FIGI does not exist in DB
					targetTicker = sec.Ticker
					err := conn.DB.QueryRow(context.Background(), "SELECT maxDate from securities where ticker = $1", sec.Ticker).Scan(&maxDate)
					if err == nil {
						if dataExists(conn.Polygon, sec.Ticker, maxDate.Time.Format(dateFormat), currentDateString) {
							diagnoses = append(diagnoses, "false delist")
							diagnoses = append(diagnoses, "figi change")
						} else {
							diagnoses = append(diagnoses, "stock listing")
						}
					} else if err == pgx.ErrNoRows {
						diagnoses = append(diagnoses, "stock listing")
					} else {
						fmt.Println("[UpdateSecuritiesV2][0f9vk3kr] Error querying maxDate:", err)
					}
				} else { // This is some DB error then since rows had an issue

				}
			} else if sec.ShareClassFIGI != "" { // If the stock does not have a Composite FIGI, check if it has a Share Class FIGI
				rows, err := conn.DB.Query(context.Background(), "SELECT ticker, maxDate FROM securities where share_class_figi=$1 order by COALESCE(maxDate, '2200-01-01') DESC", sec.ShareClassFIGI)
				if rows.Next() {
					err = rows.Scan(&targetTicker, &maxDate)
					if err != nil {
						fmt.Println("[UpdateSecuritiesV2][vm9vkopo] Error scanning rows:", err)
					}
				}
			} else { // No Share class or composite FIGI, so it is a removal
				targetTicker = sec.Ticker
				err := conn.DB.QueryRow(context.Background(), "SELECT figi, maxDate FROM securities where ticker = $1 order by COALESCE(maxDate, '2200-01-01') DESC LIMIT 1", sec.Ticker).Scan(&figiInDB, &maxDate)
				if err == nil { // ticker exists in db and data exists
					if dataExists(conn.Polygon, sec.Ticker, maxDate.Time.Format(dateFormat), currentDateString) {
						diagnoses = append(diagnoses, "false delist")
					} else {
						diagnoses = append(diagnoses, "stock listing")
					}
				} else if err == pgx.ErrNoRows {
					diagnoses = append(diagnoses, "stock listing")
				}
			}
			if contains(diagnoses, "false delist") {
				_, err := conn.DB.Exec(context.Background(), "UPDATE securities set maxDate = NULL where ticker = $1 AND (maxDate is null or maxDate = (SELECT max(maxDate) FROM securities WHERE ticker = $1))", targetTicker)
				if err != nil {
					fmt.Println("[UpdateSecuritiesV2][0f9vk3kr] Error updating maxDate:", err)
				}
			}
			if contains(diagnoses, "ticker change") {

			}
		}
		for i, sec := range removals {

			cmdTag, _ := conn.DB.Exec(context.Background(), "UPDATE securities SET maxDate = $1 where ticker = $2 and maxDate is NULL", currentDateString, sec.Ticker)
			// Log the number of rows affected if needed
			if test && cmdTag.RowsAffected() > 0 {
				log.Printf("Updated %d rows for ticker %s", cmdTag.RowsAffected(), sec.Ticker)
			}
			targetTicker := ""
			if cmdTag.RowsAffected() == 0 { //this whole thing is just for error checking but if rows affected is zero then it should be a removal of a overdue removal after a ticker change
				if sec.CompositeFIGI != "" { //if figi exists
					rows, err := conn.DB.Query(context.Background(), "SELECT ticker, maxDate FROM securities where figi = $1 order by COALESCE(maxDate, '2200-01-01') DESC", sec.CompositeFIGI) //.Scan(&tickerInDB,&maxDate)
					if err != nil {
						logAction(test, i, sec.Ticker, targetTicker, sec.CompositeFIGI, currentDateString, "query error", err)
						continue
					}
					var targetTicker string
					var maxDate sql.NullTime
					if rows.Next() {
						err = rows.Scan(&targetTicker, &maxDate)
						if err != nil {
							continue
						}
						if targetTicker != sec.Ticker {
							for rows.Next() {
								var ticker string
								var date sql.NullTime
								err = rows.Scan(&ticker, &date)
								if err != nil {
									break
								}
								if ticker == sec.Ticker {
									if date.Valid {
										ok = true
										break
									}
								}
							}
						}
					}
					rows.Close()
				}
			}
		}
		activeYesterday = activeToday
	}

}
*/

// toFilteredMap converts a slice of tickers to a filtered map.
// Currently unused but kept for future filtering features.
// nolint:unused
//
//lint:ignore U1000 kept for future use
func toFilteredMap(tickers []models.Ticker) map[string]models.Ticker {
	tickerMap := make(map[string]models.Ticker)
	for _, sec := range tickers {
		tickerMap[sec.Ticker] = sec
	}
	return tickerMap
}

// diff compares two sets of tickers and returns the differences.
// Currently unused but kept for future reconciliation features.
// nolint:unused
//
//lint:ignore U1000 kept for future use
func diff(firstSet, secondSet map[string]models.Ticker) ([]models.Ticker, []models.Ticker, []models.Ticker) {
	additions := []models.Ticker{}
	removals := []models.Ticker{}
	figiChanges := []models.Ticker{}

	// Trackers to ensure no duplicates
	usedTickers := make(map[string]struct{})

	// Process additions and figi changes
	for ticker, sec := range firstSet {
		if yesterdaySec, found := secondSet[ticker]; !found {
			if _, exists := usedTickers[ticker]; !exists {
				additions = append(additions, sec)
				usedTickers[ticker] = struct{}{}
			}
		} else {
			if yesterdaySec.CompositeFIGI != sec.CompositeFIGI {
				if _, exists := usedTickers[ticker]; !exists {
					figiChanges = append(figiChanges, sec)
					usedTickers[ticker] = struct{}{}
				}
			}
		}
	}

	// Process removals
	for ticker, sec := range secondSet {
		if _, found := firstSet[ticker]; !found {
			if _, exists := usedTickers[ticker]; !exists {
				removals = append(removals, sec)
				usedTickers[ticker] = struct{}{}
			}
		}
	}

	return additions, removals, figiChanges
}

// Helper function to truncate string if it exceeds maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
