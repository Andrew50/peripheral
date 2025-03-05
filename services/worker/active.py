import numpy as np
import json
from conn import Conn
from data import getTensor

# Constants for list lengths - these will be used for display purposes in the frontend
# but we'll store the full list in the cache


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
    if len(tensor[idx]) < 2:
        return 0
    prev_close = tensor[idx][-2][3]  # Previous day's close
    curr_open = tensor[idx][-1][0]  # Current day's open
    return ((curr_open - prev_close) / prev_close) * 100


def calculate_adr(tensor, idx, lookback_days=20):
    """Calculate Average Daily Range over the lookback period"""
    if len(tensor[idx]) < lookback_days:
        return 0
    
    segment = tensor[idx][-lookback_days:]
    # Calculate daily range as (high - low) / close * 100 (percentage)
    daily_ranges = [(bar[1] - bar[2]) / bar[3] * 100 for bar in segment if bar[3] > 0]
    
    if not daily_ranges:
        return 0
    
    return sum(daily_ranges) / len(daily_ranges)


def day_over_day_change(tensor_bars):
    """
    Returns % change from second-to-last bar's close to last bar's close.
    If fewer than 2 bars, returns None or 0.
    """
    if len(tensor_bars) < 2:
        return None  # Not enough data to calculate day-over-day change
    prev_close = tensor_bars[-2][3]
    curr_close = tensor_bars[-1][3]
    if prev_close == 0:
        return None  # Avoid division by zero
    return 100.0 * (curr_close - prev_close) / prev_close


def calculate_active(data):
    with data.db.cursor() as cursor:
        cursor.execute(
            """
            SELECT 
                ticker, 
                sector, 
                industry, 
                securityId,
                COALESCE(share_class_shares_outstanding, 0) as outstanding_shares
            FROM securities 
            WHERE maxDate is NULL
        """
        )
        tickers = cursor.fetchall()

    print(f"Processing {len(tickers)} active securities")
    instances = [{"ticker": t[0], "dt": 0} for t in tickers]
    tensor, labels = getTensor(data, instances, "1d", 270, normalize="none")
    ticker_to_idx = {label["ticker"]: i for i, label in enumerate(labels)}
    timeframes = ["1 day", "1 week", "1 month", "6 month", "1 year"]
    groups = ["stock", "sector", "industry"]

    # Helper function to convert NumPy types to native Python types
    def convert_numpy_types(obj):
        if isinstance(obj, np.integer):
            return int(obj)
        elif isinstance(obj, np.floating):
            return float(obj)
        elif isinstance(obj, np.ndarray):
            return convert_numpy_types(obj.tolist())
        elif isinstance(obj, list):
            return [convert_numpy_types(item) for item in obj]
        elif isinstance(obj, dict):
            return {key: convert_numpy_types(value) for key, value in obj.items()}
        else:
            return obj

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
            "industry": {"price": [], "volume": [], "gap": []},
        }

        # For sector and industry, track the constituent stocks by metric.
        # Each entry is a dict with keys "ticker", "securityId", and "value".
        sector_constituents = {}
        industry_constituents = {}

        # Loop over each security and compute metrics
        for i, label in enumerate(labels):
            if i > 0 and i % 1000 == 0:
                print(f"Processed {i}/{len(labels)} securities...")

            ticker = label["ticker"]
            # Look up the security details from the original tickers list
            security = next((t for t in tickers if t[0] == ticker), None)
            if not security:
                continue

            _, sector, industry, securityId, outstanding_shares = security

            idx = ticker_to_idx.get(ticker)
            if idx is None or idx >= len(tensor) or len(tensor[idx]) < (lookback_bars + 1):
                continue

            # Calculate price return in percentage over the lookback period
            current_price = tensor[idx][-1][3]  # current close
            
            # Use helper function for 1-day timeframe
            if timeframe == "1 day":
                price_val = day_over_day_change(tensor[idx])
                if price_val is None:
                    continue  # Skip if we couldn't calculate the change
            else:
                past_price = tensor[idx][-(lookback_bars+1)][3]  # past close for other timeframes
                if past_price == 0:
                    continue  # avoid division by zero
                price_val = ((current_price - past_price) / past_price) * 100

            # Skip if we have a NaN or None change value
            if price_val is None or np.isnan(price_val):
                continue

            # Compute a volume metric (as provided in the original code)
            # Note: this is raw volume
            volume_segment = tensor[idx][-lookback_bars:]
            volume_val = np.sum(volume_segment[:, 4])
            
            # Calculate dollar volume (price * volume)
            dollar_volume = current_price * volume_segment[-1, 4]
            
            # Calculate market cap (current price * outstanding shares)
            current_market_cap = current_price * outstanding_shares
            
            # Calculate ADR (Average Daily Range)
            adr_val = calculate_adr(tensor, idx)
            
            # Only calculate gap if timeframe is "1 day"
            gap_val = calculate_gap(tensor, idx) if timeframe == "1 day" else None

            # Stock-level metrics (each stock is a unique entry)
            group_results["stock"]["price"].append((ticker, securityId, price_val, current_market_cap, dollar_volume, adr_val))
            group_results["stock"]["volume"].append((ticker, securityId, volume_val, current_market_cap, dollar_volume, adr_val))
            if timeframe == "1 day":
                group_results["stock"]["gap"].append((ticker, securityId, gap_val, current_market_cap, dollar_volume, adr_val))

            # For sector group: only add if sector is known (not null/Unknown)
            if sector and sector != "Unknown":
                if sector not in sector_constituents:
                    sector_constituents[sector] = {"price": [], "volume": [], "gap": []}
                sector_constituents[sector]["price"].append(
                    {"ticker": ticker, "securityId": securityId, "value": price_val, 
                     "market_cap": current_market_cap, "dollar_volume": dollar_volume, "adr": adr_val}
                )
                sector_constituents[sector]["volume"].append(
                    {"ticker": ticker, "securityId": securityId, "value": volume_val, 
                     "market_cap": current_market_cap, "dollar_volume": dollar_volume, "adr": adr_val}
                )
                if timeframe == "1 day" and gap_val is not None and not np.isnan(gap_val):
                    sector_constituents[sector]["gap"].append(
                        {"ticker": ticker, "securityId": securityId, "value": gap_val, 
                         "market_cap": current_market_cap, "dollar_volume": dollar_volume, "adr": adr_val}
                    )

            # For industry group: only add if industry is known (not null/Unknown)
            if industry and industry != "Unknown":
                if industry not in industry_constituents:
                    industry_constituents[industry] = {
                        "price": [],
                        "volume": [],
                        "gap": [],
                    }
                industry_constituents[industry]["price"].append(
                    {"ticker": ticker, "securityId": securityId, "value": price_val, 
                     "market_cap": current_market_cap, "dollar_volume": dollar_volume, "adr": adr_val}
                )
                industry_constituents[industry]["volume"].append(
                    {"ticker": ticker, "securityId": securityId, "value": volume_val, 
                     "market_cap": current_market_cap, "dollar_volume": dollar_volume, "adr": adr_val}
                )
                if timeframe == "1 day" and gap_val is not None and not np.isnan(gap_val):
                    industry_constituents[industry]["gap"].append(
                        {"ticker": ticker, "securityId": securityId, "value": gap_val, 
                         "market_cap": current_market_cap, "dollar_volume": dollar_volume, "adr": adr_val}
                    )

        # Now aggregate the sector and industry metrics from their constituents.
        for sector, metrics in sector_constituents.items():
            # For "price" metric
            if metrics["price"]:
                # Filter out any NaN values before calculating average
                valid_prices = [item["value"] for item in metrics["price"] if item["value"] is not None and not np.isnan(item["value"])]

                if valid_prices:
                    avg_price = sum(valid_prices) / len(valid_prices)
                    group_results["sector"]["price"].append((sector, None, avg_price))
            # For "volume" metric
            if metrics["volume"]:
                # Filter out any NaN values before calculating average
                valid_volumes = [item["value"] for item in metrics["volume"] if item["value"] is not None and not np.isnan(item["value"])]
                if valid_volumes:
                    avg_volume = sum(valid_volumes) / len(valid_volumes)
                    group_results["sector"]["volume"].append((sector, None, avg_volume))
            # For "gap" metric (only if applicable)
            if timeframe == "1 day" and metrics["gap"]:
                # Filter out any NaN values before calculating average
                valid_gaps = [item["value"] for item in metrics["gap"] if item["value"] is not None and not np.isnan(item["value"])]
                if valid_gaps:
                    avg_gap = sum(valid_gaps) / len(valid_gaps)
                    group_results["sector"]["gap"].append((sector, None, avg_gap))

        for industry, metrics in industry_constituents.items():
            if metrics["price"]:
                # Filter out any NaN values before calculating average
                valid_prices = [item["value"] for item in metrics["price"] if item["value"] is not None and not np.isnan(item["value"])]
                if valid_prices:
                    avg_price = sum(valid_prices) / len(valid_prices)
                    group_results["industry"]["price"].append((industry, None, avg_price))
            if metrics["volume"]:
                # Filter out any NaN values before calculating average
                valid_volumes = [item["value"] for item in metrics["volume"] if item["value"] is not None and not np.isnan(item["value"])]
                if valid_volumes:
                    avg_volume = sum(valid_volumes) / len(valid_volumes)
                    group_results["industry"]["volume"].append((industry, None, avg_volume))
            if timeframe == "1 day" and metrics["gap"]:
                # Filter out any NaN values before calculating average
                valid_gaps = [item["value"] for item in metrics["gap"] if item["value"] is not None and not np.isnan(item["value"])]
                if valid_gaps:
                    avg_gap = sum(valid_gaps) / len(valid_gaps)
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

                # Sort values - don't crop them for storage
                leaders = sorted(values, key=lambda x: x[2], reverse=True)
                laggards = sorted(values, key=lambda x: x[2])
                
                # Debug: Print the top 5 leaders for price in 1-day timeframe
                if metric_type == "price" and timeframe == "1 day" and group == "stock" and leaders:
                    print("\nTop 5 price leaders for 1-day timeframe:")
                    for i, entry in enumerate(leaders[:5]):
                        name, sid, agg_value, market_cap, dollar_volume, adr = entry
                        print(f"  {i+1}. {name}: {agg_value:.2f}%")

                # Build results for leaders - store ALL results, not just TOP_N
                result_list = []
                for entry in leaders:
                    if group == "stock":
                        name, sid, agg_value, market_cap, dollar_volume, adr = entry
                        # For stocks, we simply output ticker and securityId along with the metrics
                        result = {
                            "ticker": name, 
                            "securityId": sid, 
                            "market_cap": market_cap, 
                            "dollar_volume": dollar_volume,
                            "adr": adr
                        }
                    else:
                        name, sid, agg_value = entry
                        # For sectors/industries, get the list of constituent stocks
                        if group == "sector":
                            constituents_data = sector_constituents.get(name, {}).get(
                                metric_type, []
                            )
                        else:  # group == "industry"
                            constituents_data = industry_constituents.get(name, {}).get(
                                metric_type, []
                            )
                        # Sort constituents descending for leader lists
                        sorted_constituents = sorted(
                            constituents_data, key=lambda x: x["value"], reverse=True
                        )
                        # Store ALL constituents, not just TOP_CONSTITUENTS
                        constituents = [
                            {
                                "ticker": c["ticker"], 
                                "securityId": c["securityId"], 
                                "market_cap": c["market_cap"], 
                                "dollar_volume": c["dollar_volume"],
                                "adr": c["adr"]
                            }
                            for c in sorted_constituents
                        ]
                        result = {"group": name, "constituents": constituents}
                    result_list.append(result)

                key = f"active:{timeframe}:{group}:{metric_type} leader"
                data.cache.set(key, json.dumps(convert_numpy_types(result_list)))

                # Build results for laggards - store ALL results, not just TOP_N
                result_list = []
                for entry in laggards:
                    if group == "stock":
                        name, sid, agg_value, market_cap, dollar_volume, adr = entry
                        result = {
                            "ticker": name, 
                            "securityId": sid, 
                            "market_cap": market_cap, 
                            "dollar_volume": dollar_volume,
                            "adr": adr
                        }
                    else:
                        name, sid, agg_value = entry
                        if group == "sector":
                            constituents_data = sector_constituents.get(name, {}).get(
                                metric_type, []
                            )
                        else:  # group == "industry"
                            constituents_data = industry_constituents.get(name, {}).get(
                                metric_type, []
                            )
                        # Sort constituents ascending for laggard lists
                        sorted_constituents = sorted(
                            constituents_data, key=lambda x: x["value"]
                        )
                        # Store ALL constituents, not just TOP_CONSTITUENTS
                        constituents = [
                            {
                                "ticker": c["ticker"], 
                                "securityId": c["securityId"], 
                                "market_cap": c["market_cap"], 
                                "dollar_volume": c["dollar_volume"],
                                "adr": c["adr"]
                            }
                            for c in sorted_constituents
                        ]
                        result = {"group": name, "constituents": constituents}
                    result_list.append(result)

                key = f"active:{timeframe}:{group}:{metric_type} laggard"
                data.cache.set(key, json.dumps(convert_numpy_types(result_list)))

    print("Market metrics update completed")    
    return "Market metrics updated successfully"


def update_active(data, **args):
    print("updating active metrics !!!!!!!!!!!!!!!!!!!!")
    return calculate_active(data)


if __name__ == "__main__":
    data = Conn(False)
    update_active(data)


