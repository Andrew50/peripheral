package utils

import (
	"bytes"
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

	"golang.org/x/net/html/charset"
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

// extractTicker attempts to extract a ticker symbol from the company name
func extractTicker(companyName string) string {
	if idx := strings.Index(companyName, "("); idx != -1 {
		if endIdx := strings.Index(companyName[idx:], ")"); endIdx != -1 {
			return companyName[idx+1 : idx+endIdx]
		}
	}
	return "" // Return empty string if no ticker is found
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

// fetchTickerFromCIK retrieves the ticker symbol for a given CIK
func fetchTickerFromCIK(cik string) string {
	// Reverse lookup in the cache
	filingCache.RLock()
	defer filingCache.RUnlock()

	for ticker, cachedCIK := range filingCache.cikMap {
		if cachedCIK == cik {
			return ticker
		}
	}

	// If not in cache, would need to query SEC API
	// This is a fallback that attempts to get ticker from SEC data
	url := "https://www.sec.gov/files/company_tickers.json"
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating request for ticker lookup: %v\n", err)
		return ""
	}

	req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error fetching ticker data: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("SEC API returned status: %d\n", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return ""
	}

	var result map[string]struct {
		CIK    int    `json:"cik_str"`
		Ticker string `json:"ticker"`
		Name   string `json:"title"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("Error unmarshaling JSON: %v\n", err)
		return ""
	}

	targetCIK := strings.TrimLeft(cik, "0")
	for _, company := range result {
		if fmt.Sprintf("%d", company.CIK) == targetCIK {
			return company.Ticker
		}
	}

	return ""
}

// FetchLatestEdgarFilings fetches the latest SEC filings from the RSS feed
func FetchLatestEdgarFilings() ([]GlobalEDGARFiling, error) {
	// Try the JSON API first, fall back to HTML parsing if that fails
	filings, err := FetchLatestEdgarFilingsJSON()
	if err != nil {
		return nil, err
	}
	return filings, nil
}

// FetchLatestEdgarFilingsJSON fetches SEC filings using the SEC.gov Atom feed
func FetchLatestEdgarFilingsJSON() ([]GlobalEDGARFiling, error) {
	// Now fetch the latest filings using the SEC browse endpoint
	url := "https://www.sec.gov/cgi-bin/browse-edgar?action=getcurrent&type=&company=&dateb=&owner=include&start=0&count=100&output=atom"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// SEC API requires detailed User-Agent header
	req.Header.Set("User-Agent", "Atlantis Equities admin@atlantis.trading")
	req.Header.Set("Accept", "application/xml, application/atom+xml, text/xml, */*;q=0.8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

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

		// Extract CIK from entry title
		//cik := extractCIK(entry)

		// Try to convert CIK to ticker
		/*ticker := ""
		if cik != "" {
			ticker = fetchTickerFromCIK(cik)
		}*/
		ticker := ""

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

// cikToTicker attempts to convert a CIK to a ticker symbol
func cikToTicker(cik string) string {
	if cik == "" {
		return ""
	}

	// Option 1: Use a predefined mapping
	tickerMap := map[string]string{
		// Some common examples
		"1018724": "AAPL", // Apple
		"1045810": "GOOG", // Alphabet (Google)
		"1326801": "FB",   // Meta (Facebook)
		"1467858": "TWTR", // Twitter
		// Add more as needed
	}

	if ticker, exists := tickerMap[cik]; exists {
		return ticker
	}

	// Option 2: Call an external API (implement as needed)
	// return fetchTickerFromAPI(cik)

	// Option 3: Query the SEC website or database
	// Example placeholder - implement the actual logic
	ticker, err := lookupTickerByCIK(cik)
	if err == nil && ticker != "" {
		return ticker
	}

	return ""
}

// lookupTickerByCIK looks up a ticker symbol using a CIK
func lookupTickerByCIK(cik string) (string, error) {
	// This is a simplified example - you would need to implement
	// the actual logic to query the SEC or a financial data provider

	// Example: Query SEC EDGAR API
	url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// SEC requires a user agent with contact info
	req.Header.Add("User-Agent", "YourApp youremail@example.com")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SEC API returned status code %d", resp.StatusCode)
	}

	var data struct {
		Tickers []string `json:"tickers"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if len(data.Tickers) > 0 {
		return data.Tickers[0], nil
	}

	return "", fmt.Errorf("no ticker found for CIK %s", cik)
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

func parseFilingDate(updated string) string {
	// Convert the ISO 8601 timestamp to the desired format
	t, err := time.Parse(time.RFC3339, updated)
	if err != nil {
		return updated
	}
	return t.Format("2006-01-02")
}
