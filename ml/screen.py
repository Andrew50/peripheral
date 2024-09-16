import requests
import datetime
from data import getTensor
import numpy as np


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
            filtered_tickers.append({"ticker":ticker,
                                     "dt": datetime.datetime.now(),
                                     "label": ticker,
                                     "currentPrice":currentPrice})

    return filtered_tickers


def getCurrentSecId(conn,ticker):
    query = f"SELECT securityID from securities where ticker = %s Order by maxdate is null desc,maxdate desc"
    with conn.db.cursor() as cursor:
        cursor.execute(query,(ticker,))
        return cursor.fetchone()[0]

def filter(conn, df,tickers, setupId, threshold):
    url = f"{conn.tf}/v1/models/{setupId}:predict"
    headers = {"Content-Type": "application/json"}
    print(df.shape)
    payload = {
        "instances": df.tolist()  # Convert numpy array to list for JSON serialization
    }
    response = requests.post(url, json=payload, headers=headers)
    if response.status_code != 200:
        raise Exception(f"Failed to make prediction: {response.text}")
    scores = response.json().get("predictions", [])
    results = []
    for ticker, score in zip(tickers, scores):
        if score[0] * 100 >= threshold:
            results.append({"ticker":ticker,"setupId":setupId,"score":score[0]*100,
                            "securityId":getCurrentSecId(conn,ticker),
                            "timestamp":0})
    return results


def screen(conn, setupIds):
    adr = 3
    dolvol = 10 * 1000000
    tf = "1d"
    threshold = 1
    with conn.db.cursor() as cursor:
        cursor.execute('SELECT MAX(bars) FROM setups WHERE setupId = ANY(%s)', (setupIds,))
        maxBars = cursor.fetchone()[0]
    filteredTickerPriceList = currentTickersAndPrice(conn, dolvol, adr, 0)
    data, tickers = getTensor(conn, filteredTickerPriceList, tf, maxBars)
    results = []
    for setupId in setupIds:
        with conn.db.cursor() as cursor:
            cursor.execute('SELECT bars FROM setups WHERE setupId = %s', (setupId,))
            bars = cursor.fetchone()[0]
        cropped_data = data[-bars:, :, :]  # Take the last `bars` from the data
        results += filter(conn, cropped_data, tickers, setupId, threshold)
    sorted_results = sorted(results, key=lambda x: x['score'], reverse=True)

    return sorted_results
