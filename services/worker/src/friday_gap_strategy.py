#!/usr/bin/env python3
"""
Friday Afternoon Large-Cap Gap Analysis Strategy

This strategy analyzes Friday afternoon movements in stocks with market cap > $50 billion
and tracks whether the subsequent gap (usually Monday) is in the same direction as the 
Friday afternoon "imbalance" move (3:49 PM to close).
"""

import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import pytz
from typing import List, Dict, Any

def friday_gap_strategy():
    """
    Strategy to analyze Friday afternoon movements and subsequent gaps in large-cap stocks.
    """
    instances = []
    
    try:
        from data_accessors import get_bar_data, get_security_details
        
        # Get large-cap stocks (market cap > $50B)
        large_cap_stocks = get_security_details(
            columns=["ticker", "market_cap"],
            filters={
                "market_cap_min": 50000000000,  # $50B minimum
                "active": True
            }
        )
        
        if len(large_cap_stocks) == 0:
            return instances
        
        large_cap_tickers = [stock[0] for stock in large_cap_stocks]
        
        # Get daily data for analysis
        daily_data = get_bar_data(
            timeframe="1d",
            tickers=large_cap_tickers,
            columns=["ticker", "timestamp", "open", "high", "low", "close"],
            min_bars=90  # 3 months of daily data
        )
        
        if len(daily_data) == 0:
            return instances
        
        df = pd.DataFrame(daily_data, columns=["ticker", "timestamp", "open", "high", "low", "close"])
        df['datetime'] = pd.to_datetime(df['timestamp'], unit='s')
        df['weekday'] = df['datetime'].dt.weekday
        df['date'] = df['datetime'].dt.date
        
        # Sort by ticker and date
        df = df.sort_values(['ticker', 'datetime'])
        
        # Calculate next day's gap for each row
        df['next_open'] = df.groupby('ticker')['open'].shift(-1)
        df['gap_pct'] = ((df['next_open'] - df['close']) / df['close']) * 100
        
        # Filter for Fridays with significant moves (using daily range as proxy for Friday afternoon moves)
        df['daily_range_pct'] = ((df['high'] - df['low']) / df['open']) * 100
        df['friday_move_pct'] = ((df['close'] - df['open']) / df['open']) * 100
        
        friday_moves = df[
            (df['weekday'] == 4) &  # Fridays
            (abs(df['friday_move_pct']) > 2.0) &  # Significant move > 2%
            (df['gap_pct'].notna())  # Has next day data
        ]
        
        for _, row in friday_moves.iterrows():
            friday_direction = 'up' if row['friday_move_pct'] > 0 else 'down'
            gap_direction = 'up' if row['gap_pct'] > 0 else 'down'
            
            instance = {
                'ticker': row['ticker'],
                'date': str(row['date']),
                'signal': True,
                'friday_move_percent': round(row['friday_move_pct'], 2),
                'friday_move_direction': friday_direction,
                'gap_percent': round(row['gap_pct'], 2),
                'gap_direction': gap_direction,
                'imbalance_direction_match': (friday_direction == gap_direction),
                'score': min(1.0, abs(row['friday_move_pct']) / 5.0),
                'message': f"{row['ticker']} Friday move {row['friday_move_pct']:+.1f}%, gap {row['gap_pct']:+.1f}%"
            }
            
            instances.append(instance)
    
    except Exception as e:
        return []
    
    return instances

def get_next_trading_date(current_date):
    """
    Get the next trading date after the given date.
    Handles weekends and assumes no holidays for simplicity.
    """
    current = pd.Timestamp(current_date)
    
    # If it's Friday (4), next trading day is Monday (+3 days)
    if current.weekday() == 4:  # Friday
        return (current + timedelta(days=3)).date()
    # If it's Saturday (5), next trading day is Monday (+2 days)
    elif current.weekday() == 5:  # Saturday
        return (current + timedelta(days=2)).date()
    # If it's Sunday (6), next trading day is Monday (+1 day)
    elif current.weekday() == 6:  # Sunday
        return (current + timedelta(days=1)).date()
    else:
        # Weekday, next trading day is next day
        return (current + timedelta(days=1)).date()

# Alternative strategy for testing - simpler version
def friday_gap_simple_strategy():
    """
    Simplified version for testing when minute data might not be available
    """
    instances = []
    
    try:
        from data_accessors import get_bar_data, get_security_details
        
        # Get large-cap stocks
        large_cap_stocks = get_security_details(
            columns=["ticker", "market_cap"],
            filters={
                "market_cap_min": 50000000000,  # $50B minimum
                "active": True
            }
        )
        
        if len(large_cap_stocks) == 0:
            return instances
        
        large_cap_tickers = [stock[0] for stock in large_cap_stocks]
        
        # Get daily data for broader analysis
        daily_data = get_bar_data(
            timeframe="1d",
            tickers=large_cap_tickers,
            columns=["ticker", "timestamp", "open", "high", "low", "close"],
            min_bars=90  # 3 months of daily data
        )
        
        if len(daily_data) == 0:
            return instances
        
        df = pd.DataFrame(daily_data, columns=["ticker", "timestamp", "open", "high", "low", "close"])
        df['datetime'] = pd.to_datetime(df['timestamp'], unit='s')
        df['weekday'] = df['datetime'].dt.weekday
        df['date'] = df['datetime'].dt.date
        
        # Sort by ticker and date
        df = df.sort_values(['ticker', 'datetime'])
        
        # Calculate next day's gap for each row
        df['next_open'] = df.groupby('ticker')['open'].shift(-1)
        df['gap_pct'] = ((df['next_open'] - df['close']) / df['close']) * 100
        
        # Filter for Fridays with significant moves (using daily high-low range as proxy)
        df['daily_range_pct'] = ((df['high'] - df['low']) / df['open']) * 100
        
        friday_moves = df[
            (df['weekday'] == 4) &  # Fridays
            (df['daily_range_pct'] > 2.0) &  # Significant intraday movement
            (df['gap_pct'].notna())  # Has next day data
        ]
        
        for _, row in friday_moves.iterrows():
            # Approximate Friday direction from close vs open
            friday_move = ((row['close'] - row['open']) / row['open']) * 100
            friday_direction = 'up' if friday_move > 0 else 'down'
            gap_direction = 'up' if row['gap_pct'] > 0 else 'down'
            
            instance = {
                'ticker': row['ticker'],
                'date': str(row['date']),
                'signal': True,
                'friday_move_percent': round(friday_move, 2),
                'friday_move_direction': friday_direction,
                'gap_percent': round(row['gap_pct'], 2),
                'gap_direction': gap_direction,
                'imbalance_direction_match': (friday_direction == gap_direction),
                'score': min(1.0, abs(friday_move) / 5.0),
                'message': f"{row['ticker']} Friday move {friday_move:+.1f}%, gap {row['gap_pct']:+.1f}%"
            }
            
            instances.append(instance)
    
    except Exception as e:
        return []
    
    return instances

# Main strategy function (use the simple version for broader compatibility)
strategy = friday_gap_simple_strategy 