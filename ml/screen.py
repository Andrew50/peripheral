import requests
import datetime
from data import getTensor

def getHistoricalTickers(conn, timestamp):
    if isinstance(timestamp, (int, float)):  # Assuming timestamp is a Unix timestamp in seconds
        timestamp = datetime.datetime.fromtimestamp(timestamp)
    with conn.db.cursor() as cursor:
        cursor.execute('''
            SELECT ticker 
            FROM securities 
            WHERE minDate <= %s 
              AND (maxDate > %s OR maxDate IS NULL)
        ''', (timestamp, timestamp))
        tickers = cursor.fetchall()
    filtered_tickers = []
    for ticker in tickers:
        filtered_tickers.append({
            "ticker": ticker[0],  # ticker[0] because fetchall() returns tuples
            "dt": timestamp,
            "label": ticker[0],
            "currentPrice": None  # Set currentPrice to None for historical data
        })
    return filtered_tickers

def getCurrentTickersAndPrice(conn):

    url = "https://api.polygon.io/v2/snapshot/locale/us/markets/stocks/tickers"
    params = {"apiKey": conn.polygon}
    response = requests.get(url, params=params)

    if response.status_code != 200:
        raise Exception(f"Failed to retrieve data: {response.text}")
    dataByTicker = response.json().get('tickers', [])
    filtered_tickers = []
    for ticker_data in dataByTicker:
        ticker = ticker_data['ticker']
        currentPrice = ticker_data.get('lastTrade', {}).get("p",None)
        filtered_tickers.append({"ticker":ticker,
                                 "dt": 0,
                                 "label": ticker,
                                 "currentPrice":currentPrice})

    return filtered_tickers


def getCurrentSecId(conn,ticker):
    query = f"SELECT securityId from securities where ticker = %s Order by maxdate is null desc,maxdate desc"
    with conn.db.cursor() as cursor:
        cursor.execute(query,(ticker,))

        val = cursor.fetchone()
        if val is not None:
            return val[0]
        else:
            return None

def filter(conn, df, metadata, setupId, setupName, threshold, dolvolReq, adrReq, mcapReq):
    # Step 1: Filter by dolvol, adr, and mcap
    filtered_metadata = []
    filtered_indices = []

    for i, meta in enumerate(metadata):
        if (meta.get('dolvol', 0) >= dolvolReq and
            meta.get('adr', 0) >= adrReq and
            meta.get('mcap', 0) >= mcapReq):
            filtered_metadata.append(meta)
            filtered_indices.append(i)

    # Step 2: Crop the df based on the filtered indices
    df = df[filtered_indices, :, :]

    # Step 3: Run the scoring as usual
    url = f"{conn.tf}/v1/models/{setupId}:predict"
    headers = {"Content-Type": "application/json"}
    payload = {
        "instances": df.tolist()  # Convert numpy array to list for JSON serialization
    }
    response = requests.post(url, json=payload, headers=headers)
    
    if response.status_code != 200:
        raise Exception(f"Failed to make prediction: {response.text}")

    result = response.json()
    scores = result.get("predictions", [])
    results = []

    # Step 4: Score filtering
    for meta, score in zip(filtered_metadata, scores):
        if score[0] * 100 >= threshold:
            secId = getCurrentSecId(conn, meta["ticker"])
            if secId:
                results.append({
                    "ticker": meta["ticker"],
                    "setupId": setupId,
                    "score": round(score[0] * 100),
                    "securityId": secId,
                    "timestamp": int(meta["timestamp"].timestamp() * 1000) if meta["timestamp"] is not 0 else 0,
                    "setup": setupName
                })
            else:
                print(f"FAILED TO GET SEC ID FOR {meta['ticker']}")
    
    return results


def screen(conn, setupIds,timestamp,threshold=25):
    tf = "1d"
    with conn.db.cursor() as cursor:
        cursor.execute('SELECT MAX(bars),MIN(dolvol),Min(adr),MIN(mcap) FROM setups WHERE setupId = ANY(%s)', (setupIds,))
        maxBars, minDolvolReq,minAdrReq,minMcapReq = cursor.fetchone()
    if timestamp == 0:
        instanceList = getCurrentTickersAndPrice(conn)
    else: 
        instanceList = getHistoricalTickers(conn,timestamp)
    data, meta = getTensor(conn, instanceList, tf, maxBars,dolvolReq=minDolvolReq,adrReq= minAdrReq,mcapReq=minMcapReq)
    results = []
    for setupId in setupIds:
        with conn.db.cursor() as cursor:
            cursor.execute('SELECT bars, name, dolvol, adr, mcap FROM setups WHERE setupId = %s', (setupId,))
            bars,setupName,dolvol,adr,mcap = cursor.fetchone()
        cropped_data = data[:, -bars:, :]  # Take the last `bars` from the data
        results += filter(conn, cropped_data, meta, setupId,setupName, threshold,dolvol,adr,mcap)
    sorted_results = sorted(results, key=lambda x: x['score'], reverse=True)

    return sorted_results
