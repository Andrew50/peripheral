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
import signal
from datetime import datetime, timedelta
from typing import Any, Dict, List
import logging

import sys
import os
import psycopg2
from psycopg2.extras import RealDictCursor
from contextlib import contextmanager
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

from engine import StrategyEngine
from validator import SecurityValidator, SecurityError
from generator import StrategyGenerator
from concurrent.futures import ThreadPoolExecutor
import threading
from utils.data_accessors import DataAccessorProvider
from agent import PythonAgentGenerator
from utils.conn import Conn
from utils.strategy_crud import StrategyCRUD
from utils.context import ExecutionContext, NoSubscribersException
from entry import backtest, screen, alert, create_strategy, python_agent
from openai import OpenAI
from google import genai

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
    handlers=[logging.StreamHandler()]
)
logger = logging.getLogger(__name__)


class TaskContext:
    def __init__(self, conn: Conn, strategy_generator: StrategyGenerator, python_agent_generator: PythonAgentGenerator, task_id: str, status_id: str, heartbeat_interval: int, queue_type: str, priority: str, worker_id: str):
        self.conn = conn
        self.ctx = executionContext
        self.openai_client = openai_client
        self.gemini_client = gemini_client


    


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

    def run(self):
        """Main queue processing loop with priority queue support"""
        logger.info(f"🎯 Strategy worker {self.worker_id} starting queue processing...")
        self._worker_start_time = time.time()


        openai_client = OpenAI(api_key=os.getenv("OPENAI_API_KEY"))
        gemini_client = GoogleGenerativeAI(api_key=os.getenv("GEMINI_API_KEY"))
        
        while True:
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
                logger.error(f"❌ Failed to parse task JSON: {e}")
                continue
                
            task_id = task_data.get('task_id')
            task_type = task_data.get('task_type')
            kwargs = json.loads(task_data.get('kwargs', '{}'))
            priority = task_data.get('priority', 'normal')
            status_id = task_data.get('status_id')  # Extract status_id for unified channel
            heartbeat_interval = task_data.get('heartbeat_interval')  # Extract heartbeat interval
            if not task_id or not task_type or not status_id or not heartbeat_interval or not priority:
                logger.error(f"❌ Missing required task data: {task_data}")
                continue

            func = self.func_map.get(task_type, None)
            if not func:
                logger.error(f"❌ Unknown task type: {task_type}.")
                continue

            execution_context = ExecutionContext(self.conn, strategy_generator, python_agent_generator, task_id, status_id, heartbeat_interval, queue_name, priority, self.worker_id) #new execution context for each task
            logger.debug(f"🔧 Executing {task_type} with args: {kwargs}")
                    
            result = None
            error = None

            try:
                result = asyncio.run(func(TaskContext(execution_context, self.conn, openai_client, gemini_client), **kwargs))
                status = "completed"
            except NoSubscribersException as e:
                logger.warning(f"Task {task_id} cancelled: {e}")
                status = "cancelled" # Special status for cancelled tasks
            except asyncio.TimeoutError as timeout_error:
                logger.error(f"💥 Task {task_id} timed out: {timeout_error}")
                status = "error"
                error = f"Task execution timed out: {str(timeout_error)}"
            except MemoryError as memory_error:
                logger.error(f"💥 Task {task_id} ran out of memory: {memory_error}")
                status = "error"
                error = f"Task execution failed due to memory constraints: {str(memory_error)}"
            except Exception as exec_error:
                error = f"Task execution failed: {str(exec_error)}"
            finally:
                execution_context.publish_result(result, status, error) #publish result and stop heartbeat
                execution_context.destroy() #stop heartbeat and context

if __name__ == "__main__":
    Worker().run()