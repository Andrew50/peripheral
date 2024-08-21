package main

import (
	"context"
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
func getQuote(client *polygon.Client, nanoTimestamp models.Nanos, ticker string, ord string, numResults int) *iter.Iter[models.Quote] {
	sortOrder := models.Desc
	if ord == "asc" {
		sortOrder = models.Asc
	}
	params := models.ListQuotesParams{

		Ticker: ticker,
	}.WithTimestamp(models.GTE, nanoTimestamp).
		WithSort(models.Timestamp).
		WithOrder(sortOrder).
		WithLimit(numResults)
	res := client.ListQuotes(context.Background(), params)
	return res
}
func listTickers(client *polygon.Client, dateString string, startTicker string, numTickers int) *iter.Iter[models.Ticker] {
	params := models.ListTickersParams{}.
		WithTicker(models.GTE, startTicker).
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

func main() {
	c := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")
	var marketTimeZone, tzErr = time.LoadLocation("America/New_York")
	if tzErr != nil {
		log.Fatal(tzErr)
		fmt.Print(marketTimeZone)
	}
	var ticker string = "COIN"

	fmt.Print(getLastQuote(c, ticker))
	ticker = "NVDA"
	//timestamp := models.Nanos(time.Date(2020, 3, 16, 9, 35, 0, 0, marketTimeZone))
	//getQuote(c, timestamp, ticker, "desc", 10000)
	res := listTickers(c, "2024-08-16", "A", 1000)
	for res.Next() {
		fmt.Print(res.Item())

	}

}
