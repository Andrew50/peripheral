"""
Security Validator
Validates Python code for security issues before execution and ensures compliance with data accessor strategy requirements.

The strategy engine expects functions that:
1. Accept no parameters (use get_bar_data() and get_general_data() instead)
2. Return a list of dictionaries with 'ticker' and 'timestamp' fields
3. Use only approved safe modules (pandas, numpy, math, datetime, etc.)
4. Use get_bar_data() and get_general_data() functions to fetch required data
5. Do not access dangerous system functions or attributes

Available data accessor functions:
- get_bar_data(timeframe, security_ids, columns, min_bars): Returns numpy array with OHLCV data
- get_general_data(security_ids, columns): Returns pandas DataFrame with security metadata
"""

import ast
import logging
import re
import keyword
from typing import Set, List, Dict, Any, Optional, Union
import sys

logger = logging.getLogger(__name__)

        # Forbidden built-in functions (only truly dangerous ones)
forbidden_functions = {
    # Code execution
    "exec", "eval", "compile", "__import__", "breakpoint",
    # File and system access
    "open", "file", "input", "raw_input",
    # Introspection and manipulation
    "globals", "locals", "vars", "dir", "delattr", "setattr", "hasattr", "getattr",
    # System control
    "exit", "quit", "help", "copyright", "credits", "license",
    # Memory and object manipulation that could be dangerous
    "memoryview", "bytearray", "callable", "classmethod", "staticmethod",
    "property", "super", "isinstance", "issubclass", "iter", "next",
    # Dangerous built-ins
    "id", "hash", "repr", "ascii", "bin", "hex", "oct",
}
        
        # Explicitly allowed built-in functions for strategy processing
allowed_functions = {
    # Type conversions needed for pandas
    "int", "float", "str", "bool", "list", "dict", "tuple", "set",
    # Math operations
    "abs", "round", "min", "max", "sum", "pow", "divmod",
    # Iteration and collections
    "len", "range", "enumerate", "zip", "sorted", "reversed",
    "any", "all", "map", "filter",
    # Type checking
    "type", "bytes",
    # Object creation
    "slice", "complex", "frozenset", "object", "format",
    # Safe console output
    "print",
    # Data accessor functions
    "get_bar_data", "get_general_data"
}

        # Forbidden modules (exhaustive security list)
forbidden_modules = {
    # System and OS
    "os", "sys", "platform", "ctypes", "winreg", "msvcrt", "nt", "posix", "pwd", "grp",
    # Process and threading
    "subprocess", "threading", "multiprocessing", "_thread", "concurrent", "asyncio",
    "queue", "sched", "signal", "resource", "mmap",
    # Network and HTTP
    "socket", "urllib", "requests", "http", "ftplib", "smtplib", "telnetlib",
    "nntplib", "poplib", "imaplib", "ssl", "selectors", "socketserver",
    # File and data persistence
    "pickle", "marshal", "shelve", "dbm", "sqlite3", "csv", "configparser",
    "tempfile", "shutil", "glob", "fnmatch", "linecache", "fileinput", "pathlib",
    # Code compilation and inspection
    "code", "codeop", "ast", "dis", "inspect", "types", "importlib", "pkgutil",
    "modulefinder", "runpy", "zipimport",
    # Security and encryption
    "hashlib", "hmac", "secrets", "uuid", "crypt", "getpass", "keyring",
    # External process interaction
    "pty", "tty", "pipes", "popen2", "commands", "distutils", "ensurepip",
    # Development and debugging
    "pdb", "trace", "traceback", "warnings", "gc", "weakref", "profile", "cProfile",
    "timeit", "doctest", "unittest", "logging", "argparse", "optparse",
    # Data formats and protocols
    "xml", "html", "email", "mailbox", "mimetypes", "base64", "binhex", "binascii",
    "quopri", "uu", "zlib", "gzip", "bz2", "lzma", "zipfile", "tarfile",
    # Internet and protocols
    "webbrowser", "cgi", "cgitb", "wsgiref", "xmlrpc", "urllib3",
    # GUI and multimedia
    "tkinter", "turtle", "cmd", "shlex", "readline", "rlcompleter",
    # Database
    "mysql", "psycopg2", "pymongo", "redis", "sqlalchemy",
    # External libraries that could be dangerous
    "flask", "django", "tornado", "twisted", "paramiko", "fabric",
}

        # Allowed safe modules for DataFrame-based data science (strict whitelist)
allowed_modules = {
    "pandas", "numpy", "math", "statistics", "random",
    "datetime", "time", "decimal", "fractions", "collections",
    "itertools", "functools", "operator", "copy", "json", "re",
    "string", "textwrap", "calendar", "bisect", "heapq", "array",
    "typing", "plotly"  # For type annotations like List[Dict]
}

        # Common module aliases mapping to canonical names
module_aliases = {
    "pd": "pandas",
    "np": "numpy",
    "px": "plotly",
    "graph_objects": "plotly",
    "express": "plotly",
    "subplots": "plotly",
    "make_subplots": "plotly"
}

        # Forbidden attributes (comprehensive dunder and internal attributes)
forbidden_attributes = {
    # Object internals
    "__globals__", "__locals__", "__code__", "__dict__", "__class__", "__bases__",
    "__mro__", "__subclasses__", "__module__", "__file__", "__name__", "__doc__",
    "__annotations__", "__qualname__", "__closure__", "__defaults__",
    "__kwdefaults__", "__builtins__", "__import__", "__cached__", "__spec__",
    "__package__", "__loader__", "__path__", "__all__", "__version__",
    # Function internals
    "func_globals", "func_code", "func_closure", "func_defaults", "func_dict",
    "im_func", "im_self", "im_class", "gi_frame", "gi_code", "cr_frame", "cr_code",
    # Type internals
    "__new__", "__init__", "__del__", "__repr__", "__str__", "__bytes__",
    "__hash__", "__bool__", "__getattribute__", "__getattr__", "__setattr__",
    "__delattr__", "__dir__", "__get__", "__set__", "__delete__", "__slots__",
    # Dangerous methods
    "__reduce__", "__reduce_ex__", "__getstate__", "__setstate__", "__getnewargs__",
    "__sizeof__", "__format__", "__subclasshook__", "__instancecheck__",
    "__subclasscheck__", "__call__", "__enter__", "__exit__",
}

        # Strategy requirements - updated for data accessor approach
required_instance_fields = {"ticker", "timestamp"}
reserved_global_names = {"pd", "pandas", "np", "numpy", "datetime", "timedelta", "math", 
                                     "get_bar_data", "get_general_data"}
        
        # Data accessor function names
data_accessor_functions = {"get_bar_data", "get_general_data"}

def _normalize_module_name(module_name: str) -> str:
    """Normalize module name by resolving aliases to canonical names"""
    root_module = module_name.split('.')[0]
    return module_aliases.get(root_module, root_module)

def extract_min_bars_requirements(code: str) -> List[Dict[str, Any]]:
    """
    Extract min_bars requirements from all get_bar_data() calls in strategy code.
    
    Args:
        code: Strategy code to analyze
        
    Returns:
        List of dictionaries with call info:
        [
            {
                'function': 'get_bar_data',
                'timeframe': '1d',
                'min_bars': 20,
                'line_number': 15
            },
            ...
        ]
    """
    requirements = []
    
    try:
        # Parse the code into an AST
        tree = ast.parse(code)
        
        # Walk through all nodes in the AST
        for node in ast.walk(tree):
            if isinstance(node, ast.Call):
                # Check if this is a get_bar_data call
                func_name = None
                if isinstance(node.func, ast.Name):
                    func_name = node.func.id
                elif isinstance(node.func, ast.Attribute):
                    func_name = node.func.attr
                
                if func_name == 'get_bar_data':
                    # Extract parameters from the call
                    call_info = _extract_get_bar_data_params(node)
                    if call_info:
                        requirements.append(call_info)
                        
    except SyntaxError as e:
        logger.warning(f"Failed to parse strategy code for min_bars extraction: {e}")
    except Exception as e:
        logger.error(f"Unexpected error extracting min_bars requirements: {e}")
        
    return requirements

def _extract_get_bar_data_params(call_node: ast.Call) -> Optional[Dict[str, Any]]:
    """
    Extract parameters from a get_bar_data() call node.
    
    Args:
        call_node: AST Call node representing get_bar_data()
        
    Returns:
        Dictionary with extracted parameters or None if extraction fails
    """
    try:
        call_info = {
            'function': 'get_bar_data',
            'timeframe': '1d',  # default
            'min_bars': 1,      # default
            'line_number': getattr(call_node, 'lineno', 0)
        }
        
        # Extract positional arguments
        if len(call_node.args) >= 1:
            # First arg is timeframe
            timeframe = _extract_string_value(call_node.args[0])
            if timeframe:
                call_info['timeframe'] = timeframe
                
        if len(call_node.args) >= 4:
            # Fourth arg is min_bars (timeframe, security_ids, columns, min_bars)
            min_bars = _extract_int_value(call_node.args[3])
            if min_bars is not None:
                call_info['min_bars'] = min_bars
        
        # Extract keyword arguments
        for keyword in call_node.keywords:
            if keyword.arg == 'timeframe':
                timeframe = _extract_string_value(keyword.value)
                if timeframe:
                    call_info['timeframe'] = timeframe
            elif keyword.arg == 'min_bars':
                min_bars = _extract_int_value(keyword.value)
                if min_bars is not None:
                    call_info['min_bars'] = min_bars
        
        return call_info
        
    except Exception as e:
        logger.debug(f"Failed to extract parameters from get_bar_data call: {e}")
        return None

def _extract_string_value(node: ast.AST) -> Optional[str]:
    """Extract string value from AST node if possible."""
    try:
        if isinstance(node, ast.Constant) and isinstance(node.value, str):
            return node.value
        elif isinstance(node, ast.Str):  # Python < 3.8 compatibility
            return node.s
    except Exception as e:
        logger.debug(f"_extract_string_value: {e}")
    return None

def _extract_int_value(node: ast.AST) -> Optional[int]:
    """Extract integer value from AST node if possible."""
    try:
        if isinstance(node, ast.Constant) and isinstance(node.value, int):
            return node.value
        elif isinstance(node, ast.Num):  # Python < 3.8 compatibility
            if isinstance(node.n, int):
                return node.n
    except Exception as e:
        logger.debug(f"_extract_int_value: {e}")
    return None

def validate_strategy(code: str) -> bool:
    """
    Comprehensive code validation including:
    1. Syntax compilation check
    2. Security AST validation  
    3. DataFrame strategy compliance
    4. Function signature validation
    5. Return value structure validation
    """
    try:
        validate_code(code)
        testResult = executeStrategy(code,"validation")                 
        return testResult.get('success', False)

    except (SyntaxError, SecurityError, StrategyComplianceError) as e:
        logger.warning(f"Code failed validation: {e}")
        raise
    except Exception as e:
        logger.error(f"Unexpected error during validation: {e}")
        return False
def validate_code(code: str) -> bool:
    try: 
        if not code or not code.strip():
            raise PythonCodeError("Code cannot be empty")
        # Parse AST for validation (this also checks syntax)
        try:
            tree = ast.parse(code)
        except SyntaxError as e:
            logger.warning(f"Code compilation failed: {e}")
            raise StrategyComplianceError(f"Syntax error in strategy code: {e}")
        
        # Single AST walk for security and compliance
        _analyze_ast(tree, code)
        # Additional pattern checks 
        _check_prohibited_patterns(code)
        return True
    except (SecurityError, PythonCodeError, StrategyComplianceError) as e:
        logger.warning(f"Code failed validation: {e}")
        raise
    except Exception as e:
        logger.error(f"Unexpected error during validation: {e}")
        return False



def _analyze_ast(tree: ast.AST, code: str) -> None:
    """Single AST walk for security and compliance validation"""
    strategy_functions = []
    
    # Single pass through all nodes
    for node in ast.walk(tree):
        # Security checks for each node
        for node_type, checker in forbidden_nodes.items():
            if isinstance(node, node_type):
                if not checker(node):
                    raise SecurityError(f"Forbidden operation: {node_type.__name__}")
        
        # Collect function definitions for compliance checking
        if isinstance(node, ast.FunctionDef):
            if not node.name.startswith('_') and not node.name.startswith('__'):
                # Check for legacy patterns and collect strategy functions
                if node.name == 'classify_symbol':
                    raise StrategyComplianceError(
                        "Old pattern function 'classify_symbol' is no longer supported. "
                        "Use 'strategy()' with get_bar_data() and get_general_data() instead."
                    )
                
                if node.name.startswith('run_'):
                    raise StrategyComplianceError(
                        f"Function '{node.name}' uses old batch pattern. "
                        "Use 'strategy()' with accessor functions instead."
                    )
                
                # Only accept 'strategy' function name
                if node.name == 'strategy':
                    strategy_functions.append(node)
    
    # Strategy compliance checks
    if not strategy_functions:
        raise StrategyComplianceError("No 'strategy' function found. Must define a function named 'strategy'.")
        
    if len(strategy_functions) > 1:
        raise StrategyComplianceError("Only one 'strategy' function is allowed")
        
    func_node = strategy_functions[0]
    
    # Validate function signature and body
    _validate_function_signature(func_node)
    _validate_function_body(func_node)



def _validate_function_signature(func_node: ast.FunctionDef) -> bool:
    """Validate that function signature matches requirements: () -> List[Dict]"""
    
    # Check function has no parameters (new data accessor approach)
    if len(func_node.args.args) != 0:
        raise StrategyComplianceError(
            f"Strategy function must have no parameters (use get_bar_data() and get_general_data() instead), "
            f"found {len(func_node.args.args)} parameters"
        )
    
    return True

def _validate_function_body(func_node: ast.FunctionDef) -> bool:
    """Validate function body follows strategy requirements"""
    
    # Check for return statements
    return_nodes = []
    for node in ast.walk(func_node):
        if isinstance(node, ast.Return):
            return_nodes.append(node)
    
    if not return_nodes:
        raise StrategyComplianceError("Strategy function must have at least one return statement")
    
    # Validate return statements structure
    for return_node in return_nodes:
        if return_node.value is None:
            raise StrategyComplianceError("Strategy function cannot return None")
    
    return True

def _check_prohibited_patterns(code: str) -> bool:
    """Check for prohibited patterns in raw code"""
    
    # Only patterns that AST checks cannot detect (very minimal list)
    prohibited_patterns = [
        # String-based dynamic imports that could bypass AST module checking
        (r'__import__\s*\(\s*["\'][^"\']*(?:os|sys)', "Dynamic import of forbidden modules is forbidden"),
    ]
    
    lines = code.split('\n')
    in_docstring = False
    docstring_delimiter = None
    
    for line_num, line in enumerate(lines, 1):
        stripped = line.strip()
        
        # Track docstring state
        if '"""' in line:
            if not in_docstring:
                in_docstring = True
                docstring_delimiter = '"""'
            elif docstring_delimiter == '"""':
                in_docstring = False
                docstring_delimiter = None
        elif "'''" in line:
            if not in_docstring:
                in_docstring = True
                docstring_delimiter = "'''"
            elif docstring_delimiter == "'''":
                in_docstring = False
                docstring_delimiter = None
        
        # Skip comments and docstrings
        if stripped.startswith('#') or in_docstring:
            continue
            
        # Check each prohibited pattern
        for pattern, message in prohibited_patterns:
            if re.search(pattern, line, re.IGNORECASE):
                raise SecurityError(f"Line {line_num}: {message}")
    
    return True



def _check_import(node: ast.Import) -> bool:
    """Check import statements"""
    for alias in node.names:
        module_name = alias.name.split('.')[0]  # Get root module
        normalized_name = _normalize_module_name(module_name)
        if normalized_name in forbidden_modules:
            raise SecurityError(f"Import of forbidden module: {module_name}")
        # Only allow explicitly safe modules
        if normalized_name not in allowed_modules:
            raise SecurityError(f"Import of non-whitelisted module: {module_name}")
    return True

def _check_import_from(node: ast.ImportFrom) -> bool:
    """Check from-import statements"""
    if node.module:
        module_name = node.module.split('.')[0]  # Get root module
        normalized_name = _normalize_module_name(module_name)
        if normalized_name in forbidden_modules:
            raise SecurityError(f"Import from forbidden module: {module_name}")
        if normalized_name not in allowed_modules:
            raise SecurityError(f"Import from non-whitelisted module: {module_name}")
    return True

def _check_function_call(node: ast.Call) -> bool:
    """Check function calls for forbidden functions"""
    if isinstance(node.func, ast.Name):
        # Allow explicitly allowed functions
        if node.func.id in allowed_functions:
            return True
        # Block forbidden functions
        if node.func.id in forbidden_functions:
            raise SecurityError(f"Forbidden function call: {node.func.id}")
        # Allow pandas and numpy functions
        if node.func.id.startswith(('pd.', 'pandas.', 'np.', 'numpy.', 'math.')):
            return True
    elif isinstance(node.func, ast.Attribute):
        # Allow pandas DataFrame/Series methods
        allowed_pandas_methods = {
            'sort_values', 'groupby', 'rolling', 'ewm', 'shift', 'diff', 'pct_change',
            'mean', 'std', 'min', 'max', 'sum', 'count', 'reset_index', 'dropna',
            'notna', 'isna', 'fillna', 'copy', 'iterrows', 'transform', 'apply',
            'head', 'tail', 'describe', 'info', 'dtypes', 'shape', 'columns',
            'index', 'values', 'loc', 'iloc', 'at', 'iat', 'where', 'query'
        }
        # Allow numpy array methods
        allowed_array_methods = {
            'astype', 'shape', 'size', 'dtype', 'ndim', 'T', 'reshape', 
            'flatten', 'ravel', 'copy', 'mean', 'std', 'min', 'max', 'sum'
        }
        
        if (node.func.attr in allowed_pandas_methods or 
            node.func.attr in allowed_array_methods):
            return True
        # Check for dangerous method calls
        if node.func.attr in forbidden_functions:
            raise SecurityError(f"Forbidden method call: {node.func.attr}")
    return True

def _check_attribute_access(node: ast.Attribute) -> bool:
    """Check attribute access for dangerous attributes"""
    if node.attr in forbidden_attributes:
        raise SecurityError(f"Forbidden attribute access: {node.attr}")
    return True

def _check_function_definition(node: ast.FunctionDef) -> bool:
    """Check function definitions"""
    # Prevent overriding reserved names
    if node.name in reserved_global_names:
        raise SecurityError(f"Cannot override reserved name: {node.name}")
    
    # Prevent Python keywords
    if keyword.iskeyword(node.name):
        raise SecurityError(f"Cannot use Python keyword as function name: {node.name}")
    
    return True

'''def _check_async_function_definition(node: ast.AsyncFunctionDef) -> bool:
    """Check async function definitions (forbidden)"""
    raise SecurityError("Async function definitions are not allowed in strategies")

def _check_class_definition(node: ast.ClassDef) -> bool:
    """Check class definitions (forbidden)"""
    raise SecurityError("Class definitions are not allowed in strategies")

def _check_while_loop(node: ast.While) -> bool:
    """Check while loops (potentially dangerous)"""
    logger.warning("While loops detected - ensure they terminate to avoid infinite loops")
    return True

def _check_global_statement(node: ast.Global) -> bool:
    """Check global statements (forbidden)"""
    raise SecurityError("Global statements are not allowed in strategies")

def _check_nonlocal_statement(node: ast.Nonlocal) -> bool:
    """Check nonlocal statements (forbidden)"""
    raise SecurityError("Nonlocal statements are not allowed in strategies")
    '''


# Node type checkers - defined after functions to avoid NameError
forbidden_nodes = {
    ast.Import: _check_import,
    ast.ImportFrom: _check_import_from,
    ast.Call: _check_function_call,
    ast.Attribute: _check_attribute_access,
    ast.FunctionDef: _check_function_definition,
    ast.AsyncFunctionDef: _check_async_function_definition,
    ast.ClassDef: _check_class_definition,
    ast.While: _check_while_loop,
    ast.Global: _check_global_statement,
    ast.Nonlocal: _check_nonlocal_statement,
}

'''
class SecurityError(Exception):
    """Raised when code contains security violations"""
    pass


class StrategyComplianceError(Exception):
    """Raised when code doesn't comply with DataFrame strategy requirements"""
    pass

class PythonCodeError(Exception):
    """Raised when generic Python code validation fails"""
    pass
'''

    async def execute_validation(
        ctx: Context, 
        strategy_code: str
    ) -> Dict[str, Any]:
        """
        Execute strategy for VALIDATION ONLY using exact min_bars requirements for speed
        
        Args:
            strategy_code: Python code defining the strategy function  
            
        Returns:
            Dict with validation result (success/error only)
        """
        #logger.info("🧪 Starting fast validation execution (exact min_bars requirements)")
        
        start_time = time.time()
        
        try:
            #TODO just use one ticker as the validator should just pass anything to validate functionality
            getBarDataFunctionCalls = extract_get_bar_data_calls(strategy_code)
            tickersInStrategyCode = get_all_tickers_from_calls(getBarDataFunctionCalls)

            symbolsForValidation = tickersInStrategyCode if len(tickersInStrategyCode) <= 2 else tickersInStrategyCode[:2]
            # Fail validation if no tickers were extracted (no default fallback)
            if not symbolsForValidation:
                raise ValueError("Validation failed: no tickers extracted from strategy code; cannot validate without ticker filters")
            
            max_timeframe, max_timeframe_min_bars = getMaxTimeframeAndMinBars(getBarDataFunctionCalls)
            
            # Calculate start date based on timeframe and min_bars (convert to days and round up)
            #TODO this cannot use date ranges as this is not equivvalent to min bars
            # because stock weekends, holidays, etc.
            # instead should just use bars
            end_date = dt.now()
            if max_timeframe and max_timeframe_min_bars > 0:
                # Parse timeframe and convert to days
                if max_timeframe.endswith('d'):
                    days_back = int(max_timeframe[:-1]) * max_timeframe_min_bars
                elif max_timeframe.endswith('h'):
                    hours_back = int(max_timeframe[:-1]) * max_timeframe_min_bars
                    days_back = math.ceil(hours_back / 24)  # Round up to nearest day
                elif max_timeframe.endswith('m'):
                    minutes_back = int(max_timeframe[:-1]) * max_timeframe_min_bars
                    days_back = math.ceil(minutes_back / (24 * 60))  # Round up to nearest day
                else:
                    days_back = 30  # Default fallback
                start_date = end_date - timedelta(days=days_back)
            else:
                start_date = end_date - timedelta(days=30)  # Default fallback
            
            # Set execution context for validation with exact requirements
            context_data = {
                'mode': 'validation',  # Special validation mode
                'symbols': symbolsForValidation,   # Use extracted tickers for more realistic validation
                'start_date': start_date,
                'end_date': end_date
            }
            
            set_execution_context(**context_data)
            
            validationMaxInstances = 100
            # Execute strategy with validation context (don't care about results)
            instances, _, _, _, error = await _execute_strategy(
                strategy_code, 
                execution_mode='validation',
                max_instances=validationMaxInstances # Lower limit for validation
            )
            if error: 
                raise error
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'validation',
                'instances_generated': len(instances),
                'instance_limit_reached': TrackedList.is_limit_reached(),
                'max_instances_configured': validationMaxInstances,  # Validation uses lower limit
                'execution_time_ms': int(execution_time),
                'message': 'Validation passed - strategy can execute without errors'
            }
            
            #logger.info(f"✅ Validation completed successfully: {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            execution_time = (time.time() - start_time) * 1000
            
            # Get detailed error information
            error_info = _get_detailed_error_info(e, strategy_code)
            detailed_error_msg = _format_detailed_error(error_info)
            
            logger.error(f"❌ Validation failed: {e}")
            logger.error(detailed_error_msg)
            
            return {
                'success': False,
                'error': str(e),
                'error_details': error_info,
                'execution_mode': 'validation',
                'strategy_prints': '',
                'execution_time_ms': int(execution_time)
            }