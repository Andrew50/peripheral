"""
Connection Manager
Centralized connection management for Redis and Database connections
"""

import os
import logging
from contextlib import contextmanager
from typing import Iterator, Optional, Type

import psycopg2
from psycopg2.extras import RealDictCursor
from psycopg2.extensions import connection as PGConnection, cursor as PGCursor

import redis
from openai import OpenAI
from google import genai

# Configure logging
logger = logging.getLogger(__name__)


class Conn:
    """Centralized connection manager for Redis and Database connections"""

    def __init__(self) -> None:
        """Initialize all connections"""
        self.redis_client = self._init_redis()
        self.db_conn = self._init_database()

        self.openai_client = None
        self.gemini_client = None
        self._init_openai_client()
        self._init_gemini_client()
        self.environment = None
        self._init_environment()

    def _init_environment(self) -> None:
        """Initialize environment variables"""
        self.environment = os.getenv('ENVIRONMENT')
        # Treat empty ENVIRONMENT or common dev variants uniformly
        if self.environment in ("dev", "development", ""):
            self.environment = "dev"
        else:
            self.environment = "prod"
    def _init_openai_client(self) -> None:
        """Initialize OpenAI client"""
        api_key = os.getenv('OPENAI_API_KEY')
        if not api_key:
            raise ValueError("OPENAI_API_KEY environment variable is required")

        self.openai_client = OpenAI(api_key=api_key)

    def _init_gemini_client(self) -> None:
        """Initialize Gemini client"""
        api_key = os.getenv('GEMINI_API_KEY')
        if not api_key:
            raise ValueError("GEMINI_API_KEY environment variable is required")

        self.gemini_client = genai.Client(api_key=api_key)
    def _init_redis(self) -> redis.Redis:
        """Initialize Redis connection"""
        redis_host = os.environ.get("REDIS_HOST", "cache")
        redis_port = int(os.environ.get("REDIS_PORT", "6379"))
        redis_password = os.environ.get("REDIS_PASSWORD", "")

        client = redis.Redis(
            host=redis_host,
            port=redis_port,
            password=redis_password if redis_password else None,
            decode_responses=True,
            socket_connect_timeout=5,
            socket_timeout=None,  # No timeout to avoid read timeouts during blocking commands like BRPOP
            health_check_interval=30
        )

        # Test connection
        try:
            client.ping()
        except redis.exceptions.RedisError as redis_error:  # Narrow catch to Redis-specific errors
            logger.error("❌ Redis connection failed: %s", redis_error)
            raise

        return client

    def _init_database(self) -> PGConnection:
        """Initialize database connection"""
        db_host = os.environ.get("DB_HOST", "db")
        db_port = os.environ.get("DB_PORT", "5432")
        db_name = os.environ.get("POSTGRES_DB", "postgres")
        db_user = os.environ.get("DB_USER", "postgres")
        db_password = os.environ.get("DB_PASSWORD", "devpassword")

        try:
            connection = psycopg2.connect(
                host=db_host,
                port=db_port,
                database=db_name,
                user=db_user,
                password=db_password
            )
            return connection
        except psycopg2.Error as db_error:  # Narrow catch to PostgreSQL-specific errors
            logger.error("❌ Failed to connect to database: %s", db_error)
            raise

    def ensure_db_connection(self) -> None:
        """Ensure database connection is healthy, reconnect if needed"""
        try:
            # Test the connection with a simple query
            with self.db_conn.cursor() as cursor:
                try:
                    cursor.execute("SELECT 1")
                    _ = cursor.fetchone()
                except psycopg2.Error as test_error:
                    logger.error("❌ Database test query failed: %s", test_error, exc_info=True)
                    logger.error("🔍 PostgreSQL error details - Code: %s, SQLSTATE: %s", getattr(test_error, 'pgcode', 'N/A'), getattr(test_error, 'sqlstate', 'N/A'))
                    raise  # Re-raise to trigger reconnection
                except Exception as unexpected_error:
                    logger.error("❌ Unexpected error during database test query: %s", unexpected_error, exc_info=True)
                    raise  # Re-raise to trigger reconnection

        except (psycopg2.OperationalError, psycopg2.InterfaceError, AttributeError) as e:
            logger.warning("🔌 Database connection test failed, attempting reconnection: %s", e, exc_info=True)
            try:
                # Close existing connection if it exists
                if hasattr(self, 'db_conn') and self.db_conn:
                    try:
                        self.db_conn.close()
                    except psycopg2.Error as close_error:  # Narrow catch to PostgreSQL errors when closing
                        logger.warning("⚠️ Error closing database connection (expected during reconnection): %s", close_error)

                # Establish new connection
                self.db_conn = self._init_database()

            except psycopg2.Error as reconnect_error:  # Narrow catch during reconnection attempt
                logger.error("❌ Failed to reconnect to database: %s", reconnect_error, exc_info=True)
                raise  # Re-raise to let caller handle the failed reconnection

        except psycopg2.Error as db_error:
            # Catch other PostgreSQL errors that might indicate transaction issues
            logger.error("❌ PostgreSQL error during connection test: %s", db_error, exc_info=True)
            logger.error("🔍 Error details - Code: %s, SQLSTATE: %s", getattr(db_error, 'pgcode', 'N/A'), getattr(db_error, 'sqlstate', 'N/A'))

            # For transaction-related errors, try to rollback and reconnect
            if hasattr(db_error, 'pgcode') and db_error.pgcode in ['25P02']:  # current transaction is aborted
                logger.warning("🔄 Detected aborted transaction (pgcode: %s), attempting recovery", db_error.pgcode)
                try:
                    # Try to rollback the transaction
                    self.db_conn.rollback()
                except psycopg2.Error as rollback_error:  # Narrow catch for rollback issues
                    logger.warning("⚠️ Transaction rollback failed, forcing reconnection: %s", rollback_error)
                    # If rollback fails, force a reconnection
                    try:
                        self.db_conn.close()
                    except psycopg2.Error:  # Ignore specific PG errors when closing before reconnection
                        pass
                    self.db_conn = self._init_database()
            else:
                # For other database errors, don't automatically reconnect to avoid infinite loops
                logger.error("❌ Database connection test failed with non-recoverable error")
                raise

        except Exception as e:
            logger.error("❌ Unexpected error testing database connection: %s", e, exc_info=True)
            # For other errors, don't reconnect to avoid infinite loops
            raise

    @contextmanager
    def transaction(self, cursor_factory: Optional[Type[PGCursor]] = RealDictCursor) -> Iterator[PGCursor]:
        """
        Database transaction context manager that automatically handles commits and rollbacks.
        This prevents 'current transaction is aborted' errors by ensuring proper transaction cleanup.

        Args:
            cursor_factory: Cursor factory to use (defaults to RealDictCursor for backward compatibility)
                           Use None for plain tuple cursor (fastest for high-volume data)

        Usage:
            with self.conn.transaction() as cursor:
                cursor.execute("SELECT ...")
                result = cursor.fetchone()

            # For high-performance data access:
            with self.conn.transaction(cursor_factory=None) as cursor:
                cursor.execute("SELECT ...")
                result = cursor.fetchall()  # Returns plain tuples
        """

        # Ensure connection is healthy before starting transaction
        try:
            self.ensure_db_connection()
        except Exception as conn_error:
            logger.error("❌ Failed to ensure database connection before transaction: %s", conn_error, exc_info=True)
            raise

        cursor: Optional[PGCursor] = None
        try:
            # Start transaction by getting cursor with specified factory
            cursor = self.db_conn.cursor(cursor_factory=cursor_factory)

            yield cursor

            # If we get here, no exception occurred - commit the transaction
            self.db_conn.commit()

        except psycopg2.Error as db_error:
            # Database error occurred - rollback transaction
            logger.error("❌ Database error in transaction, rolling back: %s", db_error, exc_info=True)
            logger.error("🔍 PostgreSQL error details - Code: %s, SQLSTATE: %s", getattr(db_error, 'pgcode', 'N/A'), getattr(db_error, 'sqlstate', 'N/A'))

            try:
                self.db_conn.rollback()
            except psycopg2.Error as rollback_error:  # Narrow catch for rollback issues
                logger.error("❌ Failed to rollback transaction: %s", rollback_error, exc_info=True)
                # Force reconnection if rollback fails
                try:
                    self.db_conn.close()
                    self.db_conn = self._init_database()
                    logger.warning("⚠️ Forced database reconnection after rollback failure")
                except psycopg2.Error as reconnect_error:  # Narrow catch when forcing reconnection
                    logger.error("❌ Failed to reconnect after rollback failure: %s", reconnect_error, exc_info=True)

            raise  # Re-raise the original database error

        except Exception as general_error:
            # Non-database error occurred - still rollback transaction to be safe
            logger.error("❌ Non-database error in transaction, rolling back: %s", general_error, exc_info=True)

            try:
                self.db_conn.rollback()
            except psycopg2.Error as rollback_error:  # Narrow catch for rollback issues
                logger.warning("⚠️ Failed to rollback transaction after non-database error: %s", rollback_error)

            raise  # Re-raise the original error

        finally:
            # Always close cursor
            if cursor:
                try:
                    cursor.close()
                except psycopg2.Error as close_error:  # Narrow catch when closing cursor
                    logger.warning("⚠️ Error closing cursor (non-critical): %s", close_error)

    @contextmanager
    def get_connection(self) -> Iterator[PGConnection]:
        """Context manager to yield a healthy database connection."""
        # Ensure the DB connection is healthy or reconnect if needed
        self.ensure_db_connection()
        yield self.db_conn

    def check_connections(self) -> None:
        """Lightweight connection check - only when necessary"""
        # Quick Redis ping - this is very fast
        try:
            self.redis_client.ping()
        except redis.exceptions.RedisError as redis_error:  # Redis-specific failures
            logger.error("Redis connection lost, reconnecting: %s", redis_error)
            self.redis_client = self._init_redis()

        # Lightweight DB connection check to prevent stale connections
        try:
            self.ensure_db_connection()
        except (psycopg2.Error, AttributeError) as db_check_error:  # Specific expected failures
            logger.error("Database connection check failed: %s", db_check_error)
            # Don't raise here to avoid interrupting the worker loop

    def close_connections(self) -> None:
        """Close all connections gracefully"""
        try:
            if hasattr(self, 'redis_client') and self.redis_client:
                self.redis_client.close()
        except redis.exceptions.RedisError as redis_error:  # Redis-specific close errors
            logger.error("Error closing Redis connection: %s", redis_error)

        try:
            if hasattr(self, 'db_conn') and self.db_conn:
                self.db_conn.close()
        except psycopg2.Error as db_error:  # PostgreSQL-specific close errors
            logger.error("Error closing database connection: %s", db_error)