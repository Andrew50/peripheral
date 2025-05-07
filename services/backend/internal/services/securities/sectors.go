// File: update_sectors.go
//
// A faithful Go rewrite of the original Python `update_sectors` script.
// – Keeps identical batching / worker‑count logic.
// – Randomised jitter + extra 200 ms pause to respect Yahoo’s informal rate‑limit.
// – Uses the unofficial “quoteSummary?modules=assetProfile” endpoint directly
//   instead of `yfinance`; no external Go Yahoo‑Finance wrapper required.
//
// Requirements
// ---------------------------------------------------------------------------
// • Go 1.21+
// • A `conn` package (conn.go) exposing:
//
//      type Conn struct {
//          DB *sql.DB
//      }
//
//   pointing at a PostgreSQL database that contains a `securities` table
//   with columns: ticker (text), sector (text), industry (text), maxDate (date).
//
// Usage
// ---------------------------------------------------------------------------
//    stats, err := UpdateSectors(context.Background(), myConn)
//    if err != nil { log.Fatal(err) }
//    fmt.Printf("%+v\n", stats)
// ---------------------------------------------------------------------------

package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"backend/internal/data" // adjust import path to where conn.go lives

	"github.com/jackc/pgx/v4/pgxpool"
)

// --------------------------- data types ---------------------------------- //

type security struct {
	Ticker           string
	CurrentSector    sql.NullString
	CurrentIndustry  sql.NullString
}

type statBlock struct {
	Total, Updated, Failed, Unchanged int
}

type result struct {
	Ticker, NewSector, NewIndustry string
	Err                            error
}

// --------------------------- public entry -------------------------------- //

func UpdateSectors(ctx context.Context, c *data.Conn) (statBlock, error) {
	stats := statBlock{}

	// ------------------------------------------------------------------ //
	// 1. Fetch distinct active tickers whose maxDate IS NULL              //
	// ------------------------------------------------------------------ //
	rows, err := c.DB.Query(ctx, `
		SELECT DISTINCT ticker, sector, industry
		FROM securities
		WHERE maxDate IS NULL`)
	if err != nil {
		return stats, fmt.Errorf("query securities: %w", err)
	}
	defer rows.Close()

	var all []security
	for rows.Next() {
		var s security
		if err := rows.Scan(&s.Ticker, &s.CurrentSector, &s.CurrentIndustry); err != nil {
			return stats, fmt.Errorf("scan row: %w", err)
		}
		all = append(all, s)
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	stats.Total = len(all)
	if stats.Total == 0 {
		log.Println("update_sectors: nothing to do")
		return stats, nil
	}

	// ------------------------------------------------------------------ //
	// 2. Optional batch‑size truncation (env: UPDATE_SECTORS_BATCH_SIZE) //
	// ------------------------------------------------------------------ //
	batch := envInt("UPDATE_SECTORS_BATCH_SIZE", 100)
	if len(all) > batch {
		log.Printf("Processing %d securities in batches of %d\n", len(all), batch)
		all = all[:batch]
		stats.Total = batch
	}

	// ------------------------------------------------------------------ //
	// 3. Worker‑pool configuration                                       //
	// ------------------------------------------------------------------ //
	// Reduce maximum concurrent workers to 2 (or even 1 for sequential processing)
	workerCount := min(
		2, // <-- Reduced from 4
		runtime.NumCPU(),
		max(1, len(all)/2),
	)

	log.Printf("Starting update_sectors with %d workers for %d securities\n",
		workerCount, len(all))

	jobs := make(chan security)
	results := make(chan result)

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go worker(ctx, &wg, jobs, results)
	}

	// Pump jobs
	go func() {
		for _, s := range all {
			jobs <- s
		}
		close(jobs)
	}()

	// Collect results
	done := make(chan struct{})
	go func() {
		for r := range results {
			if r.Err != nil {
				log.Printf("Failed to update %s: %v\n", r.Ticker, r.Err)
				stats.Failed++
				continue
			}
			curr := findCurrent(all, r.Ticker)
			if shouldUpdate(curr, r) {
				if err := applyUpdate(ctx, c.DB, r); err != nil {
					log.Printf("DB update error for %s: %v\n", r.Ticker, err)
					stats.Failed++
				} else {
					stats.Updated++
				}
			} else {
				stats.Unchanged++
			}
		}
		done <- struct{}{}
	}()

	// Wait for workers, then close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	<-done
	log.Printf("update_sectors completed: %+v\n", stats)
	return stats, nil
}

// --------------------------- worker logic -------------------------------- //

func worker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan security, out chan<- result) {
	defer wg.Done()

	for s := range jobs {
		out <- fetchTickerInfo(ctx, s)
	}
}

func fetchTickerInfo(ctx context.Context, s security) result {
	// Initial jitter delay before first request attempt
	// Increase base jitter delay, e.g., 500ms - 900ms
	jitter := time.Duration(rand.Intn(400)+500) * time.Millisecond // <-- Base increased from 100 to 500
	time.Sleep(jitter)

	sector, industry, err := queryYahoo(ctx, s.Ticker)
	// Only sleep after successful requests or non-retryable errors
	// This avoids unnecessary waits when the retry mechanism already includes backoff delays
	if err == nil || !strings.Contains(err.Error(), "retryable status") {
		// Increase sleep significantly, e.g., to 2 seconds (total average delay ~2.3s with jitter)
		time.Sleep(2000 * time.Millisecond) // <-- Increased from 800
	}

	if err != nil {
		// Return current values on error, mirroring Python behaviour
		if s.CurrentSector.Valid {
			sector = s.CurrentSector.String
		}
		if s.CurrentIndustry.Valid {
			industry = s.CurrentIndustry.String
		}
	}
	return result{
		Ticker:      s.Ticker,
		NewSector:   sector,
		NewIndustry: industry,
		Err:         err,
	}
}

// --------------------------- Yahoo Finance ------------------------------- //

func queryYahoo(ctx context.Context, ticker string) (sector, industry string, err error) {
	url := fmt.Sprintf("https://query2.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=assetProfile", ticker)
	
	// Retry configuration
	maxRetries := 3
	initialBackoff := 3 * time.Second
	
	var resp *http.Response
	var lastErr error
	
	// Retry loop with exponential backoff
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff duration with exponential increase and some jitter
			backoff := initialBackoff * time.Duration(1<<(attempt-1))
			jitterRange := int(backoff / 4)
			jitter := time.Duration(rand.Intn(jitterRange))
			waitTime := backoff + jitter
			
			log.Printf("Retrying %s (attempt %d/%d) after %v due to: %v", 
				ticker, attempt, maxRetries, waitTime, lastErr)
			
			// Wait before retrying
			select {
			case <-time.After(waitTime):
				// Continue with retry
			case <-ctx.Done():
				return "", "", fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			}
		}
		
		// Create a new request for each attempt
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("User-Agent", "Go-http-client/1.1")
		
		resp, lastErr = http.DefaultClient.Do(req)
		if lastErr != nil {
			continue // Network error, retry
		}
		
		// Check if we got a retryable status code (429 or 5xx)
		if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
			resp.Body.Close() // Important: close the body before retrying
			lastErr = fmt.Errorf("retryable status: %s", resp.Status)
			continue // Retryable status code, retry
		}
		
		// If we get here, we either have a successful response or a non-retryable error
		break
	}
	
	// Handle final error state
	if lastErr != nil {
		return "", "", lastErr
	}
	
	// Handle non-200 responses that weren't retried or still failed after retries
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("non‑200 status: %s", resp.Status)
	}

	var body struct {
		QuoteSummary struct {
			Result []struct {
				AssetProfile struct {
					Sector   string `json:"sector"`
					Industry string `json:"industry"`
				} `json:"assetProfile"`
			} `json:"result"`
			Error json.RawMessage `json:"error"`
		} `json:"quoteSummary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", err
	}
	if len(body.QuoteSummary.Result) == 0 {
		return "", "", errors.New("empty result set")
	}
	ap := body.QuoteSummary.Result[0].AssetProfile
	return ap.Sector, ap.Industry, nil
}

// --------------------------- DB helpers ---------------------------------- //

func shouldUpdate(curr security, res result) bool {
	sectorChanged := !curr.CurrentSector.Valid || curr.CurrentSector.String != res.NewSector
	industryChanged := !curr.CurrentIndustry.Valid || curr.CurrentIndustry.String != res.NewIndustry
	return sectorChanged || industryChanged
}

func applyUpdate(ctx context.Context, db *pgxpool.Pool, r result) error {
	_, err := db.Exec(ctx, `
		UPDATE securities
		SET sector = $1, industry = $2
		WHERE ticker = $3`, r.NewSector, r.NewIndustry, r.Ticker)
	return err
}

func findCurrent(list []security, ticker string) security {
	for _, s := range list {
		if s.Ticker == ticker {
			return s
		}
	}
	return security{Ticker: ticker} // should not happen
}

// --------------------------- utilities ----------------------------------- //

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil && i > 0 {
			return i
		}
	}
	return def
}

func min(a int, rest ...int) int {
	m := a
	for _, v := range rest {
		if v < m {
			m = v
		}
	}
	return m
}

func max(a int, rest ...int) int {
	m := a
	for _, v := range rest {
		if v > m {
			m = v
		}
	}
	return m
}

