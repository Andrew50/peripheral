import time
import logging
from utils.context import Context
from utils.strategy_crud import fetch_strategy_code
from engine import execute_strategy
from validator import ValidationError
from datetime import datetime, timedelta
from typing import List, Dict, Any

logger = logging.getLogger(__name__)

async def alert(ctx: Context, user_id: int = None, symbols: List[str] = None,
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
        )

        if error:
            logger.error("🔔❌ Strategy alert execution failed: %s", error)
            return {
                'success': False,
                'error_message': str(error),
            }

        return {
            'success': True,
            'instances': instances,
            'error_message': "",
        }

    except Exception as e:
        logger.error("Error during alert processing: %s", e)
        return {
            'success': False,
            'instances': [],
            'error_message': str(e),
        }
