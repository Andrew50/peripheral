#!/usr/bin/env python3
"""
Strategy Worker
Executes trading strategies via Redis queue for backtesting and screening
"""
# pylint: disable=import-error

import os
import sys

import asyncio
import json
import logging
import threading
import time
from datetime import datetime

from agent import python_agent
from backtest import backtest
from screen import screen
from alert import alert
from generator import create_strategy
from utils.conn import Conn
from utils.context import Context, NoSubscribersException
from utils.error_utils import capture_exception
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
    handlers=[logging.StreamHandler()]
)
logger = logging.getLogger(__name__)

class Worker:
    """Redis queue-based strategy execution worker"""
    
    def __init__(self):
        self.worker_id = f"worker_{threading.get_ident()}"
        self.shutdown_requested = False
        self.conn = Conn()
        self.tasks_processed = 0
        self.tasks_failed = 0
        self.tasks_completed = 0
        self._current_task_id = None
        self._current_status_id = None
        self._current_heartbeat_interval = None
        self._task_start_time = None
        self.func_map = {
            'backtest': backtest,
            'screen': screen,
            'alert': alert,
            'create_strategy': create_strategy,
            'python_agent': python_agent
        }
        self._worker_start_time = time.time()
        logger.info("🎯 Strategy worker %s started at %s", self.worker_id, datetime.now().strftime('%Y-%m-%d %H:%M:%S'))

    def run(self):
        """Main queue processing loop with priority queue support"""


        while True:
            logger.info("🔍 Waiting for task on %s", self.worker_id)
            task = self.conn.redis_client.brpop(['priority_task_queue', 'task_queue'], timeout=30)
            if not task:
                self.conn.check_connections()
                continue
            queue_name, task_data = task
            self.tasks_processed += 1
            # parsing of task data, this shouldnt fail unless the task data is malformed which is not task dependent
            # therefore this shouldnt send an error message back as this cannot happen
            try:
                task_data = json.loads(task_data)
            except json.JSONDecodeError as e:
                logger.error("❌ Failed to parse task JSON: %s", e)
                continue
                
            task_id = task_data.get('task_id')
            task_type = task_data.get('task_type')
            kwargs = json.loads(task_data.get('kwargs', '{}'))
            priority = task_data.get('priority', 'normal')
            status_id = task_data.get('status_id')  # Extract status_id for unified channel
            heartbeat_interval = task_data.get('heartbeat_interval')  # Extract heartbeat interval
            if not task_id or not task_type or not status_id or not heartbeat_interval or not priority:
                logger.error("❌ Missing required task data: %s", task_data)
                continue

            func = self.func_map.get(task_type, None)
            if not func:
                logger.error("❌ Unknown task type: %s.", task_type)
                continue


            execution_context = Context(self.conn, task_id, status_id, heartbeat_interval, queue_name, priority, self.worker_id) #new execution context for each task
            kwargs["ctx"] = execution_context
            logger.info("🔧 Executing %s with args: %s", task_type, kwargs)

            result = None
            error_obj = None
            status = "completed"

            try:
                result = func(**kwargs)
                status = "completed"
            except NoSubscribersException:
                status = "cancelled" # Special status for cancelled tasks
            except asyncio.TimeoutError as timeout_error:
                error_obj = capture_exception(logger, timeout_error)
                status = "error"
            except MemoryError as memory_error:
                error_obj = capture_exception(logger, memory_error)
                status = "error"
            except Exception as exec_error: # pylint: disable=broad-exception-caught
                error_obj = capture_exception(logger, exec_error)
                status = "error"
            finally:
                logger.info("💓 Publishing result for task %s %s", task_id, status)
                execution_context.publish_result(result, error_obj, status) #publish result and stop heartbeat
                execution_context.destroy() #stop heartbeat and context

if __name__ == "__main__":
    Worker().run()
