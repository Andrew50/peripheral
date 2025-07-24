package securities

import (
	"backend/internal/data"
	"backend/internal/data/polygon"
	"backend/internal/data/utils"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Helper function to truncate string if it exceeds maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// UpdateSecurityDetails updates detailed information for active securities including logos, icons, and financial data
func UpdateSecurityDetails(conn *data.Conn, test bool) error {
	// Query active securities (where maxDate is null)

	// First, count how many securities need updating
	var count int
	err := conn.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) 
		 FROM securities 
		 WHERE maxDate IS NULL AND (logo IS NULL OR icon IS NULL)`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count securities needing updates: %v", err)
	}

	// If no securities need updating, return success
	if count == 0 {
		return nil
	}

	rows, err := conn.DB.Query(context.Background(),
		`SELECT securityid, ticker 
		 FROM securities 
		 WHERE maxDate IS NULL`)
	if err != nil {
		return fmt.Errorf("failed to query active securities: %v", err)
	}
	defer rows.Close()

	// Create a rate limiter for 10 requests per second
	rateLimiter := time.NewTicker(100 * time.Millisecond) // 10 requests per second
	defer rateLimiter.Stop()

	// Create a worker pool with semaphore to limit concurrent requests (reduced to 3)
	maxWorkers := 3

	sem := make(chan struct{}, maxWorkers)
	errChan := make(chan error, maxWorkers)
	var wg sync.WaitGroup

	// Helper function to fetch and encode image data
	fetchImage := func(url string, polygonKey string) (string, error) {
		if url == "" {
			return "", nil
		}

		maxAttempts := 3
		delay := 1 * time.Second
		var lastErr error

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			// Create HTTP client with timeout to prevent hanging
			client := &http.Client{Timeout: 10 * time.Second}
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return "", fmt.Errorf("failed to create request: %v", err)
			}
			req.Header.Add("Authorization", "Bearer "+polygonKey)

			resp, err := client.Do(req)
			if err != nil {
				// Log timeout or network errors
				if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "context deadline exceeded") {
					log.Printf("Timeout error fetching image from %s (attempt %d/%d): %v", url, attempt, maxAttempts, err)
				} else {
					log.Printf("Network error fetching image from %s (attempt %d/%d): %v", url, attempt, maxAttempts, err)
				}
				lastErr = err
			} else {
				// We got a response, check status code
				if resp.StatusCode != http.StatusOK {
					log.Printf("HTTP error %d fetching image from %s (attempt %d/%d)", resp.StatusCode, url, attempt, maxAttempts)
					lastErr = fmt.Errorf("status code: %d", resp.StatusCode)
				} else {
					// Success path
					imageData, errRead := io.ReadAll(resp.Body)
					if closeErr := resp.Body.Close(); closeErr != nil {
						log.Printf("Warning: failed to close response body: %v", closeErr)
					}
					if errRead != nil {
						log.Printf("Error reading image data from %s: %v", url, errRead)
						lastErr = errRead
					} else if len(imageData) == 0 {
						log.Printf("Empty image data received from %s", url)
						lastErr = fmt.Errorf("empty image data")
					} else {
						contentType := resp.Header.Get("Content-Type")
						if contentType == "" {
							contentType = http.DetectContentType(imageData)
							if contentType == "" || contentType == "application/octet-stream" {
								if strings.HasSuffix(strings.ToLower(url), ".svg") {
									contentType = "image/svg+xml"
								} else if strings.HasSuffix(strings.ToLower(url), ".png") {
									contentType = "image/png"
								} else {
									contentType = "image/jpeg"
								}
							}
						}

						if strings.HasPrefix(contentType, "data:") {
							return "", fmt.Errorf("invalid content type: %s", contentType)
						}

						base64Data := base64.StdEncoding.EncodeToString(imageData)
						if strings.HasPrefix(base64Data, "data:") {
							return base64Data, nil
						}
						return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
					}
				}
			}

			// Prepare for next attempt if not last
			if attempt < maxAttempts {
				time.Sleep(delay)
				delay *= 2 // Exponential backoff
			}
		}

		return "", fmt.Errorf("failed to fetch image after %d attempts: %v", maxAttempts, lastErr)
	}

	// Worker function to process each security
	processSecurity := func(securityID int, ticker string) {
		defer wg.Done()
		defer func() { <-sem }() // Release semaphore slot

		<-rateLimiter.C // Wait for rate limiter

		details, err := polygon.GetTickerDetails(conn.Polygon, ticker, "now")
		if err != nil {
			//log.Printf("Failed to get details for %s: %v", ticker, err)
			return
		}

		// Fetch both logo and icon
		logoBase64, err := fetchImage(details.Branding.LogoURL, conn.PolygonKey)
		if err != nil {
			log.Printf("Failed to fetch logo for %s: %v", ticker, err)
		}
		iconBase64, err := fetchImage(details.Branding.IconURL, conn.PolygonKey)
		if err != nil {
			log.Printf("Failed to fetch icon for %s: %v", ticker, err)
		}
		currentPrice, err := polygon.GetMostRecentRegularClose(conn.Polygon, ticker, time.Now())
		if err != nil {
			//log.Printf("Failed to get current price for %s: %v", ticker, err)
			return
		}

		// Update the security record with all details
		_, err = conn.DB.Exec(context.Background(),
			`UPDATE securities 
			 SET name = NULLIF($1, ''),
				 market = NULLIF($2, ''),
				 locale = NULLIF($3, ''),
				 primary_exchange = NULLIF($4, ''),
				 active = $5,
				 market_cap = NULLIF($6::BIGINT, 0),
				 description = NULLIF($7, ''),
				 logo = NULLIF($8, ''),
				 icon = NULLIF($9, ''),
				 share_class_shares_outstanding = NULLIF($10::BIGINT, 0),
				 total_shares = CASE 
					 WHEN NULLIF($6::BIGINT, 0) > 0 AND NULLIF($12, 0) > 0 
					 THEN CAST(($6::BIGINT / $12) AS BIGINT)
					 ELSE NULL 
				 END,
				 share_class_figi = NULLIF($13, ''),
				 sic_code = NULLIF($14, ''),
				 sic_description = NULLIF($15, ''),
				 total_employees = NULLIF($16::BIGINT, 0),
				 weighted_shares_outstanding = NULLIF($17::BIGINT, 0)
			 WHERE securityid = $11`,
			utils.NullString(details.Name),
			utils.NullString(truncateString(string(details.Market), 50)),
			utils.NullString(truncateString(string(details.Locale), 50)),
			utils.NullString(truncateString(details.PrimaryExchange, 50)),
			details.Active,
			utils.NullInt64(int64(details.MarketCap)),
			utils.NullString(details.Description),
			utils.NullString(logoBase64),
			utils.NullString(iconBase64),
			utils.NullInt64(details.ShareClassSharesOutstanding),
			securityID,
			currentPrice,
			utils.NullString(details.ShareClassFIGI),
			utils.NullString(details.SICCode),
			utils.NullString(details.SICDescription),
			utils.NullInt64(int64(details.TotalEmployees)),
			utils.NullInt64(details.WeightedSharesOutstanding))

		if err != nil {
			if test {
				log.Printf("Failed to update details for %s: Column error - market_cap=%v, share_class_shares_outstanding=%v - Error: %v",
					ticker,
					details.MarketCap,
					details.ShareClassSharesOutstanding,
					err)
			}
			errChan <- fmt.Errorf("failed to update %s: Column error - market_cap=%v, share_class_shares_outstanding=%v - Error: %v",
				ticker,
				details.MarketCap,
				details.ShareClassSharesOutstanding,
				err)
			return
		}

		// Successfully updated details - no action needed in non-test mode
		// Uncomment the log line below if you want to enable logging in test mode
		// if test {
		//     log.Printf("Successfully updated details for %s", ticker)
		// }
	}

	// Process all securities
	for rows.Next() {
		var securityID int
		var ticker string
		if err := rows.Scan(&securityID, &ticker); err != nil {
			return fmt.Errorf("failed to scan security row: %v", err)
		}

		sem <- struct{}{} // Acquire semaphore slot
		wg.Add(1)
		go processSecurity(securityID, ticker)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors during update: %v", len(errors), errors)
	}

	return nil
}
