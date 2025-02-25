package utils

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net/http"
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

// GlobalEDGARFiling represents the structure of a filing in your application
type GlobalEDGARFiling struct {
	Type      string
	Date      time.Time
	URL       string
	Timestamp int64
	Ticker    string
	Channel   string
}

// AtomFeed represents the structure of the SEC RSS Atom feed
type AtomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []AtomEntry `xml:"entry"`
}

// AtomEntry represents an individual filing entry in the feed
type AtomEntry struct {
	Title string `xml:"title"`
	Link  struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Updated  string `xml:"updated"`
	Category struct {
		Term string `xml:"term,attr"`
	} `xml:"category"`
}

// ConvertAtomEntryToFiling converts an AtomEntry from the SEC RSS feed to a GlobalEDGARFiling
func ConvertAtomEntryToFiling(entry AtomEntry) (GlobalEDGARFiling, error) {
	// Get filing type from the category term attribute
	filingType := entry.Category.Term

	// Parse the time from Updated field
	parsedTime, err := time.Parse(time.RFC3339, entry.Updated)
	if err != nil {
		return GlobalEDGARFiling{}, fmt.Errorf("failed to parse time: %v", err)
	}

	// Extract ticker symbol from title
	companyName := ""
	parts := strings.Split(entry.Title, " - ")
	if len(parts) > 0 {
		companyName = parts[0]
	}
	ticker := extractTicker(companyName)

	// If no ticker in title, extract CIK and try to get ticker from it
	if ticker == "" && len(parts) > 1 {
		cik := extractCIK(parts[1])
		if cik != "" {
			ticker = fetchTickerFromCIK(cik)
		}
	}

	return GlobalEDGARFiling{
		Type:      filingType,
		Date:      parsedTime,
		URL:       entry.Link.Href,
		Timestamp: parsedTime.UnixMilli(),
		Ticker:    ticker,
		Channel:   "sec_rss",
	}, nil
}

// extractCIK attempts to extract a CIK from the text
func extractCIK(text string) string {
	// Look for pattern (numbers) in text
	start := strings.Index(text, "(")
	if start != -1 {
		end := strings.Index(text[start:], ")")
		if end != -1 {
			cikText := text[start+1 : start+end]
			// Remove any non-digit characters
			cik := ""
			for _, c := range cikText {
				if c >= '0' && c <= '9' {
					cik += string(c)
				}
			}
			return cik
		}
	}
	return ""
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
	return FetchPaginatedEdgarFilings(1, 100)
}

// FetchPaginatedEdgarFilings fetches multiple pages of SEC filings with pagination
func FetchPaginatedEdgarFilings(pages int, itemsPerPage int) ([]GlobalEDGARFiling, error) {
	var allFilings []GlobalEDGARFiling

	// Set up rate limiting parameters
	baseDelay := 2 * time.Second // Initial delay between requests
	maxDelay := 30 * time.Second // Maximum delay to wait
	maxRetries := 5              // Maximum number of retries

	for page := 0; page < pages; page++ {
		start := page * itemsPerPage

		// Define the SEC RSS feed URL with pagination parameters
		url := fmt.Sprintf("https://www.sec.gov/cgi-bin/browse-edgar?action=getcurrent&output=atom&start=%d&count=%d",
			start, itemsPerPage)

		// Implement retry logic with exponential backoff
		var resp *http.Response
		var err error
		currentDelay := baseDelay

		for retries := 0; retries <= maxRetries; retries++ {
			if retries > 0 {
				retryTime := currentDelay + time.Duration(rand.Intn(1000))*time.Millisecond
				fmt.Printf("Retry #%d after %v due to status %d\n", retries, retryTime, resp.StatusCode)
				time.Sleep(retryTime)
				currentDelay *= 2 // exponential backoff
				if currentDelay > maxDelay {
					currentDelay = maxDelay
				}
			}

			fmt.Printf("Fetching page %d/%d (start=%d, count=%d) - attempt %d\n",
				page+1, pages, start, itemsPerPage, retries+1)

			// Create an HTTP client with a timeout
			client := &http.Client{
				Timeout: 30 * time.Second, // Increased timeout
			}

			// Create and configure the HTTP request
			req, reqErr := http.NewRequest("GET", url, nil)
			if reqErr != nil {
				return nil, fmt.Errorf("failed to create request: %v", reqErr)
			}

			// SEC API requires detailed User-Agent header
			req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")
			req.Header.Set("Accept-Encoding", "gzip, deflate")
			req.Header.Set("Host", "www.sec.gov")

			// Execute the request
			resp, err = client.Do(req)
			if err != nil {
				if retries >= maxRetries {
					return nil, fmt.Errorf("failed to fetch SEC RSS feed after %d retries: %v",
						retries, err)
				}
				continue // Try again
			}

			// If successful or error other than 429, break the retry loop
			if resp.StatusCode == http.StatusOK ||
				(resp.StatusCode != http.StatusTooManyRequests && retries >= maxRetries) {
				break
			}

			// Close the response body if we're going to retry
			resp.Body.Close()
		}

		defer resp.Body.Close()

		// Final check of the response status
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("SEC API returned status: %d after maximum retries", resp.StatusCode)
		}

		// Decode the XML into the feed struct
		var feed AtomFeed
		decoder := xml.NewDecoder(resp.Body)
		decoder.CharsetReader = func(charsetName string, input io.Reader) (io.Reader, error) {
			return charset.NewReader(input, charsetName)
		}

		if err := decoder.Decode(&feed); err != nil {
			return nil, fmt.Errorf("failed to parse XML: %v", err)
		}

		// Check if we've reached the end of results
		if len(feed.Entries) == 0 {
			fmt.Printf("No more results available after page %d\n", page)
			break
		}

		// Convert AtomEntry objects to GlobalEDGARFiling objects
		pageFilings := 0
		for _, entry := range feed.Entries {
			filing, err := ConvertAtomEntryToFiling(entry)
			if err != nil {
				// Log the error but continue processing other entries
				fmt.Printf("Error converting filing entry: %v\n", err)
				continue
			}

			// Filter out Form 4 filings and entries without tickers
			if filing.Type != "4" && filing.Ticker != "" {
				allFilings = append(allFilings, filing)
				pageFilings++
			}
		}

		fmt.Printf("Page %d: Found %d entries, added %d filtered filings\n",
			page+1, len(feed.Entries), pageFilings)

		// If we got fewer results than requested, we've reached the end
		if len(feed.Entries) < itemsPerPage {
			fmt.Printf("Reached last page with %d entries\n", len(feed.Entries))
			break
		}

		// Add a mandatory delay between requests to respect rate limits (1-5 seconds)
		if page < pages-1 {
			delay := baseDelay + time.Duration(rand.Intn(3000))*time.Millisecond
			fmt.Printf("Waiting %v before next request...\n", delay)
			time.Sleep(delay)
		}
	}

	fmt.Printf("Successfully fetched and converted %d filings from %d pages\n", len(allFilings), pages)
	return allFilings, nil
}

func main() {
	filings, err := FetchLatestEdgarFilings()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	for _, filing := range filings {
		fmt.Printf("Type: %s, URL: %s, Date: %s, Ticker: %s\n",
			filing.Type, filing.URL, filing.Date.Format(time.RFC3339), filing.Ticker)
	}
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
