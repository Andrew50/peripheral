package data

import (
	//"fmt"
	//"github.com/polygon-io/client-go/rest/models"
	"fmt"
	"log"
	//
)

func ManualTest() {
	conn, close := InitConn(false)
	defer close()
	ticker := GetTickerDetails(conn.Polygon, "IFN.WD", "2003-09-23")
	fmt.Printf("Ticker suffix: {%s}", ticker.TickerSuffix)
	err := initTickerDatabase(conn)
	if err != nil {
		log.Fatal(err)
	}

	/*
		iter := ListTickers(conn.Polygon, "", "", models.GTE, 1000, true)
		ccc := 0
		for iter.Next() {
			ccc++
			if iter.Item().Ticker == "FB" {
				fmt.Println(iter.Item().Ticker)
			}
		}
		fmt.Println(ccc)*/
	//updateTickerDatabase(conn, "")

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
