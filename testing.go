package main

import (
	"log"

	polygonws "github.com/polygon-io/client-go/websocket"
	"github.com/polygon-io/client-go/websocket/models"
)

func main() {
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
}
