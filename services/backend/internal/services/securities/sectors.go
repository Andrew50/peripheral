// File: update_sectors.go
//
// Re-written to source sector / industry information from
// https://github.com/rreichel3/US-Stock-Symbols instead of Yahoo Finance.
// ----------------------------------------------------------------------

package securities

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
    "database/sql"

	"backend/internal/data"           // your conn.go
	"github.com/jackc/pgx/v4/pgxpool" // postgres
)


// ------------------------------------------------------------------- //
//  Types                                                              //
// ------------------------------------------------------------------- //

type security struct {
	Ticker          string
	CurrentSector   sql.NullString
	CurrentIndustry sql.NullString
}

// object layout in each *_full_ticker.json file
type listing struct {
	Symbol   string `json:"symbol"`   // e.g. "AAPL"
	Sector   string `json:"sector"`   // e.g. "Technology"
	Industry string `json:"industry"` // e.g. "Consumer Electronics"
}


// map[ticker] = (sector, industry)
type meta struct{ Sector, Industry string }

// ------------------------------------------------------------------- //
//  Public entry-point                                                 //
// ------------------------------------------------------------------- //

func UpdateSectors(ctx context.Context, c *data.Conn) error {
	// ---------------------------------------------------------------- //
	// 1. Load sector / industry map once                               //
	// ---------------------------------------------------------------- //
	m, err := buildMetaMap(ctx)
	if err != nil {
		return fmt.Errorf("load github data: %w", err)
	}

	// ---------------------------------------------------------------- //
	// 2. Pull securities that still need data                          //
	// ---------------------------------------------------------------- //
	rows, err := c.DB.Query(ctx, `
	    SELECT ticker, sector, industry
	      FROM securities
	     WHERE maxDate IS NULL`)
	if err != nil {
		return fmt.Errorf("query securities: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s security
		if err := rows.Scan(&s.Ticker, &s.CurrentSector, &s.CurrentIndustry); err != nil {
			return err
		}

		meta, ok := m[strings.ToUpper(s.Ticker)]
		if !ok { // ticker not in GitHub files
			continue
		}

		needsUpdate := (!s.CurrentSector.Valid || s.CurrentSector.String != meta.Sector) ||
			(!s.CurrentIndustry.Valid || s.CurrentIndustry.String != meta.Industry)

		if !needsUpdate {
			continue
		}

		if err := applyUpdate(ctx, c.DB, s.Ticker, meta); err != nil {
			// Log or handle failed update, but continue processing other securities
			// For now, we'll just return the error, stopping the process.
			// Depending on requirements, you might want to collect errors and continue.
			return fmt.Errorf("applyUpdate for %s: %w", s.Ticker, err)
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

// ------------------------------------------------------------------- //
//  Stage 1: build ticker → meta map                                   //
// ------------------------------------------------------------------- //

func buildMetaMap(ctx context.Context) (map[string]meta, error) {
	// each exchange has   <ex>/<ex>_full_ticker.json
	// see repo README: https://github.com/rreichel3/US-Stock-Symbols
	exchanges := []string{"nasdaq", "nyse", "amex"}
	base      := "https://raw.githubusercontent.com/rreichel3/US-Stock-Symbols/main/%s/%s_full_tickers.json"

	out := make(map[string]meta, 15_000) // rough upper bound

	for _, ex := range exchanges {
		url  := fmt.Sprintf(base, ex, ex)

		body, err := fetch(ctx, url)
		if err != nil {
			return nil, err
		}

		var lst []listing            // listing{Symbol, Sector, Industry}
		if err := json.Unmarshal(body, &lst); err != nil {
			return nil, fmt.Errorf("decode %s: %w", ex, err)
		}

		for _, l := range lst {
			if l.Sector == "" && l.Industry == "" {
				continue // ETFs & blanks
			}
			out[strings.ToUpper(l.Symbol)] = meta{l.Sector, l.Industry}
		}
	}
	return out, nil
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	// Outgoing headers
	req.Header.Set("User-Agent", "update_sectors/1.2 (+https://github.com/your-org)")
	req.Header.Set("Accept", "application/vnd.github.raw") // forces raw view

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// GitHub sometimes base-64s large JSON blobs
	if json.Valid(b) {
		return b, nil
	}
	if dec, err := base64.StdEncoding.DecodeString(string(b)); err == nil && json.Valid(dec) {
		return dec, nil
	}
	return nil, fmt.Errorf("unexpected payload from %s", url)
}

// ------------------------------------------------------------------- //
//  Stage 2: apply change                                              //
// ------------------------------------------------------------------- //

func applyUpdate(ctx context.Context, db *pgxpool.Pool, ticker string, m meta) error {
	_, err := db.Exec(ctx,
		`UPDATE securities
		    SET sector = $1,
		        industry = $2
		  WHERE ticker = $3`,
		m.Sector, m.Industry, ticker)
	return err
}

