#!/usr/bin/env python3
"""
Test script for the Redis queue-based strategy worker system
"""

import json
import time
import uuid
import redis
from datetime import datetime, timedelta

def test_queue_system():
    """Test the basic queue functionality"""
    print("Testing Redis queue system...")
    
    # Connect to Redis
    try:
        redis_client = redis.Redis(
            host='localhost',  # Change to 'cache' if running in container
            port=6379,
            decode_responses=True
        )
        redis_client.ping()
        print("✓ Redis connection successful")
    except Exception as e:
        print(f"✗ Redis connection failed: {e}")
        print("Make sure Redis is running on localhost:6379")
        return False
    
    # Test adding a backtest task
    task_id = str(uuid.uuid4())
    strategy_code = '''
def gap_up_strategy(df):
    """Test strategy that finds gap ups"""
    instances = []
    
    # Simple gap up detection
    df_filtered = df[(df['gap_pct'] > 2.0) & (df['gap_pct'].notna())]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'date': str(row['date']),
            'signal': True,
            'gap_percent': row['gap_pct'],
            'message': f"{row['ticker']} gapped up {row['gap_pct']:.1f}%"
        })
    
    return instances
'''
    
    print(f"\nAdding backtest task: {task_id}")
    
    from worker import add_backtest_task
    add_backtest_task(
        redis_client=redis_client,
        task_id=task_id,
        strategy_code=strategy_code,
        symbols=['AAPL', 'MSFT', 'GOOGL'],
        start_date=(datetime.now() - timedelta(days=30)).isoformat(),
        end_date=datetime.now().isoformat()
    )
    
    # Check if task was queued
    queue_length = redis_client.llen('strategy_queue')
    print(f"✓ Task added to queue, queue length: {queue_length}")
    
    # Test getting task from queue (simulate worker)
    print("\nSimulating worker processing...")
    task_data = redis_client.brpop('strategy_queue', timeout=5)
    
    if task_data:
        _, task_message = task_data
        task_info = json.loads(task_message)
        print(f"✓ Task retrieved: {task_info['task_id']}")
        print(f"  Task type: {task_info['task_type']}")
        print(f"  Symbols: {task_info['args']['symbols']}")
        
        # Simulate setting task status
        result = {
            "status": "completed",
            "data": {
                "success": True,
                "instances": [
                    {"ticker": "AAPL", "signal": True, "message": "Test signal"}
                ],
                "execution_time_seconds": 1.5
            },
            "updated_at": datetime.utcnow().isoformat()
        }
        
        redis_client.setex(f"task_result:{task_id}", 86400, json.dumps(result))
        print(f"✓ Task result stored for {task_id}")
        
        # Test retrieving result
        from worker import get_task_result
        retrieved_result = get_task_result(redis_client, task_id)
        if retrieved_result:
            print(f"✓ Result retrieved: {retrieved_result['status']}")
            print(f"  Execution time: {retrieved_result['data'].get('execution_time_seconds')}s")
        else:
            print("✗ Failed to retrieve result")
            
    else:
        print("✗ No task retrieved from queue")
        return False
    
    # Test screening task
    print("\nTesting screening task...")
    screening_task_id = str(uuid.uuid4())
    
    from worker import add_screening_task
    add_screening_task(
        redis_client=redis_client,
        task_id=screening_task_id,
        strategy_code=strategy_code,
        universe=['AAPL', 'MSFT', 'GOOGL', 'TSLA', 'NVDA'],
        limit=5
    )
    
    queue_length = redis_client.llen('strategy_queue')
    print(f"✓ Screening task added, queue length: {queue_length}")
    
    # Cleanup
    redis_client.delete(f"task_result:{task_id}")
    redis_client.delete(f"task_result:{screening_task_id}")
    
    print("\n🎉 Queue system test completed successfully!")
    return True

def test_task_format():
    """Test task data format validation"""
    print("\nTesting task format...")
    
    # Valid backtest task
    backtest_task = {
        "task_id": "test-123",
        "task_type": "backtest",
        "args": {
            "strategy_code": "def test(df): return []",
            "symbols": ["AAPL"],
            "start_date": "2024-01-01",
            "end_date": "2024-01-31"
        }
    }
    
    # Valid screening task
    screening_task = {
        "task_id": "test-456", 
        "task_type": "screening",
        "args": {
            "strategy_code": "def test(df): return []",
            "universe": ["AAPL", "MSFT"],
            "limit": 10
        }
    }
    
    for task_name, task in [("backtest", backtest_task), ("screening", screening_task)]:
        try:
            # Test JSON serialization
            serialized = json.dumps(task)
            deserialized = json.loads(serialized)
            
            # Validate required fields
            required_fields = ["task_id", "task_type", "args"]
            for field in required_fields:
                if field not in deserialized:
                    print(f"✗ Missing field {field} in {task_name} task")
                    return False
            
            print(f"✓ {task_name.capitalize()} task format valid")
            
        except Exception as e:
            print(f"✗ {task_name.capitalize()} task validation failed: {e}")
            return False
    
    return True

def main():
    """Run all tests"""
    print("Running queue system tests...\n")
    
    tests = [
        test_task_format,
        test_queue_system
    ]
    
    for test in tests:
        if not test():
            print("\n❌ Tests failed!")
            return 1
        print()
    
    print("✅ All tests passed!")
    return 0

if __name__ == "__main__":
    exit(main()) 