import logging
from typing import Any, Dict, List, Optional, Tuple
import ast

logger = logging.getLogger(__name__)
from typing import Any, Dict, List, Optional, Tuple




def _extract_get_bar_data_params(call_node: ast.Call) -> Optional[Dict[str, Any]]:
    """
    Extract parameters from a get_bar_data() call node.
    Returns dict with timeframe, min_bars, and filter_analysis.
    """
    try:
        call_info = {
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
                
        if len(call_node.args) >= 3:
            # Third arg is min_bars (second is columns)
            min_bars = _extract_int_value(call_node.args[2])
            if min_bars is not None:
                call_info['min_bars'] = min_bars
        
        # Extract keyword arguments
        filters_node = None
        for keyword in call_node.keywords:
            if keyword.arg == 'timeframe':
                timeframe = _extract_string_value(keyword.value)
                if timeframe:
                    call_info['timeframe'] = timeframe
            elif keyword.arg == 'min_bars':
                min_bars = _extract_int_value(keyword.value)
                if min_bars is not None:
                    call_info['min_bars'] = min_bars
            elif keyword.arg == 'filters':
                filters_node = keyword.value
        
        # Extract and analyze filters for ticker information
        call_info['filter_analysis'] = _analyze_filters_ast(filters_node)
        
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

def _analyze_filters_ast(filters_node: Optional[ast.AST]) -> Dict[str, Any]:
    """
    Analyze filters AST node to extract ticker information.
    """
    filter_analysis = {
        "has_tickers": False,
        "specific_tickers": []
    }
    
    if filters_node is None:
        return filter_analysis
    
    try:
        # Handle dict literals like {'tickers': ['AAPL', 'MSFT']}
        if isinstance(filters_node, ast.Dict):
            tickers = set()
            
            for key, value in zip(filters_node.keys, filters_node.values):
                # Look for 'tickers' or 'ticker' keys
                key_str = _extract_string_value(key)
                if key_str in ['tickers', 'ticker']:
                    # Extract ticker values
                    if isinstance(value, ast.List):
                        # Handle list of tickers: ['AAPL', 'MSFT']
                        for elem in value.elts:
                            ticker = _extract_string_value(elem)
                            tickers.add(ticker.upper())
                    elif isinstance(value, (ast.Constant, ast.Str)):
                        # Handle single ticker: 'AAPL'
                        ticker = _extract_string_value(value)
                        tickers.add(ticker.upper())
            
            if tickers:
                filter_analysis["has_tickers"] = True
                filter_analysis["specific_tickers"] = sorted(list(tickers))
                
    except Exception as e:
        logger.debug(f"Error analyzing filters AST: {e}")
    
    return filter_analysis