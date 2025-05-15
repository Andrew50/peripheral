package marketdata

import (
	"backend/internal/data"
	"backend/internal/data/edgar"
	"sync"
	"time"
)

var (
	latestFilingsMutex  sync.RWMutex
	latestFilings       []edgar.GlobalEDGARFiling
	edgarServiceRunning bool
	edgarServiceMutex   sync.Mutex
	NewFilingsChannel   = make(chan edgar.GlobalEDGARFiling, 100)
)

// StartEdgarFilingsService starts a background service to fetch and broadcast SEC filings
func StartEdgarFilingsService(conn *data.Conn) {
	edgarServiceMutex.Lock()
	defer edgarServiceMutex.Unlock()
	if edgarServiceRunning {
		////fmt.Println("EdgarFilingsService already running")
		return
	}

	////fmt.Println("Starting EdgarFilingsService")
	edgarServiceRunning = true

	// Initial fetch
	filings, err := edgar.FetchLatestEdgarFilings(conn)
	if err != nil {
		////fmt.Printf("Error fetching initial SEC filings: %v\n", err)
		return
	}
	latestFilingsMutex.Lock()
	latestFilings = filings
	latestFilingsMutex.Unlock()

	// Start periodic fetching
	go func() {
		ticker := time.NewTicker(10 * time.Second) // Adjust frequency as needed
		defer ticker.Stop()

		for range ticker.C {
			newFilings, err := edgar.FetchLatestEdgarFilings(conn)
			if err != nil {
				////fmt.Printf("Error fetching SEC filings: %v\n", err)
				continue
			}

			// Identify new filings
			latestFilingsMutex.RLock()
			currentURLs := make(map[string]bool)
			for _, f := range latestFilings {
				currentURLs[f.URL] = true
			}
			latestFilingsMutex.RUnlock()

			var newOnes []edgar.GlobalEDGARFiling
			for _, filing := range newFilings {
				if currentURLs[filing.URL] {
					break // Stop at the first known filing
				}
				newOnes = append(newOnes, filing)
			}

			// Update cache
			latestFilingsMutex.Lock()
			latestFilings = newFilings
			latestFilingsMutex.Unlock()
			// Send new filings to channel
			for _, filing := range newOnes {
				select {
				case NewFilingsChannel <- filing:
					////fmt.Printf("Sent new filing to channel: %s\n", filing.URL)
				default:
					////fmt.Println("Warning: NewFilingsChannel is full, dropping filing")
				}
			}
		}
	}()

	////fmt.Println("EdgarFilingsService started")
}

// GetLatestEdgarFilings returns a copy of the cached latest SEC filings
func GetLatestEdgarFilings() []edgar.GlobalEDGARFiling {
	latestFilingsMutex.RLock()
	defer latestFilingsMutex.RUnlock()
	filingsCopy := make([]edgar.GlobalEDGARFiling, len(latestFilings))
	copy(filingsCopy, latestFilings)
	return filingsCopy
}
