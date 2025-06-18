package chart

import (
	"backend/internal/app/filings"
	"backend/internal/data"
	"backend/internal/data/edgar"
	"backend/internal/data/postgres"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

// GetChartEventsArgs represents a structure for handling GetChartEventsArgs data.
type GetChartEventsArgs struct {
	SecurityID        int   `json:"securityId"`
	From              int64 `json:"from"` // UTC Milliseconds
	To                int64 `json:"to"`   // UTC Milliseconds
	IncludeSECFilings bool  `json:"includeSECFilings,omitempty"`
}

// Event represents a structure for handling Event data.
type Event struct {
	ID        string `json:"id"`        // Unique ID for the event (e.g., filing_id, earnings_id)
	Timestamp int64  `json:"timestamp"` // UTC Milliseconds
	Type      string `json:"type"`
	Value     string `json:"value"` // JSON string with event details
}

// GetChartEvents performs operations related to GetChartEvents functionality.
func GetChartEvents(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetChartEventsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	// Fetch events using the modified helper function (ticker is determined within)
	events, err := fetchChartEventsInRange(conn, userID, args.SecurityID, args.From, args.To, args.IncludeSECFilings, false)
	if err != nil {
		// Propagate the error from the fetch function
		return nil, err
	}

	return events, nil
}

// fetchChartEventsInRange fetches splits, dividends, and optionally SEC filings for a given securityID and time range,
// handling potential ticker changes within the range.
// fromMs and toMs should be UTC milliseconds.
func fetchChartEventsInRange(conn *data.Conn, userID int, securityID int, fromMs, toMs int64, includeSECFilings bool, filterMinorFilings bool) ([]Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Query Security Records overlapping the time range
	fromTime := time.UnixMilli(fromMs).UTC()
	toTime := time.UnixMilli(toMs).UTC()

	query := `
		SELECT ticker, minDate, maxDate
		FROM securities
		WHERE securityid = $1
		  AND (maxDate IS NULL OR maxDate >= $2) -- Record ends after range starts
		  AND (minDate IS NULL OR minDate <= $3) -- Record starts before range ends
		ORDER BY minDate ASC NULLS LAST` // Process in chronological order

	rows, err := conn.DB.Query(ctx, query, securityID, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("error querying security records for securityId %d: %w", securityID, err)
	}
	defer rows.Close()

	type securityRecord struct {
		ticker  string
		minDate *time.Time
		maxDate *time.Time
	}
	var records []securityRecord
	tickersToFetch := make(map[string]struct{}) // Use a map as a set to store unique tickers

	for rows.Next() {
		var r securityRecord
		if err := rows.Scan(&r.ticker, &r.minDate, &r.maxDate); err != nil {
			rows.Close() // Ensure rows are closed on error
			return nil, fmt.Errorf("error scanning security record row: %w", err)
		}
		records = append(records, r)
		if _, exists := tickersToFetch[r.ticker]; !exists {
			tickersToFetch[r.ticker] = struct{}{}
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating security records: %w", err)
	}
	rows.Close() // Close explicitly after successful iteration

	if len(records) == 0 {
		////fmt.Printf("Warning: No security records found for securityId %d overlapping range %d-%d\n", securityID, fromMs, toMs)
		// Attempt to find *any* ticker for the security ID as a fallback for fetching events
		// This might be needed if the time range falls entirely outside known min/max dates
		fallbackTicker, fallbackErr := postgres.GetTicker(conn, securityID, fromTime) // Use 'fromTime' for fallback
		if fallbackErr != nil {
			////fmt.Printf("Warning: Could not find fallback ticker for securityId %d: %v\n", securityID, fallbackErr)
			return []Event{}, nil // Return empty if no records and no fallback
		}
		////fmt.Printf("Using fallback ticker %s for securityId %d\n", fallbackTicker, securityID)
		tickersToFetch[fallbackTicker] = struct{}{}
	}

	// Create a WaitGroup to synchronize concurrent fetching
	var wg sync.WaitGroup
	var mutex sync.Mutex // Protect shared slices
	var allSplits []models.Split
	var allDividends []models.Dividend
	var secFilings []edgar.Filing // Fetched separately

	var splitErr, dividendErr, secFilingErr error // Collect errors from goroutines

	// Load New York location for timezone conversion
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("error loading New York timezone: %w", err)
	}

	// 2. Fetch SEC Filings (Once, if requested) - Concurrently
	if includeSECFilings {
		wg.Add(1)
		go func() {
			defer wg.Done()
			options := filings.EdgarFilingOptions{
				Start:      fromMs,
				End:        toMs,
				SecurityID: securityID,
			}
			optionsJSON, err := json.Marshal(options)
			if err != nil {
				mutex.Lock()
				secFilingErr = fmt.Errorf("error marshalling EdgarFilingOptions: %w", err)
				mutex.Unlock()
				return
			}

			res, err := filings.GetStockEdgarFilings(conn, userID, optionsJSON)
			if err != nil {
				// Log the error but don't make it fatal for the whole function
				////fmt.Printf("Warning: error fetching SEC filings for securityId %d: %v\n", securityID, err)
				mutex.Lock()
				// Store the error if needed, but don't overwrite a critical marshalling error
				if secFilingErr == nil {
					secFilingErr = err
				}
				mutex.Unlock()
				return
			}

			filingsResult, ok := res.([]edgar.Filing)
			if !ok {
				mutex.Lock()
				secFilingErr = fmt.Errorf("unexpected type returned from GetStockEdgarFilings: %T", res)
				mutex.Unlock()
				return
			}
			mutex.Lock()
			secFilings = filingsResult
			mutex.Unlock()
		}()
	}

	// 3. Fetch Splits & Dividends (Per Unique Ticker) - Concurrently
	for ticker := range tickersToFetch {
		// Launch goroutines for each ticker
		wg.Add(2)               // Add 2 tasks for each ticker (splits + dividends)
		currentTicker := ticker // Capture loop variable for goroutine

		// Fetch Splits for this ticker
		go func(t string) {
			defer wg.Done()
			splits, err := getStockSplits(conn, t) // Fetch all splits for the ticker
			mutex.Lock()
			if err != nil {
				// Log error, potentially store it if we need ticker-specific errors
				////fmt.Printf("Warning: error fetching splits for ticker %s: %v\n", t, err)
				if splitErr == nil { // Keep the first error encountered
					splitErr = err
				}
			} else {
				allSplits = append(allSplits, splits...)
			}
			mutex.Unlock()
		}(currentTicker)

		// Fetch Dividends for this ticker
		go func(t string) {
			defer wg.Done()
			dividends, err := getStockDividends(conn, t) // Fetch all dividends for the ticker
			mutex.Lock()
			if err != nil {
				////fmt.Printf("Warning: error fetching dividends for ticker %s: %v\n", t, err)
				if dividendErr == nil {
					dividendErr = err
				}
			} else {
				allDividends = append(allDividends, dividends...)
			}
			mutex.Unlock()
		}(currentTicker)
	}

	// Wait for all fetching goroutines to complete
	wg.Wait()

	// --- Error Handling (Post-Wait) ---
	// Check for critical errors collected during fetch
	//if splitErr != nil {
	////fmt.Printf("Non-fatal error encountered during split fetching: %v\n", splitErr)
	// Decide if this should be fatal - currently logged as warning
	//}
	//if dividendErr != nil {
	////fmt.Printf("Non-fatal error encountered during dividend fetching: %v\n", dividendErr)
	//}
	//if secFilingErr != nil {
	////fmt.Printf("Non-fatal error encountered during SEC filing fetching: %v\n", secFilingErr)
	//}

	// 4. Filter and Format Events based on the original time range [fromMs, toMs]
	var finalEvents []Event
	var rawEvents []Event // Temporary slice to hold events before final filtering

	// Process Splits
	processedSplitKeys := make(map[string]struct{}) // Deduplicate splits by execution date + ratio
	for _, split := range allSplits {
		splitDate := time.Time(split.ExecutionDate)
		// Align to 4 AM ET for consistent timestamp placement
		splitDateET := time.Date(splitDate.Year(), splitDate.Month(), splitDate.Day(), 4, 0, 0, 0, nyLoc)
		utcTimestamp := splitDateET.UTC().UnixMilli()

		splitTo := int(math.Round(split.SplitTo))
		splitFrom := int(math.Round(split.SplitFrom))
		ratio := fmt.Sprintf("%d:%d", splitTo, splitFrom)
		dedupeKey := fmt.Sprintf("%d-%s", utcTimestamp, ratio)

		if _, exists := processedSplitKeys[dedupeKey]; exists {
			continue // Skip duplicate split entry
		}

		if utcTimestamp >= fromMs && utcTimestamp <= toMs {
			valueMap := map[string]interface{}{
				"ratio": ratio,
				"date":  splitDateET.Format("2006-01-02"), // Use the 4 AM ET date string
			}
			valueJSON, err := json.Marshal(valueMap)
			if err != nil {
				////fmt.Printf("Warning: error creating split value JSON: %v\n", err)
				continue // Skip this event
			}
			rawEvents = append(rawEvents, Event{
				ID:        fmt.Sprintf("split_%d-%s", utcTimestamp, ratio),
				Timestamp: utcTimestamp,
				Type:      "split",
				Value:     string(valueJSON),
			})
			processedSplitKeys[dedupeKey] = struct{}{}
		}
	}

	// Process Dividends
	processedDividendKeys := make(map[string]struct{}) // Deduplicate dividends by ex-date + amount
	for _, dividend := range allDividends {
		exDate, err := time.Parse("2006-01-02", dividend.ExDividendDate)
		if err != nil {
			////fmt.Printf("Warning: error parsing dividend ex-date %s: %v\n", dividend.ExDividendDate, err)
			continue
		}
		// Align to 4 AM ET on the Ex-Dividend Date
		exDateET := time.Date(exDate.Year(), exDate.Month(), exDate.Day(), 4, 0, 0, 0, nyLoc)
		utcTimestamp := exDateET.UTC().UnixMilli()
		amountStr := fmt.Sprintf("%.2f", dividend.CashAmount)
		dedupeKey := fmt.Sprintf("%d-%.2f", utcTimestamp, dividend.CashAmount)

		if _, exists := processedDividendKeys[dedupeKey]; exists {
			continue // Skip duplicate dividend entry
		}

		if utcTimestamp >= fromMs && utcTimestamp <= toMs {
			payDate := time.Time(dividend.PayDate)
			payDateString := payDate.Format("2006-01-02")
			valueMap := map[string]interface{}{
				"amount":  amountStr,
				"exDate":  dividend.ExDividendDate, // Keep original string format
				"payDate": payDateString,
			}
			valueJSON, err := json.Marshal(valueMap)
			if err != nil {
				////fmt.Printf("Warning: error creating dividend value JSON: %v\n", err)
				continue // Skip this event
			}
			rawEvents = append(rawEvents, Event{
				ID:        fmt.Sprintf("div_%s_%.2f", dividend.ExDividendDate, dividend.CashAmount),
				Timestamp: utcTimestamp,
				Type:      "dividend",
				Value:     string(valueJSON),
			})
			processedDividendKeys[dedupeKey] = struct{}{}
		}
	}

	// Process SEC Filings (if included)
	minorFilingTypes := map[string]struct{}{
		"3": {}, "4": {}, "5": {}, "13F": {}, "144": {}, "DEFA14A": {},
	}
	type FilingValue struct { // Helper struct to unmarshal filing Value JSON
		Type string `json:"type"`
		Date string `json:"date"`
		URL  string `json:"url"`
	}

	if includeSECFilings && len(secFilings) > 0 {
		processedFilingKeys := make(map[string]struct{}) // Deduplicate by timestamp + URL
		for _, filing := range secFilings {
			utcTimestamp := filing.Timestamp // Already UTC milliseconds
			dedupeKey := fmt.Sprintf("%d-%s", utcTimestamp, filing.URL)

			if _, exists := processedFilingKeys[dedupeKey]; exists {
				continue // Skip duplicate filing entry
			}

			// Filter by time range (should be redundant if GetStockEdgarFilings worked correctly)
			if utcTimestamp >= fromMs && utcTimestamp <= toMs {
				valueMap := map[string]interface{}{
					"type": filing.Type,
					"date": filing.Date.Format("2006-01-02"), // Already UTC date
					"url":  filing.URL,
				}
				valueJSON, err := json.Marshal(valueMap)
				if err != nil {
					////fmt.Printf("Warning: error creating filing value JSON: %v\n", err)
					continue // Skip this event
				}

				// Check for filtering *before* adding to rawEvents
				var filingInfo FilingValue
				shouldFilter := false
				if filterMinorFilings {
					if err := json.Unmarshal(valueJSON, &filingInfo); err == nil {
						if _, isMinor := minorFilingTypes[filingInfo.Type]; isMinor {
							shouldFilter = true
						}
					} // else {
					////fmt.Printf("Warning: could not unmarshal filing value for filtering: %v\n", err)
					//}
				}

				if !shouldFilter {
					rawEvents = append(rawEvents, Event{
						ID:        fmt.Sprintf("filing_%s", filing.URL),
						Timestamp: utcTimestamp,
						Type:      "sec_filing",
						Value:     string(valueJSON),
					})
					processedFilingKeys[dedupeKey] = struct{}{}
				}
			}
		}
	}

	// 5. Sort the combined events by timestamp
	sort.Slice(rawEvents, func(i, j int) bool {
		return rawEvents[i].Timestamp < rawEvents[j].Timestamp
	})

	// Assign the potentially filtered and sorted events to finalEvents
	finalEvents = rawEvents

	return finalEvents, nil
}

// getStockSplits fetches stock splits for a given ticker.
func getStockSplits(conn *data.Conn, ticker string) ([]models.Split, error) {
	return postgres.GetStockSplits(conn.Polygon, ticker)
}

// getStockDividends fetches stock dividends for a given ticker.
func getStockDividends(conn *data.Conn, ticker string) ([]models.Dividend, error) {
	// Set up parameters for the dividends API call
	params := models.ListDividendsParams{
		TickerEQ: &ticker,
	}.WithOrder(models.Order("desc")).WithLimit(100)

	// Execute the API call and get an iterator
	iter := conn.Polygon.ListDividends(context.Background(), params)

	// Collect all dividends
	var dividends []models.Dividend
	for iter.Next() {
		dividend := iter.Item()
		dividends = append(dividends, dividend)
	}

	// Check for errors during iteration
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("error fetching dividends for %s: %w", ticker, err)
	}

	return dividends, nil
}
