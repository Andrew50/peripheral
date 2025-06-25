"""
AST Analysis Implementation for Data Requirements Optimization
Updated to work with numpy arrays instead of pandas DataFrames.
"""

import ast
import re
import logging
from typing import Dict, List, Set, Any, Optional, Union
from datetime import datetime, timedelta

logger = logging.getLogger(__name__)


class StrategyDataAnalyzer:
    """
    Multi-pass AST analyzer for determining optimal data requirements
    based on execution mode (screener, alert, backtest) and code complexity.
    Updated to work with numpy arrays instead of DataFrames.
    """
    
    def __init__(self):
        self.mode_optimizers = {
            'screener': self._optimize_for_screener,
            'alert': self._optimize_for_alert, 
            'backtest': self._optimize_for_backtest
        }
        
        # Column mapping for numpy array indexing
        self.column_mapping = {
            0: 'ticker',
            1: 'date', 
            2: 'open',
            3: 'high',
            4: 'low',
            5: 'close',
            6: 'volume',
            7: 'adj_close',
            # Add fundamental data columns at higher indices
            8: 'fund_pe_ratio',
            9: 'fund_pb_ratio',
            10: 'fund_market_cap',
            11: 'fund_sector',
            12: 'fund_industry',
            13: 'fund_dividend_yield'
        }
        
        # Reverse mapping for column name to index
        self.column_indices = {v: k for k, v in self.column_mapping.items()}
    
    def analyze_data_requirements(self, strategy_code: str, mode: str = 'backtest') -> Dict[str, Any]:
        """
        Main entry point: Analyze strategy code and return mode-specific data requirements
        
        Args:
            strategy_code: Python strategy function code using numpy arrays
            mode: Execution mode ('screener', 'alert', 'backtest')
            
        Returns:
            Dict with optimized data requirements for the specified mode
        """
        try:
            # Pass 1: Identify all data access patterns (numpy-based)
            data_accesses = self.find_numpy_access_patterns(strategy_code)
            
            # Pass 2: Analyze data usage context (filter vs compute)
            usage_context = self.analyze_numpy_usage_context(strategy_code, data_accesses)
            
            # Pass 3: Generate mode-specific requirements
            mode_requirements = self.generate_mode_specific_requirements(usage_context, mode)
            
            # Pass 4: Classify strategy complexity for loading strategy selection
            complexity = self.classify_strategy_complexity(usage_context)
            loading_strategy = self.select_data_loading_strategy(complexity, mode)
            
            return {
                'data_requirements': mode_requirements,
                'strategy_complexity': complexity,
                'loading_strategy': loading_strategy,
                'analysis_metadata': {
                    'data_accesses': data_accesses,
                    'usage_context': usage_context,
                    'analyzed_at': datetime.utcnow().isoformat()
                }
            }
            
        except Exception as e:
            logger.warning(f"AST analysis failed, using fallback: {e}")
            return self.handle_ast_analysis_failure(strategy_code, mode)
    
    def find_numpy_access_patterns(self, strategy_code: str) -> Dict[str, Any]:
        """
        Pass 1: Identify numpy array access patterns in the strategy code
        Looks for patterns like row[0], data[i], row[5], etc.
        """
        patterns = {
            'accessed_columns': set(),
            'accessed_indices': set(),
            'function_calls': [],
            'filtering_operations': [],
            'computational_operations': [],
            'loops_over_data': False,
            'target_tickers': set()
        }
        
        try:
            tree = ast.parse(strategy_code)
            
            for node in ast.walk(tree):
                # Array indexing: row[0], data[i, 5], etc.
                if isinstance(node, ast.Subscript):
                    if self._is_numpy_array_access(node):
                        index_info = self._extract_array_index(node)
                        if index_info:
                            patterns['accessed_indices'].add(index_info['index'])
                            if index_info['index'] in self.column_mapping:
                                patterns['accessed_columns'].add(self.column_mapping[index_info['index']])
                
                # Function calls on numpy data
                elif isinstance(node, ast.Call):
                    func_info = self._extract_numpy_function_info(node)
                    if func_info:
                        patterns['function_calls'].append(func_info)
                
                # String comparisons for ticker filtering
                elif isinstance(node, ast.Compare):
                    if self._is_ticker_comparison(node):
                        ticker = self._extract_ticker_value(node)
                        if ticker:
                            patterns['target_tickers'].add(ticker)
                
                # For loops over data arrays
                elif isinstance(node, ast.For):
                    if self._is_data_iteration(node):
                        patterns['loops_over_data'] = True
                
                # Assignments that might indicate calculations
                elif isinstance(node, ast.Assign):
                    if self._is_computational_assignment(node):
                        patterns['computational_operations'].append(self._extract_computation_info(node))
            
        except (SyntaxError, ValueError) as e:
            logger.warning(f"Failed to parse strategy code for numpy patterns: {e}")
            return self._fallback_numpy_pattern_extraction(strategy_code)
        
        # Convert sets to lists for JSON serialization
        patterns['accessed_columns'] = list(patterns['accessed_columns'])
        patterns['accessed_indices'] = list(patterns['accessed_indices'])
        patterns['target_tickers'] = list(patterns['target_tickers'])
        return patterns
    
    def analyze_numpy_usage_context(self, strategy_code: str, data_accesses: Dict[str, Any]) -> Dict[str, Any]:
        """
        Pass 2: Determine how numpy array data is used (filtering vs computation)
        """
        context = {
            'filter_only_columns': set(),
            'computation_columns': set(),
            'price_columns': set(),
            'fundamental_columns': set(),
            'volume_columns': set(),
            'requires_ticker_filtering': bool(data_accesses.get('target_tickers')),
            'has_calculations': bool(data_accesses.get('computational_operations')),
            'iterates_over_data': data_accesses.get('loops_over_data', False)
        }
        
        # Analyze each accessed column
        for column in data_accesses['accessed_columns']:
            if self._is_fundamental_column(column):
                context['fundamental_columns'].add(column)
                # If only used in comparisons, likely filter-only
                if self._is_only_used_in_filters(strategy_code, column):
                    context['filter_only_columns'].add(column)
            elif self._is_price_column(column):
                context['price_columns'].add(column)
                if self._is_used_in_calculations(strategy_code, column):
                    context['computation_columns'].add(column)
            elif self._is_volume_column(column):
                context['volume_columns'].add(column)
        
        # Calculate lookback requirements (simpler for numpy)
        context['lookback_analysis'] = self._calculate_numpy_lookback(data_accesses)
        
        # Convert sets to lists for JSON serialization
        for key in context:
            if isinstance(context[key], set):
                context[key] = list(context[key])
        
        return context
    
    def generate_mode_specific_requirements(self, usage_context: Dict[str, Any], mode: str) -> Dict[str, Any]:
        """
        Pass 3: Generate optimized requirements based on execution mode
        """
        optimizer = self.mode_optimizers.get(mode, self._optimize_for_backtest)
        return optimizer(usage_context)
    
    def _optimize_for_screener(self, usage_patterns: Dict[str, Any]) -> Dict[str, Any]:
        """Screener typically needs current snapshot only"""
        
        # For screeners, prioritize filter-only columns and fundamentals
        essential_columns = list(set(
            usage_patterns.get('filter_only_columns', []) +
            usage_patterns.get('fundamental_columns', []) +
            ['ticker', 'date']  # Always need these for numpy arrays
        ))
        
        return {
            'timeframe': '1d',
            'periods': 1,  # Current data only
            'columns': essential_columns,
            'fundamentals': usage_patterns.get('fundamental_columns', []),
            'target_tickers': usage_patterns.get('target_tickers', []),
            'filters_first': True,  # Apply filters before loading into numpy
            'estimated_rows': len(usage_patterns.get('target_tickers', [])) if usage_patterns.get('target_tickers') else 500,
            'mode_optimization': 'screener_snapshot',
            'load_strategy': 'minimal_numpy_array'
        }
    
    def _optimize_for_alert(self, usage_patterns: Dict[str, Any]) -> Dict[str, Any]:
        """Alerts need recent window + current data"""
        
        lookback = usage_patterns.get('lookback_analysis', {})
        periods = min(lookback.get('total_periods', 30), 30)  # Cap at 30 days
        
        essential_columns = (
            usage_patterns.get('price_columns', []) + 
            usage_patterns.get('computation_columns', []) +
            ['ticker', 'date']
        )
        
        return {
            'timeframe': '1d',
            'periods': periods,
            'columns': list(set(essential_columns)),
            'fundamentals': usage_patterns.get('fundamental_columns', []),
            'target_tickers': usage_patterns.get('target_tickers', []),
            'real_time': True,
            'estimated_rows': periods * len(usage_patterns.get('target_tickers', ['DEFAULT'])),
            'mode_optimization': 'alert_recent_window',
            'load_strategy': 'rolling_numpy_array'
        }
    
    def _optimize_for_backtest(self, usage_patterns: Dict[str, Any]) -> Dict[str, Any]:
        """Backtest needs historical time series"""
        
        lookback = usage_patterns.get('lookback_analysis', {})
        
        essential_columns = (
            usage_patterns.get('price_columns', []) + 
            usage_patterns.get('computation_columns', []) +
            ['ticker', 'date']
        )
        
        return {
            'timeframe': '1d',
            'periods': lookback.get('total_periods', 252),  # Default 1 year
            'columns': list(set(essential_columns)),
            'fundamentals': usage_patterns.get('fundamental_columns', []),
            'target_tickers': usage_patterns.get('target_tickers', []),
            'batch_size': 50,  # Process symbols in batches
            'estimated_rows': lookback.get('total_periods', 252) * len(usage_patterns.get('target_tickers', ['DEFAULT'])),
            'mode_optimization': 'backtest_time_series',
            'load_strategy': 'batched_numpy_array'
        }
    
    def _calculate_numpy_lookback(self, usage_patterns: Dict[str, Any]) -> Dict[str, Any]:
        """Calculate lookback requirements for numpy-based strategies"""
        
        # For numpy strategies, lookback is usually simpler
        # Most strategies iterate over data without complex rolling operations
        return {
            'base_periods': 1,
            'rolling_window_max': 0,  # Numpy strategies typically don't use rolling windows
            'shift_operations_max': 0,
            'total_periods': 30  # Default reasonable lookback
        }
    
    # Helper methods for numpy array analysis
    def _is_numpy_array_access(self, node: ast.Subscript) -> bool:
        """Check if subscript is numpy array access like row[0] or data[i]"""
        if isinstance(node.value, ast.Name):
            return node.value.id in ['row', 'data', 'current_row', 'item']
        return False
    
    def _extract_array_index(self, node: ast.Subscript) -> Optional[Dict[str, Any]]:
        """Extract index from array access like row[5]"""
        if isinstance(node.slice, ast.Constant) and isinstance(node.slice.value, int):
            return {
                'array_name': node.value.id if isinstance(node.value, ast.Name) else 'unknown',
                'index': node.slice.value
            }
        return None
    
    def _extract_numpy_function_info(self, node: ast.Call) -> Optional[Dict[str, Any]]:
        """Extract function call information for numpy operations"""
        func_info = {'function': '', 'module': '', 'args': []}
        
        if isinstance(node.func, ast.Attribute):
            func_info['function'] = node.func.attr
            if isinstance(node.func.value, ast.Name):
                func_info['module'] = node.func.value.id
        elif isinstance(node.func, ast.Name):
            func_info['function'] = node.func.id
        
        # Extract arguments
        for arg in node.args:
            if isinstance(arg, ast.Constant):
                func_info['args'].append(arg.value)
        
        # Only return if it's numpy-related
        if func_info['module'] in ['np', 'numpy'] or func_info['function'] in ['float', 'str', 'int']:
            return func_info
        return None
    
    def _is_ticker_comparison(self, node: ast.Compare) -> bool:
        """Check if comparison is for ticker filtering"""
        # Look for patterns like ticker == 'ASTS' or row[0] == 'NVDA'
        if isinstance(node.left, ast.Name) and node.left.id == 'ticker':
            return True
        if isinstance(node.left, ast.Subscript):
            index_info = self._extract_array_index(node.left)
            if index_info and index_info['index'] == 0:  # ticker is at index 0
                return True
        return False
    
    def _extract_ticker_value(self, node: ast.Compare) -> Optional[str]:
        """Extract ticker symbol from comparison"""
        if node.comparators and isinstance(node.comparators[0], ast.Constant):
            value = node.comparators[0].value
            if isinstance(value, str) and value.isupper() and 1 <= len(value) <= 5:
                return value
        return None
    
    def _is_data_iteration(self, node: ast.For) -> bool:
        """Check if for loop iterates over data array"""
        if isinstance(node.iter, ast.Name):
            return node.iter.id in ['data', 'rows', 'market_data']
        return False
    
    def _is_computational_assignment(self, node: ast.Assign) -> bool:
        """Check if assignment involves calculations"""
        # Look for mathematical operations in the value
        if isinstance(node.value, ast.BinOp):
            return True
        if isinstance(node.value, ast.Call):
            # Function calls that might be calculations
            if isinstance(node.value.func, ast.Name):
                calc_functions = {'float', 'abs', 'round', 'max', 'min'}
                return node.value.func.id in calc_functions
        return False
    
    def _extract_computation_info(self, node: ast.Assign) -> Dict[str, Any]:
        """Extract information about computational assignments"""
        return {
            'type': 'assignment',
            'has_math': isinstance(node.value, ast.BinOp),
            'has_function_call': isinstance(node.value, ast.Call)
        }
    
    def _is_only_used_in_filters(self, strategy_code: str, column: str) -> bool:
        """Check if column is only used in filtering operations"""
        # Simple heuristic: if column appears in comparisons but not calculations
        comparison_count = len(re.findall(rf'{column}.*[><=!]', strategy_code))
        calculation_count = len(re.findall(rf'{column}.*[+\-*/]', strategy_code))
        return comparison_count > 0 and calculation_count == 0
    
    def _is_used_in_calculations(self, strategy_code: str, column: str) -> bool:
        """Check if column is used in calculations"""
        # Look for mathematical operations
        return bool(re.search(rf'{column}.*[+\-*/]', strategy_code))
    
    def _is_fundamental_column(self, column: str) -> bool:
        """Check if column is fundamental data"""
        fundamental_patterns = [
            'pe_ratio', 'pb_ratio', 'market_cap', 'sector', 'industry',
            'revenue', 'earnings', 'debt', 'book_value', 'dividend'
        ]
        return any(pattern in column.lower() for pattern in fundamental_patterns)
    
    def _is_price_column(self, column: str) -> bool:
        """Check if column is price data"""
        price_columns = ['open', 'high', 'low', 'close', 'adj_close']
        return column.lower() in price_columns
    
    def _is_volume_column(self, column: str) -> bool:
        """Check if column is volume data"""
        return column.lower() in ['volume', 'vol']
    
    def _fallback_numpy_pattern_extraction(self, strategy_code: str) -> Dict[str, Any]:
        """Regex-based fallback for numpy pattern extraction"""
        patterns = {
            'accessed_columns': [],
            'accessed_indices': [],
            'function_calls': [],
            'filtering_operations': [],
            'computational_operations': [],
            'loops_over_data': False,
            'target_tickers': []
        }
        
        # Extract array indexing patterns
        index_matches = re.findall(r'row\[(\d+)\]', strategy_code)
        for match in index_matches:
            idx = int(match)
            patterns['accessed_indices'].append(idx)
            if idx in self.column_mapping:
                patterns['accessed_columns'].append(self.column_mapping[idx])
        
        # Extract ticker symbols
        ticker_matches = re.findall(r"['\"]([A-Z]{1,5})['\"]", strategy_code)
        patterns['target_tickers'] = list(set(ticker_matches))
        
        # Check for data iteration
        if re.search(r'for.*in.*data', strategy_code):
            patterns['loops_over_data'] = True
        
        return patterns
    
    def classify_strategy_complexity(self, ast_analysis: Dict[str, Any]) -> str:
        """Determine strategy complexity for numpy-based strategies"""
        
        # Numpy strategies are generally simpler than DataFrame strategies
        # Most complexity comes from the amount of calculation vs filtering
        
        has_calculations = ast_analysis.get('has_calculations', False)
        computation_columns = len(ast_analysis.get('computation_columns', []))
        
        if not has_calculations and computation_columns == 0:
            return 'simple_filter'
        elif computation_columns < 3:
            return 'numpy_optimized'
        else:
            return 'numpy_complex'
    
    def select_data_loading_strategy(self, complexity: str, mode: str) -> str:
        """Choose optimal data loading approach for numpy strategies"""
        
        strategies = {
            ('simple_filter', 'screener'): 'minimal_numpy_array',
            ('simple_filter', 'backtest'): 'filtered_numpy_array',
            ('simple_filter', 'alert'): 'recent_numpy_array',
            
            ('numpy_optimized', 'screener'): 'minimal_numpy_array',
            ('numpy_optimized', 'backtest'): 'batched_numpy_array',
            ('numpy_optimized', 'alert'): 'rolling_numpy_array',
            
            ('numpy_complex', 'screener'): 'full_numpy_array',
            ('numpy_complex', 'backtest'): 'batched_numpy_array',
            ('numpy_complex', 'alert'): 'full_numpy_array'
        }
        
        return strategies.get((complexity, mode), 'batched_numpy_array')
    
    def handle_ast_analysis_failure(self, strategy_code: str, mode: str) -> Dict[str, Any]:
        """Fallback when AST analysis is incomplete"""
        
        # Conservative approach for numpy arrays
        fallback_requirements = {
            'columns': ['ticker', 'date', 'open', 'high', 'low', 'close', 'volume'],
            'fundamentals': [],
            'periods': 30 if mode != 'screener' else 1,
            'timeframe': '1d',
            'note': 'fallback_due_to_ast_analysis_limitation'
        }
        
        return {
            'data_requirements': fallback_requirements,
            'strategy_complexity': 'unknown',
            'loading_strategy': 'batched_numpy_array',
            'analysis_metadata': {
                'fallback_used': True,
                'analyzed_at': datetime.utcnow().isoformat()
            }
        } 