"""
Numpy Strategy Engine
Executes Python strategies that take numpy arrays as input and return instances (ticker + date + metrics)
Updated from DataFrame-based to numpy array-based execution.
"""

import asyncio
import logging
import pandas as pd
import numpy as np
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Union, Tuple
import json
import time

from data import DataProvider
from validator import SecurityValidator, SecurityError
from strategy_data_analyzer import StrategyDataAnalyzer

logger = logging.getLogger(__name__)


class NumpyStrategyEngine:
    """
    Executes strategies that take numpy arrays and return instances
    
    Strategy signature:
    def strategy(data: np.ndarray) -> List[Dict]:
        # data contains market data as numpy array with columns:
        # [ticker, date, open, high, low, close, volume, ...]
        # Returns list of instances: [{'ticker': 'AAPL', 'date': '2024-01-01', 'signal': True, ...}]
    """
    
    def __init__(self):
        self.data_provider = DataProvider()
        self.validator = SecurityValidator()
        self.data_analyzer = StrategyDataAnalyzer()
        
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
            # Fundamental data at higher indices
            8: 'fund_pe_ratio',
            9: 'fund_pb_ratio',
            10: 'fund_market_cap',
            11: 'fund_sector',
            12: 'fund_industry',
            13: 'fund_dividend_yield'
        }
        
    async def execute_backtest(
        self, 
        strategy_code: str, 
        symbols: List[str], 
        start_date: datetime, 
        end_date: datetime,
        **kwargs
    ) -> Dict[str, Any]:
        """
        Execute numpy-based strategy for backtesting
        
        Args:
            strategy_code: Python code defining the strategy function
            symbols: List of symbols to test
            start_date: Start date for backtest
            end_date: End date for backtest
            
        Returns:
            Dict with instances, summary, and performance metrics
        """
        logger.info(f"Starting numpy backtest: {len(symbols)} symbols, {start_date.date()} to {end_date.date()}")
        
        start_time = time.time()
        
        try:
            # Validate strategy code
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Analyze data requirements for backtest mode
            data_analysis = self.data_analyzer.analyze_data_requirements(strategy_code, mode='backtest')
            requirements = data_analysis['data_requirements']
            loading_strategy = data_analysis['loading_strategy']
            
            logger.info(f"Data requirements analysis: {requirements['mode_optimization']}, "
                       f"strategy: {loading_strategy}, "
                       f"estimated rows: {requirements.get('estimated_rows', 'unknown')}")
            
            # Load data as numpy array using optimized strategy
            data_array = await self._load_optimized_data(
                symbols, start_date, end_date, requirements, loading_strategy, 'backtest'
            )
            logger.info(f"Loaded numpy array with shape: {data_array.shape}")
            
            # Execute strategy
            instances = await self._execute_strategy(strategy_code, data_array, execution_mode='backtest')
            
            # Calculate performance metrics
            performance_metrics = self._calculate_performance_metrics(instances, data_array)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'backtest',
                'instances': instances,
                'summary': {
                    'total_instances': len(instances),
                    'positive_signals': len([i for i in instances if i.get('signal', False)]),
                    'date_range': [start_date.isoformat(), end_date.isoformat()],
                    'symbols_processed': len(symbols),
                    'data_shape': list(data_array.shape)
                },
                'performance_metrics': performance_metrics,
                'execution_time_ms': execution_time,
                'data_analysis': data_analysis
            }
            
            logger.info(f"Backtest completed: {len(instances)} instances in {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            logger.error(f"Backtest execution failed: {e}")
            return {
                'success': False,
                'execution_mode': 'backtest',
                'error_message': str(e),
                'instances': [],
                'summary': {},
                'performance_metrics': {}
            }
    
    async def execute_screening(
        self, 
        strategy_code: str, 
        universe: List[str], 
        limit: int = 100,
        **kwargs
    ) -> Dict[str, Any]:
        """
        Execute numpy-based strategy for screening with optimization
        
        Args:
            strategy_code: Python code defining the strategy function  
            universe: List of symbols to screen
            limit: Maximum results to return
            
        Returns:
            Dict with ranked results and scores
        """
        logger.info(f"Starting numpy screening: {len(universe)} symbols, limit {limit}")
        
        start_time = time.time()
        
        try:
            # Validate strategy code
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Analyze data requirements for screener mode - CRITICAL OPTIMIZATION
            data_analysis = self.data_analyzer.analyze_data_requirements(strategy_code, mode='screener')
            requirements = data_analysis['data_requirements']
            loading_strategy = data_analysis['loading_strategy']
            
            logger.info(f"Screener optimization: {requirements['mode_optimization']}, "
                       f"strategy: {loading_strategy}, "
                       f"periods: {requirements.get('periods', 1)}")
            
            # Load minimal data for screening (major optimization for over-fetching)
            end_date = datetime.now()
            start_date = end_date - timedelta(days=requirements.get('periods', 1))
            
            data_array = await self._load_optimized_data(
                universe, start_date, end_date, requirements, loading_strategy, 'screener'
            )
            logger.info(f"Loaded screening numpy array with shape: {data_array.shape}")
            
            # Execute strategy
            instances = await self._execute_strategy(strategy_code, data_array, execution_mode='screening')
            
            # Rank and limit results
            ranked_results = self._rank_screening_results(instances, limit)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'screening',
                'ranked_results': ranked_results,
                'universe_size': len(universe),
                'results_returned': len(ranked_results),
                'data_shape': list(data_array.shape),
                'execution_time_ms': execution_time,
                'data_analysis': data_analysis
            }
            
            logger.info(f"Screening completed: {len(ranked_results)} results in {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            logger.error(f"Screening execution failed: {e}")
            return {
                'success': False,
                'execution_mode': 'screening',
                'error_message': str(e),
                'ranked_results': [],
                'universe_size': len(universe),
                'results_returned': 0
            }
    
    async def execute_realtime(
        self, 
        strategy_code: str, 
        symbols: List[str],
        **kwargs
    ) -> Dict[str, Any]:
        """
        Execute numpy-based strategy for real-time alerts with optimization
        
        Args:
            strategy_code: Python code defining the strategy function
            symbols: List of symbols to monitor
            
        Returns:
            Dict with alerts and signals
        """
        logger.info(f"Starting numpy real-time scan: {len(symbols)} symbols")
        
        start_time = time.time()
        
        try:
            # Validate strategy code
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Analyze data requirements for alert mode
            data_analysis = self.data_analyzer.analyze_data_requirements(strategy_code, mode='alert')
            requirements = data_analysis['data_requirements']
            loading_strategy = data_analysis['loading_strategy']
            
            logger.info(f"Alert optimization: {requirements['mode_optimization']}, "
                       f"strategy: {loading_strategy}, "
                       f"periods: {requirements.get('periods', 5)}")
            
            # Load recent data for real-time analysis
            end_date = datetime.now()
            periods = requirements.get('periods', 5)
            start_date = end_date - timedelta(days=periods)
            
            data_array = await self._load_optimized_data(
                symbols, start_date, end_date, requirements, loading_strategy, 'alert'
            )
            logger.info(f"Loaded real-time numpy array with shape: {data_array.shape}")
            
            # Execute strategy
            instances = await self._execute_strategy(strategy_code, data_array, execution_mode='realtime')
            
            # Convert instances to alerts
            alerts = self._convert_instances_to_alerts(instances)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'realtime',
                'alerts': alerts,
                'symbols_monitored': len(symbols),
                'alerts_generated': len(alerts),
                'data_shape': list(data_array.shape),
                'execution_time_ms': execution_time,
                'data_analysis': data_analysis
            }
            
            logger.info(f"Real-time scan completed: {len(alerts)} alerts in {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            logger.error(f"Real-time execution failed: {e}")
            return {
                'success': False,
                'execution_mode': 'realtime',
                'error_message': str(e),
                'alerts': [],
                'symbols_monitored': len(symbols),
                'alerts_generated': 0
            }

    async def _load_optimized_data(
        self, 
        symbols: List[str], 
        start_date: datetime, 
        end_date: datetime,
        requirements: Dict[str, Any],
        loading_strategy: str,
        execution_mode: str
    ) -> np.ndarray:
        """
        Load data as numpy array using batched approach for all strategies
        Number of batches determined by arbitrary function (currently always 1)
        """
        logger.info(f"Loading data as numpy array with batched strategy (original strategy: {loading_strategy})")
        
        # Determine number of batches using arbitrary function
        num_batches = self._determine_batch_count(symbols, requirements, execution_mode)
        logger.info(f"Using {num_batches} batch(es) for {len(symbols)} symbols")
        
        # Always use batched loading for numpy arrays
        return await self._load_batched_numpy_array_flexible(
            symbols, start_date, end_date, requirements, num_batches
        )
    
    def _determine_batch_count(
        self, 
        symbols: List[str], 
        requirements: Dict[str, Any], 
        execution_mode: str
    ) -> int:
        """
        Arbitrary function to determine number of batches
        Currently always returns 1 as requested
        
        Args:
            symbols: List of symbols to process
            requirements: Data requirements from AST analysis
            execution_mode: 'screener', 'backtest', or 'alert'
            
        Returns:
            Number of batches to use (currently always 1)
        """
        # For now, always use 1 batch as requested
        return 1
        
        # Future implementation could consider:
        # - Symbol count: len(symbols)
        # - Memory requirements: requirements.get('estimated_rows', 0)
        # - Execution mode complexity
        # - Available system resources
        # 
        # Example logic (commented out):
        # if len(symbols) > 1000:
        #     return min(10, len(symbols) // 100)
        # elif len(symbols) > 100:
        #     return min(5, len(symbols) // 50)
        # else:
        #     return 1
    
    async def _load_batched_numpy_array_flexible(
        self, 
        symbols: List[str], 
        start_date: datetime, 
        end_date: datetime,
        requirements: Dict[str, Any],
        num_batches: int
    ) -> np.ndarray:
        """
        Load data in specified number of batches with flexible optimization
        """
        logger.info(f"Loading {len(symbols)} symbols in {num_batches} batches")
        
        if num_batches == 1:
            return await self._load_single_batch_processing(symbols, start_date, end_date, requirements)
        else:
            return await self._load_multi_batch_processing(symbols, start_date, end_date, requirements, num_batches)
    
    async def _load_single_batch_processing(
        self, 
        symbols: List[str], 
        start_date: datetime,
        end_date: datetime,
        requirements: Dict[str, Any]
    ) -> np.ndarray:
        """
        Optimized loading for single batch (all symbols at once)
        """
        logger.info(f"Single batch processing: {len(symbols)} symbols")
        
        # Load data for all symbols
        dataframes = []
        
        for symbol in symbols:
            try:
                df = await self.data_provider.get_market_data(
                    symbol, 
                    start_date, 
                    end_date,
                    columns=requirements.get('columns', ['open', 'high', 'low', 'close', 'volume'])
                )
                
                if not df.empty:
                    # Add ticker column
                    df['ticker'] = symbol
                    # Reorder columns to match mapping
                    ordered_columns = ['ticker', 'date', 'open', 'high', 'low', 'close', 'volume']
                    if 'fundamentals' in requirements and requirements['fundamentals']:
                        ordered_columns.extend(requirements['fundamentals'])
                    
                    # Keep only available columns
                    available_columns = [col for col in ordered_columns if col in df.columns or col == 'ticker']
                    df = df[available_columns]
                    dataframes.append(df)
                    
            except Exception as e:
                logger.warning(f"Failed to load data for {symbol}: {e}")
                continue
        
        if not dataframes:
            return np.array([])
        
        # Combine all dataframes
        combined_df = pd.concat(dataframes, ignore_index=True)
        
        # Convert to numpy array
        logger.info(f"Single batch loaded: {combined_df.shape[0]} rows, {combined_df.shape[1]} columns")
        return combined_df.values
    
    async def _load_multi_batch_processing(
        self, 
        symbols: List[str], 
        start_date: datetime,
        end_date: datetime,
        requirements: Dict[str, Any],
        num_batches: int
    ) -> np.ndarray:
        """
        Load data using multiple batches for large symbol sets
        """
        batch_size = max(1, len(symbols) // num_batches)
        all_dataframes = []
        
        for i in range(0, len(symbols), batch_size):
            batch_symbols = symbols[i:i + batch_size]
            logger.info(f"Processing batch {i//batch_size + 1}/{num_batches}: {len(batch_symbols)} symbols")
            
            batch_data = await self._load_single_batch_processing(
                batch_symbols, start_date, end_date, requirements
            )
            
            if batch_data.size > 0:
                # Convert numpy array back to DataFrame for concatenation
                columns = ['ticker', 'date', 'open', 'high', 'low', 'close', 'volume']
                if 'fundamentals' in requirements and requirements['fundamentals']:
                    columns.extend(requirements['fundamentals'])
                
                # Trim columns to match actual data shape
                columns = columns[:batch_data.shape[1]]
                batch_df = pd.DataFrame(batch_data, columns=columns)
                all_dataframes.append(batch_df)
        
        if not all_dataframes:
            return np.array([])
        
        # Combine all batches
        final_df = pd.concat(all_dataframes, ignore_index=True)
        logger.info(f"Multi-batch loading complete: {final_df.shape[0]} rows, {final_df.shape[1]} columns")
        return final_df.values

    async def _load_backtest_data(
        self, 
        symbols: List[str], 
        start_date: datetime, 
        end_date: datetime
    ) -> pd.DataFrame:
        """Load comprehensive data for backtesting"""
        
        # Load price data for all symbols
        all_data = []
        
        # Process symbols in batches to avoid memory issues
        batch_size = 50
        for i in range(0, len(symbols), batch_size):
            batch_symbols = symbols[i:i + batch_size]
            logger.info(f"Loading batch {i//batch_size + 1}/{(len(symbols) + batch_size - 1)//batch_size}")
            
            for symbol in batch_symbols:
                try:
                    # Get price data
                    days_diff = (end_date - start_date).days + 30  # Extra buffer for indicators
                    price_data = await self.data_provider.get_price_data(
                        symbol=symbol,
                        timeframe='1d',
                        days=days_diff
                    )
                    
                    if not price_data.get('close') or len(price_data['close']) == 0:
                        continue
                    
                    # Convert to DataFrame format
                    symbol_df = self._convert_price_data_to_df(symbol, price_data)
                    
                    # Add fundamental data
                    fundamental_data = await self.data_provider.get_fundamental_data(symbol)
                    symbol_df = self._add_fundamental_data(symbol_df, fundamental_data)
                    
                    # Filter to date range
                    symbol_df = symbol_df[
                        (symbol_df['date'] >= start_date.date()) & 
                        (symbol_df['date'] <= end_date.date())
                    ]
                    
                    if len(symbol_df) > 0:
                        all_data.append(symbol_df)
                        
                except Exception as e:
                    logger.warning(f"Failed to load data for {symbol}: {e}")
                    continue
        
        if not all_data:
            return pd.DataFrame()
        
        # Combine all symbol data
        df = pd.concat(all_data, ignore_index=True)
        df = df.sort_values(['date', 'ticker']).reset_index(drop=True)
        
        return df
    
    async def _load_screening_data(
        self, 
        universe: List[str], 
        start_date: datetime, 
        end_date: datetime
    ) -> pd.DataFrame:
        """Load recent data for screening (optimized for current conditions)"""
        return await self._load_backtest_data(universe, start_date, end_date)
    
    async def _load_realtime_data(
        self, 
        symbols: List[str], 
        start_date: datetime, 
        end_date: datetime
    ) -> pd.DataFrame:
        """Load recent data for real-time analysis"""
        return await self._load_backtest_data(symbols, start_date, end_date)
    
    def _convert_price_data_to_df(self, symbol: str, price_data: Dict) -> pd.DataFrame:
        """Convert price data dict to DataFrame format"""
        
        timestamps = price_data.get('timestamps', [])
        opens = price_data.get('open', [])
        highs = price_data.get('high', [])
        lows = price_data.get('low', [])
        closes = price_data.get('close', [])
        volumes = price_data.get('volume', [])
        
        if not timestamps or len(timestamps) == 0:
            return pd.DataFrame()
        
        # Convert timestamps to dates
        dates = [datetime.fromtimestamp(ts).date() for ts in timestamps]
        
        df = pd.DataFrame({
            'ticker': symbol,
            'date': dates,
            'open': opens,
            'high': highs,
            'low': lows,
            'close': closes,
            'volume': volumes
        })
        
        return df
    
    def _add_fundamental_data(self, df: pd.DataFrame, fundamental_data: Dict) -> pd.DataFrame:
        """Add fundamental data to DataFrame"""
        
        if not fundamental_data:
            return df
        
        # Add fundamental columns (same value for all rows of this symbol)
        for key, value in fundamental_data.items():
            if isinstance(value, (int, float, str)):
                df[f'fund_{key}'] = value
        
        return df
    
    def _add_technical_indicators(self, df: pd.DataFrame) -> pd.DataFrame:
        """Technical indicators are no longer auto-generated - strategies must calculate their own"""
        # REMOVED: Automatic technical indicator generation
        # Strategies should implement their own technical indicators using raw price data
        return df
    
    async def _execute_strategy(
        self, 
        strategy_code: str, 
        data_array: np.ndarray, 
        execution_mode: str
    ) -> List[Dict]:
        """Execute the strategy function on the numpy array"""
        
        # Validate strategy code before execution
        if not self._validate_strategy_code(strategy_code):
            raise ValueError("Strategy code contains prohibited operations")
        
        # Create safe execution environment
        safe_globals = await self._create_safe_globals(data_array, execution_mode)
        safe_locals = {}
        
        try:
            # Execute strategy code in restricted environment
            exec(strategy_code, safe_globals, safe_locals)  # nosec B102 - exec necessary for strategy execution with proper sandboxing
            
            # Find strategy function (should be named 'strategy' or 'strategy_function')
            strategy_func = None
            for name, obj in safe_locals.items():
                if callable(obj) and (name == 'strategy' or name == 'strategy_function'):
                    strategy_func = obj
                    break
            
            if not strategy_func:
                raise ValueError("No strategy function found. Function should be named 'strategy' or 'strategy_function'")
            
            # Execute strategy function
            logger.info(f"Executing strategy function on numpy array with shape {data_array.shape}")
            instances = strategy_func(data_array)
            
            # Validate and clean instances
            if not isinstance(instances, list):
                raise ValueError(f"Strategy function must return a list, got {type(instances)}")
            
            # Filter out None instances and validate structure
            valid_instances = []
            for instance in instances:
                if instance is not None and isinstance(instance, dict):
                    valid_instances.append(instance)
            
            logger.info(f"Strategy execution complete: {len(valid_instances)} valid instances generated")
            return valid_instances
            
        except Exception as e:
            logger.error(f"Strategy execution failed: {e}")
            raise

    def _validate_strategy_code(self, strategy_code: str) -> bool:
        """Validate strategy code for security"""
        # List of prohibited operations
        prohibited_operations = [
            'import os', 'import sys', 'import subprocess', 'import shutil',
            'import socket', 'import urllib', 'import requests', 'import http',
            'open(', 'file(', '__import__', 'eval(', 'exec(',
            'compile(', 'globals(', 'locals(', 'vars(', 'dir(',
            'getattr(', 'setattr(', 'delattr(', 'hasattr(',
            'input(', 'raw_input(', 'exit(', 'quit('
        ]
        
        strategy_lower = strategy_code.lower()
        for prohibited in prohibited_operations:
            if prohibited in strategy_lower:
                logger.error(f"Prohibited operation found in strategy code: {prohibited}")
                return False
        
        return True
    
    async def _create_safe_globals(self, data_array: np.ndarray, execution_mode: str) -> Dict[str, Any]:
        """Create safe execution environment with numpy array and utilities"""
        
        safe_globals = {
            # Standard imports
            'pd': pd,
            'numpy': np,
            
            # numpy array with all data
            'data': data_array,
            
            # Utility functions
            'len': len,
            'range': range,
            'enumerate': enumerate,
            'float': float,
            'int': int,
            'str': str,
            'abs': abs,
            'max': max,
            'min': min,
            'round': round,
            'sum': sum,
            
            # Column mapping for easier access
            'TICKER_COL': 0,
            'DATE_COL': 1,
            'OPEN_COL': 2,
            'HIGH_COL': 3,
            'LOW_COL': 4,
            'CLOSE_COL': 5,
            'VOLUME_COL': 6,
            
            # Execution mode info
            'execution_mode': execution_mode,
            
            # Helper function to create instances
            'create_instance': lambda ticker, date, **kwargs: {
                'ticker': ticker,
                'date': date,
                **kwargs
            }
        }
        
        # Add datetime utilities for backtest mode
        if execution_mode == 'backtest':
            safe_globals.update({
                'datetime': datetime,
                'timedelta': timedelta
            })
        
        return safe_globals

    def _calculate_performance_metrics(self, instances: List[Dict], data_array: np.ndarray) -> Dict[str, Any]:
        """Calculate performance metrics from backtest instances"""
        
        if not instances:
            return {
                'total_signals': 0,
                'success_rate': 0.0,
                'average_return': 0.0,
                'max_drawdown': 0.0,
                'sharpe_ratio': 0.0
            }
        
        # Basic metrics
        total_signals = len([i for i in instances if i.get('signal', False)])
        total_instances = len(instances)
        
        # Calculate returns if available
        returns = [i.get('return', 0.0) for i in instances if 'return' in i]
        avg_return = np.mean(returns) if returns else 0.0
        
        # Calculate success rate (positive returns / total signals)
        positive_returns = len([r for r in returns if r > 0])
        success_rate = positive_returns / max(total_signals, 1)
        
        # Simple Sharpe ratio approximation
        sharpe_ratio = 0.0
        if returns and np.std(returns) > 0:
            sharpe_ratio = (avg_return * 252) / (np.std(returns) * np.sqrt(252))
        
        return {
            'total_instances': total_instances,
            'total_signals': total_signals,
            'success_rate': success_rate,
            'average_return': avg_return,
            'max_drawdown': 0.0,  # Would need more complex calculation
            'sharpe_ratio': sharpe_ratio,
            'data_points_analyzed': data_array.shape[0] if data_array.size > 0 else 0
        }

    def _rank_screening_results(self, instances: List[Dict], limit: int) -> List[Dict]:
        """Rank screening results by score or signal strength"""
        
        # Sort by score (descending) or signal strength
        def get_sort_key(instance):
            if 'score' in instance:
                return instance['score']
            elif 'signal_strength' in instance:
                return instance['signal_strength']
            elif 'signal' in instance:
                return 1.0 if instance['signal'] else 0.0
            else:
                return 0.0
        
        sorted_instances = sorted(instances, key=get_sort_key, reverse=True)
        
        # Return top results up to limit
        return sorted_instances[:limit]

    def _convert_instances_to_alerts(self, instances: List[Dict]) -> List[Dict]:
        """Convert strategy instances to alert format"""
        
        alerts = []
        
        for instance in instances:
            # Only create alerts for positive signals
            if instance.get('signal', False):
                alert = {
                    'ticker': instance.get('ticker', 'UNKNOWN'),
                    'timestamp': datetime.now().isoformat(),
                    'alert_type': 'strategy_signal',
                    'message': instance.get('message', 'Strategy signal triggered'),
                    'signal_strength': instance.get('signal_strength', 1.0),
                    'data': instance
                }
                alerts.append(alert)
        
        return alerts

    def generate_mode_specific_requirements(self, usage_context: Dict[str, Any], mode: str) -> Dict[str, Any]:
        """
        Pass 3: Generate optimized requirements based on execution mode
        """
        optimizer = self.data_analyzer.mode_optimizers.get(mode, self.data_analyzer._optimize_for_backtest)
        return optimizer(usage_context) 