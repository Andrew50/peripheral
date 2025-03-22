#!/usr/bin/env python3
import os
import sys
import time
import socket
from datetime import datetime
import signal

# Import connection module
from conn import Conn

# Default values for health check interval
DEFAULT_INTERVAL = 30  # seconds
MAX_INTERVAL = 300  # seconds
RETRY_BACKOFF_FACTOR = 1.5

def log(message):
    """Log message with timestamp to stdout"""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{timestamp}] {message}", flush=True)

def get_env_var(name, default):
    """Get environment variable with fallback to default"""
    value = os.environ.get(name)
    return value if value is not None else default

def handle_sigterm(signum, frame):
    """Handle SIGTERM signal"""
    log("Received SIGTERM signal. Shutting down gracefully...")
    sys.exit(0)

def main():
    # Register signal handler for graceful shutdown
    signal.signal(signal.SIGTERM, handle_sigterm)
    
    # Get health check configuration from environment
    check_interval = int(get_env_var("HEALTHCHECK_INTERVAL", DEFAULT_INTERVAL))
    max_interval = int(get_env_var("HEALTHCHECK_MAX_INTERVAL", MAX_INTERVAL))
    
    # Log startup information
    log(f"Starting health check service. Checking every {check_interval} seconds")
    hostname = socket.gethostname()
    log(f"Running on host: {hostname}")
    
    # Initialize connection
    conn = None
    consecutive_failures = 0
    current_interval = check_interval
    
    while True:
        try:
            # Initialize or reinitialize connection if needed
            if conn is None:
                log("Initializing database and Redis connections...")
                conn = Conn(inside_container=True)
            
            # Check connections
            db_ok, redis_ok = conn.check_connection()
            
            if db_ok and redis_ok:
                log("Health check: OK - Database and Redis connections are healthy")
                # Reset failure counter and interval on success
                consecutive_failures = 0
                current_interval = check_interval
            else:
                # Create detailed error message
                status = []
                if not db_ok:
                    status.append("Database connection FAILED")
                if not redis_ok:
                    status.append("Redis connection FAILED")
                
                error_msg = " | ".join(status)
                log(f"Health check: FAIL - {error_msg}")
                
                # Reset connection to force reconnect on next iteration
                conn = None
                
                # Increment failure counter
                consecutive_failures += 1
                
                # Apply backoff to interval on consecutive failures
                current_interval = min(
                    check_interval * (RETRY_BACKOFF_FACTOR ** consecutive_failures),
                    max_interval
                )
                log(f"Next check in {current_interval:.1f} seconds after {consecutive_failures} consecutive failures")
        
        except Exception as e:
            log(f"Error in health check: {str(e)}")
            conn = None  # Reset connection on error
            consecutive_failures += 1
            
            # Apply backoff to interval on consecutive failures
            current_interval = min(
                check_interval * (RETRY_BACKOFF_FACTOR ** consecutive_failures),
                max_interval
            )
        
        # Wait for the next check
        time.sleep(current_interval)

if __name__ == "__main__":
    main() 