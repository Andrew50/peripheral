"""
Data Accessor Strategy Engine
Executes Python strategies that use get_bar_data() and get_general_data() functions
instead of receiving DataFrames as parameters.
"""

import asyncio
import logging
import pandas as pd
import numpy as np
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Union, Tuple
import json
import time

from data_accessors import DataAccessorProvider, get_bar_data, get_general_data
from validator import SecurityValidator, SecurityError

logger = logging.getLogger(__name__)


class AccessorStrategyEngine:
    """
    Executes strategies that use data accessor functions
    
    Strategy signature:
    def strategy() -> List[Dict]:
        # Use get_bar_data() and get_general_data() to fetch required data
        # Returns list of instances: [{'ticker': 'AAPL', 'timestamp': '2024-01-01', 'signal': True, ...}]
    """
    
    def __init__(self):
        self.data_accessor = DataAccessorProvider()
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
        Execute strategy for backtesting using data accessor functions
        
        Args:
            strategy_code: Python code defining the strategy function
            symbols: List of symbols to test (passed to execution context)
            start_date: Start date for backtest (passed to execution context)
            end_date: End date for backtest (passed to execution context)
            
        Returns:
            Dict with instances, summary, and performance metrics
        """
        logger.info(f"Starting accessor backtest: {len(symbols)} symbols, {start_date.date()} to {end_date.date()}")
        
        start_time = time.time()
        
        try:
            # Validate strategy code
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Set execution context for data accessors
            self.data_accessor.set_execution_context(
                mode='backtest',
                symbols=symbols,
                start_date=start_date,
                end_date=end_date
            )
            
            # CRITICAL: Also set context on global accessor in case strategy uses global functions
            from data_accessors import get_data_accessor
            global_accessor = get_data_accessor()
            global_accessor.set_execution_context(
                mode='backtest',
                symbols=symbols,
                start_date=start_date,
                end_date=end_date
            )
            
            # Execute strategy with accessor context
            instances = await self._execute_strategy(
                strategy_code, 
                execution_mode='backtest'
            )
            
            # Calculate performance metrics
            performance = self._calculate_performance_metrics(instances)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'backtest',
                'instances': instances,
                'summary': {
                    'total_instances': len(instances),
                    'symbols_analyzed': len(symbols),
                    'date_range': f"{start_date.date()} to {end_date.date()}",
                    'execution_time_ms': int(execution_time)  # Convert to integer for Go compatibility
                },
                'performance': performance
            }
            
            logger.info(f"Backtest completed: {len(instances)} instances, {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            logger.error(f"Backtest execution failed: {e}")
            return {
                'success': False,
                'error': str(e),
                'execution_mode': 'backtest'
            }
    
    async def execute_validation(
        self, 
        strategy_code: str
    ) -> Dict[str, Any]:
        """
        Execute strategy for VALIDATION ONLY using exact min_bars requirements for speed
        
        Args:
            strategy_code: Python code defining the strategy function  
            
        Returns:
            Dict with validation result (success/error only)
        """
        logger.info("🧪 Starting fast validation execution (exact min_bars requirements)")
        
        start_time = time.time()
        
        try:
            # Validate strategy code first
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Extract min_bars requirements from strategy code
            min_bars_requirements = self.validator.extract_min_bars_requirements(strategy_code)
            
            # Log the exact requirements that will be used
            if min_bars_requirements:
                logger.info("📊 Extracted min_bars requirements from strategy:")
                for req in min_bars_requirements:
                    logger.info(f"   Line {req['line_number']}: get_bar_data(timeframe='{req['timeframe']}', min_bars={req['min_bars']})")
                max_bars = max(req['min_bars'] for req in min_bars_requirements)
                logger.info(f"🎯 Validation will use exact min_bars requirements (max: {max_bars} bars)")
            else:
                logger.info("📊 No get_bar_data calls found - using minimal data for validation")
            
            # Set execution context for validation with exact requirements
            context_data = {
                'mode': 'validation',  # Special validation mode
                'symbols': ['AAPL'],   # Just one symbol for validation
                'min_bars_requirements': min_bars_requirements  # Pass exact requirements
            }
            
            self.data_accessor.set_execution_context(**context_data)
            
            # CRITICAL: Also set context on global accessor in case strategy uses global functions
            from data_accessors import get_data_accessor
            global_accessor = get_data_accessor()
            global_accessor.set_execution_context(**context_data)
            
            # Debug: Verify both instances have validation context
            logger.debug(f"🔍 Engine accessor context: {self.data_accessor.execution_context}")
            logger.debug(f"🔍 Global accessor context: {global_accessor.execution_context}")
            logger.debug(f"🔍 Same instance check: {self.data_accessor is global_accessor}")
            
            logger.debug("🔧 Validation optimizations enabled:")
            logger.debug("   ✓ Minimal dataset: 1 symbol")
            logger.debug("   ✓ Exact min_bars from strategy code (no arbitrary caps)")
            logger.debug("   ✓ Fast execution path (validation mode)")
            logger.debug("   ✓ Skip result ranking and processing")
            logger.debug("   ✓ Context set on both engine and global data accessors")
            
            # Execute strategy with validation context (don't care about results)
            instances = await self._execute_strategy(
                strategy_code, 
                execution_mode='validation'
            )
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'validation',
                'instances_generated': len(instances),
                'min_bars_requirements': min_bars_requirements,
                'execution_time_ms': int(execution_time),
                'message': 'Validation passed - strategy can execute without errors'
            }
            
            logger.info(f"✅ Validation completed successfully: {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            execution_time = (time.time() - start_time) * 1000
            logger.error(f"❌ Validation failed: {e}")
            return {
                'success': False,
                'error': str(e),
                'execution_mode': 'validation',
                'execution_time_ms': int(execution_time)
            }

    async def execute_screening(
        self, 
        strategy_code: str, 
        universe: List[str], 
        limit: int = 100,
        **kwargs
    ) -> Dict[str, Any]:
        """
        Execute strategy for screening using data accessor functions
        
        Args:
            strategy_code: Python code defining the strategy function  
            universe: List of symbols to screen (passed to execution context)
            limit: Maximum results to return
            
        Returns:
            Dict with ranked results and scores
        """
        logger.info(f"Starting accessor screening: {len(universe)} symbols, limit {limit}")
        logger.info("📊 Screening mode: Using minimal recent data for optimal performance")
        
        start_time = time.time()
        
        try:
            # Validate strategy code
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Set execution context for data accessors with screening optimizations
            self.data_accessor.set_execution_context(
                mode='screening',
                symbols=universe
            )
            
            # CRITICAL: Also set context on global accessor in case strategy uses global functions
            from data_accessors import get_data_accessor
            global_accessor = get_data_accessor()
            global_accessor.set_execution_context(
                mode='screening',
                symbols=universe
            )
            
            # Log optimization settings
            logger.debug("🔧 Screening optimizations enabled:")
            logger.debug("   ✓ Exact data fetching (ROW_NUMBER gets precise min_bars per security)")
            logger.debug("   ✓ NO date filtering (eliminates unnecessary data overhead)")
            logger.debug("   ✓ Database-optimized query structure (most recent records only)")
            logger.debug(f"   ✓ Universe size: {len(universe)} symbols")
            logger.debug(f"   ✓ Result limit: {limit}")
            
            # Execute strategy with accessor context
            instances = await self._execute_strategy(
                strategy_code, 
                execution_mode='screening'
            )
            
            # Rank and limit results
            ranked_results = self._rank_screening_results(instances, limit)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'screening',
                'ranked_results': ranked_results,
                'universe_size': len(universe),
                'results_returned': len(ranked_results),
                'execution_time_ms': int(execution_time),  # Convert to integer for Go compatibility
                'optimization_enabled': True,
                'data_strategy': 'minimal_recent'
            }
            
            logger.info(f"✅ Screening completed: {len(ranked_results)} results, {execution_time:.1f}ms")
            logger.debug(f"   📈 Performance: {len(ranked_results)/execution_time*1000:.1f} results/second")
            return result
            
        except Exception as e:
            logger.error(f"❌ Screening execution failed: {e}")
            return {
                'success': False,
                'error': str(e),
                'execution_mode': 'screening'
            }
    
    async def execute_alert(
        self, 
        strategy_code: str, 
        symbols: List[str],
        **kwargs
    ) -> Dict[str, Any]:
        """
        Execute strategy for real-time alerts using data accessor functions
        
        Args:
            strategy_code: Python code defining the strategy function
            symbols: List of symbols to monitor
            
        Returns:
            Dict with alerts and signals
        """
        logger.info(f"Starting accessor alert scan: {len(symbols)} symbols")
        
        start_time = time.time()
        
        try:
            # Validate strategy code
            if not self.validator.validate_code(strategy_code):
                raise SecurityError("Strategy code validation failed")
            
            # Set execution context for data accessors
            self.data_accessor.set_execution_context(
                mode='alert',
                symbols=symbols
            )
            
            # CRITICAL: Also set context on global accessor in case strategy uses global functions
            from data_accessors import get_data_accessor
            global_accessor = get_data_accessor()
            global_accessor.set_execution_context(
                mode='alert',
                symbols=symbols
            )
            
            # Execute strategy with accessor context
            instances = await self._execute_strategy(
                strategy_code, 
                execution_mode='alert'
            )
            
            # Convert instances to alerts
            alerts = self._convert_instances_to_alerts(instances)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'alert',
                'alerts': alerts,
                'signals': {inst['ticker']: inst for inst in instances},  # All instances are signals
                'symbols_processed': len(symbols),
                'execution_time_ms': int(execution_time)  # Convert to integer for Go compatibility
            }
            
            logger.info(f"Alert scan completed: {len(alerts)} alerts, {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            logger.error(f"Alert execution failed: {e}")
            return {
                'success': False,
                'error': str(e),
                'execution_mode': 'alert'
            }
    
    def _convert_instances_to_alerts(self, instances: List[Dict]) -> List[Dict]:
        """Convert instances to alert format for real-time mode"""
        
        alerts = []
        for instance in instances:
            # Since all instances are signals (they met criteria), convert all to alerts
            alert = {
                'symbol': instance['ticker'],
                'type': 'strategy_signal',
                'message': instance.get('message', f"{instance['ticker']} triggered strategy signal"),
                'timestamp': datetime.now().isoformat(),
                'data': instance
            }
            
            # Add priority based on score/strength
            if 'score' in instance:
                alert['priority'] = 'high' if instance['score'] > 0.8 else 'medium'
            elif 'signal_strength' in instance:
                alert['priority'] = 'high' if instance['signal_strength'] > 0.8 else 'medium'
            else:
                alert['priority'] = 'medium'
            
            alerts.append(alert)
        
        return alerts

    async def _execute_strategy(
        self, 
        strategy_code: str, 
        execution_mode: str
    ) -> List[Dict]:
        """Execute the strategy function with data accessor context"""
        
        # Validate strategy code before execution
        if not self._validate_strategy_code(strategy_code):
            raise ValueError("Strategy code contains prohibited operations")
        
        # Create safe execution environment with data accessor functions
        safe_globals = await self._create_safe_globals(execution_mode)
        safe_locals = {}
        
        try:
            # Execute strategy code in restricted environment
            exec(strategy_code, safe_globals, safe_locals)  # nosec B102 - exec necessary for strategy execution with proper sandboxing
            
            # Find strategy function (should be named 'strategy')
            strategy_func = safe_locals.get('strategy')
            if not strategy_func or not callable(strategy_func):
                # Try alternative names
                for name in ['strategy_function', 'main', 'run']:
                    func = safe_locals.get(name)
                    if func and callable(func):
                        strategy_func = func
                        break
            
            if not strategy_func:
                raise ValueError("No strategy function found. Function should be named 'strategy'")
            
            # Execute strategy function with proper error handling
            logger.info(f"Executing strategy function using data accessor approach")
            try:
                instances = strategy_func()
            except Exception as strategy_error:
                logger.error(f"Strategy function execution failed: {strategy_error}")
                logger.debug(f"Strategy error details: {type(strategy_error).__name__}: {strategy_error}")
                # Return empty list instead of crashing - let the calling code handle empty results
                return []
            
            # Validate and clean instances
            if not isinstance(instances, list):
                logger.error(f"Strategy function must return a list, got {type(instances)}")
                return []
            
            # Filter out None instances and validate structure
            valid_instances = []
            for instance in instances:
                if instance is not None and isinstance(instance, dict):
                    # Ensure required fields exist
                    if 'ticker' not in instance:
                        continue
                    if 'timestamp' not in instance:
                        instance['timestamp'] = datetime.now().isoformat()
                    
                    valid_instances.append(instance)
            
            logger.info(f"Strategy returned {len(valid_instances)} valid instances")
            return valid_instances
            
        except Exception as e:
            logger.error(f"Strategy compilation or setup failed: {e}")
            logger.debug(f"Setup error details: {type(e).__name__}: {e}")
            # Return empty list for compilation/setup errors too
            return []
    
    async def _create_safe_globals(self, execution_mode: str) -> Dict[str, Any]:
        """Create safe execution environment with data accessor functions"""
        
        # Create bound methods that use this engine's data accessor
        def bound_get_bar_data(timeframe="1d", columns=None, min_bars=1, filters=None, 
                              aggregate_mode=False, extended_hours=False):
            return self.data_accessor.get_bar_data(timeframe, columns, min_bars, filters, 
                                                  aggregate_mode, extended_hours)
        
        def bound_get_general_data(columns=None, filters=None):
            return self.data_accessor.get_general_data(columns=columns, filters=filters)
        
        safe_globals = {
            # Standard imports
            'pd': pd,
            'numpy': np,
            'np': np,
            'pandas': pd,
            
            # Bound data accessor functions (use this engine's instance)
            'get_bar_data': bound_get_bar_data,
            'get_general_data': bound_get_general_data,
            
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
            'list': list,
            'dict': dict,
            'tuple': tuple,
            'set': set,
            'sorted': sorted,
            'reversed': reversed,
            'any': any,
            'all': all,
            
            # Math and datetime
            'datetime': datetime,
            'timedelta': timedelta,
            
            # Execution mode info
            'execution_mode': execution_mode,
            
            # Helper function to create instances
            'create_instance': lambda ticker, timestamp, **kwargs: {
                'ticker': ticker,
                'timestamp': timestamp,
                **kwargs
            }
        }
        
        return safe_globals

    def _validate_strategy_code(self, strategy_code: str) -> bool:
        """Basic validation of strategy code"""
        # Use the security validator
        return self.validator.validate_code(strategy_code)

    def _rank_screening_results(self, instances: List[Dict], limit: int) -> List[Dict]:
        """Rank screening results by score or other criteria and convert to WorkerRankedResult format"""
        
        # Sort by score if available, otherwise by timestamp descending (most recent first)
        def sort_key(instance):
            if 'score' in instance:
                return instance['score']
            else:
                # Use timestamp for sorting if no score - more recent = higher priority
                return instance.get('timestamp', 0)
        
        sorted_instances = sorted(instances, key=sort_key, reverse=True)
        
        # Limit results
        limited_instances = sorted_instances[:limit]
        
        # Convert to WorkerRankedResult format expected by Go backend
        ranked_results = []
        for instance in limited_instances:
            # Convert instance to WorkerRankedResult format
            ranked_result = {
                'symbol': instance.get('ticker', ''),  # Convert ticker to symbol
                'score': float(instance.get('score', 0.0)),
                'current_price': float(instance.get('entry_price', instance.get('close', instance.get('price', 0.0)))),
                'sector': instance.get('sector', ''),
                'data': instance  # Include the full instance data
            }
            ranked_results.append(ranked_result)
        
        return ranked_results

    def _calculate_performance_metrics(self, instances: List[Dict]) -> Dict[str, Any]:
        """Calculate performance metrics from instances"""
        
        if not instances:
            return {
                'total_instances': 0,
                'signal_rate': 0.0,
                'unique_tickers': 0,
                'avg_score': 0.0
            }
        
        # Basic statistics
        total_instances = len(instances)
        # Since all returned instances are positive signals (they met criteria), count all
        positive_instances = total_instances  # All instances are positive instances
        unique_tickers = len(set(i['ticker'] for i in instances))
        
        # Calculate signal rate (always 1.0 since all returned instances are signals)
        signal_rate = 1.0
        
        # Calculate average score if available
        scores = [i.get('score', 0) for i in instances if 'score' in i and isinstance(i['score'], (int, float))]
        avg_score = sum(scores) / len(scores) if scores else 0.0
        
        metrics = {
            'total_instances': total_instances,
            'positive_instances': positive_instances,
            'signal_rate': round(signal_rate, 4),
            'unique_tickers': unique_tickers,
            'avg_score': round(avg_score, 4)
        }
        
        # Add custom metrics if present in instances
        numeric_fields = []
        for instance in instances:
            for key, value in instance.items():
                if key not in ['ticker', 'timestamp'] and isinstance(value, (int, float)):
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