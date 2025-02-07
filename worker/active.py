import numpy as np
import json
from datetime import datetime, timedelta
from conn import Conn
from data import getTensor

# Constants for list lengths
TOP_N = 50             # Number of top/bottom items to show per metric
TOP_CONSTITUENTS = 10  # Number of constituents to show per sector/industry

def get_timeframe_days(timeframe):
    if timeframe == "1 day":
        return 1
    elif timeframe == "1 week":
        return 7
    elif timeframe == "1 month":
        return 30
    elif timeframe == "6 month":
        return 180
    elif timeframe == "1 year":
        return 365
    else:
        raise ValueError("Unsupported timeframe")

def calculate_gap(tensor, idx):
    """Calculate the gap between the previous close and current open for a given stock tensor.
       Assumes that tensor[idx] has at least two rows.
    """
    if len(tensor[idx]) < 2:
        return 0
    prev_close = tensor[idx][-2][3]  # Previous day's close
    curr_open = tensor[idx][-1][0]   # Current day's open
    return ((curr_open - prev_close) / prev_close) * 100

def calculate_active(data):
    # Get all active securities from the database
    with data.db.cursor() as cursor:
        cursor.execute("""
            SELECT ticker, sector, industry, securityId FROM securities 
            WHERE maxDate is NULL
        """)
        tickers = cursor.fetchall()
    
    print(f"Processing {len(tickers)} active securities")
    
    # Get tensor data for all stocks for ~1 year (270 trading days)
    instances = [{"ticker": t[0], "dt": 0} for t in tickers]
    tensor, labels = getTensor(data, instances, "1d", 270, normalize="none")
    
    # Build a lookup to map ticker to its index in the tensor array
    ticker_to_idx = {label["ticker"]: i for i, label in enumerate(labels)}
    
    timeframes = ["1 day", "1 week", "1 month", "6 month", "1 year"]
    groups = ["stock", "sector", "industry"]
    
    # Loop over each timeframe
    for timeframe in timeframes:
        print(f"\nProcessing {timeframe} timeframe...")
        days = get_timeframe_days(timeframe)
        lookback_bars = min(days, 270)
        
        # Initialize results for each group and metric.
        # For stocks we store individual values; for sectors/industries we will compute an aggregate.
        group_results = {
            "stock": {"price": [], "volume": [], "gap": []},
            "sector": {"price": [], "volume": [], "gap": []},
            "industry": {"price": [], "volume": [], "gap": []}
        }
        
        # For sector and industry, track the constituent stocks by metric.
        # Each entry is a dict with keys "ticker", "securityId", and "value".
        sector_constituents = {}
        industry_constituents = {}
        
        # Loop over each security and compute metrics
        for i, label in enumerate(labels):
            if i > 0 and i % 100 == 0:
                print(f"Processed {i}/{len(labels)} securities...")
            
            ticker = label["ticker"]
            # Look up the security details from the original tickers list
            security = next((t for t in tickers if t[0] == ticker), None)
            if not security:
                continue
                
            _, sector, industry, securityId = security
            
            idx = ticker_to_idx.get(ticker)
            if idx is None or idx >= len(tensor) or len(tensor[idx]) < lookback_bars:
                continue

            # Calculate price return in percentage over the lookback period
            current_price = tensor[idx][-1][3]        # current close
            past_price = tensor[idx][-lookback_bars][3] # past close
            print(f"timeframe: {timeframe}, ticker: {ticker}, current_price: {current_price}, past_price: {past_price}")
            if past_price == 0:
                continue  # avoid division by zero
            price_val = ((current_price - past_price) / past_price) * 100
            
            # Compute a volume metric (as provided in the original code)
            # Note: this metric multiplies close by open; adjust if needed.
            volume_segment = tensor[idx][-lookback_bars:]
            volume_val = np.sum(volume_segment[:, 4])
            
            # Only calculate gap if timeframe is "1 day"
            gap_val = calculate_gap(tensor, idx) if timeframe == "1 day" else None

            # Stock-level metrics (each stock is a unique entry)
            group_results["stock"]["price"].append((ticker, securityId, price_val))
            group_results["stock"]["volume"].append((ticker, securityId, volume_val))
            if timeframe == "1 day":
                group_results["stock"]["gap"].append((ticker, securityId, gap_val))
            
            # For sector group: only add if sector is known
            if sector and sector != "Unknown":
                if sector not in sector_constituents:
                    sector_constituents[sector] = {"price": [], "volume": [], "gap": []}
                sector_constituents[sector]["price"].append({
                    "ticker": ticker,
                    "securityId": securityId,
                    "value": price_val
                })
                sector_constituents[sector]["volume"].append({
                    "ticker": ticker,
                    "securityId": securityId,
                    "value": volume_val
                })
                if timeframe == "1 day":
                    sector_constituents[sector]["gap"].append({
                        "ticker": ticker,
                        "securityId": securityId,
                        "value": gap_val
                    })
            
            # For industry group: only add if industry is known
            if industry and industry != "Unknown":
                if industry not in industry_constituents:
                    industry_constituents[industry] = {"price": [], "volume": [], "gap": []}
                industry_constituents[industry]["price"].append({
                    "ticker": ticker,
                    "securityId": securityId,
                    "value": price_val
                })
                industry_constituents[industry]["volume"].append({
                    "ticker": ticker,
                    "securityId": securityId,
                    "value": volume_val
                })
                if timeframe == "1 day":
                    industry_constituents[industry]["gap"].append({
                        "ticker": ticker,
                        "securityId": securityId,
                        "value": gap_val
                    })
        
        # Now aggregate the sector and industry metrics from their constituents.
        for sector, metrics in sector_constituents.items():
            # For "price" metric
            if metrics["price"]:
                avg_price = sum(item["value"] for item in metrics["price"]) / len(metrics["price"])
                group_results["sector"]["price"].append((sector, None, avg_price))
            # For "volume" metric
            if metrics["volume"]:
                avg_volume = sum(item["value"] for item in metrics["volume"]) / len(metrics["volume"])
                group_results["sector"]["volume"].append((sector, None, avg_volume))
            # For "gap" metric (only if applicable)
            if timeframe == "1 day" and metrics["gap"]:
                avg_gap = sum(item["value"] for item in metrics["gap"]) / len(metrics["gap"])
                group_results["sector"]["gap"].append((sector, None, avg_gap))
        
        for industry, metrics in industry_constituents.items():
            if metrics["price"]:
                avg_price = sum(item["value"] for item in metrics["price"]) / len(metrics["price"])
                group_results["industry"]["price"].append((industry, None, avg_price))
            if metrics["volume"]:
                avg_volume = sum(item["value"] for item in metrics["volume"]) / len(metrics["volume"])
                group_results["industry"]["volume"].append((industry, None, avg_volume))
            if timeframe == "1 day" and metrics["gap"]:
                avg_gap = sum(item["value"] for item in metrics["gap"]) / len(metrics["gap"])
                group_results["industry"]["gap"].append((industry, None, avg_gap))
        
        print(f"Storing results for {timeframe}...")
        # For each group and metric, sort and then store leaders and laggards
        for group in groups:
            for metric_type in ["price", "volume", "gap"]:
                # Only process gap for 1 day timeframe
                if metric_type == "gap" and timeframe != "1 day":
                    continue
                    
                values = group_results[group][metric_type]
                if not values:
                    continue

                # Sort by the metric value (the third element in the tuple)
                values.sort(key=lambda x: x[2])
                
                # Determine leaders (highest values) and laggards (lowest values)
                leaders = values[-TOP_N:]
                laggards = values[:TOP_N]
                
                # Build results for leaders
                result_list = []
                for entry in leaders:
                    name, sid, agg_value = entry
                    if group == "stock":
                        # For stocks, we simply output ticker and securityId
                        result = {
                            "ticker": name,
                            "securityId": sid
                        }
                    else:
                        # For sectors/industries, get the list of constituent stocks
                        if group == "sector":
                            constituents_data = sector_constituents.get(name, {}).get(metric_type, [])
                        else:  # group == "industry"
                            constituents_data = industry_constituents.get(name, {}).get(metric_type, [])
                        # Sort constituents descending for leader lists
                        sorted_constituents = sorted(constituents_data, key=lambda x: x["value"], reverse=True)
                        constituents = [
                            {"ticker": c["ticker"], "securityId": c["securityId"]}
                            for c in sorted_constituents[:TOP_CONSTITUENTS]
                        ]
                        result = {
                            "group": name,
                            "constituents": constituents
                        }
                    result_list.append(result)
                
                key = f"active:{timeframe}:{group}:{metric_type} leader"
                data.cache.set(key, json.dumps(result_list))
                
                # Build results for laggards
                result_list = []
                for entry in laggards:
                    name, sid, agg_value = entry
                    if group == "stock":
                        result = {
                            "ticker": name,
                            "securityId": sid
                        }
                    else:
                        if group == "sector":
                            constituents_data = sector_constituents.get(name, {}).get(metric_type, [])
                        else:  # group == "industry"
                            constituents_data = industry_constituents.get(name, {}).get(metric_type, [])
                        # Sort constituents ascending for laggard lists
                        sorted_constituents = sorted(constituents_data, key=lambda x: x["value"])
                        constituents = [
                            {"ticker": c["ticker"], "securityId": c["securityId"]}
                            for c in sorted_constituents[:TOP_CONSTITUENTS]
                        ]
                        result = {
                            "group": name,
                            "constituents": constituents
                        }
                    result_list.append(result)
                
                key = f"active:{timeframe}:{group}:{metric_type} laggard"
                data.cache.set(key, json.dumps(result_list))
    
    print("Market metrics update completed")
    return "Market metrics updated successfully"

def update_active(data, **args):
    return calculate_active(data)

if __name__ == "__main__":
    data = Conn(False)
    update_active(data)
