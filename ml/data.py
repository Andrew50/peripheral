
from concurrent.futures import ThreadPoolExecutor, as_completed
from tqdm import tqdm
import requests, numpy as np, datetime, multiprocessing, sys, time
import asyncio
import aiohttp
import datetime

def normalize(df: np.ndarray,normType) -> np.ndarray:
    if normType == "rolling-log":
        df = np.log(df)
        close_col = np.roll(df[:, 3], shift=1)
        df = df - close_col[:,np.newaxis]
        df = df[1:]
        df = df[::-1]
        return df
    elif normType == "min-max":
        # Min-Max normalization to the range [-1, 1]
        min_vals = df.min(axis=0)
        max_vals = df.max(axis=0)
        df = 2 * (df - min_vals) / (max_vals - min_vals) - 1
        df = df[1:]     # Remove the first row to match the length
        df = df[::-1]   # Reverse the order of rows to match 'rolling-log'
        return df
    else:
        raise ValueError(f"Unknown normalization type: {normType}")

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
    #this shit is uses bandaids and only works for daily
    apiKey, ticker, dt, tf,bars, currentPrice, pm, dolvolReq, adrReq, mcapReq, normType,label = [args["polygonKey"],
    args["ticker"],args["dt"],args["tf"],args["bars"], args["currentPrice"],args["pm"],args["dolvolReq"],args["adrReq"],
        args["mcapReq"],args["normalize"],args.get("label",None)]
    if dt == 0:
        end_time = datetime.datetime.now() #- datetime.timedelta(days=1)
    else:
        end_time = dt
    multiplier, timespan = get_timeframe(tf)
    bars += 1 #normalizaton will steal a bar
 
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
        try:
            stock_data = await response.json()
        except:
            return None
        if 'results' not in stock_data:
            return None
        results = stock_data['results']
        if len(results) < bars:
            return None
        data_array = np.zeros((bars, 4))
        if dt == 0:
            bars -= 1  # Adjust because a new bar will be added
        
        dolvol = None
        adr = None
        if dolvolReq is not None or adrReq is not None:
            recent_bars = results[-20:] if len(results) >= 20 else results
            total_volume = 0
            total_range = 0
            for bar in recent_bars:
                high = bar['h']
                low = bar['l']
                close = bar['c']
                volume = bar['v']
                total_volume += close * volume
                total_range += high / low
            dolvol = total_volume / len(recent_bars)
            adr = (total_range / len(recent_bars) - 1) * 100
            if adr < adrReq or dolvol < dolvolReq:
                return None
        mcap = None
        if mcapReq is not None:
            mcap_url = f"https://api.polygon.io/v3/reference/tickers/{ticker}?apiKey={apiKey}"
            async with session.get(mcap_url) as mcap_response:
                if mcap_response.status != 200:
                    return None
                mcap_data = await mcap_response.json()
                if 'results' not in mcap_data or 'market_cap' not in mcap_data['results']:
                    return None
                mcap = mcap_data['results']['market_cap']
                if mcap < mcapReq:
                    return None
        for j, bar in enumerate(results[-bars:]):
            data_array[j, 0] = bar['o']  # Open
            data_array[j, 1] = bar['h']  # High
            data_array[j, 2] = bar['l']  # Low
            data_array[j, 3] = bar['c']  # Close
        if dt == 0: #current
            if currentPrice is not None and currentPrice != 0:
                data_array[-1, :] = np.float64(currentPrice)
            else:
                data_array[-1,:] = data_array[-2, 0]
        else: #historical
            data_array[-1,:] = data_array[-1, 0]
        data_array = normalize(data_array,normType)
        return data_array, {"ticker":ticker,"timestamp":dt,"dolvol":dolvol,"adr":adr,"mcap":mcap,"label":label}

async def async_get_tensor(conn, ticker_dt_label_currentPrice_dict, tf, bars, pm,normalize,dolvolReq,adrReq,mcapReq):
    args = [{"tf":tf,"bars":bars,"pm":pm,"polygonKey":conn.polygon,"ticker":instance["ticker"],"normalize":normalize,
             "dt":instance["dt"],"label":instance["label"],"currentPrice":instance.get("currentPrice",None),
             "dolvolReq":dolvolReq,"adrReq":adrReq,"mcapReq":mcapReq
             } for instance in ticker_dt_label_currentPrice_dict ]
    
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

        return ds.astype(np.float32), labels

# This function can now be called synchronously
def getTensor(conn, ticker_dt_label_currentPrice_dict, tf, bars, pm=False,normalize="rolling-log",dolvolReq=None,adrReq=None,mcapReq=None):
    return asyncio.run(async_get_tensor(conn, ticker_dt_label_currentPrice_dict, tf, bars, pm,normalize,dolvolReq,adrReq,mcapReq))
