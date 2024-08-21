package main

import (
    "github.com/polygon-io/client-go/rest/models"
    polygon "github.com/polygon-io/client-go/rest"
    "context"
    "fmt"
    "log"
)

const apiKey = "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm"



type relatedShit struct {
    markets []int,
    sectors []int,
    stocks []int,
}
type instance struct {
    securityId int,
    timestamp int,
    setupId int,
    relatedTickers relatedShit,
    entry string,
}





func relatedTickers (ticker string) ([]string, error) {
	c := polygon.New(apiKey)
	params := models.GetTickerRelatedCompaniesParams{
		Ticker: ticker,
	}
	r, err := c.GetTickerRelatedCompanies(context.Background(), &params)
    if err != nil {
        return nil, err
    }
    res := r.Results
    tickers := make ([]string, len(res))
    for i, comp := range res {
        tickers[i] = comp.Ticker
    }
    return tickers, err
}
    
    
func main() {

    var ticker string
    for {
    fmt.Printf("input ticker: ")
    fmt.Scanf("%s", &ticker)
    results, err := relatedTickers(ticker)
    if err != nil {
        log.Fatal(err)
    }
    for _, tick := range results {
        fmt.Print(tick)
        fmt.Println()
    }
}
}
