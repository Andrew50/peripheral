import json, traceback, datetime, psycopg2, redis



from conn import Conn
from train import train
from screen import screen
from trainerQueue import refillTrainerQueue
from trade_analysis import find_similar_trades
from trades import (
    handle_trade_upload,
    grab_user_trades,
    get_trade_statistics,
    get_ticker_trades,
    get_ticker_performance,
    delete_all_user_trades,
)
from update_sectors import update_sectors
from active import update_active
import time
import os

funcMap = {
    "train": train,
    "screen": screen,
    "refillTrainerQueue": refillTrainerQueue,
    "handle_trade_upload": handle_trade_upload,
    "grab_user_trades": grab_user_trades,
    "update_sectors": update_sectors,
    "update_active": update_active,
    "get_trade_statistics": get_trade_statistics,
    "get_ticker_trades": get_ticker_trades,
    "get_ticker_performance": get_ticker_performance,
    "find_similar_trades": find_similar_trades,
    "delete_all_user_trades": delete_all_user_trades,
}


def packageResponse(result, status):
    return json.dumps({"status": status, "result": result})


def safe_redis_operation(func, *args, **kwargs):
    """Execute a Redis operation with retry logic"""
    max_retries = int(os.environ.get("REDIS_RETRY_ATTEMPTS", "5"))
    retry_delay = int(os.environ.get("REDIS_RETRY_DELAY", "1"))
    
    for attempt in range(max_retries):
        try:
            return func(*args, **kwargs)
        except redis.exceptions.ConnectionError as e:
            if attempt < max_retries - 1:
                print(f"Redis operation failed (attempt {attempt+1}/{max_retries}): {e}", flush=True)
                time.sleep(retry_delay * (2 ** attempt))  # Exponential backoff
            else:
                print(f"Redis operation failed after {max_retries} attempts: {e}", flush=True)
                raise


def process_tasks():
    data = None
    reconnect_delay = 1
    max_reconnect_delay = 60
    
    while True:
        try:
            if data is None:
                data = Conn(True)
            
            print("starting queue listening", flush=True)
            
            while True:
                try:
                    # Use a shorter timeout to allow for more frequent connection checks
                    task = safe_redis_operation(data.cache.brpop, "queue", timeout=30)
                    
                    if not task:
                        # No task received, check connection and continue
                        try:
                            data.check_connection()
                            reconnect_delay = 1  # Reset backoff on successful check
                        except Exception as e:
                            print(f"Connection check failed: {e}", flush=True)
                            raise  # Re-raise to trigger reconnection
                    else:
                        _, task_message = task
                        task_data = json.loads(task_message)
                        task_id, func_ident, args = (
                            task_data["id"],
                            task_data["func"],
                            task_data["args"],
                        )

                        print(f"starting {func_ident} {args} {task_id}", flush=True)
                        try:
                            safe_redis_operation(data.cache.set, task_id, json.dumps("running"))
                            start = datetime.datetime.now()
                            result = funcMap[func_ident](data, **args)

                            safe_redis_operation(data.cache.set, task_id, packageResponse(result, "completed"))
                            print(f"finished {func_ident} {args} time: {datetime.datetime.now() - start}", flush=True)
                        except psycopg2.InterfaceError:
                            exception = traceback.format_exc()
                            try:
                                safe_redis_operation(data.cache.set, task_id, packageResponse(exception, "error"))
                            except redis.exceptions.ConnectionError:
                                print("Redis connection error when setting task error status", flush=True)
                            print(exception, flush=True)
                            data.check_connection()
                        except Exception:
                            exception = traceback.format_exc()
                            try:
                                safe_redis_operation(data.cache.set, task_id, packageResponse(exception, "error"))
                            except redis.exceptions.ConnectionError:
                                print("Redis connection error when setting task error status", flush=True)
                            print(exception, flush=True)
                
                except redis.exceptions.ConnectionError as e:
                    print(f"Redis connection error in task loop: {e}", flush=True)
                    print("Attempting to reconnect...", flush=True)
                    # Break inner loop to reinitialize connection
                    raise
        
        except Exception as e:
            print(f"Error in main process loop: {e}", flush=True)
            print(traceback.format_exc(), flush=True)
            
            # Reset connection
            data = None
            
            # Sleep with exponential backoff before retrying
            print(f"Retrying connection in {reconnect_delay} seconds...", flush=True)
            time.sleep(reconnect_delay)
            reconnect_delay = min(reconnect_delay * 2, max_reconnect_delay)


if __name__ == "__main__":
    process_tasks()
