import logging
from typing import Any, Dict, List, Optional

from .engine import execute_strategy
from .utils.context import Context
from .utils.strategy_crud import fetch_strategy_code

logger = logging.getLogger(__name__)

def alert(
    ctx: Context,
    user_id: Optional[int] = None,
    symbols: Optional[List[str]] = None,
    strategy_id: Optional[int] = None,
) -> Dict[str, Any]:
    """Execute alert task using new accessor strategy engine"""

    if strategy_id is None:
        raise ValueError("strategy_id is required")
    if user_id is None:
        raise ValueError("user_id is required")
    
    #try:
    strategy_code, version = fetch_strategy_code(ctx, user_id, strategy_id)

    if symbols is not None and len(symbols) == 0:
        raise ValueError("symbols length must be greater than 0")

    # start_date and end_date are not used for alerts; rely on engine defaults
    instances, _, _, _, error = execute_strategy(
        ctx,
        strategy_code,
        strategy_id=strategy_id,
        version=version,
        symbols=symbols,
    )

    if error:
        return {
            'success': False,
            'instances': [],
            'error': error,
        }

    return {
        'success': True,
        'instances': instances,
        'error': None,
    }

    #except Exception as exc:  # pylint: disable=broad-except
        #error_obj = capture_exception(logger, exc)
        #return {
            #'success': False,
            #'instances': [],
            #'error': error_obj,
        #}
