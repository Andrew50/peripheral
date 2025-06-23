"""
Data Provider
Provides market data and executes SQL queries for Python strategies
"""

import asyncio
import logging
import os
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Union

import numpy as np
import pandas as pd
import psycopg2
from sqlalchemy import create_engine

logger = logging.getLogger(__name__)


class DataProvider:
    """Provides data access for Python strategies"""

    def __init__(self):
        self.db_engine = self._create_db_engine()
        self._cache = {}  # Simple cache for repeated queries

    def _create_db_engine(self):
        """Create database engine"""
        db_host = os.environ.get("DB_HOST", "db")
        db_port = os.environ.get("DB_PORT", "5432")
        db_name = os.environ.get("POSTGRES_DB", "postgres")
        db_user = os.environ.get("DB_USER", "postgres")
        db_password = os.environ.get("DB_PASSWORD", "devpassword")

        connection_string = (
            f"postgresql://{db_user}:{db_password}@{db_host}:{db_port}/{db_name}"
        )

        try:
            engine = create_engine(connection_string, pool_size=5, max_overflow=10)
            return engine
        except Exception as e:
            logger.error(f"Failed to create database engine: {e}")
            return None

    async def execute_sql(self, sql_query: str) -> Optional[Dict[str, Any]]:
        """Execute SQL query and return results as DataFrame"""
        if not self.db_engine:
            logger.error("Database engine not available")
            return None

        try:
            # Validate SQL query for safety
            if not self._validate_sql_query(sql_query):
                logger.error("SQL query failed validation")
                return None

            # Execute query in thread pool to avoid blocking
            loop = asyncio.get_event_loop()
            df = await loop.run_in_executor(
                None, lambda: pd.read_sql_query(sql_query, self.db_engine)
            )

            # Convert DataFrame to dictionary format
            result = {
                "data": df.to_dict("records"),
                "columns": df.columns.tolist(),
                "shape": df.shape,
                "dtypes": df.dtypes.to_dict(),
            }

            logger.info(f"SQL query executed successfully, returned {len(df)} rows")
            return result

        except Exception as e:
            logger.error(f"Error executing SQL query: {e}")
            return None

    def _validate_table_name(self, table_name: str) -> bool:
        """Validate table name against allowed tables"""
        allowed_tables = {
            "ohlcv_1",
            "ohlcv_1m",
            "ohlcv_5m",
            "ohlcv_15m",
            "ohlcv_30m",
            "ohlcv_1h",
            "ohlcv_1d",
            "ohlcv_1w",
            "fundamentals",
            "securities",
            "sector_data",
        }
        return table_name in allowed_tables

    def _validate_sql_query(self, query: str) -> bool:
        """Validate SQL query for safety"""
        query_lower = query.lower().strip()

        # Only allow SELECT statements
        if not query_lower.startswith("select"):
            logger.warning("Only SELECT queries are allowed")
            return False

        # Forbidden keywords
        forbidden_keywords = [
            "insert",
            "update",
            "delete",
            "drop",
            "create",
            "alter",
            "truncate",
            "grant",
            "revoke",
            "exec",
            "execute",
            "sp_",
            "xp_",
            "bulk",
            "openrowset",
            "opendatasource",
        ]

        for keyword in forbidden_keywords:
            if keyword in query_lower:
                logger.warning(f"Forbidden keyword found: {keyword}")
                return False

        # Basic injection patterns
        injection_patterns = [
            ";",
            "--",
            "/*",
            "*/",
            "union",
            "union all",
            "information_schema",
            "pg_catalog",
            "pg_tables",
        ]

        for pattern in injection_patterns:
            if pattern in query_lower:
                logger.warning(f"Potentially dangerous pattern found: {pattern}")
                return False

        # Limit query length
        if len(query) > 10000:
            logger.warning("Query too long")
            return False

        return True

    # ==================== CORE DATA RETRIEVAL ====================

    async def get_price_data(
        self,
        symbol: str,
        timeframe: str = "1d",
        days: int = 30,
        extended_hours: bool = False,
        start_time: str = None,
        end_time: str = None,
    ) -> Dict:
        """Get price data with timeframe support"""
        try:
            # Map timeframes to database tables
            timeframe_tables = {
                "1m": "ohlcv_1m",
                "5m": "ohlcv_5m",
                "15m": "ohlcv_15m",
                "30m": "ohlcv_30m",
                "1h": "ohlcv_1h",
                "1d": "ohlcv_1d",
                "1w": "ohlcv_1w",
                "1M": "ohlcv_1d",  # Will aggregate from 1d
            }

            table_name = timeframe_tables.get(timeframe, "ohlcv_1d")

            # Validate table name for security
            if not self._validate_table_name(table_name):
                logger.error(f"Invalid table name: {table_name}")
                return {}

            # Build query with parameterized values
            # table_name is validated against allowlist above
            query = f"""
            SELECT 
                EXTRACT(EPOCH FROM o.timestamp)::bigint as timestamp,
                o.open,
                o.high,
                o.low,
                o.close,
                o.volume,
                CASE WHEN o.extended_hours IS NOT NULL THEN o.extended_hours ELSE false END as extended_hours
            FROM {table_name} o
            JOIN securities s ON o.securityid = s.securityid
            WHERE s.ticker = %s
            """  # nosec B608

            params = [symbol]

            # Add time filtering
            if start_time and timeframe in ["1m", "5m", "15m", "30m", "1h"]:
                query += " AND EXTRACT(TIME FROM o.timestamp) >= %s"
                params.append(start_time)
            if end_time and timeframe in ["1m", "5m", "15m", "30m", "1h"]:
                query += " AND EXTRACT(TIME FROM o.timestamp) <= %s"
                params.append(end_time)

            # Add extended hours filtering
            if not extended_hours and timeframe in ["1m", "5m", "15m", "30m", "1h"]:
                query += " AND (o.extended_hours IS NULL OR o.extended_hours = false)"

            # Add days limit
            if days > 0:
                query += " AND o.timestamp >= NOW() - INTERVAL %s"
                params.append(f"{days} days")

            query += " ORDER BY o.timestamp ASC LIMIT 10000"

            result = await self.execute_sql_parameterized(query, params)
            if result and result["data"]:
                data = result["data"]
                return {
                    "timestamps": [int(row["timestamp"]) for row in data],
                    "open": [float(row["open"]) for row in data],
                    "high": [float(row["high"]) for row in data],
                    "low": [float(row["low"]) for row in data],
                    "close": [float(row["close"]) for row in data],
                    "volume": [int(row["volume"]) for row in data],
                    "extended_hours": [
                        bool(row.get("extended_hours", False)) for row in data
                    ],
                }

            return {
                "timestamps": [],
                "open": [],
                "high": [],
                "low": [],
                "close": [],
                "volume": [],
                "extended_hours": [],
            }

        except Exception as e:
            logger.error(f"Error getting price data for {symbol}: {e}")
            return {
                "timestamps": [],
                "open": [],
                "high": [],
                "low": [],
                "close": [],
                "volume": [],
                "extended_hours": [],
            }

    async def execute_sql_parameterized(
        self, query: str, params: list = None
    ) -> Optional[Dict[str, Any]]:
        """Execute SQL with parameterized queries to prevent injection"""
        if not self.db_engine:
            logger.error("Database engine not available")
            return None

        try:
            # Validate the query structure
            if not self._validate_sql_query(query):
                logger.error("SQL query validation failed")
                return None

            # Execute parameterized query in thread pool to avoid blocking
            loop = asyncio.get_event_loop()

            def execute_with_params():
                if params:
                    # Convert psycopg2-style parameters (%s) to pandas-compatible format
                    param_dict = {f"param_{i}": param for i, param in enumerate(params)}
                    # Replace %s with %(param_0)s, %(param_1)s, etc.
                    formatted_query = query
                    for i in range(len(params)):
                        formatted_query = formatted_query.replace(
                            "%s", f"%(param_{i})s", 1
                        )
                    return pd.read_sql_query(
                        formatted_query, self.db_engine, params=param_dict
                    )
                else:
                    return pd.read_sql_query(query, self.db_engine)

            df = await loop.run_in_executor(None, execute_with_params)

            # Convert DataFrame to dictionary format
            result = {
                "data": df.to_dict("records"),
                "columns": df.columns.tolist(),
                "shape": df.shape,
                "dtypes": df.dtypes.to_dict(),
            }

            logger.info(
                f"Parameterized SQL query executed successfully, returned {len(df)} rows"
            )
            return result

        except Exception as e:
            logger.error(f"Error executing parameterized SQL query: {e}")
            return None

    async def get_historical_data(
        self, symbol: str, timeframe: str = "1d", periods: int = 100, offset: int = 0
    ) -> Dict:
        """Get historical price data with lag support"""
        try:
            timeframe_tables = {
                "1m": "ohlcv_1",
                "1h": "ohlcv_1h",
                "1d": "ohlcv_1d",
                "1w": "ohlcv_1w",
            }
            table_name = timeframe_tables.get(timeframe, "ohlcv_1d")

            # Validate table name for security
            if not self._validate_table_name(table_name):
                logger.error(f"Invalid table name: {table_name}")
                return {}

            # table_name is validated against allowlist above
            query = f"""
            SELECT 
                EXTRACT(EPOCH FROM o.timestamp)::bigint as timestamp,
                o.open, o.high, o.low, o.close, o.volume
            FROM {table_name} o
            JOIN securities s ON o.securityid = s.securityid
            WHERE s.ticker = %s
            ORDER BY o.timestamp DESC
            OFFSET %s
            LIMIT %s
            """  # nosec B608

            params = [symbol, offset, periods]
            result = await self.execute_sql_parameterized(query, params)
            if result and result["data"]:
                data = list(reversed(result["data"]))  # Return chronological order
                return {
                    "timestamps": [int(row["timestamp"]) for row in data],
                    "open": [float(row["open"]) for row in data],
                    "high": [float(row["high"]) for row in data],
                    "low": [float(row["low"]) for row in data],
                    "close": [float(row["close"]) for row in data],
                    "volume": [int(row["volume"]) for row in data],
                }

            return {
                "timestamps": [],
                "open": [],
                "high": [],
                "low": [],
                "close": [],
                "volume": [],
            }

        except Exception as e:
            logger.error(f"Error getting historical data for {symbol}: {e}")
            return {
                "timestamps": [],
                "open": [],
                "high": [],
                "low": [],
                "close": [],
                "volume": [],
            }

    async def get_security_info(self, symbol: str) -> Dict:
        """Get detailed security metadata and classification"""
        try:
            query = """
            SELECT 
                securityid,
                ticker,
                name,
                sector,
                industry,
                market,
                primary_exchange,
                locale,
                active,
                cik,
                composite_figi,
                share_class_figi
            FROM securities
            WHERE ticker = %s
            LIMIT 1
            """

            result = await self.execute_sql_parameterized(query, [symbol])
            if result and result["data"]:
                return result["data"][0]

            return {}

        except Exception as e:
            logger.error(f"Error getting security info for {symbol}: {e}")
            return {}

    async def get_multiple_symbols_data(
        self, symbols: List[str], timeframe: str = "1d", days: int = 30
    ) -> Dict[str, Dict]:
        """Get price data for multiple symbols efficiently"""
        result = {}
        for symbol in symbols:
            result[symbol] = await self.get_price_data(symbol, timeframe, days)
        return result

    # ==================== RAW FUNDAMENTAL DATA ====================

    async def get_fundamental_data(
        self, symbol: str, metrics: Optional[List[str]] = None
    ) -> Dict:
        """Get raw fundamental data for a symbol"""
        try:
            if not metrics:
                metrics = [
                    "market_cap",
                    "eps",
                    "revenue",
                    "dividend",
                    "shares_outstanding",
                    "book_value",
                    "debt",
                    "cash",
                    "free_cash_flow",
                    "gross_profit",
                    "operating_income",
                    "net_income",
                    "total_assets",
                    "total_liabilities",
                ]

            # Validate metrics to prevent injection
            allowed_metrics = [
                "market_cap",
                "eps",
                "revenue",
                "dividend",
                "shares_outstanding",
                "book_value",
                "debt",
                "cash",
                "free_cash_flow",
                "gross_profit",
                "operating_income",
                "net_income",
                "total_assets",
                "total_liabilities",
            ]

            # Filter metrics to only allowed ones
            safe_metrics = [m for m in metrics if m in allowed_metrics]
            if not safe_metrics:
                return {}

            # Build dynamic query based on available metrics - safe since metrics are validated
            metrics_str = ", ".join([f"f.{metric}" for metric in safe_metrics])

            # metrics are validated against allowed list above
            query = f"""
            SELECT {metrics_str}
            FROM fundamentals f
            JOIN securities s ON f.security_id = s.securityid
            WHERE s.ticker = %s
            ORDER BY f.timestamp DESC
            LIMIT 1
            """  # nosec B608

            result = await self.execute_sql_parameterized(query, [symbol])
            if result and result["data"]:
                data = result["data"][0]
                fundamentals = {}
                for metric in safe_metrics:
                    fundamentals[metric] = data.get(metric)

                return fundamentals

            return {}

        except Exception as e:
            logger.error(f"Error getting fundamental data for {symbol}: {e}")
            return {}

    # ==================== RAW MARKET & SECTOR DATA ====================

    async def get_sector_performance(
        self, sector: str = None, days: int = 5, metrics: List[str] = None
    ) -> Dict:
        """Get raw sector performance data"""
        try:
            if not metrics:
                metrics = ["return", "volume", "market_cap"]

            # Input validation
            if days <= 0 or days > 365:
                days = 5

            # Using parameterized query construction to prevent SQL injection
            params = [f"{days} days", f"{days} days"]
            
            # Build query safely using string concatenation instead of f-strings
            base_query = """
            WITH sector_data AS (
                SELECT 
                    s.sector,
                    s.ticker,
                    f.market_cap,
                    o1.close as current_close,
                    o2.close as prev_close,
                    o1.volume as current_volume
                FROM securities s
                JOIN fundamentals f ON s.securityid = f.security_id
                JOIN ohlcv_1d o1 ON s.securityid = o1.securityid
                JOIN ohlcv_1d o2 ON s.securityid = o2.securityid
                WHERE o1.timestamp >= CURRENT_DATE - INTERVAL %s
                AND o2.timestamp = o1.timestamp - INTERVAL %s"""
            
            if sector:
                params.append(sector)
                sector_filter = "\n                AND s.sector = %s"
            else:
                sector_filter = ""

            # Construct fully parameterized query - safe from SQL injection
            # sector_filter is a hardcoded string with parameterized placeholder
            query = base_query + sector_filter + """
            )
            SELECT 
                sector,
                AVG((current_close - prev_close) / prev_close * 100) as return,
                SUM(current_volume) as volume,
                SUM(market_cap) as market_cap
            FROM sector_data
            WHERE current_close IS NOT NULL AND prev_close IS NOT NULL
            GROUP BY sector
            ORDER BY return DESC
            """  # nosec B608

            result = await self.execute_sql_parameterized(query, params)
            if result and result["data"]:
                if sector:
                    return result["data"][0] if result["data"] else {}
                else:
                    return {row["sector"]: row for row in result["data"]}

            return {}

        except Exception as e:
            logger.error(f"Error getting sector performance: {e}")
            return {}

    # ==================== UTILITY FUNCTIONS ====================

    async def scan_universe(
        self, filters: Dict = None, sort_by: str = None, limit: int = 100
    ) -> Dict:
        """Screen stocks based on raw criteria"""
        try:
            # Input validation
            if limit <= 0 or limit > 1000:
                limit = 100

            query = """
            SELECT DISTINCT
                s.ticker,
                s.sector,
                s.industry,
                f.market_cap,
                f.eps,
                o.close as price,
                o.volume
            FROM securities s
            LEFT JOIN fundamentals f ON s.securityid = f.security_id
            LEFT JOIN ohlcv_1d o ON s.securityid = o.securityid
            WHERE s.active = true
            AND o.timestamp >= CURRENT_DATE - INTERVAL '7 days'
            """

            params = []

            # Apply filters with parameterized queries
            if filters:
                if "sector" in filters:
                    query += " AND s.sector = %s"
                    params.append(filters["sector"])
                if "min_market_cap" in filters:
                    query += " AND f.market_cap >= %s"
                    params.append(filters["min_market_cap"])
                if "max_pe_ratio" in filters:
                    query += " AND (f.market_cap / f.shares_outstanding) / f.eps <= %s"
                    params.append(filters["max_pe_ratio"])

            # Validate sort_by to prevent injection
            allowed_sort_fields = [
                "ticker",
                "sector",
                "market_cap",
                "eps",
                "price",
                "volume",
            ]
            if sort_by and sort_by in allowed_sort_fields:
                # Safe to interpolate since sort_by is validated against allowlist
                query += f" ORDER BY {sort_by} DESC"
            else:
                query += " ORDER BY f.market_cap DESC"

            query += " LIMIT %s"
            params.append(limit)

            result = await self.execute_sql_parameterized(query, params)
            if result and result["data"]:
                return {
                    "symbols": [row["ticker"] for row in result["data"]],
                    "data": result["data"],
                }

            return {"symbols": [], "data": []}

        except Exception as e:
            logger.error(f"Error scanning universe: {e}")
            return {"symbols": [], "data": []}
