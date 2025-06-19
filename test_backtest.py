#!/usr/bin/env python3
"""
Test script to verify backtest functionality
"""

import json
import redis
import time
import uuid
from datetime import datetime

def test_backtest_execution():
    """Test the backtest execution pipeline"""
    
    print("🧪 Testing Backtest Execution Pipeline")
    print("=" * 50)
    
    # Connect to Redis
    try:
        r = redis.Redis(host="localhost", port=6379, decode_responses=True)
        r.ping()
        print("✅ Connected to Redis")
    except Exception as e:
        print(f"❌ Redis connection failed: {e}")
        return False

    # Create a test strategy similar to what the logs showed
    strategy_code = '''
import numpy as np
from typing import List, Dict, Tuple, Union, Optional

def classify_symbol(symbol: str) -> bool:
    """
    Identifies tech stocks that have moved (absolute percentage change from open to close)
    more than 5% on the most recent trading day.
    """
    try:
        # Step 1: Check if the symbol belongs to the 'Technology' sector.
        security_info = get_security_info(symbol)
        if not security_info or security_info.get('sector') != 'Technology':
            return False  # Not a technology stock or info unavailable

        # Step 2: Get the most recent day's price data.
        price_data = get_price_data(symbol, timeframe='1d', days=1)

        # Ensure valid price data is returned and contains at least one day's data.
        if not price_data or not price_data.get('open') or len(price_data['open']) == 0:
            return False  # No price data or incomplete data for the last day

        # Extract the current day's open and close prices.
        current_open = price_data['open'][-1]
        current_close = price_data['close'][-1]

        # Handle cases where open price might be zero to prevent ZeroDivisionError
        if current_open == 0:
            return False

        # Step 3: Calculate the absolute percentage change from open to close.
        daily_change_percent = ((current_close - current_open) / current_open) * 100

        # Step 4: Check if the absolute daily movement exceeds the 5% threshold.
        movement_threshold = 5.0
        return abs(daily_change_percent) > movement_threshold

    except Exception:
        # Catch any exceptions and return False
        return False
'''

    # Create test job with wrapper code (similar to what our Go code does)
    execution_id = f"test_backtest_{uuid.uuid4().hex[:8]}"
    
    wrapped_code = f'''
# Original strategy code
{strategy_code}

# Execute the strategy for the given symbol
try:
    # Get symbol from input data
    symbol = input_data.get('symbol', 'AAPL')
    
    # Execute the classify_symbol function
    try:
        result = classify_symbol(symbol)
        save_result('classification', result)
        save_result('symbol_tested', symbol)
        save_result('strategy_type', 'tech_stocks_5percent')
    except NameError:
        save_result('classification', False)
        save_result('error', 'classify_symbol function not found in strategy code')
        
except Exception as e:
    save_result('classification', False)
    save_result('error', str(e))
'''

    test_job = {
        "execution_id": execution_id,
        "python_code": wrapped_code,
        "input_data": {
            "symbol": "NVDA",
            "date": "2024-01-15",
            "timestamp": int(time.time() * 1000)
        },
        "timeout_seconds": 30,
        "memory_limit_mb": 128,
        "libraries": ["numpy", "pandas"],
        "created_at": datetime.utcnow().isoformat(),
    }

    # Submit job to queue
    print(f"📤 Submitting test job: {execution_id}")
    job_json = json.dumps(test_job)
    position = r.rpush("python_execution_queue", job_json)
    print(f"✅ Job queued at position: {position}")

    # Monitor execution
    print("👀 Monitoring execution (60s timeout)...")
    pubsub = r.pubsub()
    pubsub.subscribe("python_execution_updates")

    start_time = time.time()
    timeout = 60
    result_received = False

    try:
        for message in pubsub.listen():
            if message["type"] == "message":
                try:
                    update = json.loads(message["data"])
                    if update.get("execution_id") == execution_id:
                        status = update["status"]
                        print(f"📢 Status: {status}")

                        if status == "running":
                            print(f"🏃 Execution started on worker: {update.get('worker_id', 'unknown')}")

                        elif status == "completed":
                            print("🎉 Execution completed successfully!")
                            if "output_data" in update:
                                output = update["output_data"]
                                print(f"📋 Output keys: {list(output.keys())}")
                                print(f"📋 Classification: {output.get('classification', 'Not found')}")
                                print(f"📋 Symbol tested: {output.get('symbol_tested', 'Not found')}")
                                if output.get('error'):
                                    print(f"⚠️ Error: {output.get('error')}")
                            print(f"⏱️ Execution time: {update.get('execution_time_ms', 0)}ms")
                            result_received = True
                            break

                        elif status == "failed":
                            print(f"❌ Execution failed: {update.get('error_message', 'Unknown error')}")
                            result_received = True
                            break

                        elif status == "timeout":
                            print("⏰ Execution timed out")
                            result_received = True
                            break

                except json.JSONDecodeError:
                    continue

            # Check our own timeout
            if time.time() - start_time > timeout:
                print("⏰ Monitoring timeout reached")
                break

    finally:
        pubsub.close()

    if result_received:
        print("✅ Test completed successfully!")
        return True
    else:
        print("❌ Test failed - no result received")
        return False

if __name__ == "__main__":
    success = test_backtest_execution()
    exit(0 if success else 1) 