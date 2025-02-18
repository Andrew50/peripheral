package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// EDGARFiling represents a single SEC filing
type EDGARFiling struct {
	Type string    `json:"type"` // e.g., "10-K", "8-K", "13F"
	Date time.Time `json:"date"`
	URL  string    `json:"url"`
}

// Cache implementation with expiration
type edgarCache struct {
	sync.RWMutex
	data   map[string][]EDGARFiling
	expiry map[string]time.Time
	cikMap map[string]string // ticker -> CIK mapping
}

var (
	filingCache = &edgarCache{
		data:   make(map[string][]EDGARFiling),
		expiry: make(map[string]time.Time),
		cikMap: make(map[string]string),
	}
	cacheExpiration = 30 * time.Minute
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

		// Format the accession number by removing dashes
		accessionNumber := strings.Replace(recent.AccessionNumber[i], "-", "", -1)

		// Create URL that points to the human-readable HTML page
		htmlURL := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/%s",
			cik, accessionNumber, recent.PrimaryDocument[i])

		filings = append(filings, EDGARFiling{
			Type: recent.Form[i],
			Date: date,
			URL:  htmlURL,
		})
	}

	fmt.Printf("Found %d filings\n", len(filings))
	return filings, nil
}

func GetRecentEdgarFilings(conn *Conn, securityId int, timestamp time.Time) ([]EDGARFiling, error) {
	// Get ticker for the security
	ticker, err := GetTicker(conn, securityId, timestamp)
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

	// Check filings cache
	filingCache.RLock()
	if filings, exists := filingCache.data[cik]; exists {
		if time.Now().Before(filingCache.expiry[cik]) {
			filingCache.RUnlock()
			return filings, nil
		}
	}
	filingCache.RUnlock()

	// Fetch from EDGAR
	filings, err := fetchEdgarFilings(cik)
	if err != nil {
		return nil, err
	}

	// Update cache
	filingCache.Lock()
	filingCache.data[cik] = filings
	filingCache.expiry[cik] = time.Now().Add(cacheExpiration)
	filingCache.Unlock()

	return filings, nil
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
