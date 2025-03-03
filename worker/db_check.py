#!/usr/bin/env python3
"""
Database and Redis connection check utility.
This script is used by the worker pods to check database and Redis connectivity.
"""

import os
import sys
import time
import psycopg2
import redis


def check_db_connection(max_retries=5, retry_delay=2):
    """
    Check database connection with retries.
    
    Args:
        max_retries: Maximum number of connection attempts
        retry_delay: Delay between retries in seconds
        
    Returns:
        bool: True if connection successful, False otherwise
    """
    # Get database credentials from environment variables
    db_host = os.environ.get('DB_HOST', 'db')
    db_port = os.environ.get('DB_PORT', '5432')
    db_user = os.environ.get('DB_USER', 'postgres')
    db_password = os.environ.get('DB_PASSWORD', '')
    
    retry_count = 0
    while retry_count < max_retries:
        try:
            print(f"Attempting to connect to database at {db_host}:{db_port} (attempt {retry_count+1}/{max_retries})...")
            conn = psycopg2.connect(
                host=db_host,
                port=int(db_port),
                user=db_user,
                password=db_password
            )
            conn.close()
            print("Database connection successful")
            return True
        except Exception as e:
            retry_count += 1
            print(f"Database connection error: {e}")
            if retry_count < max_retries:
                print(f"Retrying in {retry_delay} seconds...")
                time.sleep(retry_delay)
                # Exponential backoff
                retry_delay = min(retry_delay * 2, 30)
    
    print(f"Failed to connect to database after {max_retries} attempts")
    return False


def check_redis_connection(max_retries=5, retry_delay=2):
    """
    Check Redis connection with retries.
    
    Args:
        max_retries: Maximum number of connection attempts
        retry_delay: Delay between retries in seconds
        
    Returns:
        bool: True if connection successful, False otherwise
    """
    # Get Redis configuration from environment variables
    redis_host = os.environ.get('REDIS_HOST', 'cache')
    redis_port = os.environ.get('REDIS_PORT', '6379')
    redis_password = os.environ.get('REDIS_PASSWORD', '')
    
    retry_count = 0
    while retry_count < max_retries:
        try:
            print(f"Attempting to connect to Redis at {redis_host}:{redis_port} (attempt {retry_count+1}/{max_retries})...")
            r = redis.Redis(
                host=redis_host,
                port=int(redis_port),
                password=redis_password,
                socket_timeout=5.0,
                socket_connect_timeout=5.0
            )
            r.ping()
            print("Redis connection successful")
            return True
        except Exception as e:
            retry_count += 1
            print(f"Redis connection error: {e}")
            if retry_count < max_retries:
                print(f"Retrying in {retry_delay} seconds...")
                time.sleep(retry_delay)
                # Exponential backoff
                retry_delay = min(retry_delay * 2, 30)
    
    print(f"Failed to connect to Redis after {max_retries} attempts")
    return False


def check_all_connections(db_retries=5, redis_retries=5):
    """
    Check both database and Redis connections.
    
    Args:
        db_retries: Maximum number of database connection attempts
        redis_retries: Maximum number of Redis connection attempts
        
    Returns:
        bool: True if both connections successful, False otherwise
    """
    db_ok = check_db_connection(max_retries=db_retries)
    redis_ok = check_redis_connection(max_retries=redis_retries)
    
    return db_ok and redis_ok


if __name__ == "__main__":
    # If run as a script, check connections and exit with appropriate status code
    if check_all_connections():
        sys.exit(0)
    else:
        sys.exit(1) 