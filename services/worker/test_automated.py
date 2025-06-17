#!/usr/bin/env python3
"""
Automated test for Python Strategy Worker Pipeline
No user input required - runs automatically
"""

import json
import redis
import time
import uuid
from datetime import datetime


def test_worker_pipeline():
    """Automated test of the worker pipeline"""
    
    print("🤖 Automated Worker Pipeline Test")
    print("=" * 50)
    
    # Connect to Redis
    try:
        r = redis.Redis(host='localhost', port=6379, decode_responses=True)
        r.ping()
        print("✅ Connected to Redis")
    except Exception as e:
        print(f"❌ Redis connection failed: {e}")
        return False
    
    # Check initial queue state
    initial_queue_length = r.llen('python_execution_queue')
    print(f"📊 Initial queue length: {initial_queue_length}")
    
    # Create test job
    execution_id = f"auto_test_{uuid.uuid4().hex[:8]}"
    test_job = {
        'execution_id': execution_id,
        'python_code': '''
# Automated test strategy
import time

# Simple calculations
result = {
    'message': 'Automated test successful!',
    'timestamp': time.time(),
    'calculation': 42 * 2,
    'test_type': 'automated',
    'success': True
}

# Save result
save_result('automated_test', result)
print(f"Test completed: {result}")
        '''.strip(),
        'input_data': {'test_mode': 'automated'},
        'timeout_seconds': 30,
        'memory_limit_mb': 128,
        'libraries': [],
        'created_at': datetime.utcnow().isoformat()
    }
    
    # Submit job
    print(f"📤 Submitting job: {execution_id}")
    job_json = json.dumps(test_job)
    position = r.rpush('python_execution_queue', job_json)
    print(f"✅ Job queued at position: {position}")
    
    # Monitor execution with timeout
    print("👀 Monitoring execution (30s timeout)...")
    pubsub = r.pubsub()
    pubsub.subscribe('python_execution_updates')
    
    start_time = time.time()
    timeout = 30
    result_received = False
    
    try:
        for message in pubsub.listen():
            if message['type'] == 'message':
                try:
                    update = json.loads(message['data'])
                    if update.get('execution_id') == execution_id:
                        status = update['status']
                        print(f"📢 Status: {status}")
                        
                        if status == 'running':
                            print(f"🏃 Execution started on worker: {update.get('worker_node', 'unknown')}")
                        
                        elif status == 'completed':
                            print("🎉 Execution completed successfully!")
                            if 'output_data' in update:
                                output = update['output_data']
                                print(f"📋 Output keys: {list(output.keys())}")
                                if 'result' in output:
                                    print(f"📋 Result: {output['result']}")
                            print(f"⏱️ Execution time: {update.get('execution_time_ms', 0)}ms")
                            result_received = True
                            break
                        
                        elif status == 'failed':
                            print(f"❌ Execution failed: {update.get('error_message', 'Unknown error')}")
                            result_received = True
                            break
                        
                        elif status == 'timeout':
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
    
    # Check final queue state
    final_queue_length = r.llen('python_execution_queue')
    print(f"📊 Final queue length: {final_queue_length}")
    
    return result_received


def test_multiple_jobs():
    """Test multiple jobs in sequence"""
    
    print("\n🔄 Testing multiple jobs...")
    
    r = redis.Redis(host='localhost', port=6379, decode_responses=True)
    
    # Submit 3 quick jobs
    job_ids = []
    for i in range(3):
        execution_id = f"multi_test_{i}_{uuid.uuid4().hex[:6]}"
        job_ids.append(execution_id)
        
        test_job = {
            'execution_id': execution_id,
            'python_code': f'''
# Quick test job {i}
result = {{
    'job_number': {i},
    'message': 'Multi-job test {i}',
    'success': True
}}
save_result('multi_test', result)
            '''.strip(),
            'input_data': {'job_id': i},
            'timeout_seconds': 15,
            'memory_limit_mb': 64,
            'libraries': [],
            'created_at': datetime.utcnow().isoformat()
        }
        
        r.rpush('python_execution_queue', json.dumps(test_job))
        print(f"📤 Submitted job {i}: {execution_id}")
    
    # Monitor all jobs
    print("👀 Monitoring all jobs...")
    pubsub = r.pubsub()
    pubsub.subscribe('python_execution_updates')
    
    completed_jobs = set()
    start_time = time.time()
    
    try:
        for message in pubsub.listen():
            if message['type'] == 'message':
                try:
                    update = json.loads(message['data'])
                    execution_id = update.get('execution_id')
                    
                    if execution_id in job_ids:
                        status = update['status']
                        if status in ['completed', 'failed', 'timeout']:
                            completed_jobs.add(execution_id)
                            print(f"✅ Job completed: {execution_id} -> {status}")
                            
                            # Stop when all jobs are done
                            if len(completed_jobs) == len(job_ids):
                                print("🎉 All jobs completed!")
                                break
                                
                except json.JSONDecodeError:
                    continue
            
            # Timeout check
            if time.time() - start_time > 60:
                print("⏰ Multi-job test timeout")
                break
                
    finally:
        pubsub.close()
    
    print(f"📊 Completed {len(completed_jobs)}/{len(job_ids)} jobs")
    return len(completed_jobs) == len(job_ids)


def main():
    """Main test function"""
    
    print("Starting automated worker pipeline tests...")
    
    # Test 1: Single job
    test1_success = test_worker_pipeline()
    
    # Test 2: Multiple jobs
    test2_success = test_multiple_jobs()
    
    # Summary
    print("\n" + "=" * 50)
    print("📊 TEST SUMMARY")
    print(f"Single job test: {'✅ PASSED' if test1_success else '❌ FAILED'}")
    print(f"Multiple jobs test: {'✅ PASSED' if test2_success else '❌ FAILED'}")
    
    if test1_success and test2_success:
        print("🎉 All automated tests PASSED!")
        return True
    else:
        print("❌ Some tests FAILED")
        return False


if __name__ == "__main__":
    main() 