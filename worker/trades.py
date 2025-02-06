import base64
import pandas as pd
import io
from screen import getCurrentSecId
import traceback
from datetime import datetime
from decimal import Decimal


def grab_user_trades(conn, user_id: int):
    """Fetch all trades for a user"""
    try:
        with conn.db.cursor() as cursor:
            cursor.execute("""
                SELECT 
                    t.*,
                    array_length(entry_times, 1) as num_entries,
                    array_length(exit_times, 1) as num_exits
                FROM trades t
                WHERE t.userId = %s
                ORDER BY t.date DESC
            """, (user_id,))
            
            trades = []
            for row in cursor.fetchall():
                trade = {
                    'ticker': row[2],
                    'securityId': row[3],
                    'direction': row[4],
                    'date': row[5].strftime('%Y-%m-%d'),
                    'status': row[6],
                    'openQuantity': row[7],
                    'closedPnL': float(row[8]) if row[8] else None,
                    'entries': [
                        {
                            'time': row[9][i].strftime('%Y-%m-%d %H:%M:%S'),
                            'price': float(row[10][i]),
                            'shares': row[11][i]
                        }
                        for i in range(len(row[9])) if row[9]
                    ],
                    'exits': [
                        {
                            'time': row[12][i].strftime('%Y-%m-%d %H:%M:%S'),
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
    """
    Process trade_executions into consolidated trades
    following the same logic as the original DataFrame approach:
    - "Short" trades use negative openQuantity
    - Adding to short => openQuantity -= size
    - Exiting short => openQuantity += size
    - Adding to long => openQuantity += size
    - Exiting long => openQuantity -= size
    """

    try:
        with conn.db.cursor() as cursor:
            # Fetch all executions that have not yet been assigned a tradeId
            cursor.execute("""
                SELECT te.* 
                FROM trade_executions te
                WHERE te.userId = %s AND te.tradeId IS NULL
                ORDER BY te.timestamp ASC
            """, (user_id,))

            executions = cursor.fetchall()

            for execution in executions:
                # execution structure:
                #   0: executionId
                #   1: userId
                #   2: securityId
                #   3: ticker
                #   4: date
                #   5: price
                #   6: size
                #   7: timestamp
                #   8: direction ("Long" or "Short")
                #   9: tradeId  (currently NULL)
                execution_id = execution[0]
                ticker       = execution[3]
                securityId   = execution[2]
                trade_date   = execution[4]
                trade_price  = float(execution[5])
                trade_size   = int(execution[6])   
                trade_ts     = execution[7]           # timestamp
                direction    = execution[8]           # "Long" or "Short"

                # 1) Check if there's an open trade for this (user, ticker)
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
                    # 2) No open trade => create one
                    #    If Short => openQuantity = -trade_size, else => openQuantity = trade_size
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
                        user_id,
                        securityId,
                        ticker,
                        direction,
                        trade_date,
                        open_quantity,
                        trade_ts,        # first entry time
                        trade_price,     # first entry price
                        trade_size       
                    ))

                    new_trade_id = cursor.fetchone()[0]

                    # Link this execution to the newly created trade
                    cursor.execute("""
                        UPDATE trade_executions
                        SET tradeId = %s
                        WHERE executionId = %s
                    """, (new_trade_id, execution_id))

                else:
                    # 3) We have an open trade => decide if it's an Entry or Exit
                    trade_id      = open_trade[0]
                    trade_dir     = open_trade[4]   # "Long" or "Short"
                    old_open_qty  = float(open_trade[7])  # could be negative for short
                    entry_times   = open_trade[8]
                    entry_prices  = open_trade[9]
                    entry_shares  = open_trade[10]
                    exit_times    = open_trade[11]
                    exit_prices   = open_trade[12]
                    exit_shares   = open_trade[13]
                    new_open_qty = old_open_qty + trade_size  
                    if trade_dir == direction:
                        # Check if trade size is in same direction as open quantity
                        is_same_direction = (old_open_qty > 0 and trade_size > 0) or (old_open_qty < 0 and trade_size < 0)
                        
                        if is_same_direction:
                            # This is an additional "Entry" since sizes are in same direction
                            cursor.execute("""
                                UPDATE trades
                                SET entry_times  = array_append(entry_times, %s),
                                    entry_prices = array_append(entry_prices, %s),
                                    entry_shares = array_append(entry_shares, %s),
                                    openQuantity = %s
                                WHERE tradeId = %s
                            """, (
                                trade_ts,
                                trade_price,
                                trade_size,
                                new_open_qty,
                                trade_id
                            ))
                        else:
                            # This is an "Exit" since sizes are in opposite directions
                            cursor.execute("""
                                UPDATE trades
                                SET exit_times  = array_append(exit_times, %s),
                                    exit_prices = array_append(exit_prices, %s),
                                    exit_shares = array_append(exit_shares, %s),
                                    openQuantity = %s,
                                    status = CASE
                                        WHEN %s = 0 THEN 'Closed'
                                        ELSE 'Open'
                                    END
                                WHERE tradeId = %s
                            """, (
                                trade_ts,
                                trade_price,
                                trade_size,
                                new_open_qty,
                                new_open_qty,   # check if it's zero
                                trade_id
                            ))

                    else:
                        # Append to the "exit_" arrays
                        # Possibly close if new_open_qty hits 0
                        cursor.execute("""
                            UPDATE trades
                            SET exit_times  = array_append(exit_times, %s),
                                exit_prices = array_append(exit_prices, %s),
                                exit_shares = array_append(exit_shares, %s),
                                openQuantity = %s,
                                status = CASE
                                    WHEN %s = 0 THEN 'Closed'
                                    ELSE 'Open'
                                END
                            WHERE tradeId = %s
                        """, (
                            trade_ts, 
                            trade_price,
                            trade_size,
                            new_open_qty,
                            new_open_qty,   # check if it's zero
                            trade_id
                        ))
                        print("\n HIT HERE ")
                        # If we just closed the trade, compute P/L
                        if new_open_qty == 0:  # effectively zero
                            # We need fresh arrays to compute P/L
                            print("\nhittttttttt")
                            # Easiest might be to SELECT them again:
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
                                updated_trade[4]
                            )
                            # Update the closedPnL field
                            cursor.execute("""
                                UPDATE trades
                                SET closedPnL = %s
                                WHERE tradeId = %s
                            """, (updated_pnl, trade_id))

                    # 4) Link this execution to the trade
                    cursor.execute("""
                        UPDATE trade_executions
                        SET tradeId = %s
                        WHERE executionId = %s
                    """, (trade_id, execution_id))

            # Finally commit once all executions have been processed
            conn.db.commit()

            return {"status": "success", "message": "Trades processed successfully"}

    except Exception as e:
        conn.db.rollback()
        error_info = traceback.format_exc()
        print(f"Error processing trades:\n{error_info}")
        return {
            "status": "error",
            "message": f"Error: {str(e)}\nTraceback:\n{error_info}"
        }


def calculate_pnl(entry_prices, entry_shares, exit_prices, exit_shares, direction):
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
    print("\ncalculated pnl: ", pnl)
    return float(round(pnl, 2))
    