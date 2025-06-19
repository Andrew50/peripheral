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
        """Get raw data accessor functions only - no pre-calculated indicators"""

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

        # ==================== RAW DATA RETRIEVAL ONLY ====================

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

        # ==================== RAW FUNDAMENTAL DATA ====================

        def get_fundamental_data(
            symbol: str, metrics: Optional[List[str]] = None
        ) -> Dict:
            return make_sync_wrapper(self.data_provider.get_fundamental_data)(
                symbol, metrics
            )

        def get_earnings_data(symbol: str, quarters: int = 8) -> Dict:
            # Placeholder implementation - would need earnings table
            return {
                "eps_actual": [],
                "eps_estimate": [],
                "revenue_actual": [],
                "revenue_estimate": [],
                "report_dates": [],
                "surprise_percent": [],
            }

        def get_financial_statements(
            symbol: str, statement_type: str = "income", periods: int = 4
        ) -> Dict:
            # Placeholder implementation - would need financial statements table
            return {"periods": [], "line_items": {}}

        # ==================== RAW MARKET DATA ====================

        def get_sector_data(sector: str = None, days: int = 5) -> Dict:
            return make_sync_wrapper(self.data_provider.get_sector_performance)(
                sector, days, None
            )

        def get_market_indices(
            indices: List[str] = None, days: int = 30
        ) -> Dict[str, Dict]:
            if not indices:
                indices = ["SPY", "QQQ", "IWM", "VIX"]
            return get_multiple_symbols_data(indices, "1d", days)

        def get_economic_calendar(days_ahead: int = 30) -> List[Dict]:
            # Placeholder implementation - would need economic calendar data
            return []

        # ==================== RAW VOLUME & FLOW DATA ====================

        def get_volume_data(symbol: str, days: int = 30) -> Dict:
            price_data = get_price_data(symbol, "1d", days)
            if price_data and price_data.get("volume"):
                return {
                    "timestamps": price_data["timestamps"],
                    "volume": price_data["volume"],
                    "dollar_volume": [
                        price_data["close"][i] * price_data["volume"][i]
                        for i in range(len(price_data["close"]))
                    ],
                    "trade_count": [0] * len(price_data["volume"]),  # Placeholder
                }
            return {
                "timestamps": [],
                "volume": [],
                "dollar_volume": [],
                "trade_count": [],
            }

        def get_options_chain(symbol: str, expiration: str = None) -> Dict:
            # Placeholder implementation - would need options data
            return {"calls": [], "puts": []}

        # ==================== RAW SENTIMENT & NEWS DATA ====================

        def get_news_sentiment(symbol: str = None, days: int = 7) -> List[Dict]:
            # Placeholder implementation - would need news data
            return []

        def get_social_mentions(symbol: str, days: int = 7) -> Dict:
            # Placeholder implementation - would need social data
            return {
                "timestamps": [],
                "mention_count": [],
                "sentiment_scores": [],
                "platforms": [],
            }

        # ==================== RAW INSIDER & INSTITUTIONAL DATA ====================

        def get_insider_trades(symbol: str, days: int = 90) -> List[Dict]:
            # Placeholder implementation - would need insider data
            return []

        def get_institutional_holdings(symbol: str, quarters: int = 4) -> List[Dict]:
            # Placeholder implementation - would need institutional data
            return []

        def get_short_data(symbol: str) -> Dict:
            # Placeholder implementation - would need short interest data
            return {
                "short_interest": 0,
                "short_ratio": 0,
                "days_to_cover": 0,
                "short_percent_float": 0,
                "previous_short_interest": 0,
            }

        # ==================== SCREENING & FILTERING ====================

        def scan_universe(
            filters: Dict = None, sort_by: str = None, limit: int = 100
        ) -> Dict:
            return make_sync_wrapper(self.data_provider.scan_universe)(
                filters, sort_by, limit
            )

        def get_universe_symbols(universe: str = "sp500") -> List[str]:
            # Placeholder implementation - would need universe definitions
            if universe == "sp500":
                return ["AAPL", "MSFT", "GOOGL", "AMZN", "TSLA"]  # Sample
            return []

        # ==================== UTILITY FUNCTIONS ====================

        def validate_symbol(symbol: str) -> Dict:
            info = get_security_info(symbol)
            return {
                "valid": bool(info),
                "active": info.get("active", False),
                "exchange": info.get("primary_exchange", ""),
                "asset_type": "stock",  # Assuming stocks for now
            }

        def get_trading_calendar(
            start_date: str = None, end_date: str = None, market: str = "NYSE"
        ) -> Dict:
            # Placeholder implementation - would need trading calendar
            return {"trading_days": [], "holidays": [], "early_closes": []}

        def get_market_status() -> Dict:
            # Placeholder implementation - would need real-time market status
            return {
                "is_open": False,
                "next_open": "",
                "next_close": "",
                "current_session": "closed",
            }

        # ==================== UTILITY FUNCTIONS FOR CALCULATIONS ====================

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

        def calculate_percentile(data: List[float], percentile: float) -> float:
            """Calculate percentile of data"""
            if not data:
                return 0.0

            sorted_data = sorted(data)
            index = (percentile / 100) * (len(sorted_data) - 1)

            if index.is_integer():
                return sorted_data[int(index)]
            else:
                lower = sorted_data[int(index)]
                upper = sorted_data[int(index) + 1]
                return lower + (upper - lower) * (index - int(index))

        def normalize_data(data: List[float], method: str = "z_score") -> List[float]:
            """Normalize data using various methods"""
            if not data:
                return []

            if method == "z_score":
                mean_val = sum(data) / len(data)
                variance = sum((x - mean_val) ** 2 for x in data) / len(data)
                std_val = variance**0.5
                if std_val == 0:
                    return [0.0] * len(data)
                return [(x - mean_val) / std_val for x in data]

            elif method == "min_max":
                min_val = min(data)
                max_val = max(data)
                if max_val == min_val:
                    return [0.5] * len(data)
                return [(x - min_val) / (max_val - min_val) for x in data]

            elif method == "robust":
                median_val = sorted(data)[len(data) // 2]
                mad = sorted([abs(x - median_val) for x in data])[len(data) // 2]
                if mad == 0:
                    return [0.0] * len(data)
                return [(x - median_val) / mad for x in data]

            return data

        def vectorized_operation(
            values: List[float], operation: str, operand: float = None
        ) -> List[float]:
            """Apply vectorized operations for performance"""
            import math

            if operation == "add" and operand is not None:
                return [x + operand for x in values]
            elif operation == "subtract" and operand is not None:
                return [x - operand for x in values]
            elif operation == "multiply" and operand is not None:
                return [x * operand for x in values]
            elif operation == "divide" and operand is not None:
                return [x / operand if operand != 0 else 0 for x in values]
            elif operation == "power" and operand is not None:
                return [x**operand for x in values]
            elif operation == "log":
                return [math.log(x) if x > 0 else 0 for x in values]
            elif operation == "sqrt":
                return [math.sqrt(x) if x >= 0 else 0 for x in values]
            else:
                return values

        def compare_lists(
            list1: List[float], list2: List[float], operator: str
        ) -> List[bool]:
            """Compare two lists element-wise"""
            min_len = min(len(list1), len(list2))

            if operator == ">":
                return [list1[i] > list2[i] for i in range(min_len)]
            elif operator == "<":
                return [list1[i] < list2[i] for i in range(min_len)]
            elif operator == ">=":
                return [list1[i] >= list2[i] for i in range(min_len)]
            elif operator == "<=":
                return [list1[i] <= list2[i] for i in range(min_len)]
            elif operator == "==":
                return [list1[i] == list2[i] for i in range(min_len)]
            elif operator == "!=":
                return [list1[i] != list2[i] for i in range(min_len)]
            else:
                return [False] * min_len

        # ==================== ENHANCED SYMBOL SEARCH & UNIVERSE FUNCTIONS ====================

        def fuzzy_search_symbols(query: str, threshold: int = 80, limit: int = 10) -> List[Dict]:
            """
            Fuzzy search for symbols using difflib for approximate matching
            Can be used to find tickers when you have partial/approximate names
            """
            import difflib
            
            # Get all available symbols from scan_universe with no filters
            universe_data = scan_universe(filters=None, sort_by="market_cap", limit=1000)
            all_symbols = universe_data.get("data", [])
            
            if not query or not all_symbols:
                return []
            
            query = query.upper().strip()
            matches = []
            
            for symbol_data in all_symbols:
                ticker = symbol_data.get("ticker", "")
                # Calculate similarity ratio for ticker
                ticker_ratio = difflib.SequenceMatcher(None, query, ticker).ratio() * 100
                
                if ticker_ratio >= threshold:
                    matches.append({
                        "ticker": ticker,
                        "similarity": round(ticker_ratio, 2),
                        "symbol_data": symbol_data
                    })
            
            # Sort by similarity score
            matches.sort(key=lambda x: x["similarity"], reverse=True)
            return matches[:limit]

        def search_by_name(company_name: str, threshold: int = 70, limit: int = 10) -> List[Dict]:
            """
            Search symbols by company name using fuzzy matching
            Useful when you know company name but not ticker
            """
            import difflib
            
            # Get all available symbols with their info
            universe_data = scan_universe(filters=None, sort_by="market_cap", limit=1000)
            all_symbols = universe_data.get("data", [])
            
            if not company_name or not all_symbols:
                return []
            
            query = company_name.lower().strip()
            matches = []
            
            for symbol_data in all_symbols:
                ticker = symbol_data.get("ticker", "")
                # Get full company info
                company_info = get_security_info(ticker)
                company_full_name = company_info.get("name", "").lower()
                
                if company_full_name:
                    # Calculate similarity for company name
                    name_ratio = difflib.SequenceMatcher(None, query, company_full_name).ratio() * 100
                    
                    if name_ratio >= threshold:
                        matches.append({
                            "ticker": ticker,
                            "company_name": company_info.get("name", ""),
                            "similarity": round(name_ratio, 2),
                            "symbol_data": symbol_data,
                            "company_info": company_info
                        })
            
            # Sort by similarity score
            matches.sort(key=lambda x: x["similarity"], reverse=True)
            return matches[:limit]

        def autocomplete_symbols(partial: str, limit: int = 10) -> List[str]:
            """
            Autocomplete ticker symbols for partial matches
            Returns list of tickers that start with the partial string
            """
            if not partial:
                return []
            
            partial = partial.upper().strip()
            universe_data = scan_universe(filters=None, sort_by="market_cap", limit=1000)
            all_symbols = universe_data.get("data", [])
            
            matches = []
            for symbol_data in all_symbols:
                ticker = symbol_data.get("ticker", "")
                if ticker.startswith(partial):
                    matches.append(ticker)
            
            return sorted(matches)[:limit]

        def get_advanced_universe(universe_type: str, **kwargs) -> List[str]:
            """
            Advanced universe selection with multiple built-in universes
            Can be extended by strategies to create custom universes
            """
            universe_type = universe_type.lower()
            
            if universe_type == "sp500":
                # Large cap US stocks - filter by market cap
                data = scan_universe(
                    filters={"min_market_cap": 10000000000},  # $10B+
                    sort_by="market_cap", 
                    limit=500
                )
                return data.get("symbols", [])
            
            elif universe_type == "nasdaq100":
                # Tech-heavy large caps - filter by sector and market cap
                data = scan_universe(
                    filters={
                        "sector": "Technology",
                        "min_market_cap": 5000000000  # $5B+
                    },
                    sort_by="market_cap", 
                    limit=100
                )
                return data.get("symbols", [])
            
            elif universe_type == "smallcap":
                # Small cap stocks
                data = scan_universe(
                    filters={"min_market_cap": 300000000},  # $300M - $2B range
                    sort_by="market_cap", 
                    limit=kwargs.get("limit", 200)
                )
                # Filter to exclude large caps (assuming $2B+ is large cap)
                return [s for s in data.get("symbols", [])][:kwargs.get("limit", 200)]
            
            elif universe_type == "sector":
                # Sector-specific universe
                sector = kwargs.get("sector", "Technology")
                data = scan_universe(
                    filters={"sector": sector},
                    sort_by="market_cap",
                    limit=kwargs.get("limit", 100)
                )
                return data.get("symbols", [])
            
            elif universe_type == "high_volume":
                # High volume stocks for day trading
                data = scan_universe(
                    filters={"min_market_cap": 1000000000},  # $1B+
                    sort_by="volume",
                    limit=kwargs.get("limit", 50)
                )
                return data.get("symbols", [])
            
            elif universe_type == "custom":
                # Custom universe based on multiple criteria
                filters = kwargs.get("filters", {})
                sort_by = kwargs.get("sort_by", "market_cap")
                limit = kwargs.get("limit", 100)
                
                data = scan_universe(
                    filters=filters,
                    sort_by=sort_by,
                    limit=limit
                )
                return data.get("symbols", [])
            
            else:
                # Default to basic list
                return ["AAPL", "MSFT", "GOOGL", "AMZN", "TSLA"]

        def create_custom_universe(criteria: Dict, name: str = "custom") -> Dict:
            """
            Create a custom universe based on complex criteria
            Returns both symbols and metadata for further analysis
            """
            # Extract criteria
            min_market_cap = criteria.get("min_market_cap", 0)
            max_market_cap = criteria.get("max_market_cap", float('inf'))
            sectors = criteria.get("sectors", [])
            min_volume = criteria.get("min_volume", 0)
            max_pe = criteria.get("max_pe_ratio", float('inf'))
            
            # Start with base universe
            base_data = scan_universe(filters=None, sort_by="market_cap", limit=1000)
            all_symbols = base_data.get("data", [])
            
            filtered_symbols = []
            
            for symbol_data in all_symbols:
                # Apply market cap filter
                market_cap = symbol_data.get("market_cap", 0) or 0
                if not (min_market_cap <= market_cap <= max_market_cap):
                    continue
                
                # Apply sector filter
                if sectors:
                    sector = symbol_data.get("sector", "")
                    if sector not in sectors:
                        continue
                
                # Apply volume filter
                volume = symbol_data.get("volume", 0) or 0
                if volume < min_volume:
                    continue
                
                # Apply PE ratio filter (approximate)
                eps = symbol_data.get("eps", 0) or 0
                price = symbol_data.get("price", 0) or 0
                if eps > 0 and price > 0:
                    pe_ratio = price / eps
                    if pe_ratio > max_pe:
                        continue
                
                filtered_symbols.append(symbol_data)
            
            return {
                "name": name,
                "criteria": criteria,
                "symbols": [s["ticker"] for s in filtered_symbols],
                "count": len(filtered_symbols),
                "data": filtered_symbols
            }

        def find_similar_stocks(reference_ticker: str, similarity_type: str = "sector", limit: int = 10) -> List[Dict]:
            """
            Find stocks similar to a reference stock based on various criteria
            similarity_type: 'sector', 'industry', 'market_cap', 'fundamentals'
            """
            ref_info = get_security_info(reference_ticker)
            if not ref_info:
                return []
            
            ref_fundamentals = get_fundamental_data(reference_ticker)
            
            if similarity_type == "sector":
                # Find stocks in same sector
                sector = ref_info.get("sector", "")
                if sector:
                    data = scan_universe(
                        filters={"sector": sector},
                        sort_by="market_cap",
                        limit=limit + 1  # +1 to exclude reference
                    )
                    similar = [s for s in data.get("data", []) if s.get("ticker") != reference_ticker]
                    return similar[:limit]
            
            elif similarity_type == "industry":
                # Find stocks in same industry (would need industry-based filtering)
                industry = ref_info.get("industry", "")
                # This would require additional filtering logic
                return []
            
            elif similarity_type == "market_cap":
                # Find stocks with similar market cap
                ref_market_cap = ref_fundamentals.get("market_cap", 0)
                if ref_market_cap > 0:
                    # Define range as +/- 50% of reference market cap
                    min_cap = ref_market_cap * 0.5
                    max_cap = ref_market_cap * 1.5
                    
                    # Would need custom filtering for market cap range
                    data = scan_universe(
                        filters={"min_market_cap": min_cap},
                        sort_by="market_cap",
                        limit=limit * 2
                    )
                    
                    # Filter out reference and apply max cap
                    similar = []
                    for s in data.get("data", []):
                        if s.get("ticker") == reference_ticker:
                            continue
                        s_market_cap = s.get("market_cap", 0)
                        if s_market_cap <= max_cap:
                            similar.append(s)
                    
                    return similar[:limit]
            
            return []

        # ==================== SIMPLE PAIR TRADING & SIMILAR TICKER FUNCTIONS ====================
        
        def find_pairs(ticker: str, pair_type: str = "sector") -> List[str]:
            """
            Simple function to find potential pair trading candidates
            Usage: pairs = find_pairs("SNOW", "sector")  # Find sector peers for SNOW
            """
            info = get_security_info(ticker)
            if not info:
                return []
            
            if pair_type == "sector":
                # Find stocks in same sector with similar market cap
                sector = info.get("sector", "")
                if not sector:
                    return []
                
                # Get sector peers
                sector_stocks = scan_universe(
                    filters={"sector": sector},
                    sort_by="market_cap",
                    limit=20
                )
                
                # Filter out the original ticker and return just ticker symbols
                pairs = [s.get("ticker") for s in sector_stocks.get("data", []) 
                        if s.get("ticker") != ticker]
                return pairs[:5]  # Return top 5 candidates
            
            elif pair_type == "industry":
                # For industry-based pairing (simplified - using sector for now)
                return find_pairs(ticker, "sector")
            
            elif pair_type == "size":
                # Find stocks with similar market cap regardless of sector
                fundamentals = get_fundamental_data(ticker)
                ref_market_cap = fundamentals.get("market_cap", 0)
                
                if ref_market_cap > 0:
                    # Get stocks in similar market cap range (+/- 30%)
                    min_cap = ref_market_cap * 0.7
                    max_cap = ref_market_cap * 1.3
                    
                    similar_size = scan_universe(
                        filters={"min_market_cap": min_cap},
                        sort_by="market_cap",
                        limit=30
                    )
                    
                    # Filter by max cap and exclude original
                    pairs = []
                    for s in similar_size.get("data", []):
                        if s.get("ticker") == ticker:
                            continue
                        s_market_cap = s.get("market_cap", 0)
                        if s_market_cap <= max_cap:
                            pairs.append(s.get("ticker"))
                    
                    return pairs[:5]
            
            return []

        def get_similar_tickers(ticker: str, count: int = 3) -> List[str]:
            """
            Get similar tickers for correlation/pair analysis
            Usage: similar = get_similar_tickers("AFRM", 3)
            """
            return find_pairs(ticker, "sector")[:count]

        def check_correlation(ticker1: str, ticker2: str, days: int = 30) -> float:
            """
            Calculate simple correlation between two tickers
            Returns correlation coefficient (-1 to 1)
            Usage: corr = check_correlation("SNOW", "AFRM", 30)
            """
            # Get price data for both tickers
            data1 = get_price_data(ticker1, "1d", days)
            data2 = get_price_data(ticker2, "1d", days)
            
            if not data1 or not data2:
                return 0.0
            
            prices1 = data1.get("close", [])
            prices2 = data2.get("close", [])
            
            if len(prices1) < 2 or len(prices2) < 2:
                return 0.0
            
            # Calculate returns
            returns1 = calculate_returns(prices1, 1)
            returns2 = calculate_returns(prices2, 1)
            
            if len(returns1) != len(returns2) or len(returns1) < 2:
                return 0.0
            
            # Calculate correlation
            n = len(returns1)
            sum_x = sum(returns1)
            sum_y = sum(returns2)
            sum_xx = sum(x * x for x in returns1)
            sum_yy = sum(y * y for y in returns2)
            sum_xy = sum(returns1[i] * returns2[i] for i in range(n))
            
            # Correlation formula
            numerator = n * sum_xy - sum_x * sum_y
            denominator = ((n * sum_xx - sum_x * sum_x) * (n * sum_yy - sum_y * sum_y)) ** 0.5
            
            if denominator == 0:
                return 0.0
            
            correlation = numerator / denominator
            return round(correlation, 3)

        def create_pair_trade(long_ticker: str, short_ticker: str, market_filter: str = None) -> Dict:
            """
            Simple pair trade setup with market condition check
            Usage: pair = create_pair_trade("SNOW", "AFRM", "QQQ_up")
            """
            # Check market condition if specified
            market_condition = True
            if market_filter:
                if market_filter == "QQQ_up":
                    qqq_data = get_price_data("QQQ", "1d", 5)
                    if qqq_data and qqq_data.get("close"):
                        prices = qqq_data["close"]
                        if len(prices) >= 2:
                            # Check if QQQ is trending up (current > 5-day average)
                            recent_avg = sum(prices[-5:]) / min(5, len(prices))
                            current = prices[-1]
                            market_condition = current > recent_avg
                        else:
                            market_condition = False
                elif market_filter == "SPY_down":
                    spy_data = get_price_data("SPY", "1d", 5)
                    if spy_data and spy_data.get("close"):
                        prices = spy_data["close"]
                        if len(prices) >= 2:
                            recent_avg = sum(prices[-5:]) / min(5, len(prices))
                            current = prices[-1]
                            market_condition = current < recent_avg
                        else:
                            market_condition = False
            
            # Get current data for both tickers
            long_data = get_price_data(long_ticker, "1d", 1)
            short_data = get_price_data(short_ticker, "1d", 1)
            
            # Calculate correlation
            correlation = check_correlation(long_ticker, short_ticker, 20)
            
            return {
                "long_ticker": long_ticker,
                "short_ticker": short_ticker,
                "market_condition_met": market_condition,
                "market_filter": market_filter,
                "correlation": correlation,
                "long_price": long_data.get("close", [0])[-1] if long_data else 0,
                "short_price": short_data.get("close", [0])[-1] if short_data else 0,
                "trade_viable": market_condition and abs(correlation) > 0.3,
                "timestamp": long_data.get("timestamps", [""])[-1] if long_data else ""
            }

        def quick_sector_pairs(sector: str, market_condition: str = None) -> List[Dict]:
            """
            Quick function to find all potential pairs in a sector
            Usage: tech_pairs = quick_sector_pairs("Technology", "QQQ_up")
            """
            # Get sector stocks
            sector_stocks = scan_universe(
                filters={"sector": sector},
                sort_by="market_cap",
                limit=10
            )
            
            tickers = [s.get("ticker") for s in sector_stocks.get("data", [])]
            
            if len(tickers) < 2:
                return []
            
            # Create all possible pairs
            pairs = []
            for i in range(len(tickers)):
                for j in range(i + 1, len(tickers)):
                    ticker1, ticker2 = tickers[i], tickers[j]
                    
                    # Check correlation
                    corr = check_correlation(ticker1, ticker2, 20)
                    
                    # Only include pairs with meaningful correlation
                    if abs(corr) > 0.2:
                        pair_data = create_pair_trade(ticker1, ticker2, market_condition)
                        pairs.append(pair_data)
            
            # Sort by correlation strength
            pairs.sort(key=lambda x: abs(x.get("correlation", 0)), reverse=True)
            return pairs[:5]  # Return top 5 pairs

        def market_condition_check(index_ticker: str = "QQQ", direction: str = "up", days: int = 3) -> bool:
            """
            Simple market condition check
            Usage: qqq_bullish = market_condition_check("QQQ", "up", 3)
            """
            data = get_price_data(index_ticker, "1d", days + 2)
            if not data or not data.get("close"):
                return False
            
            prices = data["close"]
            if len(prices) < days + 1:
                return False
            
            current = prices[-1]
            past_avg = sum(prices[-(days+1):-1]) / days
            
            if direction == "up":
                return current > past_avg * 1.01  # 1% threshold
            elif direction == "down":
                return current < past_avg * 0.99  # 1% threshold
            else:
                return True

        # ==================== EXAMPLE STRATEGY IMPLEMENTATIONS ====================
        
        def example_enhanced_symbol_search():
            """
            Example demonstrating all enhanced symbol search capabilities
            This shows how strategies can implement sophisticated ticker/company finding
            """
            log("Starting enhanced symbol search examples")
            
            # Example 1: Fuzzy ticker search
            log("1. Fuzzy ticker search for 'APL' (should find AAPL)")
            fuzzy_results = fuzzy_search_symbols("APL", threshold=60, limit=5)
            for result in fuzzy_results:
                log(f"   Found: {result['ticker']} (similarity: {result['similarity']}%)")
            
            # Example 2: Company name search
            log("2. Searching for companies with 'apple' in name")
            name_results = search_by_name("apple", threshold=60, limit=3)
            for result in name_results:
                log(f"   Found: {result['ticker']} - {result['company_name']} (similarity: {result['similarity']}%)")
            
            # Example 3: Autocomplete
            log("3. Autocomplete for 'AA'")
            autocomplete_results = autocomplete_symbols("AA", limit=5)
            log(f"   Autocomplete results: {autocomplete_results}")
            
            # Example 4: Advanced universe selection
            log("4. Getting S&P 500 universe")
            sp500_symbols = get_advanced_universe("sp500")
            log(f"   S&P 500 contains {len(sp500_symbols)} symbols")
            
            log("5. Getting tech sector universe")
            tech_symbols = get_advanced_universe("sector", sector="Technology", limit=10)
            log(f"   Tech sector top 10: {tech_symbols}")
            
            # Example 5: Custom universe creation
            log("6. Creating custom universe with complex criteria")
            custom_criteria = {
                "min_market_cap": 1000000000,  # $1B+
                "max_market_cap": 50000000000,  # Max $50B
                "sectors": ["Technology", "Healthcare"],
                "min_volume": 1000000,  # 1M+ volume
                "max_pe_ratio": 30
            }
            custom_universe = create_custom_universe(custom_criteria, "mid_cap_tech_health")
            log(f"   Custom universe '{custom_universe['name']}' has {custom_universe['count']} symbols")
            log(f"   First 5: {custom_universe['symbols'][:5]}")
            
            # Example 6: Find similar stocks
            log("7. Finding stocks similar to AAPL by sector")
            similar_to_aapl = find_similar_stocks("AAPL", similarity_type="sector", limit=5)
            similar_tickers = [s['ticker'] for s in similar_to_aapl]
            log(f"   Stocks similar to AAPL: {similar_tickers}")
            
            return {
                "fuzzy_search": fuzzy_results,
                "name_search": name_results,
                "autocomplete": autocomplete_results,
                "sp500_count": len(sp500_symbols),
                "tech_symbols": tech_symbols,
                "custom_universe": custom_universe,
                "similar_to_aapl": similar_tickers
            }

        def example_custom_classification_strategy():
            """
            Example showing how to implement custom stock classifications
            This demonstrates how strategies can create their own categorization systems
            """
            import difflib
            
            log("Creating custom stock classification system")
            
            # Get universe to classify
            all_stocks = scan_universe(filters=None, sort_by="market_cap", limit=200)
            stock_data = all_stocks.get("data", [])
            
            # Custom classification categories
            classifications = {
                "mega_cap_tech": [],
                "dividend_aristocrats": [],
                "high_growth_small_cap": [],
                "value_plays": [],
                "momentum_leaders": [],
                "defensive_stocks": []
            }
            
            for stock in stock_data:
                ticker = stock.get("ticker", "")
                market_cap = stock.get("market_cap", 0) or 0
                sector = stock.get("sector", "")
                price = stock.get("price", 0) or 0
                eps = stock.get("eps", 0) or 0
                volume = stock.get("volume", 0) or 0
                
                # Get additional fundamentals
                fundamentals = get_fundamental_data(ticker)
                dividend = fundamentals.get("dividend", 0) or 0
                
                # Mega cap tech classification
                if (market_cap > 100000000000 and  # $100B+
                    sector == "Technology"):
                    classifications["mega_cap_tech"].append(ticker)
                
                # Dividend aristocrats (simplified - just dividend paying large caps)
                elif (market_cap > 10000000000 and  # $10B+
                      dividend > 0):
                    classifications["dividend_aristocrats"].append(ticker)
                
                # High growth small cap (simplified)
                elif (market_cap < 2000000000 and  # Under $2B
                      market_cap > 300000000):  # Over $300M
                    classifications["high_growth_small_cap"].append(ticker)
                
                # Value plays (low PE ratio)
                elif (eps > 0 and price > 0 and
                      (price / eps) < 15):  # PE < 15
                    classifications["value_plays"].append(ticker)
                
                # High volume momentum
                elif volume > 5000000:  # 5M+ volume
                    classifications["momentum_leaders"].append(ticker)
                
                # Defensive sectors
                elif sector in ["Utilities", "Consumer Staples", "Healthcare"]:
                    classifications["defensive_stocks"].append(ticker)
            
            # Log results
            for category, symbols in classifications.items():
                log(f"{category}: {len(symbols)} stocks - {symbols[:5]}...")
            
            return classifications

        def example_advanced_screening_strategy():
            """
            Example showing advanced screening techniques that go beyond basic filters
            Demonstrates how to implement complex multi-factor screening
            """
            log("Running advanced multi-factor screening")
            
            # Get base universe
            base_universe = scan_universe(filters=None, sort_by="market_cap", limit=500)
            all_stocks = base_universe.get("data", [])
            
            # Advanced screening criteria
            screening_results = {
                "quality_growth": [],
                "contrarian_value": [],
                "momentum_breakout": [],
                "dividend_growth": [],
                "turnaround_candidates": []
            }
            
            for stock in all_stocks:
                ticker = stock.get("ticker", "")
                market_cap = stock.get("market_cap", 0) or 0
                sector = stock.get("sector", "")
                volume = stock.get("volume", 0) or 0
                price = stock.get("price", 0) or 0
                
                # Get fundamentals for detailed analysis
                fundamentals = get_fundamental_data(ticker)
                if not fundamentals:
                    continue
                
                eps = fundamentals.get("eps", 0) or 0
                revenue = fundamentals.get("revenue", 0) or 0
                debt = fundamentals.get("debt", 0) or 0
                cash = fundamentals.get("cash", 0) or 0
                book_value = fundamentals.get("book_value", 0) or 0
                
                # Quality Growth Screen
                if (market_cap > 1000000000 and  # $1B+
                    eps > 0 and revenue > 0 and
                    debt < cash and  # Net cash position
                    15 < (price / eps) < 25):  # Reasonable PE
                    screening_results["quality_growth"].append({
                        "ticker": ticker,
                        "market_cap": market_cap,
                        "pe_ratio": round(price / eps, 2) if eps > 0 else None,
                        "net_cash": cash - debt
                    })
                
                # Contrarian Value Screen
                elif (market_cap > 500000000 and  # $500M+
                      eps > 0 and price > 0 and
                      (price / eps) < 10 and  # Very low PE
                      book_value > 0 and
                      price < book_value * 1.2):  # Near book value
                    screening_results["contrarian_value"].append({
                        "ticker": ticker,
                        "pe_ratio": round(price / eps, 2),
                        "price_to_book": round(price / book_value, 2) if book_value > 0 else None
                    })
                
                # High volume momentum
                elif volume > 2000000:  # High volume
                    screening_results["momentum_breakout"].append({
                        "ticker": ticker,
                        "volume": volume,
                        "sector": sector
                    })
            
            # Limit results for each category
            for category in screening_results:
                screening_results[category] = screening_results[category][:10]
            
            # Log summary
            for category, results in screening_results.items():
                log(f"{category}: Found {len(results)} candidates")
                if results:
                    tickers = [r["ticker"] for r in results]
                    log(f"  Top picks: {tickers[:3]}")
            
            return screening_results

        def example_simple_pair_trade_strategy():
            """
            Simple example of SNOW/AFRM pair trade when QQQ is going up
            Demonstrates how easy it is to use the new pair trading functions
            """
            log("Testing SNOW/AFRM pair trade strategy")
            
            # Step 1: Check if QQQ is trending up
            qqq_bullish = market_condition_check("QQQ", "up", 3)
            log(f"QQQ bullish condition: {qqq_bullish}")
            
            if not qqq_bullish:
                log("QQQ not trending up, skipping trade")
                return {"trade_signal": "none", "reason": "QQQ not bullish"}
            
            # Step 2: Check correlation between SNOW and AFRM
            correlation = check_correlation("SNOW", "AFRM", 20)
            log(f"SNOW/AFRM correlation: {correlation}")
            
            # Step 3: Get similar tickers to both for context
            snow_peers = get_similar_tickers("SNOW", 3)
            afrm_peers = get_similar_tickers("AFRM", 3)
            log(f"SNOW similar tickers: {snow_peers}")
            log(f"AFRM similar tickers: {afrm_peers}")
            
            # Step 4: Create the pair trade setup
            pair_trade = create_pair_trade("SNOW", "AFRM", "QQQ_up")
            log(f"Pair trade viable: {pair_trade['trade_viable']}")
            
            # Step 5: Additional analysis - check both stocks individually
            snow_data = get_price_data("SNOW", "1d", 5)
            afrm_data = get_price_data("AFRM", "1d", 5)
            
            # Simple momentum check
            if snow_data and afrm_data:
                snow_prices = snow_data.get("close", [])
                afrm_prices = afrm_data.get("close", [])
                
                if len(snow_prices) >= 2 and len(afrm_prices) >= 2:
                    snow_momentum = (snow_prices[-1] / snow_prices[-2]) - 1
                    afrm_momentum = (afrm_prices[-1] / afrm_prices[-2]) - 1
                    
                    log(f"SNOW 1-day momentum: {snow_momentum:.2%}")
                    log(f"AFRM 1-day momentum: {afrm_momentum:.2%}")
                    
                    # Determine trade direction based on relative momentum
                    if snow_momentum > afrm_momentum:
                        trade_signal = "long_SNOW_short_AFRM"
                        log("Signal: Long SNOW, Short AFRM (SNOW has better momentum)")
                    else:
                        trade_signal = "long_AFRM_short_SNOW" 
                        log("Signal: Long AFRM, Short SNOW (AFRM has better momentum)")
                else:
                    trade_signal = "insufficient_data"
            else:
                trade_signal = "no_data"
            
            # Return comprehensive results
            return {
                "strategy": "SNOW_AFRM_pair_trade",
                "market_condition": "QQQ_bullish" if qqq_bullish else "QQQ_neutral_bearish",
                "correlation": correlation,
                "pair_trade_setup": pair_trade,
                "trade_signal": trade_signal,
                "snow_peers": snow_peers,
                "afrm_peers": afrm_peers,
                "snow_price": pair_trade["long_price"] if trade_signal == "long_SNOW_short_AFRM" else pair_trade["short_price"],
                "afrm_price": pair_trade["short_price"] if trade_signal == "long_SNOW_short_AFRM" else pair_trade["long_price"],
                "timestamp": pair_trade["timestamp"]
            }

        def example_sector_pair_screening():
            """
            Example showing how to quickly screen for pair opportunities in a sector
            """
            log("Screening Technology sector for pair trading opportunities")
            
            # Get all tech pairs when QQQ is bullish
            tech_pairs = quick_sector_pairs("Technology", "QQQ_up")
            
            log(f"Found {len(tech_pairs)} viable tech pairs")
            
            # Show top 3 pairs
            top_pairs = []
            for i, pair in enumerate(tech_pairs[:3]):
                log(f"Pair {i+1}: {pair['long_ticker']} vs {pair['short_ticker']}")
                log(f"  Correlation: {pair['correlation']}")
                log(f"  Market condition met: {pair['market_condition_met']}")
                log(f"  Trade viable: {pair['trade_viable']}")
                
                top_pairs.append({
                    "rank": i + 1,
                    "pair": f"{pair['long_ticker']}/{pair['short_ticker']}",
                    "correlation": pair['correlation'],
                    "viable": pair['trade_viable']
                })
            
            return {
                "sector": "Technology",
                "total_pairs_found": len(tech_pairs),
                "top_pairs": top_pairs,
                "all_pairs": tech_pairs
            }

        # Return all raw data accessor functions
        return {
            # Raw data retrieval
            "get_price_data": get_price_data,
            "get_historical_data": get_historical_data,
            "get_security_info": get_security_info,
            "get_multiple_symbols_data": get_multiple_symbols_data,
            # Raw fundamental data
            "get_fundamental_data": get_fundamental_data,
            "get_earnings_data": get_earnings_data,
            "get_financial_statements": get_financial_statements,
            # Raw market data
            "get_sector_data": get_sector_data,
            "get_market_indices": get_market_indices,
            "get_economic_calendar": get_economic_calendar,
            # Raw volume & flow data
            "get_volume_data": get_volume_data,
            "get_options_chain": get_options_chain,
            # Raw sentiment & news data
            "get_news_sentiment": get_news_sentiment,
            "get_social_mentions": get_social_mentions,
            # Raw insider & institutional data
            "get_insider_trades": get_insider_trades,
            "get_institutional_holdings": get_institutional_holdings,
            "get_short_data": get_short_data,
            # Screening & filtering
            "scan_universe": scan_universe,
            "get_universe_symbols": get_universe_symbols,
            # Utility functions
            "validate_symbol": validate_symbol,
            "get_trading_calendar": get_trading_calendar,
            "get_market_status": get_market_status,
            # Calculation utilities
            "calculate_returns": calculate_returns,
            "calculate_log_returns": calculate_log_returns,
            "rolling_window": rolling_window,
            "calculate_percentile": calculate_percentile,
            "normalize_data": normalize_data,
            "vectorized_operation": vectorized_operation,
            "compare_lists": compare_lists,
            # Enhanced universe functions
            "fuzzy_search_symbols": fuzzy_search_symbols,
            "search_by_name": search_by_name,
            "autocomplete_symbols": autocomplete_symbols,
            "get_advanced_universe": get_advanced_universe,
            "create_custom_universe": create_custom_universe,
            "find_similar_stocks": find_similar_stocks,
            # Simple pair trading functions
            "find_pairs": find_pairs,
            "get_similar_tickers": get_similar_tickers,
            "check_correlation": check_correlation,
            "create_pair_trade": create_pair_trade,
            "quick_sector_pairs": quick_sector_pairs,
            "market_condition_check": market_condition_check,
            # Example strategies
            "example_enhanced_symbol_search": example_enhanced_symbol_search,
            "example_custom_classification_strategy": example_custom_classification_strategy,
            "example_advanced_screening_strategy": example_advanced_screening_strategy,
            "example_simple_pair_trade_strategy": example_simple_pair_trade_strategy,
            "example_sector_pair_screening": example_sector_pair_screening,
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
