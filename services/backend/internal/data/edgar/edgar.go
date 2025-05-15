package edgar

import (
	"backend/internal/data/postgres"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"backend/internal/data"

	"golang.org/x/net/html/charset"
)

// Filing represents a single SEC filing
type Filing struct {
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

// FetchLatestEdgarFilings fetches the latest SEC filings from the RSS feed
func FetchLatestEdgarFilings(conn *data.Conn) ([]GlobalEDGARFiling, error) {
	var allFilings []GlobalEDGARFiling
	maxResults := 1500 // Maximum number of filings to fetch
	perPage := 100     // Number of results per API request

	for page := 1; len(allFilings) < maxResults; page++ {
		filings, err := fetchEdgarFilingsPage(conn, page, perPage)
		if err != nil {
			// Return what we've fetched so far along with the error
			return allFilings, fmt.Errorf("error fetching page %d: %w", page, err)
		}

		if len(filings) == 0 {
			// No more results
			break
		}

		allFilings = append(allFilings, filings...)

		// If we got fewer results than requested, we've reached the end
		if len(filings) < perPage {
			break
		}

		// Add a delay between requests to avoid rate limiting
		time.Sleep(300 * time.Millisecond)
	}

	return allFilings, nil
}

// fetchEdgarFilingsTickerPage fetches a single page of SEC filings with pagination
// nolint:unused
//
//lint:ignore U1000 kept for future SEC filing pagination
func fetchEdgarFilingsTickerPage(cik string, _ int, _ int) ([]Filing, error) {
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
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("error closing response body: %v", err)
			}

			// Exponential backoff
			waitTime := retryDelay * time.Duration(1<<attempt)
			////fmt.Printf("Rate limited by SEC API (429). Retrying in %v...\n", waitTime)
			time.Sleep(waitTime)
			continue
		}

		// If we get here, we have a non-429 response
		break
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
		return []Filing{}, nil
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

	filings := make([]Filing, 0, minLen)
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
			filings = append(filings, Filing{
				Type:      recent.Form[i],
				Date:      date,
				URL:       htmlURL,
				Timestamp: utcMillis,
			})
		}
	}

	return filings, nil
}

func fetchEdgarFilingsPage(conn *data.Conn, page int, perPage int) ([]GlobalEDGARFiling, error) {
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
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("error closing response body: %v", err)
			}
			waitTime := retryDelay * time.Duration(1<<attempt)
			////fmt.Printf("Rate limited by SEC API (429). Retrying in %v (page %d)...\n", waitTime, page)
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

	return parseEdgarXMLFeed(conn, body)
}

// parseEdgarXMLFeed parses the SEC EDGAR Atom XML feed
func parseEdgarXMLFeed(conn *data.Conn, body []byte) ([]GlobalEDGARFiling, error) {
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
			////fmt.Printf("Error parsing time %s: %v\n", entry.Updated, err)
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

// FilingOptions represents options for fetching EDGAR filings
type FilingOptions struct {
	Start      int64   `json:"start,omitempty"`
	End        int64   `json:"end,omitempty"`
	SecurityID int     `json:"securityId"`
	Ticker     *string `json:"ticker,omitempty"`
	CIK        *string `json:"cik,omitempty"`
	Form       *string `json:"form,omitempty"` // e.g. "10-K", "8-K"
}

// GetStockEdgarFilings retrieves SEC filings for a security with optional filters
func GetStockEdgarFilings(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args FilingOptions
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Get ticker for the security
	start := time.UnixMilli(args.Start)
	end := time.UnixMilli(args.End)

	ticker, err := postgres.GetTicker(conn, args.SecurityID, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker: %v", err)
	}

	// Fetch CIK from SEC
	cik, err := postgres.GetCIKFromTicker(conn, ticker, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get CIK for %s: %v", ticker, err)
	}
	cikStr := fmt.Sprintf("%d", cik)
	filings, err := fetchEdgarFilings(cikStr)
	if err != nil {
		return nil, err
	}

	// Filter filings based on options
	var filteredFilings []Filing
	for _, filing := range filings {
		filingTime := time.UnixMilli(filing.Timestamp)

		if filingTime.Before(start) || filingTime.After(end) {
			continue
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
func fetchEdgarFilings(cik string) ([]Filing, error) {

	// Format CIK with leading zeros to make it 10 digits long
	paddedCik := cik
	if len(cik) < 10 {
		paddedCik = fmt.Sprintf("%010s", cik)
	}

	url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", paddedCik)

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
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("error closing response body: %v", err)
			}

			// Exponential backoff
			waitTime := retryDelay * time.Duration(1<<attempt)
			time.Sleep(waitTime)
			continue
		}

		// Check for rate limiting (429)
		if resp.StatusCode == 429 {
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("error closing response body: %v", err)
			}

			// Exponential backoff
			waitTime := retryDelay * time.Duration(1<<attempt)
			time.Sleep(waitTime)
			continue
		}
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
func parseEdgarFilingsResponse(body []byte, cik string) ([]Filing, error) {
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
		return []Filing{}, nil
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
	filings := make([]Filing, 0, minLen)

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

		filings = append(filings, Filing{
			Type:      recent.Form[i],
			Date:      date,
			URL:       htmlURL,
			Timestamp: utcMillis,
		})
	}

	return filings, nil
}

// GetEarningsTextArgs represents arguments for retrieving text from the latest 10-K or 10-Q SEC filing
type GetEarningsTextArgs struct {
	SecurityID int    `json:"securityId"`
	Quarter    string `json:"quarter,omitempty"` // Optional: Q1, Q2, Q3, Q4, K (for 10-K)
	Year       int    `json:"year,omitempty"`    // Optional: specific year to look for
}

// EarningsTextResponse represents the structure returned by GetEarningsText
type EarningsTextResponse struct {
	Type      string `json:"type"`      // 10-K or 10-Q
	URL       string `json:"url"`       // URL of the filing
	Date      string `json:"date"`      // Date of the filing
	Text      string `json:"text"`      // Extracted text content
	Timestamp int64  `json:"timestamp"` // Timestamp of the filing
	Quarter   string `json:"quarter"`   // Quarter of the filing (Q1, Q2, Q3, Q4, or "Annual" for 10-K)
	Year      int    `json:"year"`      // Year of the filing
}

// getFilingQuarter extracts the quarter and year from a filing
func getFilingQuarter(filing Filing) (string, int) {
	// Get year from the filing date
	year := filing.Date.Year()

	// For 10-K, return "Annual" as the quarter
	if filing.Type == "10-K" {
		return "Q4", year // Treat 10-K as Q4 filing
	}

	// For 10-Q, determine which quarter based on the month
	month := filing.Date.Month()
	var quarter string

	// SEC filing deadlines typically mean Q1 is filed in April/May, Q2 in July/August, Q3 in October/November
	// This is an approximation - we're determining the quarter based on when it was filed
	switch {
	case month >= 1 && month <= 4:
		quarter = "Q4" // Previous year's Q4 might be filed in early months
		if month <= 2 {
			year-- // Adjust year for Q4 filings in January/February
		}
	case month >= 4 && month <= 6:
		quarter = "Q1"
	case month >= 7 && month <= 9:
		quarter = "Q2"
	case month >= 10:
		quarter = "Q3"
	}

	return quarter, year
}

// GetEarningsText fetches the latest 10-K or 10-Q filing for a security and extracts the text content
func GetEarningsText(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetEarningsTextArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	// Get the current time
	now := time.Now()

	// Get ticker for the security
	ticker, err := postgres.GetTicker(conn, args.SecurityID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker: %v", err)
	}

	// Get CIK from ticker
	cik, err := postgres.GetCIKFromTicker(conn, ticker, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get CIK for %s: %v", ticker, err)
	}
	cikStr := fmt.Sprintf("%d", cik)

	// Fetch EDGAR filings
	filings, err := fetchEdgarFilings(cikStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch EDGAR filings: %v", err)
	}

	// Filter filings to find the specified quarter/year or the latest
	var targetFiling *Filing

	// If quarter is specified, filter by quarter and year
	if args.Quarter != "" {
		// Normalize the quarter input
		targetQuarter := strings.ToUpper(args.Quarter)
		// Convert "K" to "Q4" for consistency in filtering
		if targetQuarter == "K" || targetQuarter == "ANNUAL" {
			targetQuarter = "Q4"
		}

		// Filter filings based on quarter and year
		var matchingFilings []Filing
		for _, filing := range filings {
			// Skip non 10-K/10-Q filings
			if filing.Type != "10-K" && filing.Type != "10-Q" {
				continue
			}

			quarter, year := getFilingQuarter(filing)

			// If year is specified, must match
			if args.Year > 0 && year != args.Year {
				continue
			}

			// Check quarter match (either filing type can match the requested quarter)
			if targetQuarter == quarter {
				matchingFilings = append(matchingFilings, filing)
			}
		}

		// Find the latest matching filing
		for i, filing := range matchingFilings {
			if i == 0 || filing.Timestamp > targetFiling.Timestamp {
				targetFiling = &matchingFilings[i]
			}
		}

		// If no matching filing found
		if targetFiling == nil {
			if args.Year > 0 {
				return nil, fmt.Errorf("no filing found for %s in quarter %s, year %d", ticker, args.Quarter, args.Year)
			}
			return nil, fmt.Errorf("no filing found for %s in quarter %s", ticker, args.Quarter)
		}
	} else {
		// No quarter specified, find the latest 10-K or 10-Q
		for _, filing := range filings {
			// Check if this is a 10-K or 10-Q filing
			if filing.Type == "10-K" || filing.Type == "10-Q" {
				if targetFiling == nil || filing.Timestamp > targetFiling.Timestamp {
					targetFiling = &filing
				}
			}
		}

		if targetFiling == nil {
			return nil, fmt.Errorf("no 10-K or 10-Q filings found for %s", ticker)
		}
	}

	// Fetch the text content of the filing
	text, err := fetchFilingText(targetFiling.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch filing text: %v", err)
	}

	// Determine quarter and year for the response
	quarter, year := getFilingQuarter(*targetFiling)

	// For response, return "Annual" for 10-K reports to make it more user-friendly
	displayQuarter := quarter
	if targetFiling.Type == "10-K" {
		displayQuarter = "Annual"
	}

	// Create response
	response := EarningsTextResponse{
		Type:      targetFiling.Type,
		URL:       targetFiling.URL,
		Date:      targetFiling.Date.Format("2006-01-02"),
		Text:      text,
		Timestamp: targetFiling.Timestamp,
		Quarter:   displayQuarter,
		Year:      year,
	}

	return response, nil
}

type GetFilingTextArgs struct {
	URL string `json:"url"`
}

type GetFilingTextResponse struct {
	Text string `json:"text"`
}

// GetFilingText performs operations related to GetFilingText functionality.
func GetFilingText(_ *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetFilingTextArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	text, err := fetchFilingText(args.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch filing text: %v", err)
	}

	return GetFilingTextResponse{Text: text}, nil
}

// fetchFilingText fetches the text content of an SEC filing from its URL
func fetchFilingText(url string) (string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// SEC requires a User-Agent header
	req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")

	// Send request with retries for rate limiting
	var resp *http.Response
	maxRetries := 3
	retryDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = client.Do(req)
		if err != nil {
			return "", err
		}

		// Check for rate limiting (429)
		if resp.StatusCode == 429 {
			if err := resp.Body.Close(); err != nil {
				return "", fmt.Errorf("error closing response body: %v", err)
			}

			// Exponential backoff
			waitTime := retryDelay * time.Duration(1<<attempt)
			time.Sleep(waitTime)
			continue
		}

		// Check for other non-success status codes
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if err := resp.Body.Close(); err != nil {
				// Log the error but return the primary error
				return "", fmt.Errorf("SEC API returned status %d and error closing response: %v", resp.StatusCode, err)
			}
			return "", fmt.Errorf("SEC API returned status %d: %s", resp.StatusCode, string(body[:100])) // Show first 100 chars
		}

		// Success
		break
	}

	// Check if all retries failed
	if resp.StatusCode == 429 {
		return "", fmt.Errorf("SEC API rate limit exceeded after %d retries", maxRetries)
	}

	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Extract text content from HTML
	text := extractTextFromHTML(string(body))
	return text, nil
}

// extractTextFromHTML extracts readable text content from HTML
func extractTextFromHTML(html string) string {
	// Remove HTML tags
	textContent := removeHTMLTags(html)

	// Remove extra whitespace and normalize
	textContent = normalizeWhitespace(textContent)

	return textContent
}

// removeHTMLTags removes HTML tags from a string
func removeHTMLTags(html string) string {
	// Define a simple regex to remove HTML tags
	tagRegex := regexp.MustCompile("<[^>]*>")
	text := tagRegex.ReplaceAllString(html, " ")

	// Also remove JavaScript and CSS
	scriptRegex := regexp.MustCompile("(?s)<script.*?</script>")
	text = scriptRegex.ReplaceAllString(text, "")

	styleRegex := regexp.MustCompile("(?s)<style.*?</style>")
	text = styleRegex.ReplaceAllString(text, "")

	return text
}

// normalizeWhitespace replaces sequences of whitespace characters with a single space
func normalizeWhitespace(text string) string {
	// Replace newlines, tabs, multiple spaces with a single space
	wsRegex := regexp.MustCompile(`\s+`)
	text = wsRegex.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}
