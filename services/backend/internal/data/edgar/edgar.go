package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
    "backend/internal/data"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html/charset"
)

// EDGARFiling represents a single SEC filing
type EDGARFiling struct {
	Type      string    `json:"type"` // e.g., "10-K", "8-K", "13F"
	Date      time.Time `json:"date"`
	URL       string    `json:"url"`
	Timestamp int64     `json:"timestamp"` // UTC timestamp in milliseconds
}

// GlobalEDGARFiling represents a SEC filing in the EDGAR database
type GlobalEDGARFiling struct {
	CompanyName     string `json:"company_name"`
	Type            string `json:"type"`
	Date            string `json:"date"`
	URL             string `json:"url"`
	AccessionNumber string `json:"accession_number"`
	Description     string `json:"description,omitempty"`
	Ticker          string `json:"ticker"`
	Timestamp       int64  `json:"timestamp"` // UTC timestamp in milliseconds
}

// AtomFeed represents the root element of the SEC EDGAR Atom feed
type AtomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Entries []AtomEntry `xml:"entry"`
}

// AtomEntry represents a single filing entry in the EDGAR Atom feed
type AtomEntry struct {
	Title    string       `xml:"title"`
	Link     AtomLink     `xml:"link"`
	Summary  string       `xml:"summary"`
	Updated  string       `xml:"updated"`
	Category AtomCategory `xml:"category"`
	ID       string       `xml:"id"`
}

// AtomLink represents a link element in an Atom entry
type AtomLink struct {
	Href string `xml:"href,attr"`
}

// AtomCategory represents the category element in an Atom entry
type AtomCategory struct {
	Term  string `xml:"term,attr"`
	Label string `xml:"label,attr"`
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

// fetchTickerFromCIK retrieves the ticker symbol for a given CIK
func fetchTickerFromCIK(conn *data.Conn, cik string) string {
	// Method 1: Reverse lookup in the cache
	filingCache.RLock()
	for ticker, cachedCIK := range filingCache.cikMap {
		if cachedCIK == cik {
			filingCache.RUnlock()
			return ticker
		}
	}
	filingCache.RUnlock()

	// Method 2: Query our securities table
	if conn != nil {
		// Remove leading zeros from CIK for numeric comparison
		trimmedCIK := strings.TrimLeft(cik, "0")

		var ticker string
		err := conn.DB.QueryRow(context.Background(),
			`SELECT ticker FROM securities 
			 WHERE cik = $1 AND maxDate IS NULL 
			 LIMIT 1`,
			trimmedCIK).Scan(&ticker)

		if err == nil && ticker != "" {
			// Add to cache for future lookups
			filingCache.Lock()
			filingCache.cikMap[ticker] = cik
			filingCache.Unlock()
			return ticker
		}
	}

	return ""
}


// fetchEdgarFilings fetches SEC filings for a given CIK
// nolint:unused
//
//lint:ignore U1000 kept for future SEC filing retrieval
func fetchEdgarFilings(cik string) ([]EDGARFiling, error) {
	fmt.Printf("Fetching SEC filings for CIK: %s\n", cik)

	var allFilings []EDGARFiling
	maxResults := 1500 // Maximum number of filings to fetch
	perPage := 100     // Number of results per API call

	// For pagination
	for start := 0; start < maxResults; start += perPage {
		// Check if we already have enough filings
		if len(allFilings) >= maxResults {
			break
		}

		filings, err := fetchEdgarFilingsTickerPage(cik, start, perPage)
		if err != nil {
			return allFilings, err // Return what we've got so far with the error
		}

		// If we got fewer results than requested, we've reached the end
		if len(filings) < perPage {
			allFilings = append(allFilings, filings...)
			break
		}

		allFilings = append(allFilings, filings...)

		// Add a small delay between requests to avoid rate limiting
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Printf("Found %d filings\n", len(allFilings))
	return allFilings, nil
}

// fetchEdgarFilingsTickerPage fetches a single page of SEC filings with pagination
// nolint:unused
//
//lint:ignore U1000 kept for future SEC filing pagination
func fetchEdgarFilingsTickerPage(cik string, start int, count int) ([]EDGARFiling, error) {
	url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik)

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
			fmt.Printf("Rate limited by SEC API (429). Retrying in %v...\n", waitTime)
			time.Sleep(waitTime)
			continue
		}

		// If we get here, we have a non-429 response
		break
	}

	// Check if all retries failed
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("SEC API rate limit exceeded after %d retries", maxRetries)
	}

	defer resp.Body.Close()

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

	return filings, nil
}

func fetchEdgarFilingsPage(page int, perPage int) ([]GlobalEDGARFiling, error) {
	// Assuming the SEC API supports a page parameter
	url := fmt.Sprintf("https://www.sec.gov/cgi-bin/browse-edgar?action=getcurrent&owner=include&count=%d&start=%d&output=atom",
		perPage, (page-1)*perPage)

	// Create a client with reasonable timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Implement retry logic for rate limiting
	var resp *http.Response
	var err error
	maxRetries := 5
	retryDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Add required headers
		req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")
		req.Header.Set("Accept", "application/xml, application/atom+xml, text/xml, */*;q=0.8")

		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %v", err)
		}

		// Check for rate limiting
		if resp.StatusCode == 429 {
			resp.Body.Close()
			waitTime := retryDelay * time.Duration(1<<attempt)
			fmt.Printf("Rate limited by SEC API (429). Retrying in %v (page %d)...\n", waitTime, page)
			time.Sleep(waitTime)
			continue
		}

		// Non-429 status, break out of retry loop
		break
	}

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("SEC API rate limit exceeded after %d retries", maxRetries)
	}

	defer resp.Body.Close()

	// Process response body as before
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return parseEdgarXMLFeed(body)
}

// parseEdgarXMLFeed parses the SEC EDGAR Atom XML feed
func parseEdgarXMLFeed(body []byte) ([]GlobalEDGARFiling, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.CharsetReader = charset.NewReaderLabel

	var feed AtomFeed
	if err := decoder.Decode(&feed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XML: %v", err)
	}

	var filings []GlobalEDGARFiling
	for _, entry := range feed.Entries {
		// Extract date and time from the updated field
		updatedTime, err := time.Parse(time.RFC3339, entry.Updated)
		if err != nil {
			fmt.Printf("Error parsing time %s: %v\n", entry.Updated, err)
			// Use current time as fallback instead of skipping
			updatedTime = time.Now()
		}

		// Convert to UTC and get timestamp in milliseconds
		utcTime := updatedTime.UTC()
		utcTimestampMs := utcTime.UnixNano() / int64(time.Millisecond)

		// Format date as YYYY-MM-DD for the date field
		date := utcTime.Format("2006-01-02")
		cik := extractCIK(entry)

		// Try to convert CIK to ticker
		ticker := ""
		if cik != "" {
			ticker = fetchTickerFromCIK(conn, cik)
		}

		// Extract company name from title (Format: "FORM_TYPE - COMPANY_NAME (ID) (Role)")
		companyName := parseCompanyName(entry.Title)

		// Extract accession number from the ID field
		accessionNumber := ""
		if idParts := strings.Split(entry.ID, "="); len(idParts) > 1 {
			accessionNumber = idParts[1]
		}
		// Create filing object
		filing := GlobalEDGARFiling{
			CompanyName:     companyName,
			Type:            entry.Category.Term,
			Date:            date,
			URL:             entry.Link.Href,
			AccessionNumber: accessionNumber,
			Description:     entry.Summary,
			Timestamp:       utcTimestampMs,
			Ticker:          ticker,
		}

		if filing.Type == "4" || ticker == "" {
			continue
		}

		filings = append(filings, filing)
	}

	return filings, nil
}

// extractCIK extracts the CIK from an EDGAR Atom feed entry
func extractCIK(entry AtomEntry) string {
	// Extract CIK from title format: "FormType - CompanyName (CIK) (Role)"
	titleRegex := regexp.MustCompile(`\((\d+)\)`)
	matches := titleRegex.FindStringSubmatch(entry.Title)
	if len(matches) > 1 {
		return matches[1]
	}

	// Method 2: Try to extract from ID using regex
	cikRegex := regexp.MustCompile(`CIK=(\d+)`)
	matches = cikRegex.FindStringSubmatch(entry.ID)
	if len(matches) > 1 {
		return matches[1]
	}

	// Method 3: Try to extract from summary
	cikContentRegex := regexp.MustCompile(`CIK: (\d+)`)
	matches = cikContentRegex.FindStringSubmatch(entry.Summary)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// Helper functions to extract specific pieces from the Atom entries
func parseCompanyName(title string) string {
	// Extract company name from title like "FORM_TYPE - COMPANY_NAME (ID) (Role)"
	parts := strings.Split(title, " - ")
	if len(parts) < 2 {
		return title
	}
	companyWithID := parts[1]
	companyParts := strings.Split(companyWithID, " (")
	if len(companyParts) < 1 {
		return companyWithID
	}
	return companyParts[0]
}

// parseFilingDate parses a filing date from the SEC format
// nolint:unused
//
//lint:ignore U1000 kept for future date parsing
func parseFilingDate(updated string) string {
	// Convert the ISO 8601 timestamp to the desired format
	t, err := time.Parse(time.RFC3339, updated)
	if err != nil {
		return updated
	}
	return t.Format("2006-01-02")
}
