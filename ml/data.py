
from concurrent.futures import ThreadPoolExecutor, as_completed
from tqdm import tqdm
import requests, numpy as np, datetime, multiprocessing, sys, time
import asyncio
import aiohttp
import numpy as np
import datetime

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



async def get_instance_data(session, args):
    apiKey, ticker, dt, tf, label,bars, currentPrice, pm = args
    end_time = dt
    multiplier, timespan = get_timeframe(tf)

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
    params = {
        "apiKey": apiKey,
        "adjusted": "true",
        "sort": "asc"
    }

    async with session.get(url, params=params) as response:
        if response.status != 200:
            return None

        stock_data = await response.json()
        if 'results' not in stock_data:
            return None

        results = stock_data['results']
        if len(results) < bars:
            return None

        data_array = np.zeros((bars, 4))
        if currentPrice is not None:
            bars -= 1  # Adjust because a new bar will be added

        for j, bar in enumerate(results[-bars:]):
            data_array[j, 0] = bar['o']  # Open
            data_array[j, 1] = bar['h']  # High
            data_array[j, 2] = bar['l']  # Low
            data_array[j, 3] = bar['c']  # Close

        if currentPrice is not None:
            data_array[-1, :] = np.float64(currentPrice)
        else:
            data_array[-1:] = data_array[-1, 0]

        data_array = normalize(data_array)
        return data_array, label

async def async_get_tensor(conn, ticker_dt_label_currentPrice_dict, tf, bars, pm=False):
    args = [
        [conn.polygon, instance["ticker"], instance["dt"], tf, instance["label"], bars, instance.get("currentPrice", None), pm]
        for instance in ticker_dt_label_currentPrice_dict
    ]
    
    async with aiohttp.ClientSession() as session:
        tasks = [get_instance_data(session, arg) for arg in args]
        ds = []
        labels = []
        total_tasks = len(tasks)

        for future in asyncio.as_completed(tasks):
            res = await future
            if res is not None:
                df, label = res
                ds.append(df)
                labels.append(label)

        print(f"{((total_tasks - len(labels)) / total_tasks) * 100}% of instances failed")

        ds = np.array(ds)
        if not np.isfinite(ds).all():
            ds = np.nan_to_num(ds, nan=0.0, posinf=0.0, neginf=0.0)
            print("Bad values corrected")

        return ds.astype(np.float32), labels

# This function can now be called synchronously
def getTensor(conn, ticker_dt_label_currentPrice_dict, tf, bars, pm=False):
    return asyncio.run(async_get_tensor(conn, ticker_dt_label_currentPrice_dict, tf, bars, pm))
