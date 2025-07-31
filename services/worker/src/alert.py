import logging
from typing import Any, Dict, List

from engine import execute_strategy
from utils.context import Context
from utils.strategy_crud import fetch_strategy_code
from utils.error_utils import capture_exception

logger = logging.getLogger(__name__)

def alert(ctx: Context, user_id: int = None, symbols: List[str] = None,
                    strategy_id: int = None) -> Dict[str, Any]:
    """Execute alert task using new accessor strategy engine"""

    if not strategy_id:
        raise ValueError("strategy_id is required")
    
    try:
        strategy_code, version = fetch_strategy_code(ctx, user_id, strategy_id)

        if symbols is not None and len(symbols) == 0:
            raise ValueError("symbols length must be greater than 0")

        # start_date and end_date are not used for alerts, engine will use defaults
        instances, _, _, _, error = execute_strategy(
            ctx,
            strategy_code,
            strategy_id=strategy_id,
            version=version,
            symbols=symbols,
            start_date=None,
            end_date=None,
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

    except Exception as e:
        error_obj = capture_exception(logger, e)
        return {
            'success': False,
            'instances': [],
            'error': error_obj,
        }
