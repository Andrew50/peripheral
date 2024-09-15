
from concurrent.futures import ThreadPoolExecutor, as_completed
from tqdm import tqdm
import requests, numpy as np, datetime, multiprocessing, sys, time

def normalize(df: np.ndarray) -> np.ndarray:
    df = np.log(df)
    close_col = np.roll(df[:, 3], shift=1)
    df = df - close_col[:, np.newaxis]
    df = df[1:]
    return df

def get_timeframe(timeframe):
    last_char = timeframe[-1]
    num = int(timeframe[:-1])
    if last_char == 'm':
        return num, 'month'
    elif last_char == 'h':
        return num, 'hour'
    elif last_char == 'd':
        return num, 'day'
    elif last_char == 'w':
        return num, 'week'
    else:
        return num, 'minute'
        #raise ValueError("Incorrect timeframe passed")


def getTensor(conn,ticker_dt_label_currentPrice_dict,tf,bars,pm=False):
    args = []
    for instance in ticker_dt_label_currentPrice_dict: #ticker,dt,label, (optional) currentPrice
        args.append([conn.polygon,instance["ticker"],instance["dt"],tf,bars,instance.get("currentPrice",None),pm])
    pool_size = 50
    ds = []  
    labels = []
    total_tasks = len(args)
    with ThreadPoolExecutor(max_workers=pool_size) as executor:
        futures = [executor.submit(getInstanceData, arg) for arg in args]
        for i, future in enumerate(as_completed(futures)):
            df = future.result()
            if not i % 20:
                print(i," / ",total_tasks)
            if df is not None:
                i = futures.index(future)
                ds.append(df)
                labels.append(ticker_dt_label_currentPrice_dict[i]["label"])

    print(f"{1 -  (len(labels) / total_tasks) * 100}% of instances failed")

    return np.array(ds,dtype=np.float64), np.array(labels)

def getInstanceData(args):
    apiKey,ticker,dt,tf,bars,currentPrice,pm = args
    end_time = dt
    multiplier, timespan = get_timeframe(tf)
    #end_time = datetime.datetime.utcfromtimestamp(timestamp)
    #TODO needs to handle no data better as it will just be zeroes in the tensor


    if timespan == 'minute':
        start_time = end_time - datetime.timedelta(minutes=bars * multiplier * 2)
    elif timespan == 'hour':
        start_time = end_time - datetime.timedelta(hours=bars * multiplier * 2)
    elif timespan == 'day':
        start_time = end_time - datetime.timedelta(days=bars * multiplier * 2)
    elif timespan == 'week':
        start_time = end_time - datetime.timedelta(days=bars * multiplier * 7 * 2)
    else:
        start_time = end_time - datetime.timedelta(days=bars * multiplier * 7 * 2)


    base_url = "https://api.polygon.io/v2/aggs/ticker/{ticker}/range/{multiplier}/{timespan}/{start}/{end}"
    url = base_url.format(ticker=ticker, multiplier=multiplier, timespan=timespan,
                          start=start_time.strftime('%Y-%m-%d'), end=end_time.strftime('%Y-%m-%d'))
    params = {"apiKey": apiKey, 
              "adjusted": "true",
              "sort":"asc"}
    response = requests.get(url, params=params)
    if response.status_code != 200:
        print(f"Failed to retrieve data for {ticker} at {dt}: {response.text}")
        return None
    stock_data = response.json()
    if 'results' not in stock_data:
        print(f"No data available for {ticker} at {dt}")
        return None
    results = stock_data['results']
    if len(results) < bars:
        return None
    data_array = np.zeros((bars,4),dtype=np.float64)
    if currentPrice is not None:
        bars -= 1 #do this becuase a new bar wil lbe added one
    for j, bar in enumerate(results[-bars:]):
        data_array[j, 0] = bar['o']  # Open
        data_array[j, 1] = bar['h']  # High
        data_array[j, 2] = bar['l']  # Low
        data_array[j, 3] = bar['c']  # Close
        #data_array[i, j, 4] = bar['v']  # Volume
    if currentPrice is not None:
        data_array[-1,:] = np.float64(currentPrice)
    else:
        data_array[-1:] = data_array[-1,0]
    data_array = normalize(data_array)
    return data_array

