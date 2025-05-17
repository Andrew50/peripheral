package securities

import (
        "backend/internal/data/polygon"
        "context"
        "encoding/json"
        "errors"
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
        today := time.Now().Format("2006-01-02")

        // 1) Fetch the tickers from Polygon
        poly, err := polygon.AllTickers(conn.Polygon, today)
        if err != nil {
                return fmt.Errorf("fetch polygon tickers: %w", err)
        }

        // sanitize and deduplicate the symbols
        tickerSet := make(map[string]struct{})
        tickers := make([]string, 0, len(poly))
        for _, s := range poly {
                t := strings.ToUpper(strings.TrimSpace(s.Ticker))
                if t == "" {
                        continue
                }
                if _, exists := tickerSet[t]; exists {
                        continue
                }
                tickerSet[t] = struct{}{}
                tickers = append(tickers, t)
        }
        if len(tickers) == 0 {
                return fmt.Errorf("no tickers returned from polygon")
        }

        var errs []error

        // 2) Mark as DELISTED any ticker NOT in today's list. Skip tickers that would violate unique constraints.
        if _, err := conn.DB.Exec(ctx, `
        UPDATE securities s
           SET maxDate = CURRENT_DATE
         WHERE maxDate IS NULL
           AND ticker NOT IN (`+placeholders(len(tickers))+`)
           AND NOT EXISTS (
                   SELECT 1 FROM securities s2
                    WHERE s2.ticker = s.ticker AND s2.maxDate = CURRENT_DATE
           )
    `, stringArgs(tickers)...); err != nil {
                errs = append(errs, fmt.Errorf("delist tickers: %w", err))
        }

        // 3) REACTIVATE any ticker IN today's list
        if _, err := conn.DB.Exec(ctx, `
        UPDATE securities
           SET maxDate = NULL
         WHERE maxDate IS NOT NULL
           AND ticker IN (`+placeholders(len(tickers))+`)
    `, stringArgs(tickers)...); err != nil {
                errs = append(errs, fmt.Errorf("reactivate tickers: %w", err))
        }

        if len(errs) > 0 {
                return errors.Join(errs...)
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
