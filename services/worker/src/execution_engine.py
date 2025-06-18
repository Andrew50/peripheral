"""
Python Execution Engine
Safely executes Python trading strategies with sandbox restrictions
"""

import ast
import builtins
import importlib
import logging
import sys
from typing import Any, Dict, List, Optional, Union

import numpy as np
import pandas as pd

from .data_provider import DataProvider

logger = logging.getLogger(__name__)


class PythonExecutionEngine:
    """Secure Python execution engine for trading strategies"""
    
    def __init__(self):
        self.allowed_modules = {
            'numpy', 'np', 'pandas', 'pd', 'scipy', 'sklearn', 'matplotlib',
            'seaborn', 'plotly', 'ta', 'talib', 'zipline', 'pyfolio',
            'quantlib', 'statsmodels', 'arch', 'empyrical', 'tsfresh',
            'stumpy', 'prophet', 'math', 'statistics', 'datetime',
            'collections', 'itertools', 'functools', 're', 'json'
        }
        
        self.restricted_builtins = {
            'open', 'file', 'input', 'raw_input', 'execfile', 'reload',
            'compile', 'eval', '__import__', 'globals', 'locals', 'vars',
            'dir', 'help', 'copyright', 'credits', 'license', 'quit', 'exit'
        }
        
        # Initialize data provider
        self.data_provider = DataProvider()
        
    async def execute(
        self,
        code: str,
        context: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Execute Python code in a restricted environment with enhanced security"""
        
        # Validate code before execution
        validator = CodeValidator()
        if not validator.validate(code):
            raise SecurityError("Code validation failed - contains prohibited operations")
        
        # Additional security checks
        if not self._perform_security_checks(code):
            raise SecurityError("Code contains security violations")
        
        # Prepare execution environment
        exec_globals = await self._create_safe_globals(context)
        exec_locals = {}
        
        try:
            # Compile code first for additional validation
            compiled_code = compile(code, '<strategy>', 'exec')
            
            # Execute the compiled code in restricted environment  
            # Note: exec() is necessary here for dynamic strategy execution
            # but we've added multiple layers of security validation
            exec(compiled_code, exec_globals, exec_locals)  # nosec B102
            
            # Extract results
            result = self._extract_results(exec_locals, exec_globals)
            
            return result
            
        except Exception as e:
            logger.error(f"Execution error: {e}")
            raise
    
    def _perform_security_checks(self, code: str) -> bool:
        """Perform additional security checks on code"""
        try:
            # Parse code into AST for analysis
            tree = ast.parse(code)
            
            # Check for prohibited operations
            for node in ast.walk(tree):
                # Block eval, exec, compile usage in user code
                if isinstance(node, ast.Call):
                    if isinstance(node.func, ast.Name):
                        if node.func.id in ['eval', 'exec', 'compile', '__import__']:
                            logger.warning(f"Prohibited function call: {node.func.id}")
                            return False
                
                # Block subprocess, os module calls
                if isinstance(node, ast.Attribute):
                    if isinstance(node.value, ast.Name):
                        if node.value.id in ['os', 'subprocess', 'sys']:
                            logger.warning(f"Prohibited module access: {node.value.id}")
                            return False
                
                # Block file operations
                if isinstance(node, ast.Call):
                    if isinstance(node.func, ast.Name):
                        if node.func.id in ['open', 'file']:
                            logger.warning(f"Prohibited file operation: {node.func.id}")
                            return False
            
            # Additional string-based checks
            code_lower = code.lower()
            prohibited_patterns = [
                '__import__', 'import os', 'import sys', 'import subprocess',
                'from os', 'from sys', 'from subprocess', 'exec(', 'eval(',
                'compile(', 'globals()', 'locals()', 'vars()', 'dir()',
                'getattr(', 'setattr(', 'delattr(', 'hasattr(',
                '__builtins__', '__globals__', '__locals__'
            ]
            
            for pattern in prohibited_patterns:
                if pattern in code_lower:
                    logger.warning(f"Prohibited pattern found: {pattern}")
                    return False
            
            return True
            
        except SyntaxError as e:
            logger.error(f"Syntax error in code: {e}")
            return False
        except Exception as e:
            logger.error(f"Error performing security checks: {e}")
            return False
    
    async def _create_safe_globals(self, context: Dict[str, Any]) -> Dict[str, Any]:
        """Create a safe global namespace for execution"""
        
        # Start with restricted builtins
        safe_builtins = {
            name: getattr(builtins, name)
            for name in dir(builtins)
            if not name.startswith('_') and name not in self.restricted_builtins
        }
        
        # Add safe built-in functions
        safe_builtin_names = {
            'len', 'range', 'enumerate', 'zip', 'map', 'filter', 'sorted',
            'sum', 'min', 'max', 'abs', 'round', 'pow', 'divmod',
            'int', 'float', 'str', 'bool', 'list', 'tuple', 'dict', 'set',
            'any', 'all', 'isinstance', 'issubclass', 'hasattr', 'getattr',
            'setattr', 'type', 'callable', 'print'
        }
        
        for name in safe_builtin_names:
            if hasattr(builtins, name):
                safe_builtins[name] = getattr(builtins, name)
        
        # Core globals
        exec_globals = {
            '__builtins__': {k: getattr(builtins, k) for k in safe_builtins if hasattr(builtins, k)},
            '__name__': '__main__',
            '__doc__': None,
        }
        
        # Add allowed modules
        for module_name in self.allowed_modules:
            try:
                if module_name == 'np':
                    exec_globals['np'] = np
                elif module_name == 'pd':
                    exec_globals['pd'] = pd
                else:
                    module = importlib.import_module(module_name)
                    exec_globals[module_name] = module
            except ImportError:
                logger.warning(f"Module {module_name} not available")
                continue
        
        # Add context data
        exec_globals.update({
            'input_data': context.get('input_data', {}),
            'prepared_data': context.get('prepared_data', {}),
            'libraries': context.get('libraries', [])
        })
        
        # Add strategy helper functions and data accessors
        data_functions = await self._get_data_accessor_functions()
        exec_globals.update(data_functions)
        
        # Add other helper functions
        exec_globals.update(self._get_strategy_helpers())
        
        return exec_globals
    
    async def _get_data_accessor_functions(self) -> Dict[str, Any]:
        """Get raw data accessor functions only - no pre-calculated indicators"""
        
        # Create async wrapper for data provider methods
        def make_sync_wrapper(async_func):
            """Convert async function to sync for use in strategy code"""
            import asyncio
            def wrapper(*args, **kwargs):
                try:
                    loop = asyncio.get_event_loop()
                    if loop.is_running():
                        # If we're already in an async context, create a new task
                        import concurrent.futures
                        with concurrent.futures.ThreadPoolExecutor() as executor:
                            future = executor.submit(asyncio.run, async_func(*args, **kwargs))
                            return future.result()
                    else:
                        return loop.run_until_complete(async_func(*args, **kwargs))
                except Exception as e:
                    logger.error(f"Error in data accessor function: {e}")
                    return {}
            return wrapper
        
        # ==================== RAW DATA RETRIEVAL ONLY ====================
        
        def get_price_data(symbol: str, timeframe: str = '1d', days: int = 30, 
                          extended_hours: bool = False, start_time: str = None, end_time: str = None) -> Dict:
            return make_sync_wrapper(self.data_provider.get_price_data)(
                symbol, timeframe, days, extended_hours, start_time, end_time
            )
        
        def get_historical_data(symbol: str, timeframe: str = '1d', periods: int = 100, offset: int = 0) -> Dict:
            return make_sync_wrapper(self.data_provider.get_historical_data)(
                symbol, timeframe, periods, offset
            )
        
        def get_security_info(symbol: str) -> Dict:
            return make_sync_wrapper(self.data_provider.get_security_info)(symbol)
        
        def get_multiple_symbols_data(symbols: List[str], timeframe: str = '1d', days: int = 30) -> Dict[str, Dict]:
            return make_sync_wrapper(self.data_provider.get_multiple_symbols_data)(
                symbols, timeframe, days
            )
        
        # ==================== RAW FUNDAMENTAL DATA ====================
        
        def get_fundamental_data(symbol: str, metrics: Optional[List[str]] = None) -> Dict:
            return make_sync_wrapper(self.data_provider.get_fundamental_data)(symbol, metrics)
        
        def get_earnings_data(symbol: str, quarters: int = 8) -> Dict:
            # Placeholder implementation - would need earnings table
            return {
                'eps_actual': [], 'eps_estimate': [], 'revenue_actual': [],
                'revenue_estimate': [], 'report_dates': [], 'surprise_percent': []
            }
        
        def get_financial_statements(symbol: str, statement_type: str = 'income', periods: int = 4) -> Dict:
            # Placeholder implementation - would need financial statements table
            return {'periods': [], 'line_items': {}}
        
        # ==================== RAW MARKET DATA ====================
        
        def get_sector_data(sector: str = None, days: int = 5) -> Dict:
            return make_sync_wrapper(self.data_provider.get_sector_performance)(sector, days, None)
        
        def get_market_indices(indices: List[str] = None, days: int = 30) -> Dict[str, Dict]:
            if not indices:
                indices = ['SPY', 'QQQ', 'IWM', 'VIX']
            return get_multiple_symbols_data(indices, '1d', days)
        
        def get_economic_calendar(days_ahead: int = 30) -> List[Dict]:
            # Placeholder implementation - would need economic calendar data
            return []
        
        # ==================== RAW VOLUME & FLOW DATA ====================
        
        def get_volume_data(symbol: str, days: int = 30) -> Dict:
            price_data = get_price_data(symbol, '1d', days)
            if price_data and price_data.get('volume'):
                return {
                    'timestamps': price_data['timestamps'],
                    'volume': price_data['volume'],
                    'dollar_volume': [price_data['close'][i] * price_data['volume'][i] 
                                    for i in range(len(price_data['close']))],
                    'trade_count': [0] * len(price_data['volume'])  # Placeholder
                }
            return {'timestamps': [], 'volume': [], 'dollar_volume': [], 'trade_count': []}
        
        def get_options_chain(symbol: str, expiration: str = None) -> Dict:
            # Placeholder implementation - would need options data
            return {'calls': [], 'puts': []}
        
        # ==================== RAW SENTIMENT & NEWS DATA ====================
        
        def get_news_sentiment(symbol: str = None, days: int = 7) -> List[Dict]:
            # Placeholder implementation - would need news data
            return []
        
        def get_social_mentions(symbol: str, days: int = 7) -> Dict:
            # Placeholder implementation - would need social data
            return {'timestamps': [], 'mention_count': [], 'sentiment_scores': [], 'platforms': []}
        
        # ==================== RAW INSIDER & INSTITUTIONAL DATA ====================
        
        def get_insider_trades(symbol: str, days: int = 90) -> List[Dict]:
            # Placeholder implementation - would need insider data
            return []
        
        def get_institutional_holdings(symbol: str, quarters: int = 4) -> List[Dict]:
            # Placeholder implementation - would need institutional data
            return []
        
        def get_short_data(symbol: str) -> Dict:
            # Placeholder implementation - would need short interest data
            return {
                'short_interest': 0, 'short_ratio': 0, 'days_to_cover': 0,
                'short_percent_float': 0, 'previous_short_interest': 0
            }
        
        # ==================== SCREENING & FILTERING ====================
        
        def scan_universe(filters: Dict = None, sort_by: str = None, limit: int = 100) -> Dict:
            return make_sync_wrapper(self.data_provider.scan_universe)(filters, sort_by, limit)
        
        def get_universe_symbols(universe: str = 'sp500') -> List[str]:
            # Placeholder implementation - would need universe definitions
            if universe == 'sp500':
                return ['AAPL', 'MSFT', 'GOOGL', 'AMZN', 'TSLA']  # Sample
            return []
        
        # ==================== UTILITY FUNCTIONS ====================
        
        def validate_symbol(symbol: str) -> Dict:
            info = get_security_info(symbol)
            return {
                'valid': bool(info),
                'active': info.get('active', False),
                'exchange': info.get('primary_exchange', ''),
                'asset_type': 'stock'  # Assuming stocks for now
            }
        
        def get_trading_calendar(start_date: str = None, end_date: str = None, 
                                market: str = 'NYSE') -> Dict:
            # Placeholder implementation - would need trading calendar
            return {'trading_days': [], 'holidays': [], 'early_closes': []}
        
        def get_market_status() -> Dict:
            # Placeholder implementation - would need real-time market status
            return {
                'is_open': False, 'next_open': '', 'next_close': '',
                'current_session': 'closed'
            }
        
        # ==================== UTILITY FUNCTIONS FOR CALCULATIONS ====================
        
        def calculate_returns(prices: List[float], periods: int = 1) -> List[float]:
            """Calculate simple returns over specified periods"""
            if len(prices) <= periods:
                return []
            
            returns = []
            for i in range(periods, len(prices)):
                ret = (prices[i] / prices[i - periods]) - 1
                returns.append(ret)
            
            return returns
        
        def calculate_log_returns(prices: List[float], periods: int = 1) -> List[float]:
            """Calculate logarithmic returns"""
            if len(prices) <= periods:
                return []
            
            import math
            returns = []
            for i in range(periods, len(prices)):
                ret = math.log(prices[i] / prices[i - periods])
                returns.append(ret)
            
            return returns
        
        def rolling_window(data: List[float], window: int) -> List[List[float]]:
            """Create rolling windows of data for calculations"""
            if len(data) < window:
                return []
            
            windows = []
            for i in range(window - 1, len(data)):
                windows.append(data[i - window + 1:i + 1])
            
            return windows
        
        def calculate_percentile(data: List[float], percentile: float) -> float:
            """Calculate percentile of data"""
            if not data:
                return 0.0
            
            sorted_data = sorted(data)
            index = (percentile / 100) * (len(sorted_data) - 1)
            
            if index.is_integer():
                return sorted_data[int(index)]
            else:
                lower = sorted_data[int(index)]
                upper = sorted_data[int(index) + 1]
                return lower + (upper - lower) * (index - int(index))
        
        def normalize_data(data: List[float], method: str = 'z_score') -> List[float]:
            """Normalize data using various methods"""
            if not data:
                return []
            
            if method == 'z_score':
                mean_val = sum(data) / len(data)
                variance = sum((x - mean_val) ** 2 for x in data) / len(data)
                std_val = variance ** 0.5
                if std_val == 0:
                    return [0.0] * len(data)
                return [(x - mean_val) / std_val for x in data]
            
            elif method == 'min_max':
                min_val = min(data)
                max_val = max(data)
                if max_val == min_val:
                    return [0.5] * len(data)
                return [(x - min_val) / (max_val - min_val) for x in data]
            
            elif method == 'robust':
                median_val = sorted(data)[len(data) // 2]
                mad = sorted([abs(x - median_val) for x in data])[len(data) // 2]
                if mad == 0:
                    return [0.0] * len(data)
                return [(x - median_val) / mad for x in data]
            
            return data
        
        def vectorized_operation(values: List[float], operation: str, operand: float = None) -> List[float]:
            """Apply vectorized operations for performance"""
            import math
            
            if operation == 'add' and operand is not None:
                return [x + operand for x in values]
            elif operation == 'subtract' and operand is not None:
                return [x - operand for x in values]
            elif operation == 'multiply' and operand is not None:
                return [x * operand for x in values]
            elif operation == 'divide' and operand is not None:
                return [x / operand if operand != 0 else 0 for x in values]
            elif operation == 'power' and operand is not None:
                return [x ** operand for x in values]
            elif operation == 'log':
                return [math.log(x) if x > 0 else 0 for x in values]
            elif operation == 'sqrt':
                return [math.sqrt(x) if x >= 0 else 0 for x in values]
            else:
                return values
        
        def compare_lists(list1: List[float], list2: List[float], operator: str) -> List[bool]:
            """Compare two lists element-wise"""
            min_len = min(len(list1), len(list2))
            
            if operator == '>':
                return [list1[i] > list2[i] for i in range(min_len)]
            elif operator == '<':
                return [list1[i] < list2[i] for i in range(min_len)]
            elif operator == '>=':
                return [list1[i] >= list2[i] for i in range(min_len)]
            elif operator == '<=':
                return [list1[i] <= list2[i] for i in range(min_len)]
            elif operator == '==':
                return [list1[i] == list2[i] for i in range(min_len)]
            elif operator == '!=':
                return [list1[i] != list2[i] for i in range(min_len)]
            else:
                return [False] * min_len
        
        # Return all raw data accessor functions
        return {
            # Raw data retrieval
            'get_price_data': get_price_data,
            'get_historical_data': get_historical_data,
            'get_security_info': get_security_info,
            'get_multiple_symbols_data': get_multiple_symbols_data,
            
            # Raw fundamental data
            'get_fundamental_data': get_fundamental_data,
            'get_earnings_data': get_earnings_data,
            'get_financial_statements': get_financial_statements,
            
            # Raw market data
            'get_sector_data': get_sector_data,
            'get_market_indices': get_market_indices,
            'get_economic_calendar': get_economic_calendar,
            
            # Raw volume & flow data
            'get_volume_data': get_volume_data,
            'get_options_chain': get_options_chain,
            
            # Raw sentiment & news data
            'get_news_sentiment': get_news_sentiment,
            'get_social_mentions': get_social_mentions,
            
            # Raw insider & institutional data
            'get_insider_trades': get_insider_trades,
            'get_institutional_holdings': get_institutional_holdings,
            'get_short_data': get_short_data,
            
            # Screening & filtering
            'scan_universe': scan_universe,
            'get_universe_symbols': get_universe_symbols,
            
            # Utility functions
            'validate_symbol': validate_symbol,
            'get_trading_calendar': get_trading_calendar,
            'get_market_status': get_market_status,
            
            # Calculation utilities
            'calculate_returns': calculate_returns,
            'calculate_log_returns': calculate_log_returns,
            'rolling_window': rolling_window,
            'calculate_percentile': calculate_percentile,
            'normalize_data': normalize_data,
            'vectorized_operation': vectorized_operation,
            'compare_lists': compare_lists,
        }
    
    def _get_strategy_helpers(self) -> Dict[str, Any]:
        """Get helper functions for strategy development"""
        
        def log(message: str, level: str = 'info'):
            """Log a message during strategy execution"""
            getattr(logger, level.lower())(f"Strategy: {message}")
        
        def save_result(key: str, value: Any):
            """Save a result to be returned"""
            if not hasattr(save_result, 'results'):
                save_result.results = {}
            save_result.results[key] = value
        
        return {
            'log': log,
            'save_result': save_result,
        }
    
    def _extract_results(self, locals_dict: Dict[str, Any], globals_dict: Dict[str, Any]) -> Dict[str, Any]:
        """Extract results from execution locals and globals"""
        
        # Get saved results from helper function
        results = {}
        # Check both locals and globals for save_result function
        save_result_func = locals_dict.get('save_result') or globals_dict.get('save_result')
        if save_result_func and hasattr(save_result_func, 'results'):
            results.update(save_result_func.results)
        
        # Extract variables that don't start with underscore
        for key, value in locals_dict.items():
            if not key.startswith('_') and key not in {'save_result'}:
                try:
                    # Try to serialize the value to ensure it's JSON-compatible
                    if self._is_json_serializable(value):
                        results[key] = value
                    else:
                        # Convert to string representation for complex objects
                        results[key] = str(value)
                except Exception:
                    results[key] = f"<{type(value).__name__}>"
        
        return results
    
    def _is_json_serializable(self, obj: Any) -> bool:
        """Check if an object is JSON serializable"""
        try:
            import json
            json.dumps(obj)
            return True
        except (TypeError, ValueError):
            return False


class CodeValidator:
    """Validates Python code for security issues"""
    
    def __init__(self):
        self.forbidden_nodes = {
            ast.Import: self._check_import,
            ast.ImportFrom: self._check_import_from,
            ast.Call: self._check_function_call,
            ast.Attribute: self._check_attribute_access,
        }
        
        self.forbidden_functions = {
            'exec', 'eval', 'compile', '__import__', 'open', 'file',
            'input', 'raw_input', 'globals', 'locals', 'vars'
        }
        
        self.forbidden_modules = {
            'os', 'sys', 'subprocess', 'socket', 'urllib', 'requests',
            'http', 'ftplib', 'smtplib', 'telnetlib', 'pickle', 'marshal',
            'shelve', 'dbm', 'sqlite3', 'threading', 'multiprocessing'
        }
    
    def validate(self, code: str) -> bool:
        """Validate code for security issues"""
        try:
            tree = ast.parse(code)
            self._check_ast_node(tree)
            return True
        except (SyntaxError, SecurityError):
            return False
    
    def _check_ast_node(self, node: ast.AST):
        """Recursively check AST nodes"""
        for node_type, checker in self.forbidden_nodes.items():
            if isinstance(node, node_type):
                if not checker(node):
                    raise SecurityError(f"Forbidden operation: {node_type.__name__}")
        
        for child in ast.iter_child_nodes(node):
            self._check_ast_node(child)
    
    def _check_import(self, node: ast.Import) -> bool:
        """Check import statements"""
        for alias in node.names:
            if alias.name in self.forbidden_modules:
                return False
        return True
    
    def _check_import_from(self, node: ast.ImportFrom) -> bool:
        """Check from-import statements"""
        if node.module in self.forbidden_modules:
            return False
        return True
    
    def _check_function_call(self, node: ast.Call) -> bool:
        """Check function calls"""
        if isinstance(node.func, ast.Name):
            if node.func.id in self.forbidden_functions:
                return False
        return True
    
    def _check_attribute_access(self, node: ast.Attribute) -> bool:
        """Check attribute access"""
        # Prevent access to dangerous attributes
        dangerous_attrs = {'__globals__', '__locals__', '__code__', '__dict__'}
        if node.attr in dangerous_attrs:
            return False
        return True


class SecurityError(Exception):
    """Raised when code contains security violations"""
    pass