package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

const apiKey = "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm"

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
		WithTicker(tickerStringCompareType, startTicker).
		WithMarket(models.AssetStocks).
		WithSort(models.TickerSymbol).
		WithLimit(numTickers)
	if dateString != "now" {
		dt, err := time.Parse(time.DateOnly, dateString)
		fmt.Print(dt)
		if err != nil {
			log.Fatal(err)
		}
		dateObj := models.Date(dt)
		params = params.WithDate(dateObj)
	}
	res := client.ListTickers(context.Background(), params)
	return res
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

func main() {

	c := polygon.New(apiKey)

	// test getLastQuote()
	var ticker string = "COIN"
	fmt.Print(getLastQuote(c, ticker))

	ticker = "NVDA"
	var marketTimeZone, tzErr = time.LoadLocation("America/New_York")
	if tzErr != nil {
		log.Fatal(tzErr)
		fmt.Print(marketTimeZone)
	}
	timestamp := models.Nanos(time.Date(2020, 3, 16, 9, 35, 0, 0, marketTimeZone))
	fmt.Print(time.Time(timestamp))
	//getQuote(c, timestamp, ticker, "desc", 10000)

	// test listTickers()
	res := listTickers(c, "A", "2024-08-16", models.GTE, 1000)
	for res.Next() {

	}
	// test tickerDetails()
	tickerDetailsRes := tickerDetails(c, "COIN", "2024-08-16")
	fmt.Print(tickerDetailsRes)

	// test getTickerNews()
	tickerNews := getTickerNews(c, "SBUX", millisFromDatetimeString("2024-08-13 09:30:00"), "desc", 10, models.GTE)
	for tickerNews.Next() {
		fmt.Print(tickerNews.Item())
	}

	// test getRelatedTickers()
	relatedTickers := getRelatedTickers(c, ticker)
	fmt.Print(relatedTickers)

}
