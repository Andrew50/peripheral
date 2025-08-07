"""
Generic Plotly-to-matplotlib fallback for chart image generation
"""

import logging
from io import BytesIO
import base64

import matplotlib.pyplot as plt
import matplotlib.dates as mdates
import numpy as np

logger = logging.getLogger(__name__)


def extract_any_plottable_data(trace):
    """Extract any numeric data that can be plotted, regardless of trace type"""
    plottable_data = {}
    
    # Check all common data attributes
    for attr in ['x', 'y', 'z', 'values', 'open', 'high', 'low', 'close']:
        data = getattr(trace, attr, None)
        if data is not None and hasattr(data, '__len__') and len(data) > 0:
            try:
                # Handle various data types
                if hasattr(data, 'tolist'):
                    data = data.tolist()
                
                numeric_data = []
                for x in data:
                    if x is not None:
                        try:
                            numeric_data.append(float(x))
                        except (ValueError, TypeError):
                            continue
                
                if numeric_data:
                    plottable_data[attr] = numeric_data
            except (ValueError, TypeError, AttributeError) as e:
                logger.warning("Error extracting plottable data for attribute '%s': %s", attr, e)
                continue  # Stop further processing for this trace if unexpected error
    
    return plottable_data



def create_matlab_plot(plottable_data, trace_name, ax):
    """Create a plot from any available numeric data"""
    plotted = False
    
    # Strategy 1: Standard x,y plot
    if 'x' in plottable_data and 'y' in plottable_data:
        x_data = plottable_data['x']
        y_data = plottable_data['y']
        ax.plot(x_data, y_data, 'o', linestyle='', label=trace_name, alpha=1)
        plotted = True
    
    # Strategy 2: Y data only (create index for x)
    elif 'y' in plottable_data:
        y_data = plottable_data['y']
        x_data = list(range(len(y_data)))
        ax.plot(x_data, y_data, 'o', linestyle='', label=trace_name, alpha=0.8)
        plotted = True
    
    # Strategy 3: X data only (histogram-like - show distribution)
    elif 'x' in plottable_data:
        x_data = plottable_data['x']
        # Create a simple histogram using matplotlib
        ax.hist(x_data, bins=min(50, len(set(x_data))), alpha=0.7, label=trace_name)
        plotted = True
    
    # Strategy 4: OHLC data (just plot close prices)
    elif 'close' in plottable_data:
        y_data = plottable_data['close']
        x_data = list(range(len(y_data)))
        
        ax.plot(x_data, y_data, '-', label=f"{trace_name} (Close)", alpha=0.8)
        plotted = True
    
    # Strategy 5: Any other numeric array
    elif plottable_data:
        # Take the first available numeric array
        attr_name, data = next(iter(plottable_data.items()))
        x_data = list(range(len(data)))
        y_data = data
        
        ax.plot(x_data, y_data, '-', label=f"{trace_name} ({attr_name})", alpha=0.8)
        plotted = True
    
    return plotted


def plotly_to_matplotlib_png(plotly_fig, plot_id, id_naming, strategy_id, version=None) -> str:
    """
    Convert any Plotly figure to a simple matplotlib PNG for LLM analysis
    
    Args:
        plotly_fig: Plotly figure object
        
    Returns:
        str: Base64 encoded PNG image
    """
    try:
        # Create matplotlib figure
        fig, ax = plt.subplots(figsize=(10, 6))
        
        # Extract data from all traces generically
        traces_plotted = 0
        for i, trace in enumerate(plotly_fig.data):
            try:
                # Extract any plottable data from this trace
                plottable_data = extract_any_plottable_data(trace)
                
                if plottable_data:
                    trace_name = getattr(trace, 'name', f'Series {i+1}')
                    
                    # Try to create a plot from available data
                    if create_matlab_plot(plottable_data, trace_name, ax):
                        traces_plotted += 1
                    
            except (ValueError, TypeError, AttributeError) as trace_error:
                logger.warning("Skipping trace %s due to error: %s", i, trace_error)
                continue
        # Apply basic styling for readability
        ax.grid(True, alpha=0.3)
        ax.set_facecolor('#f8f9fa')  # Light background for better LLM analysis
        # Add legend if multiple traces
        if traces_plotted > 1:
            ax.legend()
        # Extract title if available
        try:
            if hasattr(plotly_fig, 'layout') and hasattr(plotly_fig.layout, 'title'):
                title_text = getattr(plotly_fig.layout.title, 'text', None)
                if title_text:
                    if version:
                        title_text = f"Plot {plot_id} {id_naming}: {strategy_id} v{version} - " + title_text
                    else:
                        title_text = f"Plot {plot_id} {id_naming}: {strategy_id} - " + title_text
                else:
                    if version:
                        title_text = f"Plot {plot_id} {id_naming}: {strategy_id} v{version}"
                    else:
                        title_text = f"Plot {plot_id} {id_naming}: {strategy_id}"
                ax.set_title(title_text)
        except ValueError as e:
            logger.warning("Optional: could not set plot title: %s", e)
        # Extract axis labels if available
        try:
            if hasattr(plotly_fig, 'layout'):
                if hasattr(plotly_fig.layout, 'xaxis') and hasattr(plotly_fig.layout.xaxis, 'title'):
                    x_title = getattr(plotly_fig.layout.xaxis.title, 'text', None)
                    if x_title:
                        ax.set_xlabel(x_title)
                if hasattr(plotly_fig.layout, 'yaxis') and hasattr(plotly_fig.layout.yaxis, 'title'):
                    y_title = getattr(plotly_fig.layout.yaxis.title, 'text', None)
                    if y_title:
                        ax.set_ylabel(y_title)
        except ValueError as e:
            logger.warning("Optional: could not set axis labels: %s", e)
        # Convert to PNG bytes
        buffer = BytesIO()
        plt.savefig(buffer, format='png', bbox_inches='tight', dpi=100, 
                   facecolor='white', edgecolor='none')
        plt.close(fig)  # Important: close figure to free memory
        buffer.seek(0)
        png_bytes = buffer.read()
        png_base64 = base64.b64encode(png_bytes).decode('utf-8')
        return png_base64
        
    except ValueError as e:
        logger.error("Failed to generate matplotlib fallback chart: %s", e)
        # Return empty string on complete failure
        return "" 
