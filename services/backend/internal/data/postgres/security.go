package postgres

import (
	"context"
	"fmt"
	"time"

	"backend/internal/data"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

// GetSecurityID performs operations related to GetSecurityID functionality.
func GetSecurityID(conn *data.Conn, ticker string, timestamp time.Time) (int, error) {
	var securityID int
	err := conn.DB.QueryRow(context.Background(), "SELECT securityId from securities where ticker = $1 and minDate <= $2 and (maxDate >= $2 or maxDate is NULL)", ticker, timestamp).Scan(&securityID)
	if err != nil {
		return 0, fmt.Errorf("43333ngb %v %v %v", err, ticker, timestamp)
	}
	return securityID, nil
}

// GetTicker performs operations related to GetTicker functionality.
func GetTicker(conn *data.Conn, securityID int, timestamp time.Time) (string, error) {
	var ticker string
	err := conn.DB.QueryRow(context.Background(), "SELECT ticker from securities where securityId = $1 and minDate <= $2 and (maxDate >= $2 or maxDate is NULL)", securityID, timestamp).Scan(&ticker)
	if err != nil {
		return "", fmt.Errorf("igw0ngb %v", err)
	}
	return ticker, nil
}

// GetCIKFromTicker performs operations related to GetCIKFromTicker functionality.
func GetCIKFromTicker(conn *data.Conn, ticker string, timestamp time.Time) (int64, error) {
	var cik int64
	err := conn.DB.QueryRow(context.Background(), "SELECT cik from securities where ticker = $1 and minDate <= $2 and (maxDate >= $2 or maxDate is NULL)", ticker, timestamp).Scan(&cik)
	if err != nil {
		return 0, fmt.Errorf("3333w0ngb %v", err)
	}
	return cik, nil
}

// GetTickerNews performs operations related to GetTickerNews functionality.
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

// GetLatestTickerNews performs operations related to GetLatestTickerNews functionality.
func GetLatestTickerNews(client *polygon.Client, ticker string, numResults int) *iter.Iter[models.TickerNews] {
	return GetTickerNews(client, ticker, models.Millis(time.Now()), "asc", numResults, models.LTE)
}

// GetPolygonRelatedTickers performs operations related to GetPolygonRelatedTickers functionality.
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

// GetTickerEvents performs operations related to GetTickerEvents functionality.
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
