package data

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

func getLastQuote(client *polygon.Client, ticker string) models.LastQuote {

	params := &models.GetLastQuoteParams{
		Ticker: ticker,
	}
	res, err := client.GetLastQuote(context.Background(), params)
	if err != nil {
		log.Fatal(err)
	}
	return res.Results

}
func getQuote(client *polygon.Client, ticker string, nanoTimestamp models.Nanos, ord string, compareType models.Comparator, numResults int) *iter.Iter[models.Quote] {
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
func getLastTrade(client *polygon.Client, ticker string) models.LastTrade {
	params := &models.GetLastTradeParams{
		Ticker: ticker,
	}
	res, err := client.GetLastTrade(context.Background(), params)
	if err != nil {
		log.Fatal(err)
	}
	return res.Results

}
func getTrade(client *polygon.Client, ticker string, nanoTimestamp models.Nanos, ord string, compareType models.Comparator, numResults int) *iter.Iter[models.Trade] {
	sortOrder := models.Desc
	if ord == "asc" {
		sortOrder = models.Asc
	}
	params := models.ListTradesParams{
		Ticker: ticker,
	}.WithTimestamp(compareType, nanoTimestamp).
		WithOrder(sortOrder).
		WithLimit(numResults)
	return client.ListTrades(context.Background(), params)
}

// QA STATUS: not QA'd
// create function listAllTickers(dateString string) that calls several times to listTickers
func listTickers(client *polygon.Client, startTicker string, dateString string, tickerStringCompareType models.Comparator, numTickers int) *iter.Iter[models.Ticker] {
	params := models.ListTickersParams{}.
		WithMarket(models.AssetStocks).
		WithSort(models.TickerSymbol).
		WithLimit(numTickers)
	if startTicker != "" {
		params = params.WithTicker(tickerStringCompareType, startTicker)
	}
	if dateString != "now" {
		dt, err := time.Parse(time.DateOnly, dateString)
		if err != nil {
			log.Fatal(err)
		}
		dateObj := models.Date(dt)
		params = params.WithDate(dateObj)
	}
	iter := client.ListTickers(context.Background(), params)
	return iter
}
func AllTickers(client *polygon.Client, dateString string) []models.Ticker {
	tickerList := []models.Ticker{}
	iter := listTickers(client, "", dateString, models.GT, 1000)
	for iter.Next() {
		tickerList = append(tickerList, iter.Item())
	}
	return tickerList
}
func AllTickersTickerOnly(client *polygon.Client, dateString string) *[]string {
	tickerList := []string{}
	st := time.Now()
	iter := listTickers(client, "", dateString, models.GT, 1000)
	fmt.Println(time.Since(st))
	start := time.Now()
	for iter.Next() {
		tickerList = append(tickerList, iter.Item().Ticker)
	}
	fmt.Println(time.Since(start))
	return &tickerList
}

func tickerDetails(client *polygon.Client, ticker string, dateString string) *models.Ticker {
	var params *models.GetTickerDetailsParams
	if dateString != "now" {
		dt, err := time.Parse(time.DateOnly, dateString)
		if err != nil {
			log.Fatal(err)
		}
		params = models.GetTickerDetailsParams{
			Ticker: ticker,
		}.WithDate(models.Date(dt))
	} else {
		params = &models.GetTickerDetailsParams{
			Ticker: ticker,
		}
	}
	res, err := client.GetTickerDetails(context.Background(), params)
	if err != nil {
		log.Fatal(err)
	}
	return &res.Results
}
func getTickerNews(client *polygon.Client, ticker string, millisTime models.Millis, ord string, limit int, compareType models.Comparator) *iter.Iter[models.TickerNews] {
	sortOrder := models.Asc
	if ord == "desc" {
		sortOrder = models.Desc
	}
	params := models.ListTickerNewsParams{}.
		WithTicker(models.EQ, ticker).
		WithSort(models.PublishedUTC).
		WithOrder(sortOrder).
		WithLimit(limit).
		WithPublishedUTC(compareType, millisTime)
	iter := client.ListTickerNews(context.Background(), params)
	return iter

}
func getLatestTickerNews(client *polygon.Client, ticker string, numResults int) *iter.Iter[models.TickerNews] {
	return getTickerNews(client, ticker, models.Millis(time.Now()), "asc", numResults, models.LTE)
}

func getRelatedTickers(client *polygon.Client, ticker string) *[]string {
	params := &models.GetTickerRelatedCompaniesParams{
		Ticker: ticker,
	}
	res, err := client.GetTickerRelatedCompanies(context.Background(), params)
	if err != nil {
		log.Fatal(err)
	}
	relatedTickers := []string{}
	for _, relatedTicker := range res.Results {
		relatedTickers = append(relatedTickers, relatedTicker.Ticker)
	}
	return &relatedTickers
}

// func relatedTickers (ticker string) ([]string, error) {
// 	c := polygon.New(apiKey)
// 	params := models.GetTickerRelatedCompaniesParams{
// 		Ticker: ticker,
// 	}
// 	r, err := c.GetTickerRelatedCompanies(context.Background(), &params)
// 	if err != nil {
// 		return nil, err
// 	}
// 	res := r.Results
// 	tickers := make([]string, len(res))
// 	for i, comp := range res {
// 		tickers[i] = comp.Ticker
// 	}
// 	return tickers, err
// }

// QA STATUS: NEEDS TESTING
func getAggsData(client *polygon.Client, ticker string, barLength int, timeframe string,
	fromMillis models.Millis, toMillis models.Millis, limit int) *iter.Iter[models.Agg] {
	timespan := models.Timespan(timeframe)
	params := models.ListAggsParams{
		Ticker:     ticker,
		Multiplier: barLength,
		Timespan:   timespan,
		From:       fromMillis,
		To:         toMillis,
	}.WithOrder(models.Asc).WithLimit(limit)
	iter := client.ListAggs(context.Background(), params)

	return iter

}
func millisFromDatetimeString(datetime string) models.Millis {
	layouts := []string{
		time.DateTime,
		time.DateOnly,
	}
	for _, layout := range layouts {
		if dt, err := time.Parse(layout, datetime); err == nil {
			easternTimeLocation, tzErr := time.LoadLocation("America/New_York")
			if tzErr != nil {
				log.Fatal(tzErr)
			}
			return models.Millis(dt.In(easternTimeLocation))
		}
	}
	log.Fatal(errors.New("invalid datetime string"))
	return models.Millis(time.Now())

}
func nanosFromDatetimeString(datetime string) models.Nanos {
	layouts := []string{
		time.RFC3339Nano,
		time.DateTime,
	}
	for _, layout := range layouts {
		if dt, err := time.Parse(layout, datetime); err == nil {
			easternTimeLocation, tzErr := time.LoadLocation("America/New_York")
			if tzErr != nil {
				fmt.Print("eastern timezone error")
			}
			return models.Nanos(dt.In(easternTimeLocation))
		}
	}
	log.Fatal(errors.New("invalid datetime string"))
	return models.Nanos(time.Now())
}
func getTickerFromCIK(client *polygon.Client, cik int) string {
	params := models.ListTickersParams{}.WithCIK(cik)
	iter := client.ListTickers(context.Background(), params)
	for iter.Next() {

	}
	return iter.Item().Ticker
}
func getCIK(client *polygon.Client, ticker string) int {
	params := models.ListTickersParams{}.WithTicker(models.EQ, ticker)
	iter := client.ListTickers(context.Background(), params)
	for iter.Next() {
		sfsaf
	}
	cik, err := strconv.Atoi(iter.Item().CIK)
	if err != nil {
		log.Fatal("error retreiving cik")
	}
	return cik
}
