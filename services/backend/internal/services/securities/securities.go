package securities

import (
	"backend/internal/data/polygon"
	"backend/internal/data/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
		if _, err := data.ExecWithRetry(ctx, conn.DB, `
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
		if _, err := data.ExecWithRetry(ctx, conn.DB, `
			UPDATE securities
			   SET maxDate = NULL
			 WHERE maxDate IS NOT NULL
			   AND ticker IN (`+placeholders(len(tickers))+`)
		`, stringArgs(tickers)...); err != nil {
			return fmt.Errorf("reactivate tickers for %s: %w", targetDateStr, err)
		}
		ipos, err := polygon.GetPolygonIPOs(conn.Polygon, targetDateStr)
		if err != nil {
			// Continue processing even if IPO fetch fails
		} else {
			// 4) INSERT new IPO tickers with mindate = current date
			for _, ipo := range ipos.Tickers {
				if _, err := data.ExecWithRetry(ctx, conn.DB, `
					INSERT INTO securities (ticker, mindate, figi) 
					VALUES ($1, $2, $3)
					ON CONFLICT (ticker, mindate) DO NOTHING
				`, ipo, currentDate, ""); err != nil {
					// Continue processing other IPOs even if one fails
				}
			}
		}
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
	resp, err := data.DoWithRetry(client, req)
	if err != nil {
		return fmt.Errorf("failed to fetch SEC company tickers: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}()

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
		_, err := data.ExecWithRetry(context.Background(), conn.DB,
			`UPDATE securities 
			 SET cik = $1 
			 WHERE ticker = $2 
			 AND maxDate IS NULL 
			 AND (cik IS NULL)`,
			company.CikStr, company.Ticker,
		)
		if err != nil {
			// Log the failure and continue with other tickers instead of aborting the whole job
			log.Printf("failed to update CIK for ticker %s after retries: %v", company.Ticker, err)
			continue
		}
	}

	return nil
}

// Alternative more efficient approach for large datasets
func SimpleUpdateSecuritiesV2(conn *data.Conn) error {
	// Add timeout to prevent hanging indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

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
	// che
	processedTickers := make(map[string]struct{})

	// Use America/New_York timezone for all market operations
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return fmt.Errorf("failed to load America/New_York timezone: %w", err)
	}

	todayNY := time.Now().In(nyLoc).Format("2006-01-02")

	ipos, err := polygon.GetPolygonIPOs(conn.Polygon, todayNY)
	if err != nil {
		return fmt.Errorf("failed to fetch polygon IPOs: %w", err)
	}

	for _, ipo := range ipos.Tickers {

		// Parse the listing date string into time.Time
		listingDate, err := time.Parse("2006-01-02", ipos.ListingDate)
		if err != nil {
			return fmt.Errorf("failed to parse listing date %s: %w", ipos.ListingDate, err)
		}
		// first check if this ipo is already in the database
		var existingSecurityID int
		err = conn.DB.QueryRow(ctx, `
			SELECT securityid 
			FROM securities 
			WHERE ticker = $1 AND minDate = $2`,
			ipo, listingDate).Scan(&existingSecurityID)
		if err != nil && err != pgx.ErrNoRows {
			return fmt.Errorf("failed to check if IPO ticker %s is already in the database: %w", ipo, err)
		}
		if err == nil && existingSecurityID != 0 {
			continue
		}

		// Insert new IPO security with listing date as minDate and NULL maxDate
		_, err = data.ExecWithRetry(ctx, conn.DB, `
			INSERT INTO securities (ticker, minDate, maxDate, active, figi) 
			VALUES ($1, $2, NULL, true, $3)
			ON CONFLICT (ticker, minDate) DO NOTHING`,
			ipo,
			listingDate,
			"", // Empty string for figi since we don't have it yet
		)
		if err != nil {
			continue
		}
		processedTickers[ipo] = struct{}{}
	}
	res, err := polygon.GetAllStocksDailyOHLCV(ctx, conn.Polygon, todayNY)
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
				continue
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
						continue
					}
					processedTickers[ticker] = struct{}{}
					continue
				}

				// Second Check if this is a ticker change
				res, err := postgres.GetTickerEventsCustom(conn.Polygon, ticker, conn.PolygonKey)
				if err != nil {
					continue
				}

				// Check for splits around the processing date
				isSplitDetected := false
				events := res.Events
				for _, event := range events {
					if event.Date == processingDate {
						// First check if this is just a split
						splits, err := postgres.GetStockSplits(conn.Polygon, ticker)
						if err != nil {
							continue
						}

						// Parse processing date to time.Time for comparison
						processDatetime, err := time.Parse("2006-01-02", processingDate)
						if err != nil {
							continue
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
									continue
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
				getTickerDetailsResponse, err := polygon.GetTickerDetails(conn.Polygon, ticker, processingDate)
				if err != nil {
					continue
				}
				cik, err := strconv.ParseInt(getTickerDetailsResponse.CIK, 10, 64)
				if err != nil {
					continue
				}
				figi := getTickerDetailsResponse.CompositeFIGI

				// Parse processing date and subtract 1 day
				processDatetime, err := time.Parse("2006-01-02", processingDate)
				if err != nil {
					continue
				}
				previousDay := processDatetime.AddDate(0, 0, -1).Format("2006-01-02")

				oldTicker, err := postgres.GetTickerFromCIK(conn.Polygon, int(cik), previousDay)
				if err != nil {
					continue
				}
				_, err = conn.DB.Exec(ctx, `UPDATE securities SET maxDate = $1 WHERE ticker=$2 AND securityId=$3 AND maxDate IS NULL AND NOT EXISTS (SELECT 1 FROM securities WHERE ticker = $2 AND maxDate = $1)`, processingDate, oldTicker, securityID)
				if err != nil {
					continue
				}
				// Use safe insertion with comprehensive validation
				err = safeInsertSecurityTickerChange(ctx, conn, ticker, figi, processingDate, nil, true, false, "ticker change")
				if err != nil {
					continue
				}
				processedTickers[ticker] = struct{}{}
				continue
			}
			// If no record found, then this is a new ticker for our DB
			// Figure out the start date of the ticker via 'ticker change' api lol
			err = processTickerEventsForNewListing(ctx, conn, ticker)
			if err != nil {
				continue
			}
			processedTickers[ticker] = struct{}{}
			continue
		}
		// we need to verify that there isn't more than one active row with this ticker
		var activeTickerRowCount int
		err := conn.DB.QueryRow(ctx, `
			SELECT COUNT(*) 
			FROM securities 
			WHERE ticker = $1 AND maxDate IS NULL`,
			ticker).Scan(&activeTickerRowCount)
		if err != nil {
			continue
		}
		if activeTickerRowCount > 1 {
			// Multiple active rows exist, keep only the one with highest securityid

			// Delete all active rows except the one with the highest securityid
			_, err := conn.DB.Exec(ctx, `
				DELETE FROM securities 
				WHERE ticker = $1 AND maxDate IS NULL AND securityid NOT IN (
					SELECT MIN(securityid) 
					FROM securities 
					WHERE ticker = $1 AND maxDate IS NULL
				)`,
				ticker)
			if err != nil {
				continue
			}
		}

		// we need to verify that there isn't like an overlapping / different stock with the same securityid
		// Get the securityid for the current ticker with maxdate = null
		var currentSecurityID int
		err = conn.DB.QueryRow(ctx, `
			SELECT securityid 
			FROM securities 
			WHERE ticker = $1 AND maxDate IS NULL`,
			ticker).Scan(&currentSecurityID)
		if err != nil {
			continue
		}

		// Get all rows with the same securityid
		rows, err := conn.DB.Query(ctx, `
			SELECT securityid, ticker, minDate, maxDate 
			FROM securities 
			WHERE securityid = $1 
			ORDER BY minDate`,
			currentSecurityID)
		if err != nil {
			continue
		}

		// Process the rows to check for overlaps or conflicts
		var conflictingRows []struct {
			securityID int
			ticker     string
			minDate    time.Time
			maxDate    *time.Time
		}

		for rows.Next() {
			var secID int
			var rowTicker string
			var minDate time.Time
			var maxDate *time.Time

			if err := rows.Scan(&secID, &rowTicker, &minDate, &maxDate); err != nil {
				continue
			}

			conflictingRows = append(conflictingRows, struct {
				securityID int
				ticker     string
				minDate    time.Time
				maxDate    *time.Time
			}{secID, rowTicker, minDate, maxDate})
		}

		// Check for iteration errors
		if err = rows.Err(); err != nil {
			fmt.Printf("error iterating conflicting rows for ticker %s: %v\n", ticker, err)
		}

		// Explicitly close rows to prevent resource leak
		rows.Close()

		// Handle conflicts if multiple active rows exist
		if len(conflictingRows) > 1 {
			// Check if there are multiple active (maxDate = null) rows
			var activeRows []struct {
				securityID int
				ticker     string
				minDate    time.Time
				maxDate    *time.Time
			}
			for _, row := range conflictingRows {
				if row.maxDate == nil {
					activeRows = append(activeRows, row)
				}
			}

			if len(activeRows) > 1 {
				var currentTickerRow *struct {
					securityID int
					ticker     string
					minDate    time.Time
					maxDate    *time.Time
				}
				// First, find and store the current ticker's data
				for i := range activeRows {
					if activeRows[i].ticker == ticker {
						currentTickerRow = &activeRows[i]
						break
					}
				}
				if currentTickerRow == nil {
					continue
				}
				// Simply update the current ticker's securityid to the next available serial value
				_, err := conn.DB.Exec(ctx, `
				UPDATE securities 
				SET securityid = nextval('securities_securityid_seq')
				WHERE securityid = $1 AND ticker = $2 AND maxDate IS NULL`,
					currentSecurityID, ticker)
				if err != nil {
					continue
				}

				// Delist all remaining active rows on the original securityid
				_, err = conn.DB.Exec(ctx, `
				UPDATE securities 
				SET maxDate = $1 
				WHERE securityid = $2 AND maxDate IS NULL`,
					processingDate, currentSecurityID)
				if err != nil {
					continue
				}
				// this is just inserting the previous ticker (ticker changes)
				err = processTickerEventsForExistingSecurity(ctx, conn, ticker, currentSecurityID)
				if err != nil {
					continue
				}
			}
			processedTickers[ticker] = struct{}{}
			continue
		}

		// testing if the ticker changes are correct
		err = processTickerEventsWithConflictResolution(ctx, conn, ticker, currentSecurityID)
		if err != nil {
			continue
		}

		// Mark ticker as processed
		processedTickers[ticker] = struct{}{}
	}

	// Continue with the polygon.ListTickers part...
	iter, err := polygon.ListTickers(conn.Polygon, "", "", "gte", 1000, true)
	if err != nil {
		return nil
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
					continue
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
							continue
						}
						processedTickers[ticker] = struct{}{}
						continue
					}
				}
				// If no record found, then this is a new ticker for our DB
				// Figure out the start date of the ticker via 'ticker change' api lol
				err = processTickerEventsForNewListing(ctx, conn, ticker)
				if err != nil {
					continue
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
			return nil
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
				continue
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
				continue
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
		return err
	}

	// Check if insertion would be redundant (already exists)
	var existingCount int
	err = conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM securities WHERE ticker = $1 AND minDate = $2", ticker, minDate).Scan(&existingCount)
	if err != nil {
		return fmt.Errorf("failed to check existing record: %v", err)
	}
	if existingCount > 0 {
		return nil // Not an error, just already exists
	}

	// Perform the insertion with ON CONFLICT to handle race conditions
	_, err = conn.DB.Exec(ctx, `INSERT INTO securities (ticker, figi, minDate, maxDate, active) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (ticker, minDate) DO NOTHING`, ticker, figi, minDate, maxDate, active)
	if err != nil {
		return fmt.Errorf("failed to insert %s: %w", actionDescription, err)
	}

	return nil
}

// safeInsertSecurityV2 performs validation and insertion with the extended columns format
func safeInsertSecurityV2(ctx context.Context, conn *data.Conn, ticker string, minDate string, maxDate interface{}, active bool, figi string, debug bool, actionDescription string) error {
	// Pre-insertion validation
	err := validateSecurityInsertionV2(ctx, conn, nil, ticker, minDate, figi, debug)
	if err != nil {
		return err
	}

	// Check if insertion would be redundant (already exists)
	var existingCount int
	err = conn.DB.QueryRow(ctx, "SELECT COUNT(*) FROM securities WHERE ticker = $1 AND minDate = $2", ticker, minDate).Scan(&existingCount)
	if err != nil {
		return fmt.Errorf("failed to check existing record: %v", err)
	}
	if existingCount > 0 {
		return nil // Not an error, just already exists
	}

	// Perform the insertion with ON CONFLICT to handle race conditions
	_, err = conn.DB.Exec(ctx, `INSERT INTO securities (ticker, minDate, maxDate, active, figi) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (ticker, minDate) DO NOTHING`, ticker, minDate, maxDate, active, figi)
	if err != nil {
		return fmt.Errorf("failed to insert %s: %w", actionDescription, err)
	}

	return nil
}

// processTickerEventsForNewListing handles ticker change events for a new ticker listing
func processTickerEventsForNewListing(ctx context.Context, conn *data.Conn, ticker string) error {
	res, err := postgres.GetTickerEventsCustom(conn.Polygon, ticker, conn.PolygonKey)
	if err != nil {
		return fmt.Errorf("failed to get ticker events for ticker %s: %w", ticker, err)
	}

	// Check if we have any events before accessing them
	if len(res.Events) == 0 {
		return nil
	}

	events := res.Events
	// this is sorted by date, so the most historical one is last
	var maxDateToInsert interface{}
	var isActive bool
	tickerToInsert := ticker
	// this handles ticker changes
	for eventIndex, event := range events {
		if eventIndex == 0 {
			maxDateToInsert = nil
		}
		initialDate := event.Date
		getTickerDetailsResponse, err := polygon.GetTickerDetails(conn.Polygon, tickerToInsert, initialDate)
		var figi string
		if err != nil {
			figi = ""
		} else {
			figi = getTickerDetailsResponse.CompositeFIGI
		}

		tickerToInsert = event.TickerChange["ticker"].(string)
		// first check if this is already in the database
		var existingSecurityID int
		err = conn.DB.QueryRow(ctx, `
			SELECT securityid 
			FROM securities 
			WHERE ticker = $1 AND minDate = $2 AND maxDate= $3`,
			tickerToInsert, initialDate, maxDateToInsert).Scan(&existingSecurityID)
		if err != nil && err != pgx.ErrNoRows {
			return fmt.Errorf("failed to check if ticker %s is already in the database: %w", tickerToInsert, err)
		}
		if err == nil && existingSecurityID != 0 {
			continue
		}

		// Use safe insertion with comprehensive validation
		err = safeInsertSecurityV2(ctx, conn, tickerToInsert, initialDate, maxDateToInsert, isActive, figi, true, "new ticker listing")
		if err != nil {
			// If insertion failed and tickers are similar, try with original ticker as fallback
			if shouldUseOriginalTicker(ticker, tickerToInsert) {
				err = safeInsertSecurityV2(ctx, conn, ticker, initialDate, maxDateToInsert, isActive, figi, true, "new ticker listing (fallback to original)")
				if err != nil {
					return fmt.Errorf("failed to insert new ticker %s (tried both %s and %s): %w", ticker, tickerToInsert, ticker, err)
				}
			} else {
				return fmt.Errorf("failed to insert new ticker %s: %w", tickerToInsert, err)
			}
		}

		// Parse the date string, subtract a day, and format back to string
		eventDate, err := time.Parse("2006-01-02", event.Date)
		if err != nil {
			return fmt.Errorf("failed to parse event date %s: %w", event.Date, err)
		}
		maxDateToInsert = eventDate.AddDate(0, 0, -1).Format("2006-01-02")
		isActive = false
	}

	return nil

}

// processTickerEventsForExistingSecurity handles ticker change events for an existing security with a specific securityID
func processTickerEventsForExistingSecurity(ctx context.Context, conn *data.Conn, ticker string, currentSecurityID int) error {
	res, err := postgres.GetTickerEventsCustom(conn.Polygon, ticker, conn.PolygonKey)
	if err != nil {
		return fmt.Errorf("failed to get ticker events for ticker %s: %w", ticker, err)
	}

	// Check if we have any events before accessing them
	if len(res.Events) == 0 {
		return nil
	}

	events := res.Events
	// this is sorted by date, so the most historical one is last
	var maxDateToInsert interface{}

	// this handles ticker changes
	for eventIndex, event := range events {
		if eventIndex == 0 {
			// Parse the date string, subtract a day, and format back to string
			eventDate, err := time.Parse("2006-01-02", event.Date)
			if err != nil {
				return fmt.Errorf("failed to parse event date %s: %w", event.Date, err)
			}
			maxDateToInsert = eventDate.AddDate(0, 0, -1).Format("2006-01-02")
			continue
		}

		initialDate := event.Date
		tickerToInsert := event.TickerChange["ticker"].(string)

		getTickerDetailsResponse, err := polygon.GetTickerDetails(conn.Polygon, tickerToInsert, initialDate)
		var figi string
		if err != nil {
			figi = ""
		} else {
			figi = getTickerDetailsResponse.CompositeFIGI
		}

		// first check if this row already exists
		var existingEventRowCount int
		err = conn.DB.QueryRow(ctx, `
			SELECT COUNT(*) 
			FROM securities 
			WHERE ticker = $1 AND minDate = $2`,
			tickerToInsert, initialDate).Scan(&existingEventRowCount)
		if err != nil {
			return fmt.Errorf("failed to check existing row for ticker %s: %w", tickerToInsert, err)
		}

		if existingEventRowCount > 0 {
			// delete it so we can refresh it - delete ALL rows with this ticker and minDate
			_, err = conn.DB.Exec(ctx, `
				DELETE FROM securities 
				WHERE ticker = $1 AND minDate = $2`,
				tickerToInsert, initialDate)
			if err != nil {
				return fmt.Errorf("failed to delete existing row for ticker %s: %w", tickerToInsert, err)
			}
		}

		// Insert with the current ticker's securityid to maintain the relationship
		_, err = conn.DB.Exec(ctx, `
			INSERT INTO securities (securityid, ticker, minDate, maxDate, active, figi) 
			VALUES ($1, $2, $3, $4, $5, $6) 
			ON CONFLICT (ticker, minDate) DO NOTHING`,
			currentSecurityID, tickerToInsert, initialDate, maxDateToInsert, false, figi)
		if err != nil {
			// If insertion failed and tickers are similar, try with original ticker as fallback
			if shouldUseOriginalTicker(ticker, tickerToInsert) {
				_, err = conn.DB.Exec(ctx, `
					INSERT INTO securities (securityid, ticker, minDate, maxDate, active, figi) 
					VALUES ($1, $2, $3, $4, $5, $6) 
					ON CONFLICT (ticker, minDate) DO NOTHING`,
					currentSecurityID, ticker, initialDate, maxDateToInsert, false, figi)
				if err != nil {
					return fmt.Errorf("failed to insert new ticker %s (tried both %s and %s): %w", ticker, tickerToInsert, ticker, err)
				}
			} else {
				return fmt.Errorf("failed to insert new ticker %s: %w", tickerToInsert, err)
			}
		}

		// Parse the date string, subtract a day, and format back to string
		eventDate, err := time.Parse("2006-01-02", event.Date)
		if err != nil {
			return fmt.Errorf("failed to parse event date %s: %w", event.Date, err)
		}
		maxDateToInsert = eventDate.AddDate(0, 0, -1).Format("2006-01-02")
	}

	return nil
}

// processTickerEventsWithConflictResolution handles ticker change events with advanced conflict resolution
func processTickerEventsWithConflictResolution(ctx context.Context, conn *data.Conn, ticker string, currentSecurityID int) error {
	res, err := postgres.GetTickerEventsCustom(conn.Polygon, ticker, conn.PolygonKey)
	if err != nil {
		return fmt.Errorf("failed to get ticker events for ticker %s: %w", ticker, err)
	}

	// Check if we have any events before accessing them
	if len(res.Events) == 0 {
		return nil
	}

	events := res.Events
	// this is sorted by date, so the most historical one is last
	var maxDateToInsert interface{}

	// this handles ticker changes
	for eventIndex, event := range events {
		if eventIndex == 0 {
			// Parse the date string, subtract a day, and format back to string
			eventDate, err := time.Parse("2006-01-02", event.Date)
			if err != nil {
				return fmt.Errorf("failed to parse event date %s: %w", event.Date, err)
			}
			maxDateToInsert = eventDate.AddDate(0, 0, -1).Format("2006-01-02")
			continue
		}

		initialDate := event.Date
		tickerToInsert := event.TickerChange["ticker"].(string)

		// Check if a row already exists with this ticker and date range
		var existingCount int
		err = conn.DB.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM securities 
		WHERE ticker = $1 AND minDate = $2 AND maxDate = $3 AND securityid = $4`,
			tickerToInsert, initialDate, maxDateToInsert, currentSecurityID).Scan(&existingCount)
		if err != nil {
			return fmt.Errorf("failed to check existing row for ticker %s: %w", ticker, err)
		}

		if existingCount > 0 {
			// Row already exists, skip insertion
			// Parse the date string, subtract a day, and format back to string
			eventDate, err := time.Parse("2006-01-02", event.Date)
			if err != nil {
				return fmt.Errorf("failed to parse event date %s: %w", event.Date, err)
			}
			maxDateToInsert = eventDate.AddDate(0, 0, -1).Format("2006-01-02")
			continue
		}

		var existingEventWrongSecurityIDCount int
		err = conn.DB.QueryRow(ctx, `
			SELECT COUNT(*) 
			FROM securities 
			WHERE ticker = $1 AND minDate = $2`,
			tickerToInsert, initialDate).Scan(&existingEventWrongSecurityIDCount)
		if err != nil {
			return fmt.Errorf("failed to check existing row for ticker %s: %w", ticker, err)
		}

		if existingEventWrongSecurityIDCount > 0 {
			// delete it so we can refresh it - delete ALL rows with this ticker and minDate
			_, err = conn.DB.Exec(ctx, `
				DELETE FROM securities 
				WHERE ticker = $1 AND minDate = $2`,
				tickerToInsert, initialDate)
			if err != nil {
				return fmt.Errorf("failed to delete existing row for ticker %s: %w", ticker, err)
			}
		}

		getTickerDetailsResponse, err := polygon.GetTickerDetails(conn.Polygon, tickerToInsert, initialDate)
		var figi string
		if err != nil {
			figi = ""
		} else {
			figi = getTickerDetailsResponse.CompositeFIGI
		}

		// Insert with the current ticker's securityid to maintain the relationship
		_, err = conn.DB.Exec(ctx, `
			INSERT INTO securities (securityid, ticker, minDate, maxDate, active, figi) 
			VALUES ($1, $2, $3, $4, $5, $6) 
			ON CONFLICT (ticker, minDate) DO NOTHING`,
			currentSecurityID, tickerToInsert, initialDate, maxDateToInsert, false, figi)
		if err != nil {
			// If insertion failed and tickers are similar, try with original ticker as fallback
			if shouldUseOriginalTicker(ticker, tickerToInsert) {
				_, err = conn.DB.Exec(ctx, `
					INSERT INTO securities (securityid, ticker, minDate, maxDate, active, figi) 
					VALUES ($1, $2, $3, $4, $5, $6) 
					ON CONFLICT (ticker, minDate) DO NOTHING`,
					currentSecurityID, ticker, initialDate, maxDateToInsert, false, figi)
				if err != nil {
					return fmt.Errorf("failed to insert new ticker %s (tried both %s and %s): %w", ticker, tickerToInsert, ticker, err)
				}
			} else {
				return fmt.Errorf("failed to insert new ticker %s: %w", tickerToInsert, err)
			}
		}

		// Parse the date string, subtract a day, and format back to string
		eventDate, err := time.Parse("2006-01-02", event.Date)
		if err != nil {
			return fmt.Errorf("failed to parse event date %s: %w", event.Date, err)
		}
		maxDateToInsert = eventDate.AddDate(0, 0, -1).Format("2006-01-02")
	}

	return nil
}

// shouldUseOriginalTicker checks if the event ticker is similar enough to the original ticker
// to warrant using the original ticker instead of the event ticker
func shouldUseOriginalTicker(originalTicker, eventTicker string) bool {
	// If they're the same, use original
	if originalTicker == eventTicker {
		return true
	}

	// Convert to uppercase for comparison
	orig := strings.ToUpper(originalTicker)
	event := strings.ToUpper(eventTicker)

	// Find the length of common prefix
	minLen := len(orig)
	if len(event) < minLen {
		minLen = len(event)
	}

	commonPrefixLen := 0
	for i := 0; i < minLen; i++ {
		if orig[i] == event[i] {
			commonPrefixLen++
		} else {
			break
		}
	}

	// If no common prefix or very short prefix, don't use original
	if commonPrefixLen < 3 {
		return false
	}

	// Get the suffixes after the common prefix
	origSuffix := orig[commonPrefixLen:]
	eventSuffix := event[commonPrefixLen:]

	// If both suffixes are 2 characters or less, and the common prefix is substantial,
	// use the original ticker
	if len(origSuffix) <= 2 && len(eventSuffix) <= 2 && commonPrefixLen >= len(orig)-2 {
		return true
	}

	return false
}
