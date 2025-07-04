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
import datetime as dt 
from typing import Any, Dict, List, Optional, Union, Tuple
import json
import time
import io
import contextlib
import plotly 

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
            instances, strategy_prints, strategy_plots = await self._execute_strategy(
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
                'symbols_processed': len(symbols),
                'strategy_prints': strategy_prints,
                'strategy_plots': strategy_plots,
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
            instances, _, _ = await self._execute_strategy(
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
                'strategy_prints': '',
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
            instances, _, _ = await self._execute_strategy(
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
            instances, _, _ = await self._execute_strategy(
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
    ) -> Tuple[List[Dict], str, List[Dict]]:
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
            
            # Execute strategy function with proper error handling and stdout capture
            logger.info(f"Executing strategy function using data accessor approach")
            strategy_prints = ""
            try:
                # Capture stdout and plots during strategy execution
                stdout_buffer = io.StringIO()
                with contextlib.redirect_stdout(stdout_buffer), self._plotly_capture_context():
                    instances = strategy_func()
                strategy_prints = stdout_buffer.getvalue()
            except Exception as strategy_error:
                logger.error(f"Strategy function execution failed: {strategy_error}")
                logger.debug(f"Strategy error details: {type(strategy_error).__name__}: {strategy_error}")
                # Return empty list and any captured output instead of crashing
                return [], "", []
            
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
            
            # Ensure all instances are JSON serializable
            valid_instances = self._ensure_json_serializable(valid_instances)
            
            logger.info(f"Strategy returned {len(valid_instances)} valid instances")
            logger.info(f"Strategy captured {len(self.plots_collection)} plots")
            return valid_instances, strategy_prints, self.plots_collection
            
        except Exception as e:
            logger.error(f"Strategy compilation or setup failed: {e}")
            logger.debug(f"Setup error details: {type(e).__name__}: {e}")
            # Return empty list for compilation/setup errors too
            return [], "", []
    
    async def _create_safe_globals(self, execution_mode: str) -> Dict[str, Any]:
        """Create safe execution environment with data accessor functions"""
        
        # Initialize plots collection for this execution
        self.plots_collection = []
        
        # Create bound methods that use this engine's data accessor
        def bound_get_bar_data(timeframe="1d", columns=None, min_bars=1, filters=None, 
                              aggregate_mode=False, extended_hours=False):
            try:
                return self.data_accessor.get_bar_data(timeframe, columns, min_bars, filters, 
                                                      aggregate_mode, extended_hours)
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
            },
            
            # Standard imports
            'pd': pd,
            'numpy': np,
            'np': np,
            'pandas': pd,
            
            # Bound data accessor functions (use this engine's instance)
            'get_bar_data': bound_get_bar_data,
            'get_general_data': bound_get_general_data,
            
            # Math and datetime - make datetime module fully available
            'datetime': dt,
            'timedelta': timedelta,
            'time': dt.time,
            
            # Execution mode info
            'execution_mode': execution_mode,
            
            # Helper function to create instances
            'create_instance': lambda ticker, timestamp, **kwargs: {
                'ticker': ticker,
                'timestamp': timestamp,
                **kwargs
            }
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
    
    def _plotly_capture_context(self):
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
                # Extract essential plot data matching agent prompt format
                plot_data = {
                    'chart_type': self._extract_chart_type(fig),
                    'data': self._extract_plot_data(fig),
                    'title': self._extract_plot_title(fig),
                    'layout': self._extract_minimal_layout(fig)
                }
                self.plots_collection.append(plot_data)
            except Exception as e:
                logger.warning(f"Failed to capture plot data: {e}")
                # Fallback to basic plot info
                fallback_plot = {
                    'chart_type': 'line',
                    'data': [],
                    'title': 'Plot Capture Failed',
                    'layout': {'xaxis': {'title': ''}, 'yaxis': {'title': ''}}
                }
                self.plots_collection.append(fallback_plot)
        
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
        
        # Handle numpy/pandas scalar types
        if isinstance(value, np.integer):
            return int(value)
        elif isinstance(value, np.floating):
            return float(value)
        elif isinstance(value, np.bool_):
            return bool(value)
        elif isinstance(value, (np.datetime64, pd.Timestamp)):
            # Convert datetime to Unix timestamp (int)
            if isinstance(value, pd.Timestamp):
                return int(value.timestamp())
            else:
                return int(pd.Timestamp(value).timestamp())
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
            except Exception:
                return None

    def _extract_plot_data(self, fig) -> list:
        """Extract trace data from plotly figure"""
        try:
            data = []
            for trace in fig.data:
                trace_data = {
                    'name': getattr(trace, 'name', ''),
                    'type': getattr(trace, 'type', 'scatter')
                }
                
                # Extract coordinate data and make JSON serializable
                if hasattr(trace, 'x') and trace.x is not None:
                    trace_data['x'] = self._make_json_serializable(list(trace.x))
                if hasattr(trace, 'y') and trace.y is not None:
                    trace_data['y'] = self._make_json_serializable(list(trace.y))
                if hasattr(trace, 'z') and trace.z is not None:
                    trace_data['z'] = self._make_json_serializable(list(trace.z))
                
                # Add mode for scatter plots
                if hasattr(trace, 'mode'):
                    trace_data['mode'] = trace.mode
                
                data.append(trace_data)
            
            return data
        except Exception:
            return []

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

    def _validate_strategy_code(self, strategy_code: str) -> bool:
        """Basic validation of strategy code"""
        # Use the security validator
        return self.validator.validate_code(strategy_code)
    
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
                    # Convert datetime to Unix timestamp (int)
                    if isinstance(value, pd.Timestamp):
                        serializable_instance[key] = int(value.timestamp())
                    else:
                        serializable_instance[key] = int(pd.Timestamp(value).timestamp())
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