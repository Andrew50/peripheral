'''
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