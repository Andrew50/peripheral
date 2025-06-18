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
        db_host = os.environ.get('DB_HOST', 'localhost')
        db_port = os.environ.get('DB_PORT', '5432')
        db_name = os.environ.get('DB_NAME', 'study')
        db_user = os.environ.get('DB_USER', 'postgres')
        db_password = os.environ.get('DB_PASSWORD', '')
        
        connection_string = f"postgresql://{db_user}:{db_password}@{db_host}:{db_port}/{db_name}"
        
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
                None, 
                lambda: pd.read_sql_query(sql_query, self.db_engine)
            )
            
            # Convert DataFrame to dictionary format
            result = {
                'data': df.to_dict('records'),
                'columns': df.columns.tolist(),
                'shape': df.shape,
                'dtypes': df.dtypes.to_dict()
            }
            
            logger.info(f"SQL query executed successfully, returned {len(df)} rows")
            return result
            
        except Exception as e:
            logger.error(f"Error executing SQL query: {e}")
            return None
    
    def _validate_sql_query(self, query: str) -> bool:
        """Validate SQL query for safety"""
        query_lower = query.lower().strip()
        
        # Only allow SELECT statements
        if not query_lower.startswith('select'):
            logger.warning("Only SELECT queries are allowed")
            return False
        
        # Forbidden keywords
        forbidden_keywords = [
            'insert', 'update', 'delete', 'drop', 'create', 'alter',
            'truncate', 'grant', 'revoke', 'exec', 'execute',
            'sp_', 'xp_', 'bulk', 'openrowset', 'opendatasource'
        ]
        
        for keyword in forbidden_keywords:
            if keyword in query_lower:
                logger.warning(f"Forbidden keyword found: {keyword}")
                return False
        
        # Basic injection patterns
        injection_patterns = [
            ';', '--', '/*', '*/', 'union', 'union all',
            'information_schema', 'pg_catalog', 'pg_tables'
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
        timeframe: str = '1d', 
        days: int = 30, 
        extended_hours: bool = False, 
        start_time: str = None, 
        end_time: str = None
    ) -> Dict:
        """Get OHLCV price data for a symbol with flexible timeframe support"""
        try:
            # Map timeframe to table name
            timeframe_tables = {
                '1m': 'ohlcv_1',
                '5m': 'ohlcv_1',  # Will aggregate from 1m
                '15m': 'ohlcv_1', # Will aggregate from 1m
                '30m': 'ohlcv_1', # Will aggregate from 1m
                '1h': 'ohlcv_1h',
                '4h': 'ohlcv_1h', # Will aggregate from 1h
                '1d': 'ohlcv_1d',
                '1w': 'ohlcv_1w',
                '1M': 'ohlcv_1d'  # Will aggregate from 1d
            }
            
            table_name = timeframe_tables.get(timeframe, 'ohlcv_1d')
            
            # Build query
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
            JOIN securities s ON o.security_id = s.security_id
            WHERE s.ticker = '{symbol}'
            """
            
            # Add time filtering
            if start_time and timeframe in ['1m', '5m', '15m', '30m', '1h']:
                query += f" AND EXTRACT(TIME FROM o.timestamp) >= '{start_time}'"
            if end_time and timeframe in ['1m', '5m', '15m', '30m', '1h']:
                query += f" AND EXTRACT(TIME FROM o.timestamp) <= '{end_time}'"
            
            # Add extended hours filtering
            if not extended_hours and timeframe in ['1m', '5m', '15m', '30m', '1h']:
                query += " AND (o.extended_hours IS NULL OR o.extended_hours = false)"
            
            # Add days limit
            if days > 0:
                query += f" AND o.timestamp >= NOW() - INTERVAL '{days} days'"
            
            query += " ORDER BY o.timestamp ASC LIMIT 10000"
            
            result = await self.execute_sql(query)
            if result and result['data']:
                data = result['data']
                return {
                    'timestamps': [int(row['timestamp']) for row in data],
                    'open': [float(row['open']) for row in data],
                    'high': [float(row['high']) for row in data],
                    'low': [float(row['low']) for row in data],
                    'close': [float(row['close']) for row in data],
                    'volume': [int(row['volume']) for row in data],
                    'extended_hours': [bool(row.get('extended_hours', False)) for row in data]
                }
            
            return {
                'timestamps': [], 'open': [], 'high': [], 'low': [], 
                'close': [], 'volume': [], 'extended_hours': []
            }
            
        except Exception as e:
            logger.error(f"Error getting price data for {symbol}: {e}")
            return {
                'timestamps': [], 'open': [], 'high': [], 'low': [], 
                'close': [], 'volume': [], 'extended_hours': []
            }
    
    async def get_historical_data(
        self, 
        symbol: str, 
        timeframe: str = '1d', 
        periods: int = 100, 
        offset: int = 0
    ) -> Dict:
        """Get historical price data with lag support"""
        try:
            timeframe_tables = {
                '1m': 'ohlcv_1', '1h': 'ohlcv_1h', '1d': 'ohlcv_1d', '1w': 'ohlcv_1w'
            }
            table_name = timeframe_tables.get(timeframe, 'ohlcv_1d')
            
            query = f"""
            SELECT 
                EXTRACT(EPOCH FROM o.timestamp)::bigint as timestamp,
                o.open, o.high, o.low, o.close, o.volume
            FROM {table_name} o
            JOIN securities s ON o.security_id = s.security_id
            WHERE s.ticker = '{symbol}'
            ORDER BY o.timestamp DESC
            OFFSET {offset}
            LIMIT {periods}
            """
            
            result = await self.execute_sql(query)
            if result and result['data']:
                data = list(reversed(result['data']))  # Return chronological order
                return {
                    'timestamps': [int(row['timestamp']) for row in data],
                    'open': [float(row['open']) for row in data],
                    'high': [float(row['high']) for row in data],
                    'low': [float(row['low']) for row in data],
                    'close': [float(row['close']) for row in data],
                    'volume': [int(row['volume']) for row in data]
                }
            
            return {'timestamps': [], 'open': [], 'high': [], 'low': [], 'close': [], 'volume': []}
            
        except Exception as e:
            logger.error(f"Error getting historical data for {symbol}: {e}")
            return {'timestamps': [], 'open': [], 'high': [], 'low': [], 'close': [], 'volume': []}
    
    async def get_security_info(self, symbol: str) -> Dict:
        """Get detailed security metadata and classification"""
        try:
            query = f"""
            SELECT 
                security_id as securityid,
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
            WHERE ticker = '{symbol}'
            LIMIT 1
            """
            
            result = await self.execute_sql(query)
            if result and result['data']:
                return result['data'][0]
            
            return {}
            
        except Exception as e:
            logger.error(f"Error getting security info for {symbol}: {e}")
            return {}
    
    async def get_multiple_symbols_data(
        self, 
        symbols: List[str], 
        timeframe: str = '1d', 
        days: int = 30
    ) -> Dict[str, Dict]:
        """Get price data for multiple symbols efficiently"""
        result = {}
        for symbol in symbols:
            result[symbol] = await self.get_price_data(symbol, timeframe, days)
        return result

    # ==================== RAW FUNDAMENTAL DATA ====================
    
    async def get_fundamental_data(
        self, 
        symbol: str, 
        metrics: Optional[List[str]] = None
    ) -> Dict:
        """Get raw fundamental data for a symbol"""
        try:
            if not metrics:
                metrics = [
                    'market_cap', 'eps', 'revenue', 'dividend', 'shares_outstanding',
                    'book_value', 'debt', 'cash', 'free_cash_flow', 'gross_profit',
                    'operating_income', 'net_income', 'total_assets', 'total_liabilities'
                ]
            
            # Build dynamic query based on available metrics
            available_metrics = []
            for metric in metrics:
                available_metrics.append(f'f.{metric}')
            
            metrics_str = ', '.join(available_metrics)
            
            query = f"""
            SELECT {metrics_str}
            FROM fundamentals f
            JOIN securities s ON f.security_id = s.security_id
            WHERE s.ticker = '{symbol}'
            ORDER BY f.timestamp DESC
            LIMIT 1
            """
            
            result = await self.execute_sql(query)
            if result and result['data']:
                data = result['data'][0]
                fundamentals = {}
                for metric in metrics:
                    fundamentals[metric] = data.get(metric)
                
                return fundamentals
            
            return {}
            
        except Exception as e:
            logger.error(f"Error getting fundamental data for {symbol}: {e}")
            return {}
    
    # ==================== RAW MARKET & SECTOR DATA ====================
    
    async def get_sector_performance(
        self, 
        sector: str = None, 
        days: int = 5, 
        metrics: List[str] = None
    ) -> Dict:
        """Get raw sector performance data"""
        try:
            if not metrics:
                metrics = ['return', 'volume', 'market_cap']
            
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
                JOIN fundamentals f ON s.security_id = f.security_id
                JOIN ohlcv_1d o1 ON s.security_id = o1.security_id
                JOIN ohlcv_1d o2 ON s.security_id = o2.security_id
                WHERE o1.timestamp >= CURRENT_DATE - INTERVAL '%d days'
                AND o2.timestamp = o1.timestamp - INTERVAL '%d days'
            """ % (days, days)
            
            if sector:
                base_query += f" AND s.sector = '{sector}'"
            
            query = base_query + """
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
            """
            
            result = await self.execute_sql(query)
            if result and result['data']:
                if sector:
                    return result['data'][0] if result['data'] else {}
                else:
                    return {row['sector']: row for row in result['data']}
            
            return {}
            
        except Exception as e:
            logger.error(f"Error getting sector performance: {e}")
            return {}

    # ==================== UTILITY FUNCTIONS ====================
    
    async def scan_universe(
        self, 
        filters: Dict = None, 
        sort_by: str = None, 
        limit: int = 100
    ) -> Dict:
        """Screen stocks based on raw criteria"""
        try:
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
            LEFT JOIN fundamentals f ON s.security_id = f.security_id
            LEFT JOIN ohlcv_1d o ON s.security_id = o.security_id
            WHERE s.active = true
            AND o.timestamp >= CURRENT_DATE - INTERVAL '7 days'
            """
            
            # Apply filters
            if filters:
                if 'sector' in filters:
                    query += f" AND s.sector = '{filters['sector']}'"
                if 'min_market_cap' in filters:
                    query += f" AND f.market_cap >= {filters['min_market_cap']}"
                if 'max_pe_ratio' in filters and 'pe_ratio' in filters:
                    query += f" AND (f.market_cap / f.shares_outstanding) / f.eps <= {filters['max_pe_ratio']}"
            
            if sort_by:
                query += f" ORDER BY {sort_by} DESC"
            else:
                query += " ORDER BY f.market_cap DESC"
            
            query += f" LIMIT {limit}"
            
            result = await self.execute_sql(query)
            if result and result['data']:
                return {'symbols': [row['ticker'] for row in result['data']], 'data': result['data']}
            
            return {'symbols': [], 'data': []}
            
        except Exception as e:
            logger.error(f"Error scanning universe: {e}")
            return {'symbols': [], 'data': []}

