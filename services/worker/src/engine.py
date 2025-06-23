"""
DataFrame Strategy Engine
Executes Python strategies that take DataFrames as input and return instances (ticker + date + metrics)
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

logger = logging.getLogger(__name__)


class DataFrameStrategyEngine:
    """
    Executes strategies that take DataFrames and return instances
    
    Strategy signature:
    def strategy_function(df: pd.DataFrame) -> List[Dict]:
        # df contains all required data (OHLCV, fundamentals, etc.)
        # Returns list of instances: [{'ticker': 'AAPL', 'date': '2024-01-01', 'signal': True, ...}]
    """
    
    def __init__(self):
        self.data_provider = DataProvider()
        self.validator = SecurityValidator()
        
    async def execute_backtest(
        self, 
        strategy_code: str, 
        symbols: List[str], 
        start_date: datetime, 
        end_date: datetime,
        **kwargs
    ) -> Dict[str, Any]:
        """
        Execute DataFrame-based strategy for backtesting
        
        Args:
            strategy_code: Python code defining the strategy function
            symbols: List of symbols to test
            start_date: Start date for backtest
            end_date: End date for backtest
            
        Returns:
            Dict with instances, summary, and performance metrics
        """
        logger.info(f"Starting DataFrame backtest: {len(symbols)} symbols, {start_date.date()} to {end_date.date()}")
        
        start_time = time.time()
        
        try:
            # Validate strategy code
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Load data into DataFrame
            df = await self._load_backtest_data(symbols, start_date, end_date)
            logger.info(f"Loaded DataFrame with shape: {df.shape}")
            
            # Execute strategy
            instances = await self._execute_strategy(strategy_code, df, execution_mode='backtest')
            
            # Calculate performance metrics
            performance_metrics = self._calculate_performance_metrics(instances, df)
            
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
                    'data_shape': df.shape
                },
                'performance_metrics': performance_metrics,
                'execution_time_ms': execution_time
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
        Execute DataFrame-based strategy for screening
        
        Args:
            strategy_code: Python code defining the strategy function  
            universe: List of symbols to screen
            limit: Maximum results to return
            
        Returns:
            Dict with ranked results and scores
        """
        logger.info(f"Starting DataFrame screening: {len(universe)} symbols, limit {limit}")
        
        start_time = time.time()
        
        try:
            # Validate strategy code
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Load recent data for screening (last 30 days by default)
            end_date = datetime.now()
            start_date = end_date - timedelta(days=30)
            df = await self._load_screening_data(universe, start_date, end_date)
            logger.info(f"Loaded screening DataFrame with shape: {df.shape}")
            
            # Execute strategy
            instances = await self._execute_strategy(strategy_code, df, execution_mode='screening')
            
            # Rank and limit results
            ranked_results = self._rank_screening_results(instances, limit)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'screening',
                'ranked_results': ranked_results,
                'universe_size': len(universe),
                'results_returned': len(ranked_results),
                'data_shape': df.shape,
                'execution_time_ms': execution_time
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
        Execute DataFrame-based strategy for real-time alerts
        
        Args:
            strategy_code: Python code defining the strategy function
            symbols: List of symbols to monitor
            
        Returns:
            Dict with alerts and signals
        """
        logger.info(f"Starting DataFrame real-time scan: {len(symbols)} symbols")
        
        start_time = time.time()
        
        try:
            # Validate strategy code
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Load recent data for real-time analysis
            end_date = datetime.now()
            start_date = end_date - timedelta(days=5)  # Last 5 days for context
            df = await self._load_realtime_data(symbols, start_date, end_date)
            logger.info(f"Loaded real-time DataFrame with shape: {df.shape}")
            
            # Execute strategy
            instances = await self._execute_strategy(strategy_code, df, execution_mode='realtime')
            
            # Convert instances to alerts
            alerts = self._convert_instances_to_alerts(instances)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'realtime',
                'alerts': alerts,
                'signals': {inst['ticker']: inst for inst in instances if inst.get('signal', False)},
                'symbols_processed': len(symbols),
                'data_shape': df.shape,
                'execution_time_ms': execution_time
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
                'signals': {}
            }
    
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
                    
                    # Add technical indicators
                    symbol_df = self._add_technical_indicators(symbol_df)
                    
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
        """Add common technical indicators to DataFrame"""
        
        if len(df) < 20:  # Need minimum data for indicators
            return df
        
        df = df.copy()
        
        # Price-based indicators
        df['returns'] = df['close'].pct_change()
        df['log_returns'] = np.log(df['close'] / df['close'].shift(1))
        
        # Moving averages
        df['sma_5'] = df['close'].rolling(5).mean()
        df['sma_10'] = df['close'].rolling(10).mean()
        df['sma_20'] = df['close'].rolling(20).mean()
        df['sma_50'] = df['close'].rolling(50).mean()
        
        # Exponential moving averages
        df['ema_12'] = df['close'].ewm(span=12).mean()
        df['ema_26'] = df['close'].ewm(span=26).mean()
        
        # MACD
        df['macd'] = df['ema_12'] - df['ema_26']
        df['macd_signal'] = df['macd'].ewm(span=9).mean()
        df['macd_histogram'] = df['macd'] - df['macd_signal']
        
        # RSI
        delta = df['close'].diff()
        gain = (delta.where(delta > 0, 0)).rolling(window=14).mean()
        loss = (-delta.where(delta < 0, 0)).rolling(window=14).mean()
        rs = gain / loss
        df['rsi'] = 100 - (100 / (1 + rs))
        
        # Bollinger Bands
        df['bb_middle'] = df['close'].rolling(20).mean()
        bb_std = df['close'].rolling(20).std()
        df['bb_upper'] = df['bb_middle'] + (bb_std * 2)
        df['bb_lower'] = df['bb_middle'] - (bb_std * 2)
        df['bb_width'] = df['bb_upper'] - df['bb_lower']
        df['bb_position'] = (df['close'] - df['bb_lower']) / df['bb_width']
        
        # Volume indicators
        df['volume_sma'] = df['volume'].rolling(20).mean()
        df['volume_ratio'] = df['volume'] / df['volume_sma']
        
        # Price gaps
        df['gap'] = (df['open'] - df['close'].shift(1)) / df['close'].shift(1)
        df['gap_pct'] = df['gap'] * 100
        
        # True Range and ATR
        df['tr1'] = df['high'] - df['low']
        df['tr2'] = abs(df['high'] - df['close'].shift(1))
        df['tr3'] = abs(df['low'] - df['close'].shift(1))
        df['true_range'] = df[['tr1', 'tr2', 'tr3']].max(axis=1)
        df['atr'] = df['true_range'].rolling(14).mean()
        df = df.drop(['tr1', 'tr2', 'tr3'], axis=1)
        
        # Price position within daily range
        df['price_position'] = (df['close'] - df['low']) / (df['high'] - df['low'])
        
        return df
    
    async def _execute_strategy(
        self, 
        strategy_code: str, 
        df: pd.DataFrame, 
        execution_mode: str
    ) -> List[Dict]:
        """Execute the strategy function on the DataFrame"""
        
        # Create safe execution environment
        safe_globals = await self._create_safe_globals(df, execution_mode)
        safe_locals = {}
        
        try:
            # Compile and execute strategy code
            compiled_code = compile(strategy_code, "<strategy>", "exec")
            exec(compiled_code, safe_globals, safe_locals)  # nosec B102
            
            # Look for strategy function
            strategy_func = None
            for name, obj in safe_locals.items():
                if callable(obj) and not name.startswith('_'):
                    strategy_func = obj
                    break
            
            if not strategy_func:
                raise ValueError("No callable strategy function found in code")
            
            # Execute strategy function
            logger.info(f"Executing strategy function on DataFrame with shape {df.shape}")
            instances = strategy_func(df)
            
            # Validate and clean instances
            if not isinstance(instances, list):
                raise ValueError("Strategy function must return a list of instances")
            
            # Ensure all instances have required fields
            cleaned_instances = []
            for instance in instances:
                if isinstance(instance, dict):
                    # Ensure required fields exist
                    if 'ticker' not in instance:
                        continue
                    if 'date' not in instance:
                        instance['date'] = datetime.now().date().isoformat()
                    
                    cleaned_instances.append(instance)
            
            logger.info(f"Strategy returned {len(cleaned_instances)} instances")
            return cleaned_instances
            
        except Exception as e:
            logger.error(f"Strategy execution failed: {e}")
            raise
    
    async def _create_safe_globals(self, df: pd.DataFrame, execution_mode: str) -> Dict[str, Any]:
        """Create safe execution environment with DataFrame and utilities"""
        
        safe_globals = {
            '__builtins__': {
                'len': len,
                'range': range,
                'enumerate': enumerate,
                'zip': zip,
                'list': list,
                'dict': dict,
                'tuple': tuple,
                'set': set,
                'str': str,
                'int': int,
                'float': float,
                'bool': bool,
                'abs': abs,
                'min': min,
                'max': max,
                'sum': sum,
                'round': round,
                'sorted': sorted,
                'any': any,
                'all': all,
            },
            # Data science libraries
            'pd': pd,
            'np': np,
            'pandas': pd,
            'numpy': np,
            
            # DataFrame with all data
            'df': df,
            
            # Utility functions
            'datetime': datetime,
            'timedelta': timedelta,
            
            # Logging
            'log': lambda msg: logger.info(f"Strategy: {msg}"),
        }
        
        return safe_globals
    
    def _calculate_performance_metrics(self, instances: List[Dict], df: pd.DataFrame) -> Dict[str, Any]:
        """Calculate performance metrics from backtest instances"""
        
        if not instances:
            return {}
        
        # Basic statistics
        total_instances = len(instances)
        positive_signals = len([i for i in instances if i.get('signal', False)])
        
        # Get unique symbols and dates
        unique_symbols = len(set(i['ticker'] for i in instances))
        unique_dates = len(set(i['date'] for i in instances))
        
        # Calculate signal rate
        signal_rate = positive_signals / total_instances if total_instances > 0 else 0
        
        metrics = {
            'total_instances': total_instances,
            'positive_signals': positive_signals,
            'signal_rate': round(signal_rate, 4),
            'unique_symbols': unique_symbols,
            'unique_dates': unique_dates,
            'avg_instances_per_symbol': round(total_instances / unique_symbols, 2) if unique_symbols > 0 else 0,
            'avg_instances_per_date': round(total_instances / unique_dates, 2) if unique_dates > 0 else 0
        }
        
        # Add custom metrics if present in instances
        numeric_fields = []
        for instance in instances:
            for key, value in instance.items():
                if key not in ['ticker', 'date', 'signal'] and isinstance(value, (int, float)):
                    if key not in numeric_fields:
                        numeric_fields.append(key)
        
        for field in numeric_fields:
            values = [i.get(field) for i in instances if field in i and isinstance(i[field], (int, float))]
            if values:
                metrics[f'{field}_mean'] = round(np.mean(values), 4)
                metrics[f'{field}_std'] = round(np.std(values), 4)
                metrics[f'{field}_min'] = round(min(values), 4)
                metrics[f'{field}_max'] = round(max(values), 4)
        
        return metrics
    
    def _rank_screening_results(self, instances: List[Dict], limit: int) -> List[Dict]:
        """Rank screening results by score or signal strength"""
        
        if not instances:
            return []
        
        # Sort by score if available, otherwise by signal
        def sort_key(instance):
            if 'score' in instance:
                return instance['score']
            elif 'signal_strength' in instance:
                return instance['signal_strength']
            elif 'signal' in instance:
                return 1 if instance['signal'] else 0
            else:
                return 0
        
        sorted_instances = sorted(instances, key=sort_key, reverse=True)
        return sorted_instances[:limit]
    
    def _convert_instances_to_alerts(self, instances: List[Dict]) -> List[Dict]:
        """Convert instances to alert format for real-time mode"""
        
        alerts = []
        for instance in instances:
            if instance.get('signal', False):
                alert = {
                    'symbol': instance['ticker'],
                    'type': 'strategy_signal',
                    'message': f"{instance['ticker']} triggered strategy signal",
                    'timestamp': datetime.now().isoformat(),
                    'data': instance
                }
                
                # Add custom message if provided
                if 'message' in instance:
                    alert['message'] = instance['message']
                
                # Add priority based on score/strength
                if 'score' in instance:
                    alert['priority'] = 'high' if instance['score'] > 0.8 else 'medium'
                elif 'signal_strength' in instance:
                    alert['priority'] = 'high' if instance['signal_strength'] > 0.8 else 'medium'
                else:
                    alert['priority'] = 'medium'
                
                alerts.append(alert)
        
        return alerts 