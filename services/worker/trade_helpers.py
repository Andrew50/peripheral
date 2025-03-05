import traceback
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
    """Calculate P&L for a completed trade"""
    # Convert all numbers to Decimal for consistent decimal arithmetic
    total_entry_value = Decimal("0")
    total_entry_shares = Decimal("0")
    total_exit_value = Decimal("0")
    total_exit_shares = Decimal("0")

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
    avg_entry_price = (
        total_entry_value / total_entry_shares
        if total_entry_shares > 0
        else Decimal("0")
    )
    avg_exit_price = (
        total_exit_value / total_exit_shares if total_exit_shares > 0 else Decimal("0")
    )

    # Calculate P&L based on direction
    if direction == "Long":
        pnl = (avg_exit_price - avg_entry_price) * total_exit_shares
    else:  # Short
        pnl = (avg_entry_price - avg_exit_price) * total_exit_shares

    # Check if it's an options trade (ticker contains 'C' or 'P' and is longer than 4 characters)
    is_option = len(ticker) > 4 and ("C" in ticker or "P" in ticker)
    if is_option:
        pnl = pnl * Decimal("100")  # Multiply by 100 for options contracts
        total_contracts = total_entry_shares + total_exit_shares

        # Only apply commission if it's not a buy to close under $0.65
        should_apply_commission = True
        if direction == "Short" and avg_exit_price < Decimal("0.65"):
            should_apply_commission = False

        if should_apply_commission:
            commission = Decimal("0.65") * total_contracts  # $0.65 per contract
            pnl = pnl - commission

    print("\ncalculated pnl: ", pnl)
    return float(round(pnl, 2))


def process_trades(conn, user_id: int):
    """Process trade_executions into consolidated trades"""
    try:
        with conn.db.cursor() as cursor:
            print("\nStarting process_trades", flush=True)
            cursor.execute(
                """
                SELECT te.* 
                FROM trade_executions te
                WHERE te.userId = %s AND te.tradeId IS NULL
                ORDER BY te.timestamp ASC
            """,
                (user_id,),
            )

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

                cursor.execute(
                    """
                    SELECT * 
                    FROM trades
                    WHERE userId = %s
                      AND ticker = %s
                      AND status = 'Open'
                    ORDER BY date DESC
                    LIMIT 1
                """,
                    (user_id, ticker),
                )

                open_trade = cursor.fetchone()

                if not open_trade:
                    print("\nCreating new trade", flush=True)
                    open_quantity = trade_size

                    cursor.execute(
                        """
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
                    """,
                        (
                            user_id,
                            securityId,
                            ticker,
                            direction,
                            trade_date,
                            open_quantity,
                            trade_ts,
                            trade_price,
                            trade_size,
                        ),
                    )

                    new_trade_id = cursor.fetchone()[0]
                    cursor.execute(
                        """
                        UPDATE trade_executions
                        SET tradeId = %s
                        WHERE executionId = %s
                    """,
                        (new_trade_id, execution_id),
                    )

                else:
                    print("\nUpdating existing trade", flush=True)
                    trade_id = open_trade[0]
                    trade_dir = open_trade[4]
                    old_open_qty = float(open_trade[7])
                    new_open_qty = old_open_qty + trade_size
                    print(
                        f"\nTrade details - Direction: {trade_dir}, Old Qty: {old_open_qty}, New Qty: {new_open_qty}",
                        flush=True,
                    )

                    if trade_dir == direction:
                        is_same_direction = (old_open_qty > 0 and trade_size > 0) or (
                            old_open_qty < 0 and trade_size < 0
                        )
                        print(
                            f"\nSame direction check: {is_same_direction}", flush=True
                        )

                        if is_same_direction:
                            print("\nAdding to position", flush=True)
                            cursor.execute(
                                """
                                UPDATE trades
                                SET entry_times = array_append(entry_times, %s),
                                    entry_prices = array_append(entry_prices, %s),
                                    entry_shares = array_append(entry_shares, %s),
                                    openQuantity = %s
                                WHERE tradeId = %s
                            """,
                                (
                                    trade_ts,
                                    trade_price,
                                    trade_size,
                                    new_open_qty,
                                    trade_id,
                                ),
                            )
                        else:
                            print("\nReducing position (same direction)", flush=True)
                            cursor.execute(
                                """
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
                            """,
                                (
                                    trade_ts,
                                    trade_price,
                                    trade_size,
                                    new_open_qty,
                                    new_open_qty,
                                    trade_id,
                                ),
                            )

                            # Calculate and update P/L for partial exits
                            cursor.execute(
                                """
                                SELECT entry_prices, entry_shares,
                                       exit_prices, exit_shares,
                                       tradedirection
                                FROM trades
                                WHERE tradeId = %s
                            """,
                                (trade_id,),
                            )
                            trade_data = cursor.fetchone()
                            updated_pnl = calculate_pnl(
                                trade_data[0],
                                trade_data[1],
                                trade_data[2],
                                trade_data[3],
                                trade_data[4],
                                ticker,
                            )
                            cursor.execute(
                                """
                                UPDATE trades
                                SET closedPnL = %s
                                WHERE tradeId = %s
                            """,
                                (updated_pnl, trade_id),
                            )

                    else:
                        print("\nReducing position (opposite direction)", flush=True)
                        cursor.execute(
                            """
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
                        """,
                            (
                                trade_ts,
                                trade_price,
                                trade_size,
                                new_open_qty,
                                new_open_qty,
                                trade_id,
                            ),
                        )

                        # Calculate and update P/L for partial exits
                        cursor.execute(
                            """
                            SELECT entry_prices, entry_shares,
                                   exit_prices, exit_shares,
                                   tradedirection
                            FROM trades
                            WHERE tradeId = %s
                        """,
                            (trade_id,),
                        )
                        trade_data = cursor.fetchone()
                        updated_pnl = calculate_pnl(
                            trade_data[0],
                            trade_data[1],
                            trade_data[2],
                            trade_data[3],
                            trade_data[4],
                            ticker,
                        )
                        cursor.execute(
                            """
                            UPDATE trades
                            SET closedPnL = %s
                            WHERE tradeId = %s
                        """,
                            (updated_pnl, trade_id),
                        )

                    cursor.execute(
                        """
                        UPDATE trade_executions
                        SET tradeId = %s
                        WHERE executionId = %s
                    """,
                        (trade_id, execution_id),
                    )

            conn.db.commit()
            print("\nAll trades processed successfully", flush=True)
            return {"status": "success", "message": "Trades processed successfully"}

    except Exception as e:
        conn.db.rollback()
        error_info = traceback.format_exc()
        print(f"Error processing trades:\n{error_info}", flush=True)
        return {
            "status": "error",
            "message": f"Error: {str(e)}\nTraceback:\n{error_info}",
        }
