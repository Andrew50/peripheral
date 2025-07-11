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
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

from src.strategy_engine import AccessorStrategyEngine
from src.validator import SecurityValidator, SecurityError
from src.strategy_generator import StrategyGenerator
from concurrent.futures import ThreadPoolExecutor
import threading
from src.data_accessors import DataAccessorProvider

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
        self.worker_id = f"worker_{threading.get_ident()}"
        self.shutdown_requested = False
        
        # Set up signal handlers for graceful shutdown
        self._setup_signal_handlers()
        
        # Initialize Redis connection
        self.redis_client = self._init_redis()
        # Clean up any stale heartbeats from previous instances
        self._cleanup_stale_heartbeats()
        # Initialize Database connection  
        self.db_conn = self._init_database()
        
        # Import the new data accessor
        self.data_accessor = DataAccessorProvider()
        
        self.strategy_engine = AccessorStrategyEngine()
        self.security_validator = SecurityValidator()
        self.strategy_generator = StrategyGenerator()
        
        # Log initial queue status
        self.log_queue_status()
    
    def _setup_signal_handlers(self):
        """Set up signal handlers for graceful shutdown and crash detection"""
        def signal_handler(signum, frame):
            signal_name = signal.Signals(signum).name
            logger.error(f"🚨 Received signal {signal_name} ({signum}) - initiating graceful shutdown")
            self.shutdown_requested = True
            
            # Clean up heartbeat on shutdown
            try:
                self._cleanup_heartbeat()
            except Exception as e:
                logger.error(f"❌ Error during signal cleanup: {e}")
            
            # Log the current stack trace to help debug
            logger.error(f"📄 Signal received at:")
            for line in traceback.format_stack(frame):
                logger.error(f"   {line.strip()}")
        
        # Handle common termination signals
        signal.signal(signal.SIGTERM, signal_handler)
        signal.signal(signal.SIGINT, signal_handler)
        
        # Handle segmentation fault (if possible)
        try:
            signal.signal(signal.SIGSEGV, signal_handler)
        except (OSError, ValueError):
            # SIGSEGV might not be available on all platforms
            pass
        
        
    def _init_redis(self) -> redis.Redis:
        """Initialize Redis connection"""
        redis_host = os.environ.get("REDIS_HOST", "cache")
        redis_port = int(os.environ.get("REDIS_PORT", "6379"))
        redis_password = os.environ.get("REDIS_PASSWORD", "")
        
        client = redis.Redis(
            host=redis_host,
            port=redis_port,
            password=redis_password if redis_password else None,
            decode_responses=True,
            socket_connect_timeout=5,
            socket_timeout=15,  # Shorter timeout for faster responsiveness
            health_check_interval=30
        )
        
        # Test connection
        try:
            client.ping()
            
            # Test queue access
            self._test_queue_access(client)
            
        except Exception as e:
            logger.error(f"❌ Redis connection failed: {e}")
            raise
        
        return client

    def _test_queue_access(self, redis_client: redis.Redis):
        """Test access to both priority and normal queues"""
        try:
            
            # Test priority queue
            priority_length = redis_client.llen('strategy_queue_priority')
            
            # Test normal queue
            normal_length = redis_client.llen('strategy_queue')
            logger.info(f"   📄 Normal queue length: {normal_length + priority_length}")
            
            # Test publish capability
            test_channel = "worker_test_channel"
            test_message = {"test": "message", "timestamp": datetime.utcnow().isoformat()}
            subscribers = redis_client.publish(test_channel, json.dumps(test_message))
            
        except Exception as e:
            logger.error(f"❌ Queue access test failed: {e}")
            raise
    
    def _init_database(self):
        """Initialize database connection"""
        db_host = os.environ.get("DB_HOST", "db")
        db_port = os.environ.get("DB_PORT", "5432")
        db_name = os.environ.get("POSTGRES_DB", "postgres")
        db_user = os.environ.get("DB_USER", "postgres")
        db_password = os.environ.get("DB_PASSWORD", "devpassword")
        
        try:
            connection = psycopg2.connect(
                host=db_host,
                port=db_port,
                database=db_name,
                user=db_user,
                password=db_password,
                cursor_factory=RealDictCursor
            )
            return connection
        except Exception as e:
            logger.error(f"Failed to connect to database: {e}")
            raise
    
    def _ensure_db_connection(self):
        """Ensure database connection is healthy, reconnect if needed"""
        try:
            # Test the connection with a simple query
            with self.db_conn.cursor() as cursor:
                cursor.execute("SELECT 1")
                cursor.fetchone()
        except (psycopg2.OperationalError, psycopg2.InterfaceError, AttributeError) as e:
            logger.warning(f"Database connection test failed, reconnecting: {e}")
            try:
                if hasattr(self, 'db_conn') and self.db_conn:
                    self.db_conn.close()
            except Exception:
                logger.debug("Error closing database connection (expected during reconnection)")
            self.db_conn = self._init_database()
        except Exception as e:
            logger.error(f"Unexpected error testing database connection: {e}")
            # For other errors, don't reconnect to avoid infinite loops
            pass
    
    def _fetch_strategy_code(self, strategy_id: str) -> str:
        """Fetch strategy code from database by strategy_id with connection recovery"""
        max_retries = 3
        for attempt in range(max_retries):
            try:
                # Test connection health before use
                self._ensure_db_connection()
                
                with self.db_conn.cursor() as cursor:
                    # Fetch from consolidated strategies table
                    cursor.execute(
                        "SELECT pythonCode FROM strategies WHERE strategyId = %s AND is_active = true",
                        (strategy_id,)
                    )
                    result = cursor.fetchone()
                    
                    if result and result['pythoncode']:
                        return result['pythoncode']
                    
                    raise ValueError(f"Strategy not found or has no Python code for strategy_id: {strategy_id}")
                    
            except (psycopg2.OperationalError, psycopg2.InterfaceError) as e:
                logger.warning(f"Database connection error on attempt {attempt + 1}/{max_retries}: {e}")
                if attempt < max_retries - 1:
                    # Try to reconnect
                    try:
                        self.db_conn.close()
                    except Exception:
                        logger.debug("Error closing database connection (expected during reconnection)")
                    self.db_conn = self._init_database()
                else:
                    logger.error(f"Failed to fetch strategy code after {max_retries} attempts")
                    raise
            except Exception as e:
                logger.error(f"Failed to fetch strategy code for strategy_id {strategy_id}: {e}")
                raise
    
    
    def _fetch_multiple_strategy_codes(self, strategy_ids: List[str]) -> Dict[str, str]:
        """Fetch multiple strategy codes from database"""
        strategy_codes = {}
        
        for strategy_id in strategy_ids:
            try:
                strategy_codes[strategy_id] = self._fetch_strategy_code(strategy_id)
            except Exception as e:
                logger.error(f"Failed to fetch strategy {strategy_id}: {e}")
                # Continue with other strategies
                continue
        
        return strategy_codes
    
    def run(self):
        """Main queue processing loop with priority queue support"""
        logger.info(f"🎯 Strategy worker {self.worker_id} starting queue processing...")
        
        # Set start time for uptime tracking
        self._start_time = time.time()
        
        # Start asynchronous heartbeat thread
        self._start_heartbeat_thread()
        
        # Track last queue status log time
        last_status_log = time.time()
        status_log_interval = 600  # Log queue status every 10 minutes when idle
        
        # Track statistics
        tasks_processed = 0
        priority_tasks_processed = 0
        normal_tasks_processed = 0
        
        while not self.shutdown_requested:
            try:
                # Check for shutdown signal
                if self.shutdown_requested:
                    logger.info("🛑 Shutdown requested, exiting main loop")
                    break
                
                # Use Redis BRPOP with multiple queues for efficient, atomic queue checking
                # Priority queue is checked first automatically by Redis
                task = self.redis_client.brpop(['strategy_queue_priority', 'strategy_queue'], timeout=10)
                
                if not task:
                    # No task received within timeout
                    self._check_connection()
                    
                    # Check for shutdown again after connection check
                    if self.shutdown_requested:
                        logger.info("🛑 Shutdown requested during idle period")
                        break
                    
                    # Periodically log queue status when idle
                    current_time = time.time()
                    if current_time - last_status_log > status_log_interval:
                        self.log_queue_status()
                        last_status_log = current_time
                    
                    continue
                
                # Determine queue type from Redis response
                queue_name, task_message = task
                if queue_name == 'strategy_queue_priority':
                    queue_type = "priority"
                    priority_tasks_processed += 1
                else:
                    queue_type = "normal"
                    normal_tasks_processed += 1
                    
                # Parse task
                logger.debug(f"📦 Received {queue_type} task (Total: P:{priority_tasks_processed}/N:{normal_tasks_processed})")
                
                try:
                    task_data = json.loads(task_message)
                except json.JSONDecodeError as e:
                    logger.error(f"❌ Failed to parse task JSON: {e}")
                    continue
                    
                task_id = task_data.get('task_id')
                task_type = task_data.get('task_type')
                args = task_data.get('args', {})
                priority = task_data.get('priority', 'normal')

                logger.info(f"🎯 Processing {task_type} task {task_id}")
                tasks_processed += 1
                
                # Validate task data
                if not task_id or not task_type:
                    logger.error(f"❌ Invalid task data - missing task_id or task_type: {task_data}")
                    continue
                    
                if task_type not in ['backtest', 'screening', 'alert', 'create_strategy']:
                    error_msg = f"Unknown task type: {task_type}"
                    logger.error(f"❌ {error_msg}")
                    self._set_task_result(task_id, "error", {"error": error_msg})
                    continue

                try:
                    # Set task status to running
                    logger.debug(f"▶️ Starting execution of {task_type} task {task_id}")
                    
                    # Track current task for heartbeat monitoring
                    self._current_task_id = task_id
                    
                    self._set_task_result(task_id, "running", {
                        "worker_id": self.worker_id,
                        "queue_type": queue_type,
                        "priority": priority,
                        "started_at": datetime.utcnow().isoformat(),
                        "original_task": {
                            "task_type": task_type,
                            "args": args,
                            "priority": priority,
                            "queue_type": queue_type
                        }
                    })
                    
                    start_time = time.time()
                    
                    # Execute the task with comprehensive error handling
                    logger.debug(f"🔧 Executing {task_type} with args: {json.dumps(args, indent=2)}")
                    
                    result = None
                    try:
                        if task_type == 'backtest':
                            result = asyncio.run(self._execute_backtest(task_id=task_id, **args))
                        elif task_type == 'screening':
                            result = asyncio.run(self._execute_screening(task_id=task_id, **args))
                        elif task_type == 'alert':
                            result = asyncio.run(self._execute_alert(task_id=task_id, **args))
                        elif task_type == 'create_strategy':
                            logger.info(f"🧠 Starting strategy creation for user {args.get('user_id')} with prompt: {args.get('prompt', '')[:100]}...")
                            result = asyncio.run(self._execute_create_strategy(task_id=task_id, **args))
                    except asyncio.TimeoutError as timeout_error:
                        logger.error(f"⏰ Task {task_id} timed out: {timeout_error}")
                        raise Exception(f"Task execution timed out: {str(timeout_error)}")
                    except MemoryError as memory_error:
                        logger.error(f"💾 Task {task_id} ran out of memory: {memory_error}")
                        raise Exception(f"Task execution failed due to memory constraints: {str(memory_error)}")
                    except Exception as exec_error:
                        logger.error(f"💥 Task {task_id} execution failed: {exec_error}")
                        logger.error(f"📄 Execution traceback: {traceback.format_exc()}")
                        raise exec_error
                    finally:
                        # Clear current task tracking
                        self._current_task_id = None
                    
                    # Validate result
                    if result is None:
                        raise Exception(f"Task {task_type} returned None result")
                    
                    if not isinstance(result, dict):
                        logger.warning(f"⚠️ Task {task_id} returned non-dict result: {type(result)}")
                        result = {"result": result, "warning": "Non-dict result wrapped"}
                    
                    # Calculate execution time
                    execution_time = time.time() - start_time
                    
                    # Store successful result
                    result['execution_time_seconds'] = execution_time
                    result['worker_id'] = self.worker_id
                    result['queue_type'] = queue_type
                    result['priority'] = priority
                    result['completed_at'] = datetime.utcnow().isoformat()
                    
                    self._set_task_result(task_id, "completed", result)
                    logger.info(f"✅ Completed {task_type} task {task_id} from {queue_type} queue in {execution_time:.2f}s")
                    
                except SecurityError as e:
                    # Clear current task tracking
                    self._current_task_id = None
                    # Security validation error
                    error_result = {
                        "error": f"Security validation failed: {str(e)}",
                        "queue_type": queue_type,
                        "priority": priority,
                        "completed_at": datetime.utcnow().isoformat()
                    }
                    self._set_task_result(task_id, "error", error_result)
                    logger.error(f"🚨 Security error in task {task_id}: {e}")
                    
                except Exception as e:
                    # Clear current task tracking
                    self._current_task_id = None
                    # General error - log and set error status
                    error_result = {
                        "error": str(e),
                        "traceback": traceback.format_exc(),
                        "queue_type": queue_type,
                        "priority": priority,
                        "completed_at": datetime.utcnow().isoformat()
                    }
                    self._set_task_result(task_id, "error", error_result)
                    logger.error(f"❌ Task execution error in {task_id}: {e}")
                    logger.error(f"📄 Full traceback: {traceback.format_exc()}")
                    
            except KeyboardInterrupt:
                logger.info("🛑 Received interrupt signal, shutting down worker...")
                break
                
            except Exception as e:
                logger.error(f"💥 Unexpected error in main loop: {e}")
                logger.error(f"📄 Full traceback: {traceback.format_exc()}")
                time.sleep(5)  # Brief pause before continuing
        
        # Cleanup
        logger.info(f"🧹 Cleaning up worker {self.worker_id}...")
        logger.info(f"📊 Final stats - Total: {tasks_processed}, Priority: {priority_tasks_processed}, Normal: {normal_tasks_processed}")
        
        # Stop heartbeat thread
        self._stop_heartbeat_thread()
        
        self.redis_client.close()
        if self.db_conn:
            self.db_conn.close()
        logger.info("🏁 Worker shutdown complete")
    
    async def _execute_backtest(self, task_id: str = None, symbols: List[str] = None, 
                               start_date: str = None, end_date: str = None, 
                               securities: List[str] = None, strategy_id: str = None, **kwargs) -> Dict[str, Any]:
        """Execute backtest task using new accessor strategy engine"""
        if not strategy_id:
            raise ValueError("strategy_id is required")
            
        if task_id:
            self._publish_progress(task_id, "initialization", "Fetching strategy code from database...")
        
        strategy_code = self._fetch_strategy_code(strategy_id)
        logger.debug(f"Fetched strategy code from database for strategy_id: {strategy_id}")
        
        # Handle symbols and securities filtering
        symbols_input = symbols or []
        securities_filter = securities or []
        
        # Determine target symbols (strategies will fetch their own data via accessors)
        if securities_filter:
            target_symbols = securities_filter
            logger.debug(f"Using securities filter as target symbols: {len(target_symbols)} symbols")
        elif symbols_input:
            target_symbols = symbols_input
            logger.debug(f"Using provided symbols: {len(target_symbols)} symbols")
        else:
            target_symbols = []  # Let strategy determine its own symbols
            logger.debug("No symbols specified - strategy will determine requirements")
        
        if task_id:
            self._publish_progress(task_id, "symbols", f"Prepared {len(target_symbols)} symbols for analysis", 
                                 {"symbol_count": len(target_symbols)})
        
        logger.info(f"Starting backtest for {len(target_symbols)} symbols (strategy_id: {strategy_id})")
        
        if task_id:
            self._publish_progress(task_id, "preparation", "Preparing date ranges and execution parameters...")
        
        # Parse and validate dates
        parsed_start_date = None
        parsed_end_date = None
        
        if start_date:
            try:
                # Parse as YYYY-MM-DD format only
                parsed_start_date = datetime.strptime(start_date, '%Y-%m-%d')
            except (ValueError, TypeError) as e:
                logger.warning(f"Invalid start_date format '{start_date}': {e}. Expected YYYY-MM-DD format. Using default.")
                parsed_start_date = None
        
        if end_date:
            try:
                # Parse as YYYY-MM-DD format only
                parsed_end_date = datetime.strptime(end_date, '%Y-%m-%d')
            except (ValueError, TypeError) as e:
                logger.warning(f"Invalid end_date format '{end_date}': {e}. Expected YYYY-MM-DD format. Using default.")
                parsed_end_date = None
        
        # Set defaults if parsing failed or dates not provided
        if not parsed_start_date:
            parsed_start_date = datetime.now() - timedelta(days=365)  # Default 1 year
            logger.info(f"Using default start_date: {parsed_start_date.date()}")
        
        if not parsed_end_date:
            parsed_end_date = datetime.now()
            logger.info(f"Using default end_date: {parsed_end_date.date()}")
        
        if parsed_start_date > parsed_end_date:
            raise ValueError(f"start_date ({parsed_start_date.date()}) must be before end_date ({parsed_end_date.date()})")
        
        # Log the final date range
        logger.info(f"Backtest date range: {parsed_start_date.date()} to {parsed_end_date.date()}")
        
        if task_id:
            self._publish_progress(task_id, "execution", f"Executing backtest: {parsed_start_date.date()} to {parsed_end_date.date()}", 
                                 {"start_date": parsed_start_date.isoformat(), "end_date": parsed_end_date.isoformat(), 
                                  "symbol_count": len(target_symbols)})
        
        # Execute using accessor strategy engine
        result = await self.strategy_engine.execute_backtest(
            strategy_id=strategy_id,
            strategy_code=strategy_code,
            symbols=target_symbols,
            start_date=parsed_start_date,
            end_date=parsed_end_date,
            **kwargs
        )
        
        logger.info(f"Backtest completed: {len(result.get('instances', []))} instances found")
        
        if task_id:
            instances_count = len(result.get('instances', []))
            self._publish_progress(task_id, "completed", f"Backtest finished: {instances_count} instances found", 
                                 {"instances_count": instances_count, "success": result.get('success', True)})
        
        return result
    
    async def _execute_screening(self, task_id: str = None, universe: List[str] = None, 
                                limit: int = 100, strategy_ids: List[str] = None, **kwargs) -> Dict[str, Any]:
        """Execute screening task using new accessor strategy engine"""
        if not strategy_ids:
            raise ValueError("strategy_ids is required")
            
        strategy_codes = self._fetch_multiple_strategy_codes(strategy_ids)
        logger.info(f"Fetched {len(strategy_codes)} strategy codes from database")
        
        # For now, use the first strategy code for screening
        # TODO: Implement multi-strategy screening in the future
        if strategy_codes:
            strategy_code = list(strategy_codes.values())[0]
        else:
            raise ValueError("No valid strategy codes found for provided strategy_ids")
        
        
        # Use provided universe or let strategy determine requirements
        target_universe = universe or []
        logger.info(f"Starting screening for {len(target_universe)} symbols, limit {limit} (strategy_ids: {strategy_ids})")
        
        # Execute using accessor strategy engine
        result = await self.strategy_engine.execute_screening(
            strategy_code=strategy_code,
            universe=target_universe,
            limit=limit,
            **kwargs
        )
        
        logger.info(f"Screening completed: {len(result.get('ranked_results', []))} results found")
        return result

    async def _execute_alert(self, task_id: str = None, symbols: List[str] = None, 
                        strategy_id: str = None, **kwargs) -> Dict[str, Any]:
        """Execute alert task using new accessor strategy engine"""
        if not strategy_id:
            raise ValueError("strategy_id is required")
            
        strategy_code = self._fetch_strategy_code(strategy_id)
        logger.info(f"Fetched strategy code from database for strategy_id: {strategy_id}")
        
        
        # Use provided symbols or empty list (strategies will determine their own requirements)
        target_symbols = symbols or []
        logger.info(f"Starting alert for {len(target_symbols)} symbols (strategy_id: {strategy_id})")
        
        # Execute using accessor strategy engine
        result = await self.strategy_engine.execute_alert(
            strategy_code=strategy_code,
            symbols=target_symbols,
            **kwargs
        )
        
        logger.info(f"Alert completed: {result.get('success', False)}")
        return result
    
    async def _execute_create_strategy(self, task_id: str = None, user_id: int = None, 
                                     prompt: str = None, strategy_id: int = -1, **kwargs) -> Dict[str, Any]:
        """Execute strategy creation task with detailed logging and comprehensive error handling"""
        logger.info(f"🧠 STRATEGY CREATION START - Task: {task_id}")
        logger.info(f"   👤 User ID: {user_id}")
        logger.info(f"   📝 Prompt: {prompt}")
        logger.info(f"   🆔 Strategy ID: {strategy_id} ({'Edit' if strategy_id != -1 else 'New'})")
        
        try:
            # Validate input parameters
            if not user_id:
                raise ValueError("user_id is required for strategy creation")
            if not prompt or not prompt.strip():
                raise ValueError("prompt is required for strategy creation")
            
            
            # Publish progress update
            if task_id:
                self._publish_progress(task_id, "initializing", "Starting strategy creation process...")
            
            # Call the strategy generator with comprehensive error handling
            logger.info(f"🚀 Calling StrategyGenerator.create_strategy_from_prompt...")
            
            # Add timeout to prevent hanging
            try:
                result = await asyncio.wait_for(
                    self.strategy_generator.create_strategy_from_prompt(
                        user_id=user_id,
                        prompt=prompt,
                        strategy_id=strategy_id
                    ),
                    timeout=300.0  # 5 minute timeout
                )
            except asyncio.TimeoutError:
                logger.error(f"⏰ Strategy creation timed out after 300 seconds for task {task_id}")
                raise Exception("Strategy creation timed out after 5 minutes")
            
            logger.info(f"📥 Strategy generator returned result type: {type(result)}")
            logger.debug(f"📊 Result keys: {result.keys() if isinstance(result, dict) else 'N/A'}")
            
            if task_id:
                if result.get("success"):
                    strategy_data = result.get("strategy", {})
                    logger.info(f"✅ Strategy creation SUCCESS for task {task_id}")
                    logger.info(f"   📊 Strategy Name: {strategy_data.get('name', 'Unknown')}")
                    logger.info(f"   🆔 Strategy ID: {strategy_data.get('strategyId', 'Unknown')}")
                    logger.info(f"   ✅ Validation Passed: {result.get('validation_passed', False)}")
                    
                    self._publish_progress(task_id, "completed", 
                                         f"Strategy created successfully: {strategy_data.get('name', 'Unknown')}", 
                                         {"strategy_id": strategy_data.get("strategyId")})
                else:
                    error_msg = result.get('error', 'Unknown error')
                    logger.error(f"❌ Strategy creation FAILED for task {task_id}")
                    logger.error(f"   🚨 Error: {error_msg}")
                    
                    self._publish_progress(task_id, "error", 
                                         f"Strategy creation failed: {error_msg}")
            
            logger.info(f"🏁 Strategy creation completed for task {task_id}: Success={result.get('success', False)}")
            return result
            
        except asyncio.TimeoutError as e:
            logger.error(f"⏰ TIMEOUT in strategy creation task {task_id}: {e}")
            logger.error(f"📄 Full traceback: {traceback.format_exc()}")
            
            error_result = {
                "success": False,
                "error": "Strategy creation timed out after 5 minutes",
                "error_type": "timeout",
                "traceback": traceback.format_exc()
            }
            
            if task_id:
                self._publish_progress(task_id, "error", "Strategy creation timed out")
            
            return error_result
            
        except ValueError as e:
            logger.error(f"🚨 VALIDATION ERROR in strategy creation task {task_id}: {e}")
            logger.error(f"📄 Full traceback: {traceback.format_exc()}")
            
            error_result = {
                "success": False,
                "error": f"Input validation failed: {str(e)}",
                "error_type": "validation",
                "traceback": traceback.format_exc()
            }
            
            if task_id:
                self._publish_progress(task_id, "error", f"Input validation failed: {str(e)}")
            
            return error_result
            
        except MemoryError as e:
            logger.error(f"💾 MEMORY ERROR in strategy creation task {task_id}: {e}")
            logger.error(f"📄 Full traceback: {traceback.format_exc()}")
            
            error_result = {
                "success": False,
                "error": "Strategy creation failed due to memory constraints",
                "error_type": "memory",
                "traceback": traceback.format_exc()
            }
            
            if task_id:
                self._publish_progress(task_id, "error", "Memory error during strategy creation")
            
            return error_result
            
        except Exception as e:
            logger.error(f"💥 CRITICAL ERROR in strategy creation task {task_id}: {e}")
            logger.error(f"📄 Full traceback: {traceback.format_exc()}")
            
            # Try to get more detailed error information
            error_type = type(e).__name__
            error_msg = str(e)
            
            logger.error(f"🔍 Error details:")
            logger.error(f"   Type: {error_type}")
            logger.error(f"   Message: {error_msg}")
            logger.error(f"   Args: {getattr(e, 'args', 'N/A')}")
            
            error_result = {
                "success": False,
                "error": error_msg,
                "error_type": error_type,
                "traceback": traceback.format_exc()
            }
            
            if task_id:
                self._publish_progress(task_id, "error", f"Critical error: {error_msg}")
            
            return error_result
    
    def _publish_progress(self, task_id: str, stage: str, message: str, data: Dict[str, Any] = None):
        """Publish progress updates for long-running tasks"""
        try:
            progress_update = {
                "task_id": task_id,
                "status": "progress", 
                "stage": stage,
                "message": message,
                "data": data or {},
                "updated_at": datetime.utcnow().isoformat()
            }
            
            # Publish to Redis
            channel = "worker_task_updates"
            message_json = json.dumps(progress_update)
            
            logger.debug(f"📡 Publishing progress update for {task_id}: {stage} - {message}")
            logger.debug(f"   📤 Channel: {channel}")
            logger.debug(f"   📄 Message: {message_json}")
            
            result = self.redis_client.publish(channel, message_json)
            logger.debug(f"   👥 Subscribers notified: {result}")
            
            if result == 0:
                logger.debug(f"⚠️ No subscribers listening to channel '{channel}' for task {task_id}")
            else:
                logger.debug(f"✅ Progress update published successfully to {result} subscribers")
            
        except Exception as e:
            logger.error(f"❌ Failed to publish progress for {task_id}: {e}")
            logger.error(f"📄 Full traceback: {traceback.format_exc()}")
    
    def _start_heartbeat_thread(self):
        """Start the asynchronous heartbeat thread"""
        self._heartbeat_stop_event = threading.Event()
        self._heartbeat_thread = threading.Thread(target=self._heartbeat_loop, daemon=True)
        self._heartbeat_thread.start()
    
    def _stop_heartbeat_thread(self):
        """Stop the heartbeat thread"""
        if hasattr(self, '_heartbeat_stop_event'):
            self._heartbeat_stop_event.set()
        if hasattr(self, '_heartbeat_thread'):
            self._heartbeat_thread.join(timeout=5)
            logger.info("💓 Stopped heartbeat thread")
        
        # Clean up heartbeat key from Redis
        self._cleanup_heartbeat()
    
    def _cleanup_heartbeat(self):
        """Clean up worker heartbeat from Redis on shutdown"""
        try:
            heartbeat_key = f"worker_heartbeat:{self.worker_id}"
            self.redis_client.delete(heartbeat_key)
            logger.info(f"🧹 Cleaned up heartbeat key for worker {self.worker_id}")
        except Exception as e:
            logger.error(f"❌ Failed to cleanup heartbeat: {e}")
    
    def _heartbeat_loop(self):
        """Asynchronous heartbeat loop - runs in separate thread"""
        heartbeat_interval = 5  # Send heartbeat every 5 seconds for near-instant detection
        
        while not self._heartbeat_stop_event.is_set():
            try:
                self._publish_heartbeat()
            except Exception as e:
                logger.error(f"❌ Heartbeat thread error: {e}")
                # Don't stop the thread on errors, just continue
            
            # Wait for next heartbeat or stop signal
            self._heartbeat_stop_event.wait(heartbeat_interval)

    def _publish_heartbeat(self):
        """Publish worker heartbeat for monitoring"""
        try:
            # Get current active task if any
            active_task = getattr(self, '_current_task_id', None)
            
            heartbeat = {
                "worker_id": self.worker_id,
                "status": "alive",
                "timestamp": datetime.utcnow().isoformat() + "Z",  # Add timezone for RFC3339 compatibility
                "uptime_seconds": time.time() - getattr(self, '_start_time', time.time()),
                "active_task": active_task,
                "queue_stats": self.get_queue_stats(),
                "heartbeat_interval": 5,   # Signal the 5-second interval to monitor
                "monitor_timeout": 10      # Expected timeout threshold (2 missed heartbeats)
            }
            
            # Store heartbeat in Redis with short expiration for monitoring
            heartbeat_key = f"worker_heartbeat:{self.worker_id}"
            heartbeat_json = json.dumps(heartbeat)
            
            # Store with 30 second expiration (6x the heartbeat interval for safety)
            # This ensures stale heartbeats are cleaned up quickly if worker dies
            self.redis_client.setex(heartbeat_key, 30, heartbeat_json)
            
            # Also publish to channel for real-time monitoring
            channel = "worker_heartbeat"
            self.redis_client.publish(channel, heartbeat_json)
            
            logger.debug(f"💓 Published heartbeat for worker {self.worker_id}")
            
        except Exception as e:
            logger.error(f"❌ Failed to publish heartbeat: {e}")
            # Don't log full traceback for heartbeat failures to avoid spam

    def _set_task_result(self, task_id: str, status: str, data: Dict[str, Any]):
        """Set task result in Redis and publish update"""
        try:
            result = {
                "task_id": task_id,
                "status": status,
                "data": data,
                "updated_at": datetime.utcnow().isoformat() + "Z",
                "worker_id": self.worker_id  # Track which worker handled this task
            }
            
            # Store result with 24 hour expiration
            result_key = f"task_result:{task_id}"
            result_json = json.dumps(result)
            
            logger.debug(f"💾 Setting task result for {task_id}: {status}")
            logger.debug(f"   🔑 Key: {result_key}")
            logger.debug(f"   📄 Data: {result_json[:200]}...")
            
            self.redis_client.setex(result_key, 86400, result_json)
            logger.debug(f"✅ Task result stored successfully")
            
            # Track task assignment for failure recovery
            if status == "running":
                # Store which worker is handling this task
                assignment_key = f"task_assignment:{task_id}"
                assignment_data = {
                    "worker_id": self.worker_id,
                    "task_id": task_id,
                    "started_at": datetime.utcnow().isoformat() + "Z",
                    "status": "running"
                }
                self.redis_client.setex(assignment_key, 7200, json.dumps(assignment_data))  # 2 hour expiration
                logger.debug(f"📋 Task assignment tracked for {task_id} -> worker {self.worker_id}")
            elif status in ["completed", "error"]:
                # Remove task assignment when task is finished
                assignment_key = f"task_assignment:{task_id}"
                self.redis_client.delete(assignment_key)
                logger.debug(f"🗑️ Task assignment cleared for {task_id}")
            
            # Publish task update for real-time notifications
            update_message = {
                "task_id": task_id,
                "status": status,
                "result": data,
                "updated_at": datetime.utcnow().isoformat() + "Z",
                "worker_id": self.worker_id
            }
            
            if status == "error":
                update_message["error_message"] = data.get("error", "Unknown error")
            
            # Publish to Redis
            channel = "worker_task_updates"
            update_json = json.dumps(update_message)
            
            logger.debug(f"📡 Publishing task update for {task_id}: {status}")
            logger.debug(f"   📤 Channel: {channel}")
            logger.debug(f"   📄 Update: {update_json[:200]}...")
            
            subscribers = self.redis_client.publish(channel, update_json)
            logger.debug(f"   👥 Subscribers notified: {subscribers}")
            
            if subscribers == 0:
                logger.debug(f"⚠️ No subscribers listening to channel '{channel}' for task {task_id}")
            else:
                logger.debug(f"✅ Task update published successfully to {subscribers} subscribers")
            
        except Exception as e:
            logger.error(f"❌ Failed to set task result for {task_id}: {e}")
            logger.error(f"📄 Full traceback: {traceback.format_exc()}")
    
    def _check_connection(self):
        """Lightweight connection check - only when necessary"""
        # Quick Redis ping - this is very fast
        try:
            self.redis_client.ping()
        except Exception as e:
            logger.error(f"Redis connection lost, reconnecting: {e}")
            self.redis_client = self._init_redis()
        
        # Lightweight DB connection check to prevent stale connections
        try:
            self._ensure_db_connection()
        except Exception as e:
            logger.error(f"Database connection check failed: {e}")
            # Don't raise here to avoid interrupting the worker loop

    def get_queue_stats(self) -> Dict[str, Any]:
        """Get current queue statistics"""
        try:
            priority_length = self.redis_client.llen('strategy_queue_priority')
            normal_length = self.redis_client.llen('strategy_queue')
            
            stats = {
                "priority_queue_length": priority_length,
                "normal_queue_length": normal_length,
                "total_pending_tasks": priority_length + normal_length,
                "worker_id": self.worker_id,
                "timestamp": datetime.utcnow().isoformat() + "Z"
            }
            
            logger.debug(f"Queue stats: Priority={priority_length}, Normal={normal_length}, Total={priority_length + normal_length}")
            return stats
            
        except Exception as e:
            logger.error(f"Failed to get queue statistics: {e}")
            return {
                "error": str(e),
                "worker_id": self.worker_id,
                "timestamp": datetime.utcnow().isoformat() + "Z"
            }

    def log_queue_status(self):
        """Log current queue status for monitoring"""
        stats = self.get_queue_stats()
        if "error" not in stats:
            logger.debug(f"[QUEUE STATUS] Worker {self.worker_id}: "
                       f"Priority Queue: {stats['priority_queue_length']} tasks, "
                       f"Normal Queue: {stats['normal_queue_length']} tasks, "
                       f"Total: {stats['total_pending_tasks']} tasks")
        else:
            logger.error(f"[QUEUE STATUS] Failed to get queue status: {stats['error']}")

    def _cleanup_stale_heartbeats(self):
        """Clean up any stale heartbeats from previous instances"""
        try:
            heartbeat_key = f"worker_heartbeat:{self.worker_id}"
            self.redis_client.delete(heartbeat_key)
            logger.info(f"🧹 Cleaned up heartbeat key for worker {self.worker_id}")
        except Exception as e:
            logger.error(f"❌ Failed to cleanup stale heartbeats: {e}")


# Utility functions for adding tasks to queue
def add_backtest_task(redis_client: redis.Redis, task_id: str, strategy_id: str,
                     symbols: List[str] = None, start_date: str = None, end_date: str = None,
                     securities: List[str] = None) -> None:
    """Add a backtest task to the queue"""
    task_data = {
        "task_id": task_id,
        "task_type": "backtest",
        "args": {
            "strategy_id": strategy_id,
            "symbols": symbols,
            "start_date": start_date,
            "end_date": end_date,
            "securities": securities
        },
        "created_at": datetime.utcnow().isoformat() + "Z"
    }
    # Backtest tasks go to normal queue
    redis_client.lpush('strategy_queue', json.dumps(task_data))


def add_screening_task(redis_client: redis.Redis, task_id: str, strategy_ids: List[str],
                      universe: List[str] = None, limit: int = 100) -> None:
    """Add a screening task to the queue"""
    task_data = {
        "task_id": task_id,
        "task_type": "screening",
        "args": {
            "strategy_ids": strategy_ids,
            "universe": universe,
            "limit": limit
        },
        "created_at": datetime.utcnow().isoformat() + "Z"
    }
    # Screening tasks go to normal queue
    redis_client.lpush('strategy_queue', json.dumps(task_data))


def add_alert_task(redis_client: redis.Redis, task_id: str, strategy_id: str,
                  symbols: List[str] = None) -> None:
    """Add an alert task to the queue"""
    task_data = {
        "task_id": task_id,
        "task_type": "alert",
        "args": {
            "strategy_id": strategy_id,
            "symbols": symbols
        },
        "created_at": datetime.utcnow().isoformat() + "Z"
    }
    # Alert tasks go to normal queue
    redis_client.lpush('strategy_queue', json.dumps(task_data))


def add_create_strategy_task(redis_client: redis.Redis, task_id: str, user_id: int,
                           prompt: str, strategy_id: int = -1) -> None:
    """Add a strategy creation task to the PRIORITY queue"""
    task_data = {
        "task_id": task_id,
        "task_type": "create_strategy",
        "args": {
            "user_id": user_id,
            "prompt": prompt,
            "strategy_id": strategy_id
        },
        "created_at": datetime.utcnow().isoformat() + "Z",
        "priority": "high"  # Mark as high priority
    }
    # Strategy creation/editing tasks go to PRIORITY queue
    redis_client.lpush('strategy_queue_priority', json.dumps(task_data))
    logger.info(f"Added strategy creation task {task_id} to PRIORITY queue")


def add_task_with_priority(redis_client: redis.Redis, task_id: str, task_type: str, 
                          args: Dict[str, Any], priority: str = "normal") -> None:
    """Add a task to the appropriate queue based on priority level"""
    task_data = {
        "task_id": task_id,
        "task_type": task_type,
        "args": args,
        "created_at": datetime.utcnow().isoformat() + "Z",
        "priority": priority
    }
    
    if priority == "high" or task_type == "create_strategy":
        # High priority tasks (including all strategy creation/editing)
        redis_client.lpush('strategy_queue_priority', json.dumps(task_data))
        logger.info(f"Added {task_type} task {task_id} to PRIORITY queue")
    else:
        # Normal priority tasks
        redis_client.lpush('strategy_queue', json.dumps(task_data))
        logger.info(f"Added {task_type} task {task_id} to normal queue")


def get_task_result(redis_client: redis.Redis, task_id: str) -> Dict[str, Any]:
    """Get task result from Redis"""
    result_json = redis_client.get(f"task_result:{task_id}")
    if result_json:
        return json.loads(result_json)
    return None


# Utility functions for queue management
def get_queue_statistics(redis_client: redis.Redis) -> Dict[str, Any]:
    """Get comprehensive queue statistics"""
    try:
        stats = {
            "priority_queue_length": redis_client.llen('strategy_queue_priority'),
            "normal_queue_length": redis_client.llen('strategy_queue'),
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }
        stats["total_pending_tasks"] = stats["priority_queue_length"] + stats["normal_queue_length"]
        
        return stats
        
    except Exception as e:
        return {
            "error": str(e),
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }


def clear_queue(redis_client: redis.Redis, queue_name: str) -> int:
    """Clear a specific queue and return the number of tasks removed"""
    try:
        removed_count = redis_client.delete(queue_name)
        logger.info(f"Cleared {removed_count} tasks from queue: {queue_name}")
        return removed_count
    except Exception as e:
        logger.error(f"Failed to clear queue {queue_name}: {e}")
        return 0


if __name__ == "__main__":
    
    try:
        # Initialize and start worker
        
        worker = StrategyWorker()
        
        logger.info("🎯 Starting main processing loop...")
        
        worker.run()
        
    except KeyboardInterrupt:
        logger.info("🛑 Received keyboard interrupt - shutting down gracefully")
        
    except Exception as e:
        error_msg = f"💥 FATAL ERROR during worker startup: {e}"
        logger.error(error_msg)
        logger.error(f"📄 Full traceback: {traceback.format_exc()}")
        raise
        
    finally:
        logger.info("🏁 Worker process ending")
