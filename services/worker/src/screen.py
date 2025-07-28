"""
Screening module
"""

import time
import logging
from typing import List, Dict, Any
from .engine import execute_strategy
from .utils.context import Context
#from .utils.tracked_list import TrackedList
from .utils.strategy_crud import fetch_multiple_strategy_codes

logger = logging.getLogger(__name__)

async def screen(ctx: Context,user_id: str,strategy_ids: List[str] = None,universe: List[str] = None) -> Dict[str, Any]:
    """Execute screening task using new accessor strategy engine"""
    if not strategy_ids:
        raise ValueError("strategy_ids is required")

    if len(strategy_ids) > 1:
        raise NotImplementedError("Multi-strategy screening is not implemented yet")

    strategy_codes = fetch_multiple_strategy_codes(ctx,user_id,strategy_ids)

    if strategy_codes:
        strategy_code = list(strategy_codes.values())[0]
    else:
        raise ValueError("No valid strategy codes found for provided strategy_ids")


    # Use provided universe or let strategy determine requirements
    target_universe = universe or None

    # Execute using accessor strategy engine
    start_time = time.time()

    try:


        # Execute strategy with accessor context
        instances, _, _, _, error = execute_strategy(
            ctx,
            strategy_code,
            strategy_id=strategy_ids[0],
            start_date=None,
            end_date=None,
            symbols=target_universe
        )
        if error:
            raise error
        # Rank and limit results
        ranked_results = _rank_screening_results(instances)

        execution_time = (time.time() - start_time) * 1000

        result = {
            'success': True,
            'execution_mode': 'screening',
            'ranked_results': ranked_results,
            'universe_size': len(universe),
            'results_returned': len(ranked_results),
            'execution_time_ms': int(execution_time)
        }

        logger.debug("Performance: %d results/second", len(ranked_results)/execution_time)
        return result

    except Exception as e:
        logger.error("Error during screening: %s", e)
        raise e

def _rank_screening_results(instances: List[Dict]) -> List[Dict]:
    """Rank screening results by score or other criteria and convert to WorkerRankedResult format"""

    # Sort by score if available, otherwise by timestamp descending (most recent first)
    def sort_key(instance):
        if 'score' in instance:
            return instance['score']
        else:
            # Use timestamp for sorting if no score - more recent = higher priority
            return instance.get('timestamp', 0)

    sorted_instances = sorted(instances, key=sort_key, reverse=True)
    return sorted_instances
