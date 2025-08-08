#!/usr/bin/env python3
"""
Basic test file for strategy validation.
Tests the validate_strategy function from validator.py using actual Context and Conn instances.
"""

import os
import sys
import logging
from datetime import datetime, timedelta
import traceback

# Add the src directory to the path so we can import our modules
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# Local imports after sys.path modification
# pylint: disable=wrong-import-position
from utils.conn import Conn
from utils.context import Context
from validator import validate_strategy
from engine import execute_strategy
# pylint: enable=wrong-import-position

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def test_strategy_validation() -> None:
    """Test strategy validation with a simple strategy"""
    
    # Create connection instance
    print("Creating connection instance...")
    conn = Conn()
    
    # Create context instance (using dummy values for testing)
    print("Creating context instance...")
    ctx = Context(
        conn=conn,
        task_id="test_task_123",
        status_id="test_status_123",
        heartbeat_interval=30,
        queue_type="test",
        priority="normal",
        worker_id="test_worker",
        skip_heartbeat=True

    )
    
    # Define a simple test strategy
    test_strategy = '''
def strategy():
    """Simple test strategy that finds stocks with high volume"""
    instances = []
    
    # Get bar data for a few tech stocks
    df = get_bar_data(
        timeframe="1d",
        columns=["ticker", "timestamp", "close", "volume"],
        min_bars=1,
        filters={"tickers": ["AAPL", "MSFT", "GOOGL"]}
    )
    
    if df is None or len(df) == 0:
        return instances

    
    # Find instances with volume > 20 million
    high_volume = df[df['volume'] > 20000000]
    
    for _, row in high_volume.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'timestamp': int(row['timestamp']),
            'entry_price': float(row['close']),
            'volume': float(row['volume']),
            'score': min(1.0, row['volume'] / 100000000)  # Normalize by 100M volume
        })
    
    return instances
'''
    
    print("Testing strategy validation...")
    print(f"Strategy code length: {len(test_strategy)} characters")
    
    try:
        # Validate the strategy
        is_valid, error_message = validate_strategy(ctx, test_strategy)
        
        print("\nValidation Results:")
        print(f"Is Valid: {is_valid}")
        if error_message:
            print(f"Error Message: {error_message}")
        else:
            print("No errors found!")
        
        # Execute strategy with date range (1 year ago to 2 days ago)
        print("\n--- First run (1y-ago → 2d-ago) ---")
        one_year_ago = datetime.now() - timedelta(days=365)
        two_days_ago = datetime.now() - timedelta(days=2)
        
        instances1, prints1, _plots1, _images1, err1 = execute_strategy(
            ctx,
            test_strategy,
            strategy_id=1,
            version=1,
            start_date=one_year_ago,
            end_date=two_days_ago
        )
        
        if err1:
            print(f"Error in first execution: {err1.get('message', 'Unknown error')}")
            if err1.get('traceback'):
                print(f"Traceback: {err1['traceback']}")
        else:
            print(f"Returned {len(instances1)} instances")
            if instances1:
                print("Sample instances:")
                for i, instance in enumerate(instances1[:3]):  # Show first 3
                    print(f"  {i+1}: {instance}")
            if prints1:
                print(f"Strategy output: {prints1}")
        
        # Execute strategy with no date constraints
        print("\n--- Second run (no date limits) ---")
        instances2, prints2, _plots2, _images2, err2 = execute_strategy(
            ctx,
            test_strategy,
            strategy_id=1,
            version=1,
            start_date=None,
            end_date=None
        )
        
        if err2:
            print(f"Error in second execution: {err2.get('message', 'Unknown error')}")
            if err2.get('traceback'):
                print(f"Traceback: {err2['traceback']}")
        else:
            print(f"Returned {len(instances2)} instances")
            if instances2:
                print("Sample instances:")
                for i, instance in enumerate(instances2[:3]):  # Show first 3
                    print(f"  {i+1}: {instance}")
            if prints2:
                print(f"Strategy output: {prints2}")
            
    except Exception:  # pylint: disable=broad-exception-caught
        # import moved to module top
        traceback.print_exc()
    
    finally:
        # Clean up context
        print("Cleaning up...")
        ctx.destroy()
        conn.close_connections()

if __name__ == "__main__":
    print("Starting strategy validation test...")
    test_strategy_validation()
    print("Test completed.") 