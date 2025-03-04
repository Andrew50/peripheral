package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backend/utils"

	_ "github.com/lib/pq"
)

// updateSecurities fetches the latest tickers from Polygon, checks if AAPL is present,
// and if so, updates the securities table by marking missing tickers as delisted
// (maxDate = now) and inserting brand-new tickers (keeping the same securityId for existing ones).
func simpleUpdateSecurities(conn *utils.Conn) error {
	// 1) Fetch current list of Polygon tickers (use "today" or whichever date you prefer).
	today := time.Now().Format("2006-01-02")
	fmt.Println("running simpleUpdateSecurities")
	polyTickers, err := utils.AllTickers(conn.Polygon, today)
	fmt.Println("polyTickers", len(polyTickers))
	if err != nil {
		return fmt.Errorf("failed to fetch Polygon tickers: %w", err)
	}

	// 2) Check if AAPL is in the list. If not, do nothing and return.
	aaplExists := false
	tickerSet := make(map[string]struct{}, len(polyTickers))
	for _, sec := range polyTickers {
		tickerSet[sec.Ticker] = struct{}{}
		if sec.Ticker == "AAPL" {
			aaplExists = true
		}
	}
	if !aaplExists {
		fmt.Println("AAPL not found in Polygon tickers; skipping updates.")
		return nil
	}

	// 3) Fetch all currently active (maxDate IS NULL) tickers from the db.
	rows, err := conn.DB.Query(context.Background(),
		"SELECT securityId, ticker FROM securities WHERE maxDate IS NULL",
	)
	if err != nil {
		return fmt.Errorf("failed to query active securities: %w", err)
	}
	defer rows.Close()

	dbActiveTickers := make(map[string]int)
	for rows.Next() {
		var (
			sid    int
			ticker string
		)
		if err := rows.Scan(&sid, &ticker); err != nil {
			return fmt.Errorf("failed to scan security row: %w", err)
		}
		dbActiveTickers[ticker] = sid
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("row iteration error: %w", err)
	}

	// 4) For tickers in DB but not in Polygon -> mark them delisted (set maxDate = now()).
	for ticker := range dbActiveTickers {
		if _, found := tickerSet[ticker]; !found {
			_, err := conn.DB.Exec(context.Background(),
				`UPDATE securities 
                 SET maxDate = CURRENT_DATE 
                 WHERE ticker = $1 
                   AND maxDate IS NULL`,
				ticker,
			)
			if err != nil {
				return fmt.Errorf("failed to update maxDate for ticker %s: %w", ticker, err)
			}
		}
	}

	// 5) For tickers in Polygon but not in DB -> insert a new row (preserving old ones).
	for _, sec := range polyTickers {
		if _, found := dbActiveTickers[sec.Ticker]; !found {
			// Check if the ticker exists at all, regardless of maxDate
			var exists bool
			err := conn.DB.QueryRow(context.Background(),
				`SELECT EXISTS (
					SELECT 1 FROM securities 
					WHERE ticker = $1 
					AND minDate = '2003-10-01'
				)`,
				sec.Ticker,
			).Scan(&exists)
			if err != nil {
				return fmt.Errorf("failed to check ticker existence %s: %w", sec.Ticker, err)
			}

			if exists {
				// Skip if we already have an entry for this ticker today
				continue
			}

			// Check if there's a delisted version
			var hasDelisted bool
			err = conn.DB.QueryRow(context.Background(),
				`SELECT EXISTS (
					SELECT 1 FROM securities 
					WHERE ticker = $1 
					AND maxDate IS NOT NULL
				)`,
				sec.Ticker,
			).Scan(&hasDelisted)
			if err != nil {
				return fmt.Errorf("failed to check delisted ticker %s: %w", sec.Ticker, err)
			}

			if hasDelisted {
				// First check if there's already an active record with this ticker
				var activeExists bool
				err := conn.DB.QueryRow(context.Background(),
					`SELECT EXISTS(
						SELECT 1 FROM securities 
						WHERE ticker = $1 
						AND maxDate IS NULL
					)`,
					sec.Ticker,
				).Scan(&activeExists)

				if err != nil {
					return fmt.Errorf("failed to check active ticker %s: %w", sec.Ticker, err)
				}

				if activeExists {
					// Skip this ticker as it's already active
					fmt.Printf("Skipping reactivation of %s as it's already active\n", sec.Ticker)
					continue
				}

				// Check if there's a record with the same ticker and minDate
				var duplicateExists bool
				err = conn.DB.QueryRow(context.Background(),
					`SELECT EXISTS(
						SELECT 1 FROM securities 
						WHERE ticker = $1 
						AND minDate = '2003-10-01'
						AND maxDate IS NOT NULL
					)`,
					sec.Ticker,
				).Scan(&duplicateExists)

				if err != nil {
					return fmt.Errorf("failed to check duplicate ticker %s: %w", sec.Ticker, err)
				}

				if duplicateExists {
					// If a duplicate exists, use a different approach - update the most recent record
					_, err := conn.DB.Exec(context.Background(),
						`UPDATE securities 
						SET maxDate = NULL,
							figi = $2
						WHERE ticker = $1 
						AND maxDate IS NOT NULL
						ORDER BY maxDate DESC
						LIMIT 1`,
						sec.Ticker, sec.CompositeFIGI,
					)
					if err != nil {
						return fmt.Errorf("failed to reactivate ticker %s with alternative approach: %w", sec.Ticker, err)
					}
				} else {
					// If it exists but was delisted and no duplicate exists, update the existing row
					_, err := conn.DB.Exec(context.Background(),
						`UPDATE securities 
						SET maxDate = NULL,
							minDate = '2003-10-01',
							figi = $2
						WHERE ticker = $1 
						AND maxDate IS NOT NULL`,
						sec.Ticker, sec.CompositeFIGI,
					)
					if err != nil {
						return fmt.Errorf("failed to reactivate ticker %s: %w", sec.Ticker, err)
					}
				}
			} else {
				// If it's completely new, insert it
				_, err := conn.DB.Exec(context.Background(),
					`INSERT INTO securities (ticker, minDate, figi) 
					VALUES ($1, '2003-10-01', $2)`,
					sec.Ticker, sec.CompositeFIGI,
				)
				if err != nil {
					return fmt.Errorf("failed to insert new ticker %s: %w", sec.Ticker, err)
				}
			}
		}
	}

	fmt.Println("Securities table updated successfully.")
	return nil

}

// updateSecurityCik fetches the latest CIK (Central Index Key) data from the SEC API
// and updates the securities table with CIK values for active securities.
func updateSecurityCik(conn *utils.Conn) error {
	// Create a client and request with appropriate headers
	fmt.Println("🟢 fetching sec company tickers")
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
		CikStr int    `json:"cik_str"`
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

	fmt.Println("🟢 Securities CIK values updated successfully.")
	return nil
}
