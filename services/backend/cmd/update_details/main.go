package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"backend/utils"
)

func main() {
	log.Println("Starting security details update...")

	// Create a database connection
	conn, cleanup := utils.InitConn(true)
	defer cleanup()

	log.Println("Updating security details...")

	// Call our local implementation of updateSecurityDetails
	err := updateSecurityDetails(conn, false)
	if err != nil {
		log.Printf("Warning: Update completed with some errors: %v", err)
	} else {
		log.Println("Security details update completed successfully!")
	}
}

// A minimal implementation of updateSecurityDetails based on the original function
func updateSecurityDetails(conn *utils.Conn, test bool) error {
	// Query active securities (where maxDate is null)
	fmt.Println("Updating security details")
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
	maxWorkers := 15

	sem := make(chan struct{}, maxWorkers)
	errChan := make(chan error, maxWorkers)
	var wg sync.WaitGroup

	// Counter for processed securities
	var securityCount int
	var countMutex sync.Mutex

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
	processSecurity := func(securityId int, ticker string) {
		defer wg.Done()
		defer func() { <-sem }() // Release semaphore slot

		<-rateLimiter.C // Wait for rate limiter

		details, err := utils.GetTickerDetails(conn.Polygon, ticker, "now")
		if err != nil {
			fmt.Printf("Skipping ticker %s: %v\n", ticker, err)
			return // Skip this ticker and continue with others
		}

		// Fetch both logo and icon
		logoBase64, logoErr := fetchImage(details.Branding.LogoURL, conn.PolygonKey)
		if logoErr != nil {
			fmt.Printf("Failed to fetch logo for %s: %v\n", ticker, logoErr)
			// Continue anyway, we'll just have a null logo
		}

		iconBase64, iconErr := fetchImage(details.Branding.IconURL, conn.PolygonKey)
		if iconErr != nil {
			fmt.Printf("Failed to fetch icon for %s: %v\n", ticker, iconErr)
			// Continue anyway, we'll just have a null icon
		}

		currentPrice, err := utils.GetMostRecentRegularClose(conn.Polygon, ticker, time.Now())
		if err != nil {
			fmt.Printf("Failed to get current price for %s: %v\n", ticker, err)
			// Continue anyway, we'll just have a null total_shares
		}

		// Update the security record with all details
		_, err = conn.DB.Exec(context.Background(),
			`UPDATE securities 
			 SET name = NULLIF($1, ''),
				 market = NULLIF($2, ''),
				 locale = NULLIF($3, ''),
				 primary_exchange = NULLIF($4, ''),
				 active = $5,
				 market_cap = CAST(NULLIF($6, 0) AS NUMERIC),
				 description = NULLIF($7, ''),
				 logo = NULLIF($8, ''),
				 icon = NULLIF($9, ''),
				 share_class_shares_outstanding = CAST(NULLIF($10, 0) AS BIGINT),
				 total_shares = CASE 
					 WHEN NULLIF($6::numeric, 0) > 0 AND NULLIF($12::numeric, 0) > 0 
					 THEN CAST(($6::numeric / $12::numeric) AS BIGINT)
					 ELSE NULL 
				 END
			 WHERE securityid = $11`,
			utils.NullString(details.Name),
			utils.NullString(string(details.Market)),
			utils.NullString(string(details.Locale)),
			utils.NullString(details.PrimaryExchange),
			details.Active,
			utils.NullInt64(int64(details.MarketCap)),
			utils.NullString(details.Description),
			utils.NullString(logoBase64),
			utils.NullString(iconBase64),
			utils.NullInt64(details.ShareClassSharesOutstanding),
			securityId,
			currentPrice)

		if err != nil {
			if test {
				log.Printf("Failed to update details for %s: %v", ticker, err)
			}
			errChan <- fmt.Errorf("failed to update %s: %v", ticker, err)
			return
		}

		if test {
			//log.Printf("Successfully updated details for %s", ticker)
		}

		// Increment the security count
		countMutex.Lock()
		securityCount++
		countMutex.Unlock()
	}

	// Process all securities
	for rows.Next() {
		var securityId int
		var ticker string
		if err := rows.Scan(&securityId, &ticker); err != nil {
			return fmt.Errorf("failed to scan security row: %v", err)
		}

		sem <- struct{}{} // Acquire semaphore slot
		wg.Add(1)
		go processSecurity(securityId, ticker)
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
		fmt.Printf("Completed with %d errors. Some securities may not have been updated correctly.\n", len(errors))
		// Log just the first few errors as examples
		maxErrorsToShow := 5
		for i, err := range errors {
			if i >= maxErrorsToShow {
				fmt.Printf("... and %d more errors\n", len(errors)-maxErrorsToShow)
				break
			}
			fmt.Printf("Error %d: %v\n", i+1, err)
		}
		return fmt.Errorf("encountered %d errors during update", len(errors))
	}

	fmt.Printf("Security details updated successfully. Processed %d securities.\n", securityCount)
	return nil
}
