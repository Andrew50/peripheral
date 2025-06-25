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

import sys
import os
import psycopg2
from psycopg2.extras import RealDictCursor
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))

from dataframe_strategy_engine import NumpyStrategyEngine
from validator import SecurityValidator, SecurityError
from strategy_generator import StrategyGenerator
from concurrent.futures import ThreadPoolExecutor
import threading
from src.strategy_data_analyzer import StrategyDataAnalyzer

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
        logger.info(f"🚀 Initializing Strategy Worker {self.worker_id}")
        
        # Initialize Redis connection
        logger.info("📡 Connecting to Redis...")
        self.redis_client = self._init_redis()
        logger.info("✅ Redis connection established")
        
        # Initialize Database connection  
        logger.info("🗄️ Connecting to Database...")
        self.db_conn = self._init_database()
        logger.info("✅ Database connection established")
        
        # Import the new data analyzer
        logger.info("📊 Initializing components...")
        self.data_analyzer = StrategyDataAnalyzer()
        
        self.strategy_engine = NumpyStrategyEngine()
        self.security_validator = SecurityValidator()
        self.strategy_generator = StrategyGenerator()
        
        logger.info(f"✅ Strategy worker {self.worker_id} initialized successfully")
        
        # Log initial queue status
        self.log_queue_status()
        
    def _init_redis(self) -> redis.Redis:
        """Initialize Redis connection"""
        redis_host = os.environ.get("REDIS_HOST", "cache")
        redis_port = int(os.environ.get("REDIS_PORT", "6379"))
        redis_password = os.environ.get("REDIS_PASSWORD", "")
        
        logger.info(f"🔗 Connecting to Redis at {redis_host}:{redis_port}")
        
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
            logger.info(f"✅ Redis connection successful")
            
            # Test queue access
            self._test_queue_access(client)
            
        except Exception as e:
            logger.error(f"❌ Redis connection failed: {e}")
            raise
        
        return client

    def _test_queue_access(self, redis_client: redis.Redis):
        """Test access to both priority and normal queues"""
        try:
            logger.info("🧪 Testing queue access...")
            
            # Test priority queue
            priority_length = redis_client.llen('strategy_queue_priority')
            logger.info(f"   🔥 Priority queue length: {priority_length}")
            
            # Test normal queue
            normal_length = redis_client.llen('strategy_queue')
            logger.info(f"   📄 Normal queue length: {normal_length}")
            
            # Test publish capability
            test_channel = "worker_test_channel"
            test_message = {"test": "message", "timestamp": datetime.utcnow().isoformat()}
            subscribers = redis_client.publish(test_channel, json.dumps(test_message))
            logger.info(f"   📡 Publish test - subscribers: {subscribers}")
            
            logger.info("✅ Queue access test completed successfully")
            
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
            logger.info("Database connection established")
            return connection
        except Exception as e:
            logger.error(f"Failed to connect to database: {e}")
            raise
    
    def _fetch_strategy_code(self, strategy_id: str) -> str:
        """Fetch strategy code from database by strategy_id"""
        try:
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
                
        except Exception as e:
            logger.error(f"Failed to fetch strategy code for strategy_id {strategy_id}: {e}")
            raise
    
    def _extract_required_symbols(self, strategy_code: str) -> List[str]:
        """Extract required symbols from strategy code using AST parsing"""
        import ast
        import re
        
        required_symbols = []
        
        try:
            # Parse the code into an AST
            tree = ast.parse(strategy_code)
            
            # Walk through all nodes in the AST
            for node in ast.walk(tree):
                # Look for string literals that could be symbols
                if isinstance(node, ast.Constant) and isinstance(node.value, str):
                    value = node.value.upper()
                    if self._is_valid_symbol(value):
                        required_symbols.append(value)
                
                # Look for list/tuple assignments with symbol-like strings
                elif isinstance(node, ast.Assign):
                    for target in node.targets:
                        if isinstance(target, ast.Name) and target.id.lower() in ['symbols', 'tickers', 'stocks', 'securities']:
                            if isinstance(node.value, (ast.List, ast.Tuple)):
                                for elt in node.value.elts:
                                    if isinstance(elt, ast.Constant) and isinstance(elt.value, str):
                                        symbol = elt.value.upper()
                                        if self._is_valid_symbol(symbol):
                                            required_symbols.append(symbol)
                
                # Look for dictionary keys that might be symbols
                elif isinstance(node, ast.Dict):
                    for key in node.keys:
                        if isinstance(key, ast.Constant) and isinstance(key.value, str):
                            symbol = key.value.upper()
                            if self._is_valid_symbol(symbol):
                                required_symbols.append(symbol)
                
                # Look for function calls with symbol arguments
                elif isinstance(node, ast.Call):
                    for arg in node.args:
                        if isinstance(arg, ast.Constant) and isinstance(arg.value, str):
                            symbol = arg.value.upper()
                            if self._is_valid_symbol(symbol):
                                required_symbols.append(symbol)
        
        except (SyntaxError, ValueError) as e:
            logger.warning(f"Failed to parse strategy code with AST, falling back to regex: {e}")
            # Fallback to regex-based extraction
            patterns = [
                r'["\']([A-Z]{1,5})["\']',  # String literals
                r'(?:symbols?|tickers?|stocks?)\s*=\s*\[(.*?)\]',  # List assignments
            ]
            
            for pattern in patterns:
                matches = re.findall(pattern, strategy_code, re.IGNORECASE | re.DOTALL)
                for match in matches:
                    if isinstance(match, str):
                        if self._is_valid_symbol(match.upper()):
                            required_symbols.append(match.upper())
                        else:
                            # Parse list content
                            symbols_in_list = re.findall(r'["\']([A-Z]{1,5})["\']', match)
                            required_symbols.extend([s.upper() for s in symbols_in_list if self._is_valid_symbol(s.upper())])
        
        # Remove duplicates
        return list(set(required_symbols))
    
    def _is_valid_symbol(self, symbol: str) -> bool:
        """Check if a string looks like a valid stock symbol"""
        if not symbol or not isinstance(symbol, str):
            return False
        
        symbol = symbol.upper()
        
        # Basic validation: 1-5 characters, all letters
        if not (1 <= len(symbol) <= 5 and symbol.isalpha()):
            return False
        
        # Exclude common false positives
        excluded = {
            'TRUE', 'FALSE', 'NULL', 'NONE', 'AND', 'OR', 'NOT', 'IF', 'ELSE', 'FOR', 'IN', 'DEF', 
            'RETURN', 'WHILE', 'BREAK', 'PASS', 'WITH', 'AS', 'TRY', 'CATCH', 'FINAL', 'CLASS',
            'IMPORT', 'FROM', 'PRINT', 'RAISE', 'YIELD', 'ASYNC', 'AWAIT', 'LAMBDA', 'GLOBAL',
            'LOCAL', 'SELF', 'SUPER', 'THIS', 'THAT', 'THEM', 'THEY', 'WHAT', 'WHEN', 'WHERE',
            'WHO', 'WHY', 'HOW', 'YES', 'NO', 'OPEN', 'HIGH', 'LOW', 'CLOSE', 'VOLUME', 'DATE',
            'TIME', 'DATA', 'DF', 'PD', 'NP', 'MATH', 'MIN', 'MAX', 'SUM', 'AVG', 'MEAN', 'STD'
        }
        
        return symbol not in excluded
    
    def _extract_timeframes(self, strategy_code: str) -> List[str]:
        """Extract timeframes from strategy code using AST parsing"""
        import ast
        import re
        
        timeframes = []
        
        # Common timeframe patterns
        timeframe_pattern = re.compile(r'^(\d+[smhd]|1min|5min|15min|30min|1h|4h|1d|1w|1m)$', re.IGNORECASE)
        
        try:
            # Parse the code into an AST
            tree = ast.parse(strategy_code)
            
            # Walk through all nodes in the AST
            for node in ast.walk(tree):
                # Look for string literals that could be timeframes
                if isinstance(node, ast.Constant) and isinstance(node.value, str):
                    value = node.value.lower()
                    if timeframe_pattern.match(value):
                        timeframes.append(value)
                
                # Look for assignments to timeframe-related variables
                elif isinstance(node, ast.Assign):
                    for target in node.targets:
                        if isinstance(target, ast.Name) and target.id.lower() in ['timeframe', 'tf', 'interval', 'period']:
                            if isinstance(node.value, ast.Constant) and isinstance(node.value.value, str):
                                value = node.value.value.lower()
                                if timeframe_pattern.match(value):
                                    timeframes.append(value)
        
        except (SyntaxError, ValueError) as e:
            logger.warning(f"Failed to parse strategy code for timeframes: {e}")
            # Fallback to regex
            matches = re.findall(r'["\'](\d+[smhd]|1min|5min|15min|30min|1h|4h|1d|1w|1m)["\']', strategy_code, re.IGNORECASE)
            timeframes.extend([m.lower() for m in matches])
        
        # Remove duplicates and return
        return list(set(timeframes))
    
    def _extract_dates(self, strategy_code: str) -> Dict[str, Any]:
        """Extract date information and lookback windows from strategy code"""
        import ast
        import re
        from datetime import datetime
        
        date_info = {
            'specific_dates': [],
            'lookback_days': [],
            'date_ranges': []
        }
        
        try:
            # Parse the code into an AST
            tree = ast.parse(strategy_code)
            
            # Walk through all nodes in the AST
            for node in ast.walk(tree):
                # Look for string literals that could be dates
                if isinstance(node, ast.Constant) and isinstance(node.value, str):
                    value = node.value
                    
                    # Check for ISO date format (YYYY-MM-DD)
                    if re.match(r'^\d{4}-\d{2}-\d{2}$', value):
                        try:
                            datetime.strptime(value, '%Y-%m-%d')
                            date_info['specific_dates'].append(value)
                        except ValueError:
                            pass
                    
                    # Check for other common date formats
                    date_patterns = [
                        r'^\d{4}/\d{2}/\d{2}$',  # YYYY/MM/DD
                        r'^\d{2}/\d{2}/\d{4}$',  # MM/DD/YYYY
                        r'^\d{2}-\d{2}-\d{4}$',  # MM-DD-YYYY
                    ]
                    
                    for pattern in date_patterns:
                        if re.match(pattern, value):
                            date_info['specific_dates'].append(value)
                            break
                
                # Look for numeric literals that could be lookback days
                elif isinstance(node, ast.Constant) and isinstance(node.value, int):
                    # Check if this number appears in a context suggesting days
                    if 1 <= node.value <= 3650:  # Reasonable range for days (1 day to 10 years)
                        date_info['lookback_days'].append(node.value)
                
                # Look for assignments to date-related variables
                elif isinstance(node, ast.Assign):
                    for target in node.targets:
                        if isinstance(target, ast.Name):
                            var_name = target.id.lower()
                            if any(keyword in var_name for keyword in ['days', 'lookback', 'window', 'period', 'duration']):
                                if isinstance(node.value, ast.Constant) and isinstance(node.value.value, int):
                                    if 1 <= node.value.value <= 3650:
                                        date_info['lookback_days'].append(node.value.value)
                            elif any(keyword in var_name for keyword in ['start', 'end', 'from', 'to', 'date']):
                                if isinstance(node.value, ast.Constant) and isinstance(node.value.value, str):
                                    date_str = node.value.value
                                    if re.match(r'^\d{4}-\d{2}-\d{2}$', date_str):
                                        date_info['specific_dates'].append(date_str)
        
        except (SyntaxError, ValueError) as e:
            logger.warning(f"Failed to parse strategy code for dates: {e}")
            # Fallback to regex
            
            # Extract date strings
            date_matches = re.findall(r'["\'](\d{4}-\d{2}-\d{2})["\']', strategy_code)
            date_info['specific_dates'].extend(date_matches)
            
            # Extract potential lookback days from variable assignments
            lookback_matches = re.findall(r'(?:days|lookback|window|period)\s*=\s*(\d+)', strategy_code, re.IGNORECASE)
            date_info['lookback_days'].extend([int(m) for m in lookback_matches if 1 <= int(m) <= 3650])
        
        # Remove duplicates and sort
        date_info['specific_dates'] = sorted(list(set(date_info['specific_dates'])))
        date_info['lookback_days'] = sorted(list(set(date_info['lookback_days'])))
        
        # Calculate suggested padding based on lookback windows
        if date_info['lookback_days']:
            max_lookback = max(date_info['lookback_days'])
            # Add 20% padding for technical indicators that need extra data
            suggested_padding = int(max_lookback * 1.2) + 30  # Minimum 30 days extra
            date_info['suggested_padding_days'] = suggested_padding
        else:
            date_info['suggested_padding_days'] = 30  # Default padding
        
        return date_info
    
    def analyze_strategy_code(self, strategy_code: str, mode: str = 'backtest') -> Dict[str, Any]:
        """
        Comprehensive strategy analysis using new AST analyzer
        Returns mode-specific data requirements and optimization strategies
        """
        try:
            # Use the new comprehensive data analyzer
            analysis_result = self.data_analyzer.analyze_data_requirements(strategy_code, mode)
            
            # Add legacy compatibility fields
            legacy_analysis = {
                'symbols': self._extract_required_symbols(strategy_code),
                'timeframes': self._extract_timeframes(strategy_code),
                'date_info': self._extract_dates(strategy_code)
            }
            
            # Merge with new analysis
            analysis_result['legacy_compatibility'] = legacy_analysis
            
            return analysis_result
            
        except Exception as e:
            logger.error(f"Strategy analysis failed: {e}")
            # Fallback to legacy analysis
            return {
                'data_requirements': {
                    'columns': ['open', 'high', 'low', 'close', 'volume'],
                    'fundamentals': ['pe_ratio', 'market_cap', 'sector'],
                    'periods': 30 if mode != 'screener' else 1,
                    'timeframe': '1d'
                },
                'strategy_complexity': 'unknown',
                'loading_strategy': 'full_dataframe_context',
                'legacy_compatibility': {
                    'symbols': self._extract_required_symbols(strategy_code),
                    'timeframes': self._extract_timeframes(strategy_code),
                    'date_info': self._extract_dates(strategy_code)
                },
                'analysis_metadata': {
                    'fallback_used': True,
                    'error': str(e),
                    'analyzed_at': datetime.utcnow().isoformat()
                }
            }
    
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
        logger.info("🔍 Queue Processing Order: 1) Priority Queue (strategy_queue_priority), 2) Normal Queue (strategy_queue)")
        
        # Track last queue status log time
        last_status_log = time.time()
        status_log_interval = 600  # Log queue status every 10 minutes when idle
        
        # Track statistics
        tasks_processed = 0
        priority_tasks_processed = 0
        normal_tasks_processed = 0
        
        while True:
            try:
                # Use Redis BRPOP with multiple queues for efficient, atomic queue checking
                # Priority queue is checked first automatically by Redis
                task = self.redis_client.brpop(['strategy_queue_priority', 'strategy_queue'], timeout=10)
                
                if not task:
                    # No task received within timeout
                    self._check_connection()
                    
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
                logger.info(f"📦 Received {queue_type} task (Total: P:{priority_tasks_processed}/N:{normal_tasks_processed})")
                
                try:
                    task_data = json.loads(task_message)
                except json.JSONDecodeError as e:
                    logger.error(f"❌ Failed to parse task JSON: {e}")
                    continue
                    
                task_id = task_data.get('task_id')
                task_type = task_data.get('task_type')
                args = task_data.get('args', {})
                priority = task_data.get('priority', 'normal')

                logger.info(f"🎯 Processing {task_type} task {task_id} from {queue_type} queue (Priority: {priority})")
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
                    logger.info(f"▶️ Starting execution of {task_type} task {task_id}")
                    self._set_task_result(task_id, "running", {
                        "worker_id": self.worker_id,
                        "queue_type": queue_type,
                        "priority": priority,
                        "started_at": datetime.utcnow().isoformat()
                    })
                    
                    start_time = time.time()
                    
                    # Execute the task
                    logger.info(f"🔧 Executing {task_type} with args: {json.dumps(args, indent=2)}")
                    if task_type == 'backtest':
                        result = self._execute_backtest(task_id=task_id, **args)
                    elif task_type == 'screening':
                        result = self._execute_screening(task_id=task_id, **args)
                    elif task_type == 'alert':
                        result = self._execute_alert(task_id=task_id, **args)
                    elif task_type == 'create_strategy':
                        logger.info(f"🧠 Starting strategy creation for user {args.get('user_id')} with prompt: {args.get('prompt', '')[:100]}...")
                        result = asyncio.run(self._execute_create_strategy(task_id=task_id, **args))
                    
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
        self.redis_client.close()
        if self.db_conn:
            self.db_conn.close()
        logger.info("🏁 Worker shutdown complete")
    
    def _execute_backtest(self, task_id: str = None, symbols: List[str] = None, 
                               start_date: str = None, end_date: str = None, 
                               securities: List[str] = None, strategy_id: str = None, **kwargs) -> Dict[str, Any]:
        """Execute backtest task"""
        # strategy_id is required - always fetch from database
        if not strategy_id:
            raise ValueError("strategy_id is required")
            
        if task_id:
            self._publish_progress(task_id, "initialization", "Fetching strategy code from database...")
        
        strategy_code = self._fetch_strategy_code(strategy_id)
        logger.info(f"Fetched strategy code from database for strategy_id: {strategy_id}")
        
        if task_id:
            self._publish_progress(task_id, "validation", "Validating strategy code security...")
        
        # Validate strategy code
        if not self.security_validator.validate_code(strategy_code):
            raise SecurityError("Strategy code contains prohibited operations")
        
        # Handle symbols and securities filtering
        symbols_input = symbols or []
        securities_filter = securities or []
        
        if task_id:
            self._publish_progress(task_id, "analysis", "Analyzing strategy requirements...")
        
        # Extract strategy requirements
        required_symbols = self._extract_required_symbols(strategy_code)
        timeframes = self._extract_timeframes(strategy_code)
        date_info = self._extract_dates(strategy_code)
        
        logger.info(f"Strategy analysis:")
        logger.info(f"  - Required symbols: {len(required_symbols)} {required_symbols[:5]}{'...' if len(required_symbols) > 5 else ''}")
        logger.info(f"  - Timeframes: {timeframes}")
        logger.info(f"  - Lookback days: {date_info['lookback_days']}")
        logger.info(f"  - Suggested padding: {date_info['suggested_padding_days']} days")
        
        # Union required symbols with requested symbols
        target_symbols = list(set(symbols_input) | set(required_symbols))
        logger.info(f"Target symbols: {len(target_symbols)} (union of {len(symbols_input)} requested + {len(required_symbols)} required)")
        
        if task_id:
            self._publish_progress(task_id, "symbols", f"Prepared {len(target_symbols)} symbols for analysis", 
                                 {"symbol_count": len(target_symbols), "required_symbols": len(required_symbols)})
        
        # If securities filter is provided, validate overlap
        if securities_filter:
            if target_symbols:
                overlap = set(target_symbols) & set(securities_filter)
                if not overlap:
                    raise ValueError(f"No overlap between requested symbols {target_symbols} and securities filter {securities_filter}")
                target_symbols = list(overlap)
                logger.info(f"Filtered symbols to {len(target_symbols)} based on securities list")
            else:
                # Use securities filter as the target symbols
                target_symbols = securities_filter
                logger.info(f"Using securities filter as target symbols: {len(target_symbols)} symbols")
            
        logger.info(f"Starting backtest for {len(target_symbols)} symbols (strategy_id: {strategy_id})")
        
        if task_id:
            self._publish_progress(task_id, "preparation", "Preparing date ranges and execution parameters...")
        
        # Parse dates
        if start_date:
            start_date = datetime.fromisoformat(start_date.replace('Z', '+00:00'))
        else:
            start_date = datetime.now() - timedelta(days=365)  # Default 1 year
            
        if end_date:
            end_date = datetime.fromisoformat(end_date.replace('Z', '+00:00'))
        else:
            end_date = datetime.now()
        
        if task_id:
            self._publish_progress(task_id, "execution", f"Executing backtest: {start_date.date()} to {end_date.date()}", 
                                 {"start_date": start_date.isoformat(), "end_date": end_date.isoformat(), 
                                  "symbol_count": len(target_symbols)})
        
        # Execute using DataFrame engine
        result = asyncio.run(self.strategy_engine.execute_backtest(
            strategy_code=strategy_code,
            symbols=target_symbols,
            start_date=start_date,
            end_date=end_date,
            strategy_id=strategy_id,
            timeframes=timeframes,
            date_info=date_info,
            **kwargs
        ))
        
        logger.info(f"Backtest completed: {len(result.get('instances', []))} instances found")
        
        if task_id:
            instances_count = len(result.get('instances', []))
            self._publish_progress(task_id, "completed", f"Backtest finished: {instances_count} instances found", 
                                 {"instances_count": instances_count, "success": result.get('success', True)})
        
        return result
    
    def _execute_screening(self, task_id: str = None, universe: List[str] = None, 
                                limit: int = 100, strategy_ids: List[str] = None, **kwargs) -> Dict[str, Any]:
        """Execute screening task"""
        # strategy_ids is required
        if not strategy_ids:
            raise ValueError("strategy_ids is required")
            
        strategy_codes = self._fetch_multiple_strategy_codes(strategy_ids)
        logger.info(f"Fetched {len(strategy_codes)} strategy codes from database")
        
        # For now, use the first strategy code for screening
        # TODO: Implement multi-strategy screening
        if strategy_codes:
            strategy_code = list(strategy_codes.values())[0]
        else:
            raise ValueError("No valid strategy codes found for provided strategy_ids")
        
        # Validate strategy code
        if not self.security_validator.validate_code(strategy_code):
            raise SecurityError("Strategy code contains prohibited operations")
        
        # Extract strategy requirements
        required_symbols = self._extract_required_symbols(strategy_code)
        timeframes = self._extract_timeframes(strategy_code)
        date_info = self._extract_dates(strategy_code)
        
        logger.info(f"Strategy analysis:")
        logger.info(f"  - Required symbols: {len(required_symbols)} {required_symbols[:5]}{'...' if len(required_symbols) > 5 else ''}")
        logger.info(f"  - Timeframes: {timeframes}")
        logger.info(f"  - Lookback days: {date_info['lookback_days']}")
        
        # Union required symbols with provided universe
        universe_input = universe or []
        target_universe = list(set(universe_input) | set(required_symbols))
        logger.info(f"Target universe: {len(target_universe)} symbols (union of {len(universe_input)} provided + {len(required_symbols)} required)")
            
        logger.info(f"Starting screening for {len(target_universe)} symbols, limit {limit} (strategy_ids: {strategy_ids})")
        
        # Execute using DataFrame engine
        result = asyncio.run(self.strategy_engine.execute_screening(
            strategy_code=strategy_code,
            universe=target_universe,
            limit=limit,
            strategy_ids=strategy_ids,
            timeframes=timeframes,
            date_info=date_info,
            **kwargs
        ))
        
        logger.info(f"Screening completed: {len(result.get('ranked_results', []))} results found")
        return result

    def _execute_alert(self, task_id: str = None, symbols: List[str] = None, 
                            strategy_id: str = None, **kwargs) -> Dict[str, Any]:
        """Execute alert task"""
        # strategy_id is required - always fetch from database
        if not strategy_id:
            raise ValueError("strategy_id is required")
            
        strategy_code = self._fetch_strategy_code(strategy_id)
        logger.info(f"Fetched strategy code from database for strategy_id: {strategy_id}")
        
        # Validate strategy code
        if not self.security_validator.validate_code(strategy_code):
            raise SecurityError("Strategy code contains prohibited operations")
        
        # Extract strategy requirements
        required_symbols = self._extract_required_symbols(strategy_code)
        timeframes = self._extract_timeframes(strategy_code)
        date_info = self._extract_dates(strategy_code)
        
        logger.info(f"Strategy analysis:")
        logger.info(f"  - Required symbols: {len(required_symbols)} {required_symbols[:5]}{'...' if len(required_symbols) > 5 else ''}")
        logger.info(f"  - Timeframes: {timeframes}")
        logger.info(f"  - Lookback days: {date_info['lookback_days']}")
        
        # Union required symbols with requested symbols
        symbols_input = symbols or []
        target_symbols = list(set(symbols_input) | set(required_symbols))
        logger.info(f"Target symbols: {len(target_symbols)} (union of {len(symbols_input)} requested + {len(required_symbols)} required)")
            
        logger.info(f"Starting alert for {len(target_symbols)} symbols (strategy_id: {strategy_id})")
        
        # Execute using DataFrame engine
        result = asyncio.run(self.strategy_engine.execute_realtime(
            strategy_code=strategy_code,
            symbols=target_symbols,
            timeframes=timeframes,
            date_info=date_info,
            **kwargs
        ))
        
        logger.info(f"Alert completed")
        return result
    
    async def _execute_create_strategy(self, task_id: str = None, user_id: int = None, 
                                     prompt: str = None, strategy_id: int = -1, **kwargs) -> Dict[str, Any]:
        """Execute strategy creation task with detailed logging"""
        logger.info(f"🧠 STRATEGY CREATION START - Task: {task_id}")
        logger.info(f"   👤 User ID: {user_id}")
        logger.info(f"   📝 Prompt: {prompt}")
        logger.info(f"   🆔 Strategy ID: {strategy_id} ({'Edit' if strategy_id != -1 else 'New'})")
        
        try:
            # Call the strategy generator
            logger.info(f"🚀 Calling StrategyGenerator.create_strategy_from_prompt...")
            result = await self.strategy_generator.create_strategy_from_prompt(
                user_id=user_id,
                prompt=prompt,
                strategy_id=strategy_id
            )
            
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
            
        except Exception as e:
            logger.error(f"💥 CRITICAL ERROR in strategy creation task {task_id}: {e}")
            logger.error(f"📄 Full traceback: {traceback.format_exc()}")
            
            error_result = {
                "success": False,
                "error": str(e),
                "traceback": traceback.format_exc()
            }
            
            if task_id:
                self._publish_progress(task_id, "error", f"Critical error: {str(e)}")
            
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
            
            logger.info(f"📡 Publishing progress update for {task_id}: {stage} - {message}")
            logger.debug(f"   📤 Channel: {channel}")
            logger.debug(f"   📄 Message: {message_json}")
            
            result = self.redis_client.publish(channel, message_json)
            logger.debug(f"   👥 Subscribers notified: {result}")
            
            if result == 0:
                logger.warning(f"⚠️ No subscribers listening to channel '{channel}' for task {task_id}")
            else:
                logger.debug(f"✅ Progress update published successfully to {result} subscribers")
            
        except Exception as e:
            logger.error(f"❌ Failed to publish progress for {task_id}: {e}")
            logger.error(f"📄 Full traceback: {traceback.format_exc()}")

    def _set_task_result(self, task_id: str, status: str, data: Dict[str, Any]):
        """Set task result in Redis and publish update"""
        try:
            result = {
                "task_id": task_id,
                "status": status,
                "data": data,
                "updated_at": datetime.utcnow().isoformat()
            }
            
            # Store result with 24 hour expiration
            result_key = f"task_result:{task_id}"
            result_json = json.dumps(result)
            
            logger.info(f"💾 Setting task result for {task_id}: {status}")
            logger.debug(f"   🔑 Key: {result_key}")
            logger.debug(f"   📄 Data: {result_json[:200]}...")
            
            self.redis_client.setex(result_key, 86400, result_json)
            logger.debug(f"✅ Task result stored successfully")
            
            # Publish task update for real-time notifications
            update_message = {
                "task_id": task_id,
                "status": status,
                "result": data,
                "updated_at": datetime.utcnow().isoformat()
            }
            
            if status == "error":
                update_message["error_message"] = data.get("error", "Unknown error")
            
            # Publish to Redis
            channel = "worker_task_updates"
            update_json = json.dumps(update_message)
            
            logger.info(f"📡 Publishing task update for {task_id}: {status}")
            logger.debug(f"   📤 Channel: {channel}")
            logger.debug(f"   📄 Update: {update_json[:200]}...")
            
            subscribers = self.redis_client.publish(channel, update_json)
            logger.debug(f"   👥 Subscribers notified: {subscribers}")
            
            if subscribers == 0:
                logger.warning(f"⚠️ No subscribers listening to channel '{channel}' for task {task_id}")
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
        
        # Skip DB check during normal operation to reduce overhead
        # DB connection will be checked when actually needed during task execution

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
                "timestamp": datetime.utcnow().isoformat()
            }
            
            logger.info(f"Queue stats: Priority={priority_length}, Normal={normal_length}, Total={priority_length + normal_length}")
            return stats
            
        except Exception as e:
            logger.error(f"Failed to get queue statistics: {e}")
            return {
                "error": str(e),
                "worker_id": self.worker_id,
                "timestamp": datetime.utcnow().isoformat()
            }

    def log_queue_status(self):
        """Log current queue status for monitoring"""
        stats = self.get_queue_stats()
        if "error" not in stats:
            logger.info(f"[QUEUE STATUS] Worker {self.worker_id}: "
                       f"Priority Queue: {stats['priority_queue_length']} tasks, "
                       f"Normal Queue: {stats['normal_queue_length']} tasks, "
                       f"Total: {stats['total_pending_tasks']} tasks")
        else:
            logger.error(f"[QUEUE STATUS] Failed to get queue status: {stats['error']}")


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
        "created_at": datetime.utcnow().isoformat()
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
        "created_at": datetime.utcnow().isoformat()
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
        "created_at": datetime.utcnow().isoformat()
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
        "created_at": datetime.utcnow().isoformat(),
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
        "created_at": datetime.utcnow().isoformat(),
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
            "timestamp": datetime.utcnow().isoformat()
        }
        stats["total_pending_tasks"] = stats["priority_queue_length"] + stats["normal_queue_length"]
        
        return stats
        
    except Exception as e:
        return {
            "error": str(e),
            "timestamp": datetime.utcnow().isoformat()
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
    # Print startup banner
    print("=" * 80)
    print("🚀 ATLANTIS STRATEGY WORKER STARTING UP")
    print("=" * 80)
    print(f"📅 Startup Time: {datetime.utcnow().isoformat()}")
    print(f"🐍 Python Version: {sys.version}")
    print(f"🏷️  Worker Process ID: {os.getpid()}")
    print("=" * 80)
    
    # Force flush to ensure we see the startup message
    sys.stdout.flush()
    
    # Check environment variables
    print("🔧 Environment Check:")
    env_vars = [
        "DB_HOST", "DB_USER", "DB_PASSWORD", "POSTGRES_DB",
        "REDIS_HOST", "REDIS_PORT", "OPENAI_API_KEY"
    ]
    
    for var in env_vars:
        value = os.getenv(var, "NOT_SET")
        if var in ["DB_PASSWORD", "OPENAI_API_KEY"]:
            # Don't show sensitive values
            display_value = "***SET***" if value != "NOT_SET" else "NOT_SET"
        else:
            display_value = value
        print(f"   {var}: {display_value}")
    
    sys.stdout.flush()
    
    logger.info("🎬 WORKER STARTUP INITIATED")
    logger.info("🔧 Priority Queue System: ENABLED")
    logger.info("📋 Supported Task Types: backtest, screening, alert, create_strategy")
    
    try:
        # Initialize and start worker
        print("🏗️ Initializing worker...")
        sys.stdout.flush()
        logger.info("🏗️ Initializing worker...")
        
        worker = StrategyWorker()
        
        print("✅ Worker initialized successfully!")
        sys.stdout.flush()
        logger.info("🎯 Starting main processing loop...")
        
        worker.run()
        
    except KeyboardInterrupt:
        logger.info("🛑 Received keyboard interrupt - shutting down gracefully")
        print("\n🛑 Worker shutdown requested by user")
        
    except Exception as e:
        error_msg = f"💥 FATAL ERROR during worker startup: {e}"
        logger.error(error_msg)
        logger.error(f"📄 Full traceback: {traceback.format_exc()}")
        print(error_msg)
        print(f"📄 Full traceback: {traceback.format_exc()}")
        sys.stdout.flush()
        raise
        
    finally:
        logger.info("🏁 Worker process ending")
        print("🏁 Worker process ended")
        sys.stdout.flush()
