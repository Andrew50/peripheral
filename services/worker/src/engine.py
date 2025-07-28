"""
Data Accessor Strategy Engine
Executes Python strategies that use get_bar_data() and get_general_data() functions
instead of receiving DataFrames as parameters.
"""

from datetime import datetime as dt, timedelta
import json
import datetime
import base64
import logging
import io
import contextlib
from typing import Any, Dict, List, Optional, Tuple
import numpy as np
import pandas as pd
import plotly
import plotly.express as px
from plotly.subplots import make_subplots
import plotly.graph_objects as go

from utils.plotlyToMatlab import plotly_to_matplotlib_png
from utils.context import Context
from utils.data_accessors import _get_bar_data, _get_general_data

logger = logging.getLogger(__name__)

MAX_INSTANCES = 15000

class TrackedList(list):
    """List that tracks total instances across all TrackedList objects"""
    _global_instance_count = 0
    _max_instances = 15000
    _limit_reached = False

    @classmethod
    def reset_counter(cls, max_instances=MAX_INSTANCES):
        """Reset global counter for new strategy execution"""
        cls._global_instance_count = 0
        cls._max_instances = max_instances
        cls._limit_reached = False
        logger.debug("Reset instance counter, max_instances: %s", max_instances)

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
                logger.warning("Instance limit reached: %s/%s. Stopping instance collection.",
                               TrackedList._global_instance_count, TrackedList._max_instances)
            return False  # Don't add more instances
        # Log warning when approaching limit (90% threshold)
        if new_count > TrackedList._max_instances * 0.9 and not TrackedList._limit_reached:
            logger.warning("Approaching instance limit: %s/%s", new_count,
                           TrackedList._max_instances)
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

def execute_strategy(
    ctx: Context,
    strategy_code: str, 
    strategy_id: int,
    version: int = None,
    start_date: str = "2003-01-01",
    end_date: str = dt.now().isoformat(),
    symbols: List[str] = None,
   # max_instances: int = 15000,
    #version: int = None # None means new strategy
) -> Tuple[List[Dict], str, List[Dict], List[Dict], Exception]:
    """Execute the strategy function with data accessor context"""

    # Create safe execution environment with data accessor functions
    safe_globals = _create_safe_globals(ctx, start_date, end_date)
    safe_locals = {}

    try:
        # Execute strategy code in restricted environment
        # nolint B102 - exec necessary for strategy execution with proper sandboxing
        exec(strategy_code, safe_globals, safe_locals)  # nosec  # noqa: S102 - exec necessary for strategy execution with proper sandboxing

        strategy_func = safe_locals.get('strategy')
        if not strategy_func or not callable(strategy_func):
            raise ValueError("No strategy function found. Function must be named 'strategy'")

        # Reset instance counter for this execution
        TrackedList.reset_counter(max_instances=MAX_INSTANCES)

        # Execute strategy function with proper error handling and stdout capture
        #logger.info(f"Executing strategy function using data accessor approach")
        strategy_prints = ""
        try:
            # Capture stdout and plots during strategy execution
            stdout_buffer = io.StringIO()
            with contextlib.redirect_stdout(stdout_buffer), _plotly_capture_context(strategy_id, version) as (plots_collection, response_images):
                instances = strategy_func()
            strategy_prints = stdout_buffer.getvalue()


        #npylint: disable=W0713 - exec necessary for strategy execution with proper sandboxing
        except Exception as strategy_error:
            # Get detailed error information
            #error_info = _get_detailed_error_info(strategy_error, strategy_code)
            #detailed_error_msg = _format_detailed_error(error_info)

            logger.error("Strategy function execution failed: %s", strategy_error)
            #logger.error(detailed_error_msg)

            # Return empty list and any captured output instead of crashing
            return [], "", [], [], strategy_error

        # Validate and clean instances
        if not isinstance(instances, list):
            logger.error("Strategy function must return a list, got %s", type(instances))
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
        valid_instances = _ensure_json_serializable(valid_instances)

        return valid_instances, strategy_prints, plots_collection, response_images, None

    #npylint: disable=W0713 - exec necessary for strategy execution with proper sandboxing
    except Exception as e:
        logger.error("Strategy compilation or setup failed: %s", e)
        return [], "", [], [], e

def _create_safe_globals(ctx: Context, start_date: str, end_date: str, symbolsIntersect: List[str] = None) -> Dict[str, Any]:
    """Create safe execution environment with data accessor functions"""

    # Initialize plots collection for this execution
    # Create bound methods that use this engine's data accessor
    # this is so that strategy code cannot access class data
    def apply_drawdown_styling(fig):
        """Apply custom styling for drawdown plots with red line and shaded fill"""
        # Update all traces to use red line with shaded fill
        fig.update_traces(
            line=dict(color='rgb(255, 77, 77)', width=2),
            fill='tozeroy',
            fillcolor='rgba(255, 77, 77, 0.4)'
        )
        return fig
    def apply_equity_curve_styling(fig):
        """Apply custom styling for equity curve plots - blue above 0, red below 0, no fill"""
        # Update all traces to remove fill and set basic styling
        fig.update_traces(
            fill=None,  # Remove any fill
            fillcolor=None,
            line=dict(width=2)
        )
        # For each trace, determine the predominant color based on final value
        # or split into positive/negative segments if needed
        for i, trace in enumerate(fig.data):
            if hasattr(trace, 'y') and trace.y is not None:
                y_values = list(trace.y)
                if y_values:
                    # Use the final value to determine color
                    final_value = y_values[-1]
                    color = 'rgb(0, 150, 255)' if final_value >= 0 else 'rgb(255, 77, 77)'
                    # Update the trace color
                    fig.data[i].update(line=dict(color=color, width=2))
        return fig
    
    def get_bar_data(timeframe, min_bars, columns=None, filters=None, 
                         extended_hours=False):
        return _get_bar_data(ctx,start_date,end_date,timeframe, columns, min_bars, filters, extended_hours)
    def get_general_data(columns=None, filters=None):
        #if the execution is called with a certain set of symboles then we need to intersect that with each get bar data call
        # this will cause references to statuc symbols to possibly fail like if you are referencing spy in a strategy not returning spy
        if symbolsIntersect :
            filters['symbols'] = symbolsIntersect.intersection(filters['symbols'])
        return _get_general_data(ctx, columns, filters)
    

    safe_globals = {
        # Built-ins for safe execution (including __import__ for import statements)
        # we dont use defaults becuase that would allow for things like open, eval, exec, etc.
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
        'get_bar_data': get_bar_data,
        'get_general_data': get_general_data,
        # Plot styling functions
        'apply_drawdown_styling': apply_drawdown_styling,
        'apply_equity_curve_styling': apply_equity_curve_styling,
        # Math and datetime - make datetime module fully available
        'datetime': datetime,
        'dt': dt,
        'timedelta': timedelta,
        'time': dt.time,
        # Execution mode info
        'plotly': {
            'graph_objects': go,
            'express': px,
            'subplots': {'make_subplots': make_subplots}
        },
        'go': go,
        'px': px,
        'make_subplots': make_subplots
    }
    return safe_globals

def _plotly_capture_context(strategy_id=None, version=None):
    """Context manager that temporarily patches plotly to capture plots
     instead of displaying them"""
    # Store original methods
    original_figure_show = go.Figure.show
    original_make_subplots = make_subplots
    plots_collection = []
    response_images = []
    plot_counter = 0
    # Create capture function
    def capture_plot(fig):
        """Capture plot instead of showing it - extract only essential data"""
        nonlocal plot_counter
        try:
            plot_id = plot_counter
            plot_counter += 1
            # Extract title and ticker before extracting plot data
            cleaned_title, title_ticker = _extract_plot_title_with_ticker(fig)
            # If ticker was extracted, update the figure's title to use cleaned version
            if title_ticker and hasattr(fig, 'layout') and hasattr(fig.layout, 'title'):
                if hasattr(fig.layout.title, 'text'):
                    fig.layout.title.text = cleaned_title
            # Extract entire plot data (full figure object)
            figure_data = _extract_plot_data(fig)
            # Generate PNG as base64 and add to response_images using matplotlib
            try:

                png_base64 = plotly_to_matplotlib_png(fig, plot_id, "Strategy ID", strategy_id, version)
                if png_base64:
                    response_images.append(png_base64)
                    logger.debug("Generated PNG using matplotlib for plot %s", plot_id)
                else:
                    logger.warning("Failed to generate PNG for plot %s", plot_id)
                    response_images.append(None)
            except ValueError as matplotlib_error:
                logger.warning("Failed to generate PNG for plot %s: %s", plot_id, matplotlib_error)
                # Add None to maintain index alignment with plots_collection
                response_images.append(None)

            plot_data = {
                'plotID': plot_id,
                'data': figure_data,  # Entire figure object with data, layout, config
                'titleTicker': title_ticker  # Add ticker field (None if no ticker found)
            }
            plots_collection.append(plot_data)
        except ValueError as e:
            logger.warning("Failed to capture plot data: %s", e)
            # Fallback to basic plot info with ID
            plot_id = plot_counter  # Use integer instead of string
            plot_counter += 1
            fallback_plot = {
                'plotID': plot_id,
                'data': {},  # Empty object
                'titleTicker': None  # No ticker in fallback case
            }
            plots_collection.append(fallback_plot)
            # Add None to response_images for failed plot
            response_images.append(None)
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



def _extract_plot_data(fig) -> dict:
    """Extract trace data from plotly figure using Plotly's
     built-in serialization (fig.to_dict())."""
    try:
        plot_data = json.loads(fig.to_json())
        # Decode any binary data before sending to frontend
        return _decode_binary_arrays(plot_data)
    except ValueError as e:
        logger.error("[extract_plot_data] Exception in fig.to_json(): %s", e)
        return {}

def _decode_binary_arrays(data):
    """Recursively decode binary arrays in plot data"""

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
            except ValueError as e:
                logger.error("Error decoding binary data: %s", e)
                return []
        else:
            # Regular dict - recurse through values
            return {k: _decode_binary_arrays(v) for k, v in data.items()}
    elif isinstance(data, list):
        return [_decode_binary_arrays(item) for item in data]
    else:
        return data

def _extract_plot_title_with_ticker(fig) -> Tuple[str, Optional[str]]:
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
    except ValueError:
        return 'Untitled Plot', None




def _ensure_json_serializable(instances: List[Dict]) -> List[Dict]:
    """Ensure all values in instances are JSON serializable by converting numpy/pandas types"""
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
