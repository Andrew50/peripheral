package tools

import (
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

// GetChartEventsArgs represents a structure for handling GetChartEventsArgs data.
type GetChartEventsArgs struct {
	SecurityID        int   `json:"securityId"`
	From              int64 `json:"from"`
	To                int64 `json:"to"`
	IncludeSECFilings bool  `json:"includeSECFilings"`
}

// ChartEvent represents a structure for handling ChartEvent data.
type ChartEvent struct {
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"`
	Value     string `json:"value"`
}

// GetChartEvents performs operations related to GetChartEvents functionality.
func GetChartEvents(conn *utils.Conn, userId int, rawArgs json.RawMessage) (interface{}, error) {
	var args GetChartEventsArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}
	// Convert from milliseconds to seconds for time.Unix
	ticker, err := utils.GetTicker(conn, args.SecurityID, time.Unix(args.From/1000, 0))
	if err != nil {
		return nil, fmt.Errorf("error fetching ticker for %d: %w", args.SecurityID, err)
	}

	// Create a WaitGroup to synchronize goroutines
	var wg sync.WaitGroup

	// Only add SEC filings to the waitgroup if requested
	if args.IncludeSECFilings {
		wg.Add(3) // Three tasks: splits, dividends, SEC filings
	} else {
		wg.Add(2) // Only two tasks: splits and dividends
	}

	// Create a mutex to protect the events slice during concurrent writes
	var mutex sync.Mutex
	var events []ChartEvent
	var splitErr, dividendErr, secFilingErr error

	// Load New York location for timezone conversion
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("error loading New York timezone: %w", err)
	}

	// Fetch splits in parallel
	go func() {
		defer wg.Done()
		splits, err := getStockSplits(conn, ticker)
		if err != nil {
			splitErr = fmt.Errorf("error fetching splits for %s: %w", ticker, err)
			return
		}

		// Process splits and add to events with mutex protection
		var splitEvents []ChartEvent
		for _, split := range splits {
			// Parse the execution date from the split
			splitDate := time.Time(split.ExecutionDate)

			// Set to 4 AM New York time on that date
			splitDateET := time.Date(
				splitDate.Year(),
				splitDate.Month(),
				splitDate.Day(),
				4, 0, 0, 0,
				nyLoc,
			)
			splitTo := int(math.Round(split.SplitTo))
			splitFrom := int(math.Round(split.SplitFrom))
			ratio := fmt.Sprintf("%d:%d", splitTo, splitFrom)

			// Create a structured value
			valueMap := map[string]interface{}{
				"ratio": ratio,
				"date":  splitDateET.Format("2006-01-02"),
			}

			// Convert the map to JSON
			valueJSON, err := json.Marshal(valueMap)
			if err != nil {
				splitErr = fmt.Errorf("error creating split value: %w", err)
				return
			}

			// Convert to UTC timestamp
			utcTimestamp := splitDateET.UTC().Unix() * 1000
			// Add to events if it's within the requested time range
			if utcTimestamp >= args.From && utcTimestamp <= args.To {
				splitEvents = append(splitEvents, ChartEvent{
					Timestamp: utcTimestamp,
					Type:      "split",
					Value:     string(valueJSON),
				})
			}
		}

		// Add split events to the main events slice
		mutex.Lock()
		events = append(events, splitEvents...)
		mutex.Unlock()
	}()

	// Fetch dividends in parallel
	go func() {
		defer wg.Done()
		dividends, err := getStockDividends(conn, ticker)
		if err != nil {
			dividendErr = fmt.Errorf("error fetching dividends for %s: %w", ticker, err)
			return
		}

		// Process dividends and add to events with mutex protection
		var dividendEvents []ChartEvent
		for _, dividend := range dividends {
			// Parse the ex-dividend date
			exDate, err := time.Parse("2006-01-02", dividend.ExDividendDate)
			if err != nil {
				dividendErr = fmt.Errorf("error parsing dividend date %s: %w", dividend.ExDividendDate, err)
				return
			}
			payDate := time.Time(dividend.PayDate)
			payDateString := payDate.Format("2006-01-02")
			exDateET := time.Date( //set to 4 am new york time on that date
				exDate.Year(),
				exDate.Month(),
				exDate.Day(),
				4, 0, 0, 0,
				nyLoc,
			)
			utcTimestamp := exDateET.UTC().Unix() * 1000

			// Create a structured value with multiple pieces of information
			valueMap := map[string]interface{}{
				"amount":  fmt.Sprintf("%.2f", dividend.CashAmount),
				"exDate":  dividend.ExDividendDate,
				"payDate": payDateString,
			}

			// Convert the map to JSON
			valueJSON, err := json.Marshal(valueMap)
			if err != nil {
				dividendErr = fmt.Errorf("error creating dividend value: %w", err)
				return
			}

			// Add to events if it's within the requested time range
			if utcTimestamp >= args.From && utcTimestamp <= args.To {
				dividendEvents = append(dividendEvents, ChartEvent{
					Timestamp: utcTimestamp,
					Type:      "dividend",
					Value:     string(valueJSON),
				})
			}
		}

		// Add dividend events to the main events slice
		mutex.Lock()
		events = append(events, dividendEvents...)
		mutex.Unlock()
	}()

	// Only fetch SEC filings if requested
	if args.IncludeSECFilings {
		go func() {
			defer wg.Done()
			options := EdgarFilingOptions{
				Start:      args.From,
				End:        args.To,
				SecurityID: args.SecurityID,
			}
			optionsJSON, err := json.Marshal(options)
			if err != nil {
				return
			}
			res, err := GetStockEdgarFilings(conn, userId, optionsJSON)
			if err != nil {
				// Log the error but don't fail the entire request
				secFilingErr = fmt.Errorf("error fetching SEC filings for %s: %v", ticker, err)
				return
			}

			// Process SEC filings and add to events with mutex protection
			var filingEvents []ChartEvent
			filings := res.([]utils.EDGARFiling)
			// Process SEC filings
			for _, filing := range filings {
				// The timestamp is already in UTC milliseconds
				utcTimestamp := filing.Timestamp

				// Create a structured value with filing information
				valueMap := map[string]interface{}{
					"type": filing.Type,
					"date": filing.Date.Format("2006-01-02"),
					"url":  filing.URL,
				}

				// Convert the map to JSON
				valueJSON, err := json.Marshal(valueMap)
				if err != nil {
					fmt.Printf("Error creating filing value: %v\n", err)
					continue
				}

				// Add to events if it's within the requested time range
				if utcTimestamp >= args.From && utcTimestamp <= args.To {
					filingEvents = append(filingEvents, ChartEvent{
						Timestamp: utcTimestamp,
						Type:      "sec_filing",
						Value:     string(valueJSON),
					})
				}
			}

			// Add filing events to the main events slice
			mutex.Lock()
			events = append(events, filingEvents...)
			mutex.Unlock()
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check for errors (we'll report the first one we find)
	if splitErr != nil {
		return nil, splitErr
	}
	if dividendErr != nil {
		return nil, dividendErr
	}
	if secFilingErr != nil {
		// Log the error but don't fail the request
		fmt.Println(secFilingErr)
	}

	// Sort events by timestamp in ascending order
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp < events[j].Timestamp
	})

	return events, nil
}

func getStockSplits(conn *utils.Conn, ticker string) ([]models.Split, error) {
	// Set up parameters for the splits API call
	params := models.ListSplitsParams{
		TickerEQ: &ticker,
	}.WithOrder(models.Order("desc")).WithLimit(10)

	// Execute the API call and get an iterator
	iter := conn.Polygon.ListSplits(context.Background(), params)

	// Collect all splits
	var splits []models.Split
	for iter.Next() {
		split := iter.Item()
		splits = append(splits, split)
	}
	// Check for errors during iteration
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("error fetching splits for %s: %w", ticker, err)
	}

	return splits, nil
}

func getStockDividends(conn *utils.Conn, ticker string) ([]models.Dividend, error) {
	// Set up parameters for the dividends API call
	params := models.ListDividendsParams{
		TickerEQ: &ticker,
	}.WithOrder(models.Order("desc")).WithLimit(100)

	// Execute the API call and get an iterator
	iter := conn.Polygon.ListDividends(context.Background(), params)

	// Collect all dividends
	var dividends []models.Dividend
	for iter.Next() {
		dividend := iter.Item()
		dividends = append(dividends, dividend)
	}

	// Check for errors during iteration
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("error fetching dividends for %s: %w", ticker, err)
	}

	return dividends, nil
}
