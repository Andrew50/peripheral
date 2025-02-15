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
func GetMostRecentRegularClose(client *polygon.Client, ticker string, referenceTime time.Time) (float64, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return 0, fmt.Errorf("error loading location: %v", err)
	}

	refLocal := referenceTime.In(easternLocation)
	startOfDay := time.Date(refLocal.Year(), refLocal.Month(), refLocal.Day(), 0, 0, 0, 0, easternLocation)

	// We'll use 'referenceTime' as the "end" time for scanning minute bars
	startMillis := models.Millis(startOfDay)
	endMillis := models.Millis(refLocal)

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

	// If no regular hours data found for that day, check the previous day
	yesterdayLocal := startOfDay.AddDate(0, 0, -1)
	yesterdayStart := models.Millis(yesterdayLocal)
	yesterdayEnd := models.Millis(startOfDay.Add(-time.Nanosecond))

	iter, err = GetAggsData(client, ticker, 1, "day", yesterdayStart, yesterdayEnd, 1, "desc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting previous day's data: %v", err)
	}

	for iter.Next() {
		return iter.Item().Close, nil
	}

	return 0, fmt.Errorf("no recent regular hours close found for %s", ticker)
}

// GetMostRecentExtendedHoursClose gets the most recent extended-hours close relative to referenceTime
func GetMostRecentExtendedHoursClose(client *polygon.Client, ticker string, referenceTime time.Time) (float64, error) {
	easternLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return 0, fmt.Errorf("error loading location: %v", err)
	}

	refLocal := referenceTime.In(easternLocation)
	startOfExtended := time.Date(refLocal.Year(), refLocal.Month(), refLocal.Day(), 0, 0, 0, 0, easternLocation)
	startMillis := models.Millis(startOfExtended)
	endMillis := models.Millis(refLocal)

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

	// If no regular hours data found for that day, check the previous day
	yesterdayLocal := startOfExtended.AddDate(0, 0, -1)
	yesterdayStart := models.Millis(yesterdayLocal)
	yesterdayEnd := models.Millis(startOfExtended.Add(-time.Nanosecond))

	iter, err = GetAggsData(client, ticker, 1, "day", yesterdayStart, yesterdayEnd, 1, "desc", true)
	if err != nil {
		return 0, fmt.Errorf("error getting previous day's data: %v", err)
	}

	for iter.Next() {
		// For extended hours close, we might want "Open" or "Close" from the prior day.
		// This example uses .Open, but you could revise as needed:
		return iter.Item().Open, nil
	}

	return 0, fmt.Errorf("no recent extended hours close found for %s", ticker)
}
