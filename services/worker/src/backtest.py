import logging
from datetime import datetime, timedelta
from typing import List, Dict, Any
from utils.context import Context
from utils.strategy_crud import fetch_strategy_code
from engine import execute_strategy
from validator import ValidationError
from utils.error_utils import capture_exception

logger = logging.getLogger(__name__)

def backtest(ctx: Context, user_id: int = None, symbols: List[str] = None,
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

    # TODO this will need to handle minutes ideally for intraday backtests
    parsed_start_date = datetime.strptime(start_date, '%Y-%m-%d')
    parsed_end_date = datetime.strptime(end_date, '%Y-%m-%d')
    # Override the original string parameters with datetime objects
    start_date = parsed_start_date
    end_date = parsed_end_date

    if not parsed_start_date:
        parsed_start_date = datetime.now() - timedelta(days=365)
    if not parsed_end_date:
        parsed_end_date = datetime.now()

    if parsed_start_date > parsed_end_date:
        raise ValueError("start_date must be before end_date")

    try:
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
                "error": error,
                "strategy_id": strategy_id,
                "version": version,
                "instances": [],
            }
    except Exception as e:
        error_obj = capture_exception(logger, e)
        return {
            "success": False,
            "error": error_obj,
            "strategy_id": strategy_id,
            "version": version,
            "instances": [],
        }

    positive_instances = sum(1 for i in instances  \
    if isinstance(i.get('score'), (int, float)) and i['score'] > 0)

    summary = {
        "total_instances": len(instances),
        "positive_instances": positive_instances,
        "date_range": [start_date.isoformat(), end_date.isoformat()],
        "symbols_processed": len(symbols) if symbols else 0,
        "execution_type": "backtest",
    }

    return {
        "success": True,
        "strategy_id": strategy_id,
        "version": version,
        "summary": summary,
        "instances": instances,
        "strategy_prints": strategy_prints,
        "strategy_plots": strategy_plots,
        "response_images": response_images,
        "error": None
    }
