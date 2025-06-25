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
		 WHERE maxDate IS NULL AND (logo IS NULL OR icon IS NULL)`)
	if err != nil {
		return fmt.Errorf("failed to query active securities: %v", err)
	}
	defer rows.Close()

	// Create a rate limiter for 10 requests per second
	rateLimiter := time.NewTicker(100 * time.Millisecond) // 10 requests per second
	defer rateLimiter.Stop()

	// Create a worker pool with semaphore to limit concurrent requests
	maxWorkers := 5

	sem := make(chan struct{}, maxWorkers)
	errChan := make(chan error, maxWorkers)
	var wg sync.WaitGroup

	// Helper function to fetch and encode image data
	fetchImage := func(url string, polygonKey string) (string, error) {
		if url == "" {
			return "", nil
		}

		client := &http.Client{}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Add("Authorization", "Bearer "+polygonKey)

		resp, err := client.Do(req)
		if err != nil {

			return "", fmt.Errorf("failed to fetch image: %v", err)
		}
		defer resp.Body.Close()

		// Check if the response status is OK
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to fetch image, status code: %d", resp.StatusCode)
		}

		imageData, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read image data: %v", err)
		}

		// If no image data was returned, return empty string
		if len(imageData) == 0 {
			return "", fmt.Errorf("empty image data received")
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			// Try to detect content type from image data
			contentType = http.DetectContentType(imageData)

			// If still empty, default to a safe type based on URL extension
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

		// Ensure the content type doesn't already contain a data URL prefix
		if strings.HasPrefix(contentType, "data:") {
			return "", fmt.Errorf("invalid content type: %s", contentType)
		}

		base64Data := base64.StdEncoding.EncodeToString(imageData)

		// Check if base64Data already contains a data URL prefix to prevent duplication
		if strings.HasPrefix(base64Data, "data:") {
			return base64Data, nil
		}

		return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
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
				 END
			 WHERE securityid = $11`,
			utils.NullString(truncateString(details.Name, 500)),
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
			currentPrice)

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
