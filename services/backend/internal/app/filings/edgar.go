package filings

import (
	"backend/internal/data"
	"backend/internal/data/edgar"
	"backend/internal/data/postgres"
	"backend/internal/services/marketdata"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// GetLatestEdgarFilings returns the most recent SEC filings across all companies
func GetLatestEdgarFilings(_ *data.Conn, _ int, _ json.RawMessage) (interface{}, error) {
	// Get the latest filings from the cache
	filings := marketdata.GetLatestEdgarFilings()

	// Apply a limit to avoid sending too much data
	limit := 100
	if len(filings) > limit {
		filings = filings[:limit]
	}

	return filings, nil
}

// EdgarFilingOptions represents optional parameters for fetching EDGAR filings
type EdgarFilingOptions struct {
	Start      int64   `json:"start,omitempty"`
	End        int64   `json:"end,omitempty"`
	SecurityID int     `json:"securityId"`
	Ticker     *string `json:"ticker,omitempty"`
	Form       *string `json:"form,omitempty"`
}

// GetStockEdgarFilings retrieves SEC filings for a security with optional filters
func GetStockEdgarFilings(conn *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args EdgarFilingOptions
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

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
	var filteredFilings []edgar.Filing
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
func fetchEdgarFilings(cik string) ([]edgar.Filing, error) {

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
			_ = resp.Body.Close()

			// Exponential backoff
			waitTime := retryDelay * time.Duration(1<<attempt)
			time.Sleep(waitTime)
			continue
		}

		// Check for other non-success status codes
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
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
func parseEdgarFilingsResponse(body []byte, cik string) ([]edgar.Filing, error) {
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
		return []edgar.Filing{}, nil
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
	filings := make([]edgar.Filing, 0, minLen)

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

		filings = append(filings, edgar.Filing{
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
func getFilingQuarter(filing edgar.Filing) (string, int) {
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
	var targetFiling *edgar.Filing

	// If quarter is specified, filter by quarter and year
	if args.Quarter != "" {
		// Normalize the quarter input
		targetQuarter := strings.ToUpper(args.Quarter)
		// Convert "K" to "Q4" for consistency in filtering
		if targetQuarter == "K" || targetQuarter == "ANNUAL" {
			targetQuarter = "Q4"
		}

		// Filter filings based on quarter and year
		var matchingFilings []edgar.Filing
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
			_ = resp.Body.Close()

			// Exponential backoff
			waitTime := retryDelay * time.Duration(1<<attempt)
			time.Sleep(waitTime)
			continue
		}

		// Check for other non-success status codes
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
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
func extractTextFromHTML(htmlContent string) string {
	// Remove HTML tags
	textContent := removeHTMLTags(htmlContent)
	textContent = html.UnescapeString(textContent)
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

type GetExhibitListArgs struct {
	URL string `json:"url"`
}
type ExhibitStub struct {
	FileName string `json:"fileName"`
	URL      string `json:"url"`
	DocType  string `json:"docType,omitempty"` // EX-99.1, EX-2.1… (empty if not requested / unknown)
}

// GetExhibitList performs operations related to GetExhibitList functionality.
func GetExhibitList(_ *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetExhibitListArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	_, accession, base, err := splitSECURL(args.URL)
	if err != nil {
		return nil, fmt.Errorf("issue with splitting SEC URL: %v", err)
	}
	stubs, err := stubsFromHeaders(base, headersFileName(accession))
	if err != nil {
		return nil, fmt.Errorf("failed to parse headers: %v", err)
	}

	sort.Slice(stubs, func(i, j int) bool { return stubs[i].FileName < stubs[j].FileName })
	return stubs, nil
}

// splitSECURL extracts cik, accession and base directory URL from a filing page
func splitSECURL(u string) (cik, acc, base string, err error) {
	re := regexp.MustCompile(`/data/(\d+)/(\d{18})/`)
	m := re.FindStringSubmatch(u)
	if len(m) != 3 {
		return "", "", "", fmt.Errorf("SEC URL pattern not recognised: %s", u)
	}
	cik, acc = m[1], m[2]
	base = fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/", cik, acc)
	return
}
func headersFileName(accession string) string {
	// 0001045810-18-000113-index-headers.html
	return fmt.Sprintf("%s-%s-%s-index-headers.html", accession[:10], accession[10:12], accession[12:])
}

// stubsFromHeaders pulls <FILENAME> + <TYPE> out of the SGML headers file
func stubsFromHeaders(base, url string) ([]ExhibitStub, error) {
	resp, err := httpGet(base + url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	reDoc := regexp.MustCompile(`(?s)<DOCUMENT>(.*?)</DOCUMENT>`)
	reType := regexp.MustCompile(`(?i)<TYPE>\s*([^<\r\n]+)`)
	reFile := regexp.MustCompile(`(?i)<FILENAME>\s*([^<\r\n]+)`)

	all, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Unescape HTML entities
	unescapedString := html.UnescapeString(string(all))
	all = []byte(unescapedString) // Convert back to []byte for regex and existing debug print

	filter := regexp.MustCompile(`(?i)^ex.*$`)
	stubs := []ExhibitStub{}

	for _, blk := range reDoc.FindAll(all, -1) {
		typ := reType.FindSubmatch(blk)
		fnm := reFile.FindSubmatch(blk)

		if len(typ) != 2 || len(fnm) != 2 {
			continue
		}

		extractedType := string(typ[1])
		extractedFilename := string(fnm[1])

		if !filter.MatchString(extractedType) {
			continue
		}

		stubs = append(stubs, ExhibitStub{
			FileName: extractedFilename,
			DocType:  extractedType, // already "EX-99.1" etc.
			URL:      base + extractedFilename,
		})
	}
	return stubs, nil
}

type GetExhibitContentArgs struct {
	URL string `json:"url"`
}
type ExhibitContent struct {
	Text       string        `json:"text,omitempty"`   // readable text if present
	Images     []Base64Image `json:"images,omitempty"` // loaded & encoded
	ContentURL string        `json:"contentUrl"`       // always echo back original URL
}

type Base64Image struct {
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"` // "image/png", "image/jpeg" …
	DataURI  string `json:"dataUri"`  // "data:image/png;base64,AAAA…"
}

// GetExhibitContent performs operations related to GetExhibitContent functionality.
func GetExhibitContent(_ *data.Conn, _ int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetExhibitContentArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	lo := strings.ToLower(args.URL)
	switch {
	case strings.HasSuffix(lo, ".htm"), strings.HasSuffix(lo, ".html"), strings.HasSuffix(lo, ".txt"):
		return readHTMLExhibit(args.URL)
	default:
		return nil, fmt.Errorf("unsupported exhibit type")
	}
}
func readHTMLExhibit(url string) (ExhibitContent, error) {
	resp, err := httpGet(url) // same retry helper
	if err != nil {
		return ExhibitContent{}, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	html := string(b)

	// -------- text ----------
	text := extractTextFromHTML(html)
	if len(strings.Fields(text)) < 50 { // heuristic: ignore boiler-plate
		text = ""
	}

	// -------- images ---------
	imgURLs := collectImageLinks(html, url)
	imgs := fetchAndEncodeImages(imgURLs) // may return empty

	return ExhibitContent{
		Text:       text,
		Images:     imgs,
		ContentURL: url,
	}, nil
}
func collectImageLinks(html, baseURL string) []string {
	srcRe := regexp.MustCompile(`(?i)<img[^>]+src=["']?([^"'>\s]+)`)
	matches := srcRe.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil
	}
	// base dir for resolving relative links
	base := baseURL[:strings.LastIndex(baseURL, "/")+1]
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		link := strings.TrimSpace(m[1])
		if strings.HasPrefix(link, "http") {
			out = append(out, link)
		} else {
			out = append(out, base+link)
		}
	}
	return out
}
func fetchAndEncodeImages(urls []string) []Base64Image {
	const (
		maxBytes = 1 << 20 // 1 MiB per image (enough for 300 dpi letter page)
		timeout  = 10 * time.Second
	)
	out := make([]Base64Image, 0, len(urls))
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 4) // limit concurrency

	for _, u := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(link string) {
			defer func() { <-sem; wg.Done() }()
			cli := &http.Client{Timeout: timeout}
			req, _ := http.NewRequest("GET", link, nil)
			req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")
			res, err := cli.Do(req)
			if err != nil {
				return
			}
			defer res.Body.Close()

			mt := res.Header.Get("Content-Type")
			if !strings.HasPrefix(mt, "image/") {
				return
			}

			// cap to maxBytes to avoid huge scans
			lim := io.LimitReader(res.Body, maxBytes+1)
			buf, _ := io.ReadAll(lim)
			if int64(len(buf)) > maxBytes {
				return
			} // skip oversized

			enc := base64.StdEncoding.EncodeToString(buf)
			dataURI := fmt.Sprintf("data:%s;base64,%s", mt, enc)

			mu.Lock()
			out = append(out, Base64Image{
				FileName: path.Base(link),
				MimeType: mt,
				DataURI:  dataURI,
			})
			mu.Unlock()
		}(u)
	}
	wg.Wait()
	return out
}
func httpGet(url string) (*http.Response, error) {
	const maxRetries = 2
	client := &http.Client{Timeout: 30 * time.Second}

	for i := 0; i < maxRetries; i++ {
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		if resp.StatusCode == 429 {
			_ = resp.Body.Close() // Close the body before continuing
			time.Sleep(time.Duration(1<<i) * time.Second)
			continue
		}
		b, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // Close the body after reading or if read fails
		if readErr != nil {
			return nil, fmt.Errorf("status %d and failed to read body for exhibit: %v", resp.StatusCode, readErr)
		}
		previewLength := 120
		if len(b) < previewLength {
			previewLength = len(b)
		}
		return nil, fmt.Errorf("status %d: %.120s", resp.StatusCode, b[:previewLength])
	}
	return nil, fmt.Errorf("rate-limited after %d tries: %s", maxRetries, url)
}

// GetTextFromURL fetches text content from a given URL.
func GetTextFromURL(url string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header.Set("User-Agent", "atlantis admin@atlantis.trading")

	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making HTTP request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", response.Status)
	}

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	// Convert body to string and return
	return string(body), nil
}
