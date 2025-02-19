import json, traceback, datetime, psycopg2
from conn import Conn
from train import train
from screen import screen
from trainerQueue import refillTrainerQueue
from trades import handle_trade_upload, grab_user_trades, get_trade_statistics, get_ticker_trades, get_ticker_performance
from update_sectors import update_sectors
from active import update_active

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
    "get_ticker_performance": get_ticker_performance
}


def packageResponse(result,status):
    return json.dumps({
        "status":status,
        "result":result
        })

def process_tasks():
    data = Conn(True)
    print("starting queue listening",flush=True)
    while True:
        task = data.cache.brpop('queue', timeout=60)
        if not task:
            data.check_connection()
        else:
            _, task_message = task
            task_data = json.loads(task_message)
            task_id, func_ident, args = task_data['id'], task_data['func'], task_data['args']

            print(f"starting {func_ident} {args} {task_id}", flush=True)
            try:
                data.cache.set(task_id, json.dumps('running'))
                start = datetime.datetime.now()
                result = funcMap[func_ident](data,**args)

                data.cache.set(task_id, packageResponse(result,"completed"))
                #print(f"finished {func_ident} {args} time: {datetime.datetime.now() - start} result: {result}", flush=True)
            except psycopg2.InterfaceError:
                exception = traceback.format_exc()
                data.cache.set(task_id, packageResponse(exception,"error"))
                print(exception, flush=True)
                data.check_connection()
            except:
                exception = traceback.format_exc()
                data.cache.set(task_id, packageResponse(exception,"error"))
                print(exception, flush=True)

if __name__ == "__main__":
    process_tasks()

