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
try:
    import psycopg2
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
        
    def get_connection(self):
        """Get database connection"""
        return psycopg2.connect(**self.db_config)
    
    def set_execution_context(self, mode: str, symbols: List[str] = None, 
                             start_date: datetime = None, end_date: datetime = None):
        """Set execution context for data fetching strategy"""
        self.execution_context = {
            'mode': mode,
            'symbols': symbols,
            'start_date': start_date,
            'end_date': end_date
        }

    def get_bar_data(self, timeframe: str = "1d", security_ids: List[int] = None, 
                     columns: List[str] = None, min_bars: int = 1) -> np.ndarray:
        """
        Get OHLCV bar data as numpy array with context-aware date ranges
        
        Args:
            timeframe: Data timeframe ('1d', '1h', '5m', etc.)
            security_ids: List of security IDs to fetch (None = all active securities, explicit list recommended)
            columns: Desired columns (None = all: securityid, timestamp, open, high, low, close, volume)
            min_bars: Minimum number of bars of the specified timeframe required
            
        Returns:
            numpy.ndarray with columns: securityid, timestamp, open, high, low, close, volume
        """
        try:
            # Validate inputs
            if min_bars < 1:
                min_bars = 1
            if min_bars > 10000:  # Prevent excessive data requests
                min_bars = 10000
                
            # Map timeframes to database tables
            timeframe_tables = {
                "1m": "ohlcv_1m",
                "5m": "ohlcv_5m", 
                "15m": "ohlcv_15m",
                "30m": "ohlcv_30m",
                "1h": "ohlcv_1h",
                "1d": "ohlcv_1d",
                "1w": "ohlcv_1w"
            }
            
            table_name = timeframe_tables.get(timeframe, "ohlcv_1d")
            
            # Default columns if not specified (removed ticker and adj_close)
            if columns is None:
                columns = ["securityid", "timestamp", "open", "high", "low", "close", "volume"]
            
            # Validate columns against allowed set (removed adj_close)
            allowed_columns = {"securityid", "ticker", "timestamp", "open", "high", "low", "close", "volume"}
            safe_columns = [col for col in columns if col in allowed_columns]
            
            if not safe_columns:
                return np.array([])
            
            # Determine date range based on execution context
            context = self.execution_context
            
            if context['mode'] == 'backtest':
                # For backtest: get data from (start_date - min_bars) to end_date
                if context['start_date'] and context['end_date']:
                    # Calculate how far back to go for min_bars
                    timeframe_delta = self._get_timeframe_delta(timeframe)
                    start_with_buffer = context['start_date'] - (timeframe_delta * min_bars)
                    date_filter = "o.timestamp >= %s AND o.timestamp <= %s"
                    date_params = [start_with_buffer, context['end_date']]
                else:
                    # Fallback if dates not provided
                    date_filter = "o.timestamp >= NOW() - INTERVAL '1 year'"
                    date_params = []
            else:
                # For screening/alerts: get most recent min_bars only
                timeframe_delta = self._get_timeframe_delta(timeframe)
                lookback_duration = timeframe_delta * min_bars * 2  # Extra buffer for data availability
                date_filter = "o.timestamp >= %s"
                date_params = [datetime.now() - lookback_duration]
            
            # Handle security filtering
            if security_ids is None or len(security_ids) == 0:
                # Get all active securities (don't use context symbols automatically)
                security_filter = "s.active = true AND s.maxdate IS NULL"
                security_params = []
            else:
                # Check if security_ids contains strings (ticker symbols) instead of integers
                if isinstance(security_ids, list) and len(security_ids) > 0:
                    if isinstance(security_ids[0], str):
                        # Convert ticker symbols to security IDs
                        logger.info(f"Converting ticker symbols {security_ids} to security IDs")
                        security_ids = self._get_security_ids_from_tickers(security_ids)
                        if not security_ids:
                            logger.warning("No security IDs found for provided tickers")
                            return np.array([])
                
                # Use provided security IDs
                placeholders = ','.join(['%s'] * len(security_ids))
                security_filter = f"s.securityid IN ({placeholders})"
                security_params = security_ids
            
            # Build column selection
            select_columns = []
            for col in safe_columns:
                if col == "securityid":
                    select_columns.append("s.securityid")
                elif col == "ticker":
                    select_columns.append("s.ticker")
                elif col == "timestamp":
                    select_columns.append("EXTRACT(EPOCH FROM o.timestamp)::bigint as timestamp")
                else:
                    select_columns.append(f"o.{col}")
            
            # Build the complete query
            if context['mode'] == 'backtest':
                # For backtest: get all data in range, don't limit per security
                # Build parameterized query components
                select_clause = ', '.join(select_columns)
                from_clause = f"{table_name} o JOIN securities s ON o.securityid = s.securityid"
                where_clause = f"{security_filter} AND {date_filter}"
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
                from_clause = f"{table_name} o JOIN securities s ON o.securityid = s.securityid"
                where_clause = f"{security_filter} AND {date_filter}"
                final_select_clause = ', '.join(final_columns)
                
                # nosec B608: Safe - table_name from controlled timeframe_tables dict, columns validated against allowlist, all dynamic params parameterized
                query = f"""WITH ranked_data AS (
                    SELECT {select_clause},
                           ROW_NUMBER() OVER (PARTITION BY s.securityid ORDER BY o.timestamp DESC) as rn
                    FROM {from_clause}
                    WHERE {where_clause}
                )
                SELECT {final_select_clause}
                FROM ranked_data 
                WHERE rn <= %s
                ORDER BY securityid, timestamp ASC"""  # nosec B608
                params = security_params + date_params + [min_bars]
            
            conn = self.get_connection()
            cursor = conn.cursor(cursor_factory=RealDictCursor)
            
            cursor.execute(query, params)
            results = cursor.fetchall()
            
            cursor.close()
            conn.close()
            
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
    
    def _get_security_ids_from_tickers(self, tickers: List[str]) -> List[int]:
        """Convert ticker symbols to security IDs"""
        try:
            if not tickers:
                return []
            
            placeholders = ','.join(['%s'] * len(tickers))
            # nosec B608: Safe - placeholders are just '%s' strings, tickers validated as list of strings, all values parameterized
            query = f"SELECT securityid FROM securities WHERE ticker IN ({placeholders}) AND active = true AND maxdate IS NULL"  # nosec B608
            
            conn = self.get_connection()
            cursor = conn.cursor()
            cursor.execute(query, tickers)
            results = cursor.fetchall()
            cursor.close()
            conn.close()
            
            return [row[0] for row in results]
            
        except Exception as e:
            logger.error(f"Error converting tickers to security IDs: {e}")
            return []

    def get_general_data(self, security_ids: List[int] = None, columns: List[str] = None) -> pd.DataFrame:
        """
        Get general security information as pandas DataFrame
        
        Args:
            security_ids: List of security IDs to fetch (None = all active securities)
            columns: Desired columns (None = all available)
            
        Returns:
            pandas.DataFrame with columns: ticker, name, sector, industry, market, primary_exchange, 
                                         locale, active, description, cik
        """
        try:
            # Default columns if not specified - include ticker by default
            if columns is None:
                columns = ["ticker", "name", "sector", "industry", "market", "primary_exchange", 
                          "locale", "active", "description", "cik"]
            
            # Validate columns against allowed set
            allowed_columns = {
                "securityid", "ticker", "name", "sector", "industry", "market", 
                "primary_exchange", "locale", "active", "description", "cik",
                "market_cap", "share_class_shares_outstanding"
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
                
            # Build the query
            if security_ids is None or len(security_ids) == 0:
                # Get all active securities
                select_clause = ', '.join(internal_columns)
                # nosec B608: Safe - columns validated against allowlist, no user input in table name or WHERE clause
                query = f"SELECT {select_clause} FROM securities WHERE active = true AND maxdate IS NULL ORDER BY securityid"  # nosec B608
                params = []
            else:
                # Handle ticker symbols in security_ids parameter (convert if strings detected)
                if isinstance(security_ids, list) and len(security_ids) > 0:
                    if isinstance(security_ids[0], str):
                        # Convert ticker symbols to security IDs
                        logger.info(f"Converting ticker symbols {security_ids} to security IDs for general data")
                        security_ids = self._get_security_ids_from_tickers(security_ids)
                        if not security_ids:
                            logger.warning("No security IDs found for provided tickers")
                            return pd.DataFrame()
                
                # Filter by specific security IDs
                placeholders = ','.join(['%s'] * len(security_ids))
                select_clause = ', '.join(internal_columns)
                # nosec B608: Safe - columns validated against allowlist, placeholders are just '%s' strings, all values parameterized  
                query = f"SELECT {select_clause} FROM securities WHERE securityid IN ({placeholders}) AND maxdate IS NULL ORDER BY securityid"  # nosec B608
                params = security_ids
            
            conn = self.get_connection()
            cursor = conn.cursor(cursor_factory=RealDictCursor)
            
            cursor.execute(query, params)
            results = cursor.fetchall()
            
            cursor.close()
            conn.close()
            
            if not results:
                return pd.DataFrame()
            
            # Convert to DataFrame
            df = pd.DataFrame(results)
            
            # Filter to only requested columns for the final result
            final_columns = [col for col in safe_columns if col in df.columns]
            if final_columns:
                df = df[final_columns]
            
            # Use ticker as index if available and securityid was not explicitly requested
            if "ticker" in df.columns and "securityid" not in safe_columns:
                df.set_index("ticker", inplace=True)
            elif "securityid" in df.columns:
                df.set_index("securityid", inplace=True)
            
            return df
            
        except Exception as e:
            logger.error(f"Error in get_general_data: {e}")
            return pd.DataFrame()


# Global instance for strategy execution
_data_accessor = None

def get_data_accessor() -> DataAccessorProvider:
    """Get global data accessor instance"""
    global _data_accessor
    if _data_accessor is None:
        _data_accessor = DataAccessorProvider()
    return _data_accessor

def get_bar_data(timeframe: str = "1d", tickers: List[str] = None, security_ids: List[int] = None,
                 columns: List[str] = None, min_bars: int = 1) -> np.ndarray:
    """
    Global function for strategy access to bar data
    
    Args:
        timeframe: Data timeframe ('1d', '1h', '5m', etc.)
        tickers: List of ticker symbols to fetch (e.g., ['AAPL', 'MRNA']) (None = all active securities)
        security_ids: List of security IDs to fetch (deprecated, use tickers instead)
        columns: Desired columns (None = all: ticker, timestamp, open, high, low, close, volume)
        min_bars: Minimum number of bars required per security
        
    Returns:
        numpy.ndarray with requested bar data
    """
    accessor = get_data_accessor()
    
    # Handle tickers parameter (preferred) or fall back to security_ids
    if tickers is not None:
        # Convert tickers to security_ids if provided
        if len(tickers) == 0:
            final_security_ids = []
        else:
            final_security_ids = accessor._get_security_ids_from_tickers(tickers)
    else:
        # Use security_ids directly (backward compatibility)
        final_security_ids = security_ids
    
    return accessor.get_bar_data(timeframe, final_security_ids, columns, min_bars)

def get_general_data(tickers: List[str] = None, security_ids: List[int] = None, columns: List[str] = None) -> pd.DataFrame:
    """
    Global function for strategy access to general security data
    
    Args:
        tickers: List of ticker symbols to fetch (e.g., ['AAPL', 'MRNA']) (None = all active securities)
        security_ids: List of security IDs to fetch (deprecated, use tickers instead)
        columns: Desired columns (None = all available)
        
    Returns:
        pandas.DataFrame with general security information
    """
    accessor = get_data_accessor()
    
    # Handle tickers parameter (preferred) or fall back to security_ids
    if tickers is not None:
        # Convert tickers to security_ids if provided
        if len(tickers) == 0:
            final_security_ids = []
        else:
            final_security_ids = accessor._get_security_ids_from_tickers(tickers)
    else:
        # Use security_ids directly (backward compatibility)
        final_security_ids = security_ids
    
    return accessor.get_general_data(final_security_ids, columns) 