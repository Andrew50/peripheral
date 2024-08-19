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