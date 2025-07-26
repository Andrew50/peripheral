
def get_all_tickers_from_calls(getBarDataFunctionCalls: List[Dict[str, Any]]) -> List[str]:
    """
    Extract all unique tickers from all get_bar_data calls
    
    Returns:
        List of unique ticker symbols found in filters
    """
    all_tickers = set()
    
    for call in getBarDataFunctionCalls:
        analysis = call.get("filter_analysis", {})
        if analysis.get("has_tickers"):
            specific_tickers = analysis.get("specific_tickers", [])
            all_tickers.update(specific_tickers)
    
    return sorted(list(all_tickers))

def getMaxTimeframeAndMinBars(getBarDataFunctionCalls: List[Dict[str, Any]]) -> Tuple[Optional[str], int]:
    """
    Get the max timeframe and its associated min_bars from get_bar_data calls
    
    Returns:
        Tuple of (max_timeframe, max_timeframe_min_bars)
    """
    import re
    
    max_tf_priority = (0, 0)  # (category, multiplier)
    max_tf_str = None
    max_tf_min_bars = 0
    
    # Timeframe priority: week/month > day > hour > minute
    tf_categories = {'m': 0, 'h': 1, 'd': 2, 'w': 3, 'M': 4}
    
    for call in getBarDataFunctionCalls:
        timeframe = call.get("timeframe")
        if isinstance(timeframe, str):
            # Parse timeframe (e.g., "13m" -> category=0, multiplier=13)
            match = re.match(r'(\d+)([mhdwM])', timeframe)
            if match:
                multiplier = int(match.group(1))
                category = tf_categories.get(match.group(2), 0)
                tf_priority = (category, multiplier)
                
                # Update max timeframe if this one has higher priority
                if tf_priority > max_tf_priority:
                    max_tf_priority = tf_priority
                    max_tf_str = timeframe
                    max_tf_min_bars = call.get("min_bars", 0)
    
    return max_tf_str, max_tf_min_bars

def extract_get_bar_data_calls(strategy_code: str) -> List[Dict[str, Any]]:
    """
    Extract all get_bar_data calls from strategy code using AST parsing.
    Returns list of dicts with timeframe, min_bars, and filter_analysis.
    """
    calls = []
    
    try:
        # Parse the code into an AST
        tree = ast.parse(strategy_code)
        
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
                        calls.append(call_info)
                        
    except SyntaxError as e:
        logger.warning(f"Failed to parse strategy code for get_bar_data extraction: {e}")
    except Exception as e:
        logger.warning(f"Error extracting get_bar_data calls: {e}")
        
    return calls

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