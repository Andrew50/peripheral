#!/usr/bin/env python3
"""
Strategy Worker
Executes trading strategies via Redis queue for backtesting and screening
"""

import json
import traceback
import datetime
import time
import os
import asyncio
import redis
from datetime import datetime, timedelta
from typing import Any, Dict, List
import logging

from services.worker.src.engine import DataFrameStrategyEngine
from services.worker.src.validator import SecurityValidator, SecurityError

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
    handlers=[logging.StreamHandler()]
)
logger = logging.getLogger(__name__)


class StrategyWorker:
    """Redis queue-based strategy execution worker"""
    
    def __init__(self):
        self.redis_client = self._init_redis()
        self.strategy_engine = DataFrameStrategyEngine()
        self.security_validator = SecurityValidator()
        self.worker_id = os.environ.get("HOSTNAME", "worker-1")
        
    def _init_redis(self) -> redis.Redis:
        """Initialize Redis connection"""
        redis_host = os.environ.get("REDIS_HOST", "cache")
        redis_port = int(os.environ.get("REDIS_PORT", "6379"))
        redis_password = os.environ.get("REDIS_PASSWORD", "")
        
        return redis.Redis(
            host=redis_host,
            port=redis_port,
            password=redis_password if redis_password else None,
            decode_responses=True,
            socket_connect_timeout=10,
            socket_timeout=10,
            retry_on_timeout=True,
            health_check_interval=30
        )
    
    def run(self):
        """Main queue processing loop"""
        logger.info(f"Strategy worker {self.worker_id} starting queue processing...")
        
        while True:
            try:
                # Block and wait for tasks from Redis queue
                task = self.redis_client.brpop('strategy_queue', timeout=60)
                
                if not task:
                    # No task received, check connection and continue
                    self._check_connection()
                    logger.info("No tasks, waiting...")
                    continue
                    
                # Parse task
                _, task_message = task
                task_data = json.loads(task_message)
                task_id = task_data.get('task_id')
                task_type = task_data.get('task_type')
                args = task_data.get('args', {})

                logger.info(f"Processing {task_type} task {task_id}")
                
                # Validate task data
                if not task_id or not task_type:
                    logger.error(f"Invalid task data: {task_data}")
                    continue
                    
                if task_type not in ['backtest', 'screening']:
                    error_msg = f"Unknown task type: {task_type}"
                    logger.error(error_msg)
                    self._set_task_result(task_id, "error", {"error": error_msg})
                    continue

                try:
                    # Set task status to running
                    self._set_task_result(task_id, "running", {
                        "worker_id": self.worker_id,
                        "started_at": datetime.utcnow().isoformat()
                    })
                    
                    start_time = time.time()
                    
                    # Execute the task
                    if task_type == 'backtest':
                        result = asyncio.run(self._execute_backtest(**args))
                    elif task_type == 'screening':
                        result = asyncio.run(self._execute_screening(**args))
                    
                    # Calculate execution time
                    execution_time = time.time() - start_time
                    
                    # Store successful result
                    result['execution_time_seconds'] = execution_time
                    result['worker_id'] = self.worker_id
                    result['completed_at'] = datetime.utcnow().isoformat()
                    
                    self._set_task_result(task_id, "completed", result)
                    logger.info(f"Completed {task_type} task {task_id} in {execution_time:.2f}s")
                    
                except SecurityError as e:
                    # Security validation error
                    error_result = {
                        "error": f"Security validation failed: {str(e)}",
                        "completed_at": datetime.utcnow().isoformat()
                    }
                    self._set_task_result(task_id, "error", error_result)
                    logger.error(f"Security error in task {task_id}: {e}")
                    
                except Exception as e:
                    # General error - log and set error status
                    error_result = {
                        "error": str(e),
                        "traceback": traceback.format_exc(),
                        "completed_at": datetime.utcnow().isoformat()
                    }
                    self._set_task_result(task_id, "error", error_result)
                    logger.error(f"Task execution error in {task_id}: {e}")
                    
            except KeyboardInterrupt:
                logger.info("Received interrupt signal, shutting down worker...")
                break
                
            except Exception as e:
                logger.error(f"Unexpected error in main loop: {e}")
                time.sleep(5)  # Brief pause before continuing
        
        # Cleanup
        self.redis_client.close()
        logger.info("Worker shutdown complete")
    
    async def _execute_backtest(self, strategy_code: str, symbols: List[str], 
                               start_date: str = None, end_date: str = None, **kwargs) -> Dict[str, Any]:
        """Execute backtest task"""
        logger.info(f"Starting backtest for {len(symbols)} symbols")
        
        # Validate strategy code
        if not self.security_validator.validate(strategy_code):
            raise SecurityError("Strategy code contains prohibited operations")
        
        # Parse dates
        if start_date:
            start_date = datetime.fromisoformat(start_date.replace('Z', '+00:00'))
        else:
            start_date = datetime.now() - timedelta(days=365)  # Default 1 year
            
        if end_date:
            end_date = datetime.fromisoformat(end_date.replace('Z', '+00:00'))
        else:
            end_date = datetime.now()
        
        # Execute using DataFrame engine
        result = await self.strategy_engine.execute_backtest(
            strategy_code=strategy_code,
            symbols=symbols,
            start_date=start_date,
            end_date=end_date,
            **kwargs
        )
        
        logger.info(f"Backtest completed: {len(result.get('instances', []))} instances found")
        return result
    
    async def _execute_screening(self, strategy_code: str, universe: List[str], 
                                limit: int = 100, **kwargs) -> Dict[str, Any]:
        """Execute screening task"""
        logger.info(f"Starting screening for {len(universe)} symbols, limit {limit}")
        
        # Validate strategy code
        if not self.security_validator.validate(strategy_code):
            raise SecurityError("Strategy code contains prohibited operations")
        
        # Execute using DataFrame engine
        result = await self.strategy_engine.execute_screening(
            strategy_code=strategy_code,
            universe=universe,
            limit=limit,
            **kwargs
        )
        
        logger.info(f"Screening completed: {len(result.get('ranked_results', []))} results found")
        return result
    
    def _set_task_result(self, task_id: str, status: str, data: Dict[str, Any]):
        """Set task result in Redis"""
        try:
            result = {
                "status": status,
                "data": data,
                "updated_at": datetime.utcnow().isoformat()
            }
            
            # Store result with 24 hour expiration
            self.redis_client.setex(f"task_result:{task_id}", 86400, json.dumps(result))
            
        except Exception as e:
            logger.error(f"Failed to set task result for {task_id}: {e}")
    
    def _check_connection(self):
        """Check and restore Redis connection if needed"""
        try:
            self.redis_client.ping()
        except Exception as e:
            logger.error(f"Redis connection lost, reconnecting: {e}")
            self.redis_client = self._init_redis()


# Utility functions for adding tasks to queue
def add_backtest_task(redis_client: redis.Redis, task_id: str, strategy_code: str, 
                     symbols: List[str], start_date: str = None, end_date: str = None) -> None:
    """Add a backtest task to the queue"""
    task_data = {
        "task_id": task_id,
        "task_type": "backtest",
        "args": {
            "strategy_code": strategy_code,
            "symbols": symbols,
            "start_date": start_date,
            "end_date": end_date
        }
    }
    redis_client.lpush('strategy_queue', json.dumps(task_data))


def add_screening_task(redis_client: redis.Redis, task_id: str, strategy_code: str, 
                      universe: List[str], limit: int = 100) -> None:
    """Add a screening task to the queue"""
    task_data = {
        "task_id": task_id,
        "task_type": "screening",
        "args": {
            "strategy_code": strategy_code,
            "universe": universe,
            "limit": limit
        }
    }
    redis_client.lpush('strategy_queue', json.dumps(task_data))


def get_task_result(redis_client: redis.Redis, task_id: str) -> Dict[str, Any]:
    """Get task result from Redis"""
    result_json = redis_client.get(f"task_result:{task_id}")
    if result_json:
        return json.loads(result_json)
    return None


if __name__ == "__main__":
    worker = StrategyWorker()
    worker.run()
