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
                
                # Create combined trades array sorted by timestamp
                combined_trades = []
                
                # Add entries
                for i in range(len(row[9])) if row[9] else []:
                    combined_trades.append({
                        'time': eastern.localize(row[9][i]).astimezone(utc).timestamp() * 1000,
                        'price': float(row[10][i]),
                        'shares': row[11][i],
                        'type': 'Short' if row[4] == 'Short' else 'Buy'
                    })
                
                # Add exits
                for i in range(len(row[12])) if row[12] else []:
                    combined_trades.append({
                        'time': eastern.localize(row[12][i]).astimezone(utc).timestamp() * 1000,
                        'price': float(row[13][i]),
                        'shares': row[14][i],
                        'type': 'Buy to Cover' if row[4] == 'Short' and row[7] <= 0 else 'Sell'
                    })
                
                # Sort combined trades by timestamp
                combined_trades.sort(key=lambda x: x['time'])
                
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
                    'trades': combined_trades
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
                        trade_price = float(fidelity_trade_status.split('$')[1].replace(',', ''))
                        trade_shares = float(trade['Quantity'].replace(',', ''))
                    elif 'PARTIAL' in fidelity_trade_status:
                        trade_shares = float(fidelity_trade_status.split('\n')[3].split(' ')[0].replace(',', ''))
                        description_split = trade_description.split('$')
                        if(len(description_split) == 2):
                            trade_price = float(description_split[1].replace(',', ''))
                        elif len(description_split) == 3: 
                            trade_price = float(description_split[2].replace(',', ''))
                elif 'Market' in trade_description: 
                    trade_price = float(fidelity_trade_status.split('$')[1].replace(',', ''))
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
                            userId, securityId, ticker, tradedirection, date, status, openQuantity,
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

                            # Calculate and update P/L for partial exits
                            cursor.execute("""
                                SELECT entry_prices, entry_shares,
                                       exit_prices, exit_shares,
                                       tradedirection
                                FROM trades
                                WHERE tradeId = %s
                            """, (trade_id,))
                            trade_data = cursor.fetchone()
                            updated_pnl = calculate_pnl(
                                trade_data[0],
                                trade_data[1],
                                trade_data[2],
                                trade_data[3],
                                trade_data[4],
                                ticker
                            )
                            cursor.execute("""
                                UPDATE trades
                                SET closedPnL = %s
                                WHERE tradeId = %s
                            """, (updated_pnl, trade_id))

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

                        # Calculate and update P/L for partial exits
                        cursor.execute("""
                            SELECT entry_prices, entry_shares,
                                   exit_prices, exit_shares,
                                   tradedirection
                            FROM trades
                            WHERE tradeId = %s
                        """, (trade_id,))
                        trade_data = cursor.fetchone()
                        updated_pnl = calculate_pnl(
                            trade_data[0],
                            trade_data[1],
                            trade_data[2],
                            trade_data[3],
                            trade_data[4],
                            ticker
                        )
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
        
        # Only apply commission if it's not a buy to close under $0.65
        should_apply_commission = True
        if direction == "Short" and avg_exit_price < Decimal('0.65'):
            should_apply_commission = False
            
        if should_apply_commission:
            commission = Decimal('0.65') * total_contracts  # $0.65 per contract
            pnl = pnl - commission

    print("\ncalculated pnl: ", pnl)
    return float(round(pnl, 2))

def get_trade_statistics(conn, user_id: int, start_date: str = None, end_date: str = None, ticker: str = None) -> dict:
    """
    Calculate trading statistics for a user within a date range
    
    Args:
        conn: Database connection
        user_id (int): User ID
        start_date (str): Optional start date filter in format 'YYYY-MM-DD'
        end_date (str): Optional end date filter in format 'YYYY-MM-DD'
        ticker (str): Optional ticker filter
    """
    try:
        with conn.db.cursor() as cursor:
            query = """
                SELECT 
                    COUNT(*) as total_trades,
                    COUNT(CASE WHEN closedPnL > 0 THEN 1 END) as winning_trades,
                    COUNT(CASE WHEN closedPnL <= 0 THEN 1 END) as losing_trades,
                    AVG(CASE WHEN closedPnL > 0 THEN closedPnL END) as avg_win,
                    AVG(CASE WHEN closedPnL <= 0 THEN closedPnL END) as avg_loss,
                    COALESCE(SUM(closedPnL), 0) as total_pnl
                FROM trades 
                WHERE userId = %s 
                AND status = 'Closed'
                AND closedPnL IS NOT NULL
            """
            params = [user_id]

            if start_date:
                query += " AND DATE(entry_times[1]) >= %s"
                params.append(start_date)
            
            if end_date:
                query += " AND DATE(entry_times[1]) <= %s"
                params.append(end_date)
            
            if ticker:
                query += " AND ticker = %s"
                params.append(ticker)

            cursor.execute(query, tuple(params))
            row = cursor.fetchone()
            
            total_trades = row[0]
            winning_trades = row[1]
            losing_trades = row[2]
            avg_win = float(row[3]) if row[3] else 0
            avg_loss = float(row[4]) if row[4] else 0
            total_pnl = float(row[5]) if row[5] else 0
            
            win_rate = (winning_trades / total_trades * 100) if total_trades > 0 else 0
            
            # Update P/L curve query with date range
            pnl_query = """
                SELECT 
                    entry_times[1] as trade_time,
                    closedPnL
                FROM trades 
                WHERE userId = %s 
                AND status = 'Closed'
                AND closedPnL IS NOT NULL
            """
            params = [user_id]

            if start_date:
                pnl_query += " AND DATE(entry_times[1]) >= %s"
                params.append(start_date)
            
            if end_date:
                pnl_query += " AND DATE(entry_times[1]) <= %s"
                params.append(end_date)
            
            if ticker:
                pnl_query += " AND ticker = %s"
                params.append(ticker)
                
            pnl_query += " ORDER BY entry_times[1] ASC"

            eastern = pytz.timezone('America/New_York')
            utc = pytz.UTC

            # Add queries for top and bottom trades
            trades_query = """
                SELECT 
                    ticker,
                    entry_times[1] as trade_time,
                    tradedirection,
                    closedPnL
                FROM trades 
                WHERE userId = %s 
                AND status = 'Closed'
                AND closedPnL IS NOT NULL
            """
            params = [user_id]

            if start_date:
                trades_query += " AND DATE(entry_times[1]) >= %s"
                params.append(start_date)
            
            if end_date:
                trades_query += " AND DATE(entry_times[1]) <= %s"
                params.append(end_date)
            
            if ticker:
                trades_query += " AND ticker = %s"
                params.append(ticker)

            # Get top 5 trades
            top_query = trades_query + " ORDER BY closedPnL DESC LIMIT 5"
            cursor.execute(top_query, tuple(params))
            top_trades = [{
                'ticker': row[0],
                'timestamp': int(eastern.localize(row[1]).astimezone(utc).timestamp() * 1000),
                'direction': row[2],
                'pnl': float(row[3])
            } for row in cursor.fetchall()]

            # Get bottom 5 trades
            bottom_query = trades_query + " ORDER BY closedPnL ASC LIMIT 5"
            cursor.execute(bottom_query, tuple(params))
            bottom_trades = [{
                'ticker': row[0],
                'timestamp': int(eastern.localize(row[1]).astimezone(utc).timestamp() * 1000),
                'direction': row[2],
                'pnl': float(row[3])
            } for row in cursor.fetchall()]

            # Get P/L curve data
            cursor.execute(pnl_query, tuple(params))
            pnl_data = cursor.fetchall()
            
            cumulative_pnl = []
            running_total = 0
            
            for row in pnl_data:
                trade_time = int(eastern.localize(row[0]).astimezone(utc).timestamp() * 1000)
                pnl = float(row[1])
                running_total += pnl
                cumulative_pnl.append({
                    'timestamp': trade_time,
                    'value': running_total
                })

            # Add query for hourly statistics
            hourly_query = """
                SELECT 
                    EXTRACT(HOUR FROM entry_times[1]) as hour,
                    COUNT(*) as total_trades,
                    COUNT(CASE WHEN closedPnL > 0 THEN 1 END) as winning_trades,
                    COUNT(CASE WHEN closedPnL <= 0 THEN 1 END) as losing_trades,
                    AVG(closedPnL) as avg_pnl,
                    SUM(closedPnL) as total_pnl
                FROM trades 
                WHERE userId = %s 
                AND status = 'Closed'
                AND closedPnL IS NOT NULL
            """
            params = [user_id]

            if start_date:
                hourly_query += " AND DATE(entry_times[1]) >= %s"
                params.append(start_date)
            
            if end_date:
                hourly_query += " AND DATE(entry_times[1]) <= %s"
                params.append(end_date)
            
            if ticker:
                hourly_query += " AND ticker = %s"
                params.append(ticker)
            
            hourly_query += " GROUP BY EXTRACT(HOUR FROM entry_times[1]) ORDER BY hour"

            cursor.execute(hourly_query, tuple(params))
            hourly_stats = []
            
            for row in cursor.fetchall():
                hour = int(row[0])
                total = int(row[1])
                wins = int(row[2])
                losses = int(row[3])
                avg_pnl = float(row[4]) if row[4] else 0
                total_pnl = float(row[5]) if row[5] else 0
                
                hourly_stats.append({
                    'hour': hour,
                    'hour_display': f"{hour:02d}:00",
                    'total_trades': total,
                    'winning_trades': wins,
                    'losing_trades': losses,
                    'win_rate': round((wins / total * 100), 2) if total > 0 else 0,
                    'avg_pnl': round(avg_pnl, 2),
                    'total_pnl': round(total_pnl, 2)
                })

            # Add query for ticker statistics
            ticker_query = """
                SELECT 
                    ticker,
                    COUNT(*) as total_trades,
                    COUNT(CASE WHEN closedPnL > 0 THEN 1 END) as winning_trades,
                    COUNT(CASE WHEN closedPnL <= 0 THEN 1 END) as losing_trades,
                    AVG(closedPnL) as avg_pnl,
                    SUM(closedPnL) as total_pnl
                FROM trades 
                WHERE userId = %s 
                AND status = 'Closed'
                AND closedPnL IS NOT NULL
            """
            params = [user_id]

            if start_date:
                ticker_query += " AND DATE(entry_times[1]) >= %s"
                params.append(start_date)
            
            if end_date:
                ticker_query += " AND DATE(entry_times[1]) <= %s"
                params.append(end_date)
            
            if ticker:
                ticker_query += " AND ticker = %s"
                params.append(ticker)
            
            ticker_query += """ 
                GROUP BY ticker 
                ORDER BY SUM(closedPnL) DESC
            """

            cursor.execute(ticker_query, tuple(params))
            ticker_stats = []
            
            for row in cursor.fetchall():
                ticker_name = row[0]
                total = int(row[1])
                wins = int(row[2])
                losses = int(row[3])
                avg_pnl = float(row[4]) if row[4] else 0
                total_pnl = float(row[5]) if row[5] else 0
                
                ticker_stats.append({
                    'ticker': ticker_name,
                    'total_trades': total,
                    'winning_trades': wins,
                    'losing_trades': losses,
                    'win_rate': round((wins / total * 100), 2) if total > 0 else 0,
                    'avg_pnl': round(avg_pnl, 2),
                    'total_pnl': round(total_pnl, 2)
                })

            # Verify total P&L matches sum of ticker P&Ls
            cursor.execute("""
                SELECT COALESCE(SUM(closedPnL), 0) as verification_total
                FROM trades 
                WHERE userId = %s 
                AND status = 'Closed'
                AND closedPnL IS NOT NULL
                """ + 
                (" AND DATE(entry_times[1]) >= %s" if start_date else "") +
                (" AND DATE(entry_times[1]) <= %s" if end_date else "") +
                (" AND ticker = %s" if ticker else ""),
                tuple(param for param in params if param is not None)
            )
            verification_total = float(cursor.fetchone()[0])

            # Add verification to returned data
            return {
                "total_trades": total_trades,
                "winning_trades": winning_trades,
                "losing_trades": losing_trades,
                "win_rate": round((winning_trades / total_trades * 100), 2) if total_trades > 0 else 0,
                "avg_win": round(avg_win, 2),
                "avg_loss": round(avg_loss, 2),
                "total_pnl": round(verification_total, 2),  # Use verified total
                "pnl_curve": cumulative_pnl,
                "top_trades": top_trades,
                "bottom_trades": bottom_trades,
                "hourly_stats": hourly_stats,
                "ticker_stats": ticker_stats
            }
            
    except Exception as e:
        error_info = traceback.format_exc()
        print(f"Error calculating statistics:\n{error_info}")
        return {
            "error": str(e),
            "traceback": error_info
        }

def get_ticker_trades(conn, user_id: int, ticker: str, start_date: str = None, end_date: str = None):
    """Get all trades for a specific ticker within a date range"""
    try:
        with conn.db.cursor() as cursor:
            query = """
                SELECT 
                    securityId,
                    entry_times,
                    entry_prices,
                    exit_times,
                    exit_prices,
                    tradedirection
                FROM trades 
                WHERE userId = %s 
                AND ticker = %s
                AND status = 'Closed'
            """
            params = [user_id, ticker]

            if start_date:
                query += " AND DATE(entry_times[1]) >= %s"
                params.append(start_date)
            
            if end_date:
                query += " AND DATE(entry_times[1]) <= %s"
                params.append(end_date)

            cursor.execute(query, tuple(params))
            trades = cursor.fetchall()

            entries = []
            exits = []

            for trade in trades:
                security_id = trade[0]
                entry_times = trade[1]
                entry_prices = trade[2]
                exit_times = trade[3]
                exit_prices = trade[4]
                direction = trade[5]

                # Process entries
                for time, price in zip(entry_times, entry_prices):
                    entries.append({
                        "time": int(time.timestamp() * 1000),
                        "price": float(price),
                        "isLong": direction == "Long"
                    })

                # Process exits
                for time, price in zip(exit_times, exit_prices):
                    exits.append({
                        "time": int(time.timestamp() * 1000),
                        "price": float(price),
                        "isLong": direction == "Long"
                    })

            return {
                "securityId": security_id,
                "entries": entries,
                "exits": exits
            }

    except Exception as e:
        error_info = traceback.format_exc()
        print(f"Error getting ticker trades:\n{error_info}")
        return {
            "error": str(e),
            "traceback": error_info
        }

def get_ticker_performance(conn, user_id: int, sort: str = "desc", date: str = None, hour: int = None, ticker: str = None):
    """Get performance statistics by ticker with filters"""
    try:
        with conn.db.cursor() as cursor:
            query = """
                WITH ticker_stats AS (
                    SELECT 
                        ticker,
                        MAX(securityId) as securityId,
                        COUNT(*) as total_trades,
                        COUNT(CASE WHEN closedPnL > 0 THEN 1 END) as winning_trades,
                        COUNT(CASE WHEN closedPnL <= 0 THEN 1 END) as losing_trades,
                        AVG(closedPnL) as avg_pnl,
                        SUM(closedPnL) as total_pnl,
                        MAX(entry_times[1]) as latest_entry,
                        MAX(exit_times[array_length(exit_times, 1)]) as last_exit
                    FROM trades 
                    WHERE userId = %s 
                    AND status = 'Closed'
                    AND closedPnL IS NOT NULL
                    {date_filter}
                    {hour_filter}
                    {ticker_filter}
                    GROUP BY ticker
                )
                SELECT 
                    ts.*,
                    t.tradeId,
                    t.entry_times,
                    t.entry_prices,
                    t.entry_shares,
                    t.exit_times,
                    t.exit_prices,
                    t.exit_shares,
                    t.tradedirection,
                    t.closedPnL
                FROM ticker_stats ts
                LEFT JOIN trades t ON ts.ticker = t.ticker
                WHERE t.userId = %s 
                AND t.status = 'Closed'
                AND t.closedPnL IS NOT NULL
                {date_filter}
                {hour_filter}
                {ticker_filter}
                ORDER BY ts.last_exit {sort_direction}
            """

            # Build the query with filters
            date_filter = " AND DATE(entry_times[1]) = %s" if date else ""
            hour_filter = " AND EXTRACT(HOUR FROM entry_times[1]) = %s" if hour is not None else ""
            ticker_filter = " AND ticker = %s" if ticker else ""
            
            # Replace placeholders in the query
            query = query.format(
                date_filter=date_filter,
                hour_filter=hour_filter,
                ticker_filter=ticker_filter,
                sort_direction="DESC" if sort.lower() == "desc" else "ASC"
            )

            # Build params list
            params = [user_id]
            if date:
                params.append(date)
            if hour is not None:
                params.append(hour)
            if ticker:
                params.append(ticker)
            # Add params for second part of query
            params.extend([p for p in [user_id] + params[1:]])

            eastern = pytz.timezone('America/New_York')
            utc = pytz.UTC

            cursor.execute(query, tuple(params))
            rows = cursor.fetchall()
            
            ticker_stats = []
            current_ticker = None
            current_stats = None
            
            for row in rows:
                ticker_name = row[0]
                
                if ticker_name != current_ticker:
                    if current_stats is not None:
                        ticker_stats.append(current_stats)
                    
                    current_ticker = ticker_name
                    current_stats = {
                        'ticker': ticker_name,
                        'securityId': row[1],
                        'total_trades': int(row[2]),
                        'winning_trades': int(row[3]),
                        'losing_trades': int(row[4]),
                        'avg_pnl': round(float(row[5]) if row[5] else 0, 2),
                        'total_pnl': round(float(row[6]) if row[6] else 0, 2),
                        'timestamp': int(eastern.localize(row[8]).astimezone(utc).timestamp() * 1000) if row[8] else None,
                        'trades': []  # Initialize empty trades array
                    }
                
                # Add trade details if they exist
                if row[9] is not None:  # tradeId exists
                    # Create combined trades array
                    combined_trades = []
                    
                    # Add entries
                    for i in range(len(row[10])):
                        combined_trades.append({
                            'time': int(eastern.localize(row[10][i]).astimezone(utc).timestamp() * 1000),
                            'price': float(row[11][i]),
                            'shares': float(row[12][i]),
                            'type': 'Short' if row[16] == 'Short' else 'Buy'
                        })
                    
                    # Add exits
                    for i in range(len(row[13])):
                        combined_trades.append({
                            'time': int(eastern.localize(row[13][i]).astimezone(utc).timestamp() * 1000),
                            'price': float(row[14][i]),
                            'shares': float(row[15][i]),
                            'type': 'Buy to Cover' if row[16] == 'Short' else 'Sell'
                        })
                    
                    # Sort combined trades by timestamp
                    combined_trades.sort(key=lambda x: x['time'])
                    
                    # Extend the trades array instead of overwriting it
                    current_stats['trades'].extend(combined_trades)
                    
                    # Sort all trades by timestamp after adding new ones
                    current_stats['trades'].sort(key=lambda x: x['time'])
            
            # Add the last ticker stats
            if current_stats is not None:
                ticker_stats.append(current_stats)

            return ticker_stats

    except Exception as e:
        error_info = traceback.format_exc()
        print(f"Error getting ticker performance:\n{error_info}")
        return []
    