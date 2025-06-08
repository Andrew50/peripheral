"""
Data Provider
Provides market data and executes SQL queries for Python strategies
"""

import asyncio
import logging
import os
from typing import Any, Dict, Optional

import pandas as pd
import psycopg2
from sqlalchemy import create_engine

logger = logging.getLogger(__name__)


class DataProvider:
    """Provides data access for Python strategies"""
    
    def __init__(self):
        self.db_engine = self._create_db_engine()
    
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
    
    async def get_ohlcv_data(
        self,
        symbols: list,
        timeframe: str = '1d',
        limit: int = 100,
        start_date: Optional[str] = None,
        end_date: Optional[str] = None
    ) -> Optional[pd.DataFrame]:
        """Get OHLCV data for symbols"""
        
        # Map timeframe to table name
        timeframe_tables = {
            '1': 'ohlcv_1',
            '1h': 'ohlcv_1h', 
            '1d': 'ohlcv_1d',
            '1w': 'ohlcv_1w'
        }
        
        table_name = timeframe_tables.get(timeframe, 'ohlcv_1d')
        
        # Build query
        symbols_str = "', '".join(symbols)
        query = f"""
        SELECT 
            s.ticker,
            o.timestamp,
            o.open,
            o.high,
            o.low,
            o.close,
            o.volume
        FROM {table_name} o
        JOIN securities s ON o.security_id = s.security_id
        WHERE s.ticker IN ('{symbols_str}')
        """
        
        if start_date:
            query += f" AND o.timestamp >= '{start_date}'"
        if end_date:
            query += f" AND o.timestamp <= '{end_date}'"
            
        query += f" ORDER BY o.timestamp DESC LIMIT {limit}"
        
        result = await self.execute_sql(query)
        if result:
            return pd.DataFrame(result['data'])
        return None
    
    async def get_fundamental_data(
        self,
        symbols: list,
        metrics: list = None,
        limit: int = 100
    ) -> Optional[pd.DataFrame]:
        """Get fundamental data for symbols"""
        
        if not metrics:
            metrics = ['market_cap', 'shares_outstanding', 'eps', 'revenue']
        
        metrics_str = ', '.join(f'f.{metric}' for metric in metrics)
        symbols_str = "', '".join(symbols)
        
        query = f"""
        SELECT 
            s.ticker,
            f.timestamp,
            {metrics_str}
        FROM fundamentals f
        JOIN securities s ON f.security_id = s.security_id
        WHERE s.ticker IN ('{symbols_str}')
        ORDER BY f.timestamp DESC
        LIMIT {limit}
        """
        
        result = await self.execute_sql(query)
        if result:
            return pd.DataFrame(result['data'])
        return None
    
    async def get_security_info(self, symbols: list) -> Optional[pd.DataFrame]:
        """Get security information"""
        
        symbols_str = "', '".join(symbols)
        query = f"""
        SELECT 
            ticker,
            name,
            sector,
            industry,
            market,
            primary_exchange,
            locale,
            active
        FROM securities
        WHERE ticker IN ('{symbols_str}')
        """
        
        result = await self.execute_sql(query)
        if result:
            return pd.DataFrame(result['data'])
        return None
    
    async def get_universe_data(
        self,
        filters: Dict[str, Any] = None,
        timeframe: str = '1d',
        limit: int = 1000
    ) -> Optional[pd.DataFrame]:
        """Get universe of securities based on filters"""
        
        query = f"""
        SELECT DISTINCT
            s.ticker,
            s.name,
            s.sector,
            s.industry,
            s.market,
            s.primary_exchange
        FROM securities s
        WHERE s.active = true
        """
        
        if filters:
            if 'sectors' in filters:
                sectors_str = "', '".join(filters['sectors'])
                query += f" AND s.sector IN ('{sectors_str}')"
            
            if 'markets' in filters:
                markets_str = "', '".join(filters['markets'])
                query += f" AND s.market IN ('{markets_str}')"
            
            if 'exchanges' in filters:
                exchanges_str = "', '".join(filters['exchanges'])
                query += f" AND s.primary_exchange IN ('{exchanges_str}')"
        
        query += f" ORDER BY s.ticker LIMIT {limit}"
        
        result = await self.execute_sql(query)
        if result:
            return pd.DataFrame(result['data'])
        return None