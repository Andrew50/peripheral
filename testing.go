package main

import (
	"context"
	"fmt"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
)

func main() {

	// init client
	c := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")
	fmt.Print("Hello")
	parameters := &models.ListAggsParams{
		Ticker:     "COIN",
		Multiplier: 5,
		Timespan:   "day",
		To:         models.Millis(time.Date(2024, 5, 22, 0, 0, 0, 0, time.UTC)),
		From:       models.Millis(time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC)),
	}

	iter := c.ListAggs(context.Background(), parameters)

	for iter.Next() {

		fmt.Print(iter.Item())
	}
	m := AssetClass("stocks")
	limit * int := 1000
	listParams := &models.ListTickersParams{
		Market: &m,
		Limit:  &limit,
	}
	tickerIter := c.ListTickers(context.Background(), listParams)

	for tickerIter.Next() {

		fmt.Print(tickerIter.Item())
	}

}
