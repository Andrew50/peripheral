"""
General Python Sandbox
A domain-agnostic Python code execution environment with security, plotting, and error handling.
"""

import asyncio
import contextlib
import datetime
import io
import json
import logging
import math
import sys
import time
import traceback
from dataclasses import dataclass
from datetime import datetime as dt, timedelta
from typing import Any, Dict, List, Optional, Tuple

import numpy as np
import pandas as pd
import plotly

# Set up logger before using it
logger = logging.getLogger(__name__)

try:
    from plotlyToMatlab import plotly_to_matplotlib_png
except ImportError:
    logger.warning("plotlyToMatlab module not available")
    # Handle missing plotlyToMatlab gracefully
    def plotly_to_matplotlib_png(*_args, **_kwargs):
        return None


@dataclass
class SandboxConfig:
    """Configuration for sandbox execution"""
    allowed_imports: Dict[str, Any]
    allowed_builtins: Dict[str, Any]
    execution_timeout: float = 30.0
    enable_plots: bool = True
    max_output_size: int = 1024 * 1024  # 1MB max output


@dataclass
class SandboxResult:
    """Result of sandbox execution"""
    success: bool
    result: Any = None  # Main return value (renamed from return_value)
    prints: str = ""    # Stdout output (renamed from stdout)
    stderr: str = ""
    plots: List[Dict] = None
    response_images: List[str] = None
    error: Optional[str] = None
    error_details: Optional[Dict[str, Any]] = None
    execution_time_ms: float = 0.0


class PythonSandbox:
    """
    A secure Python execution environment that can run arbitrary Python code
    with configurable imports, plotting support, and comprehensive error handling.
    """
    
    def __init__(self, config: SandboxConfig, execution_id: str = None):
        self.config = config
        self.execution_id = execution_id
        self.plots_collection = []
        self.response_images = []
        self.plot_counter = 0
        
    async def execute_code(self, code: str, additional_globals: Dict[str, Any] = None, 
                          execution_id: str = None) -> SandboxResult:
        """
        Execute Python code in a secure sandbox environment
        
        Args:
            code: Python code to execute
            additional_globals: Additional global variables to provide
            execution_id: Optional ID for tracking (used for plot generation)
            
        Returns:
            SandboxResult with execution results
        """
        start_time = time.time()
        
        if execution_id:
            self.execution_id = execution_id
            
        try:
            
            # Create execution environment
            safe_globals = self._create_safe_globals(additional_globals or {})
            safe_locals = {}
            
            # Initialize capture systems
            self._reset_capture_systems()
            
            # Execute with timeout and capture
            logger.info("🚀 Starting code execution...")
            try:
                result = await asyncio.wait_for(
                    self._execute_with_capture(code, safe_globals, safe_locals),
                    timeout=self.config.execution_timeout
                )
                
                execution_time = (time.time() - start_time) * 1000
                
                return SandboxResult(
                    success=True,
                    result=result['return_value'],
                    prints=result['prints'],
                    stderr=result['stderr'],
                    plots=self.plots_collection,
                    response_images=self.response_images,
                    execution_time_ms=execution_time
                )
                
            except asyncio.TimeoutError:
                return SandboxResult(
                    success=False,
                    error=f"Execution timed out after {self.config.execution_timeout} seconds",
                    execution_time_ms=(time.time() - start_time) * 1000
                )
            except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
                error_info = self._get_detailed_error_info(e, code)
                return SandboxResult(
                    success=False,
                    error=str(e),
                    error_details=error_info,
                    execution_time_ms=(time.time() - start_time) * 1000
                )
        except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
            error_info = self._get_detailed_error_info(e, code)
            return SandboxResult(
                success=False,
                error=str(e),
                error_details=error_info,
                execution_time_ms=(time.time() - start_time) * 1000
            )
    
    def _reset_capture_systems(self):
        """Reset plot and output capture systems"""
        self.plots_collection = []
        self.response_images = []
        self.plot_counter = 0
    
    async def _execute_with_capture(self, code: str, safe_globals: Dict[str, Any], 
                                  safe_locals: Dict[str, Any]) -> Dict[str, Any]:
        """Execute code with stdout/stderr and plot capture"""
        
        stdout_buffer = io.StringIO()
        stderr_buffer = io.StringIO()
        return_value = None
        
        try:
            # Fix exec usage
            # nosec B102 - exec necessary with proper sandboxing
            exec(code, safe_globals, safe_locals)
            # Execute with stdout/stderr capture
            code_func = safe_locals.get('code')
            if not code_func or not callable(code_func):
                func = safe_locals.get('main')
                if func and callable(func):
                    code_func = func
                
            function_prints = ""
            with contextlib.redirect_stdout(stdout_buffer), self._plotly_capture_context():
                return_value = code_func()
            function_prints = stdout_buffer.getvalue()
            stderr_content = stderr_buffer.getvalue()
            
            
            return {
                'return_value': return_value,
                'prints': function_prints,
                'stderr': stderr_content
            }
            
        except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError):
            # Capture any stderr that was generated before the error
            stderr_content = stderr_buffer.getvalue()
            if stderr_content:
                logger.error("Stderr before error: %s", stderr_content)
            raise
    
    def _create_safe_globals(self, additional_globals: Dict[str, Any]) -> Dict[str, Any]:
        """Create safe execution environment with configurable globals"""
        
        # Start with configured builtins
        safe_globals = {
            '__builtins__': self.config.allowed_builtins.copy()
        }
        
        # Add configured imports
        safe_globals.update(self.config.allowed_imports)
        
        # Add additional globals provided by caller
        safe_globals.update(additional_globals)

        # -----------------------------
        # Match StrategyEngine access
        # -----------------------------
        try:
            # Import data accessor and create bound helper functions
            try:
                from data_accessors import get_data_accessor
            except ImportError:
                logger.warning("data_accessors module not available")
                return safe_globals
                
            data_accessor = get_data_accessor()

            # Set execution context for full historical data access (like strategy engine backtest mode)
            # Use imported datetime instead of redefining
            start_date = datetime.datetime(2003, 1, 1)
            end_date = datetime.datetime.now()
            
            data_accessor.set_execution_context(
                mode='backtest',
                symbols=None,  # All symbols
                start_date=start_date,
                end_date=end_date
            )

            def bound_get_bar_data(timeframe="1d", columns=None, min_bars=1, filters=None,
                                   aggregate_mode=False, extended_hours=False, start_date=None, end_date=None):
                try:
                    return data_accessor.get_bar_data(timeframe, columns, min_bars, filters,
                                                      aggregate_mode, extended_hours, start_date, end_date)
                except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
                    logger.error("Data accessor error in get_bar_data(timeframe=%s, min_bars=%s): %s", 
                               timeframe, min_bars, e)
                    logger.debug("Data accessor error details: %s: %s", type(e).__name__, e)
                    raise  # Re-raise to maintain error propagation

            def bound_get_general_data(columns=None, filters=None):
                try:
                    return data_accessor.get_general_data(columns=columns, filters=filters)
                except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
                    logger.error("Data accessor error in get_general_data(columns=%s, filters=%s): %s", 
                               columns, filters, e)
                    logger.debug("Data accessor error details: %s: %s", type(e).__name__, e)
                    raise  # Re-raise to maintain error propagation

            def bound_generate_equity_curve(instances: list, group_column=None):
                try:
                    return data_accessor.generate_equity_curve(instances, group_column)
                except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
                    logger.error("Data accessor error in generate_equity_curve(instances=%s, group_column=%s): %s", 
                               instances, group_column, e)
                    logger.debug("Data accessor error details: %s: %s", type(e).__name__, e)
                    raise  # Re-raise to maintain error propagation

            safe_globals.update({
                'get_bar_data': bound_get_bar_data,
                'get_general_data': bound_get_general_data,
                'generate_equity_curve': bound_generate_equity_curve,
            })
            
        except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
            logger.error("❌ CRITICAL: Failed to bind data accessor functions: %s", e)
            logger.error("📄 Data accessor binding traceback: %s", traceback.format_exc())
            # Don't silently ignore - this is critical for Python agent functionality
            raise RuntimeError(f"Python agent requires data accessor functions: {e}") from e


        return safe_globals
    
    def _plotly_capture_context(self):
        """Context manager that captures plotly plots instead of displaying them"""
        
        if not self.config.enable_plots:
            return contextlib.nullcontext()
        
        # Import here to avoid circular imports but still have access when needed
        # These imports are used within this context manager
        try:
            import plotly.graph_objects as go
            import plotly.express as px  # Used in the patching context
            from plotly.subplots import make_subplots
        except ImportError:
            return contextlib.nullcontext()
        
        # Store original methods
        original_figure_show = go.Figure.show
        original_make_subplots = make_subplots
        
        def capture_plot(fig, *_args, **_kwargs):
            """Capture plot instead of showing it"""
            try:
                plot_id = self.plot_counter
                self.plot_counter += 1
                
                # Extract title and ticker
                cleaned_title, title_ticker = self._extract_plot_title_with_ticker(fig)
                
                # Update figure title if ticker was extracted
                if title_ticker and hasattr(fig, 'layout') and hasattr(fig.layout, 'title'):
                    if hasattr(fig.layout.title, 'text'):
                        fig.layout.title.text = cleaned_title
                
                # Extract plot data
                figure_data = self._extract_plot_data(fig)
                
                # Generate PNG as base64
                try:
                    png_base64 = plotly_to_matplotlib_png(fig, plot_id, "Execution ID", self.execution_id)
                    if png_base64:
                        self.response_images.append(png_base64)
                        logger.debug("Generated PNG for plot %s", plot_id)
                    else:
                        logger.warning("Failed to generate PNG for plot %s", plot_id)
                        self.response_images.append(None)
                except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
                    logger.warning("Failed to generate PNG for plot %s: %s", plot_id, e)
                    self.response_images.append(None)
                
                plot_data = {
                    'plotID': plot_id,
                    'data': figure_data,
                    'titleTicker': title_ticker
                }
                self.plots_collection.append(plot_data)
                
            except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
                logger.warning("Failed to capture plot: %s", e)
                plot_id = self.plot_counter
                self.plot_counter += 1
                fallback_plot = {
                    'plotID': plot_id,
                    'data': {},
                    'titleTicker': None
                }
                self.plots_collection.append(fallback_plot)
                self.response_images.append(None)
        
        def captured_make_subplots(*args, **kwargs):
            fig = original_make_subplots(*args, **kwargs)
            fig.show = lambda *show_args, **show_kwargs: capture_plot(fig, *show_args, **show_kwargs)
            return fig
        
        @contextlib.contextmanager
        def patch_context():
            try:
                # Apply patches
                go.Figure.show = capture_plot
                plotly.subplots.make_subplots = captured_make_subplots
                yield
            finally:
                # Restore original methods
                go.Figure.show = original_figure_show
                plotly.subplots.make_subplots = original_make_subplots
        
        return patch_context()
    
    def _extract_plot_data(self, fig) -> dict:
        """Extract plot data from plotly figure"""
        try:
            plot_data = json.loads(fig.to_json())
            return self._decode_binary_arrays(plot_data)
        except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
            logger.warning("Failed to extract plot data: %s", e)
            return {}
    
    def _decode_binary_arrays(self, data):
        """Recursively decode binary arrays in plot data"""
        # Import here to avoid circular imports
        import base64
        # Use the already imported np from the top of the file
        
        if isinstance(data, dict):
            if 'bdata' in data and 'dtype' in data:
                try:
                    binary_data = base64.b64decode(data['bdata'])
                    
                    if data['dtype'] == 'f8':
                        arr = np.frombuffer(binary_data, dtype=np.float64)
                    elif data['dtype'] == 'f4':
                        arr = np.frombuffer(binary_data, dtype=np.float32)
                    elif data['dtype'] == 'i8':
                        arr = np.frombuffer(binary_data, dtype=np.int64)
                    elif data['dtype'] == 'i4':
                        arr = np.frombuffer(binary_data, dtype=np.int32)
                    else:
                        logger.warning("Unknown dtype: %s", data['dtype'])
                        return []
                    
                    return arr.tolist()
                except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
                    logger.warning("Error decoding binary data: %s", e)
                    return []
            else:
                return {k: self._decode_binary_arrays(v) for k, v in data.items()}
        elif isinstance(data, list):
            return [self._decode_binary_arrays(item) for item in data]
        else:
            return data
    
    def _extract_plot_title_with_ticker(self, fig) -> Tuple[str, Optional[str]]:
        """Extract title and ticker from plotly figure"""
        try:
            title = 'Untitled Plot'
            if hasattr(fig, 'layout') and hasattr(fig.layout, 'title'):
                if hasattr(fig.layout.title, 'text') and fig.layout.title.text:
                    title = str(fig.layout.title.text)
            
            # Check for [TICKER] at the beginning
            ticker = None
            if title.startswith('[') and ']' in title:
                end_bracket = title.index(']')
                ticker_part = title[1:end_bracket]
                if ticker_part and ticker_part.isupper() and len(ticker_part) <= 10:
                    ticker = ticker_part
                    cleaned_title = title[end_bracket + 1:].lstrip()
                    title = cleaned_title if cleaned_title else 'Untitled Plot'
            
            return title, ticker
        except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError):
            return 'Untitled Plot', None
    
    def _get_detailed_error_info(self, error: Exception, code: str) -> Dict[str, Any]:
        """Extract detailed error information including line numbers and code context"""
        try:
            tb = traceback.format_exc()
            _, _, exc_traceback = sys.exc_info()
            
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
                tb_frame = exc_traceback
                while tb_frame:
                    frame = tb_frame.tb_frame
                    filename = frame.f_code.co_filename
                    line_number = tb_frame.tb_lineno
                    function_name = frame.f_code.co_name
                    
                    if ('<string>' in filename or 
                        'code' in function_name.lower() or
                        tb_frame.tb_next is None):
                        
                        error_info['line_number'] = line_number
                        error_info['function_name'] = function_name
                        error_info['file_name'] = filename
                        
                        if '<string>' in filename:
                            try:
                                code_lines = code.split('\n')
                                if 1 <= line_number <= len(code_lines):
                                    start_line = max(1, line_number - 3)
                                    end_line = min(len(code_lines), line_number + 3)
                                    
                                    context_lines = []
                                    for i in range(start_line, end_line + 1):
                                        line_content = code_lines[i - 1]
                                        marker = ">>> " if i == line_number else "    "
                                        context_lines.append(f"{marker}{i:3d}: {line_content}")
                                    
                                    error_info['code_context'] = '\n'.join(context_lines)
                            except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as ctx_error:
                                error_info['code_context'] = f"Could not extract code context: {ctx_error}"
                        
                        break
                    
                    tb_frame = tb_frame.tb_next
            
            return error_info
            
        except (ValueError, TypeError, AttributeError, KeyError, ImportError, RuntimeError) as e:
            return {
                'error_type': type(error).__name__,
                'error_message': str(error),
                'full_traceback': traceback.format_exc(),
                'extraction_error': f"Could not extract detailed error info: {e}"
            }


# Default configurations for common use cases
def create_default_config() -> SandboxConfig:
    """Create default sandbox configuration"""
    
    # Import here to avoid reimporting
    import plotly.graph_objects as go
    import plotly.express as px
    from plotly.subplots import make_subplots
    
    return SandboxConfig(
        allowed_imports={
            'pd': pd,
            'numpy': np,
            'np': np,
            'pandas': pd,
            'datetime': datetime,
            'dt': dt,
            'timedelta': timedelta,
            'time': dt.time,
            'math': math,
            'json': json,
            'plotly': {
                'graph_objects': go,
                'express': px,
                'subplots': {'make_subplots': make_subplots}
            },
            'go': go,
            'px': px,
            'make_subplots': make_subplots,
        },
        allowed_builtins={
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
            'list': list,
            'dict': dict,
            'tuple': tuple,
            'set': set,
            'sorted': sorted,
            'reversed': reversed,
            'any': any,
            'all': all,
            'zip': zip,
        },
        execution_timeout=30.0,
        enable_plots=True,
        max_output_size=1024 * 1024 # 1MB max output
    )


 