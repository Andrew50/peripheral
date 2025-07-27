"""
Data Accessor Functions
Provides efficient data access functions for strategy execution.
These functions replace the previous approach of passing large DataFrames to strategies.
"""

import re
import logging
import threading
from decimal import Decimal
from datetime import datetime
from typing import Dict, List, Optional
from concurrent.futures import ThreadPoolExecutor, as_completed
import numpy as np
import pandas as pd
import psycopg2
from zoneinfo import ZoneInfo
from psycopg2.extras import RealDictCursor, NamedTupleCursor

from context import Context

logger = logging.getLogger(__name__)

def _get_bar_data(ctx: Context, start_date: datetime, end_date: datetime, exectuion_mode: str, timeframe: str = "1d", columns: List[str] = None, 
                    min_bars: int = 1, filters: Dict[str, any] = None, 
                    extended_hours: bool = False) -> pd.DataFrame:
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
    try:
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
        # Check if we need to use batching (now with potentially corrected tickers)
        should_batch = _should_use_batching(ctx, tickers, exectuion_mode)
        
        if should_batch:
            return _get_bar_data_batched(ctx, timeframe, columns, min_bars, filters, extended_hours, start_date, end_date)
        else:
            return _get_bar_data_single(ctx, timeframe, columns, min_bars, filters, extended_hours, start_date, end_date)
            
    except Exception as e:
        logger.error("Error in get_bar_data: %s", e)
        return np.array([])

def _should_use_batching(ctx: Context, tickers: List[str] = None, aggregate_mode: bool = False) -> bool:
    """Determine if batching should be used based on the request parameters"""
    # Never batch if aggregate_mode is explicitly enabled
    if aggregate_mode:
        #logger.info("🔍 Aggregate mode enabled - disabling batching to provide all data at once")
        return False
    # Always batch when tickers=None (all securities)
    if tickers is None:
        #logger.info("🔄 Batching enabled: fetching all securities")
        return True
    elif not tickers:  # Empty list case
        #logger.info("🔄 Batching enabled: empty tickers list")
        return True

    # Batch when ticker list is large
    if len(tickers) > 1000:
        #logger.info(f"🔄 Batching enabled: {len(tickers)} tickers > 1000 limit")
        return True

    return False

def _get_bar_data_batched(ctx: Context, timeframe: str = "1d", columns: List[str] = None, 
            min_bars: int = 1, filters: Dict[str, any] = None, extended_hours: bool = False,
            start_date: Optional[datetime] = None, end_date: Optional[datetime] = None) -> np.ndarray:
    """Get bar data using batching approach for large datasets with parallel processing"""
    try:
        batch_size = 1000
        all_results = []

        # Extract tickers from filters
        tickers = None
        if filters and 'tickers' in filters:
            tickers = filters['tickers']
            if not isinstance(tickers, list):
                if isinstance(tickers, str):
                    tickers = [tickers]
                else:
                    tickers = None

        # Get the universe of tickers to process
        if tickers is None:
            # Get all active tickers
            #logger.info("🌍 Fetching universe of all active tickers for batching")
            universe_tickers = _get_all_active_tickers(ctx, filters)
            if not universe_tickers:
                logger.warning("No active tickers found in universe")
                return np.array([])
            #logger.info(f"📊 Found {len(universe_tickers)} active tickers in universe")
        else:
            universe_tickers = tickers
            #logger.info(f"📊 Processing {len(universe_tickers)} specified tickers")

        # Create batches
        ticker_batches = []
        for i in range(0, len(universe_tickers), batch_size):
            batch_tickers = universe_tickers[i:i + batch_size]
            ticker_batches.append(batch_tickers)

        # Helper function to process a single batch
        def process_batch(batch_tickers, batch_num):
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
                    logger.debug("✅ Batch %s returned %s rows", batch_num, len(batch_result))
                    return batch_result
                else:
                    logger.debug("⚠️ Batch %s returned no data", batch_num)
                    return None
            except ValueError as batch_error:
                logger.error("❌ Error in batch %s: %s", batch_num, batch_error)
                return None

        # Process batches in parallel using ThreadPoolExecutor
        logger.info(f"🚀 Processing {len(ticker_batches)} batches in parallel")
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
                batch_num = future_to_batch[future]
                try:
                    result = future.result()
                    if result is not None:
                        all_results.append(result)
                except Exception as exc:
                    logger.error(f"❌ Batch {batch_num} generated an exception: {exc}")

        # Combine all batch results
        if all_results:
            combined_result = np.vstack(all_results)
            logger.info(f"✅ Successfully combined {len(all_results)} batches into {len(combined_result)} total rows")
            return combined_result
        else:
            logger.warning("No data returned from any batch")
            return np.array([])

    except Exception as e:
        logger.error(f"Error in batched data fetching: {e}")
        return np.array([])

def _get_all_active_tickers(ctx: Context, filters: Dict[str, any] = None) -> List[str]:
    """Get list of all active tickers with optional filtering"""
    try:
        # Build filter conditions for active securities
        filter_parts = ["maxdate IS NULL", "active = true"]
        params = []
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
        with ctx.conn.transaction() as conn:
            cursor = conn.cursor()
            cursor.execute(query, params)
            results = cursor.fetchall()
            cursor.close()
        return [row[0] for row in results if row[0]]  # Filter out None tickers
    except Exception as e:
        logger.error(f"Error fetching active tickers: {e}")
        return []


def _build_query(ctx: Context, timeframe: str, columns: List[str], min_bars: int,
                filters: Dict[str, any] = None, extended_hours: bool = False,
                start_date: Optional[datetime] = None, end_date: Optional[datetime] = None) -> tuple[str, List, List[str]]:
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
    else:
        return _build_direct_query(base_table, safe_columns, min_bars, filters, 
                                 extended_hours, start_date, end_date, realtime_mode)


def _build_aggregated_query(bucket_sql: str, base_table: str, columns: List[str], min_bars: int,
                          filters: Dict[str, any] = None, extended_hours: bool = False,
                          start_date: Optional[datetime] = None, end_date: Optional[datetime] = None,
                          realtime_mode: bool = True) -> tuple[str, List, List[str]]:
    """Build query for aggregated timeframes using TimescaleDB time_bucket."""

    # Build the aggregation CTE
    agg_cte_sql, agg_params = _build_agg_cte(bucket_sql, base_table, columns, filters, 
                                           extended_hours, start_date, end_date)

    # Build final column selection
    final_columns = []
    for col in columns:
        if col == "ticker":
            final_columns.append("ticker")
        elif col == "timestamp":
            final_columns.append("EXTRACT(EPOCH FROM bucket_ts)::bigint AS timestamp")
        else:
            final_columns.append(col)
    
    final_select_clause = ', '.join(final_columns)
    
    if realtime_mode:
        # Realtime mode: get latest min_bars per ticker
        query = f"""WITH {agg_cte_sql},
            ranked_data AS (
                SELECT *,
                    ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY bucket_ts DESC) as rn,
                    COUNT(*) OVER (PARTITION BY ticker) as total_bars
                FROM agg
            )
            SELECT {final_select_clause}
            FROM ranked_data 
            WHERE rn <= %s AND total_bars >= %s
            ORDER BY ticker, bucket_ts DESC"""
        
        params = agg_params + [min_bars, min_bars]
    else:
        # Date-range mode: get data in range plus min_bars-1 pre-roll
        pre_bars_needed = max(min_bars - 1, 0)
        
        query = f"""WITH {agg_cte_sql},
            pre_bars AS (
                SELECT *, ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY bucket_ts DESC) as rn_pre
                FROM agg
                WHERE bucket_ts < %s
            ),
            in_range AS (
                SELECT * FROM agg
                WHERE bucket_ts >= %s AND bucket_ts <= %s
            )
            SELECT {final_select_clause}
            FROM (
                SELECT ticker, bucket_ts, open, high, low, close, volume 
                FROM pre_bars WHERE rn_pre <= %s
                UNION ALL
                SELECT ticker, bucket_ts, open, high, low, close, volume 
                FROM in_range
            ) AS combined
            ORDER BY ticker, bucket_ts ASC"""
        
        params = (agg_params + 
                 [_normalize_est(start_date), _normalize_est(start_date), 
                  _normalize_est(end_date), pre_bars_needed])
    
    return query, params, columns


def _build_direct_query(base_table: str, columns: List[str], min_bars: int,
                       filters: Dict[str, any] = None, extended_hours: bool = False,
                       start_date: Optional[datetime] = None, end_date: Optional[datetime] = None,
                       realtime_mode: bool = True) -> tuple[str, List, List[str]]:
    """Build query for direct table access (1m or 1d tables)."""
    
    # Build column selection with proper scaling for prices
    select_columns = []
    for col in columns:
        if col == "ticker":
            select_columns.append("o.ticker")
        elif col == "timestamp":
            select_columns.append("EXTRACT(EPOCH FROM o.timestamp)::bigint AS timestamp")
        elif col == "volume":
            select_columns.append("o.volume AS volume")
        elif col in ["open", "high", "low", "close"]:
            # Divide OHLC values by 1000 at database level (stored as bigint * 1000)
            select_columns.append(f"o.{col} / 1000.0 AS {col}")
        else:
            select_columns.append(f"o.{col}")
    
    # Build filters
    filter_parts, params = _build_ticker_and_date_filters(filters, start_date, end_date)
    
    # Add extended hours filtering for minute data
    if base_table == "ohlcv_1m" and not extended_hours:
        extended_hours_filter = """(
            EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) > 9 OR
            (EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) = 9 AND 
                EXTRACT(MINUTE FROM (o.timestamp AT TIME ZONE 'America/New_York')) >= 30)
        ) AND (
            EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) < 16
        ) AND (
            EXTRACT(DOW FROM (o.timestamp AT TIME ZONE 'America/New_York')) BETWEEN 1 AND 5
        )"""
        filter_parts.append(extended_hours_filter)
    
    # Build WHERE clause
    where_clause = " AND ".join(filter_parts) if filter_parts else "TRUE"
    select_clause = ', '.join(select_columns)
    
    if realtime_mode:
        # Realtime mode: get latest min_bars per ticker
        query = f"""WITH ranked_data AS (
            SELECT {select_clause},
                ROW_NUMBER() OVER (PARTITION BY o.ticker ORDER BY o.timestamp DESC) as rn,
                COUNT(*) OVER (PARTITION BY o.ticker) as total_bars
            FROM {base_table} o
            WHERE {where_clause}
        )
        SELECT {', '.join([col.split(' as ')[-1].split('.')[-1] for col in columns])}
        FROM ranked_data 
        WHERE rn <= %s AND total_bars >= %s
        ORDER BY ticker, timestamp DESC"""
        
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
        SELECT {', '.join([col.split(' as ')[-1].split('.')[-1] for col in columns])}
        FROM (
            SELECT * FROM pre_bars WHERE rn_pre <= %s
            UNION ALL
            SELECT * FROM in_range
        ) AS combined
        ORDER BY ticker, timestamp ASC"""
        
        params = (params + params + params +  # Three copies of base filters
                 [_normalize_est(start_date), _normalize_est(start_date), 
                  _normalize_est(end_date), pre_bars_needed])
    
    return query, params, columns


def _build_ticker_and_date_filters(filters: Dict[str, any] = None, 
                                  start_date: Optional[datetime] = None, 
                                  end_date: Optional[datetime] = None) -> tuple[List[str], List]:
    """Build common ticker and date filter parts for direct queries."""
    filter_parts = []
    params = []
    
    # Extract and handle ticker filters
    if filters and 'tickers' in filters:
        tickers = filters['tickers']
        if not isinstance(tickers, list):
            if isinstance(tickers, str):
                tickers = [tickers]
            else:
                tickers = None
        
        if tickers is not None and len(tickers) > 0:
            placeholders = ','.join(['%s'] * len(tickers))
            filter_parts.append(f"o.ticker IN ({placeholders})")
            params.extend(tickers)
    
    # Add date filters (only used in aggregated queries, but kept for consistency)
    if start_date:
        filter_parts.append("o.timestamp >= %s")
        params.append(_normalize_est(start_date))
    
    if end_date:
        filter_parts.append("o.timestamp <= %s")
        params.append(_normalize_est(end_date))
    
    return filter_parts, params


def _get_bar_data_single(ctx: Context, timeframe: str = "1d", columns: List[str] = None, 
            min_bars: int = 1, filters: Dict[str, any] = None, extended_hours: bool = False,
            start_date: Optional[datetime] = None, end_date: Optional[datetime] = None) -> np.ndarray:
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
    realtime_mode = False
    if start_date is None and end_date is None:
        realtime_mode = True
    if start_date is not None and end_date is None or start_date is None and end_date is not None:
        raise ValueError("start_date and end_date must be provided together")

    try:
        # Parse timeframe to get bucket SQL and base table
        bucket_sql, base_table = _parse_timeframe(timeframe)

        # Check if this is a direct table access (no aggregation needed)
        # For now, we only support direct access to 1m and 1d tables
        # All other timeframes will use aggregation
        needs_aggregation = not (
            (bucket_sql == "1 minute" and base_table == "ohlcv_1m") or
            (bucket_sql == "1 day" and base_table == "ohlcv_1d")
        )

        # Default columns if not specified - include ticker by default
        if not columns:
            columns = ["ticker", "timestamp", "open", "high", "low", "close", "volume"]

        # Validate columns against allowed set (removed adj_close and securityid - not in table)
        allowed_columns = {"ticker", "timestamp", "open", "high", 
        "low", "close", "volume", "transactions"}
        safe_columns = [col for col in columns if col in allowed_columns]

        if not safe_columns:
            return np.array([])
        
        # Determine date range - check direct parameters first, then execution context
        
        # Priority 1: Direct date parameters from function call
        
        # Build the query based on aggregation needs and execution mode
        if not realtime_mode:
            # Special backtest mode: use SQL windowing with aggregated data
            agg_cte_sql, agg_params = _build_agg_cte(
                bucket_sql, base_table, columns, filters, extended_hours
            )
            
            # Calculate how many pre-bars we need (min_bars - 1, but at least 0)
            pre_bars_needed = max(min_bars - 1, 0)
            
            # Build backtest windowing query with pre-bars and in-range data
            query = f"""WITH {agg_cte_sql},
                pre_bars AS (
                    SELECT *, ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY bucket_ts DESC) as rn_pre
                    FROM agg
                    WHERE bucket_ts < %s
                ),
                in_range AS (
                    SELECT * FROM agg
                    WHERE bucket_ts >= %s AND bucket_ts <= %s
                )
                SELECT 
                    ticker,
                    EXTRACT(EPOCH FROM bucket_ts)::bigint AS timestamp,
                    open, high, low, close, volume
                FROM (
                    SELECT * FROM pre_bars WHERE rn_pre <= %s
                    UNION ALL
                    SELECT * FROM in_range
                ) AS combined
                ORDER BY ticker, bucket_ts ASC"""
            
            # Parameters: agg_params + [start_date, start_date, end_date, pre_bars_needed]
            params = (agg_params + 
            [_normalize_est(start_date), 
            _normalize_est(start_date), 
            _normalize_est(end_date), 
            pre_bars_needed])
        
        elif needs_aggregation:
            # Use TimescaleDB aggregation
            agg_cte_sql, agg_params = _build_agg_cte(
                bucket_sql, base_table, columns, filters, extended_hours, 
                start_date, end_date
            )
            
            # Build final columns for select
            final_columns = []
            for col in safe_columns:
                if col == "ticker":
                    final_columns.append("ticker")
                elif col == "timestamp":
                    final_columns.append("EXTRACT(EPOCH FROM bucket_ts)::bigint AS timestamp")
                else:
                    final_columns.append(col)
            
            final_select_clause = ', '.join(final_columns)
            
            if realtime_mode:
                    # Use ROW_NUMBER to limit results per ticker
                query = f"""WITH {agg_cte_sql},
                ranked_data AS (
        SELECT *,
        ROW_NUMBER() OVER (PARTITION BY ticker ORDER BY bucket_ts DESC) as rn,
        COUNT(*) OVER (PARTITION BY ticker) as total_bars
        FROM agg
                )
                SELECT {final_select_clause}
                FROM ranked_data 
                WHERE rn <= %s AND total_bars >= %s
                ORDER BY ticker, bucket_ts DESC"""
                    
                params = agg_params + [min_bars, min_bars]
            else:
                    # Regular aggregated query
                query = f"""WITH {agg_cte_sql}
                    SELECT {final_select_clause}
                    FROM agg
                    ORDER BY ticker, bucket_ts ASC"""
                    
                params = agg_params
        
        else:
            # Direct table access (no aggregation) - use existing logic with modifications
            # Build column selection
            select_columns = []
            for col in safe_columns:
                if col == "ticker":
                    select_columns.append("o.ticker")
                elif col == "timestamp":
                    # Convert timestamptz to integer seconds since epoch for backward compatibility
                    select_columns.append("EXTRACT(EPOCH FROM o.timestamp)::bigint AS timestamp")
                elif col == "volume":
                    # Preserve raw volume (no scaling needed)
                    select_columns.append("o.volume AS volume")
                elif col in ["open", "high", "low", "close"]:
                    # Divide OHLC values by 1000 at database level (stored as bigint * 1000)
                    select_columns.append(f"o.{col} / 1000.0 AS {col}")
                else:
                    select_columns.append(f"o.{col}")
            
            # Build filters - simplified since no securities table join
            ticker_filter_parts = []
            ticker_params = []
            
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
            
                # Note: Other security filters (sector, industry, market_cap, etc.) are not available 
                # in the OHLCV tables directly - they would need to be joined with securities table
                # For now, we only support ticker filtering in direct table access mode
            
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
                extended_hours_filter = """(
                    EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) > 9 OR
                    (EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) = 9 AND 
            EXTRACT(MINUTE FROM (o.timestamp AT TIME ZONE 'America/New_York')) >= 30)
                ) AND (
                    EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) < 16
                ) AND (
                    EXTRACT(DOW FROM (o.timestamp AT TIME ZONE 'America/New_York')) BETWEEN 1 AND 5
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
            
            # Build query based on execution mode
            select_clause = ', '.join(select_columns)
            from_clause = f"{base_table} o"  # No securities table join needed
            
            if realtime_mode:
                # Use ROW_NUMBER to limit results per ticker
                query = f"""WITH ranked_data AS (
            SELECT {select_clause},
            ROW_NUMBER() OVER (PARTITION BY o.ticker ORDER BY o.timestamp DESC) as rn,
            COUNT(*) OVER (PARTITION BY o.ticker) as total_bars
            FROM {from_clause}
            WHERE {where_clause}
                    )
                    SELECT {', '.join([col.split(' as ')[-1].split('.')[-1] for col in safe_columns])}
                    FROM ranked_data 
                    WHERE rn <= %s AND total_bars >= %s
                    ORDER BY ticker, timestamp DESC"""
                    
                params = all_params + [min_bars, min_bars]
            else:
                    # Regular direct query
                query = f"SELECT {select_clause} FROM {from_clause} WHERE {where_clause} ORDER BY o.ticker, o.timestamp ASC"
                params = all_params
        
        # Execute query with faster cursor
        with ctx.conn.get_connection() as conn:
            cursor = conn.cursor(cursor_factory=NamedTupleCursor)
            cursor.execute(query, params)
            results = cursor.fetchall()
            cursor.close()
        if not results:
            return np.array([])
        # Convert to numpy array more efficiently
        if not results:
            return np.empty((0, len(safe_columns)), dtype=object)
        # Use numpy.fromiter for better performance
        data = np.fromiter(
            (tuple(getattr(record, col.split(' as ')[-1].split('.')[-1]) for col in safe_columns) for record in results),
            dtype=object,
            count=len(results)
        )
        data = data.reshape(len(results), len(safe_columns))
        # Convert Decimal to float to avoid type mismatch issues in strategy calculations
        for i in range(data.shape[0]):
            for j in range(data.shape[1]):
                if isinstance(data[i, j], Decimal):
                    data[i, j] = float(data[i, j])
        return data
    except Exception as e:
        logger.error("Error in get_bar_data: %s", e)
        return np.array([])


def _get_security_ids_from_tickers(ctx: Context, tickers: List[str], filters: Dict[str, any] = None) -> List[int]:
    """Convert ticker symbols to security IDs with optional filtering"""
    try:
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
        
        return [row[0] for row in results]
        
    except Exception as e:
        logger.error("Error converting tickers to security IDs: %s", e)
        return []

def get_general_data(ctx: Context, columns: List[str] = None, 
            filters: Dict[str, any] = None) -> pd.DataFrame:
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
    try:
        # Extract tickers from filters if provided
        tickers = None
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
        params = []
        
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
            #logger.info(f"Converting ticker symbols {tickers} to security IDs for general data")
            security_ids = _get_security_ids_from_tickers(tickers, filters)
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
        
    except Exception as e:
        logger.error(f"Error in get_general_data: {e}")
        return pd.DataFrame()



def _parse_timeframe(timeframe: str) -> tuple[str, str]:
    """Parse timeframe string and return (bucket_sql, base_table)
    
    Args:
        timeframe: Timeframe string like '1m', '5m', '2h', '1d', '3w', '1mo'
        
    Returns:
        Tuple of (bucket_sql, base_table) where:
        - bucket_sql: TimescaleDB interval string like '5 minutes', '2 hours', '1 day'
        - base_table: Database table to use ('ohlcv_1m' or 'ohlcv_1d')
        
    Raises:
        ValueError: If timeframe format is invalid or unsupported
    """
    
    # Validate timeframe format
    pattern = r'^(\d+)(m|h|d|w|mo)$'
    match = re.match(pattern, timeframe.lower())
    if not match:
        raise ValueError(f"Invalid timeframe format: {timeframe}. Expected format like '1m', '5m', '2h', '1d', '3w', '1mo'")
    
    value, unit = match.groups()
    value = int(value)
    
    # Map units to TimescaleDB interval strings and determine base table
    if unit == 'm':
        # Minutes - use ohlcv_1m as base
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
    elif unit == 'mo':
        # Months - use ohlcv_1d as base
        bucket_sql = f"{value} month{'s' if value != 1 else ''}"
        base_table = "ohlcv_1d"
    else:
        raise ValueError(f"Unsupported timeframe unit: {unit}")
    
    return bucket_sql, base_table

def _build_agg_cte(bucket_sql: str, base_table: str, columns: List[str], 
                    filters: Dict[str, any] = None, extended_hours: bool = False,
                    start_date: Optional[datetime] = None, end_date: Optional[datetime] = None) -> tuple[str, List]:
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
    ticker_filter_parts = []
    ticker_params = []
    
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
        extended_hours_filter = """(
            EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) > 9 OR
            (EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) = 9 AND 
                EXTRACT(MINUTE FROM (o.timestamp AT TIME ZONE 'America/New_York')) >= 30)
        ) AND (
            EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) < 16
        ) AND (
            EXTRACT(DOW FROM (o.timestamp AT TIME ZONE 'America/New_York')) BETWEEN 1 AND 5
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
    # Build CTE SQL - no securities table join needed
    # Prices are stored as bigint * 1000, so divide by 1000.0 to get actual price
    cte_sql = f"""agg AS (
        SELECT
            o.ticker,
            time_bucket(%s, o.timestamp AT TIME ZONE 'America/New_York') AS bucket_ts,
            first(o.open / 1000.0, o.timestamp) AS open,
            max(o.high / 1000.0) AS high,
            min(o.low / 1000.0) AS low,
            last(o.close / 1000.0, o.timestamp) AS close,
            sum(o.volume) AS volume
        FROM {base_table} o
        WHERE {where_clause}
        GROUP BY o.ticker, bucket_ts
    )"""
    # Add bucket_sql as first parameter
    cte_params = [bucket_sql] + all_params
    return cte_sql, cte_params

def _normalize_est(dt: datetime):
    """Return a timezone-aware datetime in America/New_York.

    If the input is naive, assume it's already in Eastern time. If it has a
    different tzinfo, convert it. No conversion to UTC is performed because
    the DB stores market timestamps in EST/EDT.
    """
    if dt is None:
        return None
    eastern = ZoneInfo("America/New_York")
    if dt.tzinfo is None:
        return dt.replace(tzinfo=eastern)
    return dt.astimezone(eastern)
