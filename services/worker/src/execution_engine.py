"""
Python Execution Engine
Safely executes Python trading strategies with sandbox restrictions
"""

import ast
import builtins
import importlib
import logging
import sys
from typing import Any, Dict, Optional

import numpy as np
import pandas as pd

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
        
    async def execute(
        self,
        code: str,
        context: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Execute Python code in a restricted environment"""
        
        # Prepare execution environment
        exec_globals = self._create_safe_globals(context)
        exec_locals = {}
        
        try:
            # Execute the code
            exec(code, exec_globals, exec_locals)
            
            # Extract results
            result = self._extract_results(exec_locals)
            
            return result
            
        except Exception as e:
            logger.error(f"Execution error: {e}")
            raise
    
    def _create_safe_globals(self, context: Dict[str, Any]) -> Dict[str, Any]:
        """Create a safe global namespace for execution"""
        
        # Start with restricted builtins
        safe_builtins = {
            name: getattr(builtins, name)
            for name in dir(builtins)
            if not name.startswith('_') and name not in self.restricted_builtins
        }
        
        # Add safe built-in functions
        safe_builtins.update({
            'len', 'range', 'enumerate', 'zip', 'map', 'filter', 'sorted',
            'sum', 'min', 'max', 'abs', 'round', 'pow', 'divmod',
            'int', 'float', 'str', 'bool', 'list', 'tuple', 'dict', 'set',
            'any', 'all', 'isinstance', 'issubclass', 'hasattr', 'getattr',
            'setattr', 'type', 'callable', 'print'
        })
        
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
        
        # Add strategy helper functions
        exec_globals.update(self._get_strategy_helpers())
        
        return exec_globals
    
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
        
        def get_market_data(symbol: str, timeframe: str = '1d', limit: int = 100):
            """Get market data for a symbol (placeholder)"""
            # This would integrate with your actual data provider
            logger.info(f"Fetching market data for {symbol}")
            return pd.DataFrame()  # Placeholder
        
        return {
            'log': log,
            'save_result': save_result,
            'get_market_data': get_market_data
        }
    
    def _extract_results(self, locals_dict: Dict[str, Any]) -> Dict[str, Any]:
        """Extract results from execution locals"""
        
        # Get saved results from helper function
        results = {}
        if hasattr(locals_dict.get('save_result'), 'results'):
            results.update(locals_dict['save_result'].results)
        
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