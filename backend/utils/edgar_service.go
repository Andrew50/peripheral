package utils

import (
	"fmt"
	"sync"
	"time"
)

var (
	latestFilingsMutex  sync.RWMutex
	latestFilings       []GlobalEDGARFiling
	edgarServiceRunning bool
	edgarServiceMutex   sync.Mutex
	NewFilingsChannel   = make(chan GlobalEDGARFiling, 100)
)

// StartEdgarFilingsService starts a background service to fetch and broadcast SEC filings
func StartEdgarFilingsService() {
	edgarServiceMutex.Lock()
	defer edgarServiceMutex.Unlock()
	if edgarServiceRunning {
		fmt.Println("EdgarFilingsService already running")
		return
	}

	fmt.Println("Starting EdgarFilingsService")
	edgarServiceRunning = true

	// Initial fetch
	filings, err := FetchLatestEdgarFilings()
	if err != nil {
		fmt.Printf("Error fetching initial SEC filings: %v\n", err)
	} else {
		latestFilingsMutex.Lock()
		latestFilings = filings
		latestFilingsMutex.Unlock()
	}

	// Start periodic fetching
	go func() {
		ticker := time.NewTicker(1 * time.Minute) // Adjust frequency as needed
		defer ticker.Stop()

		for range ticker.C {
			newFilings, err := FetchLatestEdgarFilings()
			if err != nil {
				fmt.Printf("Error fetching SEC filings: %v\n", err)
				continue
			}

			// Identify new filings
			latestFilingsMutex.RLock()
			currentURLs := make(map[string]bool)
			for _, f := range latestFilings {
				currentURLs[f.URL] = true
			}
			latestFilingsMutex.RUnlock()

			var newOnes []GlobalEDGARFiling
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
					fmt.Printf("Sent new filing to channel: %s\n", filing.URL)
				default:
					fmt.Println("Warning: NewFilingsChannel is full, dropping filing")
				}
			}
		}
	}()

	fmt.Println("EdgarFilingsService started")
}

// GetLatestEdgarFilings returns a copy of the cached latest SEC filings
func GetLatestEdgarFilings() []GlobalEDGARFiling {
	latestFilingsMutex.RLock()
	defer latestFilingsMutex.RUnlock()
	filingsCopy := make([]GlobalEDGARFiling, len(latestFilings))
	copy(filingsCopy, latestFilings)
	return filingsCopy
}
