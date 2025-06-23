"""
Security Validator
Validates Python code for security issues before execution
"""

import ast
import logging
from typing import Set

logger = logging.getLogger(__name__)


class SecurityValidator:
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

    def validate_code(self, code: str) -> bool:
        """Validate code for security issues"""
        try:
            tree = ast.parse(code)
            self._check_ast_node(tree)
            return True
        except (SyntaxError, SecurityError):
            logger.warning(f"Code failed security validation")
            return False
        except Exception as e:
            logger.error(f"Error during code validation: {e}")
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
