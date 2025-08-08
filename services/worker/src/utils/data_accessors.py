"""
Data Accessor Functions
Provides efficient data access functions for strategy execution.
These functions replace the previous approach of passing large DataFrames to strategies.
"""

import logging
import re
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple
from concurrent.futures import ThreadPoolExecutor, as_completed
from zoneinfo import ZoneInfo

import numpy as np
import pandas as pd
from psycopg2.extras import RealDictCursor

from .context import Context

logger = logging.getLogger(__name__)

# Timezone constants and helpers
tz = "'America/New_York'"

# NOTE: Timestamps coming from the DB are stored as timestamptz. `EXTRACT(EPOCH FROM
# <timestamptz>)` already returns the number of seconds since 1970-01-01 **in UTC**.
# We deliberately do *not* apply an additional `AT TIME ZONE` here because doing so
# would introduce a *second* conversion and shift bars backwards by 4-5 h (ending
# up a calendar day early when later interpreted in EST/EDT).  All downstream
# code can convert the epoch integer into whatever timezone it needs.

def _ts_to_epoch(expr: str) -> str:
    """Return SQL snippet that extracts Unix epoch seconds from a timestamptz expr."""
    return f"EXTRACT(EPOCH FROM {expr})::bigint"

def _get_bar_data(
    ctx: Context,
    start_date: datetime,
    end_date: datetime,
    timeframe: str = "1d",
    columns: Optional[List[str]] = None,
    min_bars: int = 1,
    filters: Optional[Dict[str, Any]] = None,
    extended_hours: bool = False,
) -> pd.DataFrame:
    """
    Get OHLCV bar data as numpy array with context-aware date ranges and intelligent batching

    Args:
        timeframe: Data timeframe ('1d', '1h', '5m', etc.)
        columns: Desired columns (None = all: ticker, timestamp, open, high, low, close, volume)
        min_bars: Minimum number of bars of the specified timeframe required for a single calculation to be made
        filters: Dict of filtering criteria for securities table fields:
                - tickers: List[str] (e.g., ['AAPL', 'MRNA']) (None = all active securities)
                - sector: str (e.g., 'Technology', 'Healthcare')
                - industry: str (e.g., 'Software', 'Pharmaceuticals')
                - primary_exchange: str (e.g., 'NASDAQ', 'NYSE')
                - market_cap_min: float (minimum market cap)
                - market_cap_max: float (maximum market cap)
                - active: bool (default True if not specified)
        aggregate_mode: If True, disables batching for aggregate calculations (use with caution)
        extended_hours: If True, include premarket and after-hours data for intraday timeframes (seconds, minutes, hours)
            Only affects intraday timeframes - daily and above ignore this parameter
        start_date: Optional start date for filtering data (datetime object)
        end_date: Optional end date for filtering data (datetime object)

    Returns:
        numpy.ndarray with columns: ticker, timestamp, open, high, low, close, volume
    """
    logger.info("🔍 _get_bar_data called with: timeframe=%s, start_date=%s, end_date=%s, min_bars=%s, filters=%s", 
                timeframe, start_date, end_date, min_bars, filters)
    
        # Validate inputs
        # this should already be validated before  has been called
        # Extract tickers from filters if provided
    tickers = None
    if filters and 'tickers' in filters:
        tickers = filters['tickers']
        if not isinstance(tickers, list):
            if isinstance(tickers, str):
                tickers = [tickers]  # Convert single ticker to list
            else:
                tickers = None
    
    logger.info("📊 Extracted tickers: %s", tickers)
    
    # Check if we need to use batching (now with potentially corrected tickers)
    should_batch = _should_use_batching(tickers)
    logger.info("⚖️ Should use batching: %s (tickers count: %s)", should_batch, len(tickers) if tickers else "None")

    if should_batch:
        logger.info("📦 Using batched data retrieval")
        return _get_bar_data_batched(ctx, timeframe, columns, min_bars, filters, extended_hours, start_date, end_date)
    logger.info("🎯 Using single data retrieval")
    return _get_bar_data_single(ctx, timeframe, columns, min_bars, filters, extended_hours, start_date, end_date)


def _should_use_batching(tickers: Optional[List[str]] = None, aggregate_mode: bool = False) -> bool:
    """Determine if batching should be used based on the request parameters"""
    # Never batch if aggregate_mode is explicitly enabled
    if aggregate_mode:
        return False
    # Always batch when tickers=None (all securities)
    if tickers is None:
        return True
    if not tickers:  # Empty list case
        return True

    # Batch when ticker list is large
    if len(tickers) > 250:
        return True

    return False

def _get_bar_data_batched(
    ctx: Context,
    timeframe: str = "1d",
    columns: Optional[List[str]] = None,
    min_bars: int = 1,
    filters: Optional[Dict[str, Any]] = None,
    extended_hours: bool = False,
    start_date: Optional[datetime] = None,
    end_date: Optional[datetime] = None,
) -> pd.DataFrame:
    """Get bar data using batching approach for large datasets with parallel processing"""
    logger.info("📦 _get_bar_data_batched called with: timeframe=%s, start_date=%s, end_date=%s, min_bars=%s, filters=%s", 
                timeframe, start_date, end_date, min_bars, filters)
    
    #try:
    batch_size = 1000
    all_results: List[pd.DataFrame] = []

    # Extract tickers from filters
    tickers = None
    if filters and 'tickers' in filters:
        tickers = filters['tickers']
        if not isinstance(tickers, list):
            if isinstance(tickers, str):
                tickers = [tickers]
            else:
                tickers = None

    logger.info("📊 Batching: extracted tickers=%s", tickers)

    # Get the universe of tickers to process
    if tickers is None:
        # Get all active tickers
        universe_tickers = _get_all_active_tickers(ctx, filters)
        if not universe_tickers:
            logger.warning("⚠️ No active tickers found in universe")
            return pd.DataFrame()
        logger.info("🌍 Retrieved %d active tickers from universe", len(universe_tickers))
    else:
        universe_tickers = tickers
        logger.info("🎯 Using provided %d tickers", len(universe_tickers))

    # Create batches
    ticker_batches = []
    for i in range(0, len(universe_tickers), batch_size):
        batch_tickers = universe_tickers[i:i + batch_size]
        ticker_batches.append(batch_tickers)

    logger.info("📦 Created %d batches of size %d", len(ticker_batches), batch_size)

    # Helper function to process a single batch
    def process_batch(batch_tickers: List[str], batch_num: int) -> Optional[pd.DataFrame]:
        logger.info("🔄 Processing batch %d with %d tickers: %s", batch_num, len(batch_tickers), batch_tickers[:5])
        try:
            # Create batch filters with tickers
            batch_filters = filters.copy() if filters else {}
            batch_filters['tickers'] = batch_tickers

            # Get data for this batch using the single method
            batch_result = _get_bar_data_single(
                ctx,
                timeframe=timeframe,
                columns=columns,
                min_bars=min_bars,
                filters=batch_filters,
                extended_hours=extended_hours,
                start_date=start_date,
                end_date=end_date
            )
            if batch_result is not None and len(batch_result) > 0:
                logger.info("✅ Batch %d returned %d rows", batch_num, len(batch_result))
                return batch_result
            logger.warning("⚠️ Batch %s returned no data", batch_num)
            return None
        except ValueError as batch_error:
            logger.error("❌ Error in batch %s: %s", batch_num, batch_error)
            return None

    # Process batches in parallel using ThreadPoolExecutor
    # Determine optimal number of workers based on batch count and system resources
    max_workers = min(len(ticker_batches), 10)  # Cap at 10 to avoid overwhelming the database
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        # Submit all batch processing tasks
        future_to_batch = {
            executor.submit(process_batch, batch_tickers, i + 1): i + 1
            for i, batch_tickers in enumerate(ticker_batches)
        }
        # Collect results as they complete
        for future in as_completed(future_to_batch):
            # Collect results; batch index not required here
            #try:
            result = future.result()
            if result is not None:
                all_results.append(result)
            #except Exception as exc:  # pylint: disable=broad-except
                #logger.error("❌ Batch %s generated an exception: %s", batch_num, exc)

    # Combine all batch results
    if all_results:
        # Concatenate DataFrames efficiently (no copies when possible)
        combined_result = pd.concat(all_results, ignore_index=True, copy=False)
        logger.info("🎉 Combined %d batch results into DataFrame with %d total rows", 
                    len(all_results), len(combined_result))
        return combined_result

    logger.warning("⚠️ No data returned from any batch")
    return pd.DataFrame()

    #except Exception as exc:  # pylint: disable=broad-except
        #logger.error("❌ Error in batched data fetching: %s", exc)
        #return pd.DataFrame()

def _get_all_active_tickers(ctx: Context, filters: Optional[Dict[str, Any]] = None) -> List[str]:
    """Get list of all active tickers with optional filtering"""
    #try:
    # Build filter conditions for active securities
    filter_parts: List[str] = ["maxdate IS NULL", "active = true"]
    params: List[Any] = []
    # Apply additional filters if provided (excluding tickers which is handled separately)
    if filters:
        if 'sector' in filters:
            filter_parts.append("sector = %s")
            params.append(filters['sector'])
        if 'industry' in filters:
            filter_parts.append("industry = %s")
            params.append(filters['industry'])
        if 'primary_exchange' in filters:
            filter_parts.append("primary_exchange = %s")
            params.append(filters['primary_exchange'])
        if 'market_cap_min' in filters:
            filter_parts.append("market_cap >= %s")
            params.append(filters['market_cap_min'])
        if 'market_cap_max' in filters:
            filter_parts.append("market_cap <= %s")
            params.append(filters['market_cap_max'])
        if 'total_employees_min' in filters:
            filter_parts.append("total_employees >= %s")
            params.append(filters['total_employees_min'])
        if 'total_employees_max' in filters:
            filter_parts.append("total_employees <= %s")
            params.append(filters['total_employees_max'])
        if 'weighted_shares_outstanding_min' in filters:
            filter_parts.append("weighted_shares_outstanding >= %s")
            params.append(filters['weighted_shares_outstanding_min'])
        if 'weighted_shares_outstanding_max' in filters:
            filter_parts.append("weighted_shares_outstanding <= %s")
            params.append(filters['weighted_shares_outstanding_max'])
    where_clause = " AND ".join(filter_parts)
    # Safe: filter_parts contains only hardcoded strings, all user input is parameterized
    query = f"SELECT ticker FROM securities WHERE {where_clause} ORDER BY ticker"  # nosec B608
    with ctx.conn.transaction() as cursor:
        cursor.execute(query, params)
        results = cursor.fetchall()
    return [row['ticker'] for row in results if row and row['ticker']]  # Filter out None tickers
    #except Exception as exc:  # pylint: disable=broad-except
        #logger.error("Error fetching active tickers: %s", exc)
        #return []


def _build_query(
    timeframe: str,
    columns: List[str],
    min_bars: int,
    filters: Optional[Dict[str, Any]] = None,
    extended_hours: bool = False,
    start_date: Optional[datetime] = None,
    end_date: Optional[datetime] = None,
) -> Tuple[str, List[Any], List[str]]:
    """
    Unified query builder that handles all combinations of aggregated/direct and realtime/date-range modes.

    Args:
        ctx: Database context
        timeframe: Data timeframe ('1d', '1h', '5m', etc.)
        columns: Desired columns
        min_bars: Minimum number of bars required
        filters: Security filters
        extended_hours: Include extended hours data
        start_date: Optional start date for date-range mode
        end_date: Optional end date for date-range mode

    Returns:
        Tuple of (sql_query, params, column_order)
    """
    # Parse timeframe to determine aggregation needs
    bucket_sql, base_table = _parse_timeframe(timeframe)

    # Determine execution mode
    realtime_mode = start_date is None and end_date is None
    needs_aggregation = not (
        (bucket_sql == "1 minute" and base_table == "ohlcv_1m") or
        (bucket_sql == "1 day" and base_table == "ohlcv_1d")
    )

    # Validate columns and build safe column list
    allowed_columns = {"ticker", "timestamp", "open", "high", "low", "close", "volume", "transactions"}
    safe_columns = [col for col in (columns or ["ticker", "timestamp", "open", "high", "low", "close", "volume"])
                   if col in allowed_columns]

    if not safe_columns:
        return "SELECT NULL WHERE FALSE", [], []

    if needs_aggregation:
        return _build_aggregated_query(bucket_sql, base_table, safe_columns, min_bars, filters,
                                     extended_hours, start_date, end_date, realtime_mode)

    return _build_direct_query(
        base_table,
        safe_columns,
        min_bars,
        filters,
        extended_hours,
        start_date,
        end_date,
        realtime_mode,
    )


def _build_aggregated_query(
    bucket_sql: str,
    base_table: str,
    columns: List[str],
    min_bars: int,
    filters: Optional[Dict[str, Any]] = None,
    extended_hours: bool = False,
    start_date: Optional[datetime] = None,
    end_date: Optional[datetime] = None,
    realtime_mode: bool = True,
) -> Tuple[str, List[Any], List[str]]:
    """Build query for aggregated timeframes using TimescaleDB time_bucket."""

    # Build the aggregation CTE
    agg_cte_sql, agg_params = _build_agg_cte(bucket_sql, base_table, columns, filters,
                                           extended_hours, start_date, end_date)

    # Build final column selection and intermediate selection
    final_columns = []
    select_columns = []
    for col in columns:
        if col == "ticker":
            final_columns.append("ticker")
            select_columns.append("ticker")
        elif col == "timestamp":
            final_columns.append(f"{_ts_to_epoch('bucket_ts')} AS timestamp")
            select_columns.append("bucket_ts")
        else:
            final_columns.append(col)
            select_columns.append(col)

    final_select_clause = ', '.join(final_columns)
    agg_select_clause = ', '.join(select_columns)

    if realtime_mode:
        # Realtime mode: get latest min_bars per ticker
        query = f"""WITH {agg_cte_sql},
            ranked_data AS (
                SELECT {agg_select_clause},
                    ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY bucket_ts DESC) as rn,
                    COUNT(*) OVER (PARTITION BY ticker) as total_bars
                FROM agg
            )
            SELECT {final_select_clause}
            FROM ranked_data
            WHERE rn <= %s AND total_bars >= %s
            ORDER BY ticker, bucket_ts DESC"""  # nosec B608

        params = agg_params + [min_bars, min_bars]
    else:
        # Date-range mode: get data in range plus min_bars-1 pre-roll
        pre_bars_needed = max(min_bars - 1, 0)

        query = f"""WITH {agg_cte_sql},
            pre_bars AS (
                SELECT {agg_select_clause}, ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY bucket_ts DESC) as rn_pre
                FROM agg
                WHERE bucket_ts < %s
            ),
            in_range AS (
                SELECT {agg_select_clause} FROM agg
                WHERE bucket_ts >= %s AND bucket_ts <= %s
            )
            SELECT {final_select_clause}
            FROM (
                SELECT {agg_select_clause}
                FROM pre_bars WHERE rn_pre <= %s
                UNION ALL
                SELECT {agg_select_clause}
                FROM in_range
            ) AS combined
            ORDER BY ticker, bucket_ts ASC"""  # nosec B608

        params = (
            agg_params
            + [
                _normalize_est(start_date),
                _normalize_est(start_date),
                _normalize_est(end_date),
                pre_bars_needed,
            ]
        )

    return query, params, columns


def _build_direct_query(
    base_table: str,
    columns: List[str],
    min_bars: int,
    filters: Optional[Dict[str, Any]] = None,
    extended_hours: bool = False,
    start_date: Optional[datetime] = None,
    end_date: Optional[datetime] = None,
    realtime_mode: bool = True,
) -> Tuple[str, List[Any], List[str]]:
    """Build query for direct table access (1m or 1d tables)."""

    # Build column selection with proper scaling for prices
    select_columns = []
    final_columns = []
    for col in columns:
        if col == "ticker":
            select_columns.append("o.ticker")
            final_columns.append("ticker")
        elif col == "timestamp":
            select_columns.append(f"{_ts_to_epoch('o.timestamp')} AS timestamp")
            final_columns.append("timestamp")
        elif col == "volume":
            select_columns.append("o.volume AS volume")
            final_columns.append("volume")
        elif col in ["open", "high", "low", "close"]:
            # Keep OHLC values as raw bigint (will divide by 10000 in NumPy for better performance)
            select_columns.append(f"o.{col} AS {col}")
            final_columns.append(col)
        else:
            select_columns.append(f"o.{col}")
            final_columns.append(col)

    # Build column versions without the table alias (used when selecting from CTEs)
    # Fix A: Re-use the already-aliased column names from final_columns
    select_no_alias_columns = final_columns
    select_no_alias_clause = ', '.join(select_no_alias_columns)

    # Build filters (tickers only). Date conditions for the windowing query are
    # added explicitly later to avoid contradictory clauses when we need both
    # “pre-roll” (< start_date) and "in-range" (>= start_date AND <= end_date)
    filter_parts, params = _build_ticker_and_date_filters(filters)

    # Add extended hours filtering for minute data
    if base_table == "ohlcv_1m" and not extended_hours:
        extended_hours_filter = f"""(
            EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE {tz})) > 9 OR
            (EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE {tz})) = 9 AND
                EXTRACT(MINUTE FROM (o.timestamp AT TIME ZONE {tz})) >= 30)
        ) AND (
            EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE {tz})) < 16
        ) AND (
            EXTRACT(DOW FROM (o.timestamp AT TIME ZONE {tz})) BETWEEN 1 AND 5
        )"""
        filter_parts.append(extended_hours_filter)

    # Build WHERE clause
    where_clause = " AND ".join(filter_parts) if filter_parts else "TRUE"
    select_clause = ', '.join(select_columns)
    final_select_clause = ', '.join(final_columns)

    if realtime_mode:
        # Realtime mode: get latest min_bars per ticker
        query = f"""WITH ranked_data AS (
            SELECT {select_clause},
                ROW_NUMBER() OVER (PARTITION BY o.ticker ORDER BY o.timestamp DESC) as rn,
                COUNT(*) OVER (PARTITION BY o.ticker) as total_bars
            FROM {base_table} o
            WHERE {where_clause}
        )
        SELECT {final_select_clause}
        FROM ranked_data
        WHERE rn <= %s AND total_bars >= %s
        ORDER BY ticker, timestamp DESC"""  # nosec B608

        params = params + [min_bars, min_bars]
    else:
        # Date-range mode: get data in range plus min_bars-1 pre-roll
        pre_bars_needed = max(min_bars - 1, 0)

        query = f"""WITH pre_bars AS (
            SELECT {select_clause},
                ROW_NUMBER() OVER (PARTITION BY o.ticker ORDER BY o.timestamp DESC) as rn_pre
            FROM {base_table} o
            WHERE {where_clause} AND o.timestamp < %s
        ),
        in_range AS (
            SELECT {select_clause}
            FROM {base_table} o
            WHERE {where_clause} AND o.timestamp >= %s AND o.timestamp <= %s
        )
        SELECT {final_select_clause}
        FROM (
            SELECT {select_no_alias_clause} FROM pre_bars WHERE rn_pre <= %s
            UNION ALL
            SELECT {select_no_alias_clause} FROM in_range
        ) AS combined
        ORDER BY ticker, timestamp ASC"""  # nosec B608

        # Parameter order must match placeholders:
        #   1) base filter params for pre_bars
        #   2) < start_date
        #   3) base filter params for in_range
        #   4) >= start_date, <= end_date, and rn_pre limit
        params = (
            params
            + [_normalize_est(start_date)]
            + params
            + [_normalize_est(start_date), _normalize_est(end_date), pre_bars_needed]
        )

    return query, params, columns


def _build_ticker_and_date_filters(
    filters: Optional[Dict[str, Any]] = None,
) -> Tuple[List[str], List[Any]]:
    """Build common ticker filter parts for direct queries.

    Date constraints are applied by the calling query builder (e.g. pre-roll vs in-range)
    to avoid contradictory conditions. This helper now concerns itself **only** with
    ticker lists that may come from the caller-supplied `filters` dict.
    """
    filter_parts: List[str] = []
    params: List[Any] = []

    # Handle ticker filtering, if provided
    if filters and 'tickers' in filters:
        tickers = filters['tickers']
        if not isinstance(tickers, list):
            if isinstance(tickers, str):
                tickers = [tickers]
            else:
                tickers = None

        if tickers:
            placeholders = ','.join(['%s'] * len(tickers))
            filter_parts.append(f"o.ticker IN ({placeholders})")
            params.extend(tickers)

    return filter_parts, params


def _get_bar_data_single(
    ctx: Context,
    timeframe: str = "1d",
    columns: Optional[List[str]] = None,
    min_bars: int = 1,
    filters: Optional[Dict[str, Any]] = None,
    extended_hours: bool = False,
    start_date: Optional[datetime] = None,
    end_date: Optional[datetime] = None,
) -> pd.DataFrame:
    """
    Get OHLCV bar data as numpy array using TimescaleDB aggregation with context-aware date ranges

    Args:
        timeframe: Data timeframe ('1d', '1h', '5m', etc.) - now supports any format like '90m', '13d', '3w'
        columns: Desired columns (None = default: ticker, timestamp, open, high, low, close, volume)
        min_bars: Minimum number of bars of the specified timeframe required
        filters: Dict of filtering criteria for securities table fields:
                - tickers: List[str] (e.g., ['AAPL', 'MRNA']) (None = all active securities)
                - sector: str (e.g., 'Technology', 'Healthcare')
                - industry: str (e.g., 'Software', 'Pharmaceuticals')
                - primary_exchange: str (e.g., 'NASDAQ', 'NYSE')
                - market_cap_min: float (minimum market cap)
                - market_cap_max: float (maximum market cap)
                - active: bool (default True if not specified)
        extended_hours: If True, include premarket and after-hours data for intraday timeframes (seconds, minutes, hours)
            Only affects intraday timeframes - daily and above ignore this parameter
        start_date: Optional start date for filtering data (datetime object)
        end_date: Optional end date for filtering data (datetime object)

    Returns:
        numpy.ndarray with columns: ticker, timestamp, open, high, low, close, volume
    """
    logger.info("🎯 _get_bar_data_single called with: timeframe=%s, start_date=%s, end_date=%s, min_bars=%s, filters=%s", 
                timeframe, start_date, end_date, min_bars, filters)
    
    # Validate date parameters
    if start_date is not None and end_date is None or start_date is None and end_date is not None:
        raise ValueError("start_date and end_date must be provided together")

    # Determine execution modes independently
    realtime_mode = (start_date is None and end_date is None)
    
    # Parse timeframe to get bucket SQL and base table
    bucket_sql, base_table = _parse_timeframe(timeframe)
    logger.info("📅 Parsed timeframe: bucket_sql='%s', base_table='%s'", bucket_sql, base_table)

    # Check if this is a direct table access (no aggregation needed)
    needs_aggregation = not (
        (bucket_sql == "1 minute" and base_table == "ohlcv_1m") or
        (bucket_sql == "1 day" and base_table == "ohlcv_1d")
    )
    
    logger.info("🕒 Execution mode: %s", "realtime" if realtime_mode else "date-range")
    logger.info("🔧 Needs aggregation: %s", needs_aggregation)

    # Default columns if not specified - include ticker by default
    if not columns:
        columns = ["ticker", "timestamp", "open", "high", "low", "close", "volume"]

    # Validate columns against allowed set (removed adj_close and securityid - not in table)
    allowed_columns = {"ticker", "timestamp", "open", "high",
    "low", "close", "volume", "transactions"}
    safe_columns = [col for col in columns if col in allowed_columns]
    logger.info("📋 Columns: requested=%s, safe=%s", columns, safe_columns)

    if not safe_columns:
        # Return empty DataFrame with expected columns for consistency
        col_names = [c.split(' as ')[-1].split('.')[-1] for c in safe_columns]
        return pd.DataFrame(columns=col_names)

    # Execute appropriate query based on the four execution modes
    if needs_aggregation:
        # Branch 1: Real-time mode, direct table access
        logger.info("🎯 Branch 1: Real-time, direct table access")
        query, params, _ = _build_direct_query(base_table, safe_columns, min_bars, filters, 
                                          extended_hours, start_date, end_date, realtime_mode)
        
    else: 
        query, params, _ = _build_direct_query(base_table, safe_columns, min_bars, filters, 
                                          extended_hours, start_date, end_date, realtime_mode)


    # Execute query and process results (shared for all branches)
    return _execute_and_process_query(ctx, query, params, safe_columns)


def _execute_and_process_query(
    ctx: Context, query: str, params: List[Any], safe_columns: List[str]
) -> pd.DataFrame:
    """Execute query and process results into DataFrame with price scaling"""
    # Execute query with fastest cursor (plain tuples)
    with ctx.conn.get_connection() as conn:
        cursor = conn.cursor(cursor_factory=None)  # Use plain cursor for maximum performance
        logger.info("🗃️ Executing query with %d parameters", len(params))
        logger.info("📝 Query: %s", query)
        logger.info("🔢 Parameters: %s", params)
        
        cursor.execute(query, params)
        results = cursor.fetchall()
        cursor.close()
        
    logger.info("📊 Query returned %d rows", len(results))
    
    if not results:
        # Return empty DataFrame with expected columns for consistency
        col_names = [c.split(' as ')[-1].split('.')[-1] for c in safe_columns]
        logger.info("⚠️ No results found, returning empty DataFrame with columns: %s", col_names)
        return pd.DataFrame(columns=col_names)
    
    # Create DataFrame directly from fetched tuples – fastest when columns are known
    col_names = [c.split(' as ')[-1].split('.')[-1] for c in safe_columns]
    df = pd.DataFrame(results, columns=col_names)
    logger.info("✅ Created DataFrame with shape: %s, columns: %s", df.shape, list(df.columns))
    
    # Apply vectorized scaling to OHLC price columns (divide by 1000)
    price_columns = [col for col in ['open', 'high', 'low', 'close'] if col in df.columns]
    if price_columns:
        df[price_columns] = df[price_columns].astype(np.float64).div(1000.0)
        logger.info("💰 Applied price scaling to columns: %s", price_columns)
    
    logger.info("🎉 Returning DataFrame with %d rows for %d unique tickers", 
                len(df), df['ticker'].nunique() if 'ticker' in df.columns else 0)
    return df


def _get_security_ids_from_tickers(
    ctx: Context, tickers: List[str], filters: Optional[Dict[str, Any]] = None
) -> List[int]:
    """Convert ticker symbols to security IDs with optional filtering"""
    #try:
    if not tickers:
        return []

    # Build filter conditions
    filter_parts = ["maxdate IS NULL"]
    params = []

    # Add ticker filter
    placeholders = ','.join(['%s'] * len(tickers))
    filter_parts.append(f"ticker IN ({placeholders})")
    params.extend(tickers)

    # Apply additional filters if provided
    if filters:
        if 'sector' in filters:
            filter_parts.append("sector = %s")
            params.append(filters['sector'])

        if 'industry' in filters:
            filter_parts.append("industry = %s")
            params.append(filters['industry'])

        #if 'market' in filters:
        #    filter_parts.append("market = %s")
        #    params.append(filters['market'])

        if 'primary_exchange' in filters:
            filter_parts.append("primary_exchange = %s")
            params.append(filters['primary_exchange'])

        if 'market_cap_min' in filters:
            filter_parts.append("market_cap >= %s")
            params.append(filters['market_cap_min'])

        if 'market_cap_max' in filters:
            filter_parts.append("market_cap <= %s")
            params.append(filters['market_cap_max'])

        if 'total_employees_min' in filters:
            filter_parts.append("total_employees >= %s")
            params.append(filters['total_employees_min'])

        if 'total_employees_max' in filters:
            filter_parts.append("total_employees <= %s")
            params.append(filters['total_employees_max'])

        if 'weighted_shares_outstanding_min' in filters:
            filter_parts.append("weighted_shares_outstanding >= %s")
            params.append(filters['weighted_shares_outstanding_min'])

        if 'weighted_shares_outstanding_max' in filters:
            filter_parts.append("weighted_shares_outstanding <= %s")
            params.append(filters['weighted_shares_outstanding_max'])

        if 'active' in filters:
            filter_parts.append("active = %s")
            params.append(filters['active'])
        else:
            # Default to active if not explicitly specified
            filter_parts.append("active = true")
    else:
        # Default to active if no filters provided
        filter_parts.append("active = true")

    where_clause = " AND ".join(filter_parts)
    # nosec B608: Safe - query built from validated components, all values parameterized
    query = f"SELECT securityid FROM securities WHERE {where_clause}"  # nosec B608

    with ctx.conn.transaction() as cursor:
        cursor.execute(query, params)
        results = cursor.fetchall()

    return [row['securityid'] for row in results]

    #except Exception as exc:  # pylint: disable=broad-except
        #logger.error("Error converting tickers to security IDs: %s", exc)
        #return []

def _get_general_data(
    ctx: Context,
    columns: Optional[List[str]] = None,
    filters: Optional[Dict[str, Any]] = None,
) -> pd.DataFrame:
    """
    Get general security information as pandas DataFrame

    Args:
        columns: Desired columns (None = all available)
        filters: Dict of filtering criteria for securities table fields:
                - tickers: List[str] (e.g., ['AAPL', 'MRNA']) (None = all active securities)
                - sector: str (e.g., 'Technology', 'Healthcare')
                - industry: str (e.g., 'Software', 'Pharmaceuticals')
                - primary_exchange: str (e.g., 'NASDAQ', 'NYSE')
                - market_cap_min: float (minimum market cap)
                - market_cap_max: float (maximum market cap)
                - active: bool (default True if not specified)

    Returns:
        pandas.DataFrame with columns: ticker, name, sector, industry, primary_exchange,
            active, description, cik, market_cap, etc.
    """
    #try:
    # Extract tickers from filters if provided
    tickers: Optional[List[str]] = None
    if filters and 'tickers' in filters:
        tickers = filters['tickers']
        if not isinstance(tickers, list):
            if isinstance(tickers, str):
                tickers = [tickers]  # Convert single ticker to list
            else:
                tickers = None

    # Default columns if not specified - include ticker by default
    if columns is None:
        columns = ["ticker", "name", "sector", "industry", "primary_exchange",
        "active", "description", "cik"]

    # Validate columns against allowed set
    allowed_columns = {
        "securityid", "ticker", "name", "sector", "industry", "market",
        "primary_exchange", "active", "description", "cik",
        "market_cap", "share_class_shares_outstanding", "share_class_figi",
        "total_employees", "weighted_shares_outstanding"
    }
    safe_columns = [col for col in columns if col in allowed_columns]

    if not safe_columns:
        return pd.DataFrame()

    # Always include securityid for internal processing, but include ticker for user access
    internal_columns = safe_columns.copy()
    if "securityid" not in internal_columns:
        internal_columns = ["securityid"] + internal_columns
    if "ticker" not in internal_columns and "ticker" in allowed_columns:
        internal_columns.append("ticker")

    # Build the query with filters
    filter_parts = ["maxdate IS NULL"]
    params: List[Any] = []

    # Apply filters if provided
    if filters:
        if 'sector' in filters:
            filter_parts.append("sector = %s")
            params.append(filters['sector'])

        if 'industry' in filters:
            filter_parts.append("industry = %s")
            params.append(filters['industry'])

        if 'primary_exchange' in filters:
            filter_parts.append("primary_exchange = %s")
            params.append(filters['primary_exchange'])

        if 'market_cap_min' in filters:
            filter_parts.append("market_cap >= %s")
            params.append(filters['market_cap_min'])

        if 'market_cap_max' in filters:
            filter_parts.append("market_cap <= %s")
            params.append(filters['market_cap_max'])

        if 'total_employees_min' in filters:
            filter_parts.append("total_employees >= %s")
            params.append(filters['total_employees_min'])

        if 'total_employees_max' in filters:
            filter_parts.append("total_employees <= %s")
            params.append(filters['total_employees_max'])

        if 'weighted_shares_outstanding_min' in filters:
            filter_parts.append("weighted_shares_outstanding >= %s")
            params.append(filters['weighted_shares_outstanding_min'])

        if 'weighted_shares_outstanding_max' in filters:
            filter_parts.append("weighted_shares_outstanding <= %s")
            params.append(filters['weighted_shares_outstanding_max'])

        if 'active' in filters:
            filter_parts.append("active = %s")
            params.append(filters['active'])
        else:
            # Default to active if not explicitly specified
            filter_parts.append("active = true")
    else:
        # Default to active if no filters provided
        filter_parts.append("active = true")

    # Handle ticker-specific filtering
    if tickers is not None and len(tickers) > 0:
        # Convert ticker symbols to security IDs and add to filter
        security_ids = _get_security_ids_from_tickers(ctx, tickers, filters)
        if not security_ids:
            logger.warning("No security IDs found for provided tickers")
            return pd.DataFrame()

        # Use converted security IDs
        placeholders = ','.join(['%s'] * len(security_ids))
        filter_parts.append(f"securityid IN ({placeholders})")
        params.extend(security_ids)

    # Build final query
    where_clause = " AND ".join(filter_parts)
    select_clause = ', '.join(internal_columns)
    # nosec B608: Safe - columns validated against allowlist, all values parameterized
    query = f"SELECT {select_clause} FROM securities WHERE {where_clause} ORDER BY securityid"  # nosec B608

    with ctx.conn.get_connection() as conn:
        cursor = conn.cursor(cursor_factory=RealDictCursor)

        cursor.execute(query, params)
        results = cursor.fetchall()

        cursor.close()

    if not results:
        return pd.DataFrame()

    # Convert to DataFrame
    df = pd.DataFrame(results)

    # Filter to only requested columns for the final result
    final_columns = [col for col in safe_columns if col in df.columns]
    if final_columns:
        df = df[final_columns]

    # IMPORTANT: Don't set ticker as index if it was explicitly requested as a column
    # Strategies need ticker as a column to iterate over results
    # Only set securityid as index if it was explicitly requested
    if "securityid" in safe_columns and "securityid" in df.columns:
        df.set_index("securityid", inplace=True)

    return df

    #except Exception as e:
        #logger.error(f"Error in get_general_data: {e}")
        #return pd.DataFrame()



def _parse_timeframe(timeframe: str) -> tuple[str, str]:
    """Parse timeframe string and return (bucket_sql, base_table)

    Args:
        timeframe: Timeframe string like '5', '5h', '2d', '3w', '1m', '1q', '1y'

    Returns:
        Tuple of (bucket_sql, base_table) where:
        - bucket_sql: TimescaleDB interval string like '5 minutes', '2 hours', '1 day'
        - base_table: Database table to use ('ohlcv_1m' or 'ohlcv_1d')

    Raises:
        ValueError: If timeframe format is invalid or unsupported
    """

    # Validate timeframe format
    pattern = r'^(\d+)(m|h|d|w|q|y)?$'
    match = re.match(pattern, timeframe.lower())
    if not match:
        raise ValueError(f"Invalid timeframe format: {timeframe}. Expected format like '5', '5h', '2d', '3w', '1m', '1q', '1y'")

    value, unit = match.groups()
    value = int(value)

    # Default to minutes if no unit specified
    if not unit:
        bucket_sql = f"{value} minute{'s' if value != 1 else ''}"
        base_table = "ohlcv_1m"
    elif unit == 'h':
        # Hours - use ohlcv_1m as base for sub-daily aggregation
        bucket_sql = f"{value} hour{'s' if value != 1 else ''}"
        base_table = "ohlcv_1m"
    elif unit == 'd':
        # Days - use ohlcv_1d as base
        bucket_sql = f"{value} day{'s' if value != 1 else ''}"
        base_table = "ohlcv_1d"
    elif unit == 'w':
        # Weeks - use ohlcv_1d as base
        bucket_sql = f"{value} week{'s' if value != 1 else ''}"
        base_table = "ohlcv_1d"
    elif unit == 'm':
        # Months - use ohlcv_1d as base
        bucket_sql = f"{value} month{'s' if value != 1 else ''}"
        base_table = "ohlcv_1d"
    elif unit == 'q':
        # Quarters - use ohlcv_1d as base (3 months per quarter)
        months = value * 3
        bucket_sql = f"{months} month{'s' if months != 1 else ''}"
        base_table = "ohlcv_1d"
    elif unit == 'y':
        # Years - use ohlcv_1d as base
        bucket_sql = f"{value} year{'s' if value != 1 else ''}"
        base_table = "ohlcv_1d"
    else:
        raise ValueError(f"Unsupported timeframe unit: {unit}")

    return bucket_sql, base_table

def _build_agg_cte(
    bucket_sql: str,
    base_table: str,
    columns: List[str],
    filters: Optional[Dict[str, Any]] = None,
    extended_hours: bool = False,
    start_date: Optional[datetime] = None,
    end_date: Optional[datetime] = None,
) -> Tuple[str, List[Any]]:
    """Build aggregation CTE SQL and parameters

    Args:
        bucket_sql: TimescaleDB interval string like '5 minutes'
        base_table: Database table to use
        columns: Desired columns
        filters: Security filters
        extended_hours: Include extended hours data
        start_date: Optional start date filter
        end_date: Optional end date filter

    Returns:
        Tuple of (cte_sql, params)
    """
    # Build ticker filter parts (no securities table join needed)
    ticker_filter_parts: List[str] = []
    ticker_params: List[Any] = []

    # Extract tickers from filters if provided
    tickers = None
    if filters and 'tickers' in filters:
        tickers = filters['tickers']
        if not isinstance(tickers, list):
            if isinstance(tickers, str):
                tickers = [tickers]
            else:
                tickers = None

    # Handle ticker-specific filtering directly
    if tickers is not None and len(tickers) > 0:
        placeholders = ','.join(['%s'] * len(tickers))
        ticker_filter_parts.append(f"o.ticker IN ({placeholders})")
        ticker_params.extend(tickers)

    # Note: Other security filters (sector, industry, etc.) are not available
    # in the OHLCV tables directly - they would need to be joined with securities table
    # For now, we only support ticker filtering

    # Build date filter
    date_filter_parts = []
    date_params = []

    if start_date:
        date_filter_parts.append("o.timestamp >= %s")
        date_params.append(_normalize_est(start_date))

    if end_date:
        date_filter_parts.append("o.timestamp <= %s")
        date_params.append(_normalize_est(end_date))

    # Add extended hours filtering for intraday timeframes
    extended_hours_filter = ""
    if base_table == "ohlcv_1m" and not extended_hours:
        # Filter to regular trading hours only (9:30 AM to 4:00 PM ET)
        extended_hours_filter = f"""(
            EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE {tz})) > 9 OR
            (EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE {tz})) = 9 AND
                EXTRACT(MINUTE FROM (o.timestamp AT TIME ZONE {tz})) >= 30)
        ) AND (
            EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE {tz})) < 16
        ) AND (
            EXTRACT(DOW FROM (o.timestamp AT TIME ZONE {tz})) BETWEEN 1 AND 5
        )"""
    # Combine all filters
    all_filter_parts = ticker_filter_parts + date_filter_parts
    if extended_hours_filter:
        all_filter_parts.append(extended_hours_filter)
    # Build WHERE clause - if no filters, use TRUE
    if all_filter_parts:
        where_clause = " AND ".join(all_filter_parts)
    else:
        where_clause = "TRUE"
    all_params = ticker_params + date_params

    # Build select columns based on what's actually requested
    select_parts = ["o.ticker", f"time_bucket(%s, o.timestamp AT TIME ZONE {tz}) AT TIME ZONE {tz} AS bucket_ts"]
    
    # Only include OHLCV columns that are requested
    if 'open' in columns:
        select_parts.append("first(o.open, o.timestamp) AS open")
    if 'high' in columns:
        select_parts.append("max(o.high) AS high")
    if 'low' in columns:
        select_parts.append("min(o.low) AS low")
    if 'close' in columns:
        select_parts.append("last(o.close, o.timestamp) AS close")
    if 'volume' in columns:
        select_parts.append("sum(o.volume) AS volume")
    
    select_clause = ', '.join(select_parts)

    # Build CTE SQL - no securities table join needed
    # Keep prices as raw bigint (will divide by 10000 in NumPy for better performance)
    cte_sql = f"""agg AS (
        SELECT {select_clause}
        FROM {base_table} o
        WHERE {where_clause}
        GROUP BY o.ticker, bucket_ts
    )"""  # nosec B608
    # Add bucket_sql as first parameter
    cte_params = [bucket_sql] + all_params
    return cte_sql, cte_params

def _normalize_est(dt: Optional[datetime]) -> Optional[datetime]:
    """Return a naive datetime, assuming input is already in Eastern time.

    If the input is naive, return it as-is since it's already in the correct timezone.
    If it has tzinfo, convert it to Eastern time and then make it naive.
    The DB expects naive datetimes that represent Eastern time.
    """
    if dt is None:
        return None
    if dt.tzinfo is None:
        # Already naive, assume it's in Eastern time
        logger.info("🕐 Using naive datetime as-is (assuming EST): %s", dt)
        return dt
    # Convert timezone-aware datetime to Eastern and make it naive
    eastern = ZoneInfo("America/New_York")
    result = dt.astimezone(eastern).replace(tzinfo=None)
    logger.info("🕐 Converted timezone-aware datetime %s to naive EST: %s", dt, result)
    return result

def get_available_filter_values(ctx: Context) -> Dict[str, List[str]]:
    """Get all available values for filter fields from the database"""
    #try:
    with ctx.conn.get_connection() as conn:
        cursor = conn.cursor()

        filter_values = {}

        # Get distinct sectors
        cursor.execute("""
            SELECT DISTINCT sector
            FROM securities
            WHERE maxdate IS NULL AND active = true AND sector IS NOT NULL
            ORDER BY sector
        """)
        filter_values['sectors'] = [row['sector'] for row in cursor.fetchall()]

        # Get distinct industries
        cursor.execute("""
            SELECT DISTINCT industry
            FROM securities
            WHERE maxdate IS NULL AND active = true AND industry IS NOT NULL
            ORDER BY industry
        """)
        filter_values['industries'] = [row['industry'] for row in cursor.fetchall()]

        # Get distinct primary exchanges
        cursor.execute("""
            SELECT DISTINCT primary_exchange
            FROM securities
            WHERE maxdate IS NULL AND active = true AND primary_exchange IS NOT NULL
            ORDER BY primary_exchange
        """)
        filter_values['primary_exchanges'] = [row['primary_exchange'] for row in cursor.fetchall()]

        cursor.close()
        required_keys = ['sectors', 'industries', 'primary_exchanges']
        for key in required_keys:
            if key not in filter_values or not filter_values[key]:
                raise ValueError(f"Database returned empty {key} list")
        return filter_values

#except Exception as exc:  # pylint: disable=broad-except
        #logger.error("Error fetching filter values: %s", exc)
        #return {
            #'sectors': [],
            #'industries': [],
            #'primary_exchanges': [],
        #}
