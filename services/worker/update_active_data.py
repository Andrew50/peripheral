#!/usr/bin/env python3
"""
Script to manually update active market data in Redis.
This script calls the update_active function from active.py to calculate
active market metrics and store them in Redis.
"""

from conn import Conn
from active import update_active

def main():
    """Main function to update active market data"""
    print("Initializing connection...")
    # Use inside_container=False because we're running locally
    data = Conn(inside_container=False)
    
    print("Starting active market data update...")
    result = update_active(data)
    print(f"Update completed: {result}")

if __name__ == "__main__":
    main() 