package main

import (
	"context"
	"log"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
)

func main() {

	// init client
	c := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")

	params := models.GetTickerDetailsParams{
		Ticker: "AAPL",
	}.WithDate(models.Date(time.Date(2021, 7, 22, 0, 0, 0, 0, time.Local)))

	res, err := c.GetTickerDetails(context.Background(), params)
	if err != nil {
		log.Fatal(err)
	}
	log.Print(res) // do something with the result

}
