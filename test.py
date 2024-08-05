from polygon import RESTClient 

if __name__ == '__main__':
    client = RESTClient(api_key="ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm")

    ticker = "COIN"

    aggs = []
    for a in client.list_aggs(ticker=ticker, multiplier=1, timespan='minute', from_="2023-01-01", to="2023-06-13", limit=50000):
        aggs.append(a)

    print(aggs)

