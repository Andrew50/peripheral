"""
Ticker Extractor Utility

Parses Python strategy code using AST to extract all ticker symbols referenced
in get_bar_data() and get_general_data() function calls.

This shared utility is used by both generator.py and validator.py to ensure
consistent ticker extraction across the codebase.
"""

import ast
import logging
from typing import Dict, Set, Any, Optional

logger = logging.getLogger(__name__)


def extract_tickers(strategy_code: str) -> Dict[str, Any]:
    """
    Extract all ticker symbols from strategy code by parsing AST.
    
    Args:
        strategy_code: Python strategy code to analyze
        
    Returns:
        Dictionary with:
        - all_tickers: sorted list of unique ticker strings found
        - has_global: True if strategy uses non-literal filters or no filters
                     (indicating it operates on global universe)
    """
    try:
        tree = ast.parse(strategy_code)
        
        all_tickers: Set[str] = set()
        has_global = False
        
        # Walk through all nodes to find function calls
        for node in ast.walk(tree):
            if isinstance(node, ast.Call):
                func_name = _get_function_name(node)
                
                if func_name in ['get_bar_data', 'get_general_data']:
                    tickers, is_global = _extract_tickers_from_call(node)
                    all_tickers.update(tickers)
                    if is_global:
                        has_global = True
        
        return {
            "all_tickers": sorted(list(all_tickers)),
            "has_global": has_global
        }
        
    except (SyntaxError, ValueError) as e:
        logger.warning("Failed to parse strategy code for ticker extraction: %s", e)
        # Return safe defaults - treat as global strategy
        return {
            "all_tickers": [],
            "has_global": True
        }


def _get_function_name(call_node: ast.Call) -> Optional[str]:
    """Extract function name from call node, handling both direct and attribute calls."""
    if isinstance(call_node.func, ast.Name):
        return call_node.func.id
    if isinstance(call_node.func, ast.Attribute):
        return call_node.func.attr
    return None


def _extract_tickers_from_call(call_node: ast.Call) -> tuple[Set[str], bool]:
    """
    Extract tickers from a get_bar_data or get_general_data call.
    
    Args:
        call_node: AST Call node for the function
        
    Returns:
        Tuple of (ticker_set, is_global_flag)
        - ticker_set: Set of ticker strings found in filters
        - is_global_flag: True if no explicit tickers or non-literal filters detected
    """
    tickers: Set[str] = set()
    is_global = False
    
    # Find the filters argument - can be positional (index 3) or keyword
    filters_node = None
    
    # Check positional arguments (legacy signature: timeframe, columns, min_bars, filters, ...)
    if len(call_node.args) >= 4:
        filters_node = call_node.args[3]
    
    # Check keyword arguments
    for kw in call_node.keywords:
        if kw.arg == 'filters':
            filters_node = kw.value
            break
    
    # If no filters argument found, treat as global
    if filters_node is None:
        return tickers, True
    
    # Analyze the filters AST node
    extracted_tickers, has_non_literal = _analyze_filters_node(filters_node)
    tickers.update(extracted_tickers)
    
    # If we found non-literal expressions or no tickers, mark as global
    if has_non_literal or not extracted_tickers:
        is_global = True
    
    return tickers, is_global


def _analyze_filters_node(filters_node: ast.AST) -> tuple[Set[str], bool]:
    """
    Analyze filters AST node to extract ticker information.
    
    Args:
        filters_node: AST node representing the filters parameter
        
    Returns:
        Tuple of (ticker_set, has_non_literal_flag)
        - ticker_set: Set of ticker strings found
        - has_non_literal_flag: True if non-literal expressions were encountered
    """
    tickers: Set[str] = set()
    has_non_literal = False
    
    try:
        if isinstance(filters_node, ast.Dict):
            # Handle dictionary filters: {"tickers": ["AAPL", "MSFT"], ...}
            for key, value in zip(filters_node.keys, filters_node.values):
                key_str = _extract_string_literal(key)
                
                if key_str in ['tickers', 'ticker']:
                    extracted_tickers, has_non_lit = _extract_ticker_values(value)
                    tickers.update(extracted_tickers)
                    if has_non_lit:
                        has_non_literal = True
        else:
            # Non-dictionary filters (variable, function call, etc.) - treat as global
            has_non_literal = True
    
    except (ValueError, AttributeError) as e:
        logger.debug("Error analyzing filters node: %s", e)
        has_non_literal = True
    
    return tickers, has_non_literal


def _extract_ticker_values(value_node: ast.AST) -> tuple[Set[str], bool]:
    """
    Extract ticker values from the value part of a ticker filter.
    
    Args:
        value_node: AST node representing ticker values (string, list, etc.)
        
    Returns:
        Tuple of (ticker_set, has_non_literal_flag)
    """
    tickers: Set[str] = set()
    has_non_literal = False
    
    if isinstance(value_node, (ast.List, ast.Tuple)):
        # Handle list/tuple of tickers: ["AAPL", "MSFT"]
        for elem in value_node.elts:
            ticker = _extract_string_literal(elem)
            if ticker:
                tickers.add(ticker.upper())
            else:
                has_non_literal = True
    
    elif isinstance(value_node, (ast.Constant, ast.Str)):
        # Handle single ticker string: "AAPL"
        ticker = _extract_string_literal(value_node)
        if ticker:
            tickers.add(ticker.upper())
        else:
            has_non_literal = True
    
    else:
        # Non-literal value (variable, function call, etc.)
        has_non_literal = True
    
    return tickers, has_non_literal


def _extract_string_literal(node: Optional[ast.AST]) -> Optional[str]:
    """Extract string literal from AST node if possible."""
    try:
        if node is None:
            return None
        if isinstance(node, ast.Constant) and isinstance(node.value, str):
            return node.value
        if isinstance(node, ast.Str):  # Python < 3.8 compatibility
            return node.s
    except (ValueError, AttributeError):
        pass
    return None 