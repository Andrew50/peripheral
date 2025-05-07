package edgar

import (
    "backend/internal/data"
)

// FetchLatestEdgarFilings fetches the latest SEC filings from the RSS feed
func FetchLatestEdgarFilings(conn *data.Conn) ([]GlobalEDGARFiling, error) {
	var allFilings []GlobalEDGARFiling
	maxResults := 1500 // Maximum number of filings to fetch
	perPage := 100     // Number of results per API request

	for page := 1; len(allFilings) < maxResults; page++ {
		filings, err := fetchEdgarFilingsPage(page, perPage)
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
