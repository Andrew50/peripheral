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


def process_tasks():
    data = Conn(True)
    print("starting queue listening", flush=True)
    while True:
        try:
            task = data.cache.brpop("queue", timeout=60)
            if not task:
                data.check_connection()
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
                    data.cache.set(task_id, json.dumps("running"))
                    start = datetime.datetime.now()
                    result = funcMap[func_ident](data, **args)

                    data.cache.set(task_id, packageResponse(result, "completed"))
                    print(f"finished {func_ident} {args} time: {datetime.datetime.now() - start}", flush=True)
                except psycopg2.InterfaceError:
                    exception = traceback.format_exc()
                    try:
                        data.cache.set(task_id, packageResponse(exception, "error"))
                    except redis.exceptions.ConnectionError:
                        print("Redis connection error when setting task error status", flush=True)
                    print(exception, flush=True)
                    data.check_connection()
                except Exception:
                    exception = traceback.format_exc()
                    try:
                        data.cache.set(task_id, packageResponse(exception, "error"))
                    except redis.exceptions.ConnectionError:
                        print("Redis connection error when setting task error status", flush=True)
                    print(exception, flush=True)
        except redis.exceptions.ConnectionError as e:
            print(f"Redis connection error: {e}", flush=True)
            print("Attempting to reconnect...", flush=True)
            # Reinitialize the connection
            data = Conn(True)
        except Exception as e:
            print(f"Unexpected error in main loop: {e}", flush=True)
            print(traceback.format_exc(), flush=True)
            # Sleep briefly to avoid tight loop if there's a persistent error
            time.sleep(5)


if __name__ == "__main__":
    process_tasks()
