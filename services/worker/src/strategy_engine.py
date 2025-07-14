"""
Data Accessor Strategy Engine
Executes Python strategies that use get_bar_data() and get_general_data() functions
instead of receiving DataFrames as parameters.
"""

import asyncio
import logging
import pandas as pd
import numpy as np
from datetime import datetime as dt, timedelta
import datetime
from typing import Any, Dict, List, Optional, Union, Tuple
import json
import time
import io
import contextlib
import plotly
import traceback
import linecache
import sys 
import math
import ast
from plotlyToMatlab import plotly_to_matplotlib_png

from validator import SecurityValidator, SecurityError

logger = logging.getLogger(__name__)


class TrackedList(list):
    """List that tracks total instances across all TrackedList objects"""
    _global_instance_count = 0
    _max_instances = 15000
    _limit_reached = False
    
    @classmethod
    def reset_counter(cls, max_instances=15000):
        """Reset global counter for new strategy execution"""
        cls._global_instance_count = 0
        cls._max_instances = max_instances
        cls._limit_reached = False
        logger.debug(f"Reset instance counter, max_instances: {max_instances}")
    
    @classmethod
    def is_limit_reached(cls):
        """Check if the instance limit was reached during execution"""
        return cls._limit_reached
    
    def _check_and_update_limit(self, additional_count=1):
        """Check if adding items would exceed limit and update counter if not"""
        new_count = TrackedList._global_instance_count + additional_count
        
        if new_count > TrackedList._max_instances:
            # Set flag that limit was reached but don't raise exception
            if not TrackedList._limit_reached:
                TrackedList._limit_reached = True
                logger.warning(f"Instance limit reached: {TrackedList._global_instance_count}/{TrackedList._max_instances}. Stopping instance collection.")
            return False  # Don't add more instances
        
        # Log warning when approaching limit (90% threshold)
        if new_count > TrackedList._max_instances * 0.9 and not TrackedList._limit_reached:
            logger.warning(f"Approaching instance limit: {new_count}/{TrackedList._max_instances}")
        
        TrackedList._global_instance_count = new_count
        return True  # OK to add instances
    
    def append(self, item):
        if self._check_and_update_limit(1):
            super().append(item)
    
    def extend(self, items):
        items_list = list(items) if not isinstance(items, list) else items
        if len(items_list) == 0:
            return
        
        if self._check_and_update_limit(len(items_list)):
            super().extend(items_list)
    
    def insert(self, index, item):
        if self._check_and_update_limit(1):
            super().insert(index, item)
    
    def __iadd__(self, other):
        other_list = list(other) if not isinstance(other, list) else other
        if len(other_list) == 0:
            return self
        
        if self._check_and_update_limit(len(other_list)):
            return super().__iadd__(other_list)
        return self


class AccessorStrategyEngine:
    """
    Executes strategies that use data accessor functions
    
    Strategy signature:
    def strategy() -> List[Dict]:
        # Use get_bar_data() and get_general_data() to fetch required data
        # Returns list of instances: [{'ticker': 'AAPL', 'timestamp': '2024-01-01', 'signal': True, ...}]
    """
    
    def __init__(self):
        # Use the global singleton instead of creating a new instance
        from data_accessors import get_data_accessor
        self.data_accessor = get_data_accessor()
        self.validator = SecurityValidator()
        
    async def execute_backtest(
        self, 
        strategy_id: int,
        strategy_code: str, 
        symbols: List[str], 
        start_date: dt, 
        end_date: dt,
        max_instances: int = 15000,
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
            
            # Set execution context for data accessors
            self.data_accessor.set_execution_context(
                mode='backtest',
                symbols=symbols,
                start_date=start_date,
                end_date=end_date
            )
            
            # Execute strategy with accessor context
            instances, strategy_prints, strategy_plots, response_images, error = await self._execute_strategy(
                strategy_code, 
                execution_mode='backtest',
                max_instances=max_instances,
                strategy_id=strategy_id
            )
            if error: 
                raise error
            # Calculate performance metrics
            
            execution_time = (time.time() - start_time) * 1000
            
            # Create date range array for summary compatibility
            date_range = [start_date.strftime('%Y-%m-%d'), end_date.strftime('%Y-%m-%d')]
            
            result = {
                'success': True,
                'instances': instances,
                'symbols_processed': len(symbols),
                'strategy_prints': strategy_prints,
                'strategy_plots': strategy_plots,
                'response_images': response_images,
                'instance_limit_reached': TrackedList.is_limit_reached(),
                'summary': {
                    'total_instances': len(instances),
                    'symbols_processed': len(symbols),
                    'date_range': date_range,
                },
            }
            
            logger.info(f"Backtest completed: {len(instances)} instances, {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            # Get detailed error information
            error_info = self._get_detailed_error_info(e, strategy_code)
            detailed_error_msg = self._format_detailed_error(error_info)
            
            logger.error(f"Backtest execution failed: {e}")
            logger.error(detailed_error_msg)
            
            return {
                'success': False,
                'error': str(e),
                'error_details': error_info,
                'execution_mode': 'backtest',
                'strategy_prints': '',
                'strategy_plots': []
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
            getBarDataFunctionCalls = self.extract_get_bar_data_calls(strategy_code)
            logger.info(f"📋 Extracted {len(getBarDataFunctionCalls)} get_bar_data calls: {getBarDataFunctionCalls}")
            tickersInStrategyCode = self.get_all_tickers_from_calls(getBarDataFunctionCalls)
            logger.info(f"📋 Extracted {len(tickersInStrategyCode)} tickers from get_bar_data calls: {tickersInStrategyCode}")

            symbolsForValidation = tickersInStrategyCode if len(tickersInStrategyCode) <= 10 else tickersInStrategyCode[:10]
            symbolsForValidation = symbolsForValidation if symbolsForValidation else ['AAPL']  # Default if empty
            
            max_timeframe, max_timeframe_min_bars = self.getMaxTimeframeAndMinBars(getBarDataFunctionCalls)
            # Log the exact requirements that will be used
            logger.info(f"🎯 Validation will use exact min_bars requirements (timeframe: {max_timeframe}, min_bars: {max_timeframe_min_bars})")
            
            # Calculate start date based on timeframe and min_bars (convert to days and round up)
            end_date = dt.now()
            if max_timeframe and max_timeframe_min_bars > 0:
                # Parse timeframe and convert to days
                if max_timeframe.endswith('d'):
                    days_back = int(max_timeframe[:-1]) * max_timeframe_min_bars
                elif max_timeframe.endswith('h'):
                    hours_back = int(max_timeframe[:-1]) * max_timeframe_min_bars
                    days_back = math.ceil(hours_back / 24)  # Round up to nearest day
                elif max_timeframe.endswith('m'):
                    minutes_back = int(max_timeframe[:-1]) * max_timeframe_min_bars
                    days_back = math.ceil(minutes_back / (24 * 60))  # Round up to nearest day
                else:
                    days_back = 30  # Default fallback
                start_date = end_date - timedelta(days=days_back)
            else:
                start_date = end_date - timedelta(days=30)  # Default fallback
            
            # Set execution context for validation with exact requirements
            context_data = {
                'mode': 'validation',  # Special validation mode
                'symbols': symbolsForValidation,   # Use extracted tickers for more realistic validation
                'start_date': start_date,
                'end_date': end_date
            }
            
            self.data_accessor.set_execution_context(**context_data)
            
            validationMaxInstances = 100
            # Execute strategy with validation context (don't care about results)
            instances, _, _, _, error = await self._execute_strategy(
                strategy_code, 
                execution_mode='validation',
                max_instances=validationMaxInstances # Lower limit for validation
            )
            if error: 
                raise error
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'validation',
                'instances_generated': len(instances),
                'instance_limit_reached': TrackedList.is_limit_reached(),
                'max_instances_configured': validationMaxInstances,  # Validation uses lower limit
                'execution_time_ms': int(execution_time),
                'message': 'Validation passed - strategy can execute without errors'
            }
            
            logger.info(f"✅ Validation completed successfully: {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            execution_time = (time.time() - start_time) * 1000
            
            # Get detailed error information
            error_info = self._get_detailed_error_info(e, strategy_code)
            detailed_error_msg = self._format_detailed_error(error_info)
            
            logger.error(f"❌ Validation failed: {e}")
            logger.error(detailed_error_msg)
            
            return {
                'success': False,
                'error': str(e),
                'error_details': error_info,
                'execution_mode': 'validation',
                'strategy_prints': '',
                'execution_time_ms': int(execution_time)
            }

    async def execute_screening(
        self, 
        strategy_code: str, 
        universe: List[str], 
        limit: int = 100,
        max_instances: int = 15000,
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
            
            # Set execution context for data accessors with screening optimizations
            self.data_accessor.set_execution_context(
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
            instances, _, _, _, error = await self._execute_strategy(
                strategy_code, 
                execution_mode='screening',
                max_instances=max_instances
            )
            if error: 
                raise error
            # Rank and limit results
            ranked_results = self._rank_screening_results(instances, limit)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'screening',
                'ranked_results': ranked_results,
                'universe_size': len(universe),
                'results_returned': len(ranked_results),
                'instance_limit_reached': TrackedList.is_limit_reached(),
                'max_instances_configured': max_instances,
                'execution_time_ms': int(execution_time),  # Convert to integer for Go compatibility
                'optimization_enabled': True,
                'data_strategy': 'minimal_recent'
            }
            
            logger.info(f"✅ Screening completed: {len(ranked_results)} results, {execution_time:.1f}ms")
            logger.debug(f"   📈 Performance: {len(ranked_results)/execution_time*1000:.1f} results/second")
            return result
            
        except Exception as e:
            # Get detailed error information
            error_info = self._get_detailed_error_info(e, strategy_code)
            detailed_error_msg = self._format_detailed_error(error_info)
            
            logger.error(f"❌ Screening execution failed: {e}")
            logger.error(detailed_error_msg)
            
            return {
                'success': False,
                'error': str(e),
                'error_details': error_info,
                'execution_mode': 'screening'
            }
    
    async def execute_alert(
        self, 
        strategy_code: str, 
        symbols: List[str],
        max_instances: int = 15000,
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
            
            # Set execution context for data accessors
            self.data_accessor.set_execution_context(
                mode='alert',
                symbols=symbols
            )
            
            # Execute strategy with accessor context
            instances, _, _, _, error = await self._execute_strategy(
                strategy_code, 
                execution_mode='alert',
                max_instances=max_instances
            )
            if error: 
                raise error
            # Convert instances to alerts
            alerts = self._convert_instances_to_alerts(instances)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'alert',
                'alerts': alerts,
                'signals': {inst['ticker']: inst for inst in instances},  # All instances are signals
                'symbols_processed': len(symbols),
                'instance_limit_reached': TrackedList.is_limit_reached(),
                'max_instances_configured': max_instances,
                'execution_time_ms': int(execution_time)  # Convert to integer for Go compatibility
            }
            
            logger.info(f"Alert scan completed: {len(alerts)} alerts, {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            # Get detailed error information
            error_info = self._get_detailed_error_info(e, strategy_code)
            detailed_error_msg = self._format_detailed_error(error_info)
            
            logger.error(f"Alert execution failed: {e}")
            logger.error(detailed_error_msg)
            
            return {
                'success': False,
                'error': str(e),
                'error_details': error_info,
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
                'timestamp': dt.now().isoformat(),
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
        execution_mode: str,
        max_instances: int = 15000,
        strategy_id: int = None
    ) -> Tuple[List[Dict], str, List[Dict], List[Dict], Exception]:
        """Execute the strategy function with data accessor context"""
        
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
            
            # Reset instance counter for this execution
            TrackedList.reset_counter(max_instances=max_instances)
            
            # Execute strategy function with proper error handling and stdout capture
            logger.info(f"Executing strategy function using data accessor approach")
            strategy_prints = ""
            try:
                # Capture stdout and plots during strategy execution
                stdout_buffer = io.StringIO()
                with contextlib.redirect_stdout(stdout_buffer), self._plotly_capture_context(strategy_id):
                    instances = strategy_func()
                strategy_prints = stdout_buffer.getvalue()
                
                # Check if instance limit was reached during execution
                if TrackedList.is_limit_reached():
                    logger.warning(f"Strategy execution completed with instance limit reached. Total instances: {TrackedList._global_instance_count}")
                
            except Exception as strategy_error:
                # Get detailed error information
                error_info = self._get_detailed_error_info(strategy_error, strategy_code)
                detailed_error_msg = self._format_detailed_error(error_info)
                
                logger.error(f"Strategy function execution failed: {strategy_error}")
                logger.error(detailed_error_msg)
                
                # Return empty list and any captured output instead of crashing
                return [], "", [], [], strategy_error
            
            # Validate and clean instances
            if not isinstance(instances, list):
                logger.error(f"Strategy function must return a list, got {type(instances)}")
                return [], "", [], [], "Strategy function must return a list"
            
            # Filter out None instances and validate structure
            valid_instances = []
            for instance in instances:
                if instance is not None and isinstance(instance, dict):
                    # Ensure required fields exist
                    if 'ticker' not in instance:
                        continue
                    if 'timestamp' not in instance:
                        instance['timestamp'] = dt.now().isoformat()
                    
                    valid_instances.append(instance)
            
            # Ensure all instances are JSON serializable
            valid_instances = self._ensure_json_serializable(valid_instances)
            
            # Log results with limit information
            limit_msg = " (limit reached)" if TrackedList.is_limit_reached() else ""
            logger.info(f"Strategy returned {len(valid_instances)} valid instances{limit_msg}")
            logger.info(f"Strategy captured {len(self.plots_collection)} plots")
            return valid_instances, strategy_prints, self.plots_collection, self.response_images, None
            
        except Exception as e:
            # Get detailed error information for compilation/setup errors
            error_info = self._get_detailed_error_info(e, strategy_code)
            detailed_error_msg = self._format_detailed_error(error_info)
            
            logger.error(f"Strategy compilation or setup failed: {e}")
            logger.error(detailed_error_msg)
            
            # Return empty list for compilation/setup errors too
            return [], "", [], [], e
    
    async def _create_safe_globals(self, execution_mode: str) -> Dict[str, Any]:
        """Create safe execution environment with data accessor functions"""
        
        # Initialize plots collection for this execution
        self.plots_collection = []
        self.response_images = []
        self.plot_counter = 0
        # Create bound methods that use this engine's data accessor
        def bound_get_bar_data(timeframe="1d", columns=None, min_bars=1, filters=None, 
                              aggregate_mode=False, extended_hours=False, start_date=None, end_date=None):
            try:
                return self.data_accessor.get_bar_data(timeframe, columns, min_bars, filters, 
                                                      aggregate_mode, extended_hours, start_date, end_date)
            except Exception as e:
                logger.error(f"Data accessor error in get_bar_data(timeframe={timeframe}, min_bars={min_bars}): {e}")
                logger.debug(f"Data accessor error details: {type(e).__name__}: {e}")
                raise  # Re-raise to maintain error propagation
        
        def bound_get_general_data(columns=None, filters=None):
            try:
                return self.data_accessor.get_general_data(columns=columns, filters=filters)
            except Exception as e:
                logger.error(f"Data accessor error in get_general_data(columns={columns}, filters={filters}): {e}")
                logger.debug(f"Data accessor error details: {type(e).__name__}: {e}")
                raise  # Re-raise to maintain error propagation
        def bound_generate_equity_curve(instances: [], group_column=None):
            try:
                return self.data_accessor.generate_equity_curve(instances, group_column)
            except Exception as e:
                logger.error(f"Data accessor error in generate_equity_curve(instances={instances}, group_column={group_column}): {e}")
                logger.debug(f"Data accessor error details: {type(e).__name__}: {e}")
                raise  # Re-raise to maintain error propagation
        
        safe_globals = {
            # Built-ins for safe execution (including __import__ for import statements)
            '__builtins__': {
                'print': print,
                '__import__': __import__,
                'len': len,
                'range': range,
                'enumerate': enumerate,
                'float': float,
                'int': int,
                'str': str,
                'bool': bool,
                'abs': abs,
                'max': max,
                'min': min,
                'round': round,
                'sum': sum,
                'list': TrackedList,
                'dict': dict,
                'tuple': tuple,
                'set': set,
                'sorted': sorted,
                'reversed': reversed,
                'any': any,
                'all': all,
                'zip': zip,
            },
            
            # Standard imports
            'pd': pd,
            'numpy': np,
            'np': np,
            'pandas': pd,
            
            # Bound data accessor functions (use this engine's instance)
            'get_bar_data': bound_get_bar_data,
            'get_general_data': bound_get_general_data,
            'generate_equity_curve': bound_generate_equity_curve,
            
            # Math and datetime - make datetime module fully available
            'datetime': datetime,
            'dt': dt,
            'timedelta': timedelta,
            'time': dt.time,
            
            # Execution mode info
            'execution_mode': execution_mode,
        }
        
        # Add plotly imports if available
        try:
            import plotly.graph_objects as go
            import plotly.express as px
            from plotly.subplots import make_subplots
            
            safe_globals.update({
                'plotly': {
                    'graph_objects': go,
                    'express': px,
                    'subplots': {'make_subplots': make_subplots}
                },
                'go': go,
                'px': px,
                'make_subplots': make_subplots
            })
        except ImportError:
            logger.warning("Plotly not available - plot capture disabled")
        
        return safe_globals
    
    def _plotly_capture_context(self, strategy_id=None):
        """Context manager that temporarily patches plotly to capture plots instead of displaying them"""
        
        try:
            import plotly.graph_objects as go
            import plotly.express as px
            from plotly.subplots import make_subplots
        except ImportError:
            # Return a no-op context manager if plotly not available
            return contextlib.nullcontext()
        
        # Store original methods
        original_figure_show = go.Figure.show
        original_make_subplots = make_subplots
        
        # Create capture function
        def capture_plot(fig, *args, **kwargs):
            """Capture plot instead of showing it - extract only essential data"""
            try:
                plotID = self.plot_counter 
                self.plot_counter += 1
                
                # Extract title and ticker before extracting plot data
                cleaned_title, title_ticker = self._extract_plot_title_with_ticker(fig)
                
                # If ticker was extracted, update the figure's title to use cleaned version
                if title_ticker and hasattr(fig, 'layout') and hasattr(fig.layout, 'title'):
                    if hasattr(fig.layout.title, 'text'):
                        fig.layout.title.text = cleaned_title
                
                # Extract entire plot data (full figure object)
                figure_data = self._extract_plot_data(fig)
                
                # Generate PNG as base64 and add to response_images using matplotlib
                try:

                    png_base64 = plotly_to_matplotlib_png(fig, plotID, strategy_id)
                    if png_base64:
                        self.response_images.append(png_base64)
                        logger.debug(f"Generated PNG using matplotlib for plot {plotID}")
                    else:
                        logger.warning(f"Failed to generate PNG for plot {plotID}")
                        self.response_images.append(None)
                except Exception as matplotlib_error:
                    logger.warning(f"Failed to generate PNG for plot {plotID}: {matplotlib_error}")
                    # Add None to maintain index alignment with plots_collection
                    self.response_images.append(None)
                
                plot_data = {
                    'plotID': plotID,
                    'data': figure_data,  # Entire figure object with data, layout, config
                    'titleTicker': title_ticker  # Add ticker field (None if no ticker found)
                }
                self.plots_collection.append(plot_data)
            except Exception as e:
                logger.warning(f"Failed to capture plot data: {e}")
                # Fallback to basic plot info with ID
                plotID = self.plot_counter  # Use integer instead of string
                self.plot_counter += 1
                fallback_plot = {
                    'plotID': plotID,
                    'data': {},  # Empty object
                    'titleTicker': None  # No ticker in fallback case
                }
                self.plots_collection.append(fallback_plot)
                # Add None to response_images for failed plot
                self.response_images.append(None)
        
        # Create wrapped make_subplots that returns figures with captured show
        def captured_make_subplots(*args, **kwargs):
            fig = original_make_subplots(*args, **kwargs)
            # Monkey patch the show method on this specific figure instance
            fig.show = lambda *show_args, **show_kwargs: capture_plot(fig, *show_args, **show_kwargs)
            return fig
        
        @contextlib.contextmanager
        def patch_context():
            try:
                # Apply patches
                go.Figure.show = capture_plot
                
                # Patch make_subplots in the module where it was imported
                plotly.subplots.make_subplots = captured_make_subplots
                
                yield
                
            finally:
                # Restore original methods
                go.Figure.show = original_figure_show
                plotly.subplots.make_subplots = original_make_subplots
        
        return patch_context()

    def _extract_chart_type(self, fig) -> str:
        """Extract chart type from plotly figure"""
        try:
            if not fig.data:
                return 'line'
            
            # Get the type from the first trace
            trace_type = getattr(fig.data[0], 'type', 'scatter')
            
            # Map plotly trace types to standard chart types
            type_mapping = {
                'scatter': 'line' if getattr(fig.data[0], 'mode', '') == 'lines' else 'scatter',
                'line': 'line',
                'bar': 'bar',
                'histogram': 'histogram',
                'heatmap': 'heatmap',
                'box': 'bar',  # Fallback
                'violin': 'bar',  # Fallback
                'pie': 'bar',  # Fallback
                'candlestick': 'line',  # Fallback
                'ohlc': 'line'  # Fallback
            }
            
            return type_mapping.get(trace_type, 'line')
        except Exception:
            return 'line'

    def _make_json_serializable(self, value):
        """Recursively convert numpy/pandas types to native Python types for JSON serialization"""
        import numpy as np
        import pandas as pd
        
        # Handle None and basic types
        if value is None or isinstance(value, (str, bool)):
            return value
        
        # Handle numpy arrays
        if isinstance(value, np.ndarray):
            return value.tolist()
        
        # Handle numpy/pandas scalar types
        if isinstance(value, np.integer):
            return int(value)
        elif isinstance(value, np.floating):
            return float(value)
        elif isinstance(value, np.bool_):
            return bool(value)
        elif isinstance(value, (np.datetime64, pd.Timestamp)):
            # Convert datetime to Unix timestamp (int), handle NaT values
            try:
                if isinstance(value, pd.Timestamp):
                    if pd.isna(value):
                        return None
                    else:
                        return int(value.timestamp())
                else:
                    ts = pd.Timestamp(value)
                    if pd.isna(ts):
                        return None
                    else:
                        return int(ts.timestamp())
            except (ValueError, TypeError, OverflowError) as e:
                print(f"[make_json_serializable] Exception converting datetime: {e}, value: {value}")
                return None
        elif isinstance(value, dt):
            # Handle Python datetime objects from database
            try:
                return int(value.timestamp())
            except (ValueError, TypeError, OverflowError) as e:
                print(f"[make_json_serializable] Exception converting datetime.datetime: {e}, value: {value}")
                return None
        elif pd.api.types.is_integer_dtype(type(value)) and hasattr(value, 'item'):
            # Handle pandas nullable integer types
            return int(value.item()) if pd.notna(value) else None
        elif pd.api.types.is_float_dtype(type(value)) and hasattr(value, 'item'):
            # Handle pandas nullable float types
            return float(value.item()) if pd.notna(value) else None
        elif hasattr(value, 'item'):  # Other numpy scalars
            return value.item()
        elif pd.isna(value):
            # Handle pandas NA values
            return None
        elif isinstance(value, (int, float)):
            # Native Python types are already serializable
            return value
        
        # Handle nested structures
        elif isinstance(value, list):
            return [self._make_json_serializable(item) for item in value]
        elif isinstance(value, tuple):
            return [self._make_json_serializable(item) for item in value]
        elif isinstance(value, dict):
            return {key: self._make_json_serializable(val) for key, val in value.items()}
        
        # Fallback for unknown types - try to convert to string
        else:
            try:
                return str(value)
            except Exception as e:
                print(f"[make_json_serializable] Exception in fallback str: {e}, value: {value}")
                return None

    def _extract_plot_data(self, fig) -> dict:
        """Extract trace data from plotly figure using Plotly's built-in serialization (fig.to_dict())."""
        try:        
            plot_data = json.loads(fig.to_json())
            # Decode any binary data before sending to frontend
            return self._decode_binary_arrays(plot_data)
        except Exception as e:
            print(f"[extract_plot_data] Exception in fig.to_json(): {e}")
            return {}
    
    def _decode_binary_arrays(self, data):
        """Recursively decode binary arrays in plot data"""
        import base64
        import numpy as np
        
        if isinstance(data, dict):
            if 'bdata' in data and 'dtype' in data:
                # This is a binary encoded array
                try:
                    # Decode base64
                    binary_data = base64.b64decode(data['bdata'])
                    
                    # Convert to appropriate numpy array based on dtype
                    if data['dtype'] == 'f8':
                        arr = np.frombuffer(binary_data, dtype=np.float64)
                    elif data['dtype'] == 'f4':
                        arr = np.frombuffer(binary_data, dtype=np.float32)
                    elif data['dtype'] == 'i8':
                        arr = np.frombuffer(binary_data, dtype=np.int64)
                    elif data['dtype'] == 'i4':
                        arr = np.frombuffer(binary_data, dtype=np.int32)
                    else:
                        print(f"Unknown dtype: {data['dtype']}")
                        return []
                    
                    # Convert to regular Python list for JSON serialization
                    return arr.tolist()
                except Exception as e:
                    print(f"Error decoding binary data: {e}")
                    return []
            else:
                # Regular dict - recurse through values
                return {k: self._decode_binary_arrays(v) for k, v in data.items()}
        elif isinstance(data, list):
            return [self._decode_binary_arrays(item) for item in data]
        else:
            return data

    def _extract_plot_title_with_ticker(self, fig) -> Tuple[str, Optional[str]]:
        """Extract title and ticker from plotly figure. Returns (cleaned_title, ticker)"""
        try:
            title = 'Untitled Plot'
            if hasattr(fig, 'layout') and hasattr(fig.layout, 'title'):
                if hasattr(fig.layout.title, 'text') and fig.layout.title.text:
                    title = str(fig.layout.title.text)
            
            # Check for [TICKER] at the beginning
            ticker = None
            if title.startswith('[') and ']' in title:
                end_bracket = title.index(']')
                ticker_part = title[1:end_bracket]  # Extract content between brackets
                if ticker_part and ticker_part.isupper() and len(ticker_part) <= 10:  # Basic ticker validation
                    ticker = ticker_part
                    # Clean the title by removing [TICKER] and any leading space
                    cleaned_title = title[end_bracket + 1:].lstrip()
                    title = cleaned_title if cleaned_title else 'Untitled Plot'
            
            return title, ticker
        except Exception:
            return 'Untitled Plot', None

    def _extract_plot_title(self, fig) -> str:
        """Extract title from plotly figure"""
        try:
            if hasattr(fig, 'layout') and hasattr(fig.layout, 'title'):
                if hasattr(fig.layout.title, 'text') and fig.layout.title.text:
                    return str(fig.layout.title.text)
            return 'Untitled Plot'
        except Exception:
            return 'Untitled Plot'

    def _extract_minimal_layout(self, fig) -> dict:
        """Extract minimal layout information from plotly figure"""
        try:
            layout = {}
            
            if hasattr(fig, 'layout'):
                # Extract axis titles
                if hasattr(fig.layout, 'xaxis') and hasattr(fig.layout.xaxis, 'title'):
                    if hasattr(fig.layout.xaxis.title, 'text'):
                        layout['xaxis'] = {'title': str(fig.layout.xaxis.title.text) if fig.layout.xaxis.title.text else ''}
                    else:
                        layout['xaxis'] = {'title': ''}
                else:
                    layout['xaxis'] = {'title': ''}
                
                if hasattr(fig.layout, 'yaxis') and hasattr(fig.layout.yaxis, 'title'):
                    if hasattr(fig.layout.yaxis.title, 'text'):
                        layout['yaxis'] = {'title': str(fig.layout.yaxis.title.text) if fig.layout.yaxis.title.text else ''}
                    else:
                        layout['yaxis'] = {'title': ''}
                else:
                    layout['yaxis'] = {'title': ''}
                
                # Extract dimensions if explicitly set and make JSON serializable
                if hasattr(fig.layout, 'width') and fig.layout.width:
                    layout['width'] = self._make_json_serializable(fig.layout.width)
                if hasattr(fig.layout, 'height') and fig.layout.height:
                    layout['height'] = self._make_json_serializable(fig.layout.height)
            else:
                layout = {
                    'xaxis': {'title': ''},
                    'yaxis': {'title': ''}
                }
            
            return layout
        except Exception:
            return {
                'xaxis': {'title': ''},
                'yaxis': {'title': ''}
            }

    def _get_detailed_error_info(self, error: Exception, strategy_code: str) -> Dict[str, Any]:
        """Extract detailed error information including line numbers and code context"""
        try:
            # Get the full traceback
            tb = traceback.format_exc()
            
            # Get the exception info
            exc_type, exc_value, exc_traceback = sys.exc_info()
            
            error_info = {
                'error_type': type(error).__name__,
                'error_message': str(error),
                'full_traceback': tb,
                'line_number': None,
                'code_context': None,
                'function_name': None,
                'file_name': None
            }
            
            if exc_traceback:
                # Walk through the traceback to find the strategy code execution
                tb_frame = exc_traceback
                while tb_frame:
                    frame = tb_frame.tb_frame
                    filename = frame.f_code.co_filename
                    line_number = tb_frame.tb_lineno
                    function_name = frame.f_code.co_name
                    
                    # Look for the exec frame or strategy function
                    if ('<string>' in filename or 
                        'strategy' in function_name.lower() or
                        tb_frame.tb_next is None):  # Last frame
                        
                        error_info['line_number'] = line_number
                        error_info['function_name'] = function_name
                        error_info['file_name'] = filename
                        
                        # Try to get code context from strategy_code
                        if '<string>' in filename:
                            # This is from our exec'd strategy code
                            try:
                                code_lines = strategy_code.split('\n')
                                if 1 <= line_number <= len(code_lines):
                                    # Get context around the error line
                                    start_line = max(1, line_number - 3)
                                    end_line = min(len(code_lines), line_number + 3)
                                    
                                    context_lines = []
                                    for i in range(start_line, end_line + 1):
                                        line_content = code_lines[i - 1]  # Convert to 0-based indexing
                                        marker = ">>> " if i == line_number else "    "
                                        context_lines.append(f"{marker}{i:3d}: {line_content}")
                                    
                                    error_info['code_context'] = '\n'.join(context_lines)
                            except Exception as ctx_error:
                                error_info['code_context'] = f"Could not extract code context: {ctx_error}"
                        
                        break
                    
                    tb_frame = tb_frame.tb_next
            
            return error_info
            
        except Exception as e:
            # Fallback error info
            return {
                'error_type': type(error).__name__,
                'error_message': str(error),
                'full_traceback': traceback.format_exc(),
                'extraction_error': f"Could not extract detailed error info: {e}"
            }
    
    def _format_detailed_error(self, error_info: Dict[str, Any]) -> str:
        """Format detailed error information for logging"""
        formatted = [
            f"❌ STRATEGY EXECUTION ERROR: {error_info['error_type']}",
            f"📄 Error Message: {error_info['error_message']}",
        ]
        
        if error_info.get('line_number'):
            formatted.append(f"📍 Line Number: {error_info['line_number']}")
        
        if error_info.get('function_name'):
            formatted.append(f"🔧 Function: {error_info['function_name']}")
        
        if error_info.get('code_context'):
            formatted.extend([
                "📋 Code Context:",
                error_info['code_context']
            ])
        
        if error_info.get('full_traceback'):
            formatted.extend([
                "🔍 Full Traceback:",
                error_info['full_traceback']
            ])
            
        if error_info.get('extraction_error'):
            formatted.append(f"⚠️ Error Info Extraction Issue: {error_info['extraction_error']}")
        
        return '\n'.join(formatted)

    
    def _ensure_json_serializable(self, instances: List[Dict]) -> List[Dict]:
        """Ensure all values in instances are JSON serializable by converting numpy/pandas types"""
        import numpy as np
        import pandas as pd
        
        serializable_instances = []
        
        for instance in instances:
            serializable_instance = {}
            for key, value in instance.items():
                # Convert numpy/pandas types to native Python types
                if isinstance(value, np.integer):
                    serializable_instance[key] = int(value)
                elif isinstance(value, np.floating):
                    serializable_instance[key] = float(value)
                elif isinstance(value, np.bool_):
                    serializable_instance[key] = bool(value)
                elif isinstance(value, (np.datetime64, pd.Timestamp)):
                    # Convert datetime to Unix timestamp (int), handle NaT values
                    try:
                        if isinstance(value, pd.Timestamp):
                            if pd.isna(value):
                                serializable_instance[key] = None
                            else:
                                serializable_instance[key] = int(value.timestamp())
                        else:
                            ts = pd.Timestamp(value)
                            if pd.isna(ts):
                                serializable_instance[key] = None
                            else:
                                serializable_instance[key] = int(ts.timestamp())
                    except (ValueError, TypeError, OverflowError):
                        # Handle invalid timestamps
                        serializable_instance[key] = None
                elif isinstance(value, dt):
                    # Handle Python datetime objects from database
                    try:
                        serializable_instance[key] = int(value.timestamp())
                    except (ValueError, TypeError, OverflowError):
                        # Handle invalid datetime objects
                        serializable_instance[key] = None
                elif pd.api.types.is_integer_dtype(type(value)) and hasattr(value, 'item'):
                    # Handle pandas nullable integer types
                    serializable_instance[key] = int(value.item()) if pd.notna(value) else None
                elif pd.api.types.is_float_dtype(type(value)) and hasattr(value, 'item'):
                    # Handle pandas nullable float types
                    serializable_instance[key] = float(value.item()) if pd.notna(value) else None
                elif hasattr(value, 'item'):  # Other numpy scalars
                    serializable_instance[key] = value.item()
                elif pd.isna(value):
                    # Handle pandas NA values
                    serializable_instance[key] = None
                else:
                    serializable_instance[key] = value
            
            serializable_instances.append(serializable_instance)
        
        return serializable_instances

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
    
    def get_all_tickers_from_calls(self, getBarDataFunctionCalls: List[Dict[str, Any]]) -> List[str]:
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
    
    def getMaxTimeframeAndMinBars(self, getBarDataFunctionCalls: List[Dict[str, Any]]) -> Tuple[Optional[str], int]:
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

    def extract_get_bar_data_calls(self, strategy_code: str) -> List[Dict[str, Any]]:
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
                        call_info = self._extract_get_bar_data_params(node)
                        if call_info:
                            calls.append(call_info)
                            
        except SyntaxError as e:
            logger.warning(f"Failed to parse strategy code for get_bar_data extraction: {e}")
        except Exception as e:
            logger.warning(f"Error extracting get_bar_data calls: {e}")
            
        return calls
    
    def _extract_get_bar_data_params(self, call_node: ast.Call) -> Optional[Dict[str, Any]]:
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
                timeframe = self._extract_string_value(call_node.args[0])
                if timeframe:
                    call_info['timeframe'] = timeframe
                    
            if len(call_node.args) >= 3:
                # Third arg is min_bars (second is columns)
                min_bars = self._extract_int_value(call_node.args[2])
                if min_bars is not None:
                    call_info['min_bars'] = min_bars
            
            # Extract keyword arguments
            filters_node = None
            for keyword in call_node.keywords:
                if keyword.arg == 'timeframe':
                    timeframe = self._extract_string_value(keyword.value)
                    if timeframe:
                        call_info['timeframe'] = timeframe
                elif keyword.arg == 'min_bars':
                    min_bars = self._extract_int_value(keyword.value)
                    if min_bars is not None:
                        call_info['min_bars'] = min_bars
                elif keyword.arg == 'filters':
                    filters_node = keyword.value
            
            # Extract and analyze filters for ticker information
            call_info['filter_analysis'] = self._analyze_filters_ast(filters_node)
            
            return call_info
            
        except Exception as e:
            logger.debug(f"Failed to extract parameters from get_bar_data call: {e}")
            return None
    
    def _extract_string_value(self, node: ast.AST) -> Optional[str]:
        """Extract string value from AST node if possible."""
        try:
            if isinstance(node, ast.Constant) and isinstance(node.value, str):
                return node.value
            elif isinstance(node, ast.Str):  # Python < 3.8 compatibility
                return node.s
        except:
            pass
        return None

    def _extract_int_value(self, node: ast.AST) -> Optional[int]:
        """Extract integer value from AST node if possible."""
        try:
            if isinstance(node, ast.Constant) and isinstance(node.value, int):
                return node.value
            elif isinstance(node, ast.Num):  # Python < 3.8 compatibility
                if isinstance(node.n, int):
                    return node.n
        except:
            pass
        return None
    
    def _analyze_filters_ast(self, filters_node: Optional[ast.AST]) -> Dict[str, Any]:
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
                    key_str = self._extract_string_value(key)
                    if key_str in ['tickers', 'ticker']:
                        # Extract ticker values
                        if isinstance(value, ast.List):
                            # Handle list of tickers: ['AAPL', 'MSFT']
                            for elem in value.elts:
                                ticker = self._extract_string_value(elem)
                                tickers.add(ticker.upper())
                        elif isinstance(value, (ast.Constant, ast.Str)):
                            # Handle single ticker: 'AAPL'
                            ticker = self._extract_string_value(value)
                            tickers.add(ticker.upper())
                
                if tickers:
                    filter_analysis["has_tickers"] = True
                    filter_analysis["specific_tickers"] = sorted(list(tickers))
                    
        except Exception as e:
            logger.debug(f"Error analyzing filters AST: {e}")
        
        return filter_analysis
    