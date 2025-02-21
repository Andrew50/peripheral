package jobs

import (
	"context"
	"fmt"
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
				// If it exists but was delisted, update the existing row
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
