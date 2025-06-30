#!/usr/bin/env python3
"""
Debug script to test get_general_data function directly
"""

import sys
import os
import pandas as pd

# Add the worker src directory to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "services", "worker", "src"))

from data_accessors import DataAccessorProvider

def test_market_cap_filtering():
    """Test the market cap filtering directly"""
    print("Testing get_general_data with market_cap_min filter...")
    
    # Create data accessor
    accessor = DataAccessorProvider()
    
    # Test 1: Get all securities with market cap data
    print("\n1. Testing all securities with market_cap column...")
    try:
        all_with_market_cap = accessor.get_general_data(
            columns=["ticker", "market_cap"],
            filters={}  # No filters
        )
        print(f"   Total securities returned: {len(all_with_market_cap)}")
        print(f"   Securities with non-null market_cap: {all_with_market_cap['market_cap'].notna().sum()}")
        print(f"   Max market cap: {all_with_market_cap['market_cap'].max():,.0f}")
        print(f"   Min market cap: {all_with_market_cap['market_cap'].min():,.0f}")
        
        # Show top 5 by market cap
        top_5 = all_with_market_cap.nlargest(5, 'market_cap')
        print("\n   Top 5 by market cap:")
        for _, row in top_5.iterrows():
            print(f"     {row['ticker']}: ${row['market_cap']:,.0f}")
            
    except Exception as e:
        print(f"   ERROR: {e}")
        import traceback
        traceback.print_exc()
    
    # Test 2: Filter by market cap >= 50B
    print("\n2. Testing market_cap_min filter (50B)...")
    try:
        filtered_results = accessor.get_general_data(
            columns=["ticker", "market_cap"],
            filters={"market_cap_min": 50_000_000_000}
        )
        print(f"   Securities with market cap >= $50B: {len(filtered_results)}")
        
        if len(filtered_results) > 0:
            print("\n   Sample results:")
            sample = filtered_results.head(10)
            for _, row in sample.iterrows():
                print(f"     {row['ticker']}: ${row['market_cap']:,.0f}")
        else:
            print("   No results returned!")
            
    except Exception as e:
        print(f"   ERROR: {e}")
        import traceback
        traceback.print_exc()
    
    # Test 3: Test the exact strategy logic
    print("\n3. Testing exact strategy logic...")
    try:
        # This mimics the exact call from the strategy
        general_df = accessor.get_general_data(
            tickers=None,                                   # Universe-wide scan
            columns=["ticker", "market_cap"],               # Only fields we need
            filters={"market_cap_min": 50_000_000_000}      # ≥ $50 B
        )
        
        print(f"   Strategy call result: {type(general_df)}")
        print(f"   Length: {len(general_df) if general_df is not None else 'None'}")
        
        if general_df is not None and len(general_df) > 0:
            print(f"   Columns: {list(general_df.columns)}")
            print(f"   Index: {general_df.index.name}")
            print(f"   Sample data:")
            print(general_df.head())
        else:
            print("   No data returned!")
            
    except Exception as e:
        print(f"   ERROR: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    test_market_cap_filtering() 