package securities

import (
	"backend/internal/data/polygon"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"backend/internal/data"

	//lint:ignore U1000 external package
	_ "github.com/lib/pq" // Register postgres driver
)

// SecurityDetail represents a structure for handling SecurityDetail data.

func SimpleUpdateSecurities(conn *data.Conn) error {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02") // It's good practice to use CURRENT_DATE in SQL for this

	// 1) Fetch the tickers from Polygon
	poly, err := polygon.AllTickers(conn.Polygon, today)
	if err != nil {
		return fmt.Errorf("fetch polygon tickers: %w", err)
	}

	// collect just the symbols
	tickers := make([]string, len(poly))
	for i, s := range poly {
		tickers[i] = s.Ticker
	}

	// 2) Mark as DELISTED any ticker NOT in today's list
	//    Only update if a record for this ticker with CURRENT_DATE as maxDate doesn't already exist
	//    to prevent duplicate key errors on (ticker, maxdate).
	//    We are updating rows where s.maxDate IS NULL.
	//    The conflict arises if another row s_check exists with s_check.ticker = s.ticker AND s_check.maxDate = CURRENT_DATE.
	delistSQL := `
        UPDATE securities s
           SET maxDate = CURRENT_DATE
         WHERE s.maxDate IS NULL
           AND s.ticker NOT IN (` + placeholders(len(tickers)) + `)
           AND NOT EXISTS (
               SELECT 1
               FROM securities s_check
               WHERE s_check.ticker = s.ticker
                 AND s_check.maxDate = CURRENT_DATE
           )`
	if len(tickers) > 0 { // Only run if there are tickers to compare against
		if _, err := conn.DB.Exec(ctx, delistSQL, stringArgs(tickers)...); err != nil {
			return fmt.Errorf("delist tickers: %w", err)
		}
	} else {
		// Handle the case where polygon returns no tickers.
		// Potentially delist all securities that have maxDate IS NULL,
		// but still with the NOT EXISTS check.
		delistAllSQL := `
            UPDATE securities s
               SET maxDate = CURRENT_DATE
             WHERE s.maxDate IS NULL
               AND NOT EXISTS (
                   SELECT 1
                   FROM securities s_check
                   WHERE s_check.ticker = s.ticker
                     AND s_check.maxDate = CURRENT_DATE
               )`
		if _, err := conn.DB.Exec(ctx, delistAllSQL); err != nil {
			return fmt.Errorf("delist all tickers (empty polygon list): %w", err)
		}
	}

	// 3) REACTIVATE any ticker IN today's list
	//    Set maxDate to NULL for tickers that are in the current list but might have an old maxDate.
	//    To prevent potential issues if (ticker, NULL) already exists (e.g. from bad data or a previous partial run),
	//    we add a similar NOT EXISTS check.
	reactivateSQL := `
        UPDATE securities s
           SET maxDate = NULL
         WHERE s.maxDate IS NOT NULL 
           AND s.ticker IN (` + placeholders(len(tickers)) + `)
           AND NOT EXISTS (
               SELECT 1
               FROM securities s_check
               WHERE s_check.ticker = s.ticker
                 AND s_check.maxDate IS NULL 
           )`
	if len(tickers) > 0 { // Only run if there are tickers to reactivate
		if _, err := conn.DB.Exec(ctx, reactivateSQL, stringArgs(tickers)...); err != nil {
			return fmt.Errorf("reactivate tickers: %w", err)
		}
	}
	// If len(tickers) is 0, no tickers are active, so no reactivation is needed.

	return nil
}

// placeholders(n) returns "$1,$2,…,$n"
func placeholders(n int) string {
	if n == 0 {
		// Handle cases where the list might be empty to avoid invalid SQL like "IN ()"
		// For NOT IN, an empty list means "not in nothing", which is true for all.
		// For IN, an empty list means "in nothing", which is false for all.
		// A common way to handle empty list for NOT IN is to ensure it doesn't match anything,
		// or better, structure the query to avoid it if the list is empty.
		// However, for PostgreSQL, `ticker NOT IN (NULL)` or `ticker NOT IN ()` can behave unexpectedly or error.
		// A safe bet for an empty list for `NOT IN` is to make it a condition that's always true,
		// or simply not execute the part of the query if the list is empty, as done above.
		// For `IN` with an empty list, it should evaluate to false.
		// `placeholders` function will likely not be called with n=0 if checks are in place.
		return "NULL" // This would make `col IN (NULL)` effectively false, and `col NOT IN (NULL)` also effectively false or behave strangely.
		// It's better to avoid calling placeholders(0) and handle the empty list logic in the main function.
	}
	ps := make([]string, n)
	for i := range ps {
		ps[i] = fmt.Sprintf("$%d", i+1)
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
	req.Header.Set("User-Agent", "Atlantis Equities admin@atlantis.trading") // Replace with your actual app name and contact
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
	// This part seems okay as it targets specific rows (maxDate IS NULL) and only updates cik.
	// The unique constraint is on (ticker, maxDate), and this operation doesn't change those for the matched row.
	for _, company := range secData {
		_, err := conn.DB.Exec(context.Background(),
			`UPDATE securities 
			 SET cik = $1 
			 WHERE ticker = $2 
			 AND maxDate IS NULL 
			 AND (cik IS NULL OR cik != $1)`, // Only update if CIK is NULL or different
			company.CikStr, company.Ticker,
		)
		if err != nil {
			// Log the error but continue, or decide if one failure should stop all.
			// For now, returning the first error.
			return fmt.Errorf("failed to update CIK for ticker %s: %w", company.Ticker, err)
		}
	}

	return nil
}
