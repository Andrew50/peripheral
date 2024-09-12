package data

import (
	"context"
    "backend/utils"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/jackc/pgx/v4"
	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

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
func GetTrade(client *polygon.Client, ticker string, nanoTimestamp models.Nanos, ord string, compareType models.Comparator, numResults int) *iter.Iter[models.Trade] {
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
func ListTickers(client *polygon.Client, startTicker string, dateString string, tickerStringCompareType models.Comparator, numTickers int, active bool) (*iter.Iter[models.Ticker], error) {
	params := models.ListTickersParams{}.
		WithMarket(models.AssetStocks).
		WithSort(models.TickerSymbol).
		WithLimit(numTickers).
		WithActive(active)
	if startTicker != "" {
		params = params.WithTicker(tickerStringCompareType, startTicker)
	}
	if dateString != "" {
		dt, err := time.Parse(time.DateOnly, dateString)
		if err != nil {
			return nil, err
		}
		dateObj := models.Date(dt)
		params = params.WithDate(dateObj)
	}
	iter := client.ListTickers(context.Background(), params)
	return iter, nil
}
func AllTickers(client *polygon.Client, dateString string) ([]models.Ticker, error) {
	if dateString == "" {
		dateString = time.Now().Format(time.DateOnly)
	}
	tickerList := []models.Ticker{}
	iter, err := ListTickers(client, "", dateString, models.GT, 1000, true)
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		tickerList = append(tickerList, iter.Item())
	}
	return tickerList, nil
}
func AllTickersTickerOnly(client *polygon.Client, dateString string) (*[]string, error) {
	if dateString == "" {
		dateString = time.Now().Format(time.DateOnly)
	}
	tickerList := []string{}
	iter, err := ListTickers(client, "", dateString, models.GT, 1000, true)
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		tickerList = append(tickerList, iter.Item().Ticker)
	}
	return &tickerList, nil
}

func GetTickerDetails(client *polygon.Client, ticker string, dateString string) (*models.Ticker, error) {
	var params *models.GetTickerDetailsParams
	if dateString != "now" {
		dt, err := time.Parse(time.DateOnly, dateString)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	return &res.Results, nil
}
func GetTickerNews(client *polygon.Client, ticker string, millisTime models.Millis, ord string, limit int, compareType models.Comparator) *iter.Iter[models.TickerNews] {
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
func GetLatestTickerNews(client *polygon.Client, ticker string, numResults int) *iter.Iter[models.TickerNews] {
	return GetTickerNews(client, ticker, models.Millis(time.Now()), "asc", numResults, models.LTE)
}

func GetPolygonRelatedTickers(client *polygon.Client, ticker string) ([]string, error) {
	params := &models.GetTickerRelatedCompaniesParams{
		Ticker: ticker,
	}
	res, err := client.GetTickerRelatedCompanies(context.Background(), params)
	if err != nil {
		return nil, err
	}
	relatedTickers := []string{}
	for _, relatedTicker := range res.Results {
		relatedTickers = append(relatedTickers, relatedTicker.Ticker)
	}
	return relatedTickers, nil
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
func GetAggsData(client *polygon.Client, ticker string, barLength int, timeframe string,
	fromMillis models.Millis, toMillis models.Millis, limit int, resultsOrder string) (*iter.Iter[models.Agg], error) {
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
	}.WithOrder(models.Order(resultsOrder)).WithLimit(limit)
	iter := client.ListAggs(context.Background(), params)
	return iter, nil

}
func GetTickerEvents(client *polygon.Client, id string) ([]models.TickerEventResult, error) {
	params := &models.GetTickerEventsParams{
		ID: id,
	}
	res, err := client.VX.GetTickerEvents(context.Background(), params)
	if err != nil {
		return nil, err
	}
	return res.Results, nil

}

type TickerResponse struct {
	Results []struct {
		Ticker string `json:"ticker"`
	} `json:"results"`
	Status    string `json:"status"`
	RequestID string `json:"request_id"`
	Count     int    `json:"count"`
}

func GetTickerFromCIK(client *polygon.Client, cik string) (string, error) {
	apiKey := "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm"
	url := fmt.Sprintf("https://api.polygon.io/v3/reference/tickers?cik=%s&active=true&limit=100&apiKey=%s", cik, apiKey)
	// Make the HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	// Parse the JSON response
	var tickerResponse TickerResponse
	err = json.Unmarshal(body, &tickerResponse)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	// Check if results array is not empty and return the ticker
	if len(tickerResponse.Results) > 0 {
		return tickerResponse.Results[0].Ticker, nil
	}
	// retry with active=false if no result
	resp, err = http.Get(fmt.Sprintf("https://api.polygon.io/v3/reference/tickers?cik=%s&active=false&limit=100&apiKey=%s", cik, apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	// Read the response body
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	// Parse the JSON response
	var tickerResponseInactive TickerResponse
	err = json.Unmarshal(body, &tickerResponseInactive)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Check if results array is not empty and return the ticker
	if len(tickerResponseInactive.Results) > 0 {
		return tickerResponseInactive.Results[0].Ticker, nil
	}

	// Return an error if no ticker was found
	return "", fmt.Errorf("no ticker found for CIK: %s", cik)
}

/*
	func GetTickerFromFIGI(conn *Conn, figi string, dateOnly string) (string, error) {
		// First check securities table
		var dbTicker string
		if dateOnly == "" {
			err := conn.DB.QueryRow(context.Background(), "SELECT ticker FROM securities WHERE figi = $1 AND tickerEndDate is null", figi).Scan(&dbTicker)
			if err != pgx.ErrNoRows {
				return dbTicker, nil
			}
		} else {
			err := conn.DB.QueryRow(context.Background(), "SELECT ticker FROM securities WHERE figi = $1 AND tickerStartDate <= $2 AND (tickerEndDate >= $2 OR tickerEndDate IS NULL)", figi, dateOnly).
				Scan(&dbTicker)
			if err != pgx.ErrNoRows {
				return dbTicker, nil
			}
		}

}
*/
func GetCIK(conn *utils.Conn, ticker string, dateOnly string) (string, error) {
	// First check the securities table to see if we already have the CIK associated with a ticker
	var dbCIK string
	var err error
	if dateOnly == "" {
		err = conn.DB.QueryRow(context.Background(), "SELECT cik FROM securities WHERE ticker = $1 AND tickerEndDate is null", ticker).
			Scan(&dbCIK)
	} else {
		err = conn.DB.QueryRow(context.Background(), "SELECT cik FROM securities WHERE ticker = $1 AND tickerStartDate <= $2 AND (tickerEndDate >= $2 OR tickerEndDate IS NULL)", ticker, dateOnly).
			Scan(&dbCIK)
	}
	if err != pgx.ErrNoRows {
		return dbCIK, nil
	}
	params := models.ListTickersParams{}.WithTicker(models.EQ, ticker)
	if dateOnly != "" {
		dt, err := time.Parse(time.DateOnly, dateOnly)
		if err != nil {
			return "", fmt.Errorf("function GetCIK error parsing date: {%s}, should be of format YYYY-MM-DD", dateOnly)
		}
		modelsDateObject := models.Date(dt)
		params = params.WithDate(modelsDateObject)
	}
	iter := conn.Polygon.ListTickers(context.Background(), params)
	for iter.Next() {
		if iter.Item().CIK != "" {
			return iter.Item().CIK, nil
		}
	}
	return "", fmt.Errorf("function GetCIK could not find CIK for ticker: {%s} and date: {%s}", ticker, dateOnly)
}
func GetTickerFromFIGI(conn *utils.Conn, figi string, dateOnly string) (string, error) {
	// First check securities table
	var dbTicker string
	if dateOnly == "" {
		err := conn.DB.QueryRow(context.Background(), "SELECT ticker FROM securities WHERE figi = $1 AND tickerEndDate is null", figi).Scan(&dbTicker)
		if err != pgx.ErrNoRows {
			return dbTicker, nil
		}
	} else {
		err := conn.DB.QueryRow(context.Background(), "SELECT ticker FROM securities WHERE figi = $1 AND tickerStartDate <= $2 AND (tickerEndDate >= $2 OR tickerEndDate IS NULL)", figi, dateOnly).
			Scan(&dbTicker)
		if err != pgx.ErrNoRows {
			return dbTicker, nil
		}
	}
	tickerEvents, err := GetTickerEvents(conn.Polygon, figi)
	if err != nil {
		return "", fmt.Errorf("function GetTickerFromFIGI error with GetTickerEvents; %v\n}", err)
	}
	if dateOnly == "" {
		return tickerEvents[0].Events[0].TickerChange.Ticker, nil
	}
	//dt, err := time.Parse(time.DateOnly, dateOnly)
	//if err != nil {
	//	return "", fmt.Errorf("function GetTickerFromFIGI error parsing date: {%s}", dateOnly)
	//}
	// for i, tickerEvent := range tickerEvents {
	// 	for j, event := range tickerEvent.Events {
	// 		time.
	// 		if j == len(tickerEvent.Events)-1 {
	// 			return event.TickerChange.Ticker, nil
	// 		}
	// 	}
	// }

	return "", fmt.Errorf("function GetTickerFromFIGI could not find ticker for FIGI: {%s}", figi)

}
func GetFIGI(conn *utils.Conn, ticker string, dateOnly string) (string, error) {
	// First check securities table to see if we already have the CIK associated with a ticker
	var dbFIGI string
	var err error
	if dateOnly == "" {
		err = conn.DB.QueryRow(context.Background(), "SELECT figi FROM securities WHERE ticker = $1 AND tickerEndDate is null", ticker).Scan(&dbFIGI)
	} else {
		err = conn.DB.QueryRow(context.Background(), "SELECT figi FROM securities WHERE ticker = $1 AND tickerStartDate <= $2 AND (tickerEndDate >= $2 OR tickerEndDate IS NULL)", ticker, dateOnly).
			Scan(&dbFIGI)
	}
	if err != pgx.ErrNoRows {
		return dbFIGI, nil
	}
	params := models.ListTickersParams{}.WithTicker(models.EQ, ticker)
	if dateOnly != "" {
		dt, err := time.Parse(time.DateOnly, dateOnly)
		if err != nil {
			return "", fmt.Errorf("function GetFIGI error parsing date: {%s}, should be of format YYYY-MM-DD", dateOnly)
		}
		modelsDateObject := models.Date(dt)
		params = params.WithDate(modelsDateObject)
	}
	iter := conn.Polygon.ListTickers(context.Background(), params)
	for iter.Next() {
		if iter.Item().CompositeFIGI != "" {
			return iter.Item().CompositeFIGI, nil
		}
	}
	return "", fmt.Errorf("function GetFIGI could not find FIGI for ticker: {%s} and date {%s}", ticker, dateOnly)
}
