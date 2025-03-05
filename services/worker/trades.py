import base64
import pandas as pd
import io
from screen import getCurrentSecId
import traceback
import pytz
from trade_helpers import parse_datetime, process_trades


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

            if ticker:
                base_query += " AND (t.ticker = %s OR t.ticker LIKE %s)"
                params.extend([ticker, f"{ticker}%"])

            if date:
                base_query += " AND DATE(t.entry_times[1]) = %s"
                params.append(date)

            if hour is not None:
                base_query += " AND EXTRACT(HOUR FROM t.entry_times[1]) = %s"
                params.append(hour)

            sort_direction = "DESC" if sort.lower() == "desc" else "ASC"
            base_query += f" ORDER BY t.entry_times[1] {sort_direction}"

            cursor.execute(base_query, tuple(params))

            trades = []
            eastern = pytz.timezone("America/New_York")
            utc = pytz.UTC

            for row in cursor.fetchall():
                est_time = eastern.localize(row[9][0]) if row[9] else None
                combined_trades = []

                for i in range(len(row[9])) if row[9] else []:
                    combined_trades.append(
                        {
                            "time": eastern.localize(row[9][i])
                            .astimezone(utc)
                            .timestamp()
                            * 1000,
                            "price": float(row[10][i]),
                            "shares": row[11][i],
                            "type": "Short" if row[4] == "Short" else "Buy",
                        }
                    )

                for i in range(len(row[12])) if row[12] else []:
                    combined_trades.append(
                        {
                            "time": eastern.localize(row[12][i])
                            .astimezone(utc)
                            .timestamp()
                            * 1000,
                            "price": float(row[13][i]),
                            "shares": row[14][i],
                            "type": (
                                "Buy to Cover"
                                if row[4] == "Short" and row[7] <= 0
                                else "Sell"
                            ),
                        }
                    )

                combined_trades.sort(key=lambda x: x["time"])

                trade = {
                    "tradeId": row[0],
                    "ticker": row[3],
                    "securityId": row[2],
                    "tradeStart": (
                        eastern.localize(row[9][0]).astimezone(utc).timestamp() * 1000
                        if row[9]
                        else None
                    ),
                    "timestamp": (
                        eastern.localize(row[12][-1]).astimezone(utc).timestamp() * 1000
                        if row[12]
                        else (
                            eastern.localize(row[9][0]).astimezone(utc).timestamp()
                            * 1000
                            if row[9]
                            else None
                        )
                    ),
                    "trade_direction": row[4],
                    "date": row[5].strftime("%Y-%m-%d"),
                    "status": row[6],
                    "openQuantity": row[7],
                    "closedPnL": float(row[8]) if row[8] else None,
                    "trades": combined_trades,
                }
                trades.append(trade)

            return trades

    except Exception as e:
        error_info = traceback.format_exc()
        print(f"Error fetching trades: {e}\n{error_info}")
        return []


def handle_trade_upload(
    conn, file_content: str, user_id: int, additional_args: dict = None
) -> dict:
    """
    Process uploaded trade file and return parsed trades
    """
    try:
        file_bytes = base64.b64decode(file_content)
        df = pd.read_csv(
            io.BytesIO(file_bytes),
            skiprows=3,
            dtype={
                "Order Time": str,
                "Trade Description": str,
                "Status": str,
            },
        )
        df = df.dropna(how="all")

        with conn.db.cursor() as cursor:
            for i in range(len(df) - 1, -1, -1):
                trade = df.iloc[i]
                trade_description = trade["Trade Description"]
                fidelity_trade_status = trade["Status"]

                if (
                    "Short" in trade_description
                    or "Sell to Open" in trade_description
                    or "Buy to Cover" in trade_description
                    or "Buy to Close" in trade_description
                ):
                    trade_direction = "Short"
                else:
                    trade_direction = "Long"

                trade_dt, trade_date = parse_datetime(trade["Order Time"])
                ticker = trade["Symbol"]
                securityId = getCurrentSecId(conn, ticker)

                if "Limit" in trade_description or "Stop Loss" in trade_description:
                    if "FILLED AT" in fidelity_trade_status:
                        trade_price = float(
                            fidelity_trade_status.split("$")[1].replace(",", "")
                        )
                        trade_shares = float(trade["Quantity"].replace(",", ""))
                    elif "PARTIAL" in fidelity_trade_status:
                        trade_shares = float(
                            fidelity_trade_status.split("\n")[3]
                            .split(" ")[0]
                            .replace(",", "")
                        )
                        description_split = trade_description.split("$")
                        if len(description_split) == 2:
                            trade_price = float(description_split[1].replace(",", ""))
                        elif len(description_split) == 3:
                            trade_price = float(description_split[2].replace(",", ""))
                elif "Market" in trade_description:
                    trade_price = float(
                        fidelity_trade_status.split("$")[1].replace(",", "")
                    )
                    trade_shares = float(trade["Quantity"].replace(",", ""))

                if "Sell" in trade_description or "Short" in trade_description:
                    trade_shares = -trade_shares

                cursor.execute(
                    """
                    INSERT INTO trade_executions 
                    (userId, securityId, ticker, date, price, size, timestamp, direction)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                """,
                    (
                        user_id,
                        securityId,
                        ticker,
                        trade_date,
                        trade_price,
                        trade_shares,
                        trade_dt,
                        trade_direction,
                    ),
                )

            conn.db.commit()
        process_trades(conn, user_id)
        return {"status": "success", "message": "Trades uploaded successfully"}
    except Exception as e:
        conn.db.rollback()
        error_info = traceback.format_exc()
        print(f"Error processing trade file:\n{error_info}")
        return {
            "status": "error",
            "message": f"Error: {str(e)}\nTraceback:\n{error_info}",
        }


def get_trade_statistics(
    conn, user_id: int, start_date: str = None, end_date: str = None, ticker: str = None
) -> dict:
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

            # Modified ticker filter to include options
            if ticker:
                query += " AND (ticker = %s OR ticker LIKE %s)"
                params.extend([ticker, f"{ticker}%"])

            # Rest of the existing date filters
            if start_date:
                query += " AND DATE(entry_times[1]) >= %s"
                params.append(start_date)

            if end_date:
                query += " AND DATE(entry_times[1]) <= %s"
                params.append(end_date)

            cursor.execute(query, tuple(params))
            row = cursor.fetchone()

            total_trades = row[0]
            winning_trades = row[1]
            losing_trades = row[2]
            avg_win = float(row[3]) if row[3] else 0
            avg_loss = float(row[4]) if row[4] else 0
            total_pnl = float(row[5]) if row[5] else 0

            # Update P/L curve query with modified ticker filter
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
                pnl_query += " AND (ticker = %s OR ticker LIKE %s)"
                params.extend([ticker, f"{ticker}%"])

            pnl_query += " ORDER BY entry_times[1] ASC"

            # Update trades query for top/bottom trades
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
                trades_query += " AND (ticker = %s OR ticker LIKE %s)"
                params.extend([ticker, f"{ticker}%"])

            # Update hourly query
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
                hourly_query += " AND (ticker = %s OR ticker LIKE %s)"
                params.extend([ticker, f"{ticker}%"])

            hourly_query += " GROUP BY EXTRACT(HOUR FROM entry_times[1]) ORDER BY hour"

            # Update verification query
            verification_query = """
                SELECT COALESCE(SUM(closedPnL), 0) as verification_total
                FROM trades 
                WHERE userId = %s 
                AND status = 'Closed'
                AND closedPnL IS NOT NULL
            """
            verification_params = [user_id]

            if start_date:
                verification_query += " AND DATE(entry_times[1]) >= %s"
                verification_params.append(start_date)
            if end_date:
                verification_query += " AND DATE(entry_times[1]) <= %s"
                verification_params.append(end_date)
            if ticker:
                verification_query += " AND (ticker = %s OR ticker LIKE %s)"
                verification_params.extend([ticker, f"{ticker}%"])

            cursor.execute(verification_query, tuple(verification_params))
            verification_total = float(cursor.fetchone()[0])

            cursor.execute(pnl_query, tuple(params))
            pnl_data = cursor.fetchall()

            cumulative_pnl = []
            running_total = 0
            eastern = pytz.timezone("America/New_York")
            utc = pytz.UTC
            for row in pnl_data:
                # Convert EST timestamp to UTC milliseconds
                trade_time = eastern.localize(row[0]).astimezone(utc)
                timestamp = int(trade_time.timestamp() * 1000)
                pnl = float(row[1])
                running_total += pnl
                cumulative_pnl.append({"timestamp": timestamp, "value": running_total})

            # Get top 5 trades
            cursor.execute(
                trades_query + " ORDER BY closedPnL DESC LIMIT 5", tuple(params)
            )
            top_trades = [
                {
                    "ticker": row[0],
                    "timestamp": int(
                        eastern.localize(row[1]).astimezone(utc).timestamp() * 1000
                    ),
                    "direction": row[2],
                    "pnl": float(row[3]),
                }
                for row in cursor.fetchall()
            ]

            # Get bottom 5 trades, excluding the trade IDs from top trades
            bottom_trades_query = (
                trades_query
                + """
                AND NOT (ticker, entry_times[1], tradedirection, closedPnL) IN (
                    SELECT ticker, entry_times[1], tradedirection, closedPnL
                    FROM trades 
                    WHERE userId = %s 
                    AND status = 'Closed'
                    AND closedPnL IS NOT NULL
                    ORDER BY closedPnL DESC 
                    LIMIT 5
                )
                ORDER BY closedPnL ASC 
                LIMIT 5
            """
            )

            # Add the user_id parameter again for the subquery
            bottom_params = params + [user_id]
            cursor.execute(bottom_trades_query, tuple(bottom_params))

            bottom_trades = [
                {
                    "ticker": row[0],
                    "timestamp": int(
                        eastern.localize(row[1]).astimezone(utc).timestamp() * 1000
                    ),
                    "direction": row[2],
                    "pnl": float(row[3]),
                }
                for row in cursor.fetchall()
            ]

            # Get hourly statistics
            cursor.execute(hourly_query, tuple(params))
            hourly_stats = []

            for row in cursor.fetchall():
                hour = int(row[0])
                total = int(row[1])
                wins = int(row[2])
                losses = int(row[3])
                avg_pnl = float(row[4]) if row[4] else 0
                total_pnl = float(row[5]) if row[5] else 0

                hourly_stats.append(
                    {
                        "hour": hour,
                        "hour_display": f"{hour:02d}:00",
                        "total_trades": total,
                        "winning_trades": wins,
                        "losing_trades": losses,
                        "win_rate": round((wins / total * 100), 2) if total > 0 else 0,
                        "avg_pnl": round(avg_pnl, 2),
                        "total_pnl": round(total_pnl, 2),
                    }
                )

            # Fix the ticker statistics query
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
                {date_filters}
                {ticker_filter}
                GROUP BY ticker
                ORDER BY total_pnl DESC
            """

            # Build the query with filters
            date_filters = ""
            if start_date:
                date_filters += " AND DATE(entry_times[1]) >= %s"
            if end_date:
                date_filters += " AND DATE(entry_times[1]) <= %s"
            ticker_filter = " AND (ticker = %s OR ticker LIKE %s)" if ticker else ""

            query = ticker_query.format(
                date_filters=date_filters, ticker_filter=ticker_filter
            )

            # Build params list
            params = [user_id]
            if start_date:
                params.append(start_date)
            if end_date:
                params.append(end_date)
            if ticker:
                params.extend([ticker, f"{ticker}%"])

            cursor.execute(query, tuple(params))
            ticker_stats = []

            for row in cursor.fetchall():
                ticker_name = row[0]
                total = int(row[1])
                wins = int(row[2])
                losses = int(row[3])
                avg_pnl = float(row[4]) if row[4] else 0
                total_pnl = float(row[5]) if row[5] else 0

                ticker_stats.append(
                    {
                        "ticker": ticker_name,
                        "total_trades": total,
                        "winning_trades": wins,
                        "losing_trades": losses,
                        "win_rate": round((wins / total * 100), 2) if total > 0 else 0,
                        "avg_pnl": round(avg_pnl, 2),
                        "total_pnl": round(total_pnl, 2),
                    }
                )

            return {
                "total_trades": total_trades,
                "winning_trades": winning_trades,
                "losing_trades": losing_trades,
                "win_rate": (
                    round((winning_trades / total_trades * 100), 2)
                    if total_trades > 0
                    else 0
                ),
                "avg_win": round(avg_win, 2),
                "avg_loss": round(avg_loss, 2),
                "total_pnl": round(verification_total, 2),  # Use verified total
                "pnl_curve": cumulative_pnl,
                "top_trades": top_trades,
                "bottom_trades": bottom_trades,
                "hourly_stats": hourly_stats,
                "ticker_stats": ticker_stats,
            }

    except Exception as e:
        error_info = traceback.format_exc()
        print(f"Error calculating statistics:\n{error_info}")
        return {"error": str(e), "traceback": error_info}


def get_ticker_trades(
    conn, user_id: int, ticker: str, start_date: str = None, end_date: str = None
):
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
                    entries.append(
                        {
                            "time": int(time.timestamp() * 1000),
                            "price": float(price),
                            "isLong": direction == "Long",
                        }
                    )

                # Process exits
                for time, price in zip(exit_times, exit_prices):
                    exits.append(
                        {
                            "time": int(time.timestamp() * 1000),
                            "price": float(price),
                            "isLong": direction == "Long",
                        }
                    )

            return {"securityId": security_id, "entries": entries, "exits": exits}

    except Exception as e:
        error_info = traceback.format_exc()
        print(f"Error getting ticker trades:\n{error_info}")
        return {"error": str(e), "traceback": error_info}


def get_ticker_performance(
    conn,
    user_id: int,
    sort: str = "desc",
    date: str = None,
    hour: int = None,
    ticker: str = None,
):
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

            # Modified the ticker filter condition
            ticker_filter = " AND (ticker = %s OR ticker LIKE %s)" if ticker else ""

            # Replace placeholders in the query
            query = query.format(
                date_filter=" AND DATE(entry_times[1]) = %s" if date else "",
                hour_filter=(
                    " AND EXTRACT(HOUR FROM entry_times[1]) = %s"
                    if hour is not None
                    else ""
                ),
                ticker_filter=ticker_filter,
                sort_direction="DESC" if sort.lower() == "desc" else "ASC",
            )

            # Build params list with modified ticker parameters
            params = [user_id]
            if date:
                params.append(date)
            if hour is not None:
                params.append(hour)
            if ticker:
                params.extend([ticker, f"{ticker}%"])
            # Add params for second part of query
            params.extend([p for p in [user_id] + params[1:]])

            eastern = pytz.timezone("America/New_York")
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
                        "ticker": ticker_name,
                        "securityId": row[1],
                        "total_trades": int(row[2]),
                        "winning_trades": int(row[3]),
                        "losing_trades": int(row[4]),
                        "avg_pnl": round(float(row[5]) if row[5] else 0, 2),
                        "total_pnl": round(float(row[6]) if row[6] else 0, 2),
                        "timestamp": (
                            int(
                                eastern.localize(row[8]).astimezone(utc).timestamp()
                                * 1000
                            )
                            if row[8]
                            else None
                        ),
                        "trades": [],  # Initialize empty trades array
                    }

                # Add trade details if they exist
                if row[9] is not None:  # tradeId exists
                    # Create combined trades array
                    combined_trades = []

                    # Add entries
                    for i in range(len(row[10])):
                        combined_trades.append(
                            {
                                "time": int(
                                    eastern.localize(row[10][i])
                                    .astimezone(utc)
                                    .timestamp()
                                    * 1000
                                ),
                                "price": float(row[11][i]),
                                "shares": float(row[12][i]),
                                "type": "Short" if row[16] == "Short" else "Buy",
                            }
                        )

                    # Add exits
                    for i in range(len(row[13])):
                        combined_trades.append(
                            {
                                "time": int(
                                    eastern.localize(row[13][i])
                                    .astimezone(utc)
                                    .timestamp()
                                    * 1000
                                ),
                                "price": float(row[14][i]),
                                "shares": float(row[15][i]),
                                "type": (
                                    "Buy to Cover" if row[16] == "Short" else "Sell"
                                ),
                            }
                        )

                    # Sort combined trades by timestamp
                    combined_trades.sort(key=lambda x: x["time"])

                    # Extend the trades array instead of overwriting it
                    current_stats["trades"].extend(combined_trades)

                    # Sort all trades by timestamp after adding new ones
                    current_stats["trades"].sort(key=lambda x: x["time"])

            # Add the last ticker stats
            if current_stats is not None:
                ticker_stats.append(current_stats)

            return ticker_stats

    except Exception as e:
        error_info = traceback.format_exc()
        print(f"Error getting ticker performance:\n{error_info}")
        return []


def get_similar_trades(conn, trade_id: int, n_neighbors: int = 5):
    """Get trades similar to the specified trade"""
    try:
        from trade_analysis import find_similar_trades

        similar_trades = find_similar_trades(conn, trade_id, n_neighbors)

        if not similar_trades:
            return {"status": "error", "message": "No similar trades found"}

        return {"status": "success", "similar_trades": similar_trades}

    except Exception as e:
        error_info = traceback.format_exc()
        print(f"Error getting similar trades:\n{error_info}")
        return {"status": "error", "message": str(e), "traceback": error_info}


def delete_all_user_trades(conn, user_id: int) -> dict:
    """
    Delete all trades and trade executions for a user

    Args:
        conn: Database connection
        user_id (int): User ID
    """
    try:
        with conn.db.cursor() as cursor:
            # Delete all trade executions for the user
            cursor.execute(
                """
                DELETE FROM trade_executions
                WHERE userId = %s
            """,
                (user_id,),
            )
            executions_deleted = cursor.rowcount

            # Delete all trades for the user
            cursor.execute(
                """
                DELETE FROM trades
                WHERE userId = %s
            """,
                (user_id,),
            )
            trades_deleted = cursor.rowcount

            # Commit the transaction
            conn.db.commit()

            return {
                "status": "success",
                "message": f"Successfully deleted {trades_deleted} trades and {executions_deleted} trade executions",
                "trades_deleted": trades_deleted,
                "executions_deleted": executions_deleted,
            }

    except Exception as e:
        conn.db.rollback()
        error_info = traceback.format_exc()
        print(f"Error deleting trades:\n{error_info}")
        return {
            "status": "error",
            "message": f"Error: {str(e)}",
            "traceback": error_info,
        }
