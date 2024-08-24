package data

import (
	"fmt"
	"log"

	polygon "github.com/polygon-io/client-go/rest"
)

func BenTest() {
	client := polygon.New("ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")
	res, err := GetTickerFromCIK(client, "0001679788")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(res)

	//fmt.Println(AllTickersTickerOnly(c, "2024-08-20")[0])
	// // test getLastQuote()
	// var ticker string = "COIN"
	// fmt.Print(getLastQuote(c, ticker))

	// ticker = "NVDA"
	// var marketTimeZone, tzErr = time.LoadLocation("America/New_York")
	// if tzErr != nil {
	// 	log.Fatal(tzErr)
	// 	fmt.Print(marketTimeZone)
	// }
	// timestamp := models.Nanos(time.Date(2020, 3, 16, 9, 35, 0, 0, marketTimeZone))
	// fmt.Print(time.Time(timestamp))
	// //getQuote(c, timestamp, ticker, "desc", 10000)

	// // test listTickers()
	// res := listTickers(c, "A", "2024-08-16", models.GTE, 1000)
	// for res.Next() {

	// }
	// // test tickerDetails()
	// tickerDetailsRes := tickerDetails(c, "COIN", "2024-08-16")
	// fmt.Print(tickerDetailsRes)

	// // test getTickerNews()
	// tickerNews := getTickerNews(c, "SBUX", millisFromDatetimeString("2024-08-13 09:30:00"), "desc", 10, models.GTE)
	// for tickerNews.Next() {
	// 	fmt.Print(tickerNews.Item())
	// }

	// // test getRelatedTickers()
	// relatedTickers := getRelatedTickers(c, ticker)
	// fmt.Print(relatedTickers)

}
