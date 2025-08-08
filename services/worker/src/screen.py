"""
Screening module
"""

import logging
from typing import List, Dict, Any
from .engine import execute_strategy
from .utils.context import Context
#from .utils.tracked_list import TrackedList
from .utils.strategy_crud import fetch_multiple_strategy_codes
from .utils.error_utils import capture_exception

logger = logging.getLogger(__name__)

def screen(ctx: Context, user_id: str, strategy_ids: List[str] = None, universe: List[str] = None) -> Dict[str, Any]:
    """Execute screening task using new accessor strategy engine"""
    if not strategy_ids:
        raise ValueError("strategy_ids is required")

    if len(strategy_ids) > 1:
        raise NotImplementedError("Multi-strategy screening is not implemented yet")
    
    strategy_id = strategy_ids[0]

    try:
        strategy_codes = fetch_multiple_strategy_codes(ctx, user_id, [strategy_id])

        if not strategy_codes or strategy_id not in strategy_codes:
            raise ValueError(f"No valid strategy code found for strategy_id: {strategy_id}")
        
        strategy_code = strategy_codes[strategy_id]
        
        # Use provided universe or let strategy determine requirements
        target_universe = universe or None

        # Execute strategy with accessor context
        instances, _, _, _, error = execute_strategy(
            ctx,
            strategy_code,
            strategy_id=strategy_id,
            symbols=target_universe
        )

        if error:
            return {
                'success': False,
                'instances': [],
                'error_details': error
            }
            
        # Rank and limit results
        ranked_results = _rank_screening_results(instances)

        return {
            'success': True,
            'instances': ranked_results
        }

    except ValueError as e:
        error_obj = capture_exception(logger, e)
        return {
            'success': False,
            'instances': [],
            'error_details': error_obj
        }


def _rank_screening_results(instances: List[Dict]) -> List[Dict]:
    """Rank screening results by score or other criteria and convert to WorkerRankedResult format"""

    # Sort by score if available, otherwise by timestamp descending (most recent first)
    def sort_key(instance):
        if 'score' in instance:
            return instance['score']
        # Use timestamp for sorting if no score - more recent = higher priority
        return instance.get('timestamp', 0)

    sorted_instances = sorted(instances, key=sort_key, reverse=True)
    return sorted_instances
