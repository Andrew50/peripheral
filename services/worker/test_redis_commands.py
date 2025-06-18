#!/usr/bin/env python3
"""
Simple Redis command-based test for Worker Pipeline
Uses direct Redis commands to test the queue system
"""

import json
import time
import uuid
from datetime import datetime

import redis


def test_redis_commands():
    """Test the worker pipeline using direct Redis commands"""
    
    # Connect to Redis
    r = redis.Redis(host='localhost', port=6379, decode_responses=True)
    
    print("🔗 Testing Redis connection...")
    try:
        r.ping()
        print("✅ Redis connection successful")
    except Exception as e:
        print(f"❌ Redis connection failed: {e}")
        return False
    
    # Check queue length
    queue_length = r.llen('python_execution_queue')
    print(f"📊 Current queue length: {queue_length}")
    
    # Create a simple test job
    execution_id = f"test_redis_{uuid.uuid4().hex[:8]}"
    test_job = {
        'execution_id': execution_id,
        'python_code': '''
# Simple test job
result = {
    'message': 'Hello from Redis test!',
    'timestamp': '2024-01-01T00:00:00Z',
    'calculation': 10 * 5,
    'success': True
}
save_result('redis_test', result)
        '''.strip(),
        'input_data': {'test': True},
        'timeout_seconds': 30,
        'memory_limit_mb': 128,
        'libraries': [],
        'created_at': datetime.utcnow().isoformat()
    }
    
    # Submit job to queue
    print(f"📤 Submitting job {execution_id} to queue...")
    job_json = json.dumps(test_job)
    result = r.rpush('python_execution_queue', job_json)
    print(f"✅ Job queued at position: {result}")
    
    # Monitor for updates
    print(f"👀 Monitoring execution updates for {execution_id}...")
    pubsub = r.pubsub()
    pubsub.subscribe('python_execution_updates')
    
    start_time = time.time()
    timeout = 60
    
    try:
        for message in pubsub.listen():
            if message['type'] == 'message':
                try:
                    update = json.loads(message['data'])
                    if update.get('execution_id') == execution_id:
                        status = update['status']
                        print(f"📢 Status update: {status}")
                        
                        if status in ['completed', 'failed', 'timeout']:
                            print(f"🏁 Execution finished: {status}")
                            if status == 'completed':
                                print("✅ Test job completed successfully!")
                                if 'output_data' in update:
                                    print(f"📋 Output: {update['output_data']}")
                            else:
                                print(f"❌ Test job failed: {update.get('error_message', 'Unknown error')}")
                            return status == 'completed'
                except json.JSONDecodeError:
                    continue
            
            # Check timeout
            if time.time() - start_time > timeout:
                print("⏰ Monitoring timeout reached")
                break
    finally:
        pubsub.close()
    
    print("❌ Test completed with timeout or no response")
    return False

def test_queue_operations():
    """Test basic queue operations"""
    r = redis.Redis(host='localhost', port=6379, decode_responses=True)
    
    print("\n🔧 Testing queue operations...")
    
    # Check if queue exists and its length
    initial_length = r.llen('python_execution_queue')
    print(f"📊 Initial queue length: {initial_length}")
    
    # Add a test message
    test_message = {"test": "queue_operation", "timestamp": datetime.utcnow().isoformat()}
    r.rpush('test_queue', json.dumps(test_message))
    
    # Retrieve the message
    retrieved = r.lpop('test_queue')
    if retrieved:
        retrieved_data = json.loads(retrieved)
        print(f"✅ Queue operation test: {retrieved_data}")
        return True
    
    print("❌ Queue operation failed")
    return False

def monitor_worker_activity():
    """Monitor worker activity for a short period"""
    r = redis.Redis(host='localhost', port=6379, decode_responses=True)
    
    print("\n👀 Monitoring worker activity for 30 seconds...")
    pubsub = r.pubsub()
    pubsub.subscribe('python_execution_updates')
    
    start_time = time.time()
    activity_count = 0
    
    try:
        for message in pubsub.listen():
            if message['type'] == 'message':
                try:
                    update = json.loads(message['data'])
                    activity_count += 1
                    print(f"📡 Activity {activity_count}: {update.get('execution_id')} -> {update.get('status')}")
                except json.JSONDecodeError:
                    continue
            
            if time.time() - start_time > 30:
                break
    finally:
        pubsub.close()
    
    print(f"📊 Detected {activity_count} worker activities in 30 seconds")
    return activity_count > 0

def main():
    """Main test function"""
    print("Redis Commands Test for Worker Pipeline")
    print("=" * 50)
    
    # Test Redis connection and basic operations
    if not test_queue_operations():
        print("❌ Basic queue operations failed")
        return
    
    # Ask user if worker is running
    print("\n⚠️  Make sure the Python worker is running!")
    print("Start it with: python worker.py")
    response = input("Is the worker running? (y/n): ").lower().strip()
    
    if response != 'y':
        print("Please start the worker first and run this test again.")
        return
    
    # Test the actual worker pipeline
    success = test_redis_commands()
    
    if success:
        print("\n🎉 Redis command test completed successfully!")
    else:
        print("\n❌ Redis command test failed")
    
    # Monitor activity
    monitor_worker_activity()

if __name__ == "__main__":
    main() 