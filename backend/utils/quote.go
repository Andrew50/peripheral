
package utils

import (
	"context"
	"fmt"
    "time"
	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)
func GetAggsData(client *polygon.Client, ticker string, barLength int, timeframe string,
	fromMillis models.Millis, toMillis models.Millis, limit int, resultsOrder string, isAdjusted bool) (*iter.Iter[models.Agg], error) {
	timespan := models.Timespan(timeframe)
	if resultsOrder != "asc" && resultsOrder != "desc" {
		return nil, fmt.Errorf("incorrect order string passed %s", resultsOrder)
	}
	params := models.ListAggsParams{
		Ticker:     ticker,
		Multiplier: barLength,
		Timespan:   timespan,
		From:       fromMillis,
		To:         toMillis,
	}.WithOrder(models.Order(resultsOrder)).WithLimit(limit).WithAdjusted(isAdjusted)
	iter := client.ListAggs(context.Background(), params)
	return iter, nil

}
func GetTradeAtTimestamp(client *polygon.Client, securityId int, timestamp time.Time) (models.Trade, error) {
    ticker, err := GetTicker(conn,securityId,timestamp)
	if err != nil {
		return models.Trade{}, fmt.Errorf("sif20ih %v", err)
	}
    nanoTimestamp:= models.Nanos(timestamp)
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
    ticker, err := GetTicker(conn,securityId,timestamp)
    if err != nil {
        return models.Quote{}, fmt.Errorf("doi20 %v",err)
    }
    nanoTimestamp:= models.Nanos(timestamp)
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
