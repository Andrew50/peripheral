package utils

import (
	"context"
	"fmt"
	"time"

	/*"encoding/json"
	    "net/http"
		"github.com/jackc/pgx/v4"
	    "io/ioutil"*/

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

func GetTicker(conn *Conn, securityId int, timestamp time.Time) (string, error) {
	var ticker string
	err := conn.DB.QueryRow(context.Background(), "SELECT ticker from securities where securityId = $1 and minDate <= $2 and (maxDate >= $2 or maxDate is NULL)", securityId, timestamp).Scan(&ticker)
	if err != nil {
		return "", fmt.Errorf("igw0ngb %v", err)
	}
	return ticker, nil

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

/*type TickerResponse struct {
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

func GetCIK(conn *Conn, ticker string, dateOnly string) (string, error) {
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
func GetFIGI(conn *Conn, ticker string, dateOnly string) (string, error) {
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
}*/
