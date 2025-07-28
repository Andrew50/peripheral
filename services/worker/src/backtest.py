import logging
from datetime import datetime, timedelta
from typing import List, Dict, Any
from utils.context import Context
from utils.strategy_crud import fetch_strategy_code
from engine import execute_strategy
from validator import ValidationError

logger = logging.getLogger(__name__)

async def backtest(ctx: Context, user_id: int = None, symbols: List[str] = None,
                            start_date: str = None, end_date: str = None,
                            strategy_id: int = None, version: int = None) -> Dict[str, Any]:
    """Execute backtest task using new accessor strategy engine"""
    if not strategy_id:
        raise ValueError("strategy_id is required")
    strategy_code, version = fetch_strategy_code(ctx, user_id, strategy_id)

    if symbols is not None and len(symbols) == 0:
        raise ValueError("symbols length must be greater than 0")

    if start_date is None or end_date is None:
        raise ValueError("start_date and end_date are required for backtest")

    parsed_start_date = datetime.strptime(start_date, '%Y-%m-%d')
    parsed_end_date = datetime.strptime(end_date, '%Y-%m-%d')

    if not parsed_start_date:
        parsed_start_date = datetime.now() - timedelta(days=365)
    if not parsed_end_date:
        parsed_end_date = datetime.now()

    if parsed_start_date > parsed_end_date:
        raise ValueError("start_date must be before end_date")


    instances, strategy_prints, strategy_plots, response_images, error = execute_strategy(
        ctx,
        strategy_code, 
        strategy_id=strategy_id,
        version=version,
        symbols=symbols,
        start_date=start_date,
        end_date=end_date,
    )
    if error:
        return {
            "success": False,
            "error_message": str(error),
            "strategy_id": strategy_id,
            "version": version,
            "instances": [],
        }

    positive_instances = sum(1 for i in instances if isinstance(i.get('score'), (int, float)) and i['score'] > 0)

    return {
        "success": True,
        "strategy_id": strategy_id,
        "version": version,
        "total_instances": len(instances),
        "positive_instances": positive_instances,
        "date_range": [start_date, end_date],
        "symbols_processed": len(symbols) if symbols else 0,
        "execution_type": "backtest",
        "instances": instances,
        "strategy_prints": strategy_prints,
        "strategy_plots": strategy_plots,
        "response_images": response_images,
        "error_message": ""
    }
