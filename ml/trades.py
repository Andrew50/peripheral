import base64
import pandas as pd
import io
from screen import getCurrentSecId
import traceback
from datetime import datetime


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
                    'tradeId': row['tradeId'],
                    'ticker': row['ticker'],
                    'direction': row['tradeDirection'],
                    'date': row['date'].strftime('%Y-%m-%d'),
                    'status': row['status'],
                    'openQuantity': row['openQuantity'],
                    'closedPnL': float(row['closedPnL']) if row['closedPnL'] else None,
                    'entries': [
                        {
                            'time': row['entry_times'][i].strftime('%Y-%m-%d %H:%M:%S'),
                            'price': float(row['entry_prices'][i]),
                            'shares': row['entry_shares'][i]
                        }
                        for i in range(row['num_entries']) if row['num_entries']
                    ],
                    'exits': [
                        {
                            'time': row['exit_times'][i].strftime('%Y-%m-%d %H:%M:%S'),
                            'price': float(row['exit_prices'][i]),
                            'shares': row['exit_shares'][i]
                        }
                        for i in range(row['num_exits']) if row['num_exits']
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
                print(f"trade_description: {trade_description}")
                fidelity_trade_status = trade['Status']
                
                if 'Short' in trade_description or 'Sell to Open' in trade_description:
                    trade_direction = "Short" 
                else: 
                    trade_direction = "Long"
                    
                trade_dt, trade_date = parse_datetime(trade['Order Time'])
                ticker = trade['Symbol']
                securityId = getCurrentSecId(conn, ticker)
                
                if 'Limit' in trade_description:
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
                
                if trade_direction == "Short": 
                    trade_shares = -trade_shares

                cursor.execute("""
                    INSERT INTO trade_executions 
                    (userId, securityId, date, price, size, timestamp, direction)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (userId, securityId, timestamp) DO NOTHING
                """, (user_id, securityId, trade_date, trade_price, abs(trade_shares), trade_dt, trade_direction))

            # Commit the transaction after all trades are processed
            conn.db.commit()

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
    """
    try:
        with conn.db.cursor() as cursor:
            # Get all unprocessed executions ordered by timestamp
            cursor.execute("""
                SELECT te.*, s.ticker 
                FROM trade_executions te
                JOIN securities s ON te.securityId = s.securityId
                WHERE te.userId = %s AND te.tradeId IS NULL
                ORDER BY te.timestamp
            """, (user_id,))
            
            executions = cursor.fetchall()
            
            # Process each execution
            for execution in executions:
                ticker = execution['ticker']
                direction = execution['direction']
                date = execution['date']
                price = execution['price']
                size = execution['size']
                timestamp = execution['timestamp']
                execution_id = execution['executionId']
                
                # Check if there's an open trade for this ticker
                cursor.execute("""
                    SELECT * FROM trades 
                    WHERE userId = %s AND ticker = %s AND status = 'Open'
                    ORDER BY date DESC LIMIT 1
                """, (user_id, ticker))
                
                open_trade = cursor.fetchone()
                
                if open_trade:
                    trade_id = open_trade['tradeId']
                    # Update existing trade
                    if (direction == 'Long' and open_trade['tradeDirection'] == 'Long') or \
                       (direction == 'Short' and open_trade['tradeDirection'] == 'Short'):
                        # Adding to position
                        cursor.execute("""
                            UPDATE trades 
                            SET entry_times = array_append(entry_times, %s),
                                entry_prices = array_append(entry_prices, %s),
                                entry_shares = array_append(entry_shares, %s),
                                openQuantity = openQuantity + %s
                            WHERE tradeId = %s
                        """, (timestamp, price, size, size, trade_id))
                    else:
                        # Closing position
                        cursor.execute("""
                            UPDATE trades 
                            SET exit_times = array_append(exit_times, %s),
                                exit_prices = array_append(exit_prices, %s),
                                exit_shares = array_append(exit_shares, %s),
                                openQuantity = openQuantity - %s,
                                status = CASE 
                                    WHEN openQuantity - %s = 0 THEN 'Closed'
                                    ELSE status
                                END
                            WHERE tradeId = %s
                        """, (timestamp, price, size, size, size, trade_id))
                        
                        # Calculate P&L if trade is closed
                        if open_trade['openQuantity'] - size == 0:
                            pnl = calculate_pnl(
                                open_trade['entry_prices'], 
                                open_trade['entry_shares'],
                                open_trade['exit_prices'] + [price],
                                open_trade['exit_shares'] + [size],
                                open_trade['tradeDirection']
                            )
                            cursor.execute("""
                                UPDATE trades 
                                SET closedPnL = %s
                                WHERE tradeId = %s
                            """, (pnl, trade_id))
                else:
                    # Create new trade
                    cursor.execute("""
                        INSERT INTO trades (
                            userId, ticker, tradeDirection, date, status, 
                            openQuantity, entry_times, entry_prices, entry_shares
                        ) VALUES (
                            %s, %s, %s, %s, 'Open', %s, 
                            ARRAY[%s], ARRAY[%s], ARRAY[%s]
                        ) RETURNING tradeId
                    """, (user_id, ticker, direction, date, size, 
                          timestamp, price, size))
                    trade_id = cursor.fetchone()[0]

                # Link execution to trade
                cursor.execute("""
                    UPDATE trade_executions 
                    SET tradeId = %s 
                    WHERE executionId = %s
                """, (trade_id, execution_id))
            
            conn.db.commit()
            return {"status": "success", "message": "Trades processed successfully"}

    except Exception as e:
        conn.db.rollback()
        error_info = traceback.format_ex()
        print(f"Error processing trades:\n{error_info}")
        return {
            "status": "error",
            "message": f"Error: {str(e)}\nTraceback:\n{error_info}"
        }   

def calculate_pnl(entry_prices, entry_shares, exit_prices, exit_shares, direction):
    """Calculate P&L for a completed trade"""
    total_entry_value = sum(p * s for p, s in zip(entry_prices, entry_shares))
    total_exit_value = sum(p * s for p, s in zip(exit_prices, exit_shares))
    total_entry_shares = sum(entry_shares)
    total_exit_shares = sum(exit_shares)
    
    if direction == "Long":
        pnl = total_exit_value - (total_entry_value * (total_exit_shares / total_entry_shares))
    else:  # Short
        pnl = (total_entry_value * (total_exit_shares / total_entry_shares)) - total_exit_value
    
    return round(pnl, 2)
    