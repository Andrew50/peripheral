"""
Security Validator
Validates Python code for security issues before execution and ensures compliance with NumPy DataFrame strategy requirements.

The strategy engine expects functions that:
1. Accept a single parameter: data (numpy.ndarray)
2. Return a list of dictionaries with 'ticker' and 'date' fields
3. Use only approved safe modules (numpy, math, datetime, etc.)
4. Do not access dangerous system functions or attributes

The data provided will be a structured numpy array with fields including:
- ticker (string): Stock ticker symbol
- date (datetime/string): Date of the data point
- open, high, low, close (float): OHLC price data
- volume (int): Trading volume
- And potentially other market data fields
"""

import ast
import logging
import re
import keyword
from typing import Set, List, Dict, Any, Optional, Union
import sys

logger = logging.getLogger(__name__)


class SecurityValidator:
    """Validates Python code for security issues and NumPy DataFrame strategy compliance"""

    def __init__(self):
        # Node type checkers
        self.forbidden_nodes = {
            ast.Import: self._check_import,
            ast.ImportFrom: self._check_import_from,
            ast.Call: self._check_function_call,
            ast.Attribute: self._check_attribute_access,
            ast.FunctionDef: self._check_function_definition,
            ast.AsyncFunctionDef: self._check_async_function_definition,
            ast.ClassDef: self._check_class_definition,
            ast.For: self._check_for_loop,
            ast.While: self._check_while_loop,
            ast.With: self._check_with_statement,
            ast.Try: self._check_try_statement,
            ast.Raise: self._check_raise_statement,
            ast.Delete: self._check_delete_statement,
            ast.Global: self._check_global_statement,
            ast.Nonlocal: self._check_nonlocal_statement,
            # ast.Exec removed - doesn't exist in Python 3.x
            ast.Lambda: self._check_lambda,
            ast.ListComp: self._check_comprehension,
            ast.DictComp: self._check_comprehension,
            ast.SetComp: self._check_comprehension,
            ast.GeneratorExp: self._check_comprehension,
        }

        # Forbidden built-in functions (comprehensive list)
        self.forbidden_functions = {
            # Code execution
            "exec", "eval", "compile", "__import__", "breakpoint",
            # File and system access
            "open", "file", "input", "raw_input", "print",
            # Introspection and manipulation
            "globals", "locals", "vars", "dir", "delattr", "setattr", "hasattr", "getattr",
            # System control
            "exit", "quit", "help", "copyright", "credits", "license",
            # Memory and object manipulation
            "memoryview", "bytearray", "bytes", "callable", "classmethod", "staticmethod",
            "property", "super", "type", "isinstance", "issubclass", "iter", "next",
            # Dangerous built-ins
            "id", "hash", "repr", "ascii", "bin", "hex", "oct", "format",
            "sorted", "reversed", "enumerate", "zip", "map", "filter", "reduce",
            "any", "all", "sum", "min", "max", "pow", "round", "divmod",
            # Type constructors that could be dangerous
            "complex", "frozenset", "slice", "range", "object",
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

        # Allowed safe modules for data science (strict whitelist)
        self.allowed_modules = {
            "numpy", "np", "math", "statistics", "random",
            "datetime", "time", "decimal", "fractions", "collections",
            "itertools", "functools", "operator", "copy", "json", "re",
            "string", "textwrap", "calendar", "bisect", "heapq", "array",
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

        # Strategy requirements
        self.required_instance_fields = {"ticker", "date"}
        self.reserved_global_names = {"np", "numpy", "data", "datetime", "timedelta", "math"}
        
        # Data frame fields that will always be available
        self.available_data_fields = {
            "ticker", "date", "open", "high", "low", "close", "volume",
            "timestamp", "datetime", "time", "adj_close", "split_factor", "dividend"
        }

    def validate_code(self, code: str) -> bool:
        """
        Comprehensive code validation including:
        1. Syntax compilation check
        2. Security AST validation  
        3. NumPy DataFrame strategy compliance
        4. Function signature validation
        5. Return value structure validation
        """
        try:
            # Step 1: Basic validation
            if not code or not code.strip():
                raise StrategyComplianceError("Strategy code cannot be empty")

            # Step 2: Check if code compiles
            if not self._check_compilation(code):
                return False

            # Step 3: Parse AST for validation
            tree = ast.parse(code)
            
            # Step 4: Security checks
            if not self._check_ast_security(tree):
                return False
            
            # Step 5: Strategy compliance checks
            if not self._check_strategy_compliance(tree, code):
                return False
                
            # Step 6: Additional pattern checks
            if not self._check_prohibited_patterns(code):
                return False
                
            # Step 7: Validate strategy function structure
            if not self._validate_strategy_structure(tree):
                return False
                
            logger.info("Code passed all validation checks")
            return True
            
        except (SyntaxError, SecurityError, StrategyComplianceError) as e:
            logger.warning(f"Code failed validation: {e}")
            return False
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
        """Check NumPy DataFrame strategy compliance requirements"""
        
        # Check for required strategy function
        strategy_functions = self._find_strategy_functions(tree)
        if not strategy_functions:
            raise StrategyComplianceError("No valid strategy function found. Must define exactly one non-private function.")
        
        if len(strategy_functions) > 1:
            raise StrategyComplianceError("Only one strategy function is allowed")
            
        func_node = strategy_functions[0]
        
        # Validate function signature
        if not self._validate_function_signature(func_node):
            return False
            
        # Validate function body for instance requirements
        if not self._validate_function_body(func_node):
            return False
            
        return True

    def _validate_strategy_structure(self, tree: ast.AST) -> bool:
        """Validate overall strategy structure and requirements"""
        
        # Check for numpy import
        has_numpy_import = False
        for node in ast.walk(tree):
            if isinstance(node, (ast.Import, ast.ImportFrom)):
                if self._is_numpy_import(node):
                    has_numpy_import = True
                    break
        
        if not has_numpy_import:
            raise StrategyComplianceError("Strategy must import numpy (import numpy as np)")
        
        return True

    def _is_numpy_import(self, node: Union[ast.Import, ast.ImportFrom]) -> bool:
        """Check if this is a numpy import"""
        if isinstance(node, ast.Import):
            for alias in node.names:
                if alias.name == "numpy":
                    return True
        elif isinstance(node, ast.ImportFrom):
            if node.module and node.module.startswith("numpy"):
                return True
        return False

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
        """Validate that function signature matches requirements: (data: np.ndarray) -> List[Dict]"""
        
        # Check function has exactly one parameter
        if len(func_node.args.args) != 1:
            raise StrategyComplianceError(
                f"Strategy function must have exactly one parameter (data: numpy.ndarray), "
                f"found {len(func_node.args.args)} parameters"
            )
        
        param = func_node.args.args[0]
        
        # Parameter name should be 'data' (enforcing standard)
        if param.arg != 'data':
            raise StrategyComplianceError(f"Strategy function parameter must be named 'data', found '{param.arg}'")
        
        # Check for type annotation (now required)
        if not param.annotation:
            raise StrategyComplianceError("Strategy function parameter must be type annotated as numpy.ndarray")
        
        # Extract annotation text to check for numpy array type
        annotation_text = self._get_annotation_text(param.annotation)
        if not self._is_numpy_array_annotation(annotation_text):
            raise StrategyComplianceError(
                "Strategy function parameter must be annotated as numpy.ndarray or np.ndarray"
            )
        
        return True

    def _get_annotation_text(self, annotation: ast.AST) -> str:
        """Extract annotation text safely"""
        try:
            if hasattr(ast, 'unparse'):
                return ast.unparse(annotation)
            else:
                # Fallback for older Python versions
                if isinstance(annotation, ast.Attribute):
                    if isinstance(annotation.value, ast.Name):
                        return f"{annotation.value.id}.{annotation.attr}"
                elif isinstance(annotation, ast.Name):
                    return annotation.id
                return str(annotation)
        except:
            return ""

    def _is_numpy_array_annotation(self, annotation_text: str) -> bool:
        """Check if annotation represents a numpy array"""
        valid_annotations = [
            "numpy.ndarray", "np.ndarray", "ndarray",
            "numpy.array", "np.array"
        ]
        return any(valid in annotation_text for valid in valid_annotations)

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
            (r'print\s*\(', "print() function is forbidden"),
            # System access patterns
            (r'import\s+os\b', "Importing os module is forbidden"),
            (r'import\s+sys\b', "Importing sys module is forbidden"),
            (r'from\s+os\s+import', "Importing from os module is forbidden"),
            (r'from\s+sys\s+import', "Importing from sys module is forbidden"),
        ]
        
        lines = code.split('\n')
        for line_num, line in enumerate(lines, 1):
            stripped = line.strip()
            
            # Skip comments
            if stripped.startswith('#'):
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
            if node.func.id in self.forbidden_functions:
                raise SecurityError(f"Forbidden function call: {node.func.id}")
        elif isinstance(node.func, ast.Attribute):
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

    def _check_for_loop(self, node: ast.For) -> bool:
        """Check for loops (allowed but monitored)"""
        return True

    def _check_while_loop(self, node: ast.While) -> bool:
        """Check while loops (potentially dangerous)"""
        logger.warning("While loops detected - ensure they terminate to avoid infinite loops")
        return True

    def _check_with_statement(self, node: ast.With) -> bool:
        """Check with statements (context managers)"""
        return True

    def _check_try_statement(self, node: ast.Try) -> bool:
        """Check try statements"""
        return True

    def _check_raise_statement(self, node: ast.Raise) -> bool:
        """Check raise statements"""
        return True

    def _check_delete_statement(self, node: ast.Delete) -> bool:
        """Check delete statements (potentially dangerous)"""
        logger.warning("Delete statements detected - use with caution")
        return True

    def _check_global_statement(self, node: ast.Global) -> bool:
        """Check global statements (forbidden)"""
        raise SecurityError("Global statements are not allowed in strategies")

    def _check_nonlocal_statement(self, node: ast.Nonlocal) -> bool:
        """Check nonlocal statements (forbidden)"""
        raise SecurityError("Nonlocal statements are not allowed in strategies")

    def _check_exec_statement(self, node) -> bool:
        """Check exec statements (forbidden) - not applicable in Python 3.x"""
        # ast.Exec doesn't exist in Python 3.x, so this is not needed
        return True

    def _check_lambda(self, node: ast.Lambda) -> bool:
        """Check lambda expressions (allowed but monitored)"""
        return True

    def _check_comprehension(self, node: Union[ast.ListComp, ast.DictComp, ast.SetComp, ast.GeneratorExp]) -> bool:
        """Check comprehensions (allowed but monitored)"""
        return True

    def validate_instance_fields(self, instances: List[Dict[str, Any]]) -> bool:
        """
        Validate that all instances have required fields.
        
        Required fields:
        - ticker (str): Stock ticker symbol
        - date (str/int/float): Date identifier
        
        The engine guarantees that the input data array will contain these fields
        and additional market data fields like open, high, low, close, volume.
        """
        if not instances:
            return True
            
        if not isinstance(instances, list):
            raise StrategyComplianceError("Strategy must return a list of dictionaries")
            
        for i, instance in enumerate(instances):
            if not isinstance(instance, dict):
                raise StrategyComplianceError(f"Instance {i} must be a dictionary, got {type(instance)}")
            
            # Check required fields
            missing_fields = self.required_instance_fields - set(instance.keys())
            if missing_fields:
                raise StrategyComplianceError(
                    f"Instance {i} missing required fields: {missing_fields}. "
                    f"Required fields are: {self.required_instance_fields}"
                )
            
            # Validate field types
            if 'ticker' in instance:
                if not isinstance(instance['ticker'], str):
                    raise StrategyComplianceError(
                        f"Instance {i} 'ticker' field must be a string, got {type(instance['ticker'])}"
                    )
                if not instance['ticker'].strip():
                    raise StrategyComplianceError(f"Instance {i} 'ticker' field cannot be empty")
                    
            if 'date' in instance:
                if not isinstance(instance['date'], (str, int, float)):
                    raise StrategyComplianceError(
                        f"Instance {i} 'date' field must be a string or number, got {type(instance['date'])}"
                    )
        
        return True

    def get_data_field_documentation(self) -> str:
        """Return documentation about available data fields"""
        return f"""
        Available data fields in the numpy array:
        {', '.join(sorted(self.available_data_fields))}
        
        Required instance fields:
        {', '.join(sorted(self.required_instance_fields))}
        
        The data parameter is a structured numpy array containing market data.
        Access fields using: data['field_name'] (e.g., data['close'], data['volume'])
        """


class SecurityError(Exception):
    """Raised when code contains security violations"""
    pass


class StrategyComplianceError(Exception):
    """Raised when code doesn't comply with NumPy DataFrame strategy requirements"""
    pass
