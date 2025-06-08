"""
Security Validator
Validates Python code for security issues before execution
"""

import ast
import logging
import re
from typing import List, Set

logger = logging.getLogger(__name__)


class SecurityValidator:
    """Validates Python code for security vulnerabilities"""
    
    def __init__(self):
        # Forbidden modules that could be used for system access
        self.forbidden_modules = {
            'os', 'sys', 'subprocess', 'socket', 'urllib', 'urllib2', 'urllib3',
            'requests', 'http', 'httplib', 'httplib2', 'ftplib', 'smtplib',
            'telnetlib', 'pickle', 'cPickle', 'marshal', 'shelve', 'dbm',
            'sqlite3', 'threading', 'multiprocessing', '_thread', 'thread',
            'ctypes', 'mmap', 'tempfile', 'shutil', 'glob', 'platform',
            'webbrowser', 'imp', 'importlib', 'pkgutil', 'zipimport',
            'code', 'codeop', 'runpy', 'pty', 'tty'
        }
        
        # Forbidden built-in functions
        self.forbidden_builtins = {
            'exec', 'eval', 'compile', '__import__', 'open', 'file',
            'input', 'raw_input', 'globals', 'locals', 'vars', 'dir',
            'help', 'copyright', 'credits', 'license', 'quit', 'exit',
            'reload', 'execfile', 'apply', 'buffer', 'coerce', 'intern'
        }
        
        # Forbidden attributes that could be used for introspection
        self.forbidden_attributes = {
            '__globals__', '__locals__', '__code__', '__dict__', '__class__',
            '__bases__', '__mro__', '__subclasses__', '__import__',
            'func_globals', 'func_code', 'gi_code', 'gi_frame'
        }
        
        # Patterns that should not appear in code
        self.forbidden_patterns = [
            r'__.*__',  # Double underscore methods (with exceptions)
            r'exec\s*\(',  # exec function calls
            r'eval\s*\(',  # eval function calls
            r'compile\s*\(',  # compile function calls
            r'\.system\s*\(',  # os.system calls
            r'\.popen\s*\(',  # popen calls
            r'\.call\s*\(',  # subprocess.call
            r'\.run\s*\(',  # subprocess.run
            r'\.Popen\s*\(',  # subprocess.Popen
        ]
        
        # Allowed double underscore methods
        self.allowed_dunder_methods = {
            '__init__', '__str__', '__repr__', '__len__', '__getitem__',
            '__setitem__', '__delitem__', '__iter__', '__next__', '__add__',
            '__sub__', '__mul__', '__div__', '__truediv__', '__floordiv__',
            '__mod__', '__pow__', '__and__', '__or__', '__xor__', '__eq__',
            '__ne__', '__lt__', '__le__', '__gt__', '__ge__', '__contains__'
        }
    
    def validate_code(self, code: str) -> bool:
        """Validate Python code for security issues"""
        try:
            # Check for forbidden patterns
            if not self._check_patterns(code):
                logger.warning("Code contains forbidden patterns")
                return False
            
            # Parse and check AST
            tree = ast.parse(code)
            if not self._check_ast(tree):
                logger.warning("Code contains forbidden AST nodes")
                return False
            
            # Additional checks
            if not self._check_complexity(tree):
                logger.warning("Code is too complex")
                return False
            
            return True
            
        except SyntaxError as e:
            logger.error(f"Syntax error in code: {e}")
            return False
        except Exception as e:
            logger.error(f"Error validating code: {e}")
            return False
    
    def _check_patterns(self, code: str) -> bool:
        """Check for forbidden regex patterns"""
        for pattern in self.forbidden_patterns:
            if re.search(pattern, code):
                # Special case: allow specific dunder methods
                if pattern == r'__.*__':
                    matches = re.findall(r'__\w+__', code)
                    for match in matches:
                        if match not in self.allowed_dunder_methods:
                            logger.warning(f"Forbidden dunder method: {match}")
                            return False
                else:
                    logger.warning(f"Forbidden pattern found: {pattern}")
                    return False
        return True
    
    def _check_ast(self, tree: ast.AST) -> bool:
        """Check AST for forbidden constructs"""
        for node in ast.walk(tree):
            if not self._check_node(node):
                return False
        return True
    
    def _check_node(self, node: ast.AST) -> bool:
        """Check individual AST node"""
        
        # Check imports
        if isinstance(node, (ast.Import, ast.ImportFrom)):
            if not self._check_import_node(node):
                return False
        
        # Check function calls
        elif isinstance(node, ast.Call):
            if not self._check_call_node(node):
                return False
        
        # Check attribute access
        elif isinstance(node, ast.Attribute):
            if not self._check_attribute_node(node):
                return False
        
        # Check name access
        elif isinstance(node, ast.Name):
            if not self._check_name_node(node):
                return False
        
        return True
    
    def _check_import_node(self, node: ast.AST) -> bool:
        """Check import statements"""
        if isinstance(node, ast.Import):
            for alias in node.names:
                if alias.name in self.forbidden_modules:
                    logger.warning(f"Forbidden module import: {alias.name}")
                    return False
                    
        elif isinstance(node, ast.ImportFrom):
            if node.module in self.forbidden_modules:
                logger.warning(f"Forbidden module import: {node.module}")
                return False
                
            # Check for dangerous from imports
            for alias in node.names:
                if alias.name in self.forbidden_builtins:
                    logger.warning(f"Forbidden builtin import: {alias.name}")
                    return False
        
        return True
    
    def _check_call_node(self, node: ast.Call) -> bool:
        """Check function calls"""
        # Check direct function calls
        if isinstance(node.func, ast.Name):
            if node.func.id in self.forbidden_builtins:
                logger.warning(f"Forbidden function call: {node.func.id}")
                return False
        
        # Check method calls
        elif isinstance(node.func, ast.Attribute):
            # Check for dangerous method names
            dangerous_methods = {
                'system', 'popen', 'call', 'run', 'Popen', 'check_output',
                'check_call', 'getstatusoutput', 'getoutput'
            }
            if node.func.attr in dangerous_methods:
                logger.warning(f"Forbidden method call: {node.func.attr}")
                return False
        
        return True
    
    def _check_attribute_node(self, node: ast.Attribute) -> bool:
        """Check attribute access"""
        if node.attr in self.forbidden_attributes:
            logger.warning(f"Forbidden attribute access: {node.attr}")
            return False
        return True
    
    def _check_name_node(self, node: ast.Name) -> bool:
        """Check name references"""
        if node.id in self.forbidden_builtins:
            # Only forbid if it's being loaded (not stored)
            if isinstance(node.ctx, ast.Load):
                logger.warning(f"Forbidden name reference: {node.id}")
                return False
        return True
    
    def _check_complexity(self, tree: ast.AST) -> bool:
        """Check code complexity limits"""
        
        class ComplexityChecker(ast.NodeVisitor):
            def __init__(self):
                self.depth = 0
                self.max_depth = 0
                self.node_count = 0
                self.loop_count = 0
                
            def visit(self, node):
                self.node_count += 1
                self.depth += 1
                self.max_depth = max(self.max_depth, self.depth)
                
                # Count loops
                if isinstance(node, (ast.For, ast.While)):
                    self.loop_count += 1
                
                self.generic_visit(node)
                self.depth -= 1
        
        checker = ComplexityChecker()
        checker.visit(tree)
        
        # Set reasonable limits
        MAX_NODES = 1000
        MAX_DEPTH = 20
        MAX_LOOPS = 10
        
        if checker.node_count > MAX_NODES:
            logger.warning(f"Code too complex: {checker.node_count} nodes > {MAX_NODES}")
            return False
            
        if checker.max_depth > MAX_DEPTH:
            logger.warning(f"Code too deep: {checker.max_depth} levels > {MAX_DEPTH}")
            return False
            
        if checker.loop_count > MAX_LOOPS:
            logger.warning(f"Too many loops: {checker.loop_count} > {MAX_LOOPS}")
            return False
        
        return True
    
    def get_safe_builtins(self) -> Set[str]:
        """Get list of safe built-in functions"""
        all_builtins = set(dir(__builtins__))
        return all_builtins - self.forbidden_builtins
    
    def get_allowed_modules(self) -> List[str]:
        """Get list of allowed modules"""
        allowed = [
            'numpy', 'pandas', 'scipy', 'sklearn', 'matplotlib', 'seaborn',
            'plotly', 'ta', 'talib', 'zipline', 'pyfolio', 'quantlib',
            'statsmodels', 'arch', 'empyrical', 'tsfresh', 'stumpy',
            'prophet', 'math', 'statistics', 'datetime', 'time',
            'collections', 'itertools', 'functools', 'operator',
            're', 'json', 'csv', 'random', 'decimal', 'fractions'
        ]
        return allowed