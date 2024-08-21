package data
/*'''
m := AssetClass("stocks")
limit * int := 1000
listParams := &models.ListTickersParams{
	Market: &m,
	Limit:  &limit,
}
tickerIter := c.ListTickers(context.Background(), listParams)

for tickerIter.Next() {

	fmt.Print(tickerIter.Item())
}'''


// init client
c := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")
fmt.Print("Hello")
parameters := &models.ListAggsParams{
	Ticker:     "COIN",
	Multiplier: 5,
	Timespan:   "minute",
	To:         models.Millis(time.Date(2024, 8, 16, 0, 0, 0, 0, time.UTC)),
	From:       models.Millis(time.Date(2024, 8, 15, 0, 0, 0, 0, time.UTC)),
}

iter := c.ListAggs(context.Background(), parameters)

for iter.Next() {

	// Convert Millis back to time.Time and then format it
	timeValue := time.Time(iter.Item().Timestamp)
	fmt.Println(timeValue.Format(time.RFC3339))

}



// websocket 

c, err := polygonws.New(polygonws.Config{
	APIKey: "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm",
	Feed:   polygonws.RealTime,
	Market: polygonws.Stocks,
})
if err != nil {

	log.Fatal(err)
}
defer c.Close()
if err := c.Connect(); err != nil {
	log.Fatal(err)
	return
}
if err := c.Subscribe(polygonws.StocksTrades, "SMCI"); err != nil {
	log.Fatal(err)
}
for {
	select {
	case err := <-c.Error(): // check for any fatal errors (e.g. auth failed)
		log.Fatal(err)
	case out, more := <-c.Output(): // read the next data message
		if !more {
			return
		}

		switch out.(type) {
		case models.EquityQuote:
			log.Print(out) // do something with the agg
		case models.EquityTrade:
			log.Print(out) // do something with the trade
		}
	}
}

// all tickers snapshot

package main

import (
	"context"
	"log"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
)

func main() {
	c := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")

	params := models.GetAllTickersSnapshotParams{

		Locale:     models.US,
		MarketType: models.Stocks,
	}.WithTickers("")
	res, err := c.GetAllTickersSnapshot(context.Background(), params)
	if err != nil {

		log.Fatal(err)
	}
	log.Print(res)

}
*/
