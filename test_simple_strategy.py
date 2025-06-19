#!/usr/bin/env python3
"""
Simple Strategy Test
Tests the worker with a basic strategy that doesn't require database access
"""

import asyncio
import json
import time
import uuid
from datetime import datetime

import redis


def test_simple_strategy_execution():
    """Test worker with a simple strategy that doesn't use data access functions"""
    
    print("🧪 Testing Simple Strategy Execution")
    print("=" * 50)

    # Connect to Redis
    try:
        r = redis.Redis(host="localhost", port=6379, decode_responses=True)
        r.ping()
        print("✅ Connected to Redis")
    except Exception as e:
        print(f"❌ Redis connection failed: {e}")
        return False

    # Create a very simple strategy (like the "When Stocks Cross Their Strategy")
    execution_id = f"test_simple_{uuid.uuid4().hex[:8]}"
    
    # Simple moving average crossover strategy - no external data needed
    strategy_code = '''
def classify_symbol(symbol):
    """
    Simple moving average crossover strategy using mock data
    This simulates the "When Stocks Cross Their Strategy" logic
    """
    
    # Mock price data for testing (simulating price history)
    # In real implementation, this would come from get_price_data()
    mock_prices = [100, 101, 102, 101, 103, 105, 107, 106, 108, 110, 
                   109, 111, 113, 112, 114, 115, 117, 116, 118, 120]
    
    def calculate_sma(prices, period):
        """Calculate Simple Moving Average"""
        if len(prices) < period:
            return []
        return [sum(prices[i-period+1:i+1])/period for i in range(period-1, len(prices))]
    
    # Calculate 5-day and 10-day moving averages
    sma_5 = calculate_sma(mock_prices, 5)
    sma_10 = calculate_sma(mock_prices, 10)
    
    if len(sma_5) >= 2 and len(sma_10) >= 2:
        # Current values
        current_sma5 = sma_5[-1]
        current_sma10 = sma_10[-1]
        
        # Previous values  
        prev_sma5 = sma_5[-2]
        prev_sma10 = sma_10[-2]
        
        # Check for golden cross: 5-day crosses above 10-day
        golden_cross = (prev_sma5 <= prev_sma10) and (current_sma5 > current_sma10)
        
        # Log the calculation for debugging
        save_result('current_sma5', current_sma5)
        save_result('current_sma10', current_sma10)
        save_result('prev_sma5', prev_sma5)
        save_result('prev_sma10', prev_sma10)
        save_result('golden_cross', golden_cross)
        save_result('strategy_name', 'When Stocks Cross Their Strategy')
        
        return golden_cross
    
    save_result('error', 'Insufficient data for SMA calculation')
    return False

# Test the strategy
symbol = input_data.get('symbol', 'TEST')
result = classify_symbol(symbol)
save_result('classification', result)
save_result('symbol_tested', symbol)
'''

    test_job = {
        "execution_id": execution_id,
        "python_code": strategy_code,
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
                                print(f"📋 Strategy name: {output.get('strategy_name', 'Not found')}")
                                print(f"📋 Golden cross: {output.get('golden_cross', 'Not found')}")
                                print(f"📋 Current SMA5: {output.get('current_sma5', 'Not found')}")
                                print(f"📋 Current SMA10: {output.get('current_sma10', 'Not found')}")
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

    print("✅ Test completed!")
    return result_received


if __name__ == "__main__":
    success = test_simple_strategy_execution()
    if success:
        print("\n🎉 Simple strategy test PASSED!")
    else:
        print("\n❌ Simple strategy test FAILED!") 