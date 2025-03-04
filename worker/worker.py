import json, traceback, datetime, psycopg2, redis
import random
import os


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
    """Execute a Redis operation with retry logic and improved timeout handling"""
    max_retries = int(os.environ.get("REDIS_RETRY_ATTEMPTS", "5"))
    base_retry_delay = float(os.environ.get("REDIS_RETRY_DELAY", "1"))
    max_retry_delay = float(os.environ.get("REDIS_MAX_RETRY_DELAY", "10"))  # Cap the maximum delay
    
    for attempt in range(max_retries):
        try:
            # Set a timeout for the operation if not already specified
            if 'timeout' not in kwargs and hasattr(func, '__name__'):
                if func.__name__ == 'brpop':
                    # Use a shorter timeout for blocking operations
                    kwargs['timeout'] = 5
                elif func.__name__ in ['get', 'set', 'ping']:
                    # Use a short timeout for simple operations
                    kwargs['timeout'] = 2
            
            return func(*args, **kwargs)
        except (redis.exceptions.ConnectionError, redis.exceptions.TimeoutError) as e:
            if attempt < max_retries - 1:
                # Calculate delay with exponential backoff and jitter, but cap it
                retry_delay = min(base_retry_delay * (2 ** attempt), max_retry_delay)
                jitter = random.uniform(0, 0.1 * retry_delay)  # 10% jitter
                total_delay = retry_delay + jitter
                
                print(f"Redis operation failed (attempt {attempt+1}/{max_retries}): {e}. Retrying in {total_delay:.2f}s", flush=True)
                
                # Use shorter sleep intervals with checks to allow for cleaner interruption
                sleep_interval = 0.5
                sleep_count = int(total_delay / sleep_interval)
                
                for _ in range(sleep_count):
                    time.sleep(sleep_interval)
                
                # Sleep any remaining time
                remaining_time = total_delay - (sleep_count * sleep_interval)
                if remaining_time > 0:
                    time.sleep(remaining_time)
            else:
                print(f"Redis operation failed after {max_retries} attempts: {e}", flush=True)
                raise
        except Exception as e:
            print(f"Unexpected Redis error: {e}", flush=True)
            if attempt < max_retries - 1:
                retry_delay = min(base_retry_delay * (2 ** attempt), max_retry_delay)
                print(f"Retrying in {retry_delay:.2f}s", flush=True)
                time.sleep(retry_delay)
            else:
                raise

def process_tasks():
    data = None
    reconnect_delay = 1
    max_reconnect_delay = 30  # Reduced from 60 to avoid long waits
    
    while True:
        try:
            if data is None:
                data = Conn(True)
                print("Successfully connected to both database and Redis", flush=True)
            
            print("starting queue listening", flush=True)
            
            while True:
                try:
                    # Use a shorter timeout for brpop to allow for more frequent connection checks
                    task = safe_redis_operation(data.cache.brpop, "queue", timeout=5)
                    
                    if not task:
                        # No task received, check connection and continue
                        try:
                            # Ping Redis to keep connection alive
                            safe_redis_operation(data.cache.ping)
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
                            
                            # Ping Redis after task completion to keep connection alive
                            safe_redis_operation(data.cache.ping)
                        except psycopg2.InterfaceError:
                            exception = traceback.format_exc()
                            try:
                                safe_redis_operation(data.cache.set, task_id, packageResponse(exception, "error"))
                            except (redis.exceptions.ConnectionError, redis.exceptions.TimeoutError):
                                print("Redis connection error when setting task error status", flush=True)
                            print(exception, flush=True)
                            data.check_connection()
                        except Exception:
                            exception = traceback.format_exc()
                            try:
                                safe_redis_operation(data.cache.set, task_id, packageResponse(exception, "error"))
                            except (redis.exceptions.ConnectionError, redis.exceptions.TimeoutError):
                                print("Redis connection error when setting task error status", flush=True)
                            print(exception, flush=True)
                
                except (redis.exceptions.ConnectionError, redis.exceptions.TimeoutError) as e:
                    print(f"Redis connection error in task loop: {e}", flush=True)
                    print("Attempting to reconnect...", flush=True)
                    # Break inner loop to reinitialize connection
                    raise
        
        except Exception as e:
            print(f"Error in main process loop: {e}", flush=True)
            print(traceback.format_exc(), flush=True)
            
            # Reset connection
            data = None
            
            # Sleep with exponential backoff before retrying, but with a more reasonable cap
            print(f"Retrying connection in {reconnect_delay} seconds...", flush=True)
            
            # Use shorter sleep intervals to allow for cleaner interruption
            sleep_interval = 0.5
            sleep_count = int(reconnect_delay / sleep_interval)
            
            for _ in range(sleep_count):
                time.sleep(sleep_interval)
            
            # Sleep any remaining time
            remaining_time = reconnect_delay - (sleep_count * sleep_interval)
            if remaining_time > 0:
                time.sleep(remaining_time)
                
            reconnect_delay = min(reconnect_delay * 2, max_reconnect_delay)


if __name__ == "__main__":
    process_tasks()
