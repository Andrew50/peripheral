#!/usr/bin/env python3
"""
Simple test file for data_accessors.py
Tests the _get_bar_data function with different timeframes, date ranges, and filters.
"""

import os
import sys
import logging
from datetime import datetime, timedelta

# Add the src directory to the path so we can import our modules
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# pylint: disable=wrong-import-position, import-outside-toplevel
from utils.conn import Conn
from utils.context import Context
from utils.data_accessors import _get_bar_data

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def test_bar_data_queries() -> None:
    """Test _get_bar_data with various parameters"""
    
    # Create connection instance
    print("Creating connection instance...")
    conn = Conn()
    
    # Create context instance
    print("Creating context instance...")
    ctx = Context(
        conn=conn,
        task_id="test_task_bar_data",
        status_id="test_status_bar_data",
        heartbeat_interval=30,
        queue_type="test",
        priority="normal",
        worker_id="test_worker",
        skip_heartbeat=True
    )
    
    # Define test cases
    test_cases = [ 
        {
            "name": "Daily data - 2023-07-15 weekend test - Short ticker list",
            "timeframe": "1d",
            "columns": ["ticker", "timestamp", "open", "close"],
            "min_bars": 1,
            "filters": {"tickers": ["AAPL", "MSFT", "GOOGL"]},
            "extended_hours": False,
            "start_date": datetime(2023, 7, 17),
            "end_date": datetime(2023, 7, 17) + timedelta(days=1)
        },
        {
            "name": "Daily data - 2023-07-14 trading day - Short ticker list",
            "timeframe": "1d",
            "columns": ["ticker", "timestamp", "open", "close"],
            "min_bars": 1,
            "filters": {"tickers": ["AAPL", "MSFT", "GOOGL"]},
            "extended_hours": False,
            "start_date": datetime(2023, 7, 14),
            "end_date": datetime(2023, 7, 14) + timedelta(days=1)
        },
        {
            "name": "Daily data - Recent 5 days - AAPL only",
            "timeframe": "1d",
            "columns": ["ticker", "timestamp", "close", "volume"],
            "min_bars": 5,
            "filters": {"tickers": ["AAPL"]},
            "extended_hours": False,
            "start_date": datetime(2025, 7, 20),
            "end_date": datetime(2025, 7, 30)
        },
        {
            "name": "Hourly data - Last 2 days - Tech stocks",
            "timeframe": "1h",
            "columns": ["ticker", "timestamp", "open", "high", "low", "close"],
            "min_bars": 10,
            "filters": {"tickers": ["AAPL", "MSFT", "GOOGL"]},
            "extended_hours": False,
            "start_date": datetime.now() - timedelta(days=3),
            "end_date": datetime.now() - timedelta(days=1)
        },
        {
            "name": "Weekly data - Last 3 months - Single stock",
            "timeframe": "1w",
            "columns": ["ticker", "timestamp", "close", "volume"],
            "min_bars": 8,
            "filters": {"tickers": ["TSLA"]},
            "extended_hours": False,
            "start_date": datetime.now() - timedelta(days=90),
            "end_date": datetime.now() - timedelta(days=1)
        },
        {
            "name": "5-minute data - Yesterday - Single stock",
            "timeframe": "5m",
            "columns": ["ticker", "timestamp", "close"],
            "min_bars": 20,
            "filters": {"tickers": ["SPY"]},
            "extended_hours": False,
            "start_date": datetime.now() - timedelta(days=2),
            "end_date": datetime.now() - timedelta(days=1)
        },
        {
            "name": "Real-time mode - Latest 10 bars - Multiple stocks",
            "timeframe": "1d",
            "columns": ["ticker", "timestamp", "close", "volume"],
            "min_bars": 10,
            "filters": {"tickers": ["AAPL", "MSFT"]},
            "extended_hours": False,
            "start_date": None,
            "end_date": None
        }
    ]
    
    print("\n" + "="*80)
    print("TESTING _get_bar_data WITH VARIOUS PARAMETERS")
    print("="*80)
    
    for i, test_case in enumerate(test_cases, 1):
        print(f"\n--- Test {i}: {test_case['name']} ---")
        
        try:
            # Execute the query
            df = _get_bar_data(
                ctx=ctx,
                timeframe=test_case['timeframe'],
                columns=test_case['columns'],
                min_bars=test_case['min_bars'],
                filters=test_case['filters'],
                extended_hours=test_case['extended_hours'],
                start_date=test_case['start_date'],
                end_date=test_case['end_date']
            )
            
            if df is not None and len(df) > 0:
                print(f"✅ Success: Retrieved {len(df)} rows")
                print(f"   Columns: {list(df.columns)}")
                print(f"   Unique tickers: {df['ticker'].nunique() if 'ticker' in df.columns else 'N/A'}")
                
                # Print first few rows
                print("   First 3 rows:")
                for _, row in df.head(3).iterrows():
                    row_str = ", ".join([f"{col}: {row[col]}" for col in df.columns])
                    print(f"     {row_str}")
                
                # Print last row if different from first few
                if len(df) > 3:
                    print("   Last row:")
                    last_row = df.iloc[-1]
                    row_str = ", ".join([f"{col}: {last_row[col]}" for col in df.columns])
                    print(f"     {row_str}")
                    
            else:
                print("⚠️  No data returned")
                
        except ValueError as e:
            print(f"❌ Error: {e}")
            import traceback
            print(f"   Traceback: {traceback.format_exc()}")
    
    print("\n" + "="*80)
    print("TEST SUMMARY COMPLETE")
    print("="*80)
    
    # Clean up
    print("\nCleaning up...")
    ctx.destroy()
    conn.close_connections()

if __name__ == "__main__":
    print("Starting data accessor tests...")
    test_bar_data_queries()
    print("Tests completed.") 