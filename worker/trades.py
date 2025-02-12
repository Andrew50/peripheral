import base64
import pandas as pd
import io
from screen import getCurrentSecId
import traceback
from datetime import datetime
from decimal import Decimal
import pytz


def grab_user_trades(conn, user_id: int, sort: str = "desc", date: str = None, hour: int = None, ticker: str = None):
    """
    Fetch all trades for a user with optional sorting and filtering
    
    Args:
        conn: Database connection
        user_id (int): User ID
        sort (str): Sort direction - "asc" or "desc"
        date (str): Optional date filter in format 'YYYY-MM-DD'
        hour (int): Optional hour filter (0-23)
        ticker (str): Optional ticker filter
    """
    try:
        with conn.db.cursor() as cursor:
            base_query = """
                SELECT 
                    t.*,
                    array_length(entry_times, 1) as num_entries,
                    array_length(exit_times, 1) as num_exits
                FROM trades t
                WHERE t.userId = %s
            """
            params = [user_id]
            
            # Add date filter if provided
            if date:
                base_query += " AND DATE(t.entry_times[1]) = %s"
                params.append(date)
            
            # Add hour filter if provided
            if hour is not None:
                base_query += " AND EXTRACT(HOUR FROM t.entry_times[1]) = %s"
                params.append(hour)

            # Add ticker filter if provided
            if ticker:
                base_query += " AND t.ticker = %s"
                params.append(ticker)
            
            # Add sorting
            sort_direction = "DESC" if sort.lower() == "desc" else "ASC"
            base_query += f" ORDER BY t.entry_times[1] {sort_direction}"
            
            cursor.execute(base_query, tuple(params))
            
            trades = []
            eastern = pytz.timezone('America/New_York')
            utc = pytz.UTC
            
            for row in cursor.fetchall():
                # Convert EST timestamp to UTC before getting Unix timestamp
                est_time = eastern.localize(row[9][0]) if row[9] else None
                utc_time = est_time.astimezone(utc) if est_time else None
                timestamp = int(utc_time.timestamp() * 1000) if utc_time else None
                
                trade = {
                    'ticker': row[3],
                    'securityId': row[2],
                    'tradeStart': eastern.localize(row[9][0]).astimezone(utc).timestamp() * 1000 if row[9] else None,
                    'timestamp': eastern.localize(row[12][-1]).astimezone(utc).timestamp() * 1000 if row[12] else (eastern.localize(row[9][0]).astimezone(utc).timestamp() * 1000 if row[9] else None),
                    'trade_direction': row[4],
                    'date': row[5].strftime('%Y-%m-%d'),
                    'status': row[6],
                    'openQuantity': row[7],
                    'closedPnL': float(row[8]) if row[8] else None,
                    'entries': [
                        {
                            'time': eastern.localize(row[9][i]).astimezone(utc).timestamp() * 1000,
                            'price': float(row[10][i]),
                            'shares': row[11][i]
                        }
                        for i in range(len(row[9])) if row[9]
                    ],
                    'exits': [
                        {
                            'time': eastern.localize(row[12][i]).astimezone(utc).timestamp() * 1000,
                            'price': float(row[13][i]),
                            'shares': row[14][i]
                        }
                        for i in range(len(row[12])) if row[12]
                    ]
                }
                trades.append(trade)
            
            return trades
            
    except Exception as e:
        error_info = traceback.format_exc()
        print(f"Error fetching trades:\n{error_info}")
        return []

def handle_trade_upload(conn, file_content: str, user_id: int, additional_args: dict = None) -> dict:
    """
    Process uploaded trade file and return parsed trades
    """
    try:
        # Decode base64 string back to bytes
        file_bytes = base64.b64decode(file_content)
        
        # Read CSV from bytes using pandas
        df = pd.read_csv(io.BytesIO(file_bytes), skiprows=3, dtype={
            'Order Time': str,
            'Trade Description': str,  # Ensure Trade Description is read as string
            'Status': str
        })
        df = df.dropna(how='all')

        # Start a single transaction for all trades
        with conn.db.cursor() as cursor:
            for i in range(len(df)-1, -1, -1):
                trade = df.iloc[i]
                trade_description = trade['Trade Description']
                fidelity_trade_status = trade['Status']
                
                if 'Short' in trade_description or 'Sell to Open' in trade_description or 'Buy to Cover' in trade_description or 'Buy to Close' in trade_description:
                    trade_direction = "Short" 
                else: 
                    trade_direction = "Long"
                    
                trade_dt, trade_date = parse_datetime(trade['Order Time'])
                ticker = trade['Symbol']
                securityId = getCurrentSecId(conn, ticker)
                
                if 'Limit' in trade_description or 'Stop Loss' in trade_description:
                    if 'FILLED AT' in fidelity_trade_status:
                        trade_price = float(fidelity_trade_status.split('$')[1])
                        trade_shares = float(trade['Quantity'].replace(',', ''))
                    elif 'PARTIAL' in fidelity_trade_status:
                        trade_shares = float(fidelity_trade_status.split('\n')[3].split(' ')[0].replace(',', ''))
                        description_split = trade_description.split('$')
                        if(len(description_split) == 2):
                            trade_price = float(description_split[1])
                        elif len(description_split) == 3: 
                            trade_price = float(description_split[2])
                elif 'Market' in trade_description: 
                    trade_price = float(fidelity_trade_status.split('$')[1])
                    trade_shares = float(trade['Quantity'].replace(',', ''))
                
                if 'Sell' in trade_description or 'Short' in trade_description: 
                    trade_shares = -trade_shares

                cursor.execute("""
                    INSERT INTO trade_executions 
                    (userId, securityId, ticker, date, price, size, timestamp, direction)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                """, (user_id, securityId, ticker, trade_date, trade_price, trade_shares, trade_dt, trade_direction))

            # Commit the transaction after all trades are processed
            conn.db.commit()
        process_trades(conn, user_id)
        return {
            "status": "success",
            "message": "Trades uploaded successfully"
        }      
    except Exception as e:
        # Rollback on error
        conn.db.rollback()
        error_info = traceback.format_exc()
        print(f"Error processing trade file:\n{error_info}")
        return {
            "status": "error",
            "message": f"Error: {str(e)}\nTraceback:\n{error_info}"
        }

def parse_datetime(datetime_str):
    datetime_str = ' '.join(datetime_str.split()) 
    try: 
        dt = datetime.strptime(datetime_str, '%I:%M:%S %p %m/%d/%Y')
        date_only_str = dt.date().strftime('%m/%d/%Y') 

        return dt, date_only_str    
    except ValueError as e:
        print(f"Error parsing datetime: {e}")
        return None, None

def process_trades(conn, user_id: int):
    """Process trade_executions into consolidated trades"""
    try:
        with conn.db.cursor() as cursor:
            print("\nStarting process_trades", flush=True)
            cursor.execute("""
                SELECT te.* 
                FROM trade_executions te
                WHERE te.userId = %s AND te.tradeId IS NULL
                ORDER BY te.timestamp ASC
            """, (user_id,))

            executions = cursor.fetchall()
            print(f"\nFound {len(executions)} unprocessed executions", flush=True)

            for execution in executions:
                print(f"\nProcessing execution: {execution}", flush=True)
                execution_id = execution[0]
                ticker = execution[3]
                securityId = execution[2]
                trade_date = execution[4]
                trade_price = float(execution[5])
                trade_size = int(execution[6])   
                trade_ts = execution[7]           
                direction = execution[8]          

                cursor.execute("""
                    SELECT * 
                    FROM trades
                    WHERE userId = %s
                      AND ticker = %s
                      AND status = 'Open'
                    ORDER BY date DESC
                    LIMIT 1
                """, (user_id, ticker))

                open_trade = cursor.fetchone()

                if not open_trade:
                    print("\nCreating new trade", flush=True)
                    open_quantity = trade_size  

                    cursor.execute("""
                        INSERT INTO trades (
                            userId, securityId, ticker, tradeDirection, date, status, openQuantity,
                            entry_times, entry_prices, entry_shares,
                            exit_times, exit_prices, exit_shares
                        )
                        VALUES (
                            %s, %s, %s, %s, %s, 'Open', %s,
                            ARRAY[%s], ARRAY[%s], ARRAY[%s],
                            ARRAY[]::timestamp[], ARRAY[]::decimal(10,4)[], ARRAY[]::int[]
                        )
                        RETURNING tradeId
                    """, (
                        user_id, securityId, ticker, direction, trade_date, open_quantity,
                        trade_ts, trade_price, trade_size       
                    ))

                    new_trade_id = cursor.fetchone()[0]
                    cursor.execute("""
                        UPDATE trade_executions
                        SET tradeId = %s
                        WHERE executionId = %s
                    """, (new_trade_id, execution_id))

                else:
                    print("\nUpdating existing trade", flush=True)
                    trade_id = open_trade[0]
                    trade_dir = open_trade[4]   
                    old_open_qty = float(open_trade[7])  
                    new_open_qty = old_open_qty + trade_size  
                    print(f"\nTrade details - Direction: {trade_dir}, Old Qty: {old_open_qty}, New Qty: {new_open_qty}", flush=True)

                    if trade_dir == direction:
                        is_same_direction = (old_open_qty > 0 and trade_size > 0) or (old_open_qty < 0 and trade_size < 0)
                        print(f"\nSame direction check: {is_same_direction}", flush=True)
                        
                        if is_same_direction:
                            print("\nAdding to position", flush=True)
                            cursor.execute("""
                                UPDATE trades
                                SET entry_times = array_append(entry_times, %s),
                                    entry_prices = array_append(entry_prices, %s),
                                    entry_shares = array_append(entry_shares, %s),
                                    openQuantity = %s
                                WHERE tradeId = %s
                            """, (trade_ts, trade_price, trade_size, new_open_qty, trade_id))
                        else:
                            print("\nReducing position (same direction)", flush=True)
                            cursor.execute("""
                                UPDATE trades
                                SET exit_times = array_append(exit_times, %s),
                                    exit_prices = array_append(exit_prices, %s),
                                    exit_shares = array_append(exit_shares, %s),
                                    openQuantity = %s,
                                    status = CASE
                                        WHEN %s = 0 THEN 'Closed'
                                        ELSE 'Open'
                                    END
                                WHERE tradeId = %s
                            """, (trade_ts, trade_price, trade_size, new_open_qty, new_open_qty, trade_id))

                    else:
                        print("\nReducing position (opposite direction)", flush=True)
                        cursor.execute("""
                            UPDATE trades
                            SET exit_times = array_append(exit_times, %s),
                                exit_prices = array_append(exit_prices, %s),
                                exit_shares = array_append(exit_shares, %s),
                                openQuantity = %s,
                                status = CASE
                                    WHEN %s = 0 THEN 'Closed'
                                    ELSE 'Open'
                                END
                            WHERE tradeId = %s
                        """, (trade_ts, trade_price, trade_size, new_open_qty, new_open_qty, trade_id))

                    print(f"\nChecking if trade closed. New quantity: {new_open_qty}", flush=True)
                    if abs(new_open_qty) < 1e-9:  # effectively zero
                        print("\nTrade closed - calculating P&L", flush=True)
                        cursor.execute("""
                            SELECT entry_prices, entry_shares,
                                   exit_prices, exit_shares,
                                   tradeDirection
                            FROM trades
                            WHERE tradeId = %s
                        """, (trade_id,))
                        updated_trade = cursor.fetchone()
                        updated_pnl = calculate_pnl(
                            updated_trade[0],
                            updated_trade[1],
                            updated_trade[2],
                            updated_trade[3],
                            updated_trade[4],
                            ticker
                        )
                        print(f"\nCalculated P&L: {updated_pnl}", flush=True)
                        cursor.execute("""
                            UPDATE trades
                            SET closedPnL = %s
                            WHERE tradeId = %s
                        """, (updated_pnl, trade_id))

                    cursor.execute("""
                        UPDATE trade_executions
                        SET tradeId = %s
                        WHERE executionId = %s
                    """, (trade_id, execution_id))

            conn.db.commit()
            print("\nAll trades processed successfully", flush=True)
            return {"status": "success", "message": "Trades processed successfully"}

    except Exception as e:
        conn.db.rollback()
        error_info = traceback.format_exc()
        print(f"Error processing trades:\n{error_info}", flush=True)
        return {
            "status": "error",
            "message": f"Error: {str(e)}\nTraceback:\n{error_info}"
        }


def calculate_pnl(entry_prices, entry_shares, exit_prices, exit_shares, direction, ticker):
    """Calculate P&L for a completed trade"""
    # Convert all numbers to Decimal for consistent decimal arithmetic
    total_entry_value = Decimal('0')
    total_entry_shares = Decimal('0')
    total_exit_value = Decimal('0')
    total_exit_shares = Decimal('0')

    # Calculate totals for entries
    for price, shares in zip(entry_prices, entry_shares):
        price = Decimal(str(price))
        shares = Decimal(str(abs(shares)))  # Use absolute value for shares
        total_entry_value += price * shares
        total_entry_shares += shares

    # Calculate totals for exits
    for price, shares in zip(exit_prices, exit_shares):
        price = Decimal(str(price))
        shares = Decimal(str(abs(shares)))  # Use absolute value for shares
        total_exit_value += price * shares
        total_exit_shares += shares

    # Calculate weighted average prices
    avg_entry_price = total_entry_value / total_entry_shares if total_entry_shares > 0 else Decimal('0')
    avg_exit_price = total_exit_value / total_exit_shares if total_exit_shares > 0 else Decimal('0')

    # Calculate P&L based on direction
    if direction == "Long":
        pnl = (avg_exit_price - avg_entry_price) * total_exit_shares
    else:  # Short
        pnl = (avg_entry_price - avg_exit_price) * total_exit_shares

    # Check if it's an options trade (ticker contains 'C' or 'P' and is longer than 4 characters)
    is_option = len(ticker) > 4 and ('C' in ticker or 'P' in ticker)
    if is_option:
        pnl = pnl * Decimal('100')  # Multiply by 100 for options contracts
        total_contracts = total_entry_shares + total_exit_shares
        commission = Decimal('0.65') * total_contracts  # $0.65 per contract
        pnl = pnl - commission

    print("\ncalculated pnl: ", pnl)
    return float(round(pnl, 2))
    