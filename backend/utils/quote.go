package utils

import (
	"context"
	"fmt"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

func GetAggsData(client *polygon.Client, ticker string, multiplier int, timeframe string,
	fromMillis models.Millis, toMillis models.Millis, bars int, resultsOrder string, isAdjusted bool) (*iter.Iter[models.Agg], error) {
	timespan := models.Timespan(timeframe)
	if resultsOrder != "asc" && resultsOrder != "desc" {
		return nil, fmt.Errorf("incorrect order string passed %s", resultsOrder)
	}
	params := models.ListAggsParams{
		Ticker:     ticker,
		Multiplier: multiplier,
		Timespan:   timespan,
		From:       fromMillis,
		To:         toMillis,
	}.WithOrder(models.Order(resultsOrder)).WithLimit(bars).WithAdjusted(isAdjusted)
	iter := client.ListAggs(context.Background(), params)
	return iter, nil
}

func GetTradeAtTimestamp(client *polygon.Client, securityId int, timestamp time.Time) (models.Trade, error) {
	ticker, err := GetTicker(conn, securityId, timestamp)
	if err != nil {
		return models.Trade{}, fmt.Errorf("sif20ih %v", err)
	}
	nanoTimestamp := models.Nanos(timestamp)
	iter, err := GetTrade(client, ticker, nanoTimestamp, "desc", models.LTE, 1)
	if err != nil {
		return models.Trade{}, fmt.Errorf("error initiating trade search: %v", err)
	}
	for iter.Next() {
		return iter.Item(), nil
	}
	if err := iter.Err(); err != nil {
		return models.Trade{}, fmt.Errorf("error fetching trade: %v", err)
	}
	return models.Trade{}, fmt.Errorf("no trade found for ticker %s at timestamp %v", ticker, nanoTimestamp)
}

func GetQuoteAtTimestamp(client *polygon.Client, securityId int, timestamp time.Time) (models.Quote, error) {
	ticker, err := GetTicker(conn, securityId, timestamp)
	if err != nil {
		return models.Quote{}, fmt.Errorf("doi20 %v", err)
	}
	nanoTimestamp := models.Nanos(timestamp)
	iter := GetQuote(client, ticker, nanoTimestamp, "desc", models.LTE, 1)
	for iter.Next() {
		return iter.Item(), nil
	}
	if err := iter.Err(); err != nil {
		return models.Quote{}, fmt.Errorf("error fetching quote: %v", err)
	}
	return models.Quote{}, fmt.Errorf("no quote found for ticker %s at timestamp %v", ticker, nanoTimestamp)
}

func GetLastQuote(client *polygon.Client, ticker string) (models.LastQuote, error) {
	params := &models.GetLastQuoteParams{
		Ticker: ticker,
	}
	res, err := client.GetLastQuote(context.Background(), params)
	if err != nil {
		return res.Results, err
	}
	return res.Results, nil
}

func GetQuote(client *polygon.Client, ticker string, nanoTimestamp models.Nanos, ord string, compareType models.Comparator, numResults int) *iter.Iter[models.Quote] {
	sortOrder := models.Desc
	if ord == "asc" {
		sortOrder = models.Asc
	}
	params := models.ListQuotesParams{
		Ticker: ticker,
	}.WithTimestamp(compareType, nanoTimestamp).
		WithSort(models.Timestamp).
		WithOrder(sortOrder).
		WithLimit(numResults)
	return client.ListQuotes(context.Background(), params)
}

func GetLastTrade(client *polygon.Client, ticker string) (models.LastTrade, error) {
	params := &models.GetLastTradeParams{
		Ticker: ticker,
	}
	res, err := client.GetLastTrade(context.Background(), params)
	if err != nil {
		return res.Results, err
	}
	return res.Results, nil
}

func GetTrade(client *polygon.Client, ticker string, nanoTimestamp models.Nanos, ord string, compareType models.Comparator, numResults int) (*iter.Iter[models.Trade], error) {
	sortOrder := models.Desc
	if ord != "asc" && ord != "desc" {
		return nil, fmt.Errorf("incorrect order string passed 35ltkg")
	}
	if ord == "asc" {
		sortOrder = models.Asc
	}
	params := models.ListTradesParams{
		Ticker: ticker,
	}.WithTimestamp(compareType, nanoTimestamp).
		WithOrder(sortOrder).
		WithLimit(numResults).
		WithSort(models.Timestamp)
	return client.ListTrades(context.Background(), params), nil
}

// GetMostRecentRegularClose gets the most recent close price from regular trading hours
func GetMostRecentRegularClose(client *polygon.Client, ticker string) (float64, error) {
	// Get today's date in Eastern time
	now := time.Now().In(easternLocation)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, easternLocation)

	// Convert to models.Millis
	startMillis := models.Millis(startOfDay)
	endMillis := models.Millis(now)

	// Get 1-minute bars for today
	iter, err := GetAggsData(client, ticker, 1, "minute", startMillis, endMillis, 1000, "desc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting aggregates: %v", err)
	}

	// Find the most recent bar during regular hours
	for iter.Next() {
		agg := iter.Item()
		timestamp := time.Time(agg.Timestamp)
		if IsTimestampRegularHours(timestamp) {
			return agg.Close, nil
		}
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("error iterating aggregates: %v", err)
	}

	// If no regular hours data found today, get yesterday's close
	yesterday := startOfDay.AddDate(0, 0, -1)
	yesterdayStart := models.Millis(yesterday)
	yesterdayEnd := models.Millis(startOfDay.Add(-time.Nanosecond))

	iter, err = GetAggsData(client, ticker, 1, "day", yesterdayStart, yesterdayEnd, 1, "desc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting yesterday's data: %v", err)
	}

	for iter.Next() {
		return iter.Item().Close, nil
	}

	return 0, fmt.Errorf("no recent regular hours close found for %s", ticker)
}

// GetMostRecentRegularOpen gets the most recent open price from regular trading hours
func GetMostRecentExtendedHoursClose(client *polygon.Client, ticker string) (float64, error) {
	// Get today's date in Eastern time
	now := time.Now().In(easternLocation)
	endOfExtendedHours := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, easternLocation)

	// Convert to models.Millis
	startMillis := models.Millis(endOfExtendedHours)
	endMillis := models.Millis(now)

	// Get 1-minute bars for today
	iter, err := GetAggsData(client, ticker, 1, "minute", startMillis, endMillis, 1000, "desc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting aggregates: %v", err)
	}

	// Find the most recent bar during regular hours
	for iter.Next() {
		agg := iter.Item()
		timestamp := time.Time(agg.Timestamp)
		if IsTimestampRegularHours(timestamp) {
			return agg.Close, nil
		}
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("error iterating aggregates: %v", err)
	}

	// If no regular hours data found today, get yesterday's open
	yesterday := endOfExtendedHours.AddDate(0, 0, -1)
	yesterdayStart := models.Millis(yesterday)
	yesterdayEnd := models.Millis(endOfExtendedHours.Add(-time.Nanosecond))

	iter, err = GetAggsData(client, ticker, 1, "day", yesterdayStart, yesterdayEnd, 1, "desc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting yesterday's data: %v", err)
	}

	for iter.Next() {
		return iter.Item().Open, nil
	}

	return 0, fmt.Errorf("no recent regular hours open found for %s", ticker)
}
