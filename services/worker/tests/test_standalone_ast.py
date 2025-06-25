#!/usr/bin/env python3
"""
Standalone AST Parser Test for Complex Strategy Scenarios
Tests AST parsing and data requirements analysis without external dependencies
"""

import ast
import re
import sys
import os
from datetime import datetime
from typing import Dict, List, Set, Any, Optional

# Add src to path for imports
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'src'))

class StandaloneStrategyAnalyzer:
    """Standalone strategy analyzer for testing without external dependencies"""
    
    def __init__(self):
        # Column mapping for numpy arrays
        self.column_mapping = {
            0: 'ticker',
            1: 'date', 
            2: 'open',
            3: 'high',
            4: 'low',
            5: 'close',
            6: 'volume',
            7: 'adj_close',
            8: 'fund_pe_ratio',
            9: 'fund_pb_ratio',
            10: 'fund_market_cap',
            11: 'fund_sector',
            12: 'fund_industry',
            13: 'fund_dividend_yield'
        }
        
        self.column_indices = {v: k for k, v in self.column_mapping.items()}
    
    def analyze_strategy_code(self, strategy_code: str, mode: str = 'backtest') -> Dict[str, Any]:
        """Analyze strategy code and return data requirements and complexity"""
        
        try:
            # Parse the code into an AST
            tree = ast.parse(strategy_code)
            
            # Extract data access patterns
            data_accesses = self._find_numpy_access_patterns(strategy_code, tree)
            
            # Analyze usage context
            usage_context = self._analyze_usage_context(tree, data_accesses)
            
            # Generate mode-specific requirements
            mode_requirements = self._generate_mode_requirements(usage_context, mode)
            
            # Classify complexity
            complexity = self._classify_complexity(usage_context)
            
            # Select loading strategy
            loading_strategy = self._select_loading_strategy(complexity, mode)
            
            return {
                'success': True,
                'data_requirements': mode_requirements,
                'strategy_complexity': complexity,
                'loading_strategy': loading_strategy,
                'data_accesses': data_accesses,
                'usage_context': usage_context,
                'analyzed_at': datetime.utcnow().isoformat()
            }
            
        except Exception as e:
            return {
                'success': False,
                'error': str(e),
                'analyzed_at': datetime.utcnow().isoformat()
            }
    
    def _find_numpy_access_patterns(self, strategy_code: str, tree: ast.AST) -> Dict[str, Any]:
        """Find numpy array access patterns in the strategy code"""
        
        patterns = {
            'accessed_columns': set(),
            'accessed_indices': set(),
            'function_calls': [],
            'filtering_operations': [],
            'computational_operations': [],
            'loops_over_data': False,
            'target_tickers': set(),
            'gap_analysis': False,
            'sector_analysis': False,
            'technical_indicators': False,
            'percentile_analysis': False,
            'relative_performance': False
        }
        
        for node in ast.walk(tree):
            # Array indexing: data[i, 0], data[i, 5], etc.
            if isinstance(node, ast.Subscript):
                if self._is_numpy_array_access(node):
                    index_info = self._extract_array_index(node)
                    if index_info and isinstance(index_info['index'], int):
                        patterns['accessed_indices'].add(index_info['index'])
                        if index_info['index'] in self.column_mapping:
                            patterns['accessed_columns'].add(self.column_mapping[index_info['index']])
            
            # Function calls
            elif isinstance(node, ast.Call):
                func_info = self._extract_function_info(node)
                if func_info:
                    patterns['function_calls'].append(func_info)
            
            # String comparisons for ticker filtering
            elif isinstance(node, ast.Compare):
                if self._is_ticker_comparison(node):
                    ticker = self._extract_ticker_value(node)
                    if ticker:
                        patterns['target_tickers'].add(ticker)
            
            # For loops over data
            elif isinstance(node, ast.For):
                if self._is_data_iteration(node):
                    patterns['loops_over_data'] = True
        
        # Detect strategy patterns from code analysis
        patterns.update(self._detect_strategy_patterns(strategy_code))
        
        # Convert sets to lists for JSON serialization
        patterns['accessed_columns'] = list(patterns['accessed_columns'])
        patterns['accessed_indices'] = list(patterns['accessed_indices'])
        patterns['target_tickers'] = list(patterns['target_tickers'])
        
        return patterns
    
    def _detect_strategy_patterns(self, strategy_code: str) -> Dict[str, bool]:
        """Detect high-level strategy patterns from code content"""
        
        patterns = {
            'gap_analysis': False,
            'sector_analysis': False,
            'technical_indicators': False,
            'percentile_analysis': False,
            'relative_performance': False,
            'fundamental_data': False,
            'multi_timeframe': False
        }
        
        code_lower = strategy_code.lower()
        
        # Gap analysis patterns
        if any(term in code_lower for term in ['gap', 'open_price', 'prev_close', 'previous_close']):
            patterns['gap_analysis'] = True
        
        # Sector analysis patterns
        if any(term in code_lower for term in ['sector', 'technology', 'fund_sector']):
            patterns['sector_analysis'] = True
        
        # Technical indicator patterns
        if any(term in code_lower for term in ['adr', 'macd', 'ema', 'sma', 'rsi', 'bollinger']):
            patterns['technical_indicators'] = True
        
        # Percentile analysis patterns
        if any(term in code_lower for term in ['percentile', 'sort', 'ranking', 'top_decile']):
            patterns['percentile_analysis'] = True
        
        # Relative performance patterns
        if any(term in code_lower for term in ['avgo', 'nvda', 'relative', 'vs', 'outperform']):
            patterns['relative_performance'] = True
        
        # Fundamental data patterns
        if any(term in code_lower for term in ['pe_ratio', 'market_cap', 'fund_', 'fundamental']):
            patterns['fundamental_data'] = True
        
        # Multi-timeframe patterns
        if any(term in code_lower for term in ['timeframe', '1min', '5min', 'daily', 'minute']):
            patterns['multi_timeframe'] = True
        
        return patterns
    
    def _is_numpy_array_access(self, node: ast.Subscript) -> bool:
        """Check if subscript is numpy array access like data[i, 0]"""
        if isinstance(node.value, ast.Name):
            return node.value.id in ['data', 'row', 'current_row', 'item']
        return False
    
    def _extract_array_index(self, node: ast.Subscript) -> Optional[Dict[str, Any]]:
        """Extract index from array access like data[i, 5]"""
        if isinstance(node.slice, ast.Tuple) and len(node.slice.elts) >= 2:
            # Handle data[i, column] format
            if isinstance(node.slice.elts[1], ast.Constant):
                return {
                    'array_name': node.value.id if isinstance(node.value, ast.Name) else 'unknown',
                    'index': node.slice.elts[1].value
                }
        elif isinstance(node.slice, ast.Constant) and isinstance(node.slice.value, int):
            return {
                'array_name': node.value.id if isinstance(node.value, ast.Name) else 'unknown',
                'index': node.slice.value
            }
        return None
    
    def _extract_function_info(self, node: ast.Call) -> Optional[Dict[str, Any]]:
        """Extract function call information"""
        func_info = {'function': '', 'module': '', 'args': []}
        
        if isinstance(node.func, ast.Attribute):
            func_info['function'] = node.func.attr
            if isinstance(node.func.value, ast.Name):
                func_info['module'] = node.func.value.id
        elif isinstance(node.func, ast.Name):
            func_info['function'] = node.func.id
        
        return func_info if func_info['function'] else None
    
    def _is_ticker_comparison(self, node: ast.Compare) -> bool:
        """Check if comparison involves ticker filtering"""
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
        elif isinstance(node.iter, ast.Call):
            if isinstance(node.iter.func, ast.Name) and node.iter.func.id == 'range':
                # Check if range uses data.shape[0]
                if node.iter.args:
                    arg = node.iter.args[0]
                    if isinstance(arg, ast.Attribute) and isinstance(arg.value, ast.Attribute):
                        if (isinstance(arg.value.value, ast.Name) and 
                            arg.value.value.id == 'data' and 
                            arg.value.attr == 'shape' and 
                            arg.attr == '0'):
                            return True
        return False
    
    def _analyze_usage_context(self, tree: ast.AST, data_accesses: Dict[str, Any]) -> Dict[str, Any]:
        """Analyze how data is used in the strategy"""
        
        return {
            'accessed_columns': data_accesses['accessed_columns'],
            'price_columns': [col for col in data_accesses['accessed_columns'] 
                            if col in ['open', 'high', 'low', 'close', 'volume']],
            'fundamental_columns': [col for col in data_accesses['accessed_columns'] 
                                  if col.startswith('fund_')],
            'filter_only_columns': [],  # Would need more sophisticated analysis
            'computation_columns': data_accesses['accessed_columns'],
            'target_tickers': data_accesses['target_tickers'],
            'has_calculations': len(data_accesses['computational_operations']) > 0,
            'strategy_patterns': {k: v for k, v in data_accesses.items() 
                                if k in ['gap_analysis', 'sector_analysis', 'technical_indicators', 
                                        'percentile_analysis', 'relative_performance']},
            'lookback_analysis': self._estimate_lookback_requirements(data_accesses)
        }
    
    def _estimate_lookback_requirements(self, data_accesses: Dict[str, Any]) -> Dict[str, Any]:
        """Estimate lookback requirements based on strategy patterns"""
        
        base_periods = 1
        
        if data_accesses.get('technical_indicators'):
            base_periods = max(base_periods, 26)  # MACD needs 26 days
        
        if data_accesses.get('gap_analysis'):
            base_periods = max(base_periods, 2)   # Need previous day
        
        if data_accesses.get('percentile_analysis'):
            base_periods = max(base_periods, 252) # Yearly analysis
        
        return {
            'base_periods': base_periods,
            'rolling_window_max': 0,
            'shift_operations_max': 1,
            'total_periods': base_periods
        }
    
    def _generate_mode_requirements(self, usage_context: Dict[str, Any], mode: str) -> Dict[str, Any]:
        """Generate mode-specific requirements"""
        
        if mode == 'screener':
            return {
                'timeframe': '1d',
                'periods': 1,
                'columns': list(set(usage_context['accessed_columns'] + ['ticker', 'date'])),
                'fundamentals': usage_context['fundamental_columns'],
                'target_tickers': usage_context['target_tickers'],
                'mode_optimization': 'screener_snapshot'
            }
        elif mode == 'alert':
            return {
                'timeframe': '1d',
                'periods': min(usage_context['lookback_analysis']['total_periods'], 30),
                'columns': list(set(usage_context['accessed_columns'] + ['ticker', 'date'])),
                'fundamentals': usage_context['fundamental_columns'],
                'target_tickers': usage_context['target_tickers'],
                'mode_optimization': 'alert_recent_window'
            }
        else:  # backtest
            return {
                'timeframe': '1d',
                'periods': usage_context['lookback_analysis']['total_periods'],
                'columns': list(set(usage_context['accessed_columns'] + ['ticker', 'date'])),
                'fundamentals': usage_context['fundamental_columns'],
                'target_tickers': usage_context['target_tickers'],
                'mode_optimization': 'backtest_time_series'
            }
    
    def _classify_complexity(self, usage_context: Dict[str, Any]) -> str:
        """Classify strategy complexity"""
        
        patterns = usage_context.get('strategy_patterns', {})
        num_patterns = sum(patterns.values())
        has_calculations = usage_context.get('has_calculations', False)
        
        if num_patterns == 0 and not has_calculations:
            return 'simple_filter'
        elif num_patterns <= 2:
            return 'numpy_optimized'
        else:
            return 'numpy_complex'
    
    def _select_loading_strategy(self, complexity: str, mode: str) -> str:
        """Select optimal data loading strategy"""
        
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


def test_complex_strategy_scenarios():
    """Test all complex strategy scenarios requested"""
    
    print("\n" + "="*80)
    print("STANDALONE AST PARSER TEST FOR COMPLEX STRATEGIES")
    print("="*80)
    
    analyzer = StandaloneStrategyAnalyzer()
    
    # Define all the complex strategy test cases
    test_strategies = [
        {
            'name': 'Gold Gap Up Analysis',
            'description': 'Get all times gold gapped up over 3% over the last year',
            'code': '''
def strategy(data):
    instances = []
    
    for i in range(data.shape[0]):
        ticker = data[i, 0]  # TICKER_COL = 0
        date = data[i, 1]    # DATE_COL = 1
        open_price = float(data[i, 2])   # OPEN_COL = 2
        close_price = float(data[i, 5])  # CLOSE_COL = 5
        
        # Filter for gold-related symbols
        if ticker not in ['GLD', 'GOLD', 'IAU', 'SGOL', 'GLDM']:
            continue
            
        # Find previous day's close
        prev_close = None
        for j in range(i-1, -1, -1):
            if data[j, 0] == ticker:
                prev_close = float(data[j, 5])
                break
        
        if prev_close is None:
            continue
            
        # Calculate gap percentage
        gap_percent = ((open_price - prev_close) / prev_close) * 100
        
        if gap_percent > 3.0:
            instances.append({
                'ticker': ticker,
                'date': str(date),
                'signal': True,
                'gap_percent': gap_percent
            })
    
    return instances
'''
        },
        {
            'name': 'Sector Performance Gap Analysis',
            'description': 'Get all instances when a stock whose sector was up more than 100% on the year gapped up more than 5%',
            'code': '''
def strategy(data):
    instances = []
    
    # Calculate sector performance
    sector_performance = {}
    ticker_sectors = {}
    
    for i in range(data.shape[0]):
        ticker = data[i, 0]
        sector = data[i, 11] if len(data[i]) > 11 else 'Unknown'
        close_price = float(data[i, 5])
        
        ticker_sectors[ticker] = sector
        if sector not in sector_performance:
            sector_performance[sector] = {'prices': []}
        sector_performance[sector]['prices'].append(close_price)
    
    # Calculate yearly sector returns
    sector_yearly_returns = {}
    for sector, data_dict in sector_performance.items():
        if len(data_dict['prices']) > 0:
            prices = data_dict['prices']
            yearly_return = ((prices[-1] - prices[0]) / prices[0]) * 100
            sector_yearly_returns[sector] = yearly_return
    
    # Find gap ups in strong sectors
    for i in range(data.shape[0]):
        ticker = data[i, 0]
        date = data[i, 1]
        open_price = float(data[i, 2])
        
        sector = ticker_sectors.get(ticker, 'Unknown')
        sector_return = sector_yearly_returns.get(sector, 0)
        
        if sector_return <= 100:
            continue
            
        # Find previous close and calculate gap
        prev_close = None
        for j in range(i-1, -1, -1):
            if data[j, 0] == ticker:
                prev_close = float(data[j, 5])
                break
        
        if prev_close is None:
            continue
            
        gap_percent = ((open_price - prev_close) / prev_close) * 100
        
        if gap_percent > 5.0:
            instances.append({
                'ticker': ticker,
                'date': str(date),
                'signal': True,
                'gap_percent': gap_percent,
                'sector': sector,
                'sector_yearly_return': sector_return
            })
    
    return instances
'''
        },
        {
            'name': 'Leading Stock Percentile Analysis',
            'description': 'Get the leading (>90 percentile price change) stocks on the year',
            'code': '''
def strategy(data):
    instances = []
    
    # Calculate yearly returns for all stocks
    ticker_returns = {}
    
    for i in range(data.shape[0]):
        ticker = data[i, 0]
        close_price = float(data[i, 5])
        
        if ticker not in ticker_returns:
            ticker_returns[ticker] = {'prices': []}
        ticker_returns[ticker]['prices'].append(close_price)
    
    # Calculate returns and percentiles
    yearly_returns = []
    ticker_return_map = {}
    
    for ticker, data_dict in ticker_returns.items():
        if len(data_dict['prices']) >= 2:
            prices = data_dict['prices']
            yearly_return = ((prices[-1] - prices[0]) / prices[0]) * 100
            yearly_returns.append(yearly_return)
            ticker_return_map[ticker] = yearly_return
    
    # Calculate 90th percentile threshold
    if len(yearly_returns) > 0:
        yearly_returns.sort()
        percentile_90_threshold = yearly_returns[int(len(yearly_returns) * 0.9)]
        
        # Find stocks above 90th percentile
        for ticker, return_pct in ticker_return_map.items():
            if return_pct >= percentile_90_threshold:
                instances.append({
                    'ticker': ticker,
                    'signal': True,
                    'yearly_return': return_pct,
                    'percentile_threshold': percentile_90_threshold,
                    'ranking': 'top_10_percent'
                })
    
    return instances
'''
        },
        {
            'name': 'Relative Performance Analysis',
            'description': 'Get all times AVGO was up more than NVDA on the day but then closed down more than NVDA',
            'code': '''
def strategy(data):
    instances = []
    
    # Group data by date to compare symbols
    date_data = {}
    
    for i in range(data.shape[0]):
        ticker = data[i, 0]
        date = str(data[i, 1])
        open_price = float(data[i, 2])
        close_price = float(data[i, 5])
        
        if date not in date_data:
            date_data[date] = {}
        
        date_data[date][ticker] = {
            'open': open_price,
            'close': close_price,
            'intraday_return': ((close_price - open_price) / open_price) * 100
        }
    
    # Compare AVGO vs NVDA performance
    for date, tickers in date_data.items():
        if 'AVGO' in tickers and 'NVDA' in tickers:
            avgo_data = tickers['AVGO']
            nvda_data = tickers['NVDA']
            
            avgo_intraday = avgo_data['intraday_return']
            nvda_intraday = nvda_data['intraday_return']
            
            # Check specific conditions
            avgo_outperformed_early = avgo_intraday > nvda_intraday
            both_negative = avgo_intraday < 0 and nvda_intraday < 0
            
            if avgo_outperformed_early and both_negative:
                instances.append({
                    'ticker': 'AVGO',
                    'date': date,
                    'signal': True,
                    'avgo_return': avgo_intraday,
                    'nvda_return': nvda_intraday,
                    'relative_performance': avgo_intraday - nvda_intraday
                })
    
    return instances
'''
        },
        {
            'name': 'Technical Indicator Analysis',
            'description': 'Get instances when a stock was up more than its ADR * 3 + its MACD value',
            'code': '''
def strategy(data):
    instances = []
    
    # Group by ticker for time series calculations
    ticker_data = {}
    
    for i in range(data.shape[0]):
        ticker = data[i, 0]
        date = data[i, 1]
        high = float(data[i, 3])
        low = float(data[i, 4])
        close = float(data[i, 5])
        
        if ticker not in ticker_data:
            ticker_data[ticker] = []
        
        ticker_data[ticker].append({
            'date': date,
            'high': high,
            'low': low,
            'close': close,
            'daily_range': high - low
        })
    
    # Calculate ADR and MACD for each ticker
    for ticker, data_list in ticker_data.items():
        if len(data_list) < 26:  # Need at least 26 days for MACD
            continue
            
        data_list.sort(key=lambda x: x['date'])
        
        # Calculate 14-day ADR (Average Daily Range)
        for i in range(14, len(data_list)):
            recent_ranges = [data_list[j]['daily_range'] for j in range(i-13, i+1)]
            adr = sum(recent_ranges) / len(recent_ranges)
            
            # Simple MACD calculation (12-day EMA - 26-day EMA)
            if i >= 26:
                closes = [data_list[j]['close'] for j in range(i-25, i+1)]
                ema_12 = sum(closes[-12:]) / 12
                ema_26 = sum(closes) / 26
                macd = ema_12 - ema_26
                
                # Get daily return
                current_close = data_list[i]['close']
                prev_close = data_list[i-1]['close']
                daily_return = ((current_close - prev_close) / prev_close) * 100
                
                # Check condition: daily return > (ADR * 3 + MACD)
                threshold = (adr * 3) + macd
                
                if daily_return > threshold:
                    instances.append({
                        'ticker': ticker,
                        'date': str(data_list[i]['date']),
                        'signal': True,
                        'daily_return': daily_return,
                        'adr': adr,
                        'macd': macd,
                        'threshold': threshold
                    })
    
    return instances
'''
        },
        {
            'name': 'Top Decile Technology Sector Analysis',
            'description': 'Show the top-decile stocks (10%) whose sector is Technology and whose 20-day change > sector average + 5%',
            'code': '''
def strategy(data):
    instances = []
    
    # Filter for Technology sector stocks
    tech_stocks = {}
    
    for i in range(data.shape[0]):
        ticker = data[i, 0]
        date = data[i, 1]
        close = float(data[i, 5])
        sector = data[i, 11] if len(data[i]) > 11 else 'Unknown'
        
        if sector == 'Technology':
            if ticker not in tech_stocks:
                tech_stocks[ticker] = []
            tech_stocks[ticker].append({'date': date, 'close': close})
    
    # Calculate 20-day returns
    tech_returns = []
    ticker_returns = {}
    
    for ticker, data_list in tech_stocks.items():
        data_list.sort(key=lambda x: x['date'])
        
        if len(data_list) >= 20:
            current_close = data_list[-1]['close']
            close_20_days_ago = data_list[-20]['close']
            return_20d = ((current_close - close_20_days_ago) / close_20_days_ago) * 100
            
            tech_returns.append(return_20d)
            ticker_returns[ticker] = return_20d
    
    # Calculate sector average and find top decile
    if tech_returns:
        sector_avg = sum(tech_returns) / len(tech_returns)
        threshold = sector_avg + 5.0
        
        tech_returns.sort(reverse=True)
        top_decile_threshold = tech_returns[int(len(tech_returns) * 0.1)] if tech_returns else 0
        
        for ticker, return_20d in ticker_returns.items():
            is_top_decile = return_20d >= top_decile_threshold
            beats_sector_plus_5 = return_20d > threshold
            
            if is_top_decile and beats_sector_plus_5:
                instances.append({
                    'ticker': ticker,
                    'signal': True,
                    'return_20d': return_20d,
                    'sector_avg': sector_avg,
                    'threshold': threshold,
                    'top_decile_threshold': top_decile_threshold,
                    'sector': 'Technology'
                })
    
    return instances
'''
        },
        {
            'name': 'Multi-Timeframe Gap Strategy',
            'description': 'Create a strategy for stocks that gap up more than 1.5x their ADR on daily, filter on 1min when they trade above opening range high',
            'code': '''
def strategy(data):
    instances = []
    
    # This would require multi-timeframe data
    # Simulating with daily data logic
    
    ticker_data = {}
    
    for i in range(data.shape[0]):
        ticker = data[i, 0]
        date = data[i, 1]
        open_price = float(data[i, 2])
        high = float(data[i, 3])
        low = float(data[i, 4])
        close = float(data[i, 5])
        
        if ticker not in ticker_data:
            ticker_data[ticker] = []
        
        ticker_data[ticker].append({
            'date': date,
            'open': open_price,
            'high': high,
            'low': low,
            'close': close,
            'daily_range': high - low
        })
    
    # Process each ticker
    for ticker, data_list in ticker_data.items():
        data_list.sort(key=lambda x: x['date'])
        
        if len(data_list) < 15:  # Need history for ADR
            continue
        
        # Calculate 14-day ADR
        for i in range(14, len(data_list)):
            recent_ranges = [data_list[j]['daily_range'] for j in range(i-13, i+1)]
            adr = sum(recent_ranges) / len(recent_ranges)
            
            current_open = data_list[i]['open']
            current_high = data_list[i]['high']
            prev_close = data_list[i-1]['close']
            
            # Calculate gap
            gap_amount = current_open - prev_close
            gap_vs_adr = gap_amount / adr if adr > 0 else 0
            
            # Check if gap up > 1.5x ADR
            if gap_vs_adr > 1.5:
                # Simulate intraday condition
                opening_range_high = current_open + (adr * 0.1)
                trades_above_range = current_high > opening_range_high
                
                if trades_above_range:
                    instances.append({
                        'ticker': ticker,
                        'date': str(data_list[i]['date']),
                        'signal': True,
                        'gap_amount': gap_amount,
                        'adr': adr,
                        'gap_vs_adr_ratio': gap_vs_adr,
                        'opening_range_high': opening_range_high,
                        'day_high': current_high
                    })
    
    return instances
'''
        }
    ]
    
    # Test each strategy
    results = {}
    total_success = 0
    
    for i, strategy in enumerate(test_strategies, 1):
        print(f"\n{'='*60}")
        print(f"TEST {i}: {strategy['name']}")
        print(f"{'='*60}")
        print(f"📋 Description: {strategy['description']}")
        
        try:
            # Analyze for different modes
            modes = ['backtest', 'screener', 'alert']
            strategy_results = {}
            
            for mode in modes:
                print(f"\n🔍 Analyzing for {mode.upper()} mode...")
                
                analysis = analyzer.analyze_strategy_code(strategy['code'], mode)
                strategy_results[mode] = analysis
                
                if analysis['success']:
                    print(f"   ✅ Analysis successful")
                    print(f"   📊 Complexity: {analysis['strategy_complexity']}")
                    print(f"   🎯 Loading Strategy: {analysis['loading_strategy']}")
                    print(f"   📋 Required Columns: {len(analysis['data_requirements']['columns'])}")
                    print(f"   ⏱️ Periods: {analysis['data_requirements']['periods']}")
                    print(f"   🎨 Mode Optimization: {analysis['data_requirements']['mode_optimization']}")
                    
                    # Show detected patterns
                    patterns = analysis['usage_context']['strategy_patterns']
                    detected = [k for k, v in patterns.items() if v]
                    if detected:
                        print(f"   🔍 Detected Patterns: {', '.join(detected)}")
                    
                    target_tickers = analysis['data_requirements'].get('target_tickers', [])
                    if target_tickers:
                        print(f"   🎯 Target Tickers: {target_tickers}")
                        
                else:
                    print(f"   ❌ Analysis failed: {analysis.get('error', 'Unknown error')}")
            
            # Overall success for this strategy
            all_modes_successful = all(result['success'] for result in strategy_results.values())
            results[strategy['name']] = {
                'success': all_modes_successful,
                'results_by_mode': strategy_results
            }
            
            if all_modes_successful:
                total_success += 1
                print(f"\n✅ {strategy['name']}: ALL MODES PASSED")
            else:
                print(f"\n❌ {strategy['name']}: SOME MODES FAILED")
            
        except Exception as e:
            print(f"\n❌ {strategy['name']}: ERROR - {e}")
            results[strategy['name']] = {'success': False, 'error': str(e)}
    
    # Print final summary
    print("\n" + "🏆" + "="*79)
    print("🏆 STANDALONE AST PARSER TEST SUMMARY")
    print("🏆" + "="*79)
    
    total_strategies = len(test_strategies)
    success_rate = (total_success / total_strategies) * 100
    
    print(f"📊 Total Strategies Tested: {total_strategies}")
    print(f"✅ Strategies Passed: {total_success}")
    print(f"❌ Strategies Failed: {total_strategies - total_success}")
    print(f"📈 Success Rate: {success_rate:.1f}%")
    
    print(f"\n📋 Strategy Results:")
    for name, result in results.items():
        status = "✅ PASS" if result.get('success') else "❌ FAIL"
        print(f"  {status} {name}")
    
    if success_rate >= 90:
        print("\n🎉 Excellent! AST parser is working correctly for complex strategies.")
    elif success_rate >= 75:
        print("\n👍 Good performance with minor issues.")
    else:
        print("\n⚠️ Issues detected - needs investigation.")
    
    return results


if __name__ == "__main__":
    test_complex_strategy_scenarios()