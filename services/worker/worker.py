import json, traceback, datetime, psycopg2, redis
import random
import os
import sys


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

def add_task_log(data, task_id, message, level="info"):
    """Add a log entry to a task"""
    try:
        # Get the current task
        task_json = safe_redis_operation(data.cache.get, task_id)
        if not task_json:
            print(f"Warning: Could not find task {task_id} to add log", flush=True)
            return
        
        task = json.loads(task_json)
        
        # Create a log entry
        log_entry = {
            "timestamp": datetime.datetime.now().isoformat(),
            "message": message,
            "level": level
        }
        
        # Add the log entry to the task
        if "logs" not in task:
            task["logs"] = []
        
        task["logs"].append(log_entry)
        task["updatedAt"] = datetime.datetime.now().isoformat()
        
        # Save the updated task
        safe_redis_operation(data.cache.set, task_id, json.dumps(task))
    except Exception as e:
        print(f"Error adding log to task {task_id}: {e}", flush=True)

def update_task_state(data, task_id, state):
    """Update the state of a task"""
    try:
        # Get the current task
        task_json = safe_redis_operation(data.cache.get, task_id)
        if not task_json:
            print(f"Warning: Could not find task {task_id} to update state", flush=True)
            return
        
        task = json.loads(task_json)
        
        # Update the state
        task["state"] = state
        task["updatedAt"] = datetime.datetime.now().isoformat()
        
        # Save the updated task
        safe_redis_operation(data.cache.set, task_id, json.dumps(task))
    except Exception as e:
        print(f"Error updating task state for {task_id}: {e}", flush=True)

def set_task_result(data, task_id, result):
    """Set the result of a completed task"""
    try:
        # Get the current task
        task_json = safe_redis_operation(data.cache.get, task_id)
        if not task_json:
            print(f"Warning: Could not find task {task_id} to set result", flush=True)
            return
        
        task = json.loads(task_json)
        
        # Update the task
        task["result"] = result
        task["state"] = "completed"
        task["updatedAt"] = datetime.datetime.now().isoformat()
        
        # Save the updated task
        safe_redis_operation(data.cache.set, task_id, json.dumps(task))
    except Exception as e:
        print(f"Error setting task result for {task_id}: {e}", flush=True)

def set_task_error(data, task_id, error_message):
    """Set the error of a failed task"""
    try:
        # Get the current task
        task_json = safe_redis_operation(data.cache.get, task_id)
        if not task_json:
            print(f"Warning: Could not find task {task_id} to set error", flush=True)
            return
        
        task = json.loads(task_json)
        
        # Update the task
        task["error"] = error_message
        task["state"] = "failed"
        task["updatedAt"] = datetime.datetime.now().isoformat()
        
        # Save the updated task
        safe_redis_operation(data.cache.set, task_id, json.dumps(task))
    except Exception as e:
        print(f"Error setting task error for {task_id}: {e}", flush=True)

def safe_redis_operation(func, *args, **kwargs):
    """Execute a Redis operation with retry logic and improved timeout handling"""
    max_retries = int(os.environ.get("REDIS_RETRY_ATTEMPTS", "5"))
    base_retry_delay = float(os.environ.get("REDIS_RETRY_DELAY", "1"))
    max_retry_delay = float(os.environ.get("REDIS_MAX_RETRY_DELAY", "10"))  # Cap the maximum delay
    
    for attempt in range(max_retries):
        try:
            # Special handling for brpop which needs timeout as a positional argument
            if func.__name__ == 'brpop' and 'timeout' in kwargs:
                timeout_val = kwargs.pop('timeout')
                return func(*args, timeout_val)
            
            # For all other Redis operations, don't add a timeout parameter
            # Redis operations like 'set', 'get', 'ping' don't accept a timeout parameter
            # They rely on the socket_timeout configured at the connection level
            return func(*args, **kwargs)
            
        except redis.exceptions.TimeoutError as e:
            # Handle timeout errors specifically - these often indicate network issues
            if attempt < max_retries - 1:
                retry_delay = min(base_retry_delay * (2 ** attempt), max_retry_delay)
                jitter = random.uniform(0, 0.1 * retry_delay)  # 10% jitter
                total_delay = retry_delay + jitter
                
                print(f"Redis operation timed out (attempt {attempt+1}/{max_retries}): {e}. Retrying in {total_delay:.2f}s", flush=True)
                
                # For timeout errors, we should try to reset the connection if possible
                if hasattr(func, '__self__') and hasattr(func.__self__, 'connection_pool'):
                    try:
                        # Try to reset the connection pool
                        print("Attempting to reset Redis connection pool...", flush=True)
                        func.__self__.connection_pool.reset()
                    except Exception as reset_error:
                        print(f"Failed to reset connection pool: {reset_error}", flush=True)
                
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
                print(f"Redis operation timed out after {max_retries} attempts: {e}", flush=True)
                raise
                
        except redis.exceptions.ConnectionError as e:
            if attempt < max_retries - 1:
                # Calculate delay with exponential backoff and jitter, but cap it
                retry_delay = min(base_retry_delay * (2 ** attempt), max_retry_delay)
                jitter = random.uniform(0, 0.1 * retry_delay)  # 10% jitter
                total_delay = retry_delay + jitter
                
                print(f"Redis connection error (attempt {attempt+1}/{max_retries}): {e}. Retrying in {total_delay:.2f}s", flush=True)
                
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
                print(f"Redis connection error after {max_retries} attempts: {e}", flush=True)
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
                # Reset reconnect delay after successful connection
                reconnect_delay = 1
            
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
                            # Reset backoff on successful check
                            reconnect_delay = 1
                        except Exception as e:
                            print(f"Connection check failed: {e}", flush=True)
                            # Try to reset the connection pool before raising
                            try:
                                data.cache.connection_pool.reset()
                                print("Reset Redis connection pool after failed ping", flush=True)
                            except Exception as reset_error:
                                print(f"Failed to reset connection pool: {reset_error}", flush=True)
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
                        
                        # Create a custom stdout capture class to capture logs
                        class LogCapture:
                            def __init__(self, task_id, data):
                                self.task_id = task_id
                                self.data = data
                                self.buffer = ""
                                
                            def write(self, message):
                                # Write to the original stdout
                                sys.__stdout__.write(message)
                                sys.__stdout__.flush()
                                
                                # Buffer the message until we get a newline
                                self.buffer += message
                                if '\n' in message:
                                    lines = self.buffer.split('\n')
                                    # Process all complete lines
                                    for line in lines[:-1]:
                                        if line.strip():  # Only log non-empty lines
                                            add_task_log(self.data, self.task_id, line.strip())
                                    # Keep any incomplete line in the buffer
                                    self.buffer = lines[-1]
                            
                            def flush(self):
                                sys.__stdout__.flush()
                                # If there's anything in the buffer, log it
                                if self.buffer.strip():
                                    add_task_log(self.data, self.task_id, self.buffer.strip())
                                    self.buffer = ""
                        
                        # Redirect stdout to capture logs
                        original_stdout = sys.stdout
                        sys.stdout = LogCapture(task_id, data)
                        
                        try:
                            # Set task status to running
                            update_task_state(data, task_id, "running")
                            add_task_log(data, task_id, f"Starting task {func_ident}")
                            
                            start = datetime.datetime.now()
                            
                            # Special handling for job functions
                            if func_ident.startswith("job_"):
                                # For job functions, just mark them as completed without executing
                                add_task_log(data, task_id, f"Job function {func_ident} marked as completed")
                                result = json.dumps({"status": "success", "message": f"Job {func_ident} processed"})
                            else:
                                # For regular worker functions, execute them if they exist in the function map
                                if func_ident in funcMap:
                                    result = funcMap[func_ident](data, **args)
                                else:
                                    raise KeyError(f"Function '{func_ident}' not found in function map")

                            # Set task status to completed with result
                            set_task_result(data, task_id, result)
                            add_task_log(data, task_id, f"Task completed in {datetime.datetime.now() - start}")
                            print(f"finished {func_ident} {args} time: {datetime.datetime.now() - start}", flush=True)
                            
                            # Ping Redis after task completion to keep connection alive
                            try:
                                safe_redis_operation(data.cache.ping)
                            except Exception as e:
                                print(f"Failed to ping Redis after task completion: {e}", flush=True)
                                # Try to reset the connection pool
                                try:
                                    data.cache.connection_pool.reset()
                                    print("Reset Redis connection pool after failed ping", flush=True)
                                except Exception as reset_error:
                                    print(f"Failed to reset connection pool: {reset_error}", flush=True)
                        except psycopg2.InterfaceError:
                            exception = traceback.format_exc()
                            set_task_error(data, task_id, exception)
                            add_task_log(data, task_id, f"Database interface error: {exception}", "error")
                            print(exception, flush=True)
                            # Check and potentially reconnect to the database
                            try:
                                data.check_connection()
                            except Exception as conn_err:
                                print(f"Failed to check/reconnect to database: {conn_err}", flush=True)
                                # Force a full reconnection on the next iteration
                                data = None
                                break
                        except Exception:
                            exception = traceback.format_exc()
                            set_task_error(data, task_id, exception)
                            add_task_log(data, task_id, f"Task failed with error: {exception}", "error")
                            print(exception, flush=True)
                        finally:
                            # Restore original stdout
                            sys.stdout.flush()
                            sys.stdout = original_stdout
                
                except (redis.exceptions.ConnectionError, redis.exceptions.TimeoutError) as e:
                    print(f"Redis connection error in task loop: {e}", flush=True)
                    print("Attempting to reconnect...", flush=True)
                    # Try to reset the connection pool before breaking
                    try:
                        data.cache.connection_pool.reset()
                        print("Reset Redis connection pool after connection error", flush=True)
                    except Exception as reset_error:
                        print(f"Failed to reset connection pool: {reset_error}", flush=True)
                    # Break inner loop to reinitialize connection
                    data = None
                    break
        
        except Exception as e:
            print(f"Error in main process loop: {e}", flush=True)
            print(traceback.format_exc(), flush=True)
            
            # Reset connection
            data = None
            
            # Sleep with exponential backoff before retrying, but with a more reasonable cap
            print(f"Retrying connection in {reconnect_delay} seconds...", flush=True)
            
            # Use shorter sleep intervals with checks to allow for cleaner interruption
            sleep_interval = 1
            sleep_count = int(reconnect_delay / sleep_interval)
            
            for _ in range(sleep_count):
                time.sleep(sleep_interval)
            
            # Sleep any remaining time
            remaining_time = reconnect_delay - (sleep_count * sleep_interval)
            if remaining_time > 0:
                time.sleep(remaining_time)
            
            # Increase backoff for next attempt, but cap it
            reconnect_delay = min(reconnect_delay * 2, max_reconnect_delay)


if __name__ == "__main__":
    process_tasks()
