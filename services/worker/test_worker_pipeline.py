#!/usr/bin/env python3
"""
Test script for Python Strategy Worker Pipeline
Tests the complete worker pipeline using Redis commands and monitoring
"""

import asyncio
import json
import logging
import time
import uuid
from datetime import datetime
from typing import Any, Dict

import redis

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class WorkerPipelineTester:
    """Test class for the worker pipeline"""
    
    def __init__(self):
        self.redis_client = redis.Redis(
            host='localhost',
            port=6379,
            decode_responses=True,
            socket_connect_timeout=5,
            socket_timeout=5
        )
        self.test_results = []
    
    def test_redis_connection(self):
        """Test Redis connection"""
        try:
            response = self.redis_client.ping()
            logger.info(f"✓ Redis connection test: {response}")
            return True
        except Exception as e:
            logger.error(f"✗ Redis connection failed: {e}")
            return False
    
    def create_test_job(self, job_name: str, python_code: str, input_data: Dict = None) -> Dict[str, Any]:
        """Create a test job"""
        execution_id = f"test_{job_name}_{uuid.uuid4().hex[:8]}"
        
        job = {
            'execution_id': execution_id,
            'python_code': python_code,
            'input_data': input_data or {},
            'timeout_seconds': 60,
            'memory_limit_mb': 256,
            'libraries': ['pandas', 'numpy'],
            'created_at': datetime.utcnow().isoformat()
        }
        
        return job
    
    def submit_job_to_queue(self, job: Dict[str, Any]) -> bool:
        """Submit a job to the Redis queue"""
        try:
            job_json = json.dumps(job)
            result = self.redis_client.rpush('python_execution_queue', job_json)
            logger.info(f"✓ Job {job['execution_id']} submitted to queue (position: {result})")
            return True
        except Exception as e:
            logger.error(f"✗ Failed to submit job: {e}")
            return False
    
    def monitor_execution_updates(self, execution_id: str, timeout: int = 60) -> Dict[str, Any]:
        """Monitor execution updates for a specific job"""
        pubsub = self.redis_client.pubsub()
        pubsub.subscribe('python_execution_updates')
        
        start_time = time.time()
        logger.info(f"Monitoring updates for execution {execution_id}...")
        
        try:
            for message in pubsub.listen():
                if message['type'] == 'message':
                    try:
                        update = json.loads(message['data'])
                        if update.get('execution_id') == execution_id:
                            logger.info(f"📢 Update for {execution_id}: {update['status']}")
                            
                            if update['status'] in ['completed', 'failed', 'timeout']:
                                logger.info(f"✓ Execution {execution_id} finished with status: {update['status']}")
                                return update
                    except json.JSONDecodeError:
                        continue
                
                # Check timeout
                if time.time() - start_time > timeout:
                    logger.warning(f"⚠️ Monitoring timeout for {execution_id}")
                    break
        finally:
            pubsub.close()
        
        return {'status': 'timeout', 'execution_id': execution_id}
    
    def check_queue_status(self):
        """Check the current queue status"""
        try:
            queue_length = self.redis_client.llen('python_execution_queue')
            logger.info(f"📊 Current queue length: {queue_length}")
            return queue_length
        except Exception as e:
            logger.error(f"✗ Failed to check queue status: {e}")
            return -1
    
    def test_simple_strategy(self):
        """Test a simple strategy execution"""
        logger.info("🔧 Testing simple strategy...")
        
        simple_code = '''
# Simple test strategy
result = {
    'test': 'hello from worker',
    'timestamp': '2024-01-01T00:00:00Z',
    'calculation': 2 + 2,
    'success': True
}

# Save the result
save_result('test_output', result)
'''
        
        job = self.create_test_job('simple', simple_code, {'symbol': 'TEST'})
        
        if self.submit_job_to_queue(job):
            update = self.monitor_execution_updates(job['execution_id'])
            self.test_results.append({
                'test': 'simple_strategy',
                'success': update.get('status') == 'completed',
                'details': update
            })
            return update.get('status') == 'completed'
        
        return False
    
    def test_data_access_strategy(self):
        """Test strategy with data access functions"""
        logger.info("🔧 Testing data access strategy...")
        
        data_code = '''
# Test data access functions
try:
    # Test price data access
    price_data = get_price_data('AAPL', timeframe='1d', days=30)
    
    # Test security info
    info = get_security_info('AAPL')
    
    result = {
        'price_data_available': bool(price_data and price_data.get('close')),
        'price_data_length': len(price_data.get('close', [])) if price_data else 0,
        'security_info_available': bool(info),
        'test_passed': True
    }
    
    save_result('data_test', result)
    
except Exception as e:
    save_result('error', str(e))
    save_result('test_passed', False)
'''
        
        job = self.create_test_job('data_access', data_code, {'symbol': 'AAPL'})
        
        if self.submit_job_to_queue(job):
            update = self.monitor_execution_updates(job['execution_id'])
            self.test_results.append({
                'test': 'data_access_strategy',
                'success': update.get('status') == 'completed',
                'details': update
            })
            return update.get('status') == 'completed'
        
        return False
    
    def test_calculation_strategy(self):
        """Test strategy with calculations"""
        logger.info("🔧 Testing calculation strategy...")
        
        calc_code = '''
# Test mathematical calculations
import math

def calculate_sma(prices, period):
    """Simple Moving Average"""
    if len(prices) < period:
        return []
    return [sum(prices[i-period+1:i+1])/period for i in range(period-1, len(prices))]

def calculate_rsi(prices, period=14):
    """Relative Strength Index"""
    if len(prices) < period + 1:
        return []
    
    deltas = [prices[i] - prices[i-1] for i in range(1, len(prices))]
    gains = [d if d > 0 else 0 for d in deltas]
    losses = [-d if d < 0 else 0 for d in deltas]
    
    avg_gain = sum(gains[:period]) / period
    avg_loss = sum(losses[:period]) / period
    
    if avg_loss == 0:
        return [100]
    
    rs = avg_gain / avg_loss
    rsi = 100 - (100 / (1 + rs))
    
    return [rsi]

# Test data
test_prices = [100, 102, 101, 103, 105, 104, 106, 108, 107, 109, 111, 110, 112, 114, 113]

# Calculations
sma_5 = calculate_sma(test_prices, 5)
rsi_14 = calculate_rsi(test_prices, 14)

result = {
    'sma_calculated': len(sma_5) > 0,
    'rsi_calculated': len(rsi_14) > 0,
    'sma_last': sma_5[-1] if sma_5 else None,
    'rsi_last': rsi_14[-1] if rsi_14 else None,
    'calculations_successful': True
}

save_result('calculation_test', result)
'''
        
        job = self.create_test_job('calculation', calc_code)
        
        if self.submit_job_to_queue(job):
            update = self.monitor_execution_updates(job['execution_id'])
            self.test_results.append({
                'test': 'calculation_strategy',
                'success': update.get('status') == 'completed',
                'details': update
            })
            return update.get('status') == 'completed'
        
        return False
    
    def test_security_validation(self):
        """Test security validation by submitting prohibited code"""
        logger.info("🔧 Testing security validation...")
        
        malicious_code = '''
# This should be blocked by security validation
import os
import subprocess

# Try to execute system command (should be blocked)
result = os.system('ls -la')
save_result('security_breach', result)
'''
        
        job = self.create_test_job('security_test', malicious_code)
        
        if self.submit_job_to_queue(job):
            update = self.monitor_execution_updates(job['execution_id'])
            # Security test passes if the job fails due to security violation
            success = update.get('status') == 'failed' and 'security' in update.get('error_message', '').lower()
            self.test_results.append({
                'test': 'security_validation',
                'success': success,
                'details': update
            })
            return success
        
        return False
    
    def run_all_tests(self):
        """Run all tests"""
        logger.info("🚀 Starting Worker Pipeline Tests")
        logger.info("=" * 50)
        
        # Test Redis connection first
        if not self.test_redis_connection():
            logger.error("❌ Cannot proceed without Redis connection")
            return False
        
        # Check initial queue status
        self.check_queue_status()
        
        # Run tests
        tests = [
            ('Simple Strategy', self.test_simple_strategy),
            ('Data Access Strategy', self.test_data_access_strategy),
            ('Calculation Strategy', self.test_calculation_strategy),
            ('Security Validation', self.test_security_validation)
        ]
        
        passed = 0
        total = len(tests)
        
        for test_name, test_func in tests:
            logger.info(f"\n{'='*20} {test_name} {'='*20}")
            try:
                if test_func():
                    logger.info(f"✅ {test_name} PASSED")
                    passed += 1
                else:
                    logger.error(f"❌ {test_name} FAILED")
            except Exception as e:
                logger.error(f"❌ {test_name} ERROR: {e}")
        
        # Summary
        logger.info("\n" + "="*50)
        logger.info(f"📊 TEST SUMMARY: {passed}/{total} tests passed")
        
        if passed == total:
            logger.info("🎉 All tests passed!")
        else:
            logger.warning(f"⚠️ {total - passed} tests failed")
        
        # Check final queue status
        self.check_queue_status()
        
        return passed == total


async def main():
    """Main test runner"""
    tester = WorkerPipelineTester()
    
    print("Python Strategy Worker Pipeline Tester")
    print("=" * 50)
    print("This script will test the worker pipeline by:")
    print("1. Connecting to Redis")
    print("2. Submitting test jobs to the queue")
    print("3. Monitoring execution updates")
    print("4. Validating results")
    print("\nMake sure the worker is running in another terminal!")
    print("=" * 50)
    
    input("Press Enter to start tests...")
    
    success = tester.run_all_tests()
    
    if success:
        print("\n🎉 All pipeline tests completed successfully!")
    else:
        print("\n⚠️ Some tests failed. Check the logs above.")
    
    return success


if __name__ == "__main__":
    asyncio.run(main()) 