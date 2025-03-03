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


def safe_redis_operation(operation_func, fallback_value=None, max_retries=3):
    """Execute a Redis operation safely with retries"""
    retry_count = 0
    backoff_time = 1
    
    while retry_count < max_retries:
        try:
            return operation_func()
        except (redis.exceptions.ConnectionError, redis.exceptions.TimeoutError) as e:
            retry_count += 1
            print(f"Redis operation failed (attempt {retry_count}/{max_retries}): {e}", flush=True)
            if retry_count < max_retries:
                print(f"Retrying in {backoff_time} seconds...", flush=True)
                time.sleep(backoff_time)
                backoff_time *= 2
            else:
                print(f"Max retries reached for Redis operation. Using fallback value.", flush=True)
                return fallback_value


def process_tasks():
    data = Conn(True)
    print("starting queue listening", flush=True)
    consecutive_errors = 0
    
    while True:
        try:
            # Use safe Redis operation for brpop
            task = safe_redis_operation(
                lambda: data.cache.brpop("queue", timeout=60),
                fallback_value=None
            )
            
            if not task:
                # If no task or operation failed, check connection and wait
                try:
                    data.check_connection()
                except Exception as e:
                    print(f"Connection check failed: {e}", flush=True)
                    time.sleep(5)  # Wait before retrying
                continue
                
            _, task_message = task
            task_data = json.loads(task_message)
            task_id, func_ident, args = (
                task_data["id"],
                task_data["func"],
                task_data["args"],
            )

            print(f"starting {func_ident} {args} {task_id}", flush=True)
            try:
                # Use safe Redis operation for setting task status
                safe_redis_operation(
                    lambda: data.cache.set(task_id, json.dumps("running")),
                    fallback_value=None
                )
                
                start = datetime.datetime.now()
                result = funcMap[func_ident](data, **args)

                # Use safe Redis operation for setting task result
                safe_redis_operation(
                    lambda: data.cache.set(task_id, packageResponse(result, "completed")),
                    fallback_value=None
                )
                
                print(f"finished {func_ident} {args} time: {datetime.datetime.now() - start}", flush=True)
                consecutive_errors = 0  # Reset error counter on success
                
            except psycopg2.InterfaceError:
                exception = traceback.format_exc()
                safe_redis_operation(
                    lambda: data.cache.set(task_id, packageResponse(exception, "error")),
                    fallback_value=None
                )
                print(exception, flush=True)
                data.check_connection()
                
            except Exception:
                exception = traceback.format_exc()
                safe_redis_operation(
                    lambda: data.cache.set(task_id, packageResponse(exception, "error")),
                    fallback_value=None
                )
                print(exception, flush=True)
                
        except redis.exceptions.ConnectionError as e:
            consecutive_errors += 1
            print(f"Redis connection error ({consecutive_errors}): {e}", flush=True)
            print("Attempting to reconnect...", flush=True)
            
            # Exponential backoff based on consecutive errors
            backoff_time = min(30, 2 ** consecutive_errors)
            print(f"Waiting {backoff_time} seconds before reconnecting...", flush=True)
            time.sleep(backoff_time)
            
            # Reinitialize the connection
            try:
                data = Conn(True)
                print("Connection reinitialized successfully", flush=True)
            except Exception as conn_error:
                print(f"Failed to reinitialize connection: {conn_error}", flush=True)
                
        except Exception as e:
            consecutive_errors += 1
            print(f"Unexpected error in main loop ({consecutive_errors}): {e}", flush=True)
            print(traceback.format_exc(), flush=True)
            
            # Sleep with exponential backoff to avoid tight loop if there's a persistent error
            backoff_time = min(60, 2 ** consecutive_errors)
            print(f"Waiting {backoff_time} seconds before continuing...", flush=True)
            time.sleep(backoff_time)


if __name__ == "__main__":
    process_tasks()
