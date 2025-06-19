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

try:
    from .data_provider import DataProvider
except ImportError:
    from data_provider import DataProvider

logger = logging.getLogger(__name__)


class PythonExecutionEngine:
    """Secure Python execution engine for trading strategies"""

    def __init__(self):
        self.allowed_modules = {
            "numpy",
            "np",
            "pandas",
            "pd",
            "scipy",
            "sklearn",
            "matplotlib",
            "seaborn",
            "plotly",
            "ta",
            "talib",
            "zipline",
            "pyfolio",
            "quantlib",
            "statsmodels",
            "arch",
            "empyrical",
            "tsfresh",
            "stumpy",
            "prophet",
            "math",
            "statistics",
            "datetime",
            "collections",
            "itertools",
            "functools",
            "re",
            "json",
            "fuzzywuzzy",
            "difflib",
            "string",
            "unicodedata",
        }

        self.restricted_builtins = {
            "open",
            "file",
            "input",
            "raw_input",
            "execfile",
            "reload",
            "compile",
            "eval",
            "__import__",
            "globals",
            "locals",
            "vars",
            "dir",
            "help",
            "copyright",
            "credits",
            "license",
            "quit",
            "exit",
        }

        # Initialize data provider
        self.data_provider = DataProvider()

    async def execute(self, code: str, context: Dict[str, Any]) -> Dict[str, Any]:
        """Execute Python code in a restricted environment with enhanced security"""

        # Validate code before execution
        validator = CodeValidator()
        if not validator.validate(code):
            raise SecurityError(
                "Code validation failed - contains prohibited operations"
            )

        # Additional security checks
        if not self._perform_security_checks(code):
            raise SecurityError("Code contains security violations")

        # Prepare execution environment
        exec_globals = await self._create_safe_globals(context)
        exec_locals = {}

        try:
            # Compile code first for additional validation
            compiled_code = compile(code, "<strategy>", "exec")

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
                        if node.func.id in ["eval", "exec", "compile", "__import__"]:
                            logger.warning(f"Prohibited function call: {node.func.id}")
                            return False

                # Block subprocess, os module calls
                if isinstance(node, ast.Attribute):
                    if isinstance(node.value, ast.Name):
                        if node.value.id in ["os", "subprocess", "sys"]:
                            logger.warning(f"Prohibited module access: {node.value.id}")
                            return False

                # Block file operations
                if isinstance(node, ast.Call):
                    if isinstance(node.func, ast.Name):
                        if node.func.id in ["open", "file"]:
                            logger.warning(f"Prohibited file operation: {node.func.id}")
                            return False

            # Additional string-based checks
            code_lower = code.lower()
            prohibited_patterns = [
                "import os",
                "import sys",
                "import subprocess",
                "from os",
                "from sys",
                "from subprocess",
                "exec(",
                "eval(",
                "compile(",
                "globals()",
                "locals()",
                "vars()",
                "dir()",
                "__builtins__",
                "__globals__",
                "__locals__",
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
            if not name.startswith("_") and name not in self.restricted_builtins
        }

        # Add safe built-in functions
        safe_builtin_names = {
            "len",
            "range",
            "enumerate",
            "zip",
            "map",
            "filter",
            "sorted",
            "sum",
            "min",
            "max",
            "abs",
            "round",
            "pow",
            "divmod",
            "int",
            "float",
            "str",
            "bool",
            "list",
            "tuple",
            "dict",
            "set",
            "any",
            "all",
            "isinstance",
            "issubclass",
            "hasattr",
            "getattr",
            "setattr",
            "type",
            "callable",
            "print",
        }

        for name in safe_builtin_names:
            if hasattr(builtins, name):
                safe_builtins[name] = getattr(builtins, name)

        # Create safe __import__ function
        def safe_import(name, globals=None, locals=None, fromlist=(), level=0):
            """Safe import function that only allows whitelisted modules"""
            if name in self.allowed_modules:
                return __import__(name, globals, locals, fromlist, level)
            else:
                raise ImportError(f"Module '{name}' is not allowed")

        # Add safe import to builtins
        safe_builtins["__import__"] = safe_import
        
        # Core globals
        exec_globals = {
            "__builtins__": {
                k: getattr(builtins, k) if k != "__import__" else safe_import 
                for k in safe_builtins if hasattr(builtins, k) or k == "__import__"
            },
            "__name__": "__main__",
            "__doc__": None,
        }

        # Add allowed modules
        for module_name in self.allowed_modules:
            try:
                if module_name == "np":
                    exec_globals["np"] = np
                elif module_name == "pd":
                    exec_globals["pd"] = pd
                else:
                    module = importlib.import_module(module_name)
                    exec_globals[module_name] = module
            except ImportError:
                logger.warning(f"Module {module_name} not available")
                continue

        # Add context data
        exec_globals.update(
            {
                "input_data": context.get("input_data", {}),
                "prepared_data": context.get("prepared_data", {}),
                "libraries": context.get("libraries", []),
            }
        )

        # Add strategy helper functions and data accessors
        data_functions = await self._get_data_accessor_functions()
        exec_globals.update(data_functions)

        # Add other helper functions
        exec_globals.update(self._get_strategy_helpers())

        return exec_globals

    async def _get_data_accessor_functions(self) -> Dict[str, Any]:
        """Get data accessor functions for strategy code"""

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
                            future = executor.submit(
                                asyncio.run, async_func(*args, **kwargs)
                            )
                            return future.result()
                    else:
                        return loop.run_until_complete(async_func(*args, **kwargs))
                except Exception as e:
                    logger.error(f"Error in data accessor function: {e}")
                    return {}

            return wrapper

        # Core data retrieval functions
        def get_price_data(
            symbol: str,
            timeframe: str = "1d",
            days: int = 30,
            extended_hours: bool = False,
            start_time: str = None,
            end_time: str = None,
        ) -> Dict:
            return make_sync_wrapper(self.data_provider.get_price_data)(
                symbol, timeframe, days, extended_hours, start_time, end_time
            )

        def get_historical_data(
            symbol: str, timeframe: str = "1d", periods: int = 100, offset: int = 0
        ) -> Dict:
            return make_sync_wrapper(self.data_provider.get_historical_data)(
                symbol, timeframe, periods, offset
            )

        def get_security_info(symbol: str) -> Dict:
            return make_sync_wrapper(self.data_provider.get_security_info)(symbol)

        def get_multiple_symbols_data(
            symbols: List[str], timeframe: str = "1d", days: int = 30
        ) -> Dict[str, Dict]:
            return make_sync_wrapper(self.data_provider.get_multiple_symbols_data)(
                symbols, timeframe, days
            )

        def get_fundamental_data(
            symbol: str, metrics: Optional[List[str]] = None
        ) -> Dict:
            return make_sync_wrapper(self.data_provider.get_fundamental_data)(
                symbol, metrics
            )

        def scan_universe(
            filters: Dict = None, sort_by: str = None, limit: int = 100
        ) -> Dict:
            return make_sync_wrapper(self.data_provider.scan_universe)(
                filters, sort_by, limit
            )

        # Utility calculation functions
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
                windows.append(data[i - window + 1 : i + 1])

            return windows

        def calculate_correlation(data1: List[float], data2: List[float]) -> float:
            """Calculate correlation coefficient between two datasets"""
            if len(data1) != len(data2) or len(data1) < 2:
                return 0.0

            n = len(data1)
            sum_x = sum(data1)
            sum_y = sum(data2)
            sum_xx = sum(x * x for x in data1)
            sum_yy = sum(y * y for y in data2)
            sum_xy = sum(data1[i] * data2[i] for i in range(n))

            # Correlation formula
            numerator = n * sum_xy - sum_x * sum_y
            denominator = ((n * sum_xx - sum_x * sum_x) * (n * sum_yy - sum_y * sum_y)) ** 0.5

            if denominator == 0:
                return 0.0

            return numerator / denominator

        # Return core data accessor functions
        return {
            # Data retrieval
            "get_price_data": get_price_data,
            "get_historical_data": get_historical_data,
            "get_security_info": get_security_info,
            "get_multiple_symbols_data": get_multiple_symbols_data,
            "get_fundamental_data": get_fundamental_data,
            "scan_universe": scan_universe,
            # Calculation utilities
            "calculate_returns": calculate_returns,
            "calculate_log_returns": calculate_log_returns,
            "rolling_window": rolling_window,
            "calculate_correlation": calculate_correlation,
        }

    def _get_strategy_helpers(self) -> Dict[str, Any]:
        """Get helper functions for strategy development"""

        def log(message: str, level: str = "info"):
            """Log a message during strategy execution"""
            getattr(logger, level.lower())(f"Strategy: {message}")

        def save_result(key: str, value: Any):
            """Save a result to be returned"""
            if not hasattr(save_result, "results"):
                save_result.results = {}
            save_result.results[key] = value

        return {
            "log": log,
            "save_result": save_result,
        }

    def _extract_results(
        self, locals_dict: Dict[str, Any], globals_dict: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Extract results from execution locals and globals"""

        # Get saved results from helper function
        results = {}
        # Check both locals and globals for save_result function
        save_result_func = locals_dict.get("save_result") or globals_dict.get(
            "save_result"
        )
        if save_result_func and hasattr(save_result_func, "results"):
            results.update(save_result_func.results)

        # Extract variables that don't start with underscore
        for key, value in locals_dict.items():
            if not key.startswith("_") and key not in {"save_result"}:
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
            "exec",
            "eval",
            "compile",
            "__import__",
            "open",
            "file",
            "input",
            "raw_input",
            "globals",
            "locals",
            "vars",
        }

        self.forbidden_modules = {
            "os",
            "sys",
            "subprocess",
            "socket",
            "urllib",
            "requests",
            "http",
            "ftplib",
            "smtplib",
            "telnetlib",
            "pickle",
            "marshal",
            "shelve",
            "dbm",
            "sqlite3",
            "threading",
            "multiprocessing",
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
        dangerous_attrs = {"__globals__", "__locals__", "__code__", "__dict__"}
        if node.attr in dangerous_attrs:
            return False
        return True


class SecurityError(Exception):
    """Raised when code contains security violations"""

    pass
