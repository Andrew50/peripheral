from decimal import Decimal
import numpy as np
import pandas as pd
from datetime import datetime, timedelta
import pytz
import tensorflow as tf
from tensorflow.keras.models import Sequential
from tensorflow.keras.layers import Dense, LSTM, Bidirectional, Conv1D, Input
from tensorflow.keras.optimizers import Adam


def normalize_pattern(pattern):
    """Normalize OHLCV data"""
    # First close price for normalization
    first_close = pattern[0, 3]  # Using first row's close price

    # Avoid division by zero
    if first_close == 0:
        return np.zeros_like(pattern)

    # Normalize OHLC by the first close price
    normalized = np.copy(pattern)
    normalized[:, :4] = pattern[:, :4] / first_close - 1

    # Normalize volume if present (5th column)
    if pattern.shape[1] > 4:
        max_vol = np.max(pattern[:, 4])
        if max_vol > 0:
            normalized[:, 4] = pattern[:, 4] / max_vol

    return normalized


def get_trade_patterns(conn, lookback_bars=30):
    """
    Get OHLCV patterns for all trades in the database using direct Polygon API calls
    """
    try:
        with conn.db.cursor() as cursor:
            # Get all completed trades with non-null values
            cursor.execute(
                """
                SELECT 
                    t.tradeId,
                    t.ticker,
                    t.entry_times[1] as first_entry,
                    t.tradeDirection,
                    COALESCE(t.closedPnL, 0) as closedPnL
                FROM trades t
                WHERE t.status = 'Closed'
                    AND t.tradeId IS NOT NULL 
                    AND t.ticker IS NOT NULL 
                    AND t.entry_times[1] IS NOT NULL
                    AND t.tradeDirection IS NOT NULL
                ORDER BY t.entry_times[1]
            """
            )

            trades = cursor.fetchall()

            patterns = []
            trade_info = []

            for trade in trades:
                trade_id = int(trade[0]) if trade[0] else None
                if not trade_id:
                    continue

                ticker = trade[1]
                entry_time = trade[2]

                # Calculate time range for data fetch
                end_time = entry_time
                start_time = end_time - timedelta(minutes=lookback_bars)

                # Get aggregates from Polygon
                aggs_iter, err = conn.polygon.GetAggsData(
                    ticker=ticker,
                    multiplier=1,
                    timeframe="minute",
                    from_millis=int(start_time.timestamp() * 1000),
                    to_millis=int(end_time.timestamp() * 1000),
                    bars=lookback_bars,
                    results_order="asc",
                    is_adjusted=True,
                )

                if err:
                    print(f"Error fetching data for {ticker}: {err}")
                    continue

                # Convert aggs to numpy array
                ohlcv_data = []
                for agg in aggs_iter:
                    ohlcv_data.append(
                        [agg.Open, agg.High, agg.Low, agg.Close, agg.Volume]
                    )

                if len(ohlcv_data) < lookback_bars:
                    continue

                pattern = np.array(ohlcv_data[-lookback_bars:])
                normalized_pattern = normalize_pattern(pattern)
                patterns.append(normalized_pattern.flatten())

                trade_info.append(
                    {
                        "trade_id": trade_id,
                        "ticker": ticker,
                        "entry_time": entry_time,
                        "direction": trade[3],
                        "pnl": float(trade[4]) if trade[4] else 0.0,
                    }
                )

            if patterns:
                return np.array(patterns), trade_info

            return None, None

    except Exception as e:
        print(f"Error getting trade patterns: {str(e)}")
        return None, None


def find_similar_trades(conn, trade_id, user_id, n_neighbors=5):
    """
    Find similar trades using TensorFlow's distance calculations
    """
    try:
        # Get patterns for all trades
        patterns, trade_ids = get_trade_patterns(conn)

        if patterns is None or len(patterns) < n_neighbors:
            return []

        # Find the index of our target trade
        target_idx = None
        for i, t in enumerate(trade_ids):
            if t["trade_id"] == trade_id:
                target_idx = i
                break

        if target_idx is None:
            return []

        # Convert patterns to TensorFlow tensors
        patterns_tensor = tf.convert_to_tensor(patterns, dtype=tf.float32)
        target_pattern = tf.convert_to_tensor([patterns[target_idx]], dtype=tf.float32)

        # Calculate Euclidean distances using TensorFlow
        distances = tf.norm(patterns_tensor - target_pattern, axis=1)

        # Get indices of k nearest neighbors
        _, indices = tf.nn.top_k(-distances, k=n_neighbors + 1)
        distances = distances.numpy()
        indices = indices.numpy()

        # Get similar trades (skip first one as it's the same trade)
        similar_trades = []
        for idx in indices[1:]:
            distance = distances[idx]
            trade_info = trade_ids[idx]

            # Calculate similarity score (0 to 1, where 1 is most similar)
            similarity_score = 1 / (1 + distance)

            similar_trades.append(
                {
                    "trade_id": trade_info["trade_id"],
                    "ticker": trade_info["ticker"],
                    "entry_time": trade_info["entry_time"].isoformat(),
                    "direction": trade_info["direction"],
                    "pnl": trade_info["pnl"],
                    "similarity_score": float(similarity_score),
                }
            )

        return similar_trades

    except Exception as e:
        print(f"Error finding similar trades: {str(e)}")
        return []
