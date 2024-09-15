import requests


def historicalTickersAndPrice(conn,dolvolReq,adrRewq,mcapReq):
    return

def currentTickersAndPrice(conn, dolvolReq, adrReq, mcapReq):

    url = "https://api.polygon.io/v2/snapshot/locale/us/markets/stocks/tickers"
    params = {"apiKey": conn.polygon}
    response = requests.get(url, params=params)

    if response.status_code != 200:
        raise Exception(f"Failed to retrieve data: {response.text}")
    dataByTicker = response.json().get('tickers', [])
    filtered_tickers = []
    for ticker_data in dataByTicker:
        ticker = ticker_data['ticker']
        print(ticker)
        #market_cap = ticker_data.get('market_cap', 0)
        mcap = 1000** 1000
        prevDailyCandle = ticker_data.get('prevDay', {})
        close = prevDailyCandle.get('c', 0)
        currentPrice = ticker_data.get('lastTrade', {}).get("p",close)
        volume = prevDailyCandle.get('v', 0)
        high = prevDailyCandle.get('h', 0)
        low = prevDailyCandle.get('l', 0)
        dolvol = close * volume
        adr = (high / low - 1) * 100 if low != 0 else 0
        if dolvol>= dolvolReq and adr>= adrReq and mcap>= mcapReq:
            filtered_tickers.append([ticker,currentPrice])

    return filtered_tickers



def screen(conn,setupIds):
    filteredTickerPriceList = (currentTickersAndPrice(conn,100000000,2,9))
    data, tickers = getData(conn,filteredTickerPriceList)
    return None



