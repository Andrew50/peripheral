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
    from psycopg2.extras import RealDictCursor
except ImportError:
    psycopg2 = None

logger = logging.getLogger(__name__)


class DataAccessorProvider:
    """Provides optimized data accessor functions for strategy execution"""
    
    def __init__(self):
        self.db_config = {
            'host': os.getenv('DB_HOST', 'localhost'),
            'port': os.getenv('DB_PORT', '5432'),
            'user': os.getenv('DB_USER', 'postgres'),
            'password': os.getenv('DB_PASSWORD', ''),
            'database': os.getenv('POSTGRES_DB', 'postgres'),
        }
        # Execution context for determining data range
        self.execution_context = {
            'mode': 'screening',  # 'backtest', 'screening', 'alert'
            'symbols': None,
            'start_date': None,
            'end_date': None
        }
        
    @contextmanager
    def get_connection(self):
        """Get database connection with retry logic and timeout"""
        connection = None
        max_retries = 3
        retry_delay = 1
        
        for attempt in range(max_retries):
            try:
                connection = psycopg2.connect(
                    host=self.db_config['host'],
                    port=self.db_config['port'],
                    user=self.db_config['user'],
                    password=self.db_config['password'],
                    database=self.db_config['database'],
                    connect_timeout=30,
                    application_name=f'worker_{os.getpid()}'
                )
                yield connection
                break
            except psycopg2.OperationalError as e:
                if "recovery mode" in str(e) and attempt < max_retries - 1:
                    logging.warning(f"Database in recovery mode, retrying in {retry_delay}s (attempt {attempt + 1}/{max_retries})")
                    time.sleep(retry_delay)
                    retry_delay *= 2  # Exponential backoff
                    continue
                else:
                    raise
            except Exception as e:
                logging.error(f"Database connection failed on attempt {attempt + 1}: {e}")
                if attempt < max_retries - 1:
                    time.sleep(retry_delay)
                    continue
                else:
                    raise
            finally:
                if connection:
                    connection.close()

    def set_execution_context(self, mode: str, symbols: List[str] = None, 
                             start_date: datetime = None, end_date: datetime = None,
                             min_bars_requirements: List[Dict] = None):
        """Set execution context for data fetching strategy"""
        self.execution_context = {
            'mode': mode,
            'symbols': symbols,
            'start_date': start_date,
            'end_date': end_date,
            'min_bars_requirements': min_bars_requirements or []
        }

    def get_bar_data(self, timeframe: str = "1d", columns: List[str] = None, 
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
            if min_bars < 1:
                min_bars = 1
            if min_bars > 10000:  # Prevent excessive data requests
                min_bars = 10000
            
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
            should_batch = self._should_use_batching(tickers, aggregate_mode)
            
            if should_batch:
                logger.info(f"🔄 Using batched data fetching for large dataset")
                return self._get_bar_data_batched(timeframe, columns, min_bars, filters, extended_hours, start_date, end_date)
            else:
                # Use original method for smaller datasets or when aggregate_mode is True
                return self._get_bar_data_single(timeframe, columns, min_bars, filters, extended_hours, start_date, end_date)
                
        except Exception as e:
            logger.error(f"Error in get_bar_data: {e}")
            return np.array([])
    
    def _should_use_batching(self, tickers: List[str] = None, aggregate_mode: bool = False) -> bool:
        """Determine if batching should be used based on the request parameters"""
        # Never batch if aggregate_mode is explicitly enabled
        if aggregate_mode:
            logger.info("🔍 Aggregate mode enabled - disabling batching to provide all data at once")
            return False
        
        # Always batch when tickers=None (all securities)
        if tickers is None:
            logger.info("🔄 Batching enabled: fetching all securities") 
            return True
        elif not tickers:  # Empty list case
            logger.info("🔄 Batching enabled: empty tickers list")
            return True
        
        # Batch when ticker list is large
        if len(tickers) > 1000:
            logger.info(f"🔄 Batching enabled: {len(tickers)} tickers > 1000 limit")
            return True
        
        return False
    
    def _get_bar_data_batched(self, timeframe: str = "1d", columns: List[str] = None, 
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
                logger.info("🌍 Fetching universe of all active tickers for batching")
                universe_tickers = self._get_all_active_tickers(filters)
                if not universe_tickers:
                    logger.warning("No active tickers found in universe")
                    return np.array([])
                logger.info(f"📊 Found {len(universe_tickers)} active tickers in universe")
            else:
                universe_tickers = tickers
                logger.info(f"📊 Processing {len(universe_tickers)} specified tickers")
            
            # Process in batches
            total_batches = (len(universe_tickers) + batch_size - 1) // batch_size
            logger.info(f"🔄 Processing {total_batches} batches of up to {batch_size} tickers each")
            
            for i in range(0, len(universe_tickers), batch_size):
                batch_num = i // batch_size + 1
                batch_tickers = universe_tickers[i:i + batch_size]
                
                logger.info(f"📦 Processing batch {batch_num}/{total_batches}: {len(batch_tickers)} tickers")
                
                try:
                    # Create batch filters with tickers
                    batch_filters = filters.copy() if filters else {}
                    batch_filters['tickers'] = batch_tickers
                    
                    # Get data for this batch using the single method
                    batch_result = self._get_bar_data_single(
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
                logger.info(f"✅ Batching complete: {len(combined_result)} total rows from {len(all_results)} batches")
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
            
            with self.get_connection() as conn:
                cursor = conn.cursor()
                cursor.execute(query, params)
                results = cursor.fetchall()
                cursor.close()
            
            return [row[0] for row in results if row[0]]  # Filter out None tickers
            
        except Exception as e:
            logger.error(f"Error fetching active tickers: {e}")
            return []
    
    def get_available_filter_values(self) -> Dict[str, List[str]]:
        """Get all available values for filter fields from the database"""
        try:
            with self.get_connection() as conn:
                cursor = conn.cursor()
                
                filter_values = {}
                
                # Get distinct sectors
                cursor.execute("""
                    SELECT DISTINCT sector 
                    FROM securities 
                    WHERE maxdate IS NULL AND active = true AND sector IS NOT NULL 
                    ORDER BY sector
                """)
                filter_values['sectors'] = [row[0] for row in cursor.fetchall()]
                
                # Get distinct industries
                cursor.execute("""
                    SELECT DISTINCT industry 
                    FROM securities 
                    WHERE maxdate IS NULL AND active = true AND industry IS NOT NULL 
                    ORDER BY industry
                """)
                filter_values['industries'] = [row[0] for row in cursor.fetchall()]
                
                # Get distinct primary exchanges
                cursor.execute("""
                    SELECT DISTINCT primary_exchange 
                    FROM securities 
                    WHERE maxdate IS NULL AND active = true AND primary_exchange IS NOT NULL 
                    ORDER BY primary_exchange
                """)
                filter_values['primary_exchanges'] = [row[0] for row in cursor.fetchall()]
                
                cursor.close()
                
                return filter_values
                
        except Exception as e:
            logger.error(f"Error fetching filter values: {e}")
            return {
                'sectors': [],
                'industries': [],
                'primary_exchanges': [],
            }
    
    def _get_bar_data_single(self, timeframe: str = "1d", columns: List[str] = None, 
                           min_bars: int = 1, filters: Dict[str, any] = None, extended_hours: bool = False,
                           start_date: Optional[datetime] = None, end_date: Optional[datetime] = None) -> np.ndarray:
        """
        Get OHLCV bar data as numpy array with context-aware date ranges
        
        Args:
            timeframe: Data timeframe ('1d', '1h', '5m', etc.')
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
            # Validate inputs
            if min_bars < 1:
                min_bars = 1
            if min_bars > 10000:  # Prevent excessive data requests
                min_bars = 10000
                
            # Extract tickers from filters if provided
            tickers = None
            if filters and 'tickers' in filters:
                tickers = filters['tickers']
                if not isinstance(tickers, list):
                    if isinstance(tickers, str):
                        tickers = [tickers]  # Convert single ticker to list
                    else:
                        tickers = None
                
            # Map timeframes to database tables and aggregation sources
            timeframe_tables = {
                # Direct table access (no aggregation needed)
                "1m": "ohlcv_1m",
                "1h": "ohlcv_1h", 
                "1d": "ohlcv_1d",
                "1w": "ohlcv_1w",
                # Custom aggregations (will be processed by aggregation engine)
                "5m": {"source": "ohlcv_1m", "aggregate_minutes": 5},
                "10m": {"source": "ohlcv_1m", "aggregate_minutes": 10},
                "15m": {"source": "ohlcv_1m", "aggregate_minutes": 15},
                "30m": {"source": "ohlcv_1m", "aggregate_minutes": 30},
                "2h": {"source": "ohlcv_1h", "aggregate_hours": 2},
                "4h": {"source": "ohlcv_1h", "aggregate_hours": 4},
                "6h": {"source": "ohlcv_1h", "aggregate_hours": 6},
                "8h": {"source": "ohlcv_1h", "aggregate_hours": 8},
                "12h": {"source": "ohlcv_1h", "aggregate_hours": 12},
                "2w": {"source": "ohlcv_1w", "aggregate_weeks": 2},
                "3w": {"source": "ohlcv_1w", "aggregate_weeks": 3},
                "4w": {"source": "ohlcv_1w", "aggregate_weeks": 4}
            }
            
            # Determine if we need aggregation or direct table access
            timeframe_config = timeframe_tables.get(timeframe, "ohlcv_1d")
            
            if isinstance(timeframe_config, dict):
                # Custom aggregation needed
                return self._get_aggregated_bar_data(
                    timeframe_config, columns, min_bars, filters, extended_hours, start_date, end_date
                )
            else:
                # Direct table access
                table_name = timeframe_config
            
            # Default columns if not specified - include ticker by default
            if not columns:
                columns = ["ticker", "timestamp", "open", "high", "low", "close", "volume"]
            
            # Validate columns against allowed set (removed adj_close)
            allowed_columns = {"securityid", "ticker", "timestamp", "open", "high", "low", "close", "volume"}
            safe_columns = [col for col in columns if col in allowed_columns]
            
            if not safe_columns:
                return np.array([])
            
            # Determine date range - check direct parameters first, then execution context
            context = self.execution_context
            
            # Priority 1: Direct date parameters from function call
            if start_date and end_date:
                # Use direct datetime comparison with timezone-aware timestamps
                if start_date == end_date:
                    date_filter = "o.timestamp = %s"
                else: 
                    date_filter = "o.timestamp >= %s AND o.timestamp <= %s"
                date_params = [self._normalize_est(start_date), self._normalize_est(end_date)]
                logger.info(f"📅 Using direct date filter: {start_date} to {end_date}")
            elif start_date:
                # Only start date provided
                date_filter = "o.timestamp >= %s"
                date_params = [self._normalize_est(start_date)]
                logger.info(f"📅 Using direct start date filter: {start_date}")
            elif end_date:
                # Only end date provided
                date_filter = "o.timestamp <= %s"
                date_params = [self._normalize_est(end_date)]
                logger.info(f"📅 Using direct end date filter: {end_date}")
            # Priority 2: Execution context date range
            elif context.get('start_date') and context.get('end_date'):
                # Specific date range provided: get data from (start_date - min_bars buffer) to end_date
                timeframe_delta = self._get_timeframe_delta(timeframe)
                start_with_buffer = context.get('start_date') - (timeframe_delta * min_bars)
                date_filter = "o.timestamp >= %s AND o.timestamp <= %s"
                date_params = [self._normalize_est(start_with_buffer), self._normalize_est(context.get('end_date'))]
                logger.info(f"📅 Using execution context date filter: {start_with_buffer} to {context.get('end_date')}")
            elif context.get('mode') == 'validation':
                # Validation mode: Use exact min_bars requirements for accurate validation
                # No arbitrary caps - respect the strategy's actual needs
                # nosec B608: Safe - table_name from controlled timeframe_tables dict, columns validated against allowlist, all dynamic params parameterized
                date_filter = "o.timestamp >= (SELECT MAX(timestamp) - interval '30 days' FROM {} WHERE ticker = o.ticker)".format(table_name)  # nosec B608
                date_params = []
                
                # Check if this specific min_bars matches any requirement from the strategy code
                min_bars_requirements = context.get('min_bars_requirements', [])
                matching_requirement = None
                for req in min_bars_requirements:
                    if req.get('timeframe') == timeframe and req.get('min_bars') == min_bars:
                        matching_requirement = req
                        break
                
                if matching_requirement:
                    logger.info(f"🧪 Validation mode: using exact min_bars={min_bars} for {timeframe} (from line {matching_requirement['line_number']})")
                else:
                    logger.info(f"🧪 Validation mode: using min_bars={min_bars} for {timeframe} (no arbitrary caps applied)")
                
                # No min_bars override - use the strategy's exact requirements
            elif context.get('mode') == 'screening':
                # Screening mode: NO date filtering - let ROW_NUMBER() get exact amount
                # This is much more efficient than date filtering because:
                # 1. Gets exactly min_bars per security (no more, no less)
                # 2. No unnecessary date calculations or buffers
                # 3. Database optimizer handles getting most recent records efficiently
                date_filter = "TRUE"  # No date restriction, rely on ROW_NUMBER LIMIT
                date_params = []
                logger.info("📅 Screening mode: no date filtering (using ROW_NUMBER optimization)")
            else:
                # No specific date range: get ALL available data
                date_filter = "TRUE"  # No date restriction
                date_params = []
                logger.info("📅 No date filtering: retrieving all available data")
            
            # Handle security filtering
            security_filter_parts = []
            security_params = []
            
            # Base filter for active securities
            security_filter_parts.append("s.maxdate IS NULL")
            
            # Apply additional filters if provided
            if filters:
                if 'sector' in filters:
                    security_filter_parts.append("s.sector = %s")
                    security_params.append(filters['sector'])
                
                if 'industry' in filters:
                    security_filter_parts.append("s.industry = %s")
                    security_params.append(filters['industry'])
                
                if 'primary_exchange' in filters:
                    security_filter_parts.append("s.primary_exchange = %s")
                    security_params.append(filters['primary_exchange'])
                
                if 'market_cap_min' in filters:
                    security_filter_parts.append("s.market_cap >= %s")
                    security_params.append(filters['market_cap_min'])
                
                if 'market_cap_max' in filters:
                    security_filter_parts.append("s.market_cap <= %s")
                    security_params.append(filters['market_cap_max'])
                if 'total_employees_min' in filters:
                    security_filter_parts.append("s.total_employees >= %s")
                    security_params.append(filters['total_employees_min'])
                
                if 'total_employees_max' in filters:
                    security_filter_parts.append("s.total_employees <= %s")
                    security_params.append(filters['total_employees_max'])
                
                if 'weighted_shares_outstanding_min' in filters:
                    security_filter_parts.append("s.weighted_shares_outstanding >= %s")
                    security_params.append(filters['weighted_shares_outstanding_min'])
                
                if 'weighted_shares_outstanding_max' in filters:
                    security_filter_parts.append("s.weighted_shares_outstanding <= %s")
                    security_params.append(filters['weighted_shares_outstanding_max'])
                
                if 'active' in filters:
                    security_filter_parts.append("s.active = %s")
                    security_params.append(filters['active'])
                else:
                    # Default to active if not explicitly specified
                    security_filter_parts.append("s.active = true")
            else:
                # Default to active if no filters provided
                security_filter_parts.append("s.active = true")
            
            # Handle ticker-specific filtering
            if tickers is not None and len(tickers) > 0:
                # Convert ticker symbols to security IDs and add to filter
                security_ids = self._get_security_ids_from_tickers(tickers, filters)
                if not security_ids:
                    logger.warning("No security IDs found for provided tickers")
                    return np.array([])
                
                # Use converted security IDs
                placeholders = ','.join(['%s'] * len(security_ids))
                security_filter_parts.append(f"s.securityid IN ({placeholders})")
                security_params.extend(security_ids)
            
            # Combine all filter parts
            security_filter = " AND ".join(security_filter_parts)
            
            # Add extended hours filtering for intraday timeframes
            extended_hours_filter = ""
            extended_hours_params = []
            
            # Only apply extended hours filtering for intraday timeframes (seconds, minutes, hours)
            # Daily and above timeframes ignore the extended_hours parameter
            intraday_timeframes = ["1m", "5m", "10m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h"]
            if timeframe in intraday_timeframes and not extended_hours:
                # Filter to regular trading hours only (9:30 AM to 4:00 PM ET)
                # Use PostgreSQL's EXTRACT function to get hour and minute in ET timezone
                extended_hours_filter = """AND (
                    EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) > 9 OR
                    (EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) = 9 AND 
                     EXTRACT(MINUTE FROM (o.timestamp AT TIME ZONE 'America/New_York')) >= 30)
                ) AND (
                    EXTRACT(HOUR FROM (o.timestamp AT TIME ZONE 'America/New_York')) < 16
                ) AND (
                    EXTRACT(DOW FROM (o.timestamp AT TIME ZONE 'America/New_York')) BETWEEN 1 AND 5
                )"""
            
            # Build column selection
            select_columns = []
            for col in safe_columns:
                if col == "securityid":
                    select_columns.append("s.securityid")
                elif col == "ticker":
                    select_columns.append("s.ticker")
                elif col == "timestamp":
                    # Convert timestamptz to integer seconds since epoch for backward compatibility
                    select_columns.append("EXTRACT(EPOCH FROM o.timestamp)::bigint AS timestamp")
                elif col == "volume":
                    # Preserve raw volume (do not scale by 1000)
                    select_columns.append("o.volume AS volume")
                elif col in ["open", "high", "low", "close"]:
                    # Divide OHLC values by 1000 at database level
                    select_columns.append(f"o.{col} / 1000.0 AS {col}")
                else:
                    select_columns.append(f"o.{col}")
            
            # Build the complete query
            if context.get('mode') == 'backtest' or (not context.get('start_date') and not context.get('end_date') and context.get('mode') != 'screening'):
                # For backtest mode or when no specific dates and not screening: get all data in range, don't limit per security
                # Build parameterized query components
                select_clause = ', '.join(select_columns)
                from_clause = f"{table_name} o JOIN securities s ON o.ticker = s.ticker"
                
                # Build WHERE clause - handle case where there might be no date filter
                if date_params:
                    where_clause = f"{security_filter} AND {date_filter}"
                else:
                    where_clause = security_filter
                
                # Add extended hours filter if applicable
                if extended_hours_filter:
                    where_clause += f" {extended_hours_filter}"
                    
                order_clause = "s.securityid, o.timestamp ASC"
                
                # nosec B608: Safe - table_name from controlled timeframe_tables dict, columns validated against allowlist, all dynamic params parameterized
                query = f"SELECT {select_clause} FROM {from_clause} WHERE {where_clause} ORDER BY {order_clause}"  # nosec B608
                params = security_params + date_params
            else:
                # For screening/alerts: limit to min_bars per security, most recent first
                # Build column list for the final SELECT from ranked_data
                final_columns = []
                for col in safe_columns:
                    if col == "securityid":
                        final_columns.append("securityid")
                    elif col == "ticker":
                        final_columns.append("ticker")
                    elif col == "timestamp":
                        final_columns.append("timestamp")
                    else:
                        final_columns.append(col)
                
                # Build parameterized CTE query components
                select_clause = ', '.join(select_columns)
                from_clause = f"{table_name} o JOIN securities s ON o.ticker = s.ticker"
                
                # Build WHERE clause - handle case where there might be no date filter
                if date_params:
                    where_clause = f"{security_filter} AND {date_filter}"
                else:
                    where_clause = security_filter
                
                # Add extended hours filter if applicable
                if extended_hours_filter:
                    where_clause += f" {extended_hours_filter}"
                    
                final_select_clause = ', '.join(final_columns)
                
                # Optimize ordering for screening mode - prioritize most recent data
                if context.get('mode') == 'screening':
                    # For screening, order by most recent timestamp first within each security
                    # This ensures we get the absolute latest data for screening decisions
                    order_by_columns = []
                    if "securityid" in final_columns:
                        order_by_columns.append("securityid")
                    if "timestamp" in final_columns:
                        order_by_columns.append("timestamp DESC")  # Most recent first for screening
                    
                    # Default ordering if no order columns available
                    if not order_by_columns:
                        order_by_clause = "1"  # Order by first column
                    else:
                        order_by_clause = ", ".join(order_by_columns)
                        
                    # For screening mode with small min_bars, add additional optimization
                    if min_bars <= 10:
                        # Use optimized query that focuses on index efficiency for recent data
                        # CRITICAL: Only return tickers that have at least min_bars of data
                        # nosec B608: Safe - table_name from controlled timeframe_tables dict, columns validated against allowlist, all dynamic params parameterized
                        query = f"""WITH ranked_data AS (
                            SELECT {select_clause},
                                   ROW_NUMBER() OVER (PARTITION BY s.securityid ORDER BY o.timestamp DESC) as rn,
                                   COUNT(*) OVER (PARTITION BY s.securityid) as total_bars
                            FROM {from_clause}
                            WHERE {where_clause}
                        )
                        SELECT {final_select_clause}
                        FROM ranked_data 
                        WHERE rn <= %s AND total_bars >= %s
                        ORDER BY {order_by_clause}"""  # nosec B608
                    else:
                        # For larger min_bars, use standard approach but still prioritize recent data
                        # CRITICAL: Only return tickers that have at least min_bars of data
                        # nosec B608: Safe - table_name from controlled timeframe_tables dict, columns validated against allowlist, all dynamic params parameterized
                        query = f"""WITH ranked_data AS (
                            SELECT {select_clause},
                                   ROW_NUMBER() OVER (PARTITION BY s.securityid ORDER BY o.timestamp DESC) as rn,
                                   COUNT(*) OVER (PARTITION BY s.securityid) as total_bars
                            FROM {from_clause}
                            WHERE {where_clause}
                        )
                        SELECT {final_select_clause}
                        FROM ranked_data 
                        WHERE rn <= %s AND total_bars >= %s
                        ORDER BY {order_by_clause}"""  # nosec B608
                    
                    params = security_params + date_params + [min_bars, min_bars]
                else:
                    # Non-screening mode: use existing logic
                    order_by_columns = []
                    if "securityid" in final_columns:
                        order_by_columns.append("securityid")
                    if "timestamp" in final_columns:
                        order_by_columns.append("timestamp ASC")
                    
                    # Default ordering if no order columns available
                    if not order_by_columns:
                        order_by_clause = "1"  # Order by first column
                    else:
                        order_by_clause = ", ".join(order_by_columns)
                    
                    # CRITICAL: Only return tickers that have at least min_bars of data
                    # nosec B608: Safe - table_name from controlled timeframe_tables dict, columns validated against allowlist, all dynamic params parameterized
                    query = f"""WITH ranked_data AS (
                        SELECT {select_clause},
                               ROW_NUMBER() OVER (PARTITION BY s.securityid ORDER BY o.timestamp DESC) as rn,
                               COUNT(*) OVER (PARTITION BY s.securityid) as total_bars
                        FROM {from_clause}
                        WHERE {where_clause}
                    )
                    SELECT {final_select_clause}
                    FROM ranked_data 
                    WHERE rn <= %s AND total_bars >= %s
                    ORDER BY {order_by_clause}"""  # nosec B608
                
                params = security_params + date_params + [min_bars, min_bars]
            
            with self.get_connection() as conn:
                cursor = conn.cursor(cursor_factory=RealDictCursor)
                
                cursor.execute(query, params)
                results = cursor.fetchall()
                
                cursor.close()
            
            if not results:
                return np.array([])
            
            # Convert to numpy array with consistent column order
            ordered_results = []
            for row in results:
                ordered_row = []
                for col in safe_columns:
                    # Handle aliased columns properly
                    col_key = col.split(' as ')[-1].split('.')[-1]
                    value = row[col_key]
                    
                    # Convert Decimal to float to avoid type mismatch issues in strategy calculations
                    if isinstance(value, Decimal):
                        value = float(value)
                    
                    ordered_row.append(value)
                ordered_results.append(ordered_row)
            
            return np.array(ordered_results, dtype=object)
            
        except Exception as e:
            logger.error(f"Error in get_bar_data: {e}")
            return np.array([])
    
    def _get_timeframe_delta(self, timeframe: str) -> timedelta:
        """Convert timeframe string to timedelta"""
        timeframe_map = {
            "1m": timedelta(minutes=1),
            "5m": timedelta(minutes=5),
            "15m": timedelta(minutes=15),
            "30m": timedelta(minutes=30),
            "1h": timedelta(hours=1),
            "1d": timedelta(days=1),
            "1w": timedelta(weeks=1)
        }
        return timeframe_map.get(timeframe, timedelta(days=1))
    
    def _get_security_ids_from_tickers(self, tickers: List[str], filters: Dict[str, any] = None) -> List[int]:
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
            
            with self.get_connection() as conn:
                cursor = conn.cursor()
                cursor.execute(query, params)
                results = cursor.fetchall()
                cursor.close()
            
            return [row[0] for row in results]
            
        except Exception as e:
            logger.error(f"Error converting tickers to security IDs: {e}")
            return []

    def get_general_data(self, columns: List[str] = None, 
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
                logger.info(f"Converting ticker symbols {tickers} to security IDs for general data")
                security_ids = self._get_security_ids_from_tickers(tickers, filters)
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
            
            with self.get_connection() as conn:
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

    def _get_aggregated_bar_data(self, timeframe_config: Dict[str, any], columns: List[str] = None, 
                                min_bars: int = 1, filters: Dict[str, any] = None, extended_hours: bool = False,
                                start_date: Optional[datetime] = None, end_date: Optional[datetime] = None) -> np.ndarray:
        """
        Get aggregated OHLCV data by combining base timeframe data into custom intervals
        
        Args:
            timeframe_config: Dict with source table and aggregation parameters
            columns: Desired columns
            min_bars: Minimum bars needed
            filters: Filtering criteria (including tickers)
            
        Returns:
            numpy.ndarray with aggregated OHLCV data
        """
        try:
            source_table = timeframe_config["source"]
            
            # Determine aggregation parameters
            if "aggregate_minutes" in timeframe_config:
                interval_minutes = timeframe_config["aggregate_minutes"]
                base_interval_minutes = 1  # 1-minute source
            elif "aggregate_hours" in timeframe_config:
                interval_minutes = timeframe_config["aggregate_hours"] * 60
                base_interval_minutes = 60  # 1-hour source
            elif "aggregate_weeks" in timeframe_config:
                interval_minutes = timeframe_config["aggregate_weeks"] * 7 * 24 * 60
                base_interval_minutes = 7 * 24 * 60  # 1-week source
            else:
                # Fallback to daily data
                return self._get_bar_data_single("1d", columns, min_bars, filters, extended_hours, start_date, end_date)
            
            # Calculate how many base intervals we need to get enough aggregated bars
            base_bars_needed = min_bars * (interval_minutes // base_interval_minutes)
            
            # Get base timeframe data (using the source timeframe)
            if source_table == "ohlcv_1m":
                source_timeframe = "1m"
            elif source_table == "ohlcv_1h":
                source_timeframe = "1h"
            elif source_table == "ohlcv_1w":
                source_timeframe = "1w"
            else:
                source_timeframe = "1d"
            
            # Fetch base data with increased min_bars to ensure enough data for aggregation
            base_data = self._get_bar_data_single(
                source_timeframe,
                ["securityid", "ticker", "timestamp", "open", "high", "low", "close", "volume"],
                base_bars_needed, filters, extended_hours, start_date, end_date
            )
            
            if base_data is None or len(base_data) == 0:
                return np.array([])
            
            # Perform aggregation
            aggregated_data = self._aggregate_ohlcv_data(base_data, interval_minutes, base_interval_minutes)
            
            # Filter columns if requested
            if columns and aggregated_data is not None and len(aggregated_data) > 0:
                aggregated_data = self._filter_columns(aggregated_data, columns)
            
            return aggregated_data
            
        except Exception as e:
            logger.error(f"Error in aggregated bar data: {e}")
            return np.array([])
    
    def _aggregate_ohlcv_data(self, base_data: np.ndarray, target_interval_minutes: int, 
                             base_interval_minutes: int) -> np.ndarray:
        """
        Aggregate OHLCV data from base timeframe to target interval
        
        Args:
            base_data: Source OHLCV data as numpy array
            target_interval_minutes: Target aggregation interval in minutes
            base_interval_minutes: Base data interval in minutes
            
        Returns:
            Aggregated OHLCV data as numpy array
        """
        try:
            if len(base_data) == 0:
                return np.array([])
            
            # Convert to pandas DataFrame for easier aggregation
            import pandas as pd
            
            # Determine column names (assuming standard OHLCV format)
            if base_data.shape[1] >= 8:
                columns = ["securityid", "ticker", "timestamp", "open", "high", "low", "close", "volume"]
            else:
                columns = ["securityid", "timestamp", "open", "high", "low", "close", "volume"]
            
            df = pd.DataFrame(base_data, columns=columns[:base_data.shape[1]])
            
            if len(df) == 0:
                return np.array([])
            
            # Convert timestamp to datetime for aggregation
            df['datetime'] = pd.to_datetime(df['timestamp'], unit='s')
            
            # Calculate aggregation interval
            interval_ratio = target_interval_minutes // base_interval_minutes
            
            # Group by security and time intervals
            aggregated_results = []
            
            for securityid in df['securityid'].unique():
                security_data = df[df['securityid'] == securityid].copy()
                security_data = security_data.sort_values('datetime')
                
                # Create time bins for aggregation
                if target_interval_minutes < 60:
                    # For minute aggregations
                    freq = f"{target_interval_minutes}T"
                elif target_interval_minutes < 1440:
                    # For hour aggregations
                    freq = f"{target_interval_minutes // 60}H"
                else:
                    # For day/week aggregations
                    freq = f"{target_interval_minutes // 1440}D"
                
                # Group by time intervals
                grouped = security_data.set_index('datetime').groupby(pd.Grouper(freq=freq))
                
                for interval_start, group in grouped:
                    if len(group) == 0:
                        continue
                    
                    # Aggregate OHLCV data
                    aggregated_row = {
                        'securityid': securityid,
                        'timestamp': int(interval_start.timestamp()),
                        'open': group['open'].iloc[0],  # First open
                        'high': group['high'].max(),    # Highest high
                        'low': group['low'].min(),      # Lowest low
                        'close': group['close'].iloc[-1], # Last close
                        'volume': group['volume'].sum()   # Total volume
                    }
                    
                    # Add ticker if available
                    if 'ticker' in group.columns:
                        aggregated_row['ticker'] = group['ticker'].iloc[0]
                    
                    aggregated_results.append(aggregated_row)
            
            if not aggregated_results:
                return np.array([])
            
            # Convert back to numpy array
            result_df = pd.DataFrame(aggregated_results)
            
            # Sort by securityid and timestamp
            result_df = result_df.sort_values(['securityid', 'timestamp'])
            
            # Ensure column order matches expected format
            if 'ticker' in result_df.columns:
                column_order = ['securityid', 'ticker', 'timestamp', 'open', 'high', 'low', 'close', 'volume']
            else:
                column_order = ['securityid', 'timestamp', 'open', 'high', 'low', 'close', 'volume']
            
            result_df = result_df[column_order]
            
            return result_df.values
            
        except Exception as e:
            logger.error(f"Error aggregating OHLCV data: {e}")
            return np.array([])
    
    def _filter_columns(self, data: np.ndarray, requested_columns: List[str]) -> np.ndarray:
        """Filter numpy array to only include requested columns"""
        try:
            if len(data) == 0:
                return np.array([])
            
            # Map column names to indices
            if data.shape[1] >= 8:
                available_columns = ["securityid", "ticker", "timestamp", "open", "high", "low", "close", "volume"]
            else:
                available_columns = ["securityid", "timestamp", "open", "high", "low", "close", "volume"]
            
            # Find indices of requested columns
            column_indices = []
            for col in requested_columns:
                if col in available_columns:
                    column_indices.append(available_columns.index(col))
            
            if not column_indices:
                return np.array([])
            
            # Extract only requested columns
            return data[:, column_indices]
            
        except Exception as e:
            logger.error(f"Error filtering columns: {e}")
            return np.array([])

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
_data_accessor = None

def get_data_accessor() -> DataAccessorProvider:
    """Get global data accessor instance"""
    global _data_accessor
    if _data_accessor is None:
        _data_accessor = DataAccessorProvider()
    return _data_accessor

def get_bar_data(timeframe: str = "1d", columns: List[str] = None, min_bars: int = 1, filters: Dict[str, any] = None,
                 aggregate_mode: bool = False, extended_hours: bool = False,
                 start_date: Optional[datetime] = None, end_date: Optional[datetime] = None) -> np.ndarray:
    """
    Global function for strategy access to bar data with intelligent batching
    
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
        numpy.ndarray with requested bar data
    """
    accessor = get_data_accessor()
    
    # Use new API directly
    return accessor.get_bar_data(timeframe, columns, min_bars, filters, aggregate_mode, extended_hours, start_date, end_date)

def get_general_data(columns: List[str] = None, filters: Dict[str, any] = None) -> pd.DataFrame:
    """
    Global function for strategy access to general security data
    
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
        pandas.DataFrame with general security information
    """
    accessor = get_data_accessor()
    return accessor.get_general_data(columns=columns, filters=filters)

def generate_equity_curve(instances, group_column=None):
    pass
