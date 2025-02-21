package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// EDGARFiling represents a single SEC filing
type EDGARFiling struct {
	Type      string    `json:"type"` // e.g., "10-K", "8-K", "13F"
	Date      time.Time `json:"date"`
	URL       string    `json:"url"`
	Timestamp int64     `json:"timestamp"` // UTC timestamp in milliseconds
}

// Cache implementation with expiration
type edgarCache struct {
	sync.RWMutex
	cikMap map[string]string // ticker -> CIK mapping
}

var (
	filingCache = &edgarCache{
		cikMap: make(map[string]string),
	}
)

func fetchEdgarFilings(cik string) ([]EDGARFiling, error) {
	fmt.Printf("Fetching SEC filings for CIK: %s\n", cik)

	url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik)
	fmt.Printf("Making request to URL: %s\n", url)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SEC API returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

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
		return []EDGARFiling{}, nil
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

	filings := make([]EDGARFiling, 0, minLen)
	for i := 0; i < minLen; i++ {
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

		// Format the accession number by removing dashes
		accessionNumber := strings.Replace(recent.AccessionNumber[i], "-", "", -1)

		// Create URL that points to the human-readable HTML page
		htmlURL := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/%s",
			cik, accessionNumber, recent.PrimaryDocument[i])

		if recent.Form[i] != "4" { // Only append non-Form 4 filings
			filings = append(filings, EDGARFiling{
				Type:      recent.Form[i],
				Date:      date,
				URL:       htmlURL,
				Timestamp: utcMillis,
			})
		}
	}

	fmt.Printf("Found %d filings\n", len(filings))
	return filings, nil
}

// EdgarFilingOptions represents optional parameters for fetching EDGAR filings
type EdgarFilingOptions struct {
	From  *time.Time
	To    *time.Time
	Limit int
}

// GetRecentEdgarFilings retrieves SEC filings for a security with optional filters
func GetRecentEdgarFilings(conn *Conn, securityId int, opts *EdgarFilingOptions) ([]EDGARFiling, error) {
	// Get ticker for the security
	to := time.Now()
	if opts != nil && opts.To != nil {
		to = *opts.To
	}

	ticker, err := GetTicker(conn, securityId, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker: %v", err)
	}

	// Check CIK cache first
	filingCache.RLock()
	cik, exists := filingCache.cikMap[ticker]
	filingCache.RUnlock()

	if !exists {
		// Fetch CIK from SEC
		cik, err = fetchCIKFromSEC(ticker)
		if err != nil {
			return nil, fmt.Errorf("failed to get CIK for %s: %v", ticker, err)
		}

		// Cache the CIK
		filingCache.Lock()
		filingCache.cikMap[ticker] = cik
		filingCache.Unlock()
	}

	// Fetch filings directly (no caching)
	filings, err := fetchEdgarFilings(cik)
	if err != nil {
		return nil, err
	}

	// Filter filings based on options
	var filteredFilings []EDGARFiling
	for _, filing := range filings {
		filingTime := time.UnixMilli(filing.Timestamp)

		// Apply date filters if provided
		if opts != nil {
			if opts.From != nil && filingTime.Before(*opts.From) {
				continue
			}
			if opts.To != nil && filingTime.After(*opts.To) {
				continue
			}
		}

		filteredFilings = append(filteredFilings, filing)
	}

	fmt.Printf("Before sorting: %d filings\n", len(filteredFilings))

	// Sort filings by timestamp in descending order (newest first)
	sort.Slice(filteredFilings, func(i, j int) bool {
		return filteredFilings[i].Timestamp > filteredFilings[j].Timestamp
	})

	// Apply limit to get most recent filings
	if opts != nil && opts.Limit > 0 {
		fmt.Printf("Applying limit: %d (current length: %d)\n", opts.Limit, len(filteredFilings))
		if len(filteredFilings) > opts.Limit {
			filteredFilings = filteredFilings[:opts.Limit]
			fmt.Printf("After limit: %d filings\n", len(filteredFilings))
		}
	} else {
		fmt.Printf("No limit applied. Opts: %+v\n", opts)
	}

	// Re-sort in ascending order for display
	sort.Slice(filteredFilings, func(i, j int) bool {
		return filteredFilings[i].Timestamp < filteredFilings[j].Timestamp
	})

	fmt.Printf("Final count: %d filings\n", len(filteredFilings))
	return filteredFilings, nil
}

func fetchCIKFromSEC(ticker string) (string, error) {
	// SEC company lookup endpoint
	url := fmt.Sprintf("https://www.sec.gov/files/company_tickers.json")

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// SEC requires a User-Agent header
	req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SEC API returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// The SEC returns a map of numbered entries
	var result map[string]struct {
		CIK    int    `json:"cik_str"`
		Ticker string `json:"ticker"`
		Name   string `json:"title"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// Find matching ticker (case insensitive)
	upperTicker := strings.ToUpper(ticker)
	for _, company := range result {
		if strings.ToUpper(company.Ticker) == upperTicker {
			return fmt.Sprintf("%010d", company.CIK), nil
		}
	}

	return "", fmt.Errorf("no CIK found for ticker %s", ticker)
}
