"""
Strategy Worker
Implements three core functions for strategy execution using DataFrame-based strategies:
- run_backtest(strategy_id): Complete backtesting across all historical days
- run_screener(strategy_id): Complete screening across all tickers
- run_alert(strategy_id): Complete alert monitoring across all tickers
"""

import json
import traceback
import datetime
import time
import psycopg2
from data_provider import DataProvider
from dataframe_strategy_engine import DataFrameStrategyEngine

# Initialize components
data_provider = DataProvider()
strategy_engine = DataFrameStrategyEngine()

def train(data, **args):
    """Run strategy training/backtesting"""
    try:
        strategy_id = args.get('strategy_id')
        # Implement basic backtest logic here
        result = strategy_engine.execute_backtest(
            strategy_code=args.get('strategy_code', ''),
            symbols=args.get('symbols', ['AAPL', 'MSFT']),
            start_date=args.get('start_date'),
            end_date=args.get('end_date')
        )
        return result
    except Exception as e:
        return {"error": str(e)}

def screen(data, **args):
    """Run strategy screening"""
    try:
        strategy_id = args.get('strategy_id')
        # Implement basic screening logic here
        result = strategy_engine.execute_screening(
            strategy_code=args.get('strategy_code', ''),
            universe=args.get('universe', ['AAPL', 'MSFT', 'GOOGL']),
            limit=args.get('limit', 100)
        )
        return result
    except Exception as e:
        return {"error": str(e)}

def alert(data, **args):
    """Run strategy alerts"""
    try:
        strategy_id = args.get('strategy_id')
        # Implement basic alert logic here
        result = strategy_engine.execute_realtime(
            strategy_code=args.get('strategy_code', ''),
            symbols=args.get('symbols', ['AAPL', 'MSFT'])
        )
        return result
    except Exception as e:
        return {"error": str(e)}

funcMap = {
    "train": train,
    "screen": screen,
    "alert": alert,
}

def packageResponse(result, status):
    return json.dumps({
        "status": status,
        "result": result
    })

def process_tasks():
    # TODO: Replace with actual connection setup
    # data = Conn(True)
    data = None
    print("starting queue listening", flush=True)
    
    while True:
        # TODO: Replace with actual Redis connection
        # task = data.cache.brpop('queue', timeout=60)
        task = None
        
        if not task:
            # data.check_connection()
            print("No tasks, waiting...", flush=True)
            time.sleep(60)
        else:
            _, task_message = task
            task_data = json.loads(task_message)
            task_id, func_ident, args = task_data['id'], task_data['func'], task_data['args']

            print(f"starting {func_ident} {args} {task_id}", flush=True)
            try:
                # data.cache.set(task_id, json.dumps('running'))
                start = datetime.datetime.now()
                result = funcMap[func_ident](data, **args)

                # data.cache.set(task_id, packageResponse(result, "completed"))
                print(f"finished {func_ident} {args} time: {datetime.datetime.now() - start} result: {result}", flush=True)
            except psycopg2.InterfaceError:
                exception = traceback.format_exc()
                # data.cache.set(task_id, packageResponse(exception, "error"))
                print(exception, flush=True)
                # data.check_connection()
            except:
                exception = traceback.format_exc()
                # data.cache.set(task_id, packageResponse(exception, "error"))
                print(exception, flush=True)

if __name__ == "__main__":
    process_tasks() 