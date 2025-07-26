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


class SecurityValidator:
    """Validates Python code for security issues and DataFrame strategy compliance"""

    def __init__(self, conn):
        self.conn = conn
        # Node type checkers
        self.forbidden_nodes = {
            ast.Import: self._check_import,
            ast.ImportFrom: self._check_import_from,
            ast.Call: self._check_function_call,
            ast.Attribute: self._check_attribute_access,
            ast.FunctionDef: self._check_function_definition,
            ast.AsyncFunctionDef: self._check_async_function_definition,
            ast.ClassDef: self._check_class_definition,
            ast.While: self._check_while_loop,
            ast.Global: self._check_global_statement,
            ast.Nonlocal: self._check_nonlocal_statement,
        }

        # Forbidden built-in functions (only truly dangerous ones)
        self.forbidden_functions = {
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
        self.allowed_functions = {
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
            # zip and enumerate
            "zip", "enumerate",
            # Data accessor functions
            "get_bar_data", "get_general_data"
        }

        # Forbidden modules (exhaustive security list)
        self.forbidden_modules = {
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
        self.allowed_modules = {
            "pandas", "pd", "numpy", "np", "math", "statistics", "random",
            "datetime", "time", "decimal", "fractions", "collections",
            "itertools", "functools", "operator", "copy", "json", "re",
            "string", "textwrap", "calendar", "bisect", "heapq", "array",
            "typing", "plotly", "plotly.subplots", "subplots", "make_subplots", "px", "graph_objects", "express" # For type annotations like List[Dict]
        }

        # Forbidden attributes (comprehensive dunder and internal attributes)
        self.forbidden_attributes = {
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
        self.required_instance_fields = {"ticker", "timestamp"}
        self.reserved_global_names = {"pd", "pandas", "np", "numpy", "datetime", "timedelta", "math", 
                                     "get_bar_data", "get_general_data"}
        
        # Data accessor function names
        self.data_accessor_functions = {"get_bar_data", "get_general_data"}

    def extract_min_bars_requirements(self, code: str) -> List[Dict[str, Any]]:
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
                        call_info = self._extract_get_bar_data_params(node)
                        if call_info:
                            requirements.append(call_info)
                            
        except SyntaxError as e:
            logger.warning(f"Failed to parse strategy code for min_bars extraction: {e}")
        except Exception as e:
            logger.error(f"Unexpected error extracting min_bars requirements: {e}")
            
        return requirements

    def _extract_get_bar_data_params(self, call_node: ast.Call) -> Optional[Dict[str, Any]]:
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
                timeframe = self._extract_string_value(call_node.args[0])
                if timeframe:
                    call_info['timeframe'] = timeframe
                    
            if len(call_node.args) >= 3:
                # Third arg is min_bars (second is columns)
                min_bars = self._extract_int_value(call_node.args[2])
                if min_bars is not None:
                    call_info['min_bars'] = min_bars
            
            # Extract keyword arguments
            for keyword in call_node.keywords:
                if keyword.arg == 'timeframe':
                    timeframe = self._extract_string_value(keyword.value)
                    if timeframe:
                        call_info['timeframe'] = timeframe
                elif keyword.arg == 'min_bars':
                    min_bars = self._extract_int_value(keyword.value)
                    if min_bars is not None:
                        call_info['min_bars'] = min_bars
            
            return call_info
            
        except Exception as e:
            logger.debug(f"Failed to extract parameters from get_bar_data call: {e}")
            return None

    def _extract_string_value(self, node: ast.AST) -> Optional[str]:
        """Extract string value from AST node if possible."""
        try:
            if isinstance(node, ast.Constant) and isinstance(node.value, str):
                return node.value
            elif isinstance(node, ast.Str):  # Python < 3.8 compatibility
                return node.s
        except Exception as e:
            logger.debug(f"_extract_string_value: {e}")
        return None

    def _extract_int_value(self, node: ast.AST) -> Optional[int]:
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

    def validate_strategy_code(self, code: str) -> bool:
        """
        Comprehensive code validation including:
        1. Syntax compilation check
        2. Security AST validation  
        3. DataFrame strategy compliance
        4. Function signature validation
        5. Return value structure validation
        """
        try:
            self.validate_code(code)
            tree = ast.parse(code)
            # Step 1: Strategy compliance checks
            if not self._check_strategy_compliance(tree, code):
                return False
                
            # Step 2: Validate strategy function structure
            if not self._validate_strategy_structure(tree):
                return False
                
            #logger.info("Code passed all validation checks")
            return True
            
        except (SyntaxError, SecurityError, StrategyComplianceError) as e:
            logger.warning(f"Code failed validation: {e}")
            raise
        except Exception as e:
            logger.error(f"Unexpected error during validation: {e}")
            return False
    def validate_code(self, code: str) -> bool:
        try: 
            if not code or not code.strip():
                raise PythonCodeError("Code cannot be empty")
            if not self._check_compilation(code):
                return False 
            # Step 3: Parse AST for validation
            tree = ast.parse(code)
            
            # Step 4: Security checks
            if not self._check_ast_security(tree):
                return False
            # Step 5: Additional pattern checks 
            self._check_prohibited_patterns(code)
            return True
        except (SyntaxError, SecurityError, PythonCodeError) as e:
            logger.warning(f"Code failed validation: {e}")
            raise
        except Exception as e:
            logger.error(f"Unexpected error during validation: {e}")
            return False

    def _check_compilation(self, code: str) -> bool:
        """Check if the code compiles without syntax errors"""
        try:
            compile(code, "<strategy>", "exec")
            return True
        except SyntaxError as e:
            logger.warning(f"Code compilation failed: {e}")
            raise StrategyComplianceError(f"Syntax error in strategy code: {e}")

    def _check_ast_security(self, tree: ast.AST) -> bool:
        """Perform comprehensive AST security validation"""
        try:
            self._check_ast_node(tree)
            return True
        except SecurityError:
            return False

    def _check_strategy_compliance(self, tree: ast.AST, code: str) -> bool:
        """Check DataFrame strategy compliance requirements"""
        
        # Find all functions
        all_functions = self._find_strategy_functions(tree)
        if not all_functions:
            raise StrategyComplianceError("No valid strategy function found. Must define at least one non-private function.")
        
        # Look for the main strategy function specifically
        strategy_functions = [func for func in all_functions if func.name == 'strategy']
        
        if not strategy_functions:
            raise StrategyComplianceError("No 'strategy' function found. Must define a function named 'strategy'.")
            
        if len(strategy_functions) > 1:
            raise StrategyComplianceError("Only one 'strategy' function is allowed")
            
        func_node = strategy_functions[0]
        
        # Validate function signature
        if not self._validate_function_signature(func_node):
            return False
            
        # Validate function body for instance requirements
        if not self._validate_function_body(func_node):
            return False
            
        return True

    def _validate_strategy_structure(self, tree: ast.AST) -> bool:
        """Validate overall strategy structure meets requirements"""
        
        functions = self._find_strategy_functions(tree)
        
        if not functions:
            raise StrategyComplianceError("No valid strategy functions found. Function must be named 'strategy'")
        
        strategy_functions = []
        for func in functions:
            # EXPLICIT REJECTION of old patterns
            if func.name == 'classify_symbol':
                raise StrategyComplianceError(
                    "Old pattern function 'classify_symbol' is no longer supported. "
                    "Use 'strategy()' with get_bar_data() and get_general_data() instead."
                )
            
            if func.name.startswith('run_'):
                raise StrategyComplianceError(
                    f"Function '{func.name}' uses old batch pattern. "
                    "Use 'strategy()' with accessor functions instead."
                )
            
            # ONLY ACCEPT 'strategy' function name
            if func.name == 'strategy':
                strategy_functions.append(func)
        
        if not strategy_functions:
            raise StrategyComplianceError(
                "Strategy function must be named 'strategy'. "
                "Old patterns like 'classify_symbol' are no longer supported."
            )
        
        # Validate each strategy function
        for func in strategy_functions:
            self._validate_function_signature(func)
            self._validate_function_body(func)
        
        return True

    def _find_strategy_functions(self, tree: ast.AST) -> List[ast.FunctionDef]:
        """Find valid strategy functions in the code"""
        functions = []
        for node in ast.walk(tree):
            if isinstance(node, ast.FunctionDef):
                # Skip private functions and special methods
                if not node.name.startswith('_') and not node.name.startswith('__'):
                    functions.append(node)
        return functions

    def _validate_function_signature(self, func_node: ast.FunctionDef) -> bool:
        """Validate that function signature matches requirements: () -> List[Dict]"""
        
        # Check function has no parameters (new data accessor approach)
        if len(func_node.args.args) != 0:
            raise StrategyComplianceError(
                f"Strategy function must have no parameters (use get_bar_data() and get_general_data() instead), "
                f"found {len(func_node.args.args)} parameters"
            )
        
        return True

    def _validate_function_body(self, func_node: ast.FunctionDef) -> bool:
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

    def _check_prohibited_patterns(self, code: str) -> bool:
        """Check for prohibited patterns in raw code"""
        
        prohibited_patterns = [
            # Dunder methods and attributes
            (r'__\w+__', "Double underscore methods/attributes are forbidden"),
            # Function internals
            (r'\.func_\w+', "Function internal attributes are forbidden"),
            (r'\.im_\w+', "Method internal attributes are forbidden"),
            (r'\.gi_\w+', "Generator internal attributes are forbidden"),
            (r'\.cr_\w+', "Coroutine internal attributes are forbidden"),
            # Dangerous function calls
            (r'globals\s*\(', "globals() function is forbidden"),
            (r'locals\s*\(', "locals() function is forbidden"),
            (r'vars\s*\(', "vars() function is forbidden"),
            (r'dir\s*\(', "dir() function is forbidden"),
            (r'exec\s*\(', "exec() function is forbidden"),
            (r'eval\s*\(', "eval() function is forbidden"),
            (r'compile\s*\(', "compile() function is forbidden"),
            (r'__import__\s*\(', "__import__() function is forbidden"),
            # File operations
            (r'open\s*\(', "open() function is forbidden"),
            (r'file\s*\(', "file() function is forbidden"),
            # Input/Output
            (r'input\s*\(', "input() function is forbidden"),
            # System access patterns
            (r'import\s+os\b', "Importing os module is forbidden"),
            (r'import\s+sys\b', "Importing sys module is forbidden"),
            (r'from\s+os\s+import', "Importing from os module is forbidden"),
            (r'from\s+sys\s+import', "Importing from sys module is forbidden"),
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

    def _check_ast_node(self, node: ast.AST):
        """Recursively check AST nodes for security violations"""
        
        # Check if this node type has a specific checker
        for node_type, checker in self.forbidden_nodes.items():
            if isinstance(node, node_type):
                if not checker(node):
                    raise SecurityError(f"Forbidden operation: {node_type.__name__}")

        # Continue checking child nodes
        for child in ast.iter_child_nodes(node):
            self._check_ast_node(child)

    def _check_import(self, node: ast.Import) -> bool:
        """Check import statements"""
        for alias in node.names:
            module_name = alias.name.split('.')[0]  # Get root module
            if module_name in self.forbidden_modules:
                raise SecurityError(f"Import of forbidden module: {module_name}")
            # Only allow explicitly safe modules
            if module_name not in self.allowed_modules:
                raise SecurityError(f"Import of non-whitelisted module: {module_name}")
        return True

    def _check_import_from(self, node: ast.ImportFrom) -> bool:
        """Check from-import statements"""
        if node.module:
            module_name = node.module.split('.')[0]  # Get root module
            if module_name in self.forbidden_modules:
                raise SecurityError(f"Import from forbidden module: {module_name}")
            if module_name not in self.allowed_modules:
                raise SecurityError(f"Import from non-whitelisted module: {module_name}")
        return True

    def _check_function_call(self, node: ast.Call) -> bool:
        """Check function calls for forbidden functions"""
        if isinstance(node.func, ast.Name):
            # Allow explicitly allowed functions
            if node.func.id in self.allowed_functions:
                return True
            # Block forbidden functions
            if node.func.id in self.forbidden_functions:
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
            if node.func.attr in self.forbidden_functions:
                raise SecurityError(f"Forbidden method call: {node.func.attr}")
        return True

    def _check_attribute_access(self, node: ast.Attribute) -> bool:
        """Check attribute access for dangerous attributes"""
        if node.attr in self.forbidden_attributes:
            raise SecurityError(f"Forbidden attribute access: {node.attr}")
        return True

    def _check_function_definition(self, node: ast.FunctionDef) -> bool:
        """Check function definitions"""
        # Prevent overriding reserved names
        if node.name in self.reserved_global_names:
            raise SecurityError(f"Cannot override reserved name: {node.name}")
        
        # Prevent Python keywords
        if keyword.iskeyword(node.name):
            raise SecurityError(f"Cannot use Python keyword as function name: {node.name}")
        
        return True

    def _check_async_function_definition(self, node: ast.AsyncFunctionDef) -> bool:
        """Check async function definitions (forbidden)"""
        raise SecurityError("Async function definitions are not allowed in strategies")

    def _check_class_definition(self, node: ast.ClassDef) -> bool:
        """Check class definitions (forbidden)"""
        raise SecurityError("Class definitions are not allowed in strategies")

    def _check_while_loop(self, node: ast.While) -> bool:
        """Check while loops (potentially dangerous)"""
        logger.warning("While loops detected - ensure they terminate to avoid infinite loops")
        return True

    def _check_global_statement(self, node: ast.Global) -> bool:
        """Check global statements (forbidden)"""
        raise SecurityError("Global statements are not allowed in strategies")

    def _check_nonlocal_statement(self, node: ast.Nonlocal) -> bool:
        """Check nonlocal statements (forbidden)"""
        raise SecurityError("Nonlocal statements are not allowed in strategies")


class SecurityError(Exception):
    """Raised when code contains security violations"""
    pass


class StrategyComplianceError(Exception):
    """Raised when code doesn't comply with DataFrame strategy requirements"""
    pass

class PythonCodeError(Exception):
    """Raised when generic Python code validation fails"""
    pass