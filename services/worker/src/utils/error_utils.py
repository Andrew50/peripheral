"""
Shared error handling utilities for consistent error capture and logging.
"""

import traceback
import logging
from typing import Dict, Any, List


def capture_exception(logger: logging.Logger, err: Exception) -> Dict[str, Any]:
    """
    Capture exception details with full traceback and log it.
    
    Uses the exception's own traceback (__traceback__) instead of sys.exc_info()
    to ensure reliable traceback capture even when called outside except blocks.
    
    Args:
        logger: Logger instance to use for logging
        err: Exception that was caught
        
    Returns:
        Dict containing structured error information with keys:
        - type: Exception class name
        - message: Exception message string
        - traceback: Full formatted traceback string
        - frames: List of traceback lines for programmatic access
    """
    # Use the exception's own traceback for reliable capture
    if err.__traceback__ is not None:
        tb_lines = traceback.format_exception(type(err), err, err.__traceback__)
        tb_str = "".join(tb_lines)
    else:
        # Fallback to format_exc() if __traceback__ is None (shouldn't happen in normal cases)
        tb_str = traceback.format_exc()
        tb_lines = tb_str.splitlines(keepends=True)
    
    # Log once with explicit traceback to prevent double logging
    logger.error("❌ %s\n%s", err, tb_str.rstrip())
    
    return {
        "type": err.__class__.__name__,
        "message": str(err),
        "traceback": tb_str,
        "frames": tb_lines  # For programmatic access to individual traceback lines
    } 