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
	fmt.Printf("RUNNING SIMPLE UPDATE SECURITIES from %s to %s\n", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	// Loop through each date from startDate to endDate
	for currentDate := startDate; !currentDate.After(endDate); currentDate = currentDate.AddDate(0, 0, 1) {
		if currentDate.Weekday() == time.Saturday || currentDate.Weekday() == time.Sunday {
			continue
		}
		targetDateStr := currentDate.Format("2006-01-02")
		fmt.Printf("Processing date: %s\n", targetDateStr)

		// 1) Fetch the tickers from Polygon for this date
		poly, err := polygon.AllTickers(conn.Polygon, targetDateStr)
		if err != nil {
			fmt.Printf("Warning: failed to fetch polygon tickers for %s: %v\n", targetDateStr, err)
			continue // Skip this date and continue with the next
		}

		// collect just the symbols
		tickers := make([]string, len(poly))
		for i, s := range poly {
			tickers[i] = s.Ticker
		}

		if len(tickers) == 0 {
			fmt.Printf("No tickers found for %s, skipping\n", targetDateStr)
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
			for _, ipo := range ipos {
				if _, err := conn.DB.Exec(ctx, `
					INSERT INTO securities (ticker, mindate, figi) 
					VALUES ($1, $2, $3)
				`, ipo, currentDate, ""); err != nil {
					fmt.Printf("Warning: failed to insert IPO ticker %s for %s: %v\n", ipo, targetDateStr, err)
				} else {
					fmt.Printf("Inserted IPO ticker: %s with mindate: %s\n", ipo, targetDateStr)
				}
			}
		}
		fmt.Printf("Completed processing for %s (%d tickers)\n", targetDateStr, len(tickers))
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
