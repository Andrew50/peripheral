// Package polygon provides interfaces and utilities for interacting with the Polygon.io API
// to retrieve market data, securities information, and other financial data.
package polygon

import (
	"context"
	"time"

	"backend/internal/data"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

// GetMarketStatus performs operations related to GetMarketStatus functionality.
func GetMarketStatus(conn *data.Conn) (string, error) {
	getMarketStatusResponse, err := conn.Polygon.GetMarketStatus(context.Background())
	if err != nil {
		return "", err
	}
	return getMarketStatusResponse.Market, nil
}

// ListTickers performs operations related to ListTickers functionality.
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

// AllTickers performs operations related to AllTickers functionality.
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

// AllTickersTickerOnly performs operations related to AllTickersTickerOnly functionality.
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

// GetTickerDetails performs operations related to GetTickerDetails functionality.
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

// GetTickerDetailsMarketCapShareOut fetches Polygon ticker details and returns
// the market cap and share class shares outstanding for a given ticker and
// reference date (or "now").
func GetTickerDetailsMarketCapShareOut(client *polygon.Client, ticker string, dateString string) (float64, int64, error) {
	tickerDetails, err := GetTickerDetails(client, ticker, dateString)
	if err != nil {
		return 0, 0, err
	}
	return tickerDetails.MarketCap, tickerDetails.ShareClassSharesOutstanding, nil
}

// GetPolygonIPOResult represents the listing date and the set of tickers that
// IPO'd on that date as returned by Polygon's IPO listing endpoints.
type GetPolygonIPOResult struct {
	ListingDate string
	Tickers     []string
}

// GetPolygonIPOs lists IPOs for the specified date and returns their tickers
// along with the listing date reported by Polygon.
func GetPolygonIPOs(client *polygon.Client, dateString string) (GetPolygonIPOResult, error) {
	params := models.ListIPOsParams{}.
		WithListingDate(models.EQ, dateString)
	iter := client.VX.ListIPOs(context.Background(), params)
	var listingDate string
	tickerList := []string{}
	for iter.Next() {
		listingDate = *iter.Item().ListingDate
		tickerList = append(tickerList, iter.Item().Ticker)
	}
	return GetPolygonIPOResult{
		ListingDate: listingDate,
		Tickers:     tickerList,
	}, nil
}
