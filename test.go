package main

import (
	"context"
	"log"
    "fmt"
	"time"
	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
)

func main() {

    var ticker string
    var dt string
    var tf string
	c := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")
    for {
        fmt.Print("Ticker: ")
        _, err := fmt.Scan(&ticker)
        if err != nil {
            break
        }
        fmt.Print("Datetime: ")
        _, err = fmt.Scan(&dt)
        if err != nil {
            break
        }

        fmt.Print("Timeframe: ")
        _, err = fmt.Scan(&tf)
        if err != nil {
            break
        }
        params := &models.ListAggsParams{
            Ticker: ticker,
            Multiplier: 5,
            Timespan: models.Timespan(tf),
            From: models.Millis(time.Date(2022,8,1,16,0,0,0,time.UTC)),
            To: models.Millis(time.Date(2023,8,1,16,0,0,0,time.UTC)),
        }

        iter := c.ListAggs(context.Background(), params)

        for iter.Next() {
            row := iter.Item()
            fmt.Print(row.Close)
            fmt.Print(" ")
        }
        if iter.Err() != nil {
            log.Fatal(iter.Err())
        }
        fmt.Println()
    }


}
