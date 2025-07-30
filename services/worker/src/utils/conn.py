"""
Connection Manager
Centralized connection management for Redis and Database connections
"""

import os
import json
import logging
import psycopg2
from psycopg2.extras import RealDictCursor
from contextlib import contextmanager
from datetime import datetime
import redis
from openai import OpenAI
from google import genai

# Configure logging
logger = logging.getLogger(__name__)


class Conn:
    """Centralized connection manager for Redis and Database connections"""

    def __init__(self):
        """Initialize all connections"""
        self.redis_client = self._init_redis()
        self.db_conn = self._init_database()

        self.openai_client = None
        self.gemini_client = None
        self._init_openai_client()
        self._init_gemini_client()
        self.environment = None
        self._init_environment()

    def _init_environment(self):
        """Initialize environment variables"""
        self.environment = os.getenv('ENVIRONMENT')
        if self.environment == "dev" or self.environment == "development" or self.environment == "":
            self.environment = "dev"
        else:
            self.environment = "prod"
        #logger.info(f"Environment initialized to: {self.environment}")
    def _init_openai_client(self):
        """Initialize OpenAI client"""
        api_key = os.getenv('OPENAI_API_KEY')
        if not api_key:
            raise ValueError("OPENAI_API_KEY environment variable is required")

        self.openai_client = OpenAI(api_key=api_key)
        #logger.info("OpenAI client initialized successfully")

    def _init_gemini_client(self):
        """Initialize Gemini client"""
        api_key = os.getenv('GEMINI_API_KEY')
        if not api_key:
            raise ValueError("GEMINI_API_KEY environment variable is required")

        self.gemini_client = genai.Client(api_key=api_key)
        #logger.info("Gemini client initialized successfully")
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
            #logger.info("✅ Redis connection established")
        except Exception as e:
            logger.error("❌ Redis connection failed: %s", e)
            raise

        return client

    def _init_database(self):
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
            #logger.info("✅ Database connection established")
            return connection
        except Exception as e:
            logger.error("❌ Failed to connect to database: %s", e)
            raise

    def ensure_db_connection(self):
        """Ensure database connection is healthy, reconnect if needed"""
        logger.debug("🔌 Testing database connection health")
        try:
            # Test the connection with a simple query
            logger.debug("🔍 Executing connection test query: SELECT 1")
            with self.db_conn.cursor() as cursor:
                try:
                    cursor.execute("SELECT 1")
                    result = cursor.fetchone()
                    logger.debug("✅ Connection test successful, result: %s", result)
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
                    logger.debug("🔌 Closing existing database connection")
                    try:
                        self.db_conn.close()
                        logger.debug("✅ Existing connection closed successfully")
                    except Exception as close_error:
                        logger.debug(f"⚠️ Error closing database connection (expected during reconnection): {close_error}")

                # Establish new connection
                logger.info("🔄 Establishing new database connection")
                self.db_conn = self._init_database()
                logger.info("✅ Database reconnection successful")

            except Exception as reconnect_error:
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
                    logger.debug("🔄 Attempting transaction rollback")
                    self.db_conn.rollback()
                    logger.info("✅ Transaction rollback successful")
                except Exception as rollback_error:
                    logger.warning("⚠️ Transaction rollback failed, forcing reconnection: %s", rollback_error)
                    # If rollback fails, force a reconnection
                    try:
                        self.db_conn.close()
                    except Exception:
                        pass
                    self.db_conn = self._init_database()
                    logger.info("✅ Forced database reconnection successful")
            else:
                # For other database errors, don't automatically reconnect to avoid infinite loops
                logger.error("❌ Database connection test failed with non-recoverable error")
                raise

        except Exception as e:
            logger.error("❌ Unexpected error testing database connection: %s", e, exc_info=True)
            # For other errors, don't reconnect to avoid infinite loops
            raise

    @contextmanager
    def transaction(self, cursor_factory=RealDictCursor):
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
        logger.debug("🔄 Starting database transaction context")

        # Ensure connection is healthy before starting transaction
        try:
            self.ensure_db_connection()
        except Exception as conn_error:
            logger.error("❌ Failed to ensure database connection before transaction: %s", conn_error, exc_info=True)
            raise

        cursor = None
        try:
            # Start transaction by getting cursor with specified factory
            cursor = self.db_conn.cursor(cursor_factory=cursor_factory)
            logger.debug("✅ Database transaction started successfully")

            yield cursor

            # If we get here, no exception occurred - commit the transaction
            self.db_conn.commit()
            logger.debug("✅ Database transaction committed successfully")

        except psycopg2.Error as db_error:
            # Database error occurred - rollback transaction
            logger.error("❌ Database error in transaction, rolling back: %s", db_error, exc_info=True)
            logger.error("🔍 PostgreSQL error details - Code: %s, SQLSTATE: %s", getattr(db_error, 'pgcode', 'N/A'), getattr(db_error, 'sqlstate', 'N/A'))

            try:
                self.db_conn.rollback()
                logger.info("✅ Transaction rollback successful")
            except Exception as rollback_error:
                logger.error("❌ Failed to rollback transaction: %s", rollback_error, exc_info=True)
                # Force reconnection if rollback fails
                try:
                    self.db_conn.close()
                    self.db_conn = self._init_database()
                    logger.warning("⚠️ Forced database reconnection after rollback failure")
                except Exception as reconnect_error:
                    logger.error("❌ Failed to reconnect after rollback failure: %s", reconnect_error, exc_info=True)

            raise  # Re-raise the original database error

        except Exception as general_error:
            # Non-database error occurred - still rollback transaction to be safe
            logger.error("❌ Non-database error in transaction, rolling back: %s", general_error, exc_info=True)

            try:
                self.db_conn.rollback()
                logger.info("✅ Transaction rollback successful after non-database error")
            except Exception as rollback_error:
                logger.warning(f"⚠️ Failed to rollback transaction after non-database error: {rollback_error}")

            raise  # Re-raise the original error

        finally:
            # Always close cursor
            if cursor:
                try:
                    cursor.close()
                    logger.debug("🔌 Database cursor closed")
                except Exception as close_error:
                    logger.debug("⚠️ Error closing cursor (non-critical): %s", close_error)

    @contextmanager
    def get_connection(self):
        """Context manager to yield a healthy database connection."""
        # Ensure the DB connection is healthy or reconnect if needed
        self.ensure_db_connection()
        yield self.db_conn

    def check_connections(self):
        """Lightweight connection check - only when necessary"""
        # Quick Redis ping - this is very fast
        try:
            self.redis_client.ping()
        except Exception as e:
            logger.error("Redis connection lost, reconnecting: %s", e)
            self.redis_client = self._init_redis()

        # Lightweight DB connection check to prevent stale connections
        try:
            self.ensure_db_connection()
        except Exception as e:
            logger.error("Database connection check failed: %s", e)
            # Don't raise here to avoid interrupting the worker loop

    def close_connections(self):
        """Close all connections gracefully"""
        try:
            if hasattr(self, 'redis_client') and self.redis_client:
                self.redis_client.close()
                logger.info("🔌 Redis connection closed")
        except Exception as e:
            logger.error("Error closing Redis connection: %s", e)

        try:
            if hasattr(self, 'db_conn') and self.db_conn:
                self.db_conn.close()
                logger.info("🔌 Database connection closed")
        except Exception as e:
            logger.error("Error closing database connection: %s", e)