package securities

import (
	"backend/internal/data/polygon"
	"backend/internal/data/utils"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/data"
	"backend/internal/data/postgres"

	//lint:ignore U1000 external package
	"github.com/jackc/pgx/v4"
	_ "github.com/lib/pq" // Register postgres driver
	_polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
)

// dataExists checks if market data exists for a ticker in a given date range.
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

// SecurityDetail represents a structure for handling SecurityDetail data.

func SimpleUpdateSecurities(conn *data.Conn) error {
	ctx := context.Background()

	// Find the latest maxDate from the securities table
	var latestMaxDate *time.Time
	err := conn.DB.QueryRow(ctx, `
		SELECT MAX(maxDate) 
		FROM securities 
		WHERE maxDate IS NOT NULL
	`).Scan(&latestMaxDate)
	if err != nil {
		return fmt.Errorf("failed to get latest maxDate: %w", err)
	}

	// Determine the start date and end date
	var startDate time.Time
	if latestMaxDate != nil {
		// Start from the day after the latest maxDate
		startDate = latestMaxDate.AddDate(0, 0, 1)
	} else {
		// If no maxDate exists, start from 30 days ago (or adjust as needed)
		startDate = time.Now().AddDate(0, 0, -30)
	}

	endDate := time.Now()
	// Loop through each date from startDate to endDate
	for currentDate := startDate; !currentDate.After(endDate); currentDate = currentDate.AddDate(0, 0, 1) {
		if currentDate.Weekday() == time.Saturday || currentDate.Weekday() == time.Sunday {
			continue
		}
		targetDateStr := currentDate.Format("2006-01-02")

		// 1) Fetch the tickers from Polygon for this date
		poly, err := polygon.AllTickers(conn.Polygon, targetDateStr)
		if err != nil {
			//fmt.Printf("Warning: failed to fetch polygon tickers for %s: %v\n", targetDateStr, err)
			continue // Skip this date and continue with the next
		}

		// collect just the symbols
		tickers := make([]string, len(poly))
		for i, s := range poly {
			tickers[i] = s.Ticker
		}

		if len(tickers) == 0 {
			continue
		}

		// 2) Mark as DELISTED any ticker NOT in this date's list
		if _, err := conn.DB.Exec(ctx, `
			UPDATE securities
			   SET maxDate = $1
			 WHERE maxDate IS NULL
			   AND ticker NOT IN (`+placeholdersOffset(len(tickers), 1)+`)
			   AND NOT EXISTS (
				   SELECT 1 FROM securities s2 
				   WHERE s2.ticker = securities.ticker 
				   AND s2.maxDate = $1
			   )
		`, append([]interface{}{currentDate}, stringArgs(tickers)...)...); err != nil {
			return fmt.Errorf("delist tickers for %s: %w", targetDateStr, err)
		}

		// 3) REACTIVATE any ticker IN this date's list
		if _, err := conn.DB.Exec(ctx, `
			UPDATE securities
			   SET maxDate = NULL
			 WHERE maxDate IS NOT NULL
			   AND ticker IN (`+placeholders(len(tickers))+`)
		`, stringArgs(tickers)...); err != nil {
			return fmt.Errorf("reactivate tickers for %s: %w", targetDateStr, err)
		}
		ipos, err := polygon.GetPolygonIPOs(conn.Polygon, targetDateStr)
		if err != nil {
			fmt.Printf("Warning: failed to fetch polygon IPOs for %s: %v\n", targetDateStr, err)
		} else {
			// 4) INSERT new IPO tickers with mindate = current date
			for _, ipo := range ipos.Tickers {
				if _, err := conn.DB.Exec(ctx, `
					INSERT INTO securities (ticker, mindate, figi) 
					VALUES ($1, $2, $3)
					ON CONFLICT (ticker, mindate) DO NOTHING
				`, ipo, currentDate, ""); err != nil {
					fmt.Printf("Warning: failed to insert IPO ticker %s for %s: %v\n", ipo, targetDateStr, err)
				}
			}
		}
		//fmt.Printf("Completed processing for %s (%d tickers)\n", targetDateStr, len(tickers))
	}

	return nil
}

// placeholders(n) returns "$1,$2,…,$n"
func placeholders(n int) string {
	ps := make([]string, n)
	for i := range ps {
		ps[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(ps, ",")
}

// placeholdersOffset(n, offset) returns "$(offset+1),$(offset+2),…,$(offset+n)"
func placeholdersOffset(n int, offset int) string {
	ps := make([]string, n)
	for i := range ps {
		ps[i] = fmt.Sprintf("$%d", i+1+offset)
	}
	return strings.Join(ps, ",")
}

// stringArgs converts []string to []interface{} for Exec()
func stringArgs(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// UpdateSecurityCik fetches the latest CIK (Central Index Key) data from the SEC API
// and updates the securities table with CIK values for active securities.
func UpdateSecurityCik(conn *data.Conn) error {
	// Create a client and request with appropriate headers
	////fmt.Println("🟢 fetching sec company tickers")
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.sec.gov/files/company_tickers.json", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers to make the request look like a browser
	req.Header.Set("User-Agent", "Atlantis Equities admin@atlantis.trading")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch SEC company tickers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SEC API returned non-200 status code: %d", resp.StatusCode)
	}

	// Parse the JSON response
	var secData map[string]struct {
		CikStr int64  `json:"cik_str"`
		Ticker string `json:"ticker"`
		Title  string `json:"title"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&secData); err != nil {
		return fmt.Errorf("failed to decode SEC API response: %w", err)
	}

	// Process each ticker and update the database
	for _, company := range secData {
		_, err := conn.DB.Exec(context.Background(),
			`UPDATE securities 
			 SET cik = $1 
			 WHERE ticker = $2 
			 AND maxDate IS NULL 
			 AND (cik IS NULL)`,
			company.CikStr, company.Ticker,
		)
		if err != nil {
			return fmt.Errorf("failed to update CIK for ticker %s: %w", company.Ticker, err)
		}
	}

	////fmt.Println("🟢 Securities CIK values updated successfully.")
	return nil
}

// Alternative more efficient approach for large datasets
func SimpleUpdateSecuritiesV2(conn *data.Conn) error {
	ctx := context.Background()

	// 1. Pre-load all existing tickers with maxdate IS NULL into a set
	existingTickers := make(map[string]struct{})
	rows, err := conn.DB.Query(ctx, "SELECT ticker FROM securities WHERE maxdate IS NULL")
	if err != nil {
		return fmt.Errorf("failed to pre-load existing tickers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ticker string
		if err := rows.Scan(&ticker); err != nil {
			return fmt.Errorf("failed to scan existing ticker: %w", err)
		}
		existingTickers[ticker] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating existing tickers: %w", err)
	}
	//fmt.Println("🟢 Pre-loaded existing tickers")
	processedTickers := make(map[string]struct{})

	ipos, err := polygon.GetPolygonIPOs(conn.Polygon, time.Now().Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("failed to fetch polygon IPOs: %w", err)
	}

	for _, ipo := range ipos.Tickers {

		// Parse the listing date string into time.Time
		listingDate, err := time.Parse("2006-01-02", ipos.ListingDate)
		if err != nil {
			return fmt.Errorf("failed to parse listing date %s: %w", ipos.ListingDate, err)
		}

		// Insert new IPO security with listing date as minDate and NULL maxDate
		_, err = conn.DB.Exec(context.Background(), `
			INSERT INTO securities (ticker, minDate, maxDate, active, figi) 
			VALUES ($1, $2, NULL, true, $3)
			ON CONFLICT (ticker, minDate) DO NOTHING`,
			ipo,
			listingDate,
			"", // Empty string for figi since we don't have it yet
		)
		if err != nil {
			return fmt.Errorf("failed to insert IPO ticker %s: %w", ipo, err)
		}
		processedTickers[ipo] = struct{}{}
	}
	// 2. Fetch active stocks via daily data
	res, err := polygon.GetAllStocksDailyOHLCV(ctx, conn.Polygon, time.Now().Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("failed to fetch polygon daily OHLCV: %w", err)
	}
	var processingDate string
	results := res.Results

	for _, result := range results {
		ticker := result.Ticker
		if processingDate == "" {
			processingDate = time.Time(result.Timestamp).Format("2006-01-02")
		}
		// Active ticker doesn't exist in our database, process it
		if _, exists := existingTickers[ticker]; !exists {

			// First Check if false delist
			// Check for most recent historical record of this ticker
			var mostRecentMaxDate *time.Time
			var securityID int
			err := conn.DB.QueryRow(ctx, `
				SELECT securityid, maxDate 
				FROM securities 
				WHERE ticker = $1 AND maxDate IS NOT NULL 
				ORDER BY maxDate DESC 
				LIMIT 1`,
				ticker).Scan(&securityID, &mostRecentMaxDate)
			if err != nil && err != pgx.ErrNoRows {
				return fmt.Errorf("failed to check historical record for ticker %s: %w", ticker, err)
			}
			// found record in db
			if mostRecentMaxDate != nil {
				stringMostRecentMaxDate := mostRecentMaxDate.Format("2006-01-02")

				// First Check if this is a false delist

				if dataExists(conn.Polygon, ticker, stringMostRecentMaxDate, processingDate) {
					// false delist
					// update maxdate to null
					_, err := conn.DB.Exec(ctx, `
						UPDATE securities 
						SET maxDate = NULL 
						WHERE securityid = $1 AND ticker = $2`,
						securityID, ticker)
					if err != nil {
						return fmt.Errorf("failed to update maxdate for ticker %s: %w", ticker, err)
					}
					processedTickers[ticker] = struct{}{}
					continue
				}

				// Second Check if this is a ticker change
				res, err := postgres.GetTickerEventsCustom(conn.Polygon, ticker, conn.PolygonKey)
				if err != nil {
					return fmt.Errorf("failed to get ticker events for ticker %s: %w", ticker, err)
				}

				// Check for splits around the processing date
				isSplitDetected := false
				events := res[0].Events
				for _, event := range events {
					if time.Time(event.Date).Format("2006-01-02") == processingDate {
						// First check if this is just a split
						splits, err := postgres.GetStockSplits(conn.Polygon, ticker)
						if err != nil {
							return fmt.Errorf("failed to get stock splits for ticker %s: %w", ticker, err)
						}

						// Parse processing date to time.Time for comparison
						processDatetime, err := time.Parse("2006-01-02", processingDate)
						if err != nil {
							return fmt.Errorf("failed to parse processing date %s: %w", processingDate, err)
						}

						for _, split := range splits {
							// Check if split date is within ±1 day of processing date
							splitDate := time.Time(split.ExecutionDate)
							daysDiff := splitDate.Sub(processDatetime).Hours() / 24

							if daysDiff >= -1.0 && daysDiff <= 1.0 {
								// Split is within ±1 day of processing date, then this is just a split
								_, err := conn.DB.Exec(ctx, `
									UPDATE securities 
									SET maxDate = NULL 
									WHERE securityid = $1 AND ticker = $2 AND NOT EXISTS (SELECT 1 FROM securities WHERE ticker = $2 AND maxDate = NULL)`,
									securityID, ticker)
								if err != nil {
									return fmt.Errorf("failed to update maxdate for ticker %s: %w", ticker, err)
								}
								processedTickers[ticker] = struct{}{}
								isSplitDetected = true
								break
							}
						}
						if isSplitDetected {
							break
						}
					}
				}

				if isSplitDetected {
					continue
				}
				// Then this is a ticker change and we need to link the new ticker to the old one
				//log.Printf("Ticker change detected for %s", ticker)
				getTickerDetailsResponse, err := polygon.GetTickerDetails(conn.Polygon, ticker, processingDate)
				if err != nil {
					return fmt.Errorf("failed to get ticker details for ticker %s: %w", ticker, err)
				}
				cik, err := strconv.ParseInt(getTickerDetailsResponse.CIK, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to convert CIK to int: %w", err)
				}
				figi := getTickerDetailsResponse.CompositeFIGI

				// Parse processing date and subtract 1 day
				processDatetime, err := time.Parse("2006-01-02", processingDate)
				if err != nil {
					return fmt.Errorf("failed to parse processing date %s: %w", processingDate, err)
				}
				previousDay := processDatetime.AddDate(0, 0, -1).Format("2006-01-02")

				oldTicker, err := postgres.GetTickerFromCIK(conn.Polygon, int(cik), previousDay)
				if err != nil {
					return fmt.Errorf("failed to get old ticker for ticker %s: %w", ticker, err)
				}
				//log.Printf("Ticker change: %s -> %s", oldTicker, ticker)
				_, err = conn.DB.Exec(ctx, `UPDATE securities SET maxDate = $1 WHERE ticker=$2 AND securityId=$3 AND maxDate IS NULL AND NOT EXISTS (SELECT 1 FROM securities WHERE ticker = $2 AND maxDate = $1)`, processingDate, oldTicker, securityID)
				if err != nil {
					return fmt.Errorf("failed to update maxdate for ticker %s: %w", ticker, err)
				}
				// Use safe insertion with comprehensive validation
				err = safeInsertSecurityTickerChange(ctx, conn, ticker, figi, processingDate, nil, true, true, "ticker change")
				if err != nil {
					return fmt.Errorf("failed to insert new ticker %s: %w", ticker, err)
				}
				processedTickers[ticker] = struct{}{}
				continue
			}
			// If no record found, then this is a new ticker for our DB
			//log.Printf("New ticker detected: %s", ticker)
			// Figure out the start date of the ticker via 'ticker change' api lol
			res, err := postgres.GetTickerEventsCustom(conn.Polygon, ticker, conn.PolygonKey)
			if err != nil {
				return fmt.Errorf("failed to get ticker events for ticker %s: %w", ticker, err)
			}
			// Check if we have any events before accessing them
			if len(res) == 0 {
				//log.Printf("No ticker events found for %s, skipping", ticker)
				continue
			}

			events := res[0].Events
			initialDate := time.Time(events[len(events)-1].Date).Format("2006-01-02")
			//fmt.Printf("🟢 Initial date: %s\n", initialDate)
			getTickerDetailsResponse, err := polygon.GetTickerDetails(conn.Polygon, ticker, processingDate)
			var figi string
			if err != nil {
				figi = ""
			} else {
				figi = getTickerDetailsResponse.CompositeFIGI
			}
			// Use safe insertion with comprehensive validation
			err = safeInsertSecurityV2(ctx, conn, ticker, initialDate, nil, true, figi, true, "new ticker listing")
			if err != nil {
				return fmt.Errorf("failed to insert new ticker %s: %w", ticker, err)
			}
			processedTickers[ticker] = struct{}{}
			continue
		}

		// Mark ticker as processed
		processedTickers[ticker] = struct{}{}
	}

	// Continue with the polygon.ListTickers part...
	iter, err := polygon.ListTickers(conn.Polygon, "", "", "gte", 1000, true)
	if err != nil {
		return fmt.Errorf("failed to fetch polygon tickers: %w", err)
	}
	for iter.Next() {
		ticker := iter.Item().Ticker
		if _, exists := processedTickers[ticker]; !exists {
			if _, exists := existingTickers[ticker]; !exists {
				// First Check if false delist
				// Check for most recent historical record of this ticker
				var mostRecentMaxDate *time.Time
				var securityID int
				err := conn.DB.QueryRow(ctx, `
					SELECT securityid, maxDate 
					FROM securities 
					WHERE ticker = $1 AND maxDate IS NOT NULL 
					ORDER BY maxDate DESC 
					LIMIT 1`,
					ticker).Scan(&securityID, &mostRecentMaxDate)
				if err != nil && err != pgx.ErrNoRows {
					return fmt.Errorf("failed to check historical record for ticker %s: %w", ticker, err)
				}
				// found record in db
				if mostRecentMaxDate != nil {
					stringMostRecentMaxDate := mostRecentMaxDate.Format("2006-01-02")

					// First Check if this is a false delist

					if dataExists(conn.Polygon, ticker, stringMostRecentMaxDate, processingDate) {
						// false delist
						// update maxdate to null
						_, err := conn.DB.Exec(ctx, `
							UPDATE securities 
							SET maxDate = NULL 
							WHERE securityid = $1 AND ticker = $2 AND NOT EXISTS (SELECT 1 FROM securities WHERE ticker = $2 AND maxDate = NULL)`,
							securityID, ticker)
						if err != nil {
							return fmt.Errorf("failed to update maxdate for ticker %s: %w", ticker, err)
						}
						processedTickers[ticker] = struct{}{}
						continue
					}
				}
				// If no record found, then this is a new ticker for our DB
				// Figure out the start date of the ticker via 'ticker change' api lol
				res, err := postgres.GetTickerEventsCustom(conn.Polygon, ticker, conn.PolygonKey)
				if err != nil {
					return fmt.Errorf("failed to get ticker events for ticker %s: %w", ticker, err)
				}

				// Check if we have any events before accessing them
				if len(res) == 0 {
					continue
				}

				events := res[0].Events
				initialDate := time.Time(events[len(events)-1].Date).Format("2006-01-02")
				var figi string
				getTickerDetailsResponse, err := polygon.GetTickerDetails(conn.Polygon, ticker, processingDate)
				if err != nil {
					figi = ""
				} else {
					figi = getTickerDetailsResponse.CompositeFIGI
				}
				// Use safe insertion with comprehensive validation
				err = safeInsertSecurityV2(ctx, conn, ticker, initialDate, nil, true, figi, true, "polygon ticker listing")
				if err != nil {
					return fmt.Errorf("failed to insert new ticker %s: %w", ticker, err)
				}
				processedTickers[ticker] = struct{}{}
				continue
			}
		}
	}

	// At the end, find any active tickers that weren't processed today and mark them as delisted
	if processingDate != "" {
		// Get all currently active tickers that weren't processed
		rows, err := conn.DB.Query(ctx, `
			SELECT ticker, securityid 
			FROM securities 
			WHERE maxDate IS NULL`)
		if err != nil {
			return fmt.Errorf("failed to query active tickers for delisting: %w", err)
		}
		defer rows.Close()

		var tickersToDelistt []struct {
			ticker     string
			securityID int
		}

		for rows.Next() {
			var ticker string
			var securityID int
			if err := rows.Scan(&ticker, &securityID); err != nil {
				return fmt.Errorf("failed to scan ticker for delisting: %w", err)
			}

			// If this ticker wasn't processed today, mark it for delisting
			if _, processed := processedTickers[ticker]; !processed {
				tickersToDelistt = append(tickersToDelistt, struct {
					ticker     string
					securityID int
				}{ticker: ticker, securityID: securityID})
			}
		}

		// Delist the unprocessed tickers
		for _, item := range tickersToDelistt {
			_, err := conn.DB.Exec(ctx, `
				UPDATE securities 
				SET maxDate = $1 
				WHERE securityid = $2 AND ticker = $3 AND maxDate IS NULL 
				AND NOT EXISTS (SELECT 1 FROM securities WHERE ticker = $3 AND maxDate = $1)`,
				processingDate, item.securityID, item.ticker)
			if err != nil {
				return fmt.Errorf("failed to delist ticker %s: %w", item.ticker, err)
			}
		}
	}

	return nil
}

// validateSecurityInsertionV2 checks if a security insertion would violate database constraints
func validateSecurityInsertionV2(ctx context.Context, conn *data.Conn, securityID *int, ticker string, minDate string, _ string, debug bool) error {
	// Check for (securityid, minDate) constraint violation
	if securityID != nil {
		var count int
		err := conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM securities WHERE securityid = $1 AND minDate = $2", *securityID, minDate).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check securityid+minDate constraint: %v", err)
		}
		if count > 0 {
			if debug {
				fmt.Printf("VALIDATION FAILED: securityid=%d with minDate=%s already exists\n", *securityID, minDate)
			}
			return fmt.Errorf("constraint violation: securityid=%d with minDate=%s already exists", *securityID, minDate)
		}
	}

	// Check for (ticker, minDate) constraint violation
	var count int
	err := conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM securities WHERE ticker = $1 AND minDate = $2", ticker, minDate).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check ticker+minDate constraint: %v", err)
	}
	if count > 0 {
		if debug {
			fmt.Printf("VALIDATION FAILED: ticker=%s with minDate=%s already exists\n", ticker, minDate)
		}
		return fmt.Errorf("constraint violation: ticker=%s with minDate=%s already exists", ticker, minDate)
	}

	// Check for overlapping records that could cause issues with auto-incrementing securityid
	if securityID == nil {
		var nextID int
		var existingCount int

		// Get the next auto-increment value that would be used
		err = conn.DB.QueryRow(ctx, "SELECT nextval(pg_get_serial_sequence('securities', 'securityid'))").Scan(&nextID)
		if err != nil {
			return fmt.Errorf("failed to get next securityid: %v", err)
		}

		// Reset the sequence since we only wanted to peek at the value
		_, err = conn.DB.Exec(ctx, "SELECT setval(pg_get_serial_sequence('securities', 'securityid'), $1, false)", nextID)
		if err != nil {
			return fmt.Errorf("failed to reset sequence: %v", err)
		}

		// Check if this next ID would conflict with existing records for the same minDate
		err = conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM securities WHERE securityid = $1 AND minDate = $2", nextID, minDate).Scan(&existingCount)
		if err != nil {
			return fmt.Errorf("failed to check auto-increment collision: %v", err)
		}
		if existingCount > 0 {
			if debug {
				fmt.Printf("VALIDATION FAILED: auto-increment securityid=%d would conflict with existing record for minDate=%s\n", nextID, minDate)
			}
			return fmt.Errorf("constraint violation: auto-increment securityid=%d would conflict with existing record for minDate=%s", nextID, minDate)
		}
	}

	return nil
}

// safeInsertSecurityTickerChange performs validation and insertion specifically for ticker change scenarios
func safeInsertSecurityTickerChange(ctx context.Context, conn *data.Conn, ticker string, figi string, minDate string, maxDate interface{}, active bool, debug bool, actionDescription string) error {
	// Pre-insertion validation
	err := validateSecurityInsertionV2(ctx, conn, nil, ticker, minDate, figi, debug)
	if err != nil {
		if debug {
			fmt.Printf("VALIDATION ERROR for %s (ticker=%s, minDate=%s): %v\n", actionDescription, ticker, minDate, err)
		}
		return err
	}

	// Check if insertion would be redundant (already exists)
	var existingCount int
	err = conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM securities WHERE ticker = $1 AND minDate = $2", ticker, minDate).Scan(&existingCount)
	if err != nil {
		return fmt.Errorf("failed to check existing record: %v", err)
	}
	if existingCount > 0 {
		if debug {
			fmt.Printf("SKIPPING %s: Record already exists for ticker=%s, minDate=%s\n", actionDescription, ticker, minDate)
		}
		return nil // Not an error, just already exists
	}

	// Perform the insertion with ON CONFLICT to handle race conditions
	_, err = conn.DB.Exec(ctx, `INSERT INTO securities (ticker, figi, minDate, maxDate, active) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (ticker, minDate) DO NOTHING`, ticker, figi, minDate, maxDate, active)
	if err != nil {
		if debug {
			fmt.Printf("INSERTION ERROR for %s: %v\n", actionDescription, err)
			fmt.Printf("DIAGNOSTIC INFO for failed insertion:\n")
			fmt.Printf("  - Ticker: %s\n", ticker)
			fmt.Printf("  - FIGI: %s\n", figi)
			fmt.Printf("  - MinDate: %s\n", minDate)
			fmt.Printf("  - MaxDate: %v\n", maxDate)
			fmt.Printf("  - Active: %v\n", active)

			// Check what exists in the database around this time
			rows, diagErr := conn.DB.Query(ctx,
				"SELECT securityid, ticker, figi, minDate, maxDate FROM securities WHERE ticker = $1 OR minDate = $2 ORDER BY minDate DESC LIMIT 5",
				ticker, minDate)
			if diagErr == nil {
				fmt.Printf("  - Related records in database:\n")
				for rows.Next() {
					var secID int
					var dbTicker, dbFigi string
					var dbMinDate, dbMaxDate sql.NullTime
					if scanErr := rows.Scan(&secID, &dbTicker, &dbFigi, &dbMinDate, &dbMaxDate); scanErr == nil {
						fmt.Printf("    - ID: %d, Ticker: %s, FIGI: %s, MinDate: %v, MaxDate: %v\n",
							secID, dbTicker, dbFigi, dbMinDate, dbMaxDate)
					}
				}
				rows.Close()
			}
		}
		return fmt.Errorf("failed to insert %s: %w", actionDescription, err)
	}

	// Successful insertion
	if debug {
		fmt.Printf("SUCCESS: %s completed for ticker=%s, minDate=%s\n", actionDescription, ticker, minDate)
	}
	return nil
}

// safeInsertSecurityV2 performs validation and insertion with the extended columns format
func safeInsertSecurityV2(ctx context.Context, conn *data.Conn, ticker string, minDate string, maxDate interface{}, active bool, figi string, debug bool, actionDescription string) error {
	// Pre-insertion validation
	err := validateSecurityInsertionV2(ctx, conn, nil, ticker, minDate, figi, debug)
	if err != nil {
		if debug {
			fmt.Printf("VALIDATION ERROR for %s (ticker=%s, minDate=%s): %v\n", actionDescription, ticker, minDate, err)
		}
		return err
	}

	// Check if insertion would be redundant (already exists)
	var existingCount int
	err = conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM securities WHERE ticker = $1 AND minDate = $2", ticker, minDate).Scan(&existingCount)
	if err != nil {
		return fmt.Errorf("failed to check existing record: %v", err)
	}
	if existingCount > 0 {
		if debug {
			fmt.Printf("SKIPPING %s: Record already exists for ticker=%s, minDate=%s\n", actionDescription, ticker, minDate)
		}
		return nil // Not an error, just already exists
	}

	// Perform the insertion with ON CONFLICT to handle race conditions
	_, err = conn.DB.Exec(ctx, `INSERT INTO securities (ticker, minDate, maxDate, active, figi) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (ticker, minDate) DO NOTHING`, ticker, minDate, maxDate, active, figi)
	if err != nil {
		if debug {
			fmt.Printf("INSERTION ERROR for %s: %v\n", actionDescription, err)
			fmt.Printf("DIAGNOSTIC INFO for failed insertion:\n")
			fmt.Printf("  - Ticker: %s\n", ticker)
			fmt.Printf("  - MinDate: %s\n", minDate)
			fmt.Printf("  - MaxDate: %v\n", maxDate)
			fmt.Printf("  - Active: %v\n", active)
			fmt.Printf("  - FIGI: %s\n", figi)

			// Check what exists in the database around this time
			rows, diagErr := conn.DB.Query(ctx,
				"SELECT securityid, ticker, figi, minDate, maxDate FROM securities WHERE ticker = $1 OR minDate = $2 ORDER BY minDate DESC LIMIT 5",
				ticker, minDate)
			if diagErr == nil {
				fmt.Printf("  - Related records in database:\n")
				for rows.Next() {
					var secID int
					var dbTicker, dbFigi string
					var dbMinDate, dbMaxDate sql.NullTime
					if scanErr := rows.Scan(&secID, &dbTicker, &dbFigi, &dbMinDate, &dbMaxDate); scanErr == nil {
						fmt.Printf("    - ID: %d, Ticker: %s, FIGI: %s, MinDate: %v, MaxDate: %v\n",
							secID, dbTicker, dbFigi, dbMinDate, dbMaxDate)
					}
				}
				rows.Close()
			}
		}
		return fmt.Errorf("failed to insert %s: %w", actionDescription, err)
	}

	// Successful insertion
	if debug {
		fmt.Printf("SUCCESS: %s completed for ticker=%s, minDate=%s\n", actionDescription, ticker, minDate)
	}
	return nil
}
