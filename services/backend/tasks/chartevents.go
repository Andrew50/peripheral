package tasks

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)
// GetChartEventsArgs represents a structure for handling GetChartEventsArgs data.
type GetChartEventsArgs struct {
	SecurityID        int   `json:"securityId"`
	From              int64 `json:"from"`
	To                int64 `json:"to"`
	IncludeSECFilings bool  `json:"includeSECFilings"`
}
// ChartEvent represents a structure for handling ChartEvent data.
type ChartEvent struct {
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"`
	Value     string `json:"value"`
}
// GetChartEvents performs operations related to GetChartEvents functionality.
func GetChartEvents(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetChartEventsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	// Convert from milliseconds to seconds for time.Unix
	ticker, err := utils.GetTicker(conn, args.SecurityID, time.Unix(args.From/1000, 0))
	if err != nil {
		return nil, fmt.Errorf("error fetching ticker for %d: %w", args.SecurityID, err)
	}

	// Create a WaitGroup to synchronize goroutines
	var wg sync.WaitGroup

	// Only add SEC filings to the waitgroup if requested
	if args.IncludeSECFilings {
		wg.Add(3) // Three tasks: splits, dividends, SEC filings
	} else {
		wg.Add(2) // Only two tasks: splits and dividends
	}

	// Create a mutex to protect the events slice during concurrent writes
	var mutex sync.Mutex
	var events []ChartEvent
	var splitErr, dividendErr, secFilingErr error

	// Load New York location for timezone conversion
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("error loading New York timezone: %w", err)
	}

	// Fetch splits in parallel
	go func() {
		defer wg.Done()
		splits, err := getStockSplits(conn, ticker)
		if err != nil {
			splitErr = fmt.Errorf("error fetching splits for %s: %w", ticker, err)
			return
		}

		// Process splits and add to events with mutex protection
		var splitEvents []ChartEvent
		for _, split := range splits {
			// Parse the execution date from the split
			splitDate := time.Time(split.ExecutionDate)

			// Set to 4 AM New York time on that date
			splitDateET := time.Date(
				splitDate.Year(),
				splitDate.Month(),
				splitDate.Day(),
				4, 0, 0, 0,
				nyLoc,
			)
			splitTo := int(math.Round(split.SplitTo))
			splitFrom := int(math.Round(split.SplitFrom))
			ratio := fmt.Sprintf("%d:%d", splitTo, splitFrom)

			// Create a structured value
			valueMap := map[string]interface{}{
				"ratio": ratio,
				"date":  splitDateET.Format("2006-01-02"),
			}

			// Convert the map to JSON
			valueJSON, err := json.Marshal(valueMap)
			if err != nil {
				splitErr = fmt.Errorf("error creating split value: %w", err)
				return
			}

			// Convert to UTC timestamp
			utcTimestamp := splitDateET.UTC().Unix() * 1000
			// Add to events if it's within the requested time range
			if utcTimestamp >= args.From && utcTimestamp <= args.To {
				splitEvents = append(splitEvents, ChartEvent{
					Timestamp: utcTimestamp,
					Type:      "split",
					Value:     string(valueJSON),
				})
			}
		}

		// Add split events to the main events slice
		mutex.Lock()
		events = append(events, splitEvents...)
		mutex.Unlock()
	}()

	// Fetch dividends in parallel
	go func() {
		defer wg.Done()
		dividends, err := getStockDividends(conn, ticker)
		if err != nil {
			dividendErr = fmt.Errorf("error fetching dividends for %s: %w", ticker, err)
			return
		}

		// Process dividends and add to events with mutex protection
		var dividendEvents []ChartEvent
		for _, dividend := range dividends {
			// Parse the ex-dividend date
			exDate, err := time.Parse("2006-01-02", dividend.ExDividendDate)
			if err != nil {
				dividendErr = fmt.Errorf("error parsing dividend date %s: %w", dividend.ExDividendDate, err)
				return
			}
			payDate := time.Time(dividend.PayDate)
			payDateString := payDate.Format("2006-01-02")
			exDateET := time.Date( //set to 4 am new york time on that date
				exDate.Year(),
				exDate.Month(),
				exDate.Day(),
				4, 0, 0, 0,
				nyLoc,
			)
			utcTimestamp := exDateET.UTC().Unix() * 1000

			// Create a structured value with multiple pieces of information
			valueMap := map[string]interface{}{
				"amount":  fmt.Sprintf("%.2f", dividend.CashAmount),
				"exDate":  dividend.ExDividendDate,
				"payDate": payDateString,
			}

			// Convert the map to JSON
			valueJSON, err := json.Marshal(valueMap)
			if err != nil {
				dividendErr = fmt.Errorf("error creating dividend value: %w", err)
				return
			}

			// Add to events if it's within the requested time range
			if utcTimestamp >= args.From && utcTimestamp <= args.To {
				dividendEvents = append(dividendEvents, ChartEvent{
					Timestamp: utcTimestamp,
					Type:      "dividend",
					Value:     string(valueJSON),
				})
			}
		}

		// Add dividend events to the main events slice
		mutex.Lock()
		events = append(events, dividendEvents...)
		mutex.Unlock()
	}()

	// Only fetch SEC filings if requested
	if args.IncludeSECFilings {
		go func() {
			defer wg.Done()
			from := time.Unix(args.From/1000, 0)
			to := time.Unix(args.To/1000, 0)

			filings, err := getStockEdgarFilings(conn, args.SecurityID, EdgarFilingOptions{
				From: &from,
				To:   &to,
			})
			if err != nil {
				// Log the error but don't fail the entire request
				secFilingErr = fmt.Errorf("Error fetching SEC filings for %s: %v", ticker, err)
				return
			}

			// Process SEC filings and add to events with mutex protection
			var filingEvents []ChartEvent
			// Process SEC filings
			for _, filing := range filings {
				// The timestamp is already in UTC milliseconds
				utcTimestamp := filing.Timestamp

				// Create a structured value with filing information
				valueMap := map[string]interface{}{
					"type": filing.Type,
					"date": filing.Date.Format("2006-01-02"),
					"url":  filing.URL,
				}

				// Convert the map to JSON
				valueJSON, err := json.Marshal(valueMap)
				if err != nil {
					fmt.Printf("Error creating filing value: %v\n", err)
					continue
				}

				// Add to events if it's within the requested time range
				if utcTimestamp >= args.From && utcTimestamp <= args.To {
					filingEvents = append(filingEvents, ChartEvent{
						Timestamp: utcTimestamp,
						Type:      "sec_filing",
						Value:     string(valueJSON),
					})
				}
			}

			// Add filing events to the main events slice
			mutex.Lock()
			events = append(events, filingEvents...)
			mutex.Unlock()
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check for errors (we'll report the first one we find)
	if splitErr != nil {
		return nil, splitErr
	}
	if dividendErr != nil {
		return nil, dividendErr
	}
	if secFilingErr != nil {
		// Log the error but don't fail the request
		fmt.Println(secFilingErr)
	}

	// Sort events by timestamp in ascending order
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp < events[j].Timestamp
	})

	return events, nil
}

func getStockSplits(conn *utils.Conn, ticker string) ([]models.Split, error) {
	// Set up parameters for the splits API call
	params := models.ListSplitsParams{
		TickerEQ: &ticker,
	}.WithOrder(models.Order("desc")).WithLimit(10)

	// Execute the API call and get an iterator
	iter := conn.Polygon.ListSplits(context.Background(), params)

	// Collect all splits
	var splits []models.Split
	for iter.Next() {
		split := iter.Item()
		splits = append(splits, split)
	}
	// Check for errors during iteration
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("error fetching splits for %s: %w", ticker, err)
	}

	return splits, nil
}

func getStockDividends(conn *utils.Conn, ticker string) ([]models.Dividend, error) {
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

// EdgarFilingOptions represents optional parameters for fetching EDGAR filings
type EdgarFilingOptions struct {
	From *time.Time
	To   *time.Time
}

// getStockEdgarFilings retrieves SEC filings for a security with optional filters
func getStockEdgarFilings(conn *utils.Conn, securityId int, opts EdgarFilingOptions) ([]utils.EDGARFiling, error) {
	// Get ticker for the security
	to := time.Now()
	if opts.To != nil {
		to = *opts.To
	}

	ticker, err := utils.GetTicker(conn, securityId, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker: %v", err)
	}

	// Fetch CIK from SEC
	cik, err := utils.GetCIKFromTicker(conn, ticker, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get CIK for %s: %v", ticker, err)
	}
	cikStr := fmt.Sprintf("%d", cik)
	filings, err := fetchEdgarFilings(cikStr)
	if err != nil {
		return nil, err
	}

	// Filter filings based on options
	var filteredFilings []utils.EDGARFiling
	for _, filing := range filings {
		filingTime := time.UnixMilli(filing.Timestamp)

		// Apply date filters if provided
		if opts.From != nil {
			if opts.From != nil && filingTime.Before(*opts.From) {
				continue
			}
			if opts.To != nil && filingTime.After(*opts.To) {
				continue
			}
		}

		filteredFilings = append(filteredFilings, filing)
	}

	// Sort filings by timestamp in ascending order (oldest first)
	sort.Slice(filteredFilings, func(i, j int) bool {
		return filteredFilings[i].Timestamp < filteredFilings[j].Timestamp
	})

	return filteredFilings, nil
}

// fetchEdgarFilings fetches filings for a specific CIK
func fetchEdgarFilings(cik string) ([]utils.EDGARFiling, error) {

	// Format CIK with leading zeros to make it 10 digits long
	paddedCik := cik
	if len(cik) < 10 {
		paddedCik = fmt.Sprintf("%010s", cik)
	}

	url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", paddedCik)
	fmt.Printf("Making SEC API request to: %s\n", url)

	// Create HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make the request with retries for rate limiting
	var resp *http.Response
	var err error
	maxRetries := 5
	retryDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		// SEC requires a User-Agent header
		req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")

		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}

		// Check for rate limiting (429)
		if resp.StatusCode == 429 {
			resp.Body.Close()

			// Exponential backoff
			waitTime := retryDelay * time.Duration(1<<attempt)
			time.Sleep(waitTime)
			continue
		}

		// Check for other non-success status codes
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("SEC API returned status %d: %s", resp.StatusCode, string(body[:100])) // Show first 100 chars
		}

		// If we get here, we have a successful response
		break
	}

	// Check if all retries failed
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("SEC API rate limit exceeded after %d retries", maxRetries)
	}

	defer resp.Body.Close()

	// Check content type to ensure we're getting JSON
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		body, _ := io.ReadAll(resp.Body)
		bodyPreview := string(body)
		if len(bodyPreview) > 100 {
			bodyPreview = bodyPreview[:100] + "..."
		}
		return nil, fmt.Errorf("unexpected content type: %s, response: %s", contentType, bodyPreview)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseEdgarFilingsResponse(body, cik)
}

// parseEdgarFilingsResponse parses the JSON response from SEC EDGAR API
func parseEdgarFilingsResponse(body []byte, cik string) ([]utils.EDGARFiling, error) {
	var result struct {
		Filings struct {
			Recent struct {
				AccessionNumber []string `json:"accessionNumber"`
				FilingDate      []string `json:"filingDate"`
				Form            []string `json:"form"`
				PrimaryDocument []string `json:"primaryDocument"`
				FilingTime      []string `json:"acceptanceDateTime"`
			} `json:"recent"`
		} `json:"filings"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SEC response: %v", err)
	}

	recent := result.Filings.Recent
	if recent.AccessionNumber == nil || recent.FilingDate == nil || recent.Form == nil || recent.PrimaryDocument == nil {
		return []utils.EDGARFiling{}, nil
	}

	minLen := len(recent.AccessionNumber)
	if len(recent.FilingDate) < minLen {
		minLen = len(recent.FilingDate)
	}
	if len(recent.Form) < minLen {
		minLen = len(recent.Form)
	}
	if len(recent.PrimaryDocument) < minLen {
		minLen = len(recent.PrimaryDocument)
	}

	// Create a map to track seen accession numbers to avoid duplicates
	seen := make(map[string]bool)
	filings := make([]utils.EDGARFiling, 0, minLen)

	for i := 0; i < minLen; i++ {
		// Skip Form 4 filings and duplicates
		if recent.Form[i] == "4" {
			continue
		}

		// Check for duplicates using accession number
		accessionNumber := strings.Replace(recent.AccessionNumber[i], "-", "", -1)
		if seen[accessionNumber] {
			continue
		}
		seen[accessionNumber] = true

		date, err := time.Parse("2006-01-02", recent.FilingDate[i])
		if err != nil {
			continue
		}

		// Parse the full timestamp
		var timestampTime time.Time
		if len(recent.FilingTime) > i {
			timestampTime, err = time.Parse("2006-01-02T15:04:05.000Z", recent.FilingTime[i])
			if err == nil {
				// Convert to EST to check market hours
				est, err := time.LoadLocation("America/New_York")
				if err == nil {
					estTime := timestampTime.In(est)

					// Check if outside market hours (4 AM to 8 PM EST)
					hour := estTime.Hour()
					if hour < 4 || hour >= 20 {
						// Set to midnight of the filing date
						timestampTime = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
					}
				}
			}
		}

		// If timestamp parsing failed, default to midnight of filing date in UTC
		if timestampTime.IsZero() {
			timestampTime = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		}

		// Convert to UTC milliseconds
		utcMillis := timestampTime.UTC().UnixMilli()

		// Create URL that points to the human-readable HTML page
		htmlURL := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/%s",
			cik, accessionNumber, recent.PrimaryDocument[i])

		filings = append(filings, utils.EDGARFiling{
			Type:      recent.Form[i],
			Date:      date,
			URL:       htmlURL,
			Timestamp: utcMillis,
		})
	}

	return filings, nil
}
