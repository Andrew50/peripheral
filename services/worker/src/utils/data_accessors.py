"""
Data Accessor Functions
Provides efficient data access functions for strategy execution.
These functions replace the previous approach of passing large DataFrames to strategies.
"""

import asyncio
import logging
import os
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Union
import numpy as np
import pandas as pd
from decimal import Decimal
import psycopg2
import threading
from contextlib import contextmanager
import time
from zoneinfo import ZoneInfo
try:
    from psycopg2.extras import RealDictCursor, NamedTupleCursor
except ImportError:
    psycopg2 = None

logger = logging.getLogger(__name__)
'''execution_context = {
            'mode': 'screening',  # 'backtest', 'screening', 'alert'
            'symbols': None,
            'start_date': None,
            'end_date': None
        }'''


     
    """def set_execution_context(ctx: Context, mode: str, symbols: List[str] = None, 
                start_date: datetime = None, end_date: datetime = None,
                min_bars_requirements: List[Dict] = None):
        #Set execution context for data fetching strategy
        self.execution_context = {
            'mode': mode,
            'symbols': symbols,
            'start_date': start_date,
            'end_date': end_date,
            'min_bars_requirements': min_bars_requirements or []
        }"""

    def get_bar_data(ctx: Context, timeframe: str = "1d", columns: List[str] = None, 
                     min_bars: int = 1, filters: Dict[str, any] = None, 
                     aggregate_mode: bool = False, extended_hours: bool = False,
                     start_date: Optional[datetime] = None, end_date: Optional[datetime] = None) -> np.ndarray:
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
            should_batch = _should_use_batching(tickers, aggregate_mode)
            print("should_batch", should_batch)
            
            if should_batch:
                #logger.info(f"🔄 Using batched data fetching for large dataset")
                return _get_bar_data_batched(timeframe, columns, min_bars, filters, extended_hours, start_date, end_date)
            else:
                # Use original method for smaller datasets or when aggregate_mode is True
                return _get_bar_data_single(timeframe, columns, min_bars, filters, extended_hours, start_date, end_date)
                
        except Exception as e:
            logger.error(f"Error in get_bar_data: {e}")
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
        """Get bar data using batching approach for large datasets"""
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
                universe_tickers = _get_all_active_tickers(filters)
                if not universe_tickers:
                    logger.warning("No active tickers found in universe")
                    return np.array([])
                #logger.info(f"📊 Found {len(universe_tickers)} active tickers in universe")
            else:
                universe_tickers = tickers
                #logger.info(f"📊 Processing {len(universe_tickers)} specified tickers")
            
            # Process in batches
            total_batches = (len(universe_tickers) + batch_size - 1) // batch_size
            #logger.info(f"🔄 Processing {total_batches} batches of up to {batch_size} tickers each")
            
            for i in range(0, len(universe_tickers), batch_size):
                batch_num = i // batch_size + 1
                batch_tickers = universe_tickers[i:i + batch_size]
                
                #logger.info(f"📦 Processing batch {batch_num}/{total_batches}: {len(batch_tickers)} tickers")
                
                try:
                    # Create batch filters with tickers
                    batch_filters = filters.copy() if filters else {}
                    batch_filters['tickers'] = batch_tickers
                    
                    # Get data for this batch using the single method
                    batch_result = _get_bar_data_single(
                        timeframe=timeframe,
                        columns=columns,
                        min_bars=min_bars,
                        filters=batch_filters,
                        extended_hours=extended_hours,
                        start_date=start_date,
                        end_date=end_date
                    )
                    
                    if batch_result is not None and len(batch_result) > 0:
                        all_results.append(batch_result)
                        logger.debug(f"✅ Batch {batch_num} returned {len(batch_result)} rows")
                    else:
                        logger.debug(f"⚠️ Batch {batch_num} returned no data")
                        
                except Exception as batch_error:
                    logger.error(f"❌ Error in batch {batch_num}: {batch_error}")
                    # Continue with next batch instead of failing completely
                    continue
            
            # Combine all batch results
            if all_results:
                combined_result = np.vstack(all_results)
                #logger.info(f"✅ Batching complete: {len(combined_result)} total rows from {len(all_results)} batches")
                return combined_result
            else:
                logger.warning("No data returned from any batch")
                return np.array([])
                
        except Exception as e:
            logger.error(f"Error in batched data fetching: {e}")
            return np.array([])
    
    def _get_all_active_tickers(self, filters: Dict[str, any] = None) -> List[str]:
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
    
    
    def _get_bar_data_single(self, timeframe: str = "1d", columns: List[str] = None, 
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
            allowed_columns = {"ticker", "timestamp", "open", "high", "low", "close", "volume", "transactions"}
            safe_columns = [col for col in columns if col in allowed_columns]
            
            if not safe_columns:
                return np.array([])
            
            # Determine date range - check direct parameters first, then execution context
            context = execution_context
            
            # Priority 1: Direct date parameters from function call
            if start_date and end_date:
                # Use direct datetime comparison with timezone-aware timestamps
                if start_date == end_date:
                    date_filter_start = start_date
                    date_filter_end = end_date
                else: 
                    date_filter_start = start_date
                    date_filter_end = end_date
                #logger.info(f"📅 Using direct date filter: {start_date} to {end_date}")
                use_backtest_sql_windowing = False
            elif start_date:
                # Only start date provided
                date_filter_start = start_date
                date_filter_end = None
                #logger.info(f"📅 Using direct start date filter: {start_date}")
                use_backtest_sql_windowing = False
            elif end_date:
                # Only end date provided
                date_filter_start = None
                date_filter_end = end_date
                #logger.info(f"📅 Using direct end date filter: {end_date}")
                use_backtest_sql_windowing = False
            # Priority 2: Execution context date range
            elif context.get('start_date') and context.get('end_date'):
                # For backtest mode: use SQL windowing to get exact bar counts
                if context.get('mode') == 'backtest':
                    use_backtest_sql_windowing = True
                    date_filter_start = None
                    date_filter_end = None
                    #logger.info(f"📅 Backtest mode: using SQL windowing for exact bar counts")
                else:
                    # Non-backtest with date range: get data from (start_date - min_bars buffer) to end_date
                    #timeframe_delta = self._get_timeframe_delta(timeframe)
                    date_filter_start = context.get('start_date') - (timeframe_delta * min_bars)
                    date_filter_end = context.get('end_date')
                    #logger.info(f"📅 Using execution context date filter: {date_filter_start} to {date_filter_end}")
                    use_backtest_sql_windowing = False
            elif context.get('mode') == 'validation':
                # Validation mode: Use recent data window
                date_filter_start = None
                date_filter_end = None
                use_backtest_sql_windowing = False
                #logger.info(f"🧪 Validation mode: using recent data window")
            elif context.get('mode') in ['screening', 'alert']:
                # Screening and alert modes: NO date filtering - let ROW_NUMBER() get exact amount
                date_filter_start = None
                date_filter_end = None
                use_backtest_sql_windowing = False
                #logger.info(f"📅 {context.get('mode').title()} mode: no date filtering (using ROW_NUMBER optimization)")
            else:
                # No specific date range: get ALL available data
                date_filter_start = None
                date_filter_end = None
                use_backtest_sql_windowing = False
                #logger.info("📅 No date filtering: retrieving all available data")
            
            # Build the query based on aggregation needs and execution mode
            if use_backtest_sql_windowing:
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
                [_normalize_est(context.get('start_date')), 
                _normalize_est(context.get('start_date')), 
                _normalize_est(context.get('end_date')), 
                pre_bars_needed])
            
            elif needs_aggregation:
                # Use TimescaleDB aggregation
                agg_cte_sql, agg_params = _build_agg_cte(
                    bucket_sql, base_table, columns, filters, extended_hours, 
                    date_filter_start, date_filter_end
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
                
                if context.get('mode') in ['screening', 'alert']:
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
                
                if date_filter_start:
                    date_filter_parts.append("o.timestamp >= %s")
                    date_params.append(_normalize_est(date_filter_start))
                
                if date_filter_end:
                    date_filter_parts.append("o.timestamp <= %s")
                    date_params.append(_normalize_est(date_filter_end))
                
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
                
                if context.get('mode') in ['screening', 'alert']:
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
            with ctx.conn.transaction() as cursor:
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
            logger.error(f"Error in get_bar_data: {e}")
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
            logger.error(f"Error converting tickers to security IDs: {e}")
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



    def _parse_timeframe(self, timeframe: str) -> tuple[str, str]:
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
        import re
        
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

    def _build_agg_cte(ctx: Context, bucket_sql: str, base_table: str, columns: List[str], 
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

def _normalize_est(self, dt: datetime):
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


# Global instance for strategy execution
'''_data_accessor = None

def get_data_accessor(conn) -> DataAccessorProvider:
    """Get global data accessor instance"""
    global _data_accessor
    if _data_accessor is None:
        if conn is None:
            raise ValueError("conn must be provided to initialize the data accessor")
        _data_accessor = DataAccessorProvider(conn=conn)
    return _data_accessor'''
