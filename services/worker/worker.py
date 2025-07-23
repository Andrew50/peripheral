#!/usr/bin/env python3
"""
Strategy Worker
Executes trading strategies via Redis queue for backtesting and screening
"""

import asyncio
import datetime
import json
import logging
import os
import signal
import sys
import threading
import time
import traceback
from datetime import datetime, timedelta
from typing import Any, Dict, List

import psycopg2
import redis
from psycopg2.extras import RealDictCursor

# Add src directory to Python path before importing local modules
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

from src.data_accessors import DataAccessorProvider
from src.pythonAgentGenerator import PythonAgentGenerator
from src.strategy_engine import AccessorStrategyEngine
from src.strategy_generator import StrategyGenerator
from src.validator import SecurityValidator, SecurityError

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
		
		# Initialize attributes that will be set later
		self._start_time = None
		self._current_task_id = None
		self._heartbeat_stop_event = None
		self._heartbeat_thread = None
		
		# Set up signal handlers for graceful shutdown
		self._setup_signal_handlers()
		
		# Initialize Redis connection
		self.redis_client = self._init_redis()
		# Clear queues on startup
		self._clear_queues_on_startup()
		# Clean up any stale heartbeats from previous instances
		self._cleanup_stale_heartbeats()
		# Initialize Database connection  
		self.db_conn = self._init_database()
		
		# Import the new data accessor
		self.data_accessor = DataAccessorProvider()
		
		self.strategy_engine = AccessorStrategyEngine()
		self.security_validator = SecurityValidator()
		self.strategy_generator = StrategyGenerator()
		self.python_agent_generator = PythonAgentGenerator()
	
		
		# Log initial queue status
		self.log_queue_status()
	
	def _setup_signal_handlers(self):
		"""Set up signal handlers for graceful shutdown and crash detection"""
		def signal_handler(signum, frame):
			signal_name = signal.Signals(signum).name
			logger.error("🚨 Received signal %s (%s) - initiating graceful shutdown", signal_name, signum)
			self.shutdown_requested = True
			
			# Clean up heartbeat on shutdown
			try:
				self._cleanup_heartbeat()
			except (redis.RedisError, OSError, RuntimeError) as e:
				logger.error("❌ Error during signal cleanup: %s", e)
			
			# Log the current stack trace to help debug
			logger.error("📄 Signal received at:")
			for line in traceback.format_stack(frame):
				logger.error("   %s", line.strip())
		
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
			
			
		except Exception as e:
			logger.error("❌ Redis connection failed: %s", e)
			raise
		
		return client

	
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
			logger.error("Failed to connect to database: %s", e)
			raise
	
	def _ensure_db_connection(self):
		"""Ensure database connection is healthy, reconnect if needed"""
		try:
			# Test the connection with a simple query
			with self.db_conn.cursor() as cursor:
				cursor.execute("SELECT 1")
				cursor.fetchone()
		except (psycopg2.OperationalError, psycopg2.InterfaceError, AttributeError) as e:
			logger.warning("Database connection test failed, reconnecting: %s", e)
			try:
				if hasattr(self, 'db_conn') and self.db_conn:
					self.db_conn.close()
			except (psycopg2.Error, OSError):
				logger.debug("Error closing database connection (expected during reconnection)")
			self.db_conn = self._init_database()
		except (psycopg2.Error, OSError, RuntimeError) as e:
			logger.error("Unexpected error testing database connection: %s", e)
			# For other errors, don't reconnect to avoid infinite loops
	
	def _fetch_strategy_code(self, strategy_id: str) -> str:
		"""Fetch strategy code from database by strategy_id"""
		result = self._fetch_multiple_strategy_codes([strategy_id])
		
		if strategy_id not in result:
			raise ValueError(f"Strategy not found or has no Python code for strategy_id: {strategy_id}")
		
		return result[strategy_id]
	
	
	def _fetch_multiple_strategy_codes(self, strategy_ids: List[str]) -> Dict[str, str]:
		"""Fetch multiple strategy codes from database in a single query"""
		if not strategy_ids:
			return {}
		
		# Remove duplicates and None values
		unique_ids = list(set(filter(None, strategy_ids)))
		if not unique_ids:
			return {}
		
		# Convert string IDs to integers for database query
		try:
			unique_int_ids = [int(id_str) for id_str in unique_ids]
		except ValueError as e:
			logger.error("Failed to convert strategy_ids to integers: %s, error: %s", unique_ids, e)
			return {}
		
		max_retries = 3
		for attempt in range(max_retries):
			try:
				self._ensure_db_connection()
				
				with self.db_conn.cursor() as cursor:
					cursor.execute(
						"SELECT strategyId, pythonCode FROM strategies WHERE strategyId = ANY(%s) AND is_active = true",
						(unique_int_ids,)
					)
					results = cursor.fetchall()
					
					# Build result dictionary - convert integer keys back to strings
					strategy_codes = {}
					for row in results:
						if row['pythoncode']:  # Only include strategies with code
							strategy_codes[str(row['strategyid'])] = row['pythoncode']
					
					# Log missing strategies
					missing_strategies = set(unique_ids) - set(strategy_codes.keys())
					if missing_strategies:
						logger.warning("Strategies not found or missing Python code: %s", missing_strategies)
					
					return strategy_codes
					
			except (psycopg2.OperationalError, psycopg2.InterfaceError) as e:
				logger.warning("Database connection error on attempt %s/%s: %s", attempt + 1, max_retries, e)
				if attempt < max_retries - 1:
					try:
						self.db_conn.close()
					except (psycopg2.Error, OSError):
						logger.debug("Error closing database connection (expected during reconnection)")
					self.db_conn = self._init_database()
				else:
					logger.error("Failed to fetch strategy codes after %s attempts", max_retries)
					raise
			except (psycopg2.Error, ValueError, TypeError) as e:
				logger.error("Failed to fetch strategy codes for strategy_ids %s: %s", unique_ids, e)
				raise
		
		return {}
	
	def run(self):
		"""Main queue processing loop with priority queue support"""
		logger.info("🎯 Strategy worker %s starting queue processing...", self.worker_id)
		
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
				logger.debug("📦 Received %s task (Total: P:%s/N:%s)", queue_type, priority_tasks_processed, normal_tasks_processed)
				
				try:
					task_data = json.loads(task_message)
				except json.JSONDecodeError as e:
					logger.error("❌ Failed to parse task JSON: %s", e)
					continue
					
				task_id = task_data.get('task_id')
				task_type = task_data.get('task_type')
				args = task_data.get('args', {})
				priority = task_data.get('priority', 'normal')

				logger.info("🎯 Processing %s task %s", task_type, task_id)
				tasks_processed += 1
				
				# Validate task data
				if not task_id or not task_type:
					logger.error("❌ Invalid task data - missing task_id or task_type: %s", task_data)
					continue

				try:
					# Set task status to running
					logger.debug("▶️ Starting execution of %s task %s", task_type, task_id)
					
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
					logger.debug("🔧 Executing %s with args: %s", task_type, json.dumps(args, indent=2))
					
					result = None
					try:
						if task_type == 'backtest':
							result = asyncio.run(self._execute_backtest(task_id=task_id, **args))
						elif task_type == 'screening':
							result = asyncio.run(self._execute_screening(task_id=task_id, **args))
						elif task_type == 'alert':
							result = asyncio.run(self._execute_alert(task_id=task_id, **args))
						elif task_type == 'create_strategy':
							logger.info("🧠 Starting strategy creation for user %s with prompt: %s...", args.get('user_id'), args.get('prompt', '')[:100])
							result = asyncio.run(self._execute_create_strategy(task_id=task_id, **args))
						elif task_type == 'general_python_agent':
							result = asyncio.run(self._execute_general_python_agent(task_id=task_id, user_id=args.get('user_id'), prompt=args.get('prompt'), data=args.get('data'), conversationID=args.get('conversationID'), messageID=args.get('messageID')))
						else: # unknown task type
						  
							raise ValueError(f"Unknown task type: {task_type}.")
					except asyncio.TimeoutError as timeout_error:
						logger.error("⏰ Task %s timed out: %s", task_id, timeout_error)
						raise RuntimeError(f"Task execution timed out: {str(timeout_error)}") from timeout_error
					except MemoryError as memory_error:
						logger.error("💾 Task %s ran out of memory: %s", task_id, memory_error)
						raise RuntimeError(f"Task execution failed due to memory constraints: {str(memory_error)}") from memory_error
					except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError, OSError) as exec_error:
						logger.error("💥 Task %s execution failed: %s", task_id, exec_error)
						logger.error("📄 Execution traceback: %s", traceback.format_exc())
						raise exec_error
					finally:
						# Clear current task tracking
						self._current_task_id = None
				
					if not isinstance(result, dict):
						logger.warning("⚠️ Task %s returned non-dict result: %s", task_id, type(result))
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
					logger.info("✅ Completed %s task %s from %s queue in %.2fs", task_type, task_id, queue_type, execution_time)
					
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
					logger.error("🚨 Security error in task %s: %s", task_id, e)
					
				except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError, OSError) as e:
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
					logger.error("❌ Task execution error in %s: %s", task_id, e)
					logger.error("📄 Full traceback: %s", traceback.format_exc())
					
			except KeyboardInterrupt:
				logger.info("🛑 Received interrupt signal, shutting down worker...")
				break
				
			except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError, OSError, redis.RedisError) as e:
				logger.error("💥 Unexpected error in main loop: %s", e)
				logger.error("📄 Full traceback: %s", traceback.format_exc())
				time.sleep(5)  # Brief pause before continuing
		
		# Cleanup
		logger.info("🧹 Cleaning up worker %s...", self.worker_id)
		logger.info("📊 Final stats - Total: %s, Priority: %s, Normal: %s", tasks_processed, priority_tasks_processed, normal_tasks_processed)
		
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
		logger.debug("Fetched strategy code from database for strategy_id: %s", strategy_id)
		
		# Handle symbols and securities filtering
		symbols_input = symbols or []
		securities_filter = securities or []
		
		# Determine target symbols (strategies will fetch their own data via accessors)
		if securities_filter:
			target_symbols = securities_filter
			logger.debug("Using securities filter as target symbols: %s symbols", len(target_symbols))
		elif symbols_input:
			target_symbols = symbols_input
			logger.debug("Using provided symbols: %s symbols", len(target_symbols))
		else:
			target_symbols = []  # Let strategy determine its own symbols
			logger.debug("No symbols specified - strategy will determine requirements")
		
		if task_id:
			self._publish_progress(task_id, "symbols", f"Prepared {len(target_symbols)} symbols for analysis", 
								 {"symbol_count": len(target_symbols)})
		
		logger.info("Starting backtest for %s symbols (strategy_id: %s)", len(target_symbols), strategy_id)
		
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
				logger.warning("Invalid start_date format '%s': %s. Expected YYYY-MM-DD format. Using default.", start_date, e)
				parsed_start_date = None
		
		if end_date:
			try:
				# Parse as YYYY-MM-DD format only
				parsed_end_date = datetime.strptime(end_date, '%Y-%m-%d')
			except (ValueError, TypeError) as e:
				logger.warning("Invalid end_date format '%s': %s. Expected YYYY-MM-DD format. Using default.", end_date, e)
				parsed_end_date = None
		
		# Set defaults if parsing failed or dates not provided
		if not parsed_start_date:
			parsed_start_date = datetime.now() - timedelta(days=365)  # Default 1 year
			logger.info("Using default start_date: %s", parsed_start_date.date())
		
		if not parsed_end_date:
			parsed_end_date = datetime.now()
			logger.info("Using default end_date: %s", parsed_end_date.date())
		
		if parsed_start_date > parsed_end_date:
			raise ValueError(f"start_date ({parsed_start_date.date()}) must be before end_date ({parsed_end_date.date()})")
		
		# Log the final date range
		logger.info("Backtest date range: %s to %s", parsed_start_date.date(), parsed_end_date.date())
		
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
		
		logger.info("Backtest completed: %s instances found", len(result.get('instances', [])))
		
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
		logger.info("Fetched %s strategy codes from database", len(strategy_codes))
		
		# For now, use the first strategy code for screening
		# TODO: Implement multi-strategy screening in the future
		if strategy_codes:
			strategy_code = list(strategy_codes.values())[0]
		else:
			raise ValueError("No valid strategy codes found for provided strategy_ids")
		
		
		# Use provided universe or let strategy determine requirements
		target_universe = universe or []
		logger.info("Starting screening for %s symbols, limit %s (strategy_ids: %s)", len(target_universe), limit, strategy_ids)
		
		# Execute using accessor strategy engine
		result = await self.strategy_engine.execute_screening(
			strategy_code=strategy_code,
			universe=target_universe,
			limit=limit,
			**kwargs
		)
		
		logger.info("Screening completed: %s results found", len(result.get('ranked_results', [])))
		return result

	async def _execute_alert(self, task_id: str = None, symbols: List[str] = None, 
						strategy_id: str = None, **kwargs) -> Dict[str, Any]:
		"""Execute alert task using new accessor strategy engine"""
		if not strategy_id:
			raise ValueError("strategy_id is required")
			
		strategy_code = self._fetch_strategy_code(strategy_id)
		logger.info("Fetched strategy code from database for strategy_id: %s", strategy_id)
		
		
		# Use provided symbols or empty list (strategies will determine their own requirements)
		target_symbols = symbols or []
		logger.info("Starting alert for %s symbols (strategy_id: %s)", len(target_symbols), strategy_id)
		
		# Execute using accessor strategy engine
		result = await self.strategy_engine.execute_alert(
			strategy_code=strategy_code,
			symbols=target_symbols,
			**kwargs
		)
		
		logger.info("Alert completed: %s", result.get('success', False))
		return result
	
	async def _execute_create_strategy(self, task_id: str = None, user_id: int = None, 
									 prompt: str = None, strategy_id: int = -1, conversationID: str = None, messageID: str = None, **_kwargs) -> Dict[str, Any]:
		"""Execute strategy creation task with detailed logging and comprehensive error handling"""
		logger.info("🧠 STRATEGY CREATION START - Task: %s", task_id)
		logger.info("   👤 User ID: %s", user_id)
		logger.info("   📝 Prompt: %s", prompt)
		logger.info("   🆔 Strategy ID: %s (%s)", strategy_id, 'Edit' if strategy_id != -1 else 'New')
		
		try:
			# Validate input parameters
			if user_id is None:
				raise ValueError("user_id is required for strategy creation")
			if not prompt or not prompt.strip():
				raise ValueError("prompt is required for strategy creation")
			
			
			# Publish progress update
			if task_id:
				self._publish_progress(task_id, "initializing", "Starting strategy creation process...")
			
			# Call the strategy generator with comprehensive error handling
			logger.info("🚀 Calling StrategyGenerator.create_strategy_from_prompt...")
			
			# Add timeout to prevent hanging
			try:
				result = await asyncio.wait_for(
					self.strategy_generator.create_strategy_from_prompt(
						user_id=user_id,
						prompt=prompt,
						strategy_id=strategy_id,
						conversationID=conversationID,
						messageID=messageID
					),
					timeout=300.0  # 5 minute timeout
				)
			except asyncio.TimeoutError as e:
				logger.error("⏰ Strategy creation timed out after 300 seconds for task %s", task_id)
				raise RuntimeError("Strategy creation timed out after 5 minutes") from e
			
			logger.info("📥 Strategy generator returned result type: %s", type(result))
			logger.debug("📊 Result keys: %s", result.keys() if isinstance(result, dict) else 'N/A')
			
			if task_id:
				if result.get("success"):
					strategy_data = result.get("strategy", {})
					logger.info("✅ Strategy creation SUCCESS for task %s", task_id)
					logger.info("   📊 Strategy Name: %s", strategy_data.get('name', 'Unknown'))
					logger.info("   🆔 Strategy ID: %s", strategy_data.get('strategyId', 'Unknown'))
					logger.info("   ✅ Validation Passed: %s", result.get('validation_passed', False))
					
					self._publish_progress(task_id, "completed", 
										 f"Strategy created successfully: {strategy_data.get('name', 'Unknown')}", 
										 {"strategy_id": strategy_data.get("strategyId")})
				else:
					strategy_error = result.get('error', 'Unknown error')
					logger.error("❌ Strategy creation FAILED for task %s", task_id)
					logger.error("   🚨 Error: %s", strategy_error)
					
					self._publish_progress(task_id, "error", 
										 f"Strategy creation failed: {strategy_error}")
			
			logger.info("🏁 Strategy creation completed for task %s: Success=%s", task_id, result.get('success', False))
			return result
			
		except asyncio.TimeoutError as e:
			logger.error("⏰ TIMEOUT in strategy creation task %s: %s", task_id, e)
			logger.error("📄 Full traceback: %s", traceback.format_exc())
			
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
			logger.error("🚨 VALIDATION ERROR in strategy creation task %s: %s", task_id, e)
			logger.error("📄 Full traceback: %s", traceback.format_exc())
			
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
			logger.error("💾 MEMORY ERROR in strategy creation task %s: %s", task_id, e)
			logger.error("📄 Full traceback: %s", traceback.format_exc())
			
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
			logger.error("💥 CRITICAL ERROR in strategy creation task %s: %s", task_id, e)
			logger.error("📄 Full traceback: %s", traceback.format_exc())
			
			# Try to get more detailed error information
			error_type = type(e).__name__
			error_msg = str(e)
			
			logger.error("🔍 Error details:")
			logger.error("   Type: %s", error_type)
			logger.error("   Message: %s", error_msg)
			logger.error("   Args: %s", getattr(e, 'args', 'N/A'))
			
			error_result = {
				"success": False,
				"error": error_msg,
				"error_type": error_type,
				"traceback": traceback.format_exc()
			}
			
			if task_id:
				self._publish_progress(task_id, "error", f"Critical error: {error_msg}")
			
			return error_result
	
	async def _execute_general_python_agent(self, task_id: str = None, user_id: int = None, 
										   prompt: str = None, data: str = None, conversationID: str = None, messageID: str = None, **_kwargs) -> Dict[str, Any]:
		# Initialize defaults to avoid scope issues
		result, prints, plots, response_images = [], "", [], []
		execution_id = None  # Initialize to avoid UnboundLocalError
		
		try:
			# Validate input parameters
			if user_id is None:
				raise ValueError("user_id is required for general Python agent")
			if not prompt or not prompt.strip():
				raise ValueError("prompt is required for general Python agent")
			
			# Publish progress update
			if task_id:
				self._publish_progress(task_id, "initializing", "Starting general Python agent execution...")         
			
			# Execute with timeout
			result, prints, plots, response_images, execution_id, error = await asyncio.wait_for(
				self.python_agent_generator.start_general_python_agent(
					user_id=user_id,
					prompt=prompt,
					data=data,
					conversationID=conversationID,
					messageID=messageID
				),
				timeout=240.0  # 4 minute timeout
			)
			
			# Check if there was an error
			if error:
				logger.error("❌ General Python agent execution FAILED for task %s: %s", task_id, error)
				if task_id:
					self._publish_progress(task_id, "error", f"Execution failed: {str(error)}")
				raise error
			
			# Success case
			logger.info("✅ General Python agent execution SUCCESS for task %s", task_id)
			if task_id:
				self._publish_progress(task_id, "completed", "Python agent execution completed successfully")
			
			return {
				"success": True,
				"result": result,
				"prints": prints,
				"plots": plots,
				"responseImages": response_images,
				"executionID": execution_id,
			}
			
		except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError, OSError) as e:
			logger.error("💥 General Python agent task %s failed: %s", task_id, e)
			
			if task_id:
				self._publish_progress(task_id, "error", f"Error: {str(e)}")
			return {
				"success": False,
				"error": str(e),
				"result": result,
				"prints": prints,
				"plots": plots,
				"responseImages": response_images,
				"executionID": execution_id,
			}
	
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
			
			logger.debug("📡 Publishing progress update for %s: %s - %s", task_id, stage, message)
			logger.debug("   📤 Channel: %s", channel)
			logger.debug("   📄 Message: %s", message_json)
			
			result = self.redis_client.publish(channel, message_json)
			logger.debug("   👥 Subscribers notified: %s", result)
			
			if result == 0:
				logger.debug("⚠️ No subscribers listening to channel '%s' for task %s", channel, task_id)
			else:
				logger.debug("✅ Progress update published successfully to %s subscribers", result)
			
		except (redis.RedisError, ValueError, TypeError) as e:
			logger.error("❌ Failed to publish progress for %s: %s", task_id, e)
			logger.error("📄 Full traceback: %s", traceback.format_exc())
	
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
			logger.info("🧹 Cleaned up heartbeat key for worker %s", self.worker_id)
		except (redis.RedisError, OSError) as e:
			logger.error("❌ Failed to cleanup heartbeat: %s", e)
	
	def _heartbeat_loop(self):
		"""Asynchronous heartbeat loop - runs in separate thread"""
		heartbeat_interval = 5  # Send heartbeat every 5 seconds for near-instant detection
		
		while not self._heartbeat_stop_event.is_set():
			try:
				self._publish_heartbeat()
			except (redis.RedisError, ValueError, TypeError, OSError) as e:
				logger.error("❌ Heartbeat thread error: %s", e)
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
			
			logger.debug("💓 Published heartbeat for worker %s", self.worker_id)
			
		except (redis.RedisError, ValueError, TypeError, OSError) as e:
			logger.error("❌ Failed to publish heartbeat: %s", e)
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
			
			logger.debug("💾 Setting task result for %s: %s", task_id, status)
			logger.debug("   🔑 Key: %s", result_key)
			logger.debug("   📄 Data: %s...", result_json[:200])
			
			self.redis_client.setex(result_key, 86400, result_json)
			logger.debug("✅ Task result stored successfully")
			
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
				logger.debug("📋 Task assignment tracked for %s -> worker %s", task_id, self.worker_id)
			elif status in ["completed", "error"]:
				# Remove task assignment when task is finished
				assignment_key = f"task_assignment:{task_id}"
				self.redis_client.delete(assignment_key)
				logger.debug("🗑️ Task assignment cleared for %s", task_id)
			
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
			
			logger.debug("📡 Publishing task update for %s: %s", task_id, status)
			logger.debug("   📤 Channel: %s", channel)
			logger.debug("   📄 Update: %s...", update_json[:200])
			
			subscribers = self.redis_client.publish(channel, update_json)
			logger.debug("   👥 Subscribers notified: %s", subscribers)
			
			if subscribers == 0:
				logger.debug("⚠️ No subscribers listening to channel '%s' for task %s", channel, task_id)
			else:
				logger.debug("✅ Task update published successfully to %s subscribers", subscribers)
			
		except (redis.RedisError, ValueError, TypeError, OSError) as e:
			logger.error("❌ Failed to set task result for %s: %s", task_id, e)
			logger.error("📄 Full traceback: %s", traceback.format_exc())
	
	def _check_connection(self):
		"""Lightweight connection check - only when necessary"""
		# Quick Redis ping - this is very fast
		try:
			self.redis_client.ping()
		except (redis.RedisError, OSError) as e:
			logger.error("Redis connection lost, reconnecting: %s", e)
			self.redis_client = self._init_redis()
		
		# Lightweight DB connection check to prevent stale connections
		try:
			self._ensure_db_connection()
		except (psycopg2.Error, OSError, RuntimeError) as e:
			logger.error("Database connection check failed: %s", e)
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
			
			logger.debug("Queue stats: Priority=%s, Normal=%s, Total=%s", priority_length, normal_length, priority_length + normal_length)
			return stats
			
		except (redis.RedisError, OSError) as e:
			logger.error("Failed to get queue statistics: %s", e)
			return {
				"error": str(e),
				"worker_id": self.worker_id,
				"timestamp": datetime.utcnow().isoformat() + "Z"
			}

	def log_queue_status(self):
		"""Log current queue status for monitoring"""
		stats = self.get_queue_stats()
		if "error" not in stats:
			logger.debug("[QUEUE STATUS] Worker %s: Priority Queue: %s tasks, Normal Queue: %s tasks, Total: %s tasks", 
						self.worker_id, stats['priority_queue_length'], stats['normal_queue_length'], stats['total_pending_tasks'])
		else:
			logger.error("[QUEUE STATUS] Failed to get queue status: %s", stats['error'])

	def _cleanup_stale_heartbeats(self):
		"""Clean up any stale heartbeats from previous instances"""
		try:
			heartbeat_key = f"worker_heartbeat:{self.worker_id}"
			self.redis_client.delete(heartbeat_key)
			logger.info("🧹 Cleaned up heartbeat key for worker %s", self.worker_id)
		except (redis.RedisError, OSError) as e:
			logger.error("❌ Failed to cleanup stale heartbeats: %s", e)

	def _clear_queues_on_startup(self):
		"""Clear worker queues and stuck tasks on startup"""
		try:
			# Clear main queues
			priority_cleared = clear_queue(self.redis_client, 'strategy_queue_priority')
			normal_cleared = clear_queue(self.redis_client, 'strategy_queue')
			
			# Clear stuck task results
			task_results_cleared = self._clear_redis_pattern('task_result:*')
			
			# Clear stuck task assignments
			task_assignments_cleared = self._clear_redis_pattern('task_assignment:*')
			
			total_queue_cleared = priority_cleared + normal_cleared
			total_stuck_cleared = task_results_cleared + task_assignments_cleared
			
			if total_queue_cleared > 0 or total_stuck_cleared > 0:
				logger.info("🧹 Startup cleanup: %s queue tasks (P:%s, N:%s), %s stuck tasks (Results:%s, Assignments:%s)", 
						   total_queue_cleared, priority_cleared, normal_cleared, total_stuck_cleared, task_results_cleared, task_assignments_cleared)
			else:
				logger.info("🧹 No tasks to clear on startup")
				
		except (redis.RedisError, OSError) as e:
			logger.error("❌ Failed to clear queues on startup: %s", e)
	
	def _clear_redis_pattern(self, pattern: str) -> int:
		"""Clear all Redis keys matching a pattern"""
		try:
			cleared_count = 0
			cursor = 0
			
			while True:
				cursor, keys = self.redis_client.scan(cursor=cursor, match=pattern, count=100)
				
				if keys:
					# Delete keys in batches
					deleted = self.redis_client.delete(*keys)
					cleared_count += deleted
				
				if cursor == 0:
					break
			
			return cleared_count
			
		except (redis.RedisError, OSError) as e:
			logger.error("❌ Failed to clear Redis pattern %s: %s", pattern, e)
			return 0



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
		
	except (redis.RedisError, OSError) as e:
		return {
			"error": str(e),
			"timestamp": datetime.utcnow().isoformat() + "Z"
		}


def clear_queue(redis_client: redis.Redis, queue_name: str) -> int:
	"""Clear a specific queue and return the number of tasks removed"""
	try:
		removed_count = redis_client.delete(queue_name)
		logger.info("Cleared %s tasks from queue: %s", removed_count, queue_name)
		return removed_count
	except (redis.RedisError, OSError) as e:
		logger.error("Failed to clear queue %s: %s", queue_name, e)
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
		error_message = f"💥 FATAL ERROR during worker startup: {e}"
		logger.error(error_message)
		logger.error("📄 Full traceback: %s", traceback.format_exc())
		raise
		
	finally:
		logger.info("🏁 Worker process ending")
