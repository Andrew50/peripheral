import base64
import pandas as pd
import io
from screen import getCurrentSecId
import traceback
import pytz
from datetime import datetime
from decimal import Decimal


def parse_datetime(datetime_str):
    datetime_str = " ".join(datetime_str.split())
    try:
        dt = datetime.strptime(datetime_str, "%I:%M:%S %p %m/%d/%Y")
        date_only_str = dt.date().strftime("%m/%d/%Y")

        return dt, date_only_str
    except ValueError as e:
        print(f"Error parsing datetime: {e}")
        return None, None


def calculate_pnl(
    entry_prices, entry_shares, exit_prices, exit_shares, direction, ticker
):
    """Calculate P&L for a trade"""
    try:
        # Ensure we have data to work with
        if not entry_prices or not exit_prices:
            return Decimal('0.00')
            
        total_pnl = Decimal('0.00')
        
        # For long trades
        if direction.lower() == 'long':
            for i in range(min(len(entry_prices), len(exit_prices))):
                # Calculate P&L for each entry/exit pair
                entry_cost = Decimal(str(entry_prices[i])) * Decimal(str(entry_shares[i]))
                exit_value = Decimal(str(exit_prices[i])) * Decimal(str(exit_shares[i]))
                pair_pnl = exit_value - entry_cost
                total_pnl += pair_pnl
        
        # For short trades
        elif direction.lower() == 'short':
            for i in range(min(len(entry_prices), len(exit_prices))):
                # Calculate P&L for each entry/exit pair (reversed for short)
                entry_value = Decimal(str(entry_prices[i])) * Decimal(str(entry_shares[i]))
                exit_cost = Decimal(str(exit_prices[i])) * Decimal(str(exit_shares[i]))
                pair_pnl = entry_value - exit_cost
                total_pnl += pair_pnl
                
        return total_pnl.quantize(Decimal('0.01'))
        
    except Exception as e:
        print(f"Error calculating P&L for {ticker}: {str(e)}")
        return Decimal('0.00')


def process_trades(conn, user_id: int):
    """Process user trades after upload or modification"""
    try:
        # Update trade statistics
        with conn.db.cursor() as cursor:
            cursor.execute(
                """
                UPDATE users 
                SET trade_stats = (
                    SELECT json_build_object(
                        'total_trades', COUNT(*),
                        'winning_trades', SUM(CASE WHEN closedPnL > 0 THEN 1 ELSE 0 END),
                        'losing_trades', SUM(CASE WHEN closedPnL < 0 THEN 1 ELSE 0 END),
                        'total_pnl', SUM(closedPnL)
                    )
                    FROM trades
                    WHERE userId = %s AND status = 'Closed'
                )
                WHERE id = %s
                """,
                [user_id, user_id],
            )
            
        conn.db.commit()
        return True
        
    except Exception as e:
        print(f"Error processing trades: {str(e)}")
        conn.db.rollback()
        return False


def grab_user_trades(
    conn,
    user_id: int,
    sort: str = "desc",
    date: str = None,
    hour: int = None,
    ticker: str = None,
):
    """
    Fetch all trades for a user with optional sorting and filtering
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

            if ticker:
                base_query += " AND (t.ticker = %s OR t.ticker LIKE %s)"
                params.extend([ticker, f"{ticker}%"])

            if date:
                base_query += " AND DATE(t.entry_times[1]) = %s"
                params.append(date)

            if hour is not None:
                base_query += " AND EXTRACT(HOUR FROM t.entry_times[1]) = %s"
                params.append(hour)

            # Add sorting
            if sort.lower() == "asc":
                base_query += " ORDER BY entry_times[1] ASC"
            else:
                base_query += " ORDER BY entry_times[1] DESC"

            cursor.execute(base_query, params)
            trades = cursor.fetchall()

            # Get column names
            columns = [desc[0] for desc in cursor.description]
            
            result = []
            for trade in trades:
                trade_dict = {columns[i]: trade[i] for i in range(len(columns))}
                
                # Convert any datetime objects to ISO format for JSON serialization
                for key, value in trade_dict.items():
                    if isinstance(value, datetime):
                        trade_dict[key] = value.isoformat()
                    elif isinstance(value, Decimal):
                        trade_dict[key] = float(value)
                        
                result.append(trade_dict)
                
            return result

    except Exception as e:
        print(f"Error grabbing user trades: {str(e)}")
        return []


def handle_trade_upload(
    conn, file_content: str, user_id: int, additional_args: dict = None
) -> dict:
    """
    Process a CSV trade upload
    """
    try:
        # Decode base64 content if needed
        if file_content.startswith('data:'):
            # Extract the base64 part
            base64_content = file_content.split(',')[1]
            decoded_content = base64.b64decode(base64_content).decode('utf-8')
        else:
            decoded_content = file_content
            
        # Load the CSV data
        df = pd.read_csv(io.StringIO(decoded_content))
        
        # Basic validation
        required_columns = ['Ticker', 'Direction', 'Entry Time', 'Entry Price', 'Entry Shares']
        for col in required_columns:
            if col not in df.columns:
                return {"success": False, "message": f"Missing required column: {col}"}
                
        # Process each trade
        trades_added = 0
        with conn.db.cursor() as cursor:
            for _, row in df.iterrows():
                ticker = row['Ticker'].strip().upper()
                direction = row['Direction'].strip().capitalize()
                
                # Parse dates
                entry_dt, _ = parse_datetime(row['Entry Time'])
                if not entry_dt:
                    continue
                    
                # Get sector ID
                sector_id = getCurrentSecId(conn, ticker)
                
                # Insert the trade
                cursor.execute(
                    """
                    INSERT INTO trades (
                        userId, ticker, tradeDirection, entry_times, entry_prices, 
                        entry_shares, sectorId, status
                    ) VALUES (
                        %s, %s, %s, ARRAY[%s], ARRAY[%s], ARRAY[%s], %s, 'Open'
                    )
                    RETURNING tradeId
                    """,
                    [
                        user_id, ticker, direction, entry_dt, 
                        float(row['Entry Price']), int(row['Entry Shares']),
                        sector_id
                    ]
                )
                trades_added += 1
                
            conn.db.commit()
            
        # Update user's trade statistics
        process_trades(conn, user_id)
        
        return {
            "success": True,
            "message": f"Successfully added {trades_added} trades",
            "trades_added": trades_added
        }
        
    except Exception as e:
        conn.db.rollback()
        print(f"Error handling trade upload: {str(e)}")
        traceback.print_exc()
        return {"success": False, "message": f"Error processing trades: {str(e)}"}


def get_trade_statistics(
    conn, user_id: int, start_date: str = None, end_date: str = None, ticker: str = None
) -> dict:
    """
    Get aggregated trade statistics for a user
    """
    try:
        with conn.db.cursor() as cursor:
            # Base query
            query = """
                SELECT 
                    COUNT(*) as total_trades,
                    SUM(CASE WHEN closedPnL > 0 THEN 1 ELSE 0 END) as winning_trades,
                    SUM(CASE WHEN closedPnL < 0 THEN 1 ELSE 0 END) as losing_trades,
                    SUM(closedPnL) as total_pnl,
                    AVG(CASE WHEN closedPnL > 0 THEN closedPnL ELSE NULL END) as avg_win,
                    AVG(CASE WHEN closedPnL < 0 THEN ABS(closedPnL) ELSE NULL END) as avg_loss
                FROM trades
                WHERE userId = %s AND status = 'Closed'
            """
            params = [user_id]
            
            # Add filters
            if start_date:
                query += " AND entry_times[1] >= %s"
                params.append(f"{start_date} 00:00:00")
                
            if end_date:
                query += " AND entry_times[1] <= %s"
                params.append(f"{end_date} 23:59:59")
                
            if ticker:
                query += " AND ticker = %s"
                params.append(ticker)
                
            cursor.execute(query, params)
            stats = cursor.fetchone()
            
            if not stats:
                return {
                    "total_trades": 0,
                    "winning_trades": 0,
                    "losing_trades": 0,
                    "win_rate": 0,
                    "total_pnl": 0,
                    "avg_win": 0,
                    "avg_loss": 0,
                    "profit_factor": 0
                }
                
            total_trades = stats[0] or 0
            winning_trades = stats[1] or 0
            losing_trades = stats[2] or 0
            total_pnl = float(stats[3] or 0)
            avg_win = float(stats[4] or 0)
            avg_loss = float(stats[5] or 0)
            
            # Calculate derived metrics
            win_rate = (winning_trades / total_trades * 100) if total_trades > 0 else 0
            profit_factor = (avg_win * winning_trades) / (avg_loss * losing_trades) if (avg_loss * losing_trades) > 0 else 0
            
            return {
                "total_trades": total_trades,
                "winning_trades": winning_trades,
                "losing_trades": losing_trades,
                "win_rate": round(win_rate, 2),
                "total_pnl": round(total_pnl, 2),
                "avg_win": round(avg_win, 2),
                "avg_loss": round(avg_loss, 2),
                "profit_factor": round(profit_factor, 2)
            }
            
    except Exception as e:
        print(f"Error getting trade statistics: {str(e)}")
        return {
            "total_trades": 0,
            "winning_trades": 0,
            "losing_trades": 0, 
            "win_rate": 0,
            "total_pnl": 0,
            "avg_win": 0,
            "avg_loss": 0,
            "profit_factor": 0
        }


def get_ticker_trades(
    conn, user_id: int, ticker: str, start_date: str = None, end_date: str = None
):
    """
    Get trades for a specific ticker
    """
    try:
        # Call the more general function with ticker filter
        return grab_user_trades(
            conn,
            user_id=user_id,
            ticker=ticker,
            date=start_date  # For simplicity, we're just using the start_date
        )
            
    except Exception as e:
        print(f"Error getting ticker trades: {str(e)}")
        return []


def get_ticker_performance(
    conn,
    user_id: int,
    sort: str = "desc",
    date: str = None,
    hour: int = None,
    ticker: str = None,
):
    """
    Get performance statistics by ticker
    """
    try:
        with conn.db.cursor() as cursor:
            # Base query to get performance by ticker
            query = """
                SELECT 
                    ticker,
                    COUNT(*) as total_trades,
                    SUM(CASE WHEN closedPnL > 0 THEN 1 ELSE 0 END) as winning_trades,
                    SUM(CASE WHEN closedPnL < 0 THEN 1 ELSE 0 END) as losing_trades,
                    SUM(closedPnL) as total_pnl
                FROM trades
                WHERE userId = %s AND status = 'Closed'
            """
            params = [user_id]
            
            # Add filters
            if date:
                query += " AND DATE(entry_times[1]) = %s"
                params.append(date)
                
            if hour is not None:
                query += " AND EXTRACT(HOUR FROM entry_times[1]) = %s"
                params.append(hour)
                
            if ticker:
                query += " AND ticker = %s"
                params.append(ticker)
                
            # Group by ticker
            query += " GROUP BY ticker"
            
            # Add sorting
            if sort.lower() == "asc":
                query += " ORDER BY total_pnl ASC"
            else:
                query += " ORDER BY total_pnl DESC"
                
            cursor.execute(query, params)
            results = cursor.fetchall()
            
            # Format the results
            performance = []
            for row in results:
                ticker = row[0]
                total_trades = row[1]
                winning_trades = row[2] or 0
                losing_trades = row[3] or 0
                total_pnl = float(row[4] or 0)
                
                # Calculate win rate
                win_rate = (winning_trades / total_trades * 100) if total_trades > 0 else 0
                
                performance.append({
                    "ticker": ticker,
                    "total_trades": total_trades,
                    "winning_trades": winning_trades,
                    "losing_trades": losing_trades,
                    "win_rate": round(win_rate, 2),
                    "total_pnl": round(total_pnl, 2)
                })
                
            return performance
            
    except Exception as e:
        print(f"Error getting ticker performance: {str(e)}")
        return []


def delete_all_user_trades(conn, user_id: int) -> dict:
    """
    Delete all trades for a user
    """
    try:
        with conn.db.cursor() as cursor:
            cursor.execute("DELETE FROM trades WHERE userId = %s", [user_id])
            deleted_count = cursor.rowcount
            
            # Reset user stats
            cursor.execute(
                """
                UPDATE users 
                SET trade_stats = '{}'::jsonb
                WHERE id = %s
                """,
                [user_id]
            )
            
            conn.db.commit()
            return {"success": True, "message": f"Deleted {deleted_count} trades", "count": deleted_count}
            
    except Exception as e:
        conn.db.rollback()
        print(f"Error deleting trades: {str(e)}")
        return {"success": False, "message": f"Error deleting trades: {str(e)}"} 