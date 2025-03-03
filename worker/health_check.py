#!/usr/bin/env python3
"""
Health check script that uses the connection checking functionality from conn.py
"""

import os
import sys
import time
from conn import Conn

def main():
    """
    Main health check function that continuously monitors connections
    """
    # Get check intervals from environment
    check_interval = int(os.environ.get('HEALTHCHECK_INTERVAL', '30'))
    max_interval = int(os.environ.get('HEALTHCHECK_MAX_INTERVAL', '300'))
    current_interval = check_interval

    while True:
        try:
            # Create connection object
            conn = Conn(inside_container=True)
            
            # If successful, reset the check interval
            current_interval = check_interval
            print("Health check passed - all connections are healthy", flush=True)
            
        except Exception as e:
            print(f"Health check failed: {e}", flush=True)
            # Increase check interval (with cap) on failure
            current_interval = min(current_interval * 2, max_interval)
            sys.exit(1)
            
        time.sleep(current_interval)

if __name__ == "__main__":
    main() 