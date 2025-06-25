#!/usr/bin/env python3
"""
Test Priority Queue System
Tests that strategy creation/editing tasks get priority over other tasks.
"""

import json
import time
import uuid
import redis
from datetime import datetime


def test_priority_queue_system():
    """Test the priority queue functionality"""
    print("🚀 Testing Priority Queue System...")
    
    # Connect to Redis
    try:
        redis_client = redis.Redis(
            host='localhost',  # Change to 'cache' if running in container
            port=6379,
            decode_responses=True
        )
        redis_client.ping()
        print("✅ Redis connection successful")
    except Exception as e:
        print(f"❌ Redis connection failed: {e}")
        print("Make sure Redis is running on localhost:6379")
        return False
    
    # Clear queues first
    redis_client.delete('strategy_queue')
    redis_client.delete('strategy_queue_priority')
    print("🧹 Cleared existing queues")
    
    # Add some normal priority tasks first
    print("\n📋 Adding normal priority tasks...")
    for i in range(3):
        task_id = f"backtest_{uuid.uuid4().hex[:8]}"
        task_data = {
            "task_id": task_id,
            "task_type": "backtest",
            "args": {
                "strategy_id": f"strategy_{i}",
                "symbols": ["AAPL", "MSFT"],
            },
            "created_at": datetime.utcnow().isoformat(),
            "priority": "normal"
        }
        redis_client.lpush('strategy_queue', json.dumps(task_data))
        print(f"  ➕ Added backtest task {i+1}: {task_id}")
    
    # Add a high priority strategy creation task
    print("\n⭐ Adding HIGH PRIORITY strategy creation task...")
    strategy_task_id = f"create_strategy_{uuid.uuid4().hex[:8]}"
    strategy_task_data = {
        "task_id": strategy_task_id,
        "task_type": "create_strategy",
        "args": {
            "user_id": 1,
            "prompt": "Test strategy creation",
            "strategy_id": -1
        },
        "created_at": datetime.utcnow().isoformat(),
        "priority": "high"
    }
    redis_client.lpush('strategy_queue_priority', json.dumps(strategy_task_data))
    print(f"  🌟 Added strategy creation task: {strategy_task_id}")
    
    # Add more normal tasks after the priority task
    print("\n📋 Adding more normal priority tasks...")
    for i in range(2):
        task_id = f"screening_{uuid.uuid4().hex[:8]}"
        task_data = {
            "task_id": task_id,
            "task_type": "screening",
            "args": {
                "strategy_ids": [f"strategy_{i+10}"],
                "universe": ["AAPL", "MSFT", "GOOGL"],
                "limit": 10
            },
            "created_at": datetime.utcnow().isoformat(),
            "priority": "normal"
        }
        redis_client.lpush('strategy_queue', json.dumps(task_data))
        print(f"  ➕ Added screening task {i+1}: {task_id}")
    
    # Check queue lengths
    priority_length = redis_client.llen('strategy_queue_priority')
    normal_length = redis_client.llen('strategy_queue')
    
    print(f"\n📊 Queue Status:")
    print(f"  🔥 Priority Queue: {priority_length} tasks")
    print(f"  📄 Normal Queue: {normal_length} tasks")
    print(f"  📈 Total: {priority_length + normal_length} tasks")
    
    # Simulate worker processing to test priority
    print(f"\n🔄 Simulating worker processing order...")
    processed_tasks = []
    
    while True:
        # Check priority queue first (like the worker does)
        priority_task = redis_client.brpop('strategy_queue_priority', timeout=1)
        if priority_task:
            _, task_message = priority_task
            task_data = json.loads(task_message)
            task_type = task_data.get('task_type')
            task_id = task_data.get('task_id')
            priority = task_data.get('priority', 'normal')
            processed_tasks.append({
                'task_id': task_id,
                'task_type': task_type,
                'priority': priority,
                'queue': 'priority'
            })
            print(f"  🌟 Processed from PRIORITY queue: {task_type} ({task_id}) - Priority: {priority}")
            continue
        
        # Check normal queue second
        normal_task = redis_client.brpop('strategy_queue', timeout=1)
        if normal_task:
            _, task_message = normal_task
            task_data = json.loads(task_message)
            task_type = task_data.get('task_type')
            task_id = task_data.get('task_id')
            priority = task_data.get('priority', 'normal')
            processed_tasks.append({
                'task_id': task_id,
                'task_type': task_type,
                'priority': priority,
                'queue': 'normal'
            })
            print(f"  📄 Processed from normal queue: {task_type} ({task_id}) - Priority: {priority}")
            continue
        
        # No more tasks
        break
    
    # Analyze results
    print(f"\n🎯 Processing Results:")
    print(f"  Total tasks processed: {len(processed_tasks)}")
    
    # Check if strategy creation was processed first
    if processed_tasks:
        first_task = processed_tasks[0]
        if first_task['task_type'] == 'create_strategy' and first_task['queue'] == 'priority':
            print("  ✅ SUCCESS: Strategy creation task was processed FIRST (priority queue)")
        else:
            print("  ❌ FAIL: Strategy creation task was NOT processed first")
            print(f"     First task: {first_task['task_type']} from {first_task['queue']} queue")
    
    # Count priority vs normal tasks
    priority_tasks = [t for t in processed_tasks if t['queue'] == 'priority']
    normal_tasks = [t for t in processed_tasks if t['queue'] == 'normal']
    
    print(f"  🔥 Priority queue tasks: {len(priority_tasks)}")
    print(f"  📄 Normal queue tasks: {len(normal_tasks)}")
    
    # Show processing order
    print(f"\n📋 Processing Order:")
    for i, task in enumerate(processed_tasks, 1):
        queue_symbol = "🌟" if task['queue'] == 'priority' else "📄"
        print(f"  {i}. {queue_symbol} {task['task_type']} ({task['task_id'][:12]}...) - {task['queue']} queue")
    
    print(f"\n🎉 Priority queue test completed!")
    return True


if __name__ == "__main__":
    test_priority_queue_system() 